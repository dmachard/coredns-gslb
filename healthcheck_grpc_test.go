package gslb

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

func generateTestCerts(t *testing.T) (certFile, keyFile, caFile string) {
	t.Helper()
	tempDir := t.TempDir()

	// 1. Generate CA key and certificate
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Test CA",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caDer, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	assert.NoError(t, err)

	caPemFile := filepath.Join(tempDir, "ca.pem")
	caOut, err := os.Create(caPemFile)
	assert.NoError(t, err)
	pem.Encode(caOut, &pem.Block{Type: "CERTIFICATE", Bytes: caDer})
	caOut.Close()

	// 2. Generate Cert key and certificate (for both server and client)
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(t, err)

	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:    []string{"localhost"},
	}

	certDer, err := x509.CreateCertificate(rand.Reader, certTemplate, caTemplate, &certKey.PublicKey, caKey)
	assert.NoError(t, err)

	certPemFile := filepath.Join(tempDir, "cert.pem")
	certOut, err := os.Create(certPemFile)
	assert.NoError(t, err)
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDer})
	certOut.Close()

	keyPemFile := filepath.Join(tempDir, "key.pem")
	keyOut, err := os.OpenFile(keyPemFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	assert.NoError(t, err)
	privBytes, err := x509.MarshalECPrivateKey(certKey)
	assert.NoError(t, err)
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	keyOut.Close()

	return certPemFile, keyPemFile, caPemFile
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

func TestGRPCHealthCheck_TLS(t *testing.T) {
	certPath, keyPath, caPath := generateTestCerts(t)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer lis.Close()

	// Load server credentials
	serverCreds, err := credentials.NewServerTLSFromFile(certPath, keyPath)
	assert.NoError(t, err)

	s := grpc.NewServer(grpc.Creds(serverCreds))
	mockServer := &mockHealthServer{status: healthpb.HealthCheckResponse_SERVING}
	healthpb.RegisterHealthServer(s, mockServer)
	go s.Serve(lis)
	defer s.Stop()

	port := lis.Addr().(*net.TCPAddr).Port

	t.Run("TLS success with skip verify", func(t *testing.T) {
		hc := &GRPCHealthCheck{
			Host:          "localhost",
			Port:          port,
			Timeout:       2 * time.Second,
			EnableTLS:     true,
			CA:            caPath,
			SkipTLSVerify: true,
		}
		assert.NoError(t, hc.Check())
	})

	t.Run("TLS success with CA verification", func(t *testing.T) {
		hc := &GRPCHealthCheck{
			Host:          "localhost",
			Port:          port,
			Timeout:       2 * time.Second,
			EnableTLS:     true,
			CA:            caPath,
			SkipTLSVerify: false,
		}
		assert.NoError(t, hc.Check())
	})
}

func TestGRPCHealthCheck_mTLS(t *testing.T) {
	certPath, keyPath, caPath := generateTestCerts(t)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer lis.Close()

	// Configure mTLS on the server
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	assert.NoError(t, err)
	caPem, err := os.ReadFile(caPath)
	assert.NoError(t, err)
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caPem)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}
	serverCreds := credentials.NewTLS(tlsConfig)

	s := grpc.NewServer(grpc.Creds(serverCreds))
	mockServer := &mockHealthServer{status: healthpb.HealthCheckResponse_SERVING}
	healthpb.RegisterHealthServer(s, mockServer)
	go s.Serve(lis)
	defer s.Stop()

	port := lis.Addr().(*net.TCPAddr).Port

	t.Run("mTLS success with client cert", func(t *testing.T) {
		hc := &GRPCHealthCheck{
			Host:          "localhost",
			Port:          port,
			Timeout:       2 * time.Second,
			EnableTLS:     true,
			Cert:          certPath,
			Key:           keyPath,
			CA:            caPath,
			SkipTLSVerify: false,
		}
		assert.NoError(t, hc.Check())
	})

	t.Run("mTLS failure without client cert", func(t *testing.T) {
		hc := &GRPCHealthCheck{
			Host:          "localhost",
			Port:          port,
			Timeout:       2 * time.Second,
			EnableTLS:     true,
			CA:            caPath,
			SkipTLSVerify: true, // skip verify server cert but we still lack client cert
		}
		// The connection should fail because the server requires client certificate
		err := hc.Check()
		assert.Error(t, err)
	})
}
