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
	"gopkg.in/yaml.v3"
)

type mockResponseWriter struct {
	msg *dns.Msg
	ip  net.IP
}

func (m *mockResponseWriter) WriteMsg(msg *dns.Msg) error {
	m.msg = msg
	return nil
}
func (m *mockResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (m *mockResponseWriter) Close() error              { return nil }
func (m *mockResponseWriter) TsigStatus() error         { return nil }
func (m *mockResponseWriter) TsigTimersOnly(bool)       {}
func (m *mockResponseWriter) Hijack()                   {}
func (m *mockResponseWriter) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53}
}
func (m *mockResponseWriter) RemoteAddr() net.Addr {
	ip := m.ip
	if ip == nil {
		ip = net.ParseIP("127.0.0.1")
	}
	return &net.TCPAddr{IP: ip, Port: 12345}
}
func (m *mockResponseWriter) SetReply(*dns.Msg) {}
func (m *mockResponseWriter) Msg() *dns.Msg     { return nil }
func (m *mockResponseWriter) Size() int         { return 512 }
func (m *mockResponseWriter) Scrub(bool)        {}
func (m *mockResponseWriter) WroteMsg()         {}

func TestExtractClientIP_WithECS(t *testing.T) {
	g := &GSLB{UseEDNSCSubnet: true}
	w := &mockResponseWriter{msg: new(dns.Msg)}

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
	w := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("192.168.1.1")}
	r := new(dns.Msg)

	ip, prefixLen := g.extractClientIP(w, r)

	assert.Equal(t, "192.168.1.1", ip.String())
	assert.Equal(t, uint8(32), prefixLen)
}

func TestExtractClientIP_FallbackToRemoteAddr_IPv6(t *testing.T) {
	g := &GSLB{UseEDNSCSubnet: false}
	w := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("2001:db8::1")}
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

// TestServeDNS validates the ServeDNS method for various FQDN cases
func TestServeDNS(t *testing.T) {
	backend := &Backend{Address: "192.168.1.1", Enable: true, Priority: 1}
	record := &Record{
		Fqdn:      "test.example.org.",
		Mode:      "failover",
		Backends:  []BackendInterface{backend},
		RecordTTL: 60,
	}

	testCases := []struct {
		name          string
		fqdn          string
		zone          string
		recordQ       uint16
		expectSuccess bool
	}{
		{"lowercase fqdn, lowercase zone", "test.example.org.", "example.org.", dns.TypeA, true},
		{"uppercase fqdn, lowercase zone", "TEST.EXAMPLE.ORG.", "example.org.", dns.TypeA, true},
		{"mixedcase fqdn, lowercase zone", "Test.Example.Org.", "example.org.", dns.TypeA, true},
		{"fqdn not in zone", "test.otherzone.org.", "example.org.", dns.TypeA, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := &GSLB{
				Zones:   map[string]string{tc.zone: "dummy.yml"},
				Records: map[string]*Record{"test.example.org.": record},
			}
			msg := new(dns.Msg)
			msg.SetQuestion(tc.fqdn, tc.recordQ)
			w := &mockResponseWriter{msg: new(dns.Msg)}
			code, err := g.ServeDNS(context.Background(), w, msg)
			if tc.expectSuccess {
				assert.NoError(t, err)
				assert.Equal(t, dns.RcodeSuccess, code)
			} else {
				assert.Error(t, err)
				assert.Equal(t, 2, code) // plugin.NextOrFailure returns 2 for non-authoritative
			}
		})
	}
}

// Plugin suivant qui capture l'appel pour les tests ServeDNS
type nextPlugin struct{ called bool }

func (n *nextPlugin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	n.called = true
	return dns.RcodeSuccess, nil
}
func (n *nextPlugin) Name() string { return "testnext" }

func TestServeDNS_DisableTXT(t *testing.T) {
	backend := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true, Priority: 10}}
	backend.On("IsHealthy").Return(true)
	record := &Record{
		Fqdn:      "example.com.",
		Mode:      "failover",
		Backends:  []BackendInterface{backend},
		RecordTTL: 60,
	}

	n := &nextPlugin{}
	g := &GSLB{
		Records:    map[string]*Record{"example.com.": record},
		Zones:      map[string]string{"example.com.": "dummy.yml"},
		DisableTXT: true,
		Next:       n,
	}

	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeTXT)
	w := &mockResponseWriter{}
	ctx := context.Background()
	code, err := g.ServeDNS(ctx, w, msg)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.Nil(t, w.msg)
	assert.True(t, n.called, "Next plugin should be called when DisableTXT is true")

	// Test sans DisableTXT
	n.called = false
	g.DisableTXT = false
	code, err = g.ServeDNS(ctx, w, msg)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.NotNil(t, w.msg)
	assert.False(t, n.called, "Next plugin should NOT be called when DisableTXT is false")
}

