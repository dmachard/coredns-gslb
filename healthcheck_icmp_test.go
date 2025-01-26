//go:build icmp_test
// +build icmp_test

package gslb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// requires that it be run with super-user privileges.
func TestICMPHealthCheckPerformCheck(t *testing.T) {
	healthCheck := &ICMPHealthCheck{
		Count:   2,
		Timeout: "5s",
	}

	backend := &Backend{
		Address: "127.0.0.1", // Ping localhost
	}

	fqdn := "test.localhost"

	result := healthCheck.PerformCheck(backend, fqdn, 1)

	// Assert that the health check passes for localhost
	assert.True(t, result, "ICMP health check should succeed for localhost")
}
