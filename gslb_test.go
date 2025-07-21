package gslb

import (
	"context"
	"net"
	"os"
	"strings"
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
			if strings.Contains(txt.Txt[0], "Backend: 192.168.1.1") &&
				strings.Contains(txt.Txt[0], "Priority: 10") &&
				strings.Contains(txt.Txt[0], "Status: healthy") &&
				strings.Contains(txt.Txt[0], "Enabled: true") &&
				strings.Contains(txt.Txt[0], "LastHealthcheck:") {
				found1 = true
			}
			if strings.Contains(txt.Txt[0], "Backend: 192.168.1.2") &&
				strings.Contains(txt.Txt[0], "Priority: 20") &&
				strings.Contains(txt.Txt[0], "Status: unhealthy") &&
				strings.Contains(txt.Txt[0], "Enabled: false") &&
				strings.Contains(txt.Txt[0], "LastHealthcheck:") {
				found2 = true
			}
		}
	}
	assert.True(t, found1, "Expected TXT record for backend1 with LastHealthcheck")
	assert.True(t, found2, "Expected TXT record for backend2 with LastHealthcheck")
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

func TestGSLB_IsAuthoritative(t *testing.T) {
	g := &GSLB{
		Zones: map[string]string{
			"example.com.": "",
		},
	}
	assert.True(t, g.isAuthoritative("foo.example.com."))
	assert.False(t, g.isAuthoritative("bar.other.com."))
}

func TestGSLB_UpdateLastResolutionTime(t *testing.T) {
	g := &GSLB{}
	domain := "test.example.com."
	g.updateLastResolutionTime(domain)
	v, ok := g.LastResolution.Load(domain)
	assert.True(t, ok)
	timeVal, ok := v.(time.Time)
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now(), timeVal, time.Second)
}

func TestGSLB_Name(t *testing.T) {
	g := &GSLB{}
	assert.Equal(t, "gslb", g.Name())
}

func TestGSLB_SendAddressRecordResponse(t *testing.T) {
	g := &GSLB{}

	// Create a mock DNS message
	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeA)

	// Create a mock response writer
	w := &TestResponseWriter{}

	// Test A record response
	ipAddresses := []string{"192.168.1.1", "192.168.1.2"}
	code, err := g.sendAddressRecordResponse(w, msg, "example.com.", ipAddresses, 30, dns.TypeA)

	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.NotNil(t, w.Msg)
	assert.Len(t, w.Msg.Answer, 2)

	// Verify A records
	for i, rr := range w.Msg.Answer {
		if a, ok := rr.(*dns.A); ok {
			assert.Equal(t, "example.com.", a.Hdr.Name)
			assert.Equal(t, dns.TypeA, a.Hdr.Rrtype)
			assert.Equal(t, uint32(30), a.Hdr.Ttl)
			assert.Equal(t, ipAddresses[i], a.A.String())
		}
	}

	// Test AAAA record response
	msgAAAA := new(dns.Msg)
	msgAAAA.SetQuestion("example.com.", dns.TypeAAAA)
	wAAAA := &TestResponseWriter{}

	ipv6Addresses := []string{"2001:db8::1", "2001:db8::2"}
	codeAAAA, errAAAA := g.sendAddressRecordResponse(wAAAA, msgAAAA, "example.com.", ipv6Addresses, 60, dns.TypeAAAA)

	assert.NoError(t, errAAAA)
	assert.Equal(t, dns.RcodeSuccess, codeAAAA)
	assert.NotNil(t, wAAAA.Msg)
	assert.Len(t, wAAAA.Msg.Answer, 2)

	// Verify AAAA records
	for i, rr := range wAAAA.Msg.Answer {
		if aaaa, ok := rr.(*dns.AAAA); ok {
			assert.Equal(t, "example.com.", aaaa.Hdr.Name)
			assert.Equal(t, dns.TypeAAAA, aaaa.Hdr.Rrtype)
			assert.Equal(t, uint32(60), aaaa.Hdr.Ttl)
			assert.Equal(t, ipv6Addresses[i], aaaa.AAAA.String())
		}
	}
}
