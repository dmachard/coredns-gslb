package gslb

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/creasty/defaults"
)

// HTTPHealthCheck represents HTTP-specific health check settings.
type HTTPHealthCheck struct {
	Port          int               `yaml:"port" default:"443"`
	EnableTLS     bool              `yaml:"enable_tls" default:"true"`
	URI           string            `yaml:"uri" default:"/"`
	Method        string            `yaml:"method" default:"GET"`
	Host          string            `yaml:"host" default:"localhost"`
	Headers       map[string]string `yaml:"headers"`
	Timeout       string            `yaml:"timeout" default:"5s"`
	ExpectedCode  int               `yaml:"expected_code" default:"200"`
	ExpectedBody  string            `yaml:"expected_body" default:""`
	SkipTLSVerify bool              `yaml:"skip_tls_verify" default:"false"`
}

func (h *HTTPHealthCheck) SetDefault() {
	defaults.Set(h)
}

func (h *HTTPHealthCheck) GetType() string {
	if h.EnableTLS {
		return fmt.Sprintf("https/%d", h.Port)
	}
	return fmt.Sprintf("http/%d", h.Port)
}

// createHTTPClient returns an http client with appropriate transport settings, including timeout and TLS configuration.
func createHTTPClient(enableTLS bool, skipTLSVerify bool, timeout time.Duration) *http.Client {
	// Create the HTTP transport configuration
	var transport http.RoundTripper = &http.Transport{}
	if enableTLS {
		// Configure TLS settings if enabled
		tlsConfig := &tls.Config{}
		if skipTLSVerify {
			tlsConfig.InsecureSkipVerify = true
		}
		transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	// Return the HTTP client with the specified transport and timeout
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

// retryHealthCheck retries the HTTP request up to the specified retries.
func (h *HTTPHealthCheck) retryHealthCheck(client *http.Client, req *http.Request, backend *Backend, fqdn string, maxRetries int) (*http.Response, error) {
	var resp *http.Response
	var err error
	for retry := 0; retry <= maxRetries; retry++ {
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == h.ExpectedCode {
			// Check the body if expected
			if h.ExpectedBody != "" {
				if err := h.checkExpectedBody(resp.Body, fqdn); err != nil {
					log.Debugf("[%s] HTTP healthcheck body mismatch: %v", fqdn, err)
					if retry == maxRetries {
						return nil, err
					}
					continue
				}
			}
			return resp, nil
		}

		// Log errors and retry
		if err != nil {
			log.Debugf("[%s] HTTP healthcheck failed (retries=%d/%d): [backend=%s:%d uri:%s method:%s host:%s] %v", fqdn, retry, maxRetries, backend.Address, h.Port, h.URI, h.Method, h.Host, err)
		} else {
			log.Debugf("[%s] HTTP healthcheck failed (retries=%d/%d): [backend=%s:%d uri:%s method:%s host:%s] unexpected status code: got %d, want %d", fqdn, retry, maxRetries, backend.Address, h.Port, h.URI, h.Method, h.Host, resp.StatusCode, h.ExpectedCode)
		}
		if retry == maxRetries {
			return nil, fmt.Errorf("[%s] HTTP health check failed after %d retries", fqdn, maxRetries)
		}
	}
	return nil, err
}

// checkExpectedBody reads and checks the response body against the expected body.
func (h *HTTPHealthCheck) checkExpectedBody(body io.ReadCloser, fqdn string) error {
	defer body.Close()

	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("[%s] failed to read response body: %w", fqdn, err)
	}

	if matched, err := regexp.MatchString(h.ExpectedBody, string(bodyBytes)); err != nil {
		return fmt.Errorf("[%s] invalid regex for expected body: %w", fqdn, err)
	} else if !matched {
		return fmt.Errorf("[%s] body mismatch: expected regex '%s', got '%s'", fqdn, h.ExpectedBody, string(bodyBytes))
	}
	return nil
}

// PerformCheck implements the HealthCheck interface for HTTP health checks
func (h *HTTPHealthCheck) PerformCheck(backend *Backend, fqdn string, maxRetries int) bool {
	typeStr := h.GetType()
	address := backend.Address
	start := time.Now()
	result := false
	defer func() {
		ObserveHealthcheck(typeStr, address, start, result)
	}()

	scheme := "http"
	if h.EnableTLS {
		scheme = "https"
	}

	// Build URL for the health check
	url := buildHealthCheckURL(scheme, backend.Address, h.Port, h.URI)

	t, err := time.ParseDuration(h.Timeout)
	if err != nil {
		log.Errorf("[%s] invalid timeout format: %v", fqdn, err)
		return false
	}

	client := createHTTPClient(h.EnableTLS, h.SkipTLSVerify, t)

	// Create HTTP request
	req, err := http.NewRequest(h.Method, url, nil)
	if err != nil {
		log.Debugf("[%s] HTTP healthcheck failed: [backend=%s:%d scheme:%s uri:%s method:%s host:%s] error to create http request: %v", fqdn, backend.Address, h.Port, scheme, h.URI, h.Method, h.Host, err)
		return false
	}
	req.Host = h.Host
	for key, value := range h.Headers {
		req.Header.Add(key, value)
	}

	// Retry health check
	resp, err := h.retryHealthCheck(client, req, backend, fqdn, maxRetries)
	if err != nil {
		return false
	}

	// Log successful health check
	defer resp.Body.Close()

	log.Debugf("[%s] HTTP healthcheck success [backend=%s:%d scheme:%s uri:%s method:%s host:%s]", fqdn, backend.Address, h.Port, scheme, h.URI, h.Method, h.Host)
	result = true
	return true
}

// Equals compares two HTTPHealthCheck objects for equality.
func (h *HTTPHealthCheck) Equals(other GenericHealthCheck) bool {
	otherHTTP, ok := other.(*HTTPHealthCheck)
	if !ok {
		return false
	}

	// Compare all fields
	if h.Port != otherHTTP.Port ||
		h.EnableTLS != otherHTTP.EnableTLS ||
		h.URI != otherHTTP.URI ||
		h.Method != otherHTTP.Method ||
		h.Host != otherHTTP.Host ||
		h.Timeout != otherHTTP.Timeout ||
		h.ExpectedCode != otherHTTP.ExpectedCode ||
		h.ExpectedBody != otherHTTP.ExpectedBody ||
		h.SkipTLSVerify != otherHTTP.SkipTLSVerify ||
		len(h.Headers) != len(otherHTTP.Headers) {
		return false
	}

	// Compare headers
	for key, value := range h.Headers {
		if otherValue, exists := otherHTTP.Headers[key]; !exists || value != otherValue {
			return false
		}
	}

	return true
}

// Helper function to build the health check URL
func buildHealthCheckURL(scheme, address string, port int, uri string) string {
	return fmt.Sprintf("%s://%s:%d%s", scheme, address, port, uri)
}
