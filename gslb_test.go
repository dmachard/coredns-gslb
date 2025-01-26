package gslb

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

func TestGSLB_PickBackendWithFailover_IPv4(t *testing.T) {
	// Create mock backends with different priorities and health statuses
	backendHealthy := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true, Priority: 10}}
	backendUnhealthy := &MockBackend{Backend: &Backend{Address: "192.168.1.2", Enable: true, Priority: 20}}

	// Mock the behavior of the IsHealthy method
	backendHealthy.On("IsHealthy").Return(true)
	backendUnhealthy.On("IsHealthy").Return(false)

	// Create a record
	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "failover",
		Backends: []BackendInterface{backendHealthy, backendUnhealthy},
	}

	// Create the GSLB object
	g := &GSLB{}

	// Test the pickFailoverBackend method
	ip, err := g.pickBackendWithFailover(record, dns.TypeA)

	// Assert the results
	assert.NoError(t, err, "Expected pickFailoverBackend to succeed")
	assert.Equal(t, "192.168.1.1", ip, "Expected the healthy backend to be selected")
}

func TestGSLB_PickBackendWithFailover_IPv6(t *testing.T) {
	// Create mock backends with different priorities and health statuses
	backendHealthy := &MockBackend{Backend: &Backend{Address: "2001:db8::1", Enable: true, Priority: 10}}
	backendUnhealthy := &MockBackend{Backend: &Backend{Address: "2001:db8::2", Enable: true, Priority: 20}}

	// Mock the behavior of the IsHealthy method
	backendHealthy.On("IsHealthy").Return(true)
	backendUnhealthy.On("IsHealthy").Return(false)

	// Create a record
	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "failover",
		Backends: []BackendInterface{backendHealthy, backendUnhealthy},
	}

	// Create the GSLB object
	g := &GSLB{}

	// Test the pickFailoverBackend method
	ip, err := g.pickBackendWithFailover(record, dns.TypeAAAA)

	// Assert the results
	assert.NoError(t, err, "Expected pickFailoverBackend to succeed")
	assert.Equal(t, "2001:db8::1", ip, "Expected the healthy backend to be selected")
}

func TestGSLB_PickAllAddresses_IPv4(t *testing.T) {
	// Create mock backends
	backend1 := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true, Priority: 10}}
	backend2 := &MockBackend{Backend: &Backend{Address: "192.168.1.2", Enable: true, Priority: 20}}

	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "failover",
		Backends: []BackendInterface{backend1, backend2},
	}

	// Create the GSLB object
	g := &GSLB{
		Records: make(map[string]*Record),
	}
	g.Records["example.com."] = record

	// Test the pickAll method
	ipAddresses, err := g.pickAllAddresses("example.com.", dns.TypeA)

	// Assert the results
	assert.NoError(t, err, "Expected pickAll to succeed")
	assert.Len(t, ipAddresses, 2, "Expected to retrieve two backend IPs")
	assert.Contains(t, ipAddresses, "192.168.1.1", "Expected IP 192.168.1.1 to be included")
	assert.Contains(t, ipAddresses, "192.168.1.2", "Expected IP 192.168.1.2 to be included")
}

func TestGSLB_PickAllAddresses_IPv6(t *testing.T) {
	// Create mock backends
	backend1 := &MockBackend{Backend: &Backend{Address: "2001:db8::1", Enable: true, Priority: 10}}
	backend2 := &MockBackend{Backend: &Backend{Address: "2001:db8::2", Enable: true, Priority: 20}}

	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "failover",
		Backends: []BackendInterface{backend1, backend2},
	}

	// Create the GSLB object
	g := &GSLB{
		Records: make(map[string]*Record),
	}
	g.Records["example.com."] = record

	// Test the pickAll method
	ipAddresses, err := g.pickAllAddresses("example.com.", dns.TypeAAAA)

	// Assert the results
	assert.NoError(t, err, "Expected pickAll to succeed")
	assert.Len(t, ipAddresses, 2, "Expected to retrieve two backend IPs")
	assert.Contains(t, ipAddresses, "2001:db8::1", "Expected IP 2001:db8::1 to be included")
	assert.Contains(t, ipAddresses, "2001:db8::2", "Expected IP 2001:db8::2 to be included")
}

func TestGSLB_PickAllAddresses_DisabledBackend(t *testing.T) {
	// Create mock backends
	backend1 := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true, Priority: 10}}
	backend2 := &MockBackend{Backend: &Backend{Address: "192.168.1.2", Enable: false, Priority: 20}}

	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "failover",
		Backends: []BackendInterface{backend1, backend2},
	}

	// Create the GSLB object
	g := &GSLB{
		Records: make(map[string]*Record),
	}
	g.Records["example.com."] = record

	// Test the pickAll method
	ipAddresses, err := g.pickAllAddresses("example.com.", dns.TypeA)

	// Assert the results
	assert.NoError(t, err, "Expected pickAll to succeed")
	assert.Len(t, ipAddresses, 1, "Expected to retrieve only one backend IP")
	assert.Contains(t, ipAddresses, "192.168.1.1", "Expected IP 192.168.1.1 to be included")
}

