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
