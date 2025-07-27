package gslb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/creasty/defaults"
	"gopkg.in/yaml.v3"
)

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
	ticker         *time.Ticker
	mutex          sync.RWMutex
	cancelFunc     context.CancelFunc
}

func (r *Record) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw struct {
		Mode           string        `yaml:"mode" default:"failover"`
		Owner          string        `yaml:"owner" default:""`
		Description    string        `yaml:"description" default:""`
		Ttl            int           `yaml:"record_ttl" default:"30"`
		ScrapeInterval string        `yaml:"scrape_interval" default:"10s"`
		ScrapeRetries  int           `yaml:"scrape_retries" default:"1"`
		ScrapeTimeout  string        `yaml:"scrape_timeout" default:"5s"`
		Backends       []interface{} `yaml:"backends"`
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
					log.Debugf("[%s] Slow down scrape interval to %s", r.Fqdn, scrapeInterval)
				} else {
					log.Debugf("[%s] Resume normal scrape interval to %s", r.Fqdn, scrapeInterval)
				}
			}

			// Run health checks for backends
			for _, backend := range r.Backends {
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
			for _, backend := range r.Backends {
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
	// Check if any backend is healthy
	hasHealthyBackend := false
	for _, backend := range r.Backends {
		if backend.IsHealthy() {
			hasHealthyBackend = true
			break
		}
	}

	// Set health status: 1 if any backend is healthy, 0 otherwise
	if hasHealthyBackend {
		SetRecordHealthStatus(r.Fqdn, "healthy", 1)
		SetRecordHealthStatus(r.Fqdn, "unhealthy", 0)
	} else {
		SetRecordHealthStatus(r.Fqdn, "healthy", 0)
		SetRecordHealthStatus(r.Fqdn, "unhealthy", 1)
	}

	// Update individual backend health status
	for _, backend := range r.Backends {
		switch {
		case !backend.IsEnabled():
			// Backend is disabled
			SetBackendHealthStatus(r.Fqdn, backend.GetAddress(), "healthy", 0)
			SetBackendHealthStatus(r.Fqdn, backend.GetAddress(), "unhealthy", 0)
			SetBackendHealthStatus(r.Fqdn, backend.GetAddress(), "disabled", 1)
		case backend.IsHealthy():
			// Backend is enabled and healthy
			SetBackendHealthStatus(r.Fqdn, backend.GetAddress(), "healthy", 1)
			SetBackendHealthStatus(r.Fqdn, backend.GetAddress(), "unhealthy", 0)
			SetBackendHealthStatus(r.Fqdn, backend.GetAddress(), "disabled", 0)
		default:
			// Backend is enabled but unhealthy
			SetBackendHealthStatus(r.Fqdn, backend.GetAddress(), "healthy", 0)
			SetBackendHealthStatus(r.Fqdn, backend.GetAddress(), "unhealthy", 1)
			SetBackendHealthStatus(r.Fqdn, backend.GetAddress(), "disabled", 0)
		}

		// Update healthcheck status for each type
		for _, healthcheck := range backend.GetHealthChecks() {
			healthcheckType := healthcheck.GetType()
			// For now, we'll set based on overall backend health
			// In a more detailed implementation, you could track individual healthcheck results
			switch {
			case !backend.IsEnabled():
				// Backend is disabled, no healthcheck status
				SetBackendHealthcheckStatus(r.Fqdn, backend.GetAddress(), healthcheckType, "success", 0)
				SetBackendHealthcheckStatus(r.Fqdn, backend.GetAddress(), healthcheckType, "fail", 0)
			case backend.IsHealthy():
				SetBackendHealthcheckStatus(r.Fqdn, backend.GetAddress(), healthcheckType, "success", 1)
				SetBackendHealthcheckStatus(r.Fqdn, backend.GetAddress(), healthcheckType, "fail", 0)
			default:
				SetBackendHealthcheckStatus(r.Fqdn, backend.GetAddress(), healthcheckType, "success", 0)
				SetBackendHealthcheckStatus(r.Fqdn, backend.GetAddress(), healthcheckType, "fail", 1)
			}
		}
	}
}
