package gslb

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type GRPCHealthCheck struct {
	Host          string        `yaml:"host"`
	Port          int           `yaml:"port"`
	Service       string        `yaml:"service"`
	Timeout       time.Duration `yaml:"timeout"`
	EnableTLS     bool          `yaml:"enable_tls"`
	Cert          string        `yaml:"cert_path"`
	Key           string        `yaml:"key_path"`
	CA            string        `yaml:"ca_path"`
	SkipTLSVerify bool          `yaml:"skip_tls_verify"`
}

func (h *GRPCHealthCheck) Check() error {
	addr := fmt.Sprintf("%s:%d", h.Host, h.Port)
	ctx, cancel := context.WithTimeout(context.Background(), h.Timeout)
	defer cancel()

	var creds credentials.TransportCredentials
	if h.EnableTLS {
		tlsConfig, err := NewTLSClientConfig(h.Cert, h.Key, h.CA)
		if err != nil {
			IncHealthcheckFailures("grpc", addr, "connection")
			return fmt.Errorf("failed to create TLS config: %w", err)
		}
		tlsConfig.InsecureSkipVerify = h.SkipTLSVerify
		if h.Host != "" {
			tlsConfig.ServerName = h.Host
		}
		creds = credentials.NewTLS(tlsConfig)
	} else {
		creds = insecure.NewCredentials()
	}

	cc, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		IncHealthcheckFailures("grpc", addr, "connection")
		return fmt.Errorf("gRPC connection failed: %w", err)
	}
	defer cc.Close()
	client := healthpb.NewHealthClient(cc)
	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: h.Service})
	if err != nil {
		IncHealthcheckFailures("grpc", addr, "connection")
		return err
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		IncHealthcheckFailures("grpc", addr, "protocol")
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
		Host:          host,
		Port:          h.Port,
		Service:       h.Service,
		Timeout:       h.Timeout,
		EnableTLS:     h.EnableTLS,
		Cert:          h.Cert,
		Key:           h.Key,
		CA:            h.CA,
		SkipTLSVerify: h.SkipTLSVerify,
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
	return h.Host == otherGrpc.Host &&
		h.Port == otherGrpc.Port &&
		h.Service == otherGrpc.Service &&
		h.Timeout == otherGrpc.Timeout &&
		h.EnableTLS == otherGrpc.EnableTLS &&
		h.Cert == otherGrpc.Cert &&
		h.Key == otherGrpc.Key &&
		h.CA == otherGrpc.CA &&
		h.SkipTLSVerify == otherGrpc.SkipTLSVerify
}
