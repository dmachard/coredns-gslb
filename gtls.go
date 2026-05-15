package gslb

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func setTLSDefaults(ctls *tls.Config) {
	ctls.MinVersion = tls.VersionTLS12
	ctls.MaxVersion = tls.VersionTLS13
	ctls.CipherSuites = []uint16{
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}
}

// NewTLSConfig returns a TLS config that includes a certificate and key pair
// If caPath is empty, default root store will be used
// If certPath or keyPath are empty, client certificate will not be loaded
func NewTLSClientConfig(certPath, keyPath, caPath string) (*tls.Config, error) {
	var tlsConfig *tls.Config
	roots, err := loadRoots(caPath)
	if err != nil {
		return nil, err
	}
	if certPath != "" && keyPath != "" {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("could not load TLS cert: %w", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      roots,
		}
	} else {
		tlsConfig = &tls.Config{
			RootCAs: roots,
		}
	}
	setTLSDefaults(tlsConfig)

	return tlsConfig, nil
}

func loadRoots(caPath string) (*x509.CertPool, error) {
	if caPath == "" {
		return nil, nil
	}

	roots := x509.NewCertPool()
	pem, err := os.ReadFile(filepath.Clean(caPath))
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", caPath, err)
	}
	ok := roots.AppendCertsFromPEM(pem)
	if !ok {
		return nil, fmt.Errorf("could not read root certs: %w", err)
	}
	return roots, nil
}

func NewHTTPSTransport(cc *tls.Config) *http.Transport {
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cc,
		MaxIdleConnsPerHost: 25,
	}

	return tr
}
