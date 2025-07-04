package gslb

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

type mockResponseWriter struct {
	addr net.Addr
}

func (m *mockResponseWriter) WriteMsg(*dns.Msg) error   { return nil }
func (m *mockResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (m *mockResponseWriter) Close() error              { return nil }
func (m *mockResponseWriter) TsigStatus() error         { return nil }
func (m *mockResponseWriter) TsigTimersOnly(bool)       {}
func (m *mockResponseWriter) Hijack()                   {}
func (m *mockResponseWriter) LocalAddr() net.Addr       { return nil }
func (m *mockResponseWriter) RemoteAddr() net.Addr      { return m.addr }
func (m *mockResponseWriter) SetReply(*dns.Msg)         {}
func (m *mockResponseWriter) Msg() *dns.Msg             { return nil }
func (m *mockResponseWriter) Size() int                 { return 512 }
func (m *mockResponseWriter) Scrub(bool)                {}
func (m *mockResponseWriter) WroteMsg()                 {}

func TestExtractClientIP_WithECS(t *testing.T) {
	g := &GSLB{UseEDNSCSubnet: true}
	w := &mockResponseWriter{addr: &net.UDPAddr{IP: net.ParseIP("9.9.9.9"), Port: 53}}

	// Create a DNS message with ECS option
	r := new(dns.Msg)
	r.SetQuestion("example.com.", dns.TypeA)
	o := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	ecs := &dns.EDNS0_SUBNET{
		Code:          dns.EDNS0SUBNET,
		Address:       net.ParseIP("1.2.3.4"),
		SourceNetmask: 24,
		Family:        1,
	}
	o.Option = append(o.Option, ecs)
	r.Extra = append(r.Extra, o)

	ip, prefixLen := g.extractClientIP(w, r)

	assert.Equal(t, "1.2.3.4", ip.String())
	assert.Equal(t, uint8(24), prefixLen)
}

func TestExtractClientIP_FallbackToRemoteAddr_IPv4(t *testing.T) {
	g := &GSLB{UseEDNSCSubnet: false}
	w := &mockResponseWriter{addr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 53}}
	r := new(dns.Msg)

	ip, prefixLen := g.extractClientIP(w, r)

	assert.Equal(t, "192.168.1.1", ip.String())
	assert.Equal(t, uint8(32), prefixLen)
}

func TestExtractClientIP_FallbackToRemoteAddr_IPv6(t *testing.T) {
	g := &GSLB{UseEDNSCSubnet: false}
	w := &mockResponseWriter{addr: &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 53}}
	r := new(dns.Msg)

	ip, prefixLen := g.extractClientIP(w, r)

	assert.Equal(t, "2001:db8::1", ip.String())
	assert.Equal(t, uint8(128), prefixLen)
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

	// Use a dummy client IP and prefix for TXT record test
	clientIP := net.ParseIP("192.168.1.1")
	clientPrefixLen := uint8(32)
	ctx := WithClientInfo(context.Background(), clientIP, clientPrefixLen)
	code, err := g.handleTXTRecord(ctx, w, msg, "example.com.")
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

func TestGetResolutionIdleTimeout_WithCustomValue(t *testing.T) {
	r := &GSLB{
		ResolutionIdleTimeout: "100s",
	}

	timeout := r.GetResolutionIdleTimeout()

	assert.Equal(t, 100*time.Second, timeout)
}

func TestGetResolutionIdleTimeout_DefaultValue(t *testing.T) {
	r := &GSLB{}

	timeout := r.GetResolutionIdleTimeout()

	assert.Equal(t, 3600*time.Second, timeout)
}

func TestLoadCustomLocationMap(t *testing.T) {
	// Create a temporary YAML file for the location map
	tmpFile, err := os.CreateTemp("", "location_map_test_*.yml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `subnets:
  - subnet: "192.168.0.0/16"
    location: "eu-west-1"
  - subnet: "10.0.0.0/8"
    location: "us-east-1"
`
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	g := &GSLB{}
	err = g.loadCustomLocationsMap(tmpFile.Name())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if g.LocationMap["192.168.0.0/16"] != "eu-west-1" {
		t.Errorf("Expected eu-west-1, got %v", g.LocationMap["192.168.0.0/16"])
	}
	if g.LocationMap["10.0.0.0/8"] != "us-east-1" {
		t.Errorf("Expected us-east-1, got %v", g.LocationMap["10.0.0.0/8"])
	}
}

func TestLoadLocationMap_FileNotFound(t *testing.T) {
	g := &GSLB{}
	err := g.loadCustomLocationsMap("/nonexistent/location_map.yml")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestLoadLocationMap_EmptyPath(t *testing.T) {
	g := &GSLB{}
	err := g.loadCustomLocationsMap("")
	if err != nil {
		t.Errorf("Expected no error for empty path, got: %v", err)
	}
	if g.LocationMap != nil {
		t.Errorf("Expected LocationMap to be nil for empty path")
	}
}
