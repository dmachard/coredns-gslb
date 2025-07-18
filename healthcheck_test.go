package gslb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test error handling for unsupported health check types
func TestToSpecificHealthCheck_Unsupported(t *testing.T) {
	// Test for a health check with an unknown type
	hc := &HealthCheck{
		Type:   "unsupported_type",
		Params: map[string]interface{}{},
	}

	// Ensure the conversion fails
	_, err := hc.ToSpecificHealthCheck()
	assert.Error(t, err)
	assert.Equal(t, "unsupported healthcheck type: unsupported_type", err.Error())
}

// Test that all known healthcheck types are handled in ToSpecificHealthCheck
func TestToSpecificHealthCheck_AllTypesHandled(t *testing.T) {
	types := []string{"http", "icmp", "tcp", "mysql", "grpc", "lua"}
	for _, typ := range types {
		hc := &HealthCheck{
			Type:   typ,
			Params: map[string]interface{}{},
		}
		_, err := hc.ToSpecificHealthCheck()
		assert.NoErrorf(t, err, "Type '%s' should be handled in ToSpecificHealthCheck", typ)
	}
}

func TestToSpecificHealthCheck_AllKnownTypes(t *testing.T) {
	types := []string{"http", "icmp", "tcp", "mysql", "grpc", "lua"}
	for _, typ := range types {
		hc := &HealthCheck{
			Type:   typ,
			Params: map[string]interface{}{},
		}
		_, err := hc.ToSpecificHealthCheck()
		if err != nil {
			t.Errorf("Type '%s' should be handled in ToSpecificHealthCheck, got error: %v", typ, err)
		}
	}
}
