package gslb

import (
	"time"

	"github.com/creasty/defaults"
	probing "github.com/prometheus-community/pro-bing"
)

// ICMPHealthCheck represents the configuration for an ICMP health check.
type ICMPHealthCheck struct {
	Count   int    `yaml:"count" default:"3"`    // Number of ICMP packets to send
	Timeout string `yaml:"timeout" default:"5s"` // Maximum duration for pings
}

// SetDefault applies default values to ICMPHealthCheck fields.
func (h *ICMPHealthCheck) SetDefault() {
	defaults.Set(h)
}

// GetType returns the type of the health check as a string.
func (h *ICMPHealthCheck) GetType() string {
	return "icmp"
}

// PerformCheck executes the ICMP health check for a backend.
func (h *ICMPHealthCheck) PerformCheck(backend *Backend, fqdn string, maxRetries int) bool {
	typeStr := h.GetType()
	address := backend.Address
	start := time.Now()
	result := false
	defer func() {
		ObserveHealthcheck(fqdn, typeStr, address, start, result)
	}()

	timeout, err := time.ParseDuration(h.Timeout)
	if err != nil {
		log.Errorf("[%s] invalid timeout format: %v", fqdn, err)
		IncHealthcheckFailures(typeStr, address, "timeout")
		return false
	}

	for retry := 0; retry <= maxRetries; retry++ {
		pinger, err := createPinger(backend.Address, h.Count, timeout)
		if err != nil {
			log.Errorf("[%s] ICMP health check failed to initialize pinger: %v", fqdn, err)
			if retry == maxRetries {
				IncHealthcheckFailures(typeStr, address, "connection")
				return false
			}
			continue
		}
		pinger.SetPrivileged(true) // Required for ICMP on most systems

		log.Debugf("[%s] Starting ICMP health check for backend: %s", fqdn, backend.Address)
		err = pinger.Run()
		if err != nil {
			log.Debugf("[%s] ICMP health check failed: %v", fqdn, err)
			if retry == maxRetries {
				IncHealthcheckFailures(typeStr, address, "connection")
				return false
			}
			continue
		}

		stats := pinger.Statistics()
		if stats.PacketsRecv > 0 {
			log.Debugf("[%s] ICMP health check successful: %s received %d/%d packets", fqdn, backend.Address, stats.PacketsRecv, stats.PacketsSent)
			result = true
			return true
		}
	}

	IncHealthcheckFailures(typeStr, address, "other")
	return false
}

// Equals compares two ICMPHealthCheck objects for equality.
func (h *ICMPHealthCheck) Equals(other GenericHealthCheck) bool {
	otherICMP, ok := other.(*ICMPHealthCheck)
	if !ok {
		return false
	}

	return h.Count == otherICMP.Count &&
		h.Timeout == otherICMP.Timeout
}

type Pinger interface {
	Run() error
	Statistics() *probing.Statistics
	SetPrivileged(privileged bool)
}

type RealPinger struct {
	pinger *probing.Pinger
}

func (r *RealPinger) Run() error {
	return r.pinger.Run()
}

func (r *RealPinger) Statistics() *probing.Statistics {
	return r.pinger.Statistics()
}

func (r *RealPinger) SetPrivileged(privileged bool) {
	r.pinger.SetPrivileged(privileged)
}

func createPinger(address string, count int, timeout time.Duration) (Pinger, error) {
	pinger, err := probing.NewPinger(address)
	if err != nil {
		return nil, err
	}
	pinger.Count = count
	pinger.Timeout = timeout
	return &RealPinger{pinger: pinger}, nil
}
