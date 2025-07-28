package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
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
