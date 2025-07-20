package gslb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/creasty/defaults"
)

// Backend represents an individual backend with health check settings.
type Backend struct {
	Fqdn            string               // Fully qualified domain name
	Description     string               // Description of the backend
	Address         string               // IP address or hostname
	Priority        int                  // Priority for load balancing
	Enable          bool                 // Enable or disable the backend
	HealthChecks    []GenericHealthCheck // Health check configurations
	Timeout         string               // Timeout for requests
	Alive           bool                 // Indicates if the backend is alive
	Countries       []string             // List of country codes for GeoIP
	Cities          []string             // List of city names for GeoIP
	ASNs            []uint               // List of ASNs for GeoIP
	CustomLocations []string             // List of custom location strings
	mutex           sync.RWMutex
}

func (b *Backend) Lock() {
	b.mutex.Lock()
}

func (b *Backend) Unlock() {
	b.mutex.Unlock()
}

func (b *Backend) GetFqdn() string {
	return b.Fqdn
}

func (b *Backend) SetFqdn(fqdn string) {
	b.Fqdn = fqdn
}

func (b *Backend) GetDescription() string {
	return b.Description
}

func (b *Backend) GetAddress() string {
	return b.Address
}

func (b *Backend) GetPriority() int {
	return b.Priority
}

func (b *Backend) IsEnabled() bool {
	return b.Enable
}

func (b *Backend) GetHealthChecks() []GenericHealthCheck {
	return b.HealthChecks
}

func (b *Backend) GetTimeout() string {
	return b.Timeout
}

func (b *Backend) GetCountries() []string {
	return b.Countries
}

func (b *Backend) GetCities() []string {
	return b.Cities
}

func (b *Backend) GetASNs() []uint {
	return b.ASNs
}

func (b *Backend) GetCustomLocations() []string {
	return b.CustomLocations
}

func (b *Backend) GetCountry() string {
	if len(b.Countries) > 0 {
		return b.Countries[0]
	}
	return ""
}

func (b *Backend) GetLocation() string {
	if len(b.CustomLocations) > 0 {
		return b.CustomLocations[0]
	}
	return ""
}

func (b *Backend) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw struct {
		Description     string        `yaml:"description" default:""`
		Address         string        `yaml:"address" default:"127.0.0.1"`
		Priority        int           `yaml:"priority" default:"0"`
		Enable          bool          `yaml:"enable" default:"true"`
		Timeout         string        `yaml:"timeout" default:"5s"`
		HealthChecks    []HealthCheck `yaml:"healthchecks"`
		Countries       []string      `yaml:"location_countries"`
		Cities          []string      `yaml:"location_cities"`
		ASNs            []uint        `yaml:"location_asns"`
		CustomLocations []string      `yaml:"locations_custom"`
	}
	defaults.Set(&raw)
	if err := unmarshal(&raw); err != nil {
		return err
	}

	b.Address = raw.Address
	b.Priority = raw.Priority
	b.Enable = raw.Enable
	b.Timeout = raw.Timeout
	b.Description = raw.Description
	b.Countries = raw.Countries
	b.Cities = raw.Cities
	b.ASNs = raw.ASNs
	b.CustomLocations = raw.CustomLocations

	for _, hc := range raw.HealthChecks {
		specificHC, err := hc.ToSpecificHealthCheck()
		if err != nil {
			return fmt.Errorf("error converting healthcheck for backend %s: %w", b.Address, err)
		}
		b.HealthChecks = append(b.HealthChecks, specificHC)
	}

	return nil
}

// removeBackend stops the health check and performs cleanup for the backend
func (b *Backend) removeBackend() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	log.Debugf("[%s] backend %s successfully removed", b.Fqdn, b.Address)
}

// updateBackend updates the settings of an existing backend
func (b *Backend) updateBackend(newBackend BackendInterface) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.Priority != newBackend.GetPriority() {
		log.Debugf("[%s] backend %s updated, priority changed from %d to %d", b.Fqdn, b.Address, b.Priority, newBackend.GetPriority())
		b.Priority = newBackend.GetPriority()
	}

	if b.Enable != newBackend.IsEnabled() {
		log.Debugf("[%s] backend %s updated, enable changed from %v to %v", b.Fqdn, b.Address, b.Enable, newBackend.IsEnabled())
		b.Enable = newBackend.IsEnabled()
	}

	// Check if health checks have changed
	if !healthChecksEqual(b.HealthChecks, newBackend.GetHealthChecks()) {
		log.Debugf("[%s] backend %s health checks have changed.", b.Fqdn, b.Address)
		b.HealthChecks = newBackend.GetHealthChecks()
	}
}

func (b *Backend) runHealthChecks(maxRetries int, scrapeTimeout time.Duration) {
	var wg sync.WaitGroup
	results := make([]bool, len(b.HealthChecks))

	log.Debugf("[%s] starting health check for backend: %s", b.Fqdn, b.Address)

	// Gather the list of health check types
	var healthChecksList []string
	for _, healthCheck := range b.HealthChecks {
		healthChecksList = append(healthChecksList, healthCheck.GetType())
	}

	// Iterate over all health checks
	for i, hc := range b.HealthChecks {
		wg.Add(1) // Increment WaitGroup counter for each health check
		go func(i int, hc GenericHealthCheck) {
			defer wg.Done() // Decrement WaitGroup counter when the goroutine finishes

			// Create a context with timeout for the health check
			ctx, cancel := context.WithTimeout(context.Background(), scrapeTimeout)
			defer cancel()

			resultChan := make(chan bool, 1)

			// Goroutine to perform the health check
			go func() {
				resultChan <- hc.PerformCheck(b, b.Fqdn, maxRetries)
			}()

			// Wait for either the result or a timeout
			select {
			case results[i] = <-resultChan:
			case <-ctx.Done():
				log.Debugf("[%s] health check timed out for backend: %s, check: %s", b.Fqdn, b.Address, hc.GetType())
				results[i] = false
			}
		}(i, hc)
	}

	// Wait for all health check goroutines to complete before returning the results.
	wg.Wait()

	// Update the backend's Alive status
	alive := true
	for _, result := range results {
		if !result {
			alive = false
			break
		}
	}
	b.mutex.Lock()
	b.Alive = alive
	b.mutex.Unlock()

	log.Debugf("[%s] backend status [address=%s]: healthchecks=%s alive=%v", b.Fqdn, b.Address, healthChecksList, b.Alive)
}

func (b *Backend) IsHealthy() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.Alive && b.Enable
}

type BackendInterface interface {
	GetFqdn() string
	SetFqdn(fqdn string)
	GetDescription() string
	GetAddress() string
	GetPriority() int
	IsEnabled() bool
	GetHealthChecks() []GenericHealthCheck
	GetTimeout() string
	GetCountries() []string
	GetCities() []string
	GetASNs() []uint
	GetCustomLocations() []string
	IsHealthy() bool
	runHealthChecks(retries int, timeout time.Duration)
	removeBackend()
	updateBackend(newBackend BackendInterface)
	Lock()
	Unlock()
}
