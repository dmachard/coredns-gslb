package gslb

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v3"
)

func TestBackend_UnmarshalYAML(t *testing.T) {
	yamlData := `
address: "127.0.0.1"
port: 8080
priority: 10
description: "helloworld"
continent: "EU"
country: "FR"
subdivision: "IDF"
city: "Paris"
asn: "64500"
location: "edge-eu"
longitude: 2.3522
latitude: 48.8566
enable: true
timeout: "10s"
healthchecks:
  - type: "http"
    params:
      uri: "/health"
`

	var backend Backend
	err := yaml.Unmarshal([]byte(yamlData), &backend)
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1", backend.Address)
	assert.Equal(t, 8080, backend.Port)
	assert.Equal(t, 10, backend.Priority)
	assert.Equal(t, true, backend.Enable)
	assert.Equal(t, "10s", backend.Timeout)
	assert.Equal(t, "helloworld", backend.Description)
	assert.Equal(t, "EU", backend.Continent)
	assert.Equal(t, "FR", backend.Country)
	assert.Equal(t, "IDF", backend.Subdivision)
	assert.Equal(t, "Paris", backend.City)
	assert.Equal(t, "64500", backend.ASN)
	assert.Equal(t, "edge-eu", backend.Location)
	assert.Equal(t, 2.3522, backend.Longitude)
	assert.Equal(t, 48.8566, backend.Latitude)
	assert.True(t, backend.HasCoordinates)
	assert.Len(t, backend.HealthChecks, 1)
	assert.IsType(t, &HTTPHealthCheck{}, backend.HealthChecks[0])
}

func TestBackend_RunHealthChecks(t *testing.T) {
	// Create a backend with a mocked health check
	backend := &Backend{
		Address: "127.0.0.1",
		HealthChecks: []GenericHealthCheck{
			&MockHealthCheck{},
		},
	}

	// Run the health checks (mocked to always return true)
	backend.runHealthChecks(3, 5*time.Second)

	// Assert that the backend's Alive status is true (since the mock always returns true)
	assert.True(t, backend.Alive)
}

func TestBackend_Getters(t *testing.T) {
	b := &Backend{
		Fqdn:           "test.example.com.",
		Description:    "desc",
		Address:        "1.2.3.4",
		Port:           8080,
		Priority:       10,
		Enable:         true,
		HealthChecks:   []GenericHealthCheck{},
		Timeout:        "5s",
		Continent:      "EU",
		Country:        "FR",
		Subdivision:    "IDF",
		Location:       "eu-west-1",
		Longitude:      2.3522,
		Latitude:       48.8566,
		HasCoordinates: true,
	}

	assert.Equal(t, "test.example.com.", b.GetFqdn())
	assert.Equal(t, "desc", b.GetDescription())
	assert.Equal(t, "1.2.3.4", b.GetAddress())
	assert.Equal(t, 8080, b.GetPort())
	assert.Equal(t, 10, b.GetPriority())
	assert.Equal(t, true, b.IsEnabled())
	assert.Equal(t, []GenericHealthCheck{}, b.GetHealthChecks())
	assert.Equal(t, "5s", b.GetTimeout())
	assert.Equal(t, "EU", b.GetContinent())
	assert.Equal(t, "FR", b.GetCountry())
	assert.Equal(t, "IDF", b.GetSubdivision())
	assert.Equal(t, "eu-west-1", b.GetLocation())
	assert.Equal(t, 2.3522, b.GetLongitude())
	assert.Equal(t, 48.8566, b.GetLatitude())
	assert.True(t, b.HasGeoCoordinates())
	assert.Equal(t, "FR", b.GetCountry())
	assert.Equal(t, "eu-west-1", b.GetLocation())

	now := time.Now()
	b.SetLastHealthcheck(now)
	assert.Equal(t, now, b.GetLastHealthcheck())
}

func TestBackend_IsHealthy(t *testing.T) {
	// Test backend enabled and alive
	b1 := &Backend{
		Enable: true,
		Alive:  true,
	}
	assert.True(t, b1.IsHealthy())

	// Test backend enabled but not alive
	b2 := &Backend{
		Enable: true,
		Alive:  false,
	}
	assert.False(t, b2.IsHealthy())

	// Test backend disabled but alive
	b3 := &Backend{
		Enable: false,
		Alive:  true,
	}
	assert.False(t, b3.IsHealthy())

	// Test backend disabled and not alive
	b4 := &Backend{
		Enable: false,
		Alive:  false,
	}
	assert.False(t, b4.IsHealthy())
}

func TestBackend_AssumeHealthy_Unmarshal(t *testing.T) {
	configYAML := `
address: "1.2.3.4"
enable: true
assume_healthy: true
`
	var b Backend
	err := yaml.Unmarshal([]byte(configYAML), &b)
	assert.NoError(t, err)
	assert.True(t, b.AssumeHealthy)
	assert.True(t, b.GetAssumeHealthy())

	// Test default false
	configYAMLDefault := `
address: "1.2.3.4"
enable: true
`
	var bDefault Backend
	err = yaml.Unmarshal([]byte(configYAMLDefault), &bDefault)
	assert.NoError(t, err)
	assert.False(t, bDefault.AssumeHealthy)
	assert.False(t, bDefault.GetAssumeHealthy())
}

