package gslb

import (
	"testing"
)

func TestMySQLHealthCheck_Defaults(t *testing.T) {
	h := &MySQLHealthCheck{}
	h.SetDefault()
	if h.Port != 3306 {
		t.Errorf("expected default port 3306, got %d", h.Port)
	}
	if h.Timeout != "3s" {
		t.Errorf("expected default timeout 3s, got %s", h.Timeout)
	}
	if h.Query != "SELECT 1" {
		t.Errorf("expected default query 'SELECT 1', got %s", h.Query)
	}
}

func TestMySQLHealthCheck_Equals(t *testing.T) {
	h1 := &MySQLHealthCheck{Host: "127.0.0.1", Port: 3306, User: "a", Database: "b", Query: "SELECT 1"}
	h2 := &MySQLHealthCheck{Host: "127.0.0.1", Port: 3306, User: "a", Database: "b", Query: "SELECT 1"}
	h3 := &MySQLHealthCheck{Host: "127.0.0.2", Port: 3306, User: "a", Database: "b", Query: "SELECT 1"}
	if !h1.Equals(h2) {
		t.Error("expected h1 == h2")
	}
	if h1.Equals(h3) {
		t.Error("expected h1 != h3")
	}
}
