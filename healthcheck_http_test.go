package gslb

import (
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test HTTPHealthCheck
var requestCount int

func TestHTTPHealthCheck(t *testing.T) {
	// Define test cases
	tests := []struct {
		name          string
		handler       http.HandlerFunc
		expectedError bool
		expectedBody  string
		expectedCode  int
		retries       int
	}{
		{
			name: "Success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/health", r.URL.Path)
				assert.Equal(t, "GET", r.Method)
				assert.Regexp(t, regexp.MustCompile(`CoreDNS/.+ GSLB/.+`), r.Header.Get("User-Agent"))
				w.WriteHeader(200)
				w.Write([]byte("OK"))
			},
			expectedError: false,
			expectedCode:  200,
			expectedBody:  "OK",
			retries:       0,
		},
		{
			name: "Success302IgnoreForward",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/health", r.URL.Path)
				assert.Equal(t, "GET", r.Method)
				w.Header().Set("Location", "/somewhere-else")
				w.WriteHeader(302)
				w.Write([]byte("OK"))
			},
			expectedError: false,
			expectedCode:  302,
			retries:       0,
		},
		{
			name: "FailStatusCode",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
			},
			expectedError: true,
			expectedCode:  200,
			retries:       0,
		},
		{
			name: "FailBody",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				w.Write([]byte("Incorrect body"))
			},
			expectedError: true,
			expectedCode:  200,
			expectedBody:  "OK", // Expecting "OK", but server returns "Incorrect body"
			retries:       0,
		},
		{
			name: "RetryLogic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				if requestCount <= 2 {
					w.WriteHeader(500) // Simulate failure for the first 2 attempts
				} else {
					w.WriteHeader(200) // Simulate success on the 3rd attempt
					w.Write([]byte("OK"))
				}
			},
			expectedError: false,
			expectedCode:  200,
			expectedBody:  "OK",
			retries:       3,
		},
	}

	// Iterate over test cases
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Setup the test server
			server := httptest.NewServer(test.handler)
			defer server.Close()

			// Define the backend
			backend := &Backend{Address: server.Listener.Addr().(*net.TCPAddr).IP.String()}

			// Create an HTTPHealthCheck
			hc := &HTTPHealthCheck{
				Port:         server.Listener.Addr().(*net.TCPAddr).Port,
				EnableTLS:    false,
				URI:          "/health",
				Method:       "GET",
				Host:         server.URL,
				Timeout:      "2s",
				ExpectedCode: test.expectedCode,
				ExpectedBody: test.expectedBody,
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

// Test the Equals method.
func TestHTTPHealthCheck_Equals(t *testing.T) {
	hc1 := &HTTPHealthCheck{
		Port:         80,
		EnableTLS:    false,
		URI:          "/health",
		Method:       "GET",
		Host:         "localhost",
		Timeout:      "2s",
		ExpectedCode: 200,
		ExpectedBody: "OK",
	}

	hc2 := &HTTPHealthCheck{
		Port:         80,
		EnableTLS:    false,
		URI:          "/health",
		Method:       "GET",
		Host:         "localhost",
		Timeout:      "2s",
		ExpectedCode: 200,
		ExpectedBody: "OK",
	}

	hc3 := &HTTPHealthCheck{
		Port:         8080, // Different port
		EnableTLS:    false,
		URI:          "/health",
		Method:       "GET",
		Host:         "localhost",
		Timeout:      "2s",
		ExpectedCode: 200,
		ExpectedBody: "OK",
	}

	// Assert that hc1 and hc2 are equal
	assert.True(t, hc1.Equals(hc2))

	// Assert that hc1 and hc3 are not equal
	assert.False(t, hc1.Equals(hc3))
}

func TestHTTPHealthCheck_More(t *testing.T) {
	// 1. Invalid timeout
	hc := &HTTPHealthCheck{
		Timeout: "invalid",
	}
	backend := &Backend{Address: "127.0.0.1"}
	assert.False(t, hc.PerformCheck(backend, "example.com", 0))

	// 2. Regex Failure
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	}))
	defer ts.Close()

	hcRegex := &HTTPHealthCheck{
		Port:         ts.Listener.Addr().(*net.TCPAddr).Port,
		EnableTLS:    false,
		URI:          "/health",
		Method:       "GET",
		Host:         ts.URL,
		Timeout:      "2s",
		ExpectedCode: 200,
		ExpectedBody: "[[", // Invalid regex
	}
	assert.False(t, hcRegex.PerformCheck(backend, "example.com", 0))

	// 3. TLS cert creation failure (triggers error inside NewTLSClientConfig)
	hcTLS := &HTTPHealthCheck{
		Port:         443,
		EnableTLS:    true,
		Cert:         "/invalid/cert.pem",
		Key:          "/invalid/key.pem",
		Timeout:      "2s",
		ExpectedCode: 200,
	}
	_ = hcTLS.PerformCheck(backend, "example.com", 0)

	// 4. Equals more checks
	base := &HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "k", CA: "ca", URI: "/", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "B"}}

	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 81, EnableTLS: true, Cert: "c", Key: "k", CA: "ca", URI: "/", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "B"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: false, Cert: "c", Key: "k", CA: "ca", URI: "/", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "B"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "x", Key: "k", CA: "ca", URI: "/", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "B"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "x", CA: "ca", URI: "/", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "B"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "k", CA: "x", URI: "/", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "B"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "k", CA: "ca", URI: "/x", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "B"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "k", CA: "ca", URI: "/", Method: "POST", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "B"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "k", CA: "ca", URI: "/", Method: "GET", Host: "x", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "B"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "k", CA: "ca", URI: "/", Method: "GET", Host: "h", Timeout: "2s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "B"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "k", CA: "ca", URI: "/", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 500, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "B"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "k", CA: "ca", URI: "/", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "x", SkipTLSVerify: true, Headers: map[string]string{"A": "B"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "k", CA: "ca", URI: "/", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: false, Headers: map[string]string{"A": "B"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "k", CA: "ca", URI: "/", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "B", "C": "D"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "k", CA: "ca", URI: "/", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"A": "X"}}))
	assert.False(t, base.Equals(&HTTPHealthCheck{Port: 80, EnableTLS: true, Cert: "c", Key: "k", CA: "ca", URI: "/", Method: "GET", Host: "h", Timeout: "1s", ExpectedCode: 200, ExpectedBody: "ok", SkipTLSVerify: true, Headers: map[string]string{"Y": "B"}}))
	assert.False(t, base.Equals(&TCPHealthCheck{}))
}