func TestBackend_IsHealthy_AssumeHealthy(t *testing.T) {
	// Enabled, not alive, but assume healthy is true
	b1 := &Backend{
		Enable:        true,
		Alive:         false,
		AssumeHealthy: true,
	}
	assert.True(t, b1.IsHealthy())

	// Disabled, not alive, but assume healthy is true -> should be false
	b2 := &Backend{
		Enable:        false,
		Alive:         false,
		AssumeHealthy: true,
	}
	assert.False(t, b2.IsHealthy())
}

func TestBackend_RunHealthChecks_AssumeHealthy(t *testing.T) {
	// Create a backend with assume_healthy: true and an invalid health check
	// If assume_healthy is respected, the check should not run and the status should be set to alive
	b := &Backend{
		Address:       "invalid-address",
		Enable:        true,
		AssumeHealthy: true,
		HealthChecks: []GenericHealthCheck{
			&MySQLHealthCheck{
				Host: "127.0.0.1",
				Port: 3306,
			},
		},
	}

	assert.False(t, b.Alive)
	b.runHealthChecks(1, 1*time.Second)
	assert.True(t, b.Alive)
}

// Mock Backend and Record
// For testing purpopose
type MockBackend struct {
	mock.Mock
	*Backend
}

func (m *MockBackend) IsHealthy() bool {
	args := m.Called()
	return args.Bool(0)
}

//nolint:staticcheck
func TestBackend_LockUnlock(t *testing.T) {
	b := &Backend{
		Address: "1.2.3.4",
		Enable:  true,
	}

	// Test that Lock/Unlock don't panic
	assert.NotPanics(t, func() {
		b.Lock()
		b.Unlock()
	})

	// Test concurrent access
	done := make(chan bool)
	go func() {
		b.Lock()
		b.Address = "5.6.7.8"
		b.Unlock()
		done <- true
	}()

	b.Lock()
	b.Enable = false
	b.Unlock() //nolint:staticcheck

	<-done
}

func TestBackend_UpdateBackend(t *testing.T) {
	b := &Backend{
		Address:        "1.2.3.4",
		Port:           8080,
		Priority:       10,
		Weight:         5,
		Enable:         true,
		Description:    "old description",
		Tags:           []string{"tag1", "tag2"},
		Timeout:        "5s",
		Continent:      "NA",
		Country:        "US",
		Subdivision:    "NY",
		City:           "New York",
		ASN:            "64512",
		Location:       "us-east",
		Longitude:      -74.0060,
		Latitude:       40.7128,
		HasCoordinates: true,
		HealthChecks: []GenericHealthCheck{
			&MockHealthCheck{},
		},
	}

	newBackend := &Backend{
		Address:        "1.2.3.4", // Same address
		Port:           9090,      // Different port
		Priority:       20,        // Different priority
		Weight:         10,        // Different weight
		Enable:         false,     // Different enable state
		Description:    "new description",
		Tags:           []string{"tag3", "tag4", "tag5"},
		Timeout:        "10s",
		Continent:      "EU",
		Country:        "FR",
		Subdivision:    "IDF",
		City:           "Paris",
		ASN:            "64513",
		Location:       "eu-west",
		Longitude:      2.3522,
		Latitude:       48.8566,
		HasCoordinates: true,
		HealthChecks: []GenericHealthCheck{
			&MockHealthCheck{},
			&MockHealthCheck{},
		},
	}

	// Test that updateBackend doesn't panic
	assert.NotPanics(t, func() {
		b.updateBackend(newBackend)
	})

	// Verify all fields were updated
	assert.Equal(t, 9090, b.Port, "Port should be updated")
	assert.Equal(t, 20, b.Priority, "Priority should be updated")
	assert.Equal(t, 10, b.Weight, "Weight should be updated")
	assert.Equal(t, false, b.Enable, "Enable should be updated")
	assert.Equal(t, "new description", b.Description, "Description should be updated")
	assert.Equal(t, []string{"tag3", "tag4", "tag5"}, b.Tags, "Tags should be updated")
	assert.Equal(t, "10s", b.Timeout, "Timeout should be updated")
	assert.Equal(t, "EU", b.Continent, "Continent should be updated")
	assert.Equal(t, "FR", b.Country, "Country should be updated")
	assert.Equal(t, "IDF", b.Subdivision, "Subdivision should be updated")
	assert.Equal(t, "Paris", b.City, "City should be updated")
	assert.Equal(t, "64513", b.ASN, "ASN should be updated")
	assert.Equal(t, "eu-west", b.Location, "Location should be updated")
	assert.Equal(t, 2.3522, b.Longitude, "Longitude should be updated")
	assert.Equal(t, 48.8566, b.Latitude, "Latitude should be updated")
	assert.True(t, b.HasCoordinates, "HasCoordinates should be updated")
	assert.Len(t, b.HealthChecks, 2, "HealthChecks should be updated")
}

