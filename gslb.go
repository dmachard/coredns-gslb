package gslb

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/oschwald/geoip2-golang"
	"github.com/redis/go-redis/v9"
)

var log = clog.NewWithPlugin("gslb")

type CustomSubnet struct {
	Subnet   string
	IPNet    *net.IPNet
	Location string
}

type GSLB struct {
	Next                plugin.Handler
	Zones               map[string]string             // List of authoritative domains
	Records             map[string]map[string]*Record // zone -> fqdn -> record
	HealthcheckProfiles map[string]*HealthCheck       `yaml:"healthcheck_profiles"`

	Zone                      string   // Zone attendue pour la vérification des records
	LastResolution            sync.Map // key: domain (string), value: time.Time
	RoundRobinIndex           sync.Map
	MaxStaggerStart           string
	BatchSizeStart            int
	ResolutionIdleTimeout     string
	ResolutionIdleMultiplier  int // Multiplier for slow healthcheck interval
	HealthcheckIdleMultiplier int // Multiplier for slow healthcheck interval
	Mutex                     sync.RWMutex
	LocationMapMutex          sync.Mutex
	UseEDNSCSubnet            bool
	LocationMap               map[string]string
	LocationMapIPNet          []CustomSubnet
	GeoIPCountryDB            *geoip2.Reader // Loaded MaxMind DB (country)
	GeoIPCityDB               *geoip2.Reader // Loaded MaxMind DB (city)
	GeoIPASNDB                *geoip2.Reader // Loaded MaxMind DB (ASN)
	APIEnable                 bool           // Enable/disable API HTTP server
	APICertPath               string         // TLS certificate path for API
	APIKeyPath                string         // TLS key path for API
	APIListenAddr             string         // API listen address (default 0.0.0.0)
	APIListenPort             string         // API listen port (default 8080)
	APIBasicUser              string         // HTTP Basic Auth username (optional)
	APIBasicPass              string         // HTTP Basic Auth password (optional)
	// DisableTXT disables TXT record resolution if set to true
	DisableTXT bool

	RedisEnable    bool
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	RedisKeyPrefix string
	RedisSyncMode  string

	redisClient  *redis.Client
	redisContext context.Context
	redisCancel  context.CancelFunc

	StatePersistEnable   bool
	StatePersistPath     string
	StatePersistInterval string
	StateMaxAge          string

	statePersistCancel context.CancelFunc
}

func (g *GSLB) Name() string { return "gslb" }

func (g *GSLB) ServeAPI() {
	mux := http.NewServeMux()
	g.RegisterAPIHandlers(mux)
	listenAddr := g.APIListenAddr + ":" + g.APIListenPort
	if g.APICertPath != "" && g.APIKeyPath != "" {
		go func() {
			_ = http.ListenAndServeTLS(listenAddr, g.APICertPath, g.APIKeyPath, mux)
		}()
	} else {
		go func() {
			_ = http.ListenAndServe(listenAddr, mux)
		}()
	}
}

func (g *GSLB) updateRecords(ctx context.Context, newGSLB *GSLB) {
	for zone, newRecords := range newGSLB.Records {
		oldRecords, exists := g.Records[zone]
		if !exists {
			log.Infof("Not yet implemented: new zone %s", zone)
			continue
		}
		// This zone exists, update existing records
		for fqdn, newRecord := range newRecords {
			oldRecord, exists := oldRecords[fqdn]
			if !exists {
				newRecord.Fqdn = fqdn
				newRecord.Zone = zone
				g.Records[zone][fqdn] = newRecord
				log.Infof("Added new record for zone %s: %s", zone, fqdn)
				newRecord.updateRecordHealthStatus()
				recordCtx, cancel := context.WithCancel(ctx)
				newRecord.cancelFunc = cancel
				go newRecord.scrapeBackends(recordCtx, g)
			} else {
				log.Infof("Reloading record %s in zone %s", fqdn, zone)
				oldRecord.updateRecord(newRecord)
				oldRecord.updateRecordHealthStatus()
			}
		}
		// Remove records from old zone that are no longer present in newGSLB.Records
		for fqdn := range oldRecords {
			if _, exists := newRecords[fqdn]; !exists {
				if record := oldRecords[fqdn]; record.cancelFunc != nil {
					record.cancelFunc()
				}
				delete(g.Records[zone], fqdn)
				log.Infof("Records [%s] removed from zone %s", fqdn, zone)
			}
		}
	}

	// Update metrics
	g.updateMetrics()
}

