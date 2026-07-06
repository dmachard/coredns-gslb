package gslb

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/creasty/defaults"
)

// Backend represents an individual backend with health check settings.
type Backend struct {
	Fqdn                 string               // Fully qualified domain name
	Description          string               // Description of the backend
	Address              string               // IP address or hostname
	Port                 int                  // Port number of the backend
	Priority             int                  // Priority for load balancing
	Weight               int                  // Weight for weighted load balancing
	Enable               bool                 // Enable or disable the backend
	Tags                 []string             // List of tags for filtering or grouping
	HealthChecks         []GenericHealthCheck `yaml:"healthchecks"` // Health check configurations
	Timeout              string               // Timeout for requests
	Alive                bool                 // Indicates if the backend is alive
	Continent            string               // Continent code for GeoIP (e.g. EU)
	Country              string               // Country code for GeoIP
	Subdivision          string               // Subdivision/state code for GeoIP (e.g. CA, NY)
	City                 string               // City name for GeoIP
	ASN                  string               // ASN for GeoIP
	Location             string               // location
	Longitude            float64              // Longitude for distance-based GeoIP routing
	Latitude             float64              // Latitude for distance-based GeoIP routing
	LongitudeRad         float64              // Precomputed longitude in radians for distance calculations
	LatitudeRad          float64              // Precomputed latitude in radians for distance calculations
	HasCoordinates       bool                 // Indicates whether both coordinates were explicitly configured
	LastHealthcheck      time.Time            // Last time a healthcheck was launched
	AssumeHealthy        bool                 `yaml:"assume_healthy"` // Bypass healthchecks and assume UP
	Passive              bool                 `yaml:"passive"`        // Passive health checking, read from Redis only
	Rise                 int                  `yaml:"rise" default:"2"`
	Fall                 int                  `yaml:"fall" default:"3"`
	consecutiveSuccesses int
	consecutiveFailures  int
	mutex                sync.RWMutex
}

func (b *Backend) Lock() {
	b.mutex.Lock()
}

func (b *Backend) Unlock() {
	b.mutex.Unlock()
}

func (b *Backend) GetFqdn() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Fqdn
}

func (b *Backend) SetFqdn(fqdn string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.Fqdn = fqdn
}

func (b *Backend) GetDescription() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Description
}

func (b *Backend) GetAddress() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Address
}

func (b *Backend) GetPort() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Port
}

func (b *Backend) GetPriority() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Priority
}

func (b *Backend) GetWeight() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	if b.Weight <= 0 {
		return 1
	}
	return b.Weight
}

func (b *Backend) IsEnabled() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Enable
}

func (b *Backend) GetAssumeHealthy() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.AssumeHealthy
}

func (b *Backend) IsPassive() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Passive
}

func (b *Backend) GetRise() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Rise
}

func (b *Backend) GetFall() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Fall
}

func (b *Backend) GetTags() []string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Tags
}

func (b *Backend) GetHealthChecks() []GenericHealthCheck {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.HealthChecks
}

func (b *Backend) GetTimeout() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Timeout
}

func (b *Backend) GetContinent() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Continent
}

func (b *Backend) GetCountry() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Country
}

func (b *Backend) GetSubdivision() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Subdivision
}

func (b *Backend) GetCity() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.City
}

func (b *Backend) GetASN() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.ASN
}

func (b *Backend) GetLocation() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Location
}

func (b *Backend) GetLongitude() float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Longitude
}

func (b *Backend) GetLatitude() float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Latitude
}

func (b *Backend) GetLongitudeRad() float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.LongitudeRad
}

func (b *Backend) GetLatitudeRad() float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.LatitudeRad
}

func (b *Backend) HasGeoCoordinates() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.HasCoordinates
}

func (b *Backend) recomputeCoordinateRadians() {
	if !b.HasCoordinates {
		b.LongitudeRad = 0
		b.LatitudeRad = 0
		return
	}
	b.LongitudeRad = b.Longitude * math.Pi / 180
	b.LatitudeRad = b.Latitude * math.Pi / 180
}

func (b *Backend) IsAlive() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Alive
}

func (b *Backend) SetAlive(alive bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.Alive = alive
}

