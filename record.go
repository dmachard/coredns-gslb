package gslb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/creasty/defaults"
	"gopkg.in/yaml.v3"
)

// FailoverPolicy represents the behavior of the record when all backends are unhealthy/disabled.
type FailoverPolicy struct {
	Mode        string   `yaml:"mode"`
	FallbackIPs []string `yaml:"fallback_ips"`
	Rcode       string   `yaml:"rcode"`
}

// Record represents a GSLB record in the YAML config.
type Record struct {
	Fqdn           string
	Mode           string
	Backends       []BackendInterface
	Owner          string
	Description    string
	RecordTTL      int
	ScrapeInterval string
	ScrapeRetries  int
	ScrapeTimeout  string
	FailoverPolicy FailoverPolicy
	Discovery      *DiscoveryConfig
	ticker         *time.Ticker
	mutex          sync.RWMutex
	cancelFunc     context.CancelFunc
}

func (r *Record) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw struct {
		Mode           string           `yaml:"mode" default:"failover"`
		Owner          string           `yaml:"owner" default:""`
		Description    string           `yaml:"description" default:""`
		Ttl            int              `yaml:"record_ttl" default:"30"`
		ScrapeInterval string           `yaml:"scrape_interval" default:"10s"`
		ScrapeRetries  int              `yaml:"scrape_retries" default:"1"`
		ScrapeTimeout  string           `yaml:"scrape_timeout" default:"5s"`
		Backends       []interface{}    `yaml:"backends"`
		FailoverPolicy FailoverPolicy   `yaml:"failover_policy"`
		Discovery      *DiscoveryConfig `yaml:"discovery"`
	}
	defaults.Set(&raw)

	if err := unmarshal(&raw); err != nil {
		return err
	}

	r.Mode = raw.Mode
	r.Owner = raw.Owner
	r.Description = raw.Description
	r.RecordTTL = raw.Ttl
	r.ScrapeInterval = raw.ScrapeInterval
	r.ScrapeRetries = raw.ScrapeRetries
	r.ScrapeTimeout = raw.ScrapeTimeout
	r.FailoverPolicy = raw.FailoverPolicy
	r.Discovery = raw.Discovery

	for _, backendData := range raw.Backends {
		var backend Backend
		backendYaml, err := yaml.Marshal(backendData)
		if err != nil {
			return fmt.Errorf("failed to serialize backend: %w", err)
		}

		err = yaml.Unmarshal(backendYaml, &backend)
		if err != nil {
			return fmt.Errorf("failed to decode backend: %w", err)
		}

		r.Backends = append(r.Backends, &backend)
	}
	// No direct call to SetBackendsTotal or SetRecordsTotal here; these are set globally after all records are loaded/updated.
	return nil
}

