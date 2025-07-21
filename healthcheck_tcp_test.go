package gslb

import (
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
		retries       int
		expectedError bool
	}{
		{
			name:          "Success",
			startServer:   true,
			retries:       0,
			expectedError: false,
		},
		{
			name:          "FailNoServer",
			startServer:   false,
			retries:       0,
			expectedError: true,
		},
		{
			name:          "RetryLogic",
			startServer:   true,
			retries:       2,
			expectedError: false,
		},
	}

	// Iterate over test cases
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var port int
			var server net.Listener
			var err error

			if test.startServer {
				server, err = net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatalf("Failed to find free port: %v", err)
				}
				port = server.Addr().(*net.TCPAddr).Port
				if test.name == "RetryLogic" {
					go func() {
						time.Sleep(2 * time.Second)
						server.Accept()
					}()
				} else {
					go func() {
						defer server.Close()
						conn, err := server.Accept()
						if err == nil {
							conn.Close()
						}
					}()
				}
				defer server.Close()
			} else {
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatalf("Failed to find free port: %v", err)
				}
				port = ln.Addr().(*net.TCPAddr).Port
				ln.Close() // close immediately so nothing is listening
			}

			backend := &Backend{Address: "127.0.0.1"}
			hc := &TCPHealthCheck{
				Port:    port,
				Timeout: "1s",
			}
			result := hc.PerformCheck(backend, "example.com", test.retries)
			if test.expectedError {
				assert.False(t, result, "Expected failure, but got success in test: %s", test.name)
			} else {
				assert.True(t, result, "Expected success, but got failure in test: %s", test.name)
			}
		})
	}

	t.Run("Success", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to find free port: %v", err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		go func() {
			defer ln.Close()
			conn, err := ln.Accept()
			if err == nil {
				conn.Close()
			}
		}()
		hc := &TCPHealthCheck{Port: port, Timeout: "1s"}
		backend := &Backend{Address: "127.0.0.1"}
		ok := hc.PerformCheck(backend, "test", 0)
		if !ok {
			t.Errorf("Expected TCP healthcheck to succeed on open port %d", port)
		}
	})

	t.Run("FailNoServer", func(t *testing.T) {
		// Use a free port, but don't start a server
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to find free port: %v", err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		ln.Close() // close immediately so nothing is listening
		hc := &TCPHealthCheck{Port: port, Timeout: "1s"}
		backend := &Backend{Address: "127.0.0.1"}
		ok := hc.PerformCheck(backend, "test", 0)
		if ok {
			t.Errorf("Expected TCP healthcheck to fail on closed port %d", port)
		}
	})

	t.Run("RetryLogic", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to find free port: %v", err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		acceptCount := 0
		go func() {
			defer ln.Close()
			for acceptCount < 2 {
				conn, err := ln.Accept()
				if err == nil {
					conn.Close()
					acceptCount++
				}
			}
		}()
		hc := &TCPHealthCheck{Port: port, Timeout: "1s"}
		backend := &Backend{Address: "127.0.0.1"}
		ok := hc.PerformCheck(backend, "test", 2)
		if !ok {
			t.Errorf("Expected TCP healthcheck to succeed with retries on port %d", port)
		}
	})
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
