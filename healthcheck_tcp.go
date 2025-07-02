package gslb

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/creasty/defaults"
)

// TCPHealthCheck represents TCP-specific health check settings.
type TCPHealthCheck struct {
	Port    int    `yaml:"port" default:"80"`    // TCP port to connect to
	Timeout string `yaml:"timeout" default:"5s"` // Timeout for the TCP connection
}

// SetDefault applies default values to TCPHealthCheck fields.
func (h *TCPHealthCheck) SetDefault() {
	defaults.Set(h)
}

// GetType returns the type of the health check as a string.
func (h *TCPHealthCheck) GetType() string {
	return fmt.Sprintf("tcp/%d", h.Port)
}

// PerformCheck implements the health check logic for TCP connections.
func (h *TCPHealthCheck) PerformCheck(backend *Backend, fqdn string, maxRetries int) bool {
	typeStr := h.GetType()
	address := backend.Address
	start := time.Now()
	result := false
	defer func() {
		ObserveHealthcheck(typeStr, address, start, result)
	}()

	timeout, err := time.ParseDuration(h.Timeout)
	if err != nil {
		log.Errorf("[%s] invalid timeout format: %v", fqdn, err)
		return false
	}

	addressPort := net.JoinHostPort(backend.Address, strconv.Itoa(h.Port))
	for retry := 0; retry <= maxRetries; retry++ {
		log.Debugf("[%s] Attempting TCP health check on %s", fqdn, addressPort)

		conn, err := net.DialTimeout("tcp", addressPort, timeout)
		if err != nil {
			log.Debugf("[%s] TCP health check failed (retries=%d/%d): %v", fqdn, retry, maxRetries, err)
			if retry == maxRetries {
				return false
			}
			continue
		}

		// Successfully connected
		conn.Close()
		log.Debugf("[%s] TCP health check successful for %s", fqdn, addressPort)
		result = true
		return true
	}

	return false
}

// Equals compares two TCPHealthCheck objects for equality.
func (h *TCPHealthCheck) Equals(other GenericHealthCheck) bool {
	otherTCP, ok := other.(*TCPHealthCheck)
	if !ok {
		return false
	}

	return h.Port == otherTCP.Port && h.Timeout == otherTCP.Timeout
}
