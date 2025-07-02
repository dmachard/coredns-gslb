package gslb

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type GRPCHealthCheck struct {
	Host    string
	Port    int
	Service string
	Timeout time.Duration
}

func (h *GRPCHealthCheck) Check() error {
	addr := fmt.Sprintf("%s:%d", h.Host, h.Port)
	ctx, cancel := context.WithTimeout(context.Background(), h.Timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer conn.Close()
	client := healthpb.NewHealthClient(conn)
	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: h.Service})
	if err != nil {
		return err
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("gRPC health status: %s", resp.Status.String())
	}
	return nil
}

func (h *GRPCHealthCheck) SetDefault() {
	if h.Timeout == 0 {
		h.Timeout = 5 * time.Second
	}
	if h.Service == "" {
		h.Service = ""
	}
}

func (h *GRPCHealthCheck) PerformCheck(backend *Backend, fqdn string, maxRetries int) bool {
	host := h.Host
	if host == "" && backend != nil {
		host = backend.Address
	}
	check := &GRPCHealthCheck{
		Host:    host,
		Port:    h.Port,
		Service: h.Service,
		Timeout: h.Timeout,
	}
	return check.Check() == nil
}

func (h *GRPCHealthCheck) GetType() string {
	return "grpc"
}

func (h *GRPCHealthCheck) Equals(other GenericHealthCheck) bool {
	otherGrpc, ok := other.(*GRPCHealthCheck)
	if !ok {
		return false
	}
	return h.Host == otherGrpc.Host && h.Port == otherGrpc.Port && h.Service == otherGrpc.Service && h.Timeout == otherGrpc.Timeout
}
