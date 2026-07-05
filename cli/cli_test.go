package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
	assert.Contains(t, stdout.String(), "Configuration is valid.")

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

	stdout.Reset()
	stderr.Reset()
	code = runValidateCmd([]string{warnFile}, &stdout, &stderr)
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout.String(), "duplicate priority value '5'")
	assert.Contains(t, stdout.String(), "configured without any healthchecks")

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
	assert.Contains(t, stderr.String(), "duplicate backend address '1.2.3.4'")

	// 4. Other errors case (file not found)
	stdout.Reset()
	stderr.Reset()
	code = runValidateCmd([]string{"nonexistent_file.yml"}, &stdout, &stderr)
	assert.Equal(t, 2, code)
}