// updateRecord updates an existing record incrementally
func (r *Record) updateRecord(newRecord *Record) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.Mode != newRecord.Mode {
		log.Debugf("[%s] mode changed from %s to %s", r.Fqdn, r.Mode, newRecord.Mode)
		r.Mode = newRecord.Mode
	}

	if r.Owner != newRecord.Owner {
		log.Debugf("[%s] owner changed from %s to %s", r.Fqdn, r.Owner, newRecord.Owner)
		r.Owner = newRecord.Owner
	}

	if r.RecordTTL != newRecord.RecordTTL {
		log.Debugf("[%s] DNS TTL changed from %d to %d", r.Fqdn, r.RecordTTL, newRecord.RecordTTL)
		r.RecordTTL = newRecord.RecordTTL
	}

	if r.Description != newRecord.Description {
		log.Debugf("[%s] description changed", r.Fqdn)
		r.Description = newRecord.Description
	}

	if r.ScrapeInterval != newRecord.ScrapeInterval {
		log.Debugf("[%s] scrape interval changed from %s to %s", r.Fqdn, r.ScrapeInterval, newRecord.ScrapeInterval)
		r.ScrapeInterval = newRecord.ScrapeInterval
		r.ticker.Reset(r.GetScrapeInterval())
	}

	if r.ScrapeRetries != newRecord.ScrapeRetries {
		log.Debugf("[%s] scrape retries changed from %d to %d", r.Fqdn, r.ScrapeRetries, newRecord.ScrapeRetries)
		r.ScrapeRetries = newRecord.ScrapeRetries
	}

	if r.ScrapeTimeout != newRecord.ScrapeTimeout {
		log.Debugf("[%s] scrape timeout changed from %s to %s", r.Fqdn, r.ScrapeTimeout, newRecord.ScrapeTimeout)
		r.ScrapeTimeout = newRecord.ScrapeTimeout
	}

	if r.FailoverPolicy.Mode != newRecord.FailoverPolicy.Mode ||
		r.FailoverPolicy.Rcode != newRecord.FailoverPolicy.Rcode ||
		len(r.FailoverPolicy.FallbackIPs) != len(newRecord.FailoverPolicy.FallbackIPs) {
		log.Debugf("[%s] failover policy changed", r.Fqdn)
		r.FailoverPolicy = newRecord.FailoverPolicy
	}

	r.Discovery = newRecord.Discovery

	// Update or add backends
	for _, newBackend := range newRecord.Backends {
		newBackend.SetFqdn(r.Fqdn)

		found := false
		for _, oldBackend := range r.Backends {
			// need to update backend ?
			if oldBackend.GetAddress() == newBackend.GetAddress() {
				oldBackend.updateBackend(newBackend)
				found = true
				break
			}
		}

		// add new backend and start healthcheck
		if !found {
			log.Debugf("[%s] new backend added %s", r.Fqdn, newBackend.GetAddress())
			r.Backends = append(r.Backends, newBackend)
			if newBackend.IsEnabled() {
				go newBackend.runHealthChecks(r.ScrapeRetries, r.GetScrapeTimeout())
			}
		}
	}

	// Remove deleted backends
	for i := 0; i < len(r.Backends); {
		backend := r.Backends[i]
		found := false
		for _, newBackend := range newRecord.Backends {
			if backend.GetAddress() == newBackend.GetAddress() {
				found = true
				break
			}
		}
		if !found {
			backend.removeBackend()
			r.Backends = append(r.Backends[:i], r.Backends[i+1:]...)
		} else {
			i++
		}
	}
}

// GetScrapeInterval returns the health check interval for HTTPHealthCheck
func (r *Record) GetScrapeInterval() time.Duration {
	return parseDurationWithDefault(r.ScrapeInterval, "10s")
}

// GetScrapeTimeout returns the health check timeout for HTTPHealthCheck
func (r *Record) GetScrapeTimeout() time.Duration {
	return parseDurationWithDefault(r.ScrapeTimeout, "5s")
}

func (r *Record) scrapeBackends(ctx context.Context, g *GSLB) {
	log.Debugf("[%s] Scraper loop started with interval %s", r.Fqdn, r.GetScrapeInterval())
	// Initialize ticker if it does not exist
	scrapeInterval := r.GetScrapeInterval()
	if r.ticker == nil {
		r.ticker = time.NewTicker(scrapeInterval)
		defer r.ticker.Stop()
	}

	// for tracing only
	for _, backend := range r.Backends {
		backend.SetFqdn(r.Fqdn)
	}

	for {
		select {
		case <-r.ticker.C:
			log.Debugf("[%s] Scraper ticker fired", r.Fqdn)
			now := time.Now()

			// Check if scraping should be slowed down
			shouldSlowDown := false
			value, exists := g.LastResolution.Load(r.Fqdn)
			if exists {
				lastResolution := value.(time.Time)
				if now.Sub(lastResolution) > g.GetResolutionIdleTimeout() {
					shouldSlowDown = true
				}
			}

			// Adjust the scraping interval based on activity
			newInterval := r.GetScrapeInterval()
			if shouldSlowDown {
				newInterval = r.GetScrapeInterval() * time.Duration(g.HealthcheckIdleMultiplier)
			}

			// If the interval changes, reset the ticker
			if newInterval != scrapeInterval {
				scrapeInterval = newInterval
				r.ticker.Reset(scrapeInterval)
				if shouldSlowDown {
					log.Infof("[%s] Slow down scrape interval to %s", r.Fqdn, scrapeInterval)
				} else {
					log.Infof("[%s] Resume normal scrape interval to %s", r.Fqdn, scrapeInterval)
				}
			}

			// Run service discovery if configured
			if r.Discovery != nil {
				log.Debugf("[%s] Fetching endpoints from discovery type: %s", r.Fqdn, r.Discovery.Type)
				endpoints, err := r.Discovery.FetchEndpoints()
				if err == nil {
					log.Infof("[%s] Discovered endpoints: %+v", r.Fqdn, endpoints)
					r.updateBackendsFromDiscovery(endpoints)
				} else {
					log.Errorf("[%s] service discovery failed: %v", r.Fqdn, err)
				}
			}

			// Run health checks for backends
			r.mutex.RLock()
			backendsCopy := make([]BackendInterface, len(r.Backends))
			copy(backendsCopy, r.Backends)
			r.mutex.RUnlock()

			for _, backend := range backendsCopy {
				backend.Lock()
				if !backend.IsEnabled() {
					backend.Unlock()
					continue
				}
				backend.Unlock()
				backend.runHealthChecks(r.ScrapeRetries, r.GetScrapeTimeout())
			}

			// Update Prometheus gauge for active backends
			healthyCount := 0
			for _, backend := range backendsCopy {
				if backend.IsHealthy() {
					healthyCount++
				}
			}
			SetActiveBackends(r.Fqdn, float64(healthyCount))

			// Update record health status
			r.updateRecordHealthStatus()
		case <-ctx.Done():
			log.Debugf("[%s] stopping health checks", r.Fqdn)
			return
		}
	}
}