func (b *Backend) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw struct {
		Description   string        `yaml:"description" default:""`
		Address       string        `yaml:"address" default:"127.0.0.1"`
		Port          int           `yaml:"port" default:"0"`
		Priority      int           `yaml:"priority" default:"0"`
		Weight        int           `yaml:"weight" default:"1"`
		Enable        bool          `yaml:"enable" default:"true"`
		Tags          []string      `yaml:"tags"`
		Timeout       string        `yaml:"timeout" default:"5s"`
		HealthChecks  []HealthCheck `yaml:"healthchecks"`
		Continent     string        `yaml:"continent"`
		Country       string        `yaml:"country"`
		Subdivision   string        `yaml:"subdivision"`
		City          string        `yaml:"city"`
		ASN           string        `yaml:"asn"`
		Location      string        `yaml:"location"`
		Longitude     *float64      `yaml:"longitude"`
		Latitude      *float64      `yaml:"latitude"`
		AssumeHealthy bool          `yaml:"assume_healthy" default:"false"`
		Passive       bool          `yaml:"passive" default:"false"`
		Rise          int           `yaml:"rise" default:"2"`
		Fall          int           `yaml:"fall" default:"3"`
	}
	defaults.Set(&raw)
	if err := unmarshal(&raw); err != nil {
		return err
	}
	b.Description = raw.Description
	b.Address = raw.Address
	b.Port = raw.Port
	b.Priority = raw.Priority
	b.Weight = raw.Weight
	b.Enable = raw.Enable
	b.Tags = raw.Tags
	b.Timeout = raw.Timeout
	b.Continent = raw.Continent
	b.Country = raw.Country
	b.Subdivision = raw.Subdivision
	b.City = raw.City
	b.ASN = raw.ASN
	b.Location = raw.Location
	b.AssumeHealthy = raw.AssumeHealthy
	b.Passive = raw.Passive
	b.Rise = raw.Rise
	b.Fall = raw.Fall
	longitudeSet := false
	latitudeSet := false
	if raw.Longitude != nil {
		b.Longitude = *raw.Longitude
		longitudeSet = true
	}
	if raw.Latitude != nil {
		b.Latitude = *raw.Latitude
		latitudeSet = true
	}
	b.HasCoordinates = longitudeSet && latitudeSet
	b.recomputeCoordinateRadians()
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
	log.Infof("[%s] backend %s successfully removed", b.Fqdn, b.Address)
}

// updateBackend updates the settings of an existing backend
func (b *Backend) updateBackend(newBackend BackendInterface) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.Port != newBackend.GetPort() {
		log.Infof("[%s] backend %s updated, port changed from %d to %d", b.Fqdn, b.Address, b.Port, newBackend.GetPort())
		b.Port = newBackend.GetPort()
	}

	if b.Priority != newBackend.GetPriority() {
		log.Infof("[%s] backend %s updated, priority changed from %d to %d", b.Fqdn, b.Address, b.Priority, newBackend.GetPriority())
		b.Priority = newBackend.GetPriority()
	}

	if b.Weight != newBackend.GetWeight() {
		log.Infof("[%s] backend %s updated, weight changed from %d to %d", b.Fqdn, b.Address, b.Weight, newBackend.GetWeight())
		b.Weight = newBackend.GetWeight()
	}

	if b.Enable != newBackend.IsEnabled() {
		log.Infof("[%s] backend %s updated, enable changed from %v to %v", b.Fqdn, b.Address, b.Enable, newBackend.IsEnabled())
		b.Enable = newBackend.IsEnabled()
	}

	if b.AssumeHealthy != newBackend.GetAssumeHealthy() {
		log.Infof("[%s] backend %s updated, assume_healthy changed from %v to %v", b.Fqdn, b.Address, b.AssumeHealthy, newBackend.GetAssumeHealthy())
		b.AssumeHealthy = newBackend.GetAssumeHealthy()
	}

	if b.Rise != newBackend.GetRise() {
		log.Infof("[%s] backend %s updated, rise changed from %d to %d", b.Fqdn, b.Address, b.Rise, newBackend.GetRise())
		b.Rise = newBackend.GetRise()
	}

	if b.Fall != newBackend.GetFall() {
		log.Infof("[%s] backend %s updated, fall changed from %d to %d", b.Fqdn, b.Address, b.Fall, newBackend.GetFall())
		b.Fall = newBackend.GetFall()
	}

	if b.Description != newBackend.GetDescription() {
		log.Infof("[%s] backend %s updated, description changed", b.Fqdn, b.Address)
		b.Description = newBackend.GetDescription()
	}

	if b.Timeout != newBackend.GetTimeout() {
		log.Infof("[%s] backend %s updated, timeout changed from %s to %s", b.Fqdn, b.Address, b.Timeout, newBackend.GetTimeout())
		b.Timeout = newBackend.GetTimeout()
	}

	if b.Continent != newBackend.GetContinent() {
		log.Infof("[%s] backend %s updated, continent changed from %s to %s", b.Fqdn, b.Address, b.Continent, newBackend.GetContinent())
		b.Continent = newBackend.GetContinent()
	}

	if b.Country != newBackend.GetCountry() {
		log.Infof("[%s] backend %s updated, country changed from %s to %s", b.Fqdn, b.Address, b.Country, newBackend.GetCountry())
		b.Country = newBackend.GetCountry()
	}

	if b.Subdivision != newBackend.GetSubdivision() {
		log.Infof("[%s] backend %s updated, subdivision changed from %s to %s", b.Fqdn, b.Address, b.Subdivision, newBackend.GetSubdivision())
		b.Subdivision = newBackend.GetSubdivision()
	}

	if b.City != newBackend.GetCity() {
		log.Infof("[%s] backend %s updated, city changed from %s to %s", b.Fqdn, b.Address, b.City, newBackend.GetCity())
		b.City = newBackend.GetCity()
	}

	if b.ASN != newBackend.GetASN() {
		log.Infof("[%s] backend %s updated, asn changed from %s to %s", b.Fqdn, b.Address, b.ASN, newBackend.GetASN())
		b.ASN = newBackend.GetASN()
	}

	if b.Location != newBackend.GetLocation() {
		log.Infof("[%s] backend %s updated, location changed from %s to %s", b.Fqdn, b.Address, b.Location, newBackend.GetLocation())
		b.Location = newBackend.GetLocation()
	}

	if b.Longitude != newBackend.GetLongitude() {
		log.Infof("[%s] backend %s updated, longitude changed from %f to %f", b.Fqdn, b.Address, b.Longitude, newBackend.GetLongitude())
		b.Longitude = newBackend.GetLongitude()
	}

	if b.Latitude != newBackend.GetLatitude() {
		log.Infof("[%s] backend %s updated, latitude changed from %f to %f", b.Fqdn, b.Address, b.Latitude, newBackend.GetLatitude())
		b.Latitude = newBackend.GetLatitude()
	}

	if b.HasCoordinates != newBackend.HasGeoCoordinates() {
		log.Infof("[%s] backend %s updated, coordinate availability changed from %v to %v", b.Fqdn, b.Address, b.HasCoordinates, newBackend.HasGeoCoordinates())
		b.HasCoordinates = newBackend.HasGeoCoordinates()
	}

	b.recomputeCoordinateRadians()

	// Compare tags slice
	if !tagsEqual(b.Tags, newBackend.GetTags()) {
		log.Infof("[%s] backend %s updated, tags changed", b.Fqdn, b.Address)
		b.Tags = newBackend.GetTags()
	}

	// Check if health checks have changed
	if !healthChecksEqual(b.HealthChecks, newBackend.GetHealthChecks()) {
		log.Infof("[%s] backend %s health checks have changed.", b.Fqdn, b.Address)
		b.HealthChecks = newBackend.GetHealthChecks()
	}
}