// Test UnmarshalYAML with healthcheck profiles
func TestGSLB_UnmarshalYAML_WithHealthcheckProfiles(t *testing.T) {
	yamlData := `
healthcheck_profiles:
  http_profile:
    type: http
    params:
      enable_tls: true
      port: 443
      uri: /health
      expected_code: 200
  tcp_profile:
    type: tcp
    params:
      port: 80
      timeout: 5s

records:
  test.example.com.:
    backends:
      - address: 192.168.1.1
        healthchecks: [ http_profile ]
        priority: 1
      - address: 192.168.1.2
        healthchecks: [ http_profile, tcp_profile ]
        priority: 2
    mode: failover
    record_ttl: 30
`

	var gslb GSLB
	err := yaml.Unmarshal([]byte(yamlData), &gslb)
	assert.NoError(t, err)

	// Verify healthcheck profiles were loaded
	assert.NotNil(t, gslb.HealthcheckProfiles)
	assert.Len(t, gslb.HealthcheckProfiles, 2)
	assert.Contains(t, gslb.HealthcheckProfiles, "http_profile")
	assert.Contains(t, gslb.HealthcheckProfiles, "tcp_profile")

	// Verify profiles have correct configuration
	httpProfile := gslb.HealthcheckProfiles["http_profile"]
	assert.Equal(t, "http", httpProfile.Type)
	assert.Equal(t, true, httpProfile.Params["enable_tls"])
	assert.Equal(t, 443, httpProfile.Params["port"])

	tcpProfile := gslb.HealthcheckProfiles["tcp_profile"]
	assert.Equal(t, "tcp", tcpProfile.Type)
	assert.Equal(t, 80, tcpProfile.Params["port"])

	// Verify records were processed correctly
	assert.NotNil(t, gslb.Records)
	assert.Len(t, gslb.Records, 1)

	record := gslb.Records["test.example.com."]
	assert.NotNil(t, record)
	assert.Equal(t, "failover", record.Mode)
	assert.Len(t, record.Backends, 2)

	// Verify backend 1 has 1 healthcheck (http_profile)
	backend1 := record.Backends[0]
	assert.Equal(t, "192.168.1.1", backend1.GetAddress())
	healthchecks1 := backend1.GetHealthChecks()
	assert.Len(t, healthchecks1, 1)
	assert.Equal(t, "https/443", healthchecks1[0].GetType())

	// Verify backend 2 has 2 healthchecks (http_profile + tcp_profile)
	backend2 := record.Backends[1]
	assert.Equal(t, "192.168.1.2", backend2.GetAddress())
	healthchecks2 := backend2.GetHealthChecks()
	assert.Len(t, healthchecks2, 2)
}

// Test processRecordHealthchecks method
func TestGSLB_processRecordHealthchecks(t *testing.T) {
	gslb := &GSLB{
		HealthcheckProfiles: map[string]*HealthCheck{
			"test_profile": {
				Type: "http",
				Params: map[string]interface{}{
					"port": 80,
					"uri":  "/status",
				},
			},
		},
	}

	// Test with valid record data containing profile references
	recordData := map[string]interface{}{
		"mode": "failover",
		"backends": []interface{}{
			map[string]interface{}{
				"address": "1.2.3.4",
				"healthchecks": []interface{}{
					"test_profile",
				},
			},
		},
	}

	processedData, err := gslb.processRecordHealthchecks(recordData)
	assert.NoError(t, err)

	processedRecord := processedData.(map[string]interface{})
	backends := processedRecord["backends"].([]interface{})
	backend := backends[0].(map[string]interface{})
	healthchecks := backend["healthchecks"].([]interface{})

	assert.Len(t, healthchecks, 1)
	hc := healthchecks[0].(map[string]interface{})
	assert.Equal(t, "http", hc["type"])
	assert.Equal(t, map[string]interface{}{"port": 80, "uri": "/status"}, hc["params"])
}

