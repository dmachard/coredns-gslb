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