func (b *Backend) runHealthChecks(maxRetries int, scrapeTimeout time.Duration) bool {
	b.mutex.Lock()
	b.LastHealthcheck = time.Now()
	b.mutex.Unlock()

	b.mutex.RLock()
	assumeHealthy := b.AssumeHealthy
	b.mutex.RUnlock()

	if assumeHealthy {
		b.mutex.Lock()
		b.Alive = true
		b.mutex.Unlock()
		log.Debugf("[%s] health check bypassed for backend: %s (assume_healthy is true)", b.Fqdn, b.Address)
		return true
	}

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

	b.ApplyHealthCheckResult(alive)

	// Keep old log format for log parsing
	log.Debugf("[%s] backend status [address=%s]: healthchecks=%s alive=%v", b.Fqdn, b.Address, healthChecksList, b.IsAlive())
	return alive
}

func (b *Backend) ApplyHealthCheckResult(alive bool) {
	b.mutex.Lock()
	oldAlive := b.Alive
	rise := b.Rise
	if rise <= 0 {
		rise = 1
	}
	fallThreshold := b.Fall
	if fallThreshold <= 0 {
		fallThreshold = 1
	}

	if alive {
		b.consecutiveSuccesses++
		b.consecutiveFailures = 0
		if !b.Alive && b.consecutiveSuccesses >= rise {
			b.Alive = true
		}
	} else {
		b.consecutiveFailures++
		b.consecutiveSuccesses = 0
		if b.Alive && b.consecutiveFailures >= fallThreshold {
			b.Alive = false
		}
	}

	// Log backend health changes with higher log level
	if b.Alive != oldAlive {
		log.Infof("[%s] backend status change [address=%s]: alive changed from %v to %v", b.Fqdn, b.Address, oldAlive, b.Alive)
	}
	b.mutex.Unlock()
}

func (b *Backend) IsHealthy() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return (b.Alive || b.AssumeHealthy) && b.Enable
}

// tagsEqual compares two slices of strings (tags) for equality.
func tagsEqual(t1, t2 []string) bool {
	if len(t1) != len(t2) {
		return false
	}
	for i, tag := range t1 {
		if tag != t2[i] {
			return false
		}
	}
	return true
}

type BackendInterface interface {
	GetFqdn() string
	SetFqdn(fqdn string)
	GetDescription() string
	GetAddress() string
	GetPort() int
	GetPriority() int
	GetWeight() int
	IsEnabled() bool
	GetAssumeHealthy() bool
	IsPassive() bool
	GetRise() int
	GetFall() int
	GetTags() []string
	GetHealthChecks() []GenericHealthCheck
	GetTimeout() string
	GetContinent() string
	GetCountry() string
	GetSubdivision() string
	GetCity() string
	GetASN() string
	GetLocation() string
	GetLongitude() float64
	GetLatitude() float64
	GetLongitudeRad() float64
	GetLatitudeRad() float64
	HasGeoCoordinates() bool
	IsHealthy() bool
	IsAlive() bool
	SetAlive(alive bool)
	runHealthChecks(retries int, timeout time.Duration) bool
	ApplyHealthCheckResult(alive bool)
	removeBackend()
	updateBackend(newBackend BackendInterface)
	Lock()
	Unlock()
}
