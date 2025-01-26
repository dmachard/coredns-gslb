package gslb

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTCPHealthCheck(t *testing.T) {
	// Define test cases
	tests := []struct {
		name          string
		startServer   bool
		port          int
		retries       int
		expectedError bool
	}{
		{
			name:          "Success",
			startServer:   true,
			port:          8080,
			retries:       0,
			expectedError: false,
		},
		{
			name:          "FailNoServer",
			startServer:   false,
			port:          8081,
			retries:       0,
			expectedError: true,
		},
		{
			name:          "RetryLogic",
			startServer:   true,
			port:          8082,
			retries:       2,
			expectedError: false,
		},
	}

	// Iterate over test cases
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var server net.Listener
			var err error

			// Start a mock TCP server if required by the test case
			if test.startServer {
				server, err = net.Listen("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", test.port)))
				assert.NoError(t, err, "Failed to start mock server")
				defer server.Close()

				// Simulate a delay in starting the server for retry logic
				if test.name == "RetryLogic" {
					go func() {
						time.Sleep(2 * time.Second) // Delay the server start
						server.Accept()
					}()
				}
			}

			// Define the backend
			backend := &Backend{Address: "127.0.0.1"}

			// Create a TCPHealthCheck
			hc := &TCPHealthCheck{
				Port:    test.port,
				Timeout: "1s",
			}

			// Run the health check with retries
			result := hc.PerformCheck(backend, "example.com", test.retries)

			// Assert the result
			if test.expectedError {
				assert.False(t, result, "Expected failure, but got success in test: %s", test.name)
			} else {
				assert.True(t, result, "Expected success, but got failure in test: %s", test.name)
			}
		})
	}
}

// Test the Equals method for TCPHealthCheck
func TestTCPHealthCheck_Equals(t *testing.T) {
	hc1 := &TCPHealthCheck{
		Port:    3306,
		Timeout: "1s",
	}

	hc2 := &TCPHealthCheck{
		Port:    3306,
		Timeout: "1s",
	}

	hc3 := &TCPHealthCheck{
		Port:    5432, // Different port
		Timeout: "1s",
	}

	// Assert that hc1 and hc2 are equal
	assert.True(t, hc1.Equals(hc2))

	// Assert that hc1 and hc3 are not equal
	assert.False(t, hc1.Equals(hc3))
}