func (g *GSLB) initializeRecordsFromFiles(ctx context.Context, zoneFiles map[string]string) error {
	g.Records = make(map[string]map[string]*Record)
	for zone, file := range zoneFiles {
		log.Infof("Loading records for zone %s from %s", zone, file)
		if err := loadConfigFile(g, file, zone); err != nil {
			log.Errorf("Failed to load records for zone %s from %s: %v", zone, file, err)
			return err
		}
		log.Infof("Loaded %d records for zone %s", len(g.Records[zone]), zone)
	}

	// Load initial state from file if enabled
	if g.StatePersistEnable && !g.RedisEnable {
		if err := g.LoadState(); err != nil {
			log.Errorf("Failed to load initial state: %v", err)
		}
	}

	groups := g.batchRecords(g.BatchSizeStart)
	for i, group := range groups {
		type recordContext struct {
			record *Record
			ctx    context.Context
		}
		groupCtxs := make([]recordContext, len(group))
		for idx, record := range group {
			recordCtx, cancel := context.WithCancel(ctx)
			record.cancelFunc = cancel
			groupCtxs[idx] = recordContext{record: record, ctx: recordCtx}
		}

		go func(gCtxs []recordContext, delay time.Duration) {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
			for _, rCtx := range gCtxs {
				record := rCtx.record
				recordCtx := rCtx.ctx
				if recordCtx.Err() != nil {
					continue
				}
				domain := record.Fqdn
				log.Debugf("[%s] Starting health checks for backends", domain)

				// Initialize health status from Redis if enabled
				if g.RedisEnable {
					record.mutex.Lock()
					for _, backend := range record.Backends {
						alive, err := g.GetRedisHealth(recordCtx, record.Zone, record.Fqdn, backend.GetAddress())
						if err == nil {
							backend.SetAlive(alive)
						}
					}
					record.mutex.Unlock()
				}

				// Initialize health status for existing record
				record.updateRecordHealthStatus()
				go record.scrapeBackends(recordCtx, g)
			}
		}(groupCtxs, time.Duration(i)*g.staggerDelay(len(groups)))
	}

	// Update metrics
	g.updateMetrics()
	return nil
}

func (g *GSLB) updateMetrics() {
	SetZonesTotal(float64(len(g.Records)))

	// Set total records configured
	totalRecords := 0
	for _, records := range g.Records {
		totalRecords += len(records)
	}
	SetRecordsTotal(float64(totalRecords))

	// Set total backends configured
	totalBackends := 0
	for _, records := range g.Records {
		for _, record := range records {
			totalBackends += len(record.Backends)
		}
	}
	SetBackendsTotal(float64(totalBackends))

	// Set total healthchecks configured
	totalHealthchecks := 0
	for _, records := range g.Records {
		for _, record := range records {
			for _, backend := range record.Backends {
				totalHealthchecks += len(backend.GetHealthChecks())
			}
		}
	}
	SetHealthchecksTotal(float64(totalHealthchecks))
}

func (g *GSLB) batchRecords(batchSize int) [][]*Record {
	var groups [][]*Record
	var current []*Record

	for _, records := range g.Records {
		for domain, record := range records {
			record.Fqdn = domain
			current = append(current, record)
			if len(current) == batchSize {
				groups = append(groups, current)
				current = nil
			}
		}
	}

	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

func (g *GSLB) staggerDelay(totalBatches int) time.Duration {
	if totalBatches == 0 {
		return 0
	}
	return g.GetMaxStaggerStart() / time.Duration(totalBatches)
}

func (g *GSLB) updateLastResolutionTime(domain string) {
	g.LastResolution.Store(domain, time.Now())
}

func (g *GSLB) GetMaxStaggerStart() time.Duration {
	d, err := time.ParseDuration(g.MaxStaggerStart)
	if err != nil {
		d, _ = time.ParseDuration("60s")
	}
	return d
}

func (g *GSLB) GetResolutionIdleTimeout() time.Duration {
	d, err := time.ParseDuration(g.ResolutionIdleTimeout)
	if err != nil {
		d, _ = time.ParseDuration("3600s")
	}
	return d
}
