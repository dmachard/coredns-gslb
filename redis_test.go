package gslb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRedisIntegration(t *testing.T) {
	g := &GSLB{
		RedisEnable:    true,
		RedisAddr:      "127.0.0.1:6379",
		RedisPassword:  "",
		RedisDB:        0,
		RedisKeyPrefix: "testgslb:",
		RedisSyncMode:  "lock",
	}

	err := g.ConnectRedis()
	if err != nil {
		t.Skipf("Skipping Redis test, connection failed: %v", err)
	}
	defer g.redisClient.Close()
	defer g.redisCancel()

	ctx := context.Background()

	// Cleanup
	g.redisClient.Del(ctx, "testgslb:health:testzone:testfqdn:1.2.3.4")
	g.redisClient.Del(ctx, "testgslb:lock:testzone:testfqdn:1.2.3.4")

	// Test lock acquisition
	acquired, err := g.AcquireRedisLock(ctx, "testzone", "testfqdn", "1.2.3.4", 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Try acquiring again - should fail (locked)
	acquired2, err := g.AcquireRedisLock(ctx, "testzone", "testfqdn", "1.2.3.4", 2*time.Second)
	assert.NoError(t, err)
	assert.False(t, acquired2)

	// Test writing health check results
	err = g.SetRedisHealth(ctx, "testzone", "testfqdn", "1.2.3.4", true, 2*time.Second)
	assert.NoError(t, err)

	// Test reading health check results
	alive, err := g.GetRedisHealth(ctx, "testzone", "testfqdn", "1.2.3.4")
	assert.NoError(t, err)
	assert.True(t, alive)

	// Verify Pub/Sub updates the local cache
	b := &Backend{Address: "1.2.3.4", Enable: true, Alive: false}
	rec := &Record{
		Fqdn:     "testfqdn",
		Zone:     "testzone",
		Backends: []BackendInterface{b},
	}
	g.Mutex.Lock()
	g.Records = map[string]map[string]*Record{
		"testzone": {
			"testfqdn": rec,
		},
	}
	g.Mutex.Unlock()

	// Write "unhealthy" status through SetRedisHealth (which publishes it)
	err = g.SetRedisHealth(ctx, "testzone", "testfqdn", "1.2.3.4", false, 2*time.Second)
	assert.NoError(t, err)

	// Wait for Pub/Sub delivery
	time.Sleep(150 * time.Millisecond)

	assert.False(t, b.IsAlive())
}

func TestRedisPassiveHealthCheck(t *testing.T) {
	g := &GSLB{
		RedisEnable:    true,
		RedisAddr:      "127.0.0.1:6379",
		RedisPassword:  "",
		RedisDB:        0,
		RedisKeyPrefix: "testgslbpassive:",
		RedisSyncMode:  "lock",
	}

	err := g.ConnectRedis()
	if err != nil {
		t.Skipf("Skipping Redis test, connection failed: %v", err)
	}
	defer g.redisClient.Close()
	defer g.redisCancel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Clean keys
	g.redisClient.Del(ctx, "testgslbpassive:health:testzone:passivefqdn:10.10.10.10")

	// Set health in Redis first
	err = g.SetRedisHealth(ctx, "testzone", "passivefqdn", "10.10.10.10", true, 10*time.Second)
	assert.NoError(t, err)

	// Create a passive backend
	b := &Backend{
		Address: "10.10.10.10",
		Enable:  true,
		Alive:   false,
		Passive: true, // Passive backend!
		Rise:    1,
		Fall:    1,
	}

	rec := &Record{
		Fqdn:           "passivefqdn",
		Zone:           "testzone",
		Backends:       []BackendInterface{b},
		ScrapeInterval: "1s",
	}

	g.Mutex.Lock()
	g.Records = map[string]map[string]*Record{
		"testzone": {
			"passivefqdn": rec,
		},
	}
	g.Mutex.Unlock()

	go rec.scrapeBackends(ctx, g)

	// Wait for scrape cycle to pick up Redis state
	time.Sleep(1200 * time.Millisecond)

	assert.True(t, b.IsAlive())

	// Write false to Redis
	err = g.SetRedisHealth(ctx, "testzone", "passivefqdn", "10.10.10.10", false, 10*time.Second)
	assert.NoError(t, err)

	time.Sleep(1200 * time.Millisecond)
	assert.False(t, b.IsAlive())
}
