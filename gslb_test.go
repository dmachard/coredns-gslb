package gslb

import (
	"context"
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
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

func TestGSLB_PickBackendWithGeoIP_CustomDB(t *testing.T) {
	locationMap := map[string]string{
		"10.0.0.0/24":    "eu-west",
		"192.168.1.0/24": "us-east",
	}

	backendEU := &MockBackend{Backend: &Backend{Address: "10.0.0.42", Enable: true, Priority: 10, CustomLocations: []string{"eu-west"}}}
	backendUS := &MockBackend{Backend: &Backend{Address: "192.168.1.42", Enable: true, Priority: 20, CustomLocations: []string{"us-east"}}}
	backendOther := &MockBackend{Backend: &Backend{Address: "172.16.0.1", Enable: true, Priority: 30, CustomLocations: []string{"other"}}}
	backendEU.On("IsHealthy").Return(true)
	backendUS.On("IsHealthy").Return(true)
	backendOther.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendEU, backendUS, backendOther},
	}

	g := &GSLB{
		LocationMap: locationMap,
	}

	testCases := []struct {
		name     string
		clientIP string
		expect   []string
	}{
		{"us-east subnet", "192.168.1.50", []string{"192.168.1.42"}},
		{"eu-west subnet", "10.0.0.50", []string{"10.0.0.42"}},
		{"us-east subnet 2", "192.168.1.100", []string{"192.168.1.42"}},
		{"eu-west subnet 2", "10.0.0.200", []string{"10.0.0.42"}},
		{"unmatched IP fallback", "8.8.8.8", []string{"10.0.0.42"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP(tc.clientIP))
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, ips)
		})
	}

	// Test fallback when LocationMap is nil
	g.LocationMap = nil
	t.Run("fallback no location map", func(t *testing.T) {
		ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP("8.8.8.8"))
		assert.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.42"}, ips)
	})
}

func TestGSLB_PickBackendWithGeoIP_Country_MaxMind(t *testing.T) {
	db, err := geoip2.Open("coredns/GeoLite2-Country.mmdb")
	if err != nil {
		t.Skip("GeoLite2-Country.mmdb not found, skipping real MaxMind test")
	}
	defer db.Close()

	backendUS := &MockBackend{Backend: &Backend{Address: "20.0.0.1", Enable: true, Priority: 10, Countries: []string{"US"}}}
	backendAU := &MockBackend{Backend: &Backend{Address: "30.0.0.1", Enable: true, Priority: 20, Countries: []string{"AU"}}}
	backendOther := &MockBackend{Backend: &Backend{Address: "40.0.0.1", Enable: true, Priority: 30, Countries: []string{"DE"}}}
	backendUS.On("IsHealthy").Return(true)
	backendAU.On("IsHealthy").Return(true)
	backendOther.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendUS, backendAU, backendOther},
	}

	g := &GSLB{
		GeoIPCountryDB: db,
	}

	testCases := []struct {
		name     string
		clientIP string
		expect   []string
	}{
		{"US IP", "8.8.8.8", []string{"20.0.0.1"}},
		{"AU IP", "1.144.110.23", []string{"30.0.0.1"}},
		{"Unknown country fallback", "127.0.0.1", []string{"20.0.0.1"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP(tc.clientIP))
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, ips)
		})
	}
}

func TestGSLB_PickBackendWithGeoIP_City_MaxMind(t *testing.T) {
	db, err := geoip2.Open("coredns/GeoLite2-City.mmdb")
	if err != nil {
		t.Skip("GeoLite2-City.mmdb not found, skipping real MaxMind city test")
	}
	defer db.Close()

	backendParis := &MockBackend{Backend: &Backend{Address: "10.10.10.1", Enable: true, Priority: 10, Cities: []string{"Paris"}}}
	backendBerlin := &MockBackend{Backend: &Backend{Address: "20.20.20.1", Enable: true, Priority: 20, Cities: []string{"Berlin"}}}
	backendOther := &MockBackend{Backend: &Backend{Address: "30.30.30.1", Enable: true, Priority: 30, Cities: []string{"OtherCity"}}}
	backendParis.On("IsHealthy").Return(true)
	backendBerlin.On("IsHealthy").Return(true)
	backendOther.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendParis, backendBerlin, backendOther},
	}

	g := &GSLB{
		GeoIPCityDB: db,
	}

	testCases := []struct {
		name     string
		clientIP string
		expect   []string
	}{
		{"Paris IP", "81.185.159.80", []string{"10.10.10.1"}},        // IP in Paris
		{"Berlin IP", "141.20.20.1", []string{"20.20.20.1"}},         // IP in Berlin
		{"Unknown city fallback", "8.8.8.8", []string{"10.10.10.1"}}, // fallback to lowest priority
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP(tc.clientIP))
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, ips)
		})
	}
}

func TestGSLB_PickBackendWithGeoIP_ASN_MaxMind(t *testing.T) {
	db, err := geoip2.Open("coredns/GeoLite2-ASN.mmdb")
	if err != nil {
		t.Skip("GeoLite2-ASN.mmdb not found, skipping real MaxMind ASN test")
	}
	defer db.Close()

	backendGoogle := &MockBackend{Backend: &Backend{Address: "8.8.8.8", Enable: true, Priority: 10, ASNs: []uint{15169}}}     // Google ASN
	backendCloudflare := &MockBackend{Backend: &Backend{Address: "1.1.1.1", Enable: true, Priority: 20, ASNs: []uint{13335}}} // Cloudflare ASN
	backendOther := &MockBackend{Backend: &Backend{Address: "9.9.9.9", Enable: true, Priority: 30, ASNs: []uint{0}}}
	backendGoogle.On("IsHealthy").Return(true)
	backendCloudflare.On("IsHealthy").Return(true)
	backendOther.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendGoogle, backendCloudflare, backendOther},
	}

	g := &GSLB{
		GeoIPASNDB: db,
	}

	testCases := []struct {
		name     string
		clientIP string
		expect   []string
	}{
		{"Google ASN IP", "8.8.8.8", []string{"8.8.8.8"}},
		{"Cloudflare ASN IP", "1.1.1.1", []string{"1.1.1.1"}},
		{"Unknown ASN fallback", "9.9.9.9", []string{"8.8.8.8"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP(tc.clientIP))
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, ips)
		})
	}
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