func TestGSLB_PickAllAddresses_NoBackends(t *testing.T) {
	// Create a record with no backends
	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "failover",
		Backends: []BackendInterface{},
	}

	// Create the GSLB object
	g := &GSLB{
		Records: make(map[string]*Record),
	}
	g.Records["example.com."] = record

	// Test the pickAll method
	ipAddresses, err := g.pickAllAddresses("example.com.", dns.TypeA)

	// Assert the results
	assert.Error(t, err, "Expected an error when no backends exist")
	assert.EqualError(t, err, "no backends exist for domain: example.com.", "Expected specific error message")
	assert.Nil(t, ipAddresses, "Expected no IP addresses to be returned")
}

func TestGSLB_PickAllAddresses_UnknownDomain(t *testing.T) {
	g := &GSLB{
		Records: make(map[string]*Record),
	}

	ipAddresses, err := g.pickAllAddresses("unknown.com.", 1)

	assert.Error(t, err, "Expected an error for unknown domain")
	assert.EqualError(t, err, "domain not found: unknown.com.", "Expected specific error message")
	assert.Nil(t, ipAddresses, "Expected no IP addresses to be returned")
}

func TestGSLB_PickBackendWithRoundRobin_IPv4(t *testing.T) {
	// Create mock backends with IPv4 addresses
	backend1 := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true}}
	backend2 := &MockBackend{Backend: &Backend{Address: "192.168.1.2", Enable: true}}
	backend3 := &MockBackend{Backend: &Backend{Address: "192.168.1.3", Enable: true}}

	// Mock the behavior of the IsHealthy method
	backend1.On("IsHealthy").Return(true)
	backend2.On("IsHealthy").Return(true)
	backend3.On("IsHealthy").Return(true)

	// Create a record with healthy backends
	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "round-robin",
		Backends: []BackendInterface{backend1, backend2, backend3},
	}

	// Create the GSLB object
	g := &GSLB{}

	// Perform the first selection; index should be 0
	ip, err := g.pickBackendWithRoundRobin("example.com.", record, dns.TypeA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "192.168.1.1", ip, "Expected the first backend to be selected")

	// Perform the second selection; index should be 1
	ip, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "192.168.1.2", ip, "Expected the second backend to be selected")

	// Perform the third selection; index should be 2
	ip, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "192.168.1.3", ip, "Expected the third backend to be selected")

	// Perform the fourth selection; index should wrap back to 0
	ip, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "192.168.1.1", ip, "Expected the first backend to be selected again")
}

func TestGSLB_PickBackendWithRoundRobin_IPv6(t *testing.T) {
	// Create mock backends with IPv6 addresses
	backend1 := &MockBackend{Backend: &Backend{Address: "2001:db8::1", Enable: true}}
	backend2 := &MockBackend{Backend: &Backend{Address: "2001:db8::2", Enable: true}}
	backend3 := &MockBackend{Backend: &Backend{Address: "2001:db8::3", Enable: true}}

	// Mock the behavior of the IsHealthy method
	backend1.On("IsHealthy").Return(true)
	backend2.On("IsHealthy").Return(true)
	backend3.On("IsHealthy").Return(true)

	// Create a record with healthy backends
	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "round-robin",
		Backends: []BackendInterface{backend1, backend2, backend3},
	}

	// Create the GSLB object
	g := &GSLB{}

	// Perform the first selection; index should be 0
	ip, err := g.pickBackendWithRoundRobin("example.com.", record, dns.TypeAAAA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "2001:db8::1", ip, "Expected the first IPv6 backend to be selected")

	// Perform the second selection; index should be 1
	ip, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeAAAA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "2001:db8::2", ip, "Expected the second IPv6 backend to be selected")

	// Perform the third selection; index should be 2
	ip, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeAAAA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "2001:db8::3", ip, "Expected the third IPv6 backend to be selected")

	// Perform the fourth selection; index should wrap back to 0
	ip, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeAAAA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "2001:db8::1", ip, "Expected the first IPv6 backend to be selected again")
}

func TestGSLB_PickBackendWithRandom_IPv4(t *testing.T) {
	// Create mock backends
	backend1 := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true}}
	backend2 := &MockBackend{Backend: &Backend{Address: "192.168.1.2", Enable: true}}
	backend3 := &MockBackend{Backend: &Backend{Address: "192.168.1.3", Enable: true}}

	// Mock the behavior of the IsHealthy method
	backend1.On("IsHealthy").Return(true)
	backend2.On("IsHealthy").Return(true)
	backend3.On("IsHealthy").Return(true)

	// Create a record
	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "random",
		Backends: []BackendInterface{backend1, backend2, backend3},
	}

	// Create the GSLB object
	g := &GSLB{}

	// Perform the random selection multiple times
	selectedIPs := make(map[string]bool)
	for i := 0; i < 10; i++ {
		ip, err := g.pickBackendWithRandom(record, dns.TypeA)
		assert.NoError(t, err, "Expected pickBackendWithRandom to succeed")
		selectedIPs[ip] = true
	}

	// Assert that the IPs are from the healthy backends
	assert.GreaterOrEqual(t, len(selectedIPs), 2, "Expected at least two different backends to be selected randomly")
	assert.Contains(t, selectedIPs, "192.168.1.1", "Expected IP 192.168.1.1 to be selected")
	assert.Contains(t, selectedIPs, "192.168.1.2", "Expected IP 192.168.1.2 to be selected")
	assert.Contains(t, selectedIPs, "192.168.1.3", "Expected IP 192.168.1.3 to be selected")
}
