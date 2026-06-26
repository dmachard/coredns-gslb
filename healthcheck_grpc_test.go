package gslb

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type mockHealthServer struct {
	healthpb.UnimplementedHealthServer
	status healthpb.HealthCheckResponse_ServingStatus
	err    error
}

func (m *mockHealthServer) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &healthpb.HealthCheckResponse{Status: m.status}, nil
}

func TestGRPCHealthCheck_Check(t *testing.T) {
	hc := &GRPCHealthCheck{
		Host:    "localhost",
		Port:    50051,
		Service: "",
		Timeout: 1 * time.Second,
	}
	// This test expects no gRPC server running, so it should fail
	err := hc.Check()
	assert.Error(t, err)
}

func TestGRPCHealthCheck_Full(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer lis.Close()

	s := grpc.NewServer()
	mockServer := &mockHealthServer{status: healthpb.HealthCheckResponse_SERVING}
	healthpb.RegisterHealthServer(s, mockServer)
	go s.Serve(lis)
	defer s.Stop()

	port := lis.Addr().(*net.TCPAddr).Port

	t.Run("serving status success", func(t *testing.T) {
		mockServer.status = healthpb.HealthCheckResponse_SERVING
		mockServer.err = nil

		hc := &GRPCHealthCheck{
			Host:    "127.0.0.1",
			Port:    port,
			Timeout: 2 * time.Second,
		}
		assert.NoError(t, hc.Check())

		backend := &Backend{Address: "127.0.0.1"}
		hcPerform := &GRPCHealthCheck{Port: port, Timeout: 2 * time.Second}
		assert.True(t, hcPerform.PerformCheck(backend, "test.local.", 1))
	})

	t.Run("not serving status failure", func(t *testing.T) {
		mockServer.status = healthpb.HealthCheckResponse_NOT_SERVING
		mockServer.err = nil

		hc := &GRPCHealthCheck{
			Host:    "127.0.0.1",
			Port:    port,
			Timeout: 2 * time.Second,
		}
		assert.Error(t, hc.Check())
	})

	t.Run("grpc service check returns error", func(t *testing.T) {
		mockServer.err = errors.New("check error")

		hc := &GRPCHealthCheck{
			Host:    "127.0.0.1",
			Port:    port,
			Timeout: 2 * time.Second,
		}
		assert.Error(t, hc.Check())
	})

	t.Run("Equals", func(t *testing.T) {
		h1 := &GRPCHealthCheck{Host: "127.0.0.1", Port: 80, Service: "srv", Timeout: 1 * time.Second}
		h2 := &GRPCHealthCheck{Host: "127.0.0.1", Port: 80, Service: "srv", Timeout: 1 * time.Second}
		h3 := &GRPCHealthCheck{Host: "127.0.0.2", Port: 80, Service: "srv", Timeout: 1 * time.Second}

		assert.True(t, h1.Equals(h2))
		assert.False(t, h1.Equals(h3))
		assert.False(t, h1.Equals(&HTTPHealthCheck{}))
	})
}
