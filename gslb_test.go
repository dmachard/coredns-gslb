package gslb

import (
	"context"
	"net"
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
	ipAddresses, err := g.pickBackendWithFailover(record, dns.TypeA)

	// Assert the results
	assert.NoError(t, err, "Expected pickFailoverBackend to succeed")
	assert.Equal(t, "192.168.1.1", ipAddresses[0], "Expected the healthy backend to be selected")
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
	ipAddresses, err := g.pickBackendWithFailover(record, dns.TypeAAAA)

	// Assert the results
	assert.NoError(t, err, "Expected pickFailoverBackend to succeed")
	assert.Equal(t, "2001:db8::1", ipAddresses[0], "Expected the healthy backend to be selected")
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
	ipAddresses, err := g.pickBackendWithRoundRobin("example.com.", record, dns.TypeA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "192.168.1.1", ipAddresses[0], "Expected the first backend to be selected")

	// Perform the second selection; index should be 1
	ipAddresses, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "192.168.1.2", ipAddresses[0], "Expected the second backend to be selected")

	// Perform the third selection; index should be 2
	ipAddresses, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "192.168.1.3", ipAddresses[0], "Expected the third backend to be selected")

	// Perform the fourth selection; index should wrap back to 0
	ipAddresses, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "192.168.1.1", ipAddresses[0], "Expected the first backend to be selected again")
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
	ipAddresses, err := g.pickBackendWithRoundRobin("example.com.", record, dns.TypeAAAA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "2001:db8::1", ipAddresses[0], "Expected the first IPv6 backend to be selected")

	// Perform the second selection; index should be 1
	ipAddresses, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeAAAA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "2001:db8::2", ipAddresses[0], "Expected the second IPv6 backend to be selected")

	// Perform the third selection; index should be 2
	ipAddresses, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeAAAA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "2001:db8::3", ipAddresses[0], "Expected the third IPv6 backend to be selected")

	// Perform the fourth selection; index should wrap back to 0
	ipAddresses, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeAAAA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "2001:db8::1", ipAddresses[0], "Expected the first IPv6 backend to be selected again")
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
		ipAddresses, err := g.pickBackendWithRandom(record, dns.TypeA)
		assert.NoError(t, err, "Expected pickBackendWithRandom to succeed")
		for _, ip := range ipAddresses {
			selectedIPs[ip] = true
		}
	}

	// Assert that the IPs are from the healthy backends
	assert.GreaterOrEqual(t, len(selectedIPs), 2, "Expected at least two different backends to be selected randomly")
	assert.Contains(t, selectedIPs, "192.168.1.1", "Expected IP 192.168.1.1 to be selected")
	assert.Contains(t, selectedIPs, "192.168.1.2", "Expected IP 192.168.1.2 to be selected")
	assert.Contains(t, selectedIPs, "192.168.1.3", "Expected IP 192.168.1.3 to be selected")
}

func TestGSLB_PickBackendWithFailover_MultipleSamePriority(t *testing.T) {
	// Deux backends healthy, même priorité
	backendHealthy1 := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true, Priority: 10}}
	backendHealthy2 := &MockBackend{Backend: &Backend{Address: "192.168.1.2", Enable: true, Priority: 10}}
	backendUnhealthy := &MockBackend{Backend: &Backend{Address: "192.168.1.3", Enable: true, Priority: 20}}

	backendHealthy1.On("IsHealthy").Return(true)
	backendHealthy2.On("IsHealthy").Return(true)
	backendUnhealthy.On("IsHealthy").Return(false)

	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "failover",
		Backends: []BackendInterface{backendHealthy1, backendHealthy2, backendUnhealthy},
	}

	g := &GSLB{}

	ipAddresses, err := g.pickBackendWithFailover(record, dns.TypeA)

	assert.NoError(t, err, "Expected pickBackendWithFailover to succeed")
	assert.Len(t, ipAddresses, 2, "Expected two healthy backends of same priority to be returned")
	assert.Contains(t, ipAddresses, "192.168.1.1")
	assert.Contains(t, ipAddresses, "192.168.1.2")
}

func TestGSLB_HandleTXTRecord(t *testing.T) {
	// Create mock backends
	backend1 := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true, Priority: 10}}
	backend2 := &MockBackend{Backend: &Backend{Address: "192.168.1.2", Enable: false, Priority: 20}}
	backend1.On("IsHealthy").Return(true)
	backend2.On("IsHealthy").Return(false)

	record := &Record{
		Fqdn:      "example.com.",
		Mode:      "failover",
		Backends:  []BackendInterface{backend1, backend2},
		RecordTTL: 60,
	}

	g := &GSLB{
		Records: map[string]*Record{"example.com.": record},
	}

	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeTXT)
	w := &TestResponseWriter{}

	code, err := g.handleTXTRecord(context.Background(), w, msg, "example.com.")
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.NotEmpty(t, w.Msg.Answer)

	// Check that the TXT records contain backend info
	found1, found2 := false, false
	for _, rr := range w.Msg.Answer {
		if txt, ok := rr.(*dns.TXT); ok {
			if txt.Txt[0] == "Backend: 192.168.1.1 | Priority: 10 | Status: healthy | Enabled: true" {
				found1 = true
			}
			if txt.Txt[0] == "Backend: 192.168.1.2 | Priority: 20 | Status: unhealthy | Enabled: false" {
				found2 = true
			}
		}
	}
	assert.True(t, found1, "Expected TXT record for backend1")
	assert.True(t, found2, "Expected TXT record for backend2")
}

// TestResponseWriter is a mock dns.ResponseWriter for testing
// It captures the DNS message sent by WriteMsg

type TestResponseWriter struct {
	Msg *dns.Msg
}

func (w *TestResponseWriter) WriteMsg(m *dns.Msg) error {
	w.Msg = m
	return nil
}
func (w *TestResponseWriter) LocalAddr() net.Addr       { return nil }
func (w *TestResponseWriter) RemoteAddr() net.Addr      { return nil }
func (w *TestResponseWriter) Close() error              { return nil }
func (w *TestResponseWriter) TsigStatus() error         { return nil }
func (w *TestResponseWriter) TsigTimersOnly(bool)       {}
func (w *TestResponseWriter) Hijack()                   {}
func (w *TestResponseWriter) Write([]byte) (int, error) { return 0, nil }
