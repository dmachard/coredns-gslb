package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCorefile(t *testing.T) {
	corefile := `
api_basic_user admin
api_basic_pass secret
api_tls_cert /path/to/cert.pem
api_listen_addr 0.0.0.0
api_listen_port 1234
`
	f, err := os.CreateTemp("", "corefile_test_*.conf")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	_, err = f.Write([]byte(corefile))
	f.Close()
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	os.Setenv("CORE_DNS_COREFILE", f.Name())
	cfg := parseCorefile()
	if cfg.User != "admin" || cfg.Pass != "secret" || !cfg.TLS || cfg.Addr != "0.0.0.0" || cfg.Port != "1234" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestPrettyPrintStatus(t *testing.T) {
	input := []byte(`{"foo":[{"bar":1}]}`)
	var pretty bytes.Buffer
	err := json.Indent(&pretty, input, "", "  ")
	if err != nil {
		t.Fatalf("json.Indent failed: %v", err)
	}
	want := "{\n  \"foo\": [\n    {\n      \"bar\": 1\n    }\n  ]\n}"
	if pretty.String() != want {
		t.Errorf("pretty print failed:\nGot:\n%s\nWant:\n%s", pretty.String(), want)
	}
}

func TestValidateCmd(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Success case
	validConfig := `records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
        healthchecks:
          - type: http
            params:
              port: 80
`
	validFile := filepath.Join(tmpDir, "valid.yml")
	if err := os.WriteFile(validFile, []byte(validConfig), 0644); err != nil {
		t.Fatalf("failed to write valid config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runValidateCmd([]string{validFile}, &stdout, &stderr)
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout.String(), "1 records parsed")
	assert.Contains(t, stdout.String(), "0 healthcheck profiles loaded")
	assert.Contains(t, stdout.String(), "0 errors, 0 warnings — validation succeeded")

	// 2. Warning case (missing healthchecks & duplicate priority)
	warnConfig := `records:
  test.example.com.:
    mode: failover
    backends:
      - address: 1.2.3.4
        priority: 5
      - address: 1.2.3.5
        priority: 5
`
	warnFile := filepath.Join(tmpDir, "warn.yml")
	if err := os.WriteFile(warnFile, []byte(warnConfig), 0644); err != nil {
		t.Fatalf("failed to write warn config: %v", err)
	}

	// 2a. Without --strict: should succeed (exit code 0)
	stdout.Reset()
	stderr.Reset()
	code = runValidateCmd([]string{warnFile}, &stdout, &stderr)
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout.String(), "WARN   duplicate priority value '5'")
	assert.Contains(t, stdout.String(), "WARN   backend '1.2.3.4'")
	assert.Contains(t, stdout.String(), "0 errors, 3 warnings — validation succeeded")

	// 2b. With --strict: should fail (exit code 1)
	stdout.Reset()
	stderr.Reset()
	code = runValidateCmd([]string{"--strict", warnFile}, &stdout, &stderr)
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr.String(), "0 errors, 3 warnings — validation failed")

	// 3. Validation error case (duplicate backend address)
	invalidConfig := `records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
      - address: 1.2.3.4
`
	invalidFile := filepath.Join(tmpDir, "invalid.yml")
	if err := os.WriteFile(invalidFile, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = runValidateCmd([]string{invalidFile}, &stdout, &stderr)
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr.String(), "✗ ERROR  duplicate backend address '1.2.3.4'")
	assert.Contains(t, stderr.String(), "1 error, 3 warnings — validation failed")

	// 4. Other errors case (file not found)
	stdout.Reset()
	stderr.Reset()
	code = runValidateCmd([]string{"nonexistent_file.yml"}, &stdout, &stderr)
	assert.Equal(t, 2, code)
}

func TestApiURL(t *testing.T) {
	cfg1 := Config{TLS: true, Addr: "127.0.0.1", Port: "8443"}
	assert.Equal(t, "https://127.0.0.1:8443", apiURL(cfg1))

	cfg2 := Config{TLS: false, Addr: "localhost", Port: "8080"}
	assert.Equal(t, "http://localhost:8080", apiURL(cfg2))
}

func TestAddAuth(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost", nil)
	cfg := Config{User: "admin", Pass: "secret"}
	addAuth(req, cfg)
	authHeader := req.Header.Get("Authorization")
	assert.True(t, strings.HasPrefix(authHeader, "Basic "))

	req2, _ := http.NewRequest("GET", "http://localhost", nil)
	cfg2 := Config{}
	addAuth(req2, cfg2)
	assert.Empty(t, req2.Header.Get("Authorization"))
}

func TestUsage(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	usage()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	assert.Contains(t, buf.String(), "Usage: gslbctl")
}

func TestBackendsCmd_EnableDisable(t *testing.T) {
	serverCalled := false
	var requestPath string
	var requestBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		requestPath = r.URL.Path
		requestBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "backends": [{"record": "api.example.com.", "address": "1.2.3.4"}]}`))
	}))
	defer ts.Close()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := Config{User: "admin", Pass: "secret"}
	backendsCmd([]string{"enable", "--tags", "t1,t2", "--address", "1.2.3.4", "--location", "us"}, ts.URL, cfg)

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	assert.True(t, serverCalled)
	assert.Equal(t, "/api/backends/enable", requestPath)
	assert.Contains(t, buf.String(), "api.example.com.")
	assert.Contains(t, buf.String(), "1.2.3.4")

	var parsedBody map[string]interface{}
	err := json.Unmarshal(requestBody, &parsedBody)
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4", parsedBody["address_prefix"])
	assert.Equal(t, "us", parsedBody["location"])
}

func TestStatusCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"example.com.": [
				{
					"record": "api.example.com.",
					"status": "healthy",
					"backends": [
						{
							"address": "1.2.3.4",
							"alive": "true",
							"last_healthcheck": "2026-07-07T09:00:00Z"
						}
					]
				}
			]
		}`))
	}))
	defer ts.Close()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := Config{}
	statusCmd(ts.URL, cfg)

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	assert.Contains(t, buf.String(), "example.com.")
	assert.Contains(t, buf.String(), "api.example.com.")
	assert.Contains(t, buf.String(), "healthy")
	assert.Contains(t, buf.String(), "1.2.3.4")
}