// Test processHealthchecks method
func TestGSLB_processHealthchecks(t *testing.T) {
	gslb := &GSLB{
		HealthcheckProfiles: map[string]*HealthCheck{
			"profile1": {
				Type: "http",
				Params: map[string]interface{}{
					"port": 443,
					"uri":  "/health",
				},
			},
			"profile2": {
				Type: "tcp",
				Params: map[string]interface{}{
					"port":    80,
					"timeout": "5s",
				},
			},
		},
	}

	t.Run("Profile references only", func(t *testing.T) {
		healthchecks := []interface{}{"profile1", "profile2"}

		result, err := gslb.processHealthchecks(healthchecks)
		assert.NoError(t, err)
		assert.Len(t, result, 2)

		// Check first healthcheck
		hc1 := result[0].(map[string]interface{})
		assert.Equal(t, "http", hc1["type"])
		assert.Equal(t, map[string]interface{}{"port": 443, "uri": "/health"}, hc1["params"])

		// Check second healthcheck
		hc2 := result[1].(map[string]interface{})
		assert.Equal(t, "tcp", hc2["type"])
		assert.Equal(t, map[string]interface{}{"port": 80, "timeout": "5s"}, hc2["params"])
	})

	t.Run("Mixed profile references and inline definitions", func(t *testing.T) {
		healthchecks := []interface{}{
			"profile1",
			map[string]interface{}{
				"type": ICMPType,
				"params": map[string]interface{}{
					"count":   3,
					"timeout": "2s",
				},
			},
		}

		result, err := gslb.processHealthchecks(healthchecks)
		assert.NoError(t, err)
		assert.Len(t, result, 2)

		// Check profile reference
		hc1 := result[0].(map[string]interface{})
		assert.Equal(t, "http", hc1["type"])

		// Check inline definition (should be unchanged)
		hc2 := result[1].(map[string]interface{})
		assert.Equal(t, ICMPType, hc2["type"])
		params := hc2["params"].(map[string]interface{})
		assert.Equal(t, 3, params["count"])
	})

	t.Run("Invalid profile reference", func(t *testing.T) {
		healthchecks := []interface{}{"non_existent_profile"}

		result, err := gslb.processHealthchecks(healthchecks)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "healthcheck profile 'non_existent_profile' not found")
	})

	t.Run("No profiles defined", func(t *testing.T) {
		gslbNoProfiles := &GSLB{HealthcheckProfiles: nil}
		healthchecks := []interface{}{"some_profile"}

		result, err := gslbNoProfiles.processHealthchecks(healthchecks)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Invalid healthchecks format", func(t *testing.T) {
		// healthchecks should be an array, not a string
		healthchecks := "invalid_format"

		result, err := gslb.processHealthchecks(healthchecks)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "healthchecks must be an array")
	})
}

// Test UnmarshalYAML error cases
func TestGSLB_UnmarshalYAML_ErrorCases(t *testing.T) {
	t.Run("Invalid YAML", func(t *testing.T) {
		yamlData := `
healthcheck_profiles:
  invalid: [
records:
  test: {}
`
		var gslb GSLB
		err := yaml.Unmarshal([]byte(yamlData), &gslb)
		assert.Error(t, err)
	})

	t.Run("Invalid profile reference in record", func(t *testing.T) {
		yamlData := `
healthcheck_profiles:
  valid_profile:
    type: http
    params:
      port: 80

records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
        healthchecks: [invalid_profile]
    mode: failover
`
		var gslb GSLB
		err := yaml.Unmarshal([]byte(yamlData), &gslb)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "healthcheck profile 'invalid_profile' not found")
	})
}

// Test UnmarshalYAML with no profiles (backward compatibility)
func TestGSLB_UnmarshalYAML_NoProfiles(t *testing.T) {
	yamlData := `
records:
  test.example.com.:
    backends:
      - address: 192.168.1.1
        healthchecks:
          - type: http
            params:
              enable_tls: false
              port: 80
              uri: /health
    mode: failover
`

	var gslb GSLB
	err := yaml.Unmarshal([]byte(yamlData), &gslb)
	assert.NoError(t, err)

	// Should work without profiles
	assert.Nil(t, gslb.HealthcheckProfiles)
	assert.NotNil(t, gslb.Records)
	assert.Len(t, gslb.Records, 1)

	record := gslb.Records["test.example.com."]
	assert.NotNil(t, record)
	assert.Len(t, record.Backends, 1)

	backend := record.Backends[0]
	healthchecks := backend.GetHealthChecks()
	assert.Len(t, healthchecks, 1)
	assert.Equal(t, "http/80", healthchecks[0].GetType())
}

func TestGSLB_RecordsMatchZone(t *testing.T) {
	testCases := []struct {
		name      string
		yamlData  string
		zone      string
		shouldErr bool
	}{
		{
			name: "All records match zone",
			yamlData: `
records:
  valid1.example.org.:
    backends:
      - address: 1.1.1.1
  valid2.example.org.:
    backends:
      - address: 2.2.2.2
`,
			zone:      ".example.org.",
			shouldErr: false,
		},
		{
			name: "One record does not match zone",
			yamlData: `
records:
  valid1.example.org.:
    backends:
      - address: 1.1.1.1
  invalid.example.com.:
    backends:
      - address: 3.3.3.3
`,
			zone:      ".example.org.",
			shouldErr: true,
		},
		{
			name: "All records do not match zone",
			yamlData: `
records:
  invalid1.example.com.:
    backends:
      - address: 4.4.4.4
  invalid2.example.net.:
    backends:
      - address: 5.5.5.5
`,
			zone:      ".example.org.",
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gslb := &GSLB{Zone: tc.zone}
			err := yaml.Unmarshal([]byte(tc.yamlData), gslb)
			if tc.shouldErr {
				assert.Error(t, err, "Expected error for zone mismatch")
			} else {
				assert.NoError(t, err, "Expected no error for matching records")
				for fqdn := range gslb.Records {
					assert.True(t, strings.HasSuffix(fqdn, tc.zone), "Record %s does not match zone %s", fqdn, tc.zone)
				}
			}
		})
	}
}
