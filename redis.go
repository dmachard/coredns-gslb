package gslb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisHealthUpdate struct {
	Zone    string `json:"zone"`
	Fqdn    string `json:"fqdn"`
	Address string `json:"address"`
	Alive   bool   `json:"alive"`
}

// ConnectRedis initializes the Redis client and starts the Pub/Sub subscriber.
func (g *GSLB) ConnectRedis() error {
	g.redisClient = redis.NewClient(&redis.Options{
		Addr:     g.RedisAddr,
		Password: g.RedisPassword,
		DB:       g.RedisDB,
	})

	g.redisContext, g.redisCancel = context.WithCancel(context.Background())

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := g.redisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	// Start Pub/Sub subscription in background
	go g.subscribeRedisUpdates()

	return nil
}

// subscribeRedisUpdates listens for health updates from Redis and updates local state.
func (g *GSLB) subscribeRedisUpdates() {
	pubKey := g.RedisKeyPrefix + "health:updates"
	pubsub := g.redisClient.Subscribe(g.redisContext, pubKey)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-g.redisContext.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var update RedisHealthUpdate
			if err := json.Unmarshal([]byte(msg.Payload), &update); err != nil {
				log.Errorf("Failed to unmarshal Redis health update: %v", err)
				continue
			}

			g.Mutex.Lock()
			records, exists := g.Records[update.Zone]
			if !exists {
				g.Mutex.Unlock()
				continue
			}
			record, exists := records[update.Fqdn]
			g.Mutex.Unlock()

			if exists {
				record.mutex.Lock()
				found := false
				for _, b := range record.Backends {
					if b.GetAddress() == update.Address {
						oldAlive := b.IsAlive()
						if oldAlive != update.Alive {
							log.Infof("[%s] backend status change from Redis [address=%s]: alive changed from %v to %v", record.Fqdn, b.GetAddress(), oldAlive, update.Alive)
							b.SetAlive(update.Alive)
						}
						found = true
						break
					}
				}
				record.mutex.Unlock()

				if found {
					record.updateRecordHealthStatus()
				}
			}
		}
	}
}

// SetRedisHealth stores the health check result in Redis and publishes it.
func (g *GSLB) SetRedisHealth(ctx context.Context, zone, fqdn, address string, alive bool, ttl time.Duration) error {
	if g.redisClient == nil {
		return nil
	}
	key := fmt.Sprintf("%shealth:%s:%s:%s", g.RedisKeyPrefix, zone, fqdn, address)
	val := "0"
	if alive {
		val = "1"
	}
	err := g.redisClient.Set(ctx, key, val, ttl).Err()
	if err != nil {
		return err
	}

	// Publish update to channel
	pubKey := g.RedisKeyPrefix + "health:updates"
	update := RedisHealthUpdate{
		Zone:    zone,
		Fqdn:    fqdn,
		Address: address,
		Alive:   alive,
	}
	data, err := json.Marshal(update)
	if err == nil {
		_ = g.redisClient.Publish(ctx, pubKey, string(data)).Err()
	}
	return nil
}

// GetRedisHealth fetches the current health check result from Redis.
func (g *GSLB) GetRedisHealth(ctx context.Context, zone, fqdn, address string) (bool, error) {
	if g.redisClient == nil {
		return false, fmt.Errorf("redis client is not initialized")
	}
	key := fmt.Sprintf("%shealth:%s:%s:%s", g.RedisKeyPrefix, zone, fqdn, address)
	val, err := g.redisClient.Get(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return val == "1", nil
}

// AcquireRedisLock attempts to acquire a lock for performing a health check.
func (g *GSLB) AcquireRedisLock(ctx context.Context, zone, fqdn, address string, ttl time.Duration) (bool, error) {
	if g.redisClient == nil {
		return false, fmt.Errorf("redis client is not initialized")
	}
	lockKey := fmt.Sprintf("%slock:%s:%s:%s", g.RedisKeyPrefix, zone, fqdn, address)
	return g.redisClient.SetNX(ctx, lockKey, "1", ttl).Result()
}
