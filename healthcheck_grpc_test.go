package gslb

import (
	"testing"
	"time"
)

func TestGRPCHealthCheck_Check(t *testing.T) {
	hc := &GRPCHealthCheck{
		Host:    "localhost",
		Port:    50051, // Use a test gRPC server in real tests
		Service: "",
		Timeout: 1 * time.Second,
	}
	// This test expects no gRPC server running, so it should fail
	err := hc.Check()
	if err == nil {
		t.Error("expected error when no gRPC server is running")
	}
}