func TestBackend_UpdateBackend_NoChanges(t *testing.T) {
	// Test that when fields are the same, no update occurs
	b := &Backend{
		Address:     "1.2.3.4",
		Priority:    10,
		Weight:      5,
		Enable:      true,
		Description: "same description",
		Tags:        []string{"tag1"},
		Timeout:     "5s",
		Country:     "US",
	}

	newBackend := &Backend{
		Address:     "1.2.3.4",
		Priority:    10,
		Weight:      5,
		Enable:      true,
		Description: "same description",
		Tags:        []string{"tag1"},
		Timeout:     "5s",
		Country:     "US",
	}

	// Should not panic when fields are the same
	assert.NotPanics(t, func() {
		b.updateBackend(newBackend)
	})

	// Verify fields remain the same
	assert.Equal(t, 10, b.Priority)
	assert.Equal(t, 5, b.Weight)
	assert.Equal(t, true, b.Enable)
	assert.Equal(t, "same description", b.Description)
}

func TestBackend_RemoveBackend(t *testing.T) {
	b := &Backend{
		Address: "1.2.3.4",
		Enable:  true,
	}

	// Test that removeBackend doesn't panic
	assert.NotPanics(t, func() {
		b.removeBackend()
	})
}

func TestBackend_RecomputeCoordinateRadians(t *testing.T) {
	b := &Backend{
		Longitude:      -74.0060,
		Latitude:       40.7128,
		HasCoordinates: true,
	}
	b.recomputeCoordinateRadians()

	assert.InDelta(t, -74.0060*math.Pi/180, b.LongitudeRad, 1e-9)
	assert.InDelta(t, 40.7128*math.Pi/180, b.LatitudeRad, 1e-9)

	// Test case where HasCoordinates is false
	b2 := &Backend{
		Longitude:      -74.0060,
		Latitude:       40.7128,
		HasCoordinates: false,
	}
	b2.recomputeCoordinateRadians()
	assert.Equal(t, 0.0, b2.LongitudeRad)
	assert.Equal(t, 0.0, b2.LatitudeRad)
}

func TestBackend_RiseFallThresholds(t *testing.T) {
	// 1. Test Rise threshold: needs 3 successes to go from DOWN to UP
	b := &Backend{
		Address: "127.0.0.1",
		Rise:    3,
		Fall:    2,
		Alive:   false, // starts DOWN
		HealthChecks: []GenericHealthCheck{
			&MockHealthCheck{}, // always returns true
		},
	}

	// Run 1: success, but consecutiveSuccesses is 1 < 3. Should remain DOWN.
	b.runHealthChecks(1, 1*time.Second)
	assert.False(t, b.Alive)

	// Run 2: success, consecutiveSuccesses is 2 < 3. Should remain DOWN.
	b.runHealthChecks(1, 1*time.Second)
	assert.False(t, b.Alive)

	// Run 3: success, consecutiveSuccesses is 3 >= 3. Should transition to UP.
	b.runHealthChecks(1, 1*time.Second)
	assert.True(t, b.Alive)

	// 2. Test Fall threshold: needs 2 failures to go from UP to DOWN
	failingCheck := &MockFailingHealthCheck{}
	b.HealthChecks = []GenericHealthCheck{failingCheck}

	// Run 1: failure, consecutiveFailures is 1 < 2. Should remain UP.
	b.runHealthChecks(1, 1*time.Second)
	assert.True(t, b.Alive)

	// Run 2: failure, consecutiveFailures is 2 >= 2. Should transition to DOWN.
	b.runHealthChecks(1, 1*time.Second)
	assert.False(t, b.Alive)
}

type MockFailingHealthCheck struct{}

func (hc *MockFailingHealthCheck) PerformCheck(backend *Backend, fqdn string, maxRetries int) bool {
	return false
}
func (hc *MockFailingHealthCheck) GetType() string {
	return "mock_fail"
}
func (hc *MockFailingHealthCheck) Equals(other GenericHealthCheck) bool {
	_, ok := other.(*MockFailingHealthCheck)
	return ok
}

func TestBackend_ApplyHealthCheckResult(t *testing.T) {
	b := &Backend{
		Address: "127.0.0.1",
		Rise:    3,
		Fall:    2,
		Alive:   false, // starts DOWN
	}

	// 1. Successes transition from DOWN to UP
	b.ApplyHealthCheckResult(true)
	assert.False(t, b.Alive)
	b.ApplyHealthCheckResult(true)
	assert.False(t, b.Alive)
	b.ApplyHealthCheckResult(true)
	assert.True(t, b.Alive)

	// 2. Failures transition from UP to DOWN
	b.ApplyHealthCheckResult(false)
	assert.True(t, b.Alive)
	b.ApplyHealthCheckResult(false)
	assert.False(t, b.Alive)
}
