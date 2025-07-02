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
