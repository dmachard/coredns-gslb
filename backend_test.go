package gslb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v3"
)

func TestBackend_UnmarshalYAML(t *testing.T) {
	yamlData := `
address: "127.0.0.1"
priority: 10
description: "helloworld"
location_countries: ["FR", "DE"]
location_cities: ["Paris", "Berlin"]
location_asns: [64500, 64501]
locations_custom: ["edge-eu", "edge-de"]
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
	assert.Equal(t, 10, backend.Priority)
	assert.Equal(t, true, backend.Enable)
	assert.Equal(t, "10s", backend.Timeout)
	assert.Equal(t, "helloworld", backend.Description)
	assert.ElementsMatch(t, []string{"FR", "DE"}, backend.Countries)
	assert.ElementsMatch(t, []string{"Paris", "Berlin"}, backend.Cities)
	assert.ElementsMatch(t, []uint{64500, 64501}, backend.ASNs)
	assert.ElementsMatch(t, []string{"edge-eu", "edge-de"}, backend.CustomLocations)
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
		Fqdn:            "test.example.com.",
		Description:     "desc",
		Address:         "1.2.3.4",
		Priority:        10,
		Enable:          true,
		HealthChecks:    []GenericHealthCheck{},
		Timeout:         "5s",
		Countries:       []string{"FR"},
		CustomLocations: []string{"eu-west-1"},
	}

	assert.Equal(t, "test.example.com.", b.GetFqdn())
	assert.Equal(t, "desc", b.GetDescription())
	assert.Equal(t, "1.2.3.4", b.GetAddress())
	assert.Equal(t, 10, b.GetPriority())
	assert.Equal(t, true, b.IsEnabled())
	assert.Equal(t, []GenericHealthCheck{}, b.GetHealthChecks())
	assert.Equal(t, "5s", b.GetTimeout())
	assert.Equal(t, []string{"FR"}, b.GetCountries())
	assert.Equal(t, []string{"eu-west-1"}, b.GetCustomLocations())
	assert.Equal(t, "FR", b.GetCountry())
	assert.Equal(t, "eu-west-1", b.GetLocation())
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
		Address:  "1.2.3.4",
		Priority: 10,
		Enable:   true,
	}

	newBackend := &Backend{
		Address:  "1.2.3.4", // Same address
		Priority: 20,        // Different priority
		Enable:   false,     // Different enable state
	}

	// Test that updateBackend doesn't panic
	assert.NotPanics(t, func() {
		b.updateBackend(newBackend)
	})

	// Verify the update worked
	assert.Equal(t, 20, b.Priority)
	assert.Equal(t, false, b.Enable)
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
