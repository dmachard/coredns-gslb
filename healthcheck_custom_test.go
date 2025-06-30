package gslb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCustomHealthCheck_Success(t *testing.T) {
	// Script always returns 0
	check := &CustomHealthCheck{
		Script:  "exit 0",
		Timeout: 2 * time.Second,
	}
	backend := &Backend{Address: "127.0.0.1", Priority: 1, Enable: true}
	result := check.PerformCheck(backend, "test.local.", 1)
	assert.True(t, result, "Expected custom healthcheck to succeed")
}

func TestCustomHealthCheck_Fail(t *testing.T) {
	// Script always returns 1
	check := &CustomHealthCheck{
		Script:  "exit 1",
		Timeout: 2 * time.Second,
	}
	backend := &Backend{Address: "127.0.0.1", Priority: 1, Enable: true}
	result := check.PerformCheck(backend, "test.local.", 1)
	assert.False(t, result, "Expected custom healthcheck to fail")
}

func TestCustomHealthCheck_Timeout(t *testing.T) {
	// Script sleeps longer than timeout
	check := &CustomHealthCheck{
		Script:  "sleep 5",
		Timeout: 1 * time.Second,
	}
	backend := &Backend{Address: "127.0.0.1", Priority: 1, Enable: true}
	result := check.PerformCheck(backend, "test.local.", 1)
	assert.False(t, result, "Expected custom healthcheck to timeout and fail")
}

func TestCustomHealthCheck_EnvVars(t *testing.T) {
	// Script checks env var
	check := &CustomHealthCheck{
		Script:  "[ \"$BACKEND_ADDRESS\" = '1.2.3.4' ]",
		Timeout: 2 * time.Second,
	}
	backend := &Backend{Address: "1.2.3.4", Priority: 42, Enable: false}
	result := check.PerformCheck(backend, "fqdn.test.", 1)
	assert.True(t, result, "Expected custom healthcheck to see env var BACKEND_ADDRESS")
}