func parseDurationWithDefault(durationStr string, defaultStr string) time.Duration {
	d, err := time.ParseDuration(durationStr)
	if err != nil {
		d, _ = time.ParseDuration(defaultStr)
	}
	return d
}

func (r *Record) UpdateRecord() {
	// Update record health status
	r.updateRecordHealthStatus()
}

func (r *Record) updateRecordHealthStatus() {
	r.mutex.RLock()
	backendsCopy := make([]BackendInterface, len(r.Backends))
	copy(backendsCopy, r.Backends)
	r.mutex.RUnlock()

	// Check if any backend is healthy
	hasHealthyBackend := false
	for _, backend := range backendsCopy {
		if backend.IsHealthy() {
			hasHealthyBackend = true
			break
		}
	}

	// Set health status: 1 if any backend is healthy, 0 otherwise
	if hasHealthyBackend {
		SetRecordHealthStatus(r.Fqdn, 1)
	} else {
		SetRecordHealthStatus(r.Fqdn, 0)
	}

	// Update individual backend health status
	for _, backend := range backendsCopy {
		switch {
		case !backend.IsEnabled():
			SetBackendHealthStatus(r.Fqdn, backend.GetAddress(), 2)
		case backend.IsHealthy():
			SetBackendHealthStatus(r.Fqdn, backend.GetAddress(), 1)
		default:
			SetBackendHealthStatus(r.Fqdn, backend.GetAddress(), 0)
		}

		// Update healthcheck status for each type
		for _, healthcheck := range backend.GetHealthChecks() {
			healthcheckType := healthcheck.GetType()
			if backend.IsHealthy() {
				SetBackendHealthcheckStatus(r.Fqdn, backend.GetAddress(), healthcheckType, 1)
			} else {
				SetBackendHealthcheckStatus(r.Fqdn, backend.GetAddress(), healthcheckType, 0)
			}
		}
	}
}

func (r *Record) updateBackendsFromDiscovery(discovered []DiscoveredEndpoint) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var newBackends []BackendInterface
	for _, dep := range discovered {
		var found BackendInterface
		for _, b := range r.Backends {
			if b.GetAddress() == dep.Address && b.GetPort() == dep.Port {
				found = b
				break
			}
		}

		if found != nil {
			newBackends = append(newBackends, found)
		} else {
			b := &Backend{
				Fqdn:          r.Fqdn,
				Address:       dep.Address,
				Port:          dep.Port,
				Enable:        true,
				Weight:        1,
				Priority:      0,
				AssumeHealthy: false,
			}
			newBackends = append(newBackends, b)
		}
	}
	r.Backends = newBackends
}
