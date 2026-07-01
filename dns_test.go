package gslb

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
	"github.com/stretchr/testify/assert"
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
		Records: make(map[string]map[string]*Record),
	}
	g.Records["example.com."] = make(map[string]*Record)
	g.Records["example.com."]["example.com."] = record

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
		Records: make(map[string]map[string]*Record),
	}
	g.Records["example.com."] = make(map[string]*Record)
	g.Records["example.com."]["example.com."] = record

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
		Records: make(map[string]map[string]*Record),
	}
	g.Records["example.com."] = make(map[string]*Record)
	g.Records["example.com."]["example.com."] = record

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
		Records: make(map[string]map[string]*Record),
	}
	g.Records["example.com."] = make(map[string]*Record)
	g.Records["example.com."]["example.com."] = record

	// Test the pickAll method
	ipAddresses, err := g.pickAllAddresses("example.com.", dns.TypeA)

	// Assert the results
	assert.Error(t, err, "Expected an error when no backends exist")
	assert.EqualError(t, err, "no backends exist for domain: example.com.", "Expected specific error message")
	assert.Nil(t, ipAddresses, "Expected no IP addresses to be returned")
}

func TestGSLB_PickAllAddresses_UnknownDomain(t *testing.T) {
	g := &GSLB{
		Records: make(map[string]map[string]*Record),
	}
	g.Records["example.com."] = make(map[string]*Record)

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
		Records: map[string]map[string]*Record{"example.com.": {"example.com.": record}},
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

func TestGSLB_IsAuthoritative(t *testing.T) {
	g := &GSLB{
		Zones: map[string]string{
			"example.com.": "",
		},
	}
	assert.True(t, g.isAuthoritative("foo.example.com."))
	assert.False(t, g.isAuthoritative("bar.other.com."))
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
				Records: map[string]map[string]*Record{"test.example.org.": {"test.example.org.": record}},
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

func TestServeDNS_IPHash(t *testing.T) {
	backend1 := &Backend{Address: "192.168.1.1", Enable: true, Alive: true}
	backend2 := &Backend{Address: "192.168.1.2", Enable: true, Alive: true}
	record := &Record{
		Fqdn:      "hash.example.org.",
		Mode:      "ip-hash",
		Backends:  []BackendInterface{backend1, backend2},
		RecordTTL: 60,
	}

	g := &GSLB{
		Zones:   map[string]string{"example.org.": "dummy.yml"},
		Records: map[string]map[string]*Record{"example.org.": {"hash.example.org.": record}},
	}

	msg := new(dns.Msg)
	msg.SetQuestion("hash.example.org.", dns.TypeA)
	w := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("192.168.100.1")}

	code, err := g.ServeDNS(context.Background(), w, msg)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.NotNil(t, w.msg)
	assert.NotEmpty(t, w.msg.Answer)

	// Verify that the returned IP is one of the backends
	aRecord, ok := w.msg.Answer[0].(*dns.A)
	assert.True(t, ok)
	returnedIP := aRecord.A.String()
	assert.Contains(t, []string{"192.168.1.1", "192.168.1.2"}, returnedIP)
}

// Plugin following which captures the call for tests ServeDNS
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
		Records:    map[string]map[string]*Record{"example.com.": {"example.com.": record}},
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

	// Test without DisableTXT
	n.called = false
	g.DisableTXT = false
	code, err = g.ServeDNS(ctx, w, msg)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.NotNil(t, w.msg)
	assert.False(t, n.called, "Next plugin should NOT be called when DisableTXT is false")
}

func TestGSLB_WildcardRecordMatching(t *testing.T) {
	backendWildcard := &Backend{Address: "192.168.1.1", Enable: true, Priority: 1}
	recordWildcard := &Record{
		Fqdn:      "*.example.com.",
		Mode:      "failover",
		Backends:  []BackendInterface{backendWildcard},
		RecordTTL: 60,
	}

	backendSpecific := &Backend{Address: "192.168.1.2", Enable: true, Priority: 1}
	recordSpecific := &Record{
		Fqdn:      "specific.example.com.",
		Mode:      "failover",
		Backends:  []BackendInterface{backendSpecific},
		RecordTTL: 60,
	}

	g := &GSLB{
		Zones: map[string]string{
			"example.com.": "dummy.yml",
		},
		Records: map[string]map[string]*Record{
			"example.com.": {
				"*.example.com.":        recordWildcard,
				"specific.example.com.": recordSpecific,
			},
		},
	}

	t.Run("Exact match wins over wildcard", func(t *testing.T) {
		rec, zone := g.findRecord("specific.example.com.")
		assert.Equal(t, "example.com.", zone)
		assert.NotNil(t, rec)
		assert.Equal(t, "specific.example.com.", rec.Fqdn)
	})

	t.Run("Wildcard match for single-label subdomain", func(t *testing.T) {
		rec, zone := g.findRecord("anything.example.com.")
		assert.Equal(t, "example.com.", zone)
		assert.NotNil(t, rec)
		assert.Equal(t, "*.example.com.", rec.Fqdn)
	})

	t.Run("Wildcard match for multi-label subdomain", func(t *testing.T) {
		rec, zone := g.findRecord("sub.anything.example.com.")
		assert.Equal(t, "example.com.", zone)
		assert.NotNil(t, rec)
		assert.Equal(t, "*.example.com.", rec.Fqdn)
	})

	t.Run("No match for non-existent domain outside zone", func(t *testing.T) {
		rec, _ := g.findRecord("other.com.")
		assert.Nil(t, rec)
	})
}

func TestGSLB_FailoverPolicy(t *testing.T) {
	// 1. Setup mock backends that are unhealthy
	backend1 := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true}}
	backend1.On("IsHealthy").Return(false)

	// 2. Setup GSLB and different records to test policies
	g := &GSLB{
		Zones:   map[string]string{"example.org.": "dummy.yml"},
		Records: make(map[string]map[string]*Record),
	}
	g.Records["example.org."] = make(map[string]*Record)

	// A: Record with default fail-open
	g.Records["example.org."]["fail-open.example.org."] = &Record{
		Fqdn:      "fail-open.example.org.",
		Mode:      "failover",
		Backends:  []BackendInterface{backend1},
		RecordTTL: 60,
	}

	// B: Record with fail-closed and NXDOMAIN
	g.Records["example.org."]["fail-closed-nxdomain.example.org."] = &Record{
		Fqdn:      "fail-closed-nxdomain.example.org.",
		Mode:      "failover",
		Backends:  []BackendInterface{backend1},
		RecordTTL: 60,
		FailoverPolicy: FailoverPolicy{
			Mode:  "fail-closed",
			Rcode: "NXDOMAIN",
		},
	}

	// C: Record with fail-closed and REFUSED
	g.Records["example.org."]["fail-closed-refused.example.org."] = &Record{
		Fqdn:      "fail-closed-refused.example.org.",
		Mode:      "failover",
		Backends:  []BackendInterface{backend1},
		RecordTTL: 60,
		FailoverPolicy: FailoverPolicy{
			Mode:  "fail-closed",
			Rcode: "REFUSED",
		},
	}

	// D: Record with fail-specific and fallback IPs
	g.Records["example.org."]["fail-specific.example.org."] = &Record{
		Fqdn:      "fail-specific.example.org.",
		Mode:      "failover",
		Backends:  []BackendInterface{backend1},
		RecordTTL: 60,
		FailoverPolicy: FailoverPolicy{
			Mode:        "fail-specific",
			FallbackIPs: []string{"1.2.3.4", "5.6.7.8"},
		},
	}

	// E: Record with fail-specific and fallback CNAME
	g.Records["example.org."]["fail-specific-cname.example.org."] = &Record{
		Fqdn:      "fail-specific-cname.example.org.",
		Mode:      "failover",
		Backends:  []BackendInterface{backend1},
		RecordTTL: 60,
		FailoverPolicy: FailoverPolicy{
			Mode:          "fail-specific",
			FallbackCNAME: "backup.cdn.cloudflare.net",
		},
	}

	// Run Test cases
	t.Run("fail-open (default) returns all enabled backends", func(t *testing.T) {
		msg := new(dns.Msg)
		msg.SetQuestion("fail-open.example.org.", dns.TypeA)
		w := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
		code, err := g.ServeDNS(context.Background(), w, msg)
		assert.NoError(t, err)
		assert.Equal(t, dns.RcodeSuccess, code)
		assert.Len(t, w.msg.Answer, 1)
		assert.Equal(t, "192.168.1.1", w.msg.Answer[0].(*dns.A).A.String())
	})

	t.Run("fail-closed with NXDOMAIN returns NameError and empty answers", func(t *testing.T) {
		msg := new(dns.Msg)
		msg.SetQuestion("fail-closed-nxdomain.example.org.", dns.TypeA)
		w := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
		code, err := g.ServeDNS(context.Background(), w, msg)
		assert.NoError(t, err)
		assert.Equal(t, dns.RcodeSuccess, code) // ServeDNS returns RcodeSuccess because it wrote the response successfully
		assert.Equal(t, dns.RcodeNameError, w.msg.Rcode)
		assert.Len(t, w.msg.Answer, 0)
	})

	t.Run("fail-closed with REFUSED returns Refused and empty answers", func(t *testing.T) {
		msg := new(dns.Msg)
		msg.SetQuestion("fail-closed-refused.example.org.", dns.TypeA)
		w := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
		code, err := g.ServeDNS(context.Background(), w, msg)
		assert.NoError(t, err)
		assert.Equal(t, dns.RcodeSuccess, code)
		assert.Equal(t, dns.RcodeRefused, w.msg.Rcode)
		assert.Len(t, w.msg.Answer, 0)
	})

	t.Run("fail-specific returns fallback IPs", func(t *testing.T) {
		msg := new(dns.Msg)
		msg.SetQuestion("fail-specific.example.org.", dns.TypeA)
		w := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
		code, err := g.ServeDNS(context.Background(), w, msg)
		assert.NoError(t, err)
		assert.Equal(t, dns.RcodeSuccess, code)
		assert.Len(t, w.msg.Answer, 2)
		assert.Equal(t, "1.2.3.4", w.msg.Answer[0].(*dns.A).A.String())
		assert.Equal(t, "5.6.7.8", w.msg.Answer[1].(*dns.A).A.String())
	})

	t.Run("fail-specific returns fallback CNAME for A/AAAA/SVCB/HTTPS queries", func(t *testing.T) {
		// A query
		msgA := new(dns.Msg)
		msgA.SetQuestion("fail-specific-cname.example.org.", dns.TypeA)
		wA := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
		codeA, errA := g.ServeDNS(context.Background(), wA, msgA)
		assert.NoError(t, errA)
		assert.Equal(t, dns.RcodeSuccess, codeA)
		assert.Len(t, wA.msg.Answer, 1)
		assert.Equal(t, dns.TypeCNAME, wA.msg.Answer[0].Header().Rrtype)
		assert.Equal(t, "backup.cdn.cloudflare.net.", wA.msg.Answer[0].(*dns.CNAME).Target)

		// AAAA query
		msgAAAA := new(dns.Msg)
		msgAAAA.SetQuestion("fail-specific-cname.example.org.", dns.TypeAAAA)
		wAAAA := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
		codeAAAA, errAAAA := g.ServeDNS(context.Background(), wAAAA, msgAAAA)
		assert.NoError(t, errAAAA)
		assert.Equal(t, dns.RcodeSuccess, codeAAAA)
		assert.Len(t, wAAAA.msg.Answer, 1)
		assert.Equal(t, dns.TypeCNAME, wAAAA.msg.Answer[0].Header().Rrtype)
		assert.Equal(t, "backup.cdn.cloudflare.net.", wAAAA.msg.Answer[0].(*dns.CNAME).Target)

		// SVCB query
		msgSVCB := new(dns.Msg)
		msgSVCB.SetQuestion("fail-specific-cname.example.org.", dns.TypeSVCB)
		wSVCB := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
		codeSVCB, errSVCB := g.ServeDNS(context.Background(), wSVCB, msgSVCB)
		assert.NoError(t, errSVCB)
		assert.Equal(t, dns.RcodeSuccess, codeSVCB)
		assert.Len(t, wSVCB.msg.Answer, 1)
		assert.Equal(t, dns.TypeCNAME, wSVCB.msg.Answer[0].Header().Rrtype)
		assert.Equal(t, "backup.cdn.cloudflare.net.", wSVCB.msg.Answer[0].(*dns.CNAME).Target)
	})
}

func TestServeDNS_GeoIPAffinity(t *testing.T) {
	db, err := geoip2.Open("tests/GeoLite2-City.mmdb")
	if err != nil {
		t.Skip("GeoLite2-City.mmdb not found, skipping TestServeDNS_GeoIPAffinity")
	}
	defer db.Close()

	backend := &Backend{
		Address: "192.168.1.10", Enable: true, Priority: 1, Country: "FR",
		Latitude: 48.8566, Longitude: 2.3522, HasCoordinates: true,
	}
	backend.recomputeCoordinateRadians()

	record := &Record{
		Fqdn:      "geo-affinity.example.org.",
		Mode:      "geoip_affinity",
		Backends:  []BackendInterface{backend},
		RecordTTL: 60,
	}

	g := &GSLB{
		Zones:       map[string]string{"example.org.": "dummy.yml"},
		Records:     map[string]map[string]*Record{"geo-affinity.example.org.": {"geo-affinity.example.org.": record}},
		GeoIPCityDB: db,
	}

	msg := new(dns.Msg)
	msg.SetQuestion("geo-affinity.example.org.", dns.TypeA)
	w := &mockResponseWriter{msg: new(dns.Msg)}

	// Simulate client IP from Paris (81.185.159.80) using WithClientInfo context
	ctx := WithClientInfo(context.Background(), net.ParseIP("81.185.159.80"), 32)
	code, err := g.ServeDNS(ctx, w, msg)

	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.NotNil(t, w.msg)
	assert.Len(t, w.msg.Answer, 1)
	if len(w.msg.Answer) > 0 {
		if a, ok := w.msg.Answer[0].(*dns.A); ok {
			assert.Equal(t, "192.168.1.10", a.A.String())
		} else {
			t.Errorf("expected A record, got %T", w.msg.Answer[0])
		}
	}
}

func TestServeDNS_UnsupportedRecordTypeReturnsNoData(t *testing.T) {
	// Create a record with IPv4-only backend (no AAAA)
	backend := &Backend{Address: "192.168.1.1", Enable: true, Priority: 1}
	record := &Record{
		Fqdn:      "ipv4-only.example.org.",
		Mode:      "failover",
		Backends:  []BackendInterface{backend},
		RecordTTL: 60,
	}

	g := &GSLB{
		Zones:   map[string]string{"example.org.": "dummy.yml"},
		Records: map[string]map[string]*Record{"ipv4-only.example.org.": {"ipv4-only.example.org.": record}},
	}

	// Query for AAAA
	msg := new(dns.Msg)
	msg.SetQuestion("ipv4-only.example.org.", dns.TypeAAAA)
	w := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}

	code, err := g.ServeDNS(context.Background(), w, msg)
	assert.NoError(t, err)
	// We expect NOERROR (dns.RcodeSuccess) and empty Answer
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.Equal(t, dns.RcodeSuccess, w.msg.Rcode)
	assert.Len(t, w.msg.Answer, 0)
}

func TestGSLB_ECSEchoing(t *testing.T) {
	// Create mock backends
	backend1 := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true, Priority: 1}}
	backend1.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:      "example.com.",
		Mode:      "geoip", // GeoIP mode means geo-specific
		Backends:  []BackendInterface{backend1},
		RecordTTL: 60,
	}

	g := &GSLB{
		UseEDNSCSubnet: true,
		Records:        map[string]map[string]*Record{"example.com.": {"example.com.": record}},
		Zones:          map[string]string{"example.com.": "dummy.yml"},
	}

	// Create DNS request with ECS option
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

	w := &TestResponseWriter{}
	ctx := context.Background()

	// Call ServeDNS
	code, err := g.ServeDNS(ctx, w, r)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.NotNil(t, w.Msg)

	// Verify that the response contains the EDNS0 option with expected scope
	respOpt := w.Msg.IsEdns0()
	assert.NotNil(t, respOpt, "Response should have EDNS0")
	var respEcs *dns.EDNS0_SUBNET
	for _, opt := range respOpt.Option {
		if sub, ok := opt.(*dns.EDNS0_SUBNET); ok {
			respEcs = sub
			break
		}
	}
	assert.NotNil(t, respEcs, "Response should have EDNS0 subnet option")
	assert.Equal(t, uint8(24), respEcs.SourceNetmask)
	assert.Equal(t, uint8(24), respEcs.SourceScope, "Scope netmask should match Source netmask in geo mode")
	assert.Equal(t, "1.2.3.4", respEcs.Address.String())
}

func TestGSLB_ECSEchoing_NonGeo(t *testing.T) {
	// Create mock backends
	backend1 := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true, Priority: 1}}
	backend1.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:      "example.com.",
		Mode:      "roundrobin", // Non-geo mode
		Backends:  []BackendInterface{backend1},
		RecordTTL: 60,
	}

	g := &GSLB{
		UseEDNSCSubnet: true,
		Records:        map[string]map[string]*Record{"example.com.": {"example.com.": record}},
		Zones:          map[string]string{"example.com.": "dummy.yml"},
	}

	// Create DNS request with ECS option
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

	w := &TestResponseWriter{}
	ctx := context.Background()

	// Call ServeDNS
	code, err := g.ServeDNS(ctx, w, r)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.NotNil(t, w.Msg)

	// Verify that the response contains the EDNS0 option with scope 0
	respOpt := w.Msg.IsEdns0()
	assert.NotNil(t, respOpt, "Response should have EDNS0")
	var respEcs *dns.EDNS0_SUBNET
	for _, opt := range respOpt.Option {
		if sub, ok := opt.(*dns.EDNS0_SUBNET); ok {
			respEcs = sub
			break
		}
	}
	assert.NotNil(t, respEcs, "Response should have EDNS0 subnet option")
	assert.Equal(t, uint8(24), respEcs.SourceNetmask)
	assert.Equal(t, uint8(0), respEcs.SourceScope, "Scope netmask should be 0 in non-geo mode")
	assert.Equal(t, "1.2.3.4", respEcs.Address.String())
}

type badRemoteAddrWriter struct {
	mockResponseWriter
	addrStr string
}

func (b *badRemoteAddrWriter) RemoteAddr() net.Addr {
	return &customAddr{addrStr: b.addrStr}
}

type customAddr struct {
	net.Addr
	addrStr string
}

func (c *customAddr) String() string {
	return c.addrStr
}

func TestGSLB_RemainingEdgeCases_DNS(t *testing.T) {
	g := &GSLB{UseEDNSCSubnet: false}
	r := new(dns.Msg)

	// net.SplitHostPort error
	wBadSplit := &badRemoteAddrWriter{addrStr: "invalid_address_no_port"}
	ip, prefix := g.extractClientIP(wBadSplit, r)
	assert.Nil(t, ip)
	assert.Equal(t, uint8(0), prefix)

	// net.ParseIP error
	wBadParse := &badRemoteAddrWriter{addrStr: "invalid-ip:12345"}
	ip, prefix = g.extractClientIP(wBadParse, r)
	assert.Nil(t, ip)
	assert.Equal(t, uint8(0), prefix)
}

func TestServeDNS_SVCBAndHTTPS(t *testing.T) {
	backend1 := &Backend{Address: "192.168.1.1", Enable: true, Port: 8443, Alive: true}
	backend2 := &Backend{Address: "2001:db8::2", Enable: true, Port: 8443, Alive: true}

	record := &Record{
		Fqdn:      "app.example.org.",
		Mode:      "failover",
		Backends:  []BackendInterface{backend1, backend2},
		RecordTTL: 60,
		ALPN:      []string{"h3", "h2"},
	}

	g := &GSLB{
		Zones:   map[string]string{"example.org.": "dummy.yml"},
		Records: map[string]map[string]*Record{"example.org.": {"app.example.org.": record}},
	}

	// 1. Query for HTTPS
	msg := new(dns.Msg)
	msg.SetQuestion("app.example.org.", dns.TypeHTTPS)
	w := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}

	code, err := g.ServeDNS(context.Background(), w, msg)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.Len(t, w.msg.Answer, 1)

	httpsRecord, ok := w.msg.Answer[0].(*dns.HTTPS)
	assert.True(t, ok)
	assert.Equal(t, uint16(1), httpsRecord.Priority)
	assert.Equal(t, ".", httpsRecord.Target)

	// Check Value fields (Port, ALPN, ipv4hint, ipv6hint)
	var foundPort uint16
	var foundAlpn []string
	var foundV4Hints []net.IP
	var foundV6Hints []net.IP

	for _, kv := range httpsRecord.Value {
		switch kv.Key() {
		case dns.SVCB_PORT:
			foundPort = kv.(*dns.SVCBPort).Port
		case dns.SVCB_ALPN:
			foundAlpn = kv.(*dns.SVCBAlpn).Alpn
		case dns.SVCB_IPV4HINT:
			foundV4Hints = kv.(*dns.SVCBIPv4Hint).Hint
		case dns.SVCB_IPV6HINT:
			foundV6Hints = kv.(*dns.SVCBIPv6Hint).Hint
		}
	}

	assert.Equal(t, uint16(8443), foundPort)
	assert.Equal(t, []string{"h3", "h2"}, foundAlpn)
	assert.Equal(t, []net.IP{net.ParseIP("192.168.1.1")}, foundV4Hints)
	assert.Equal(t, []net.IP{net.ParseIP("2001:db8::2")}, foundV6Hints)

	// 2. Query for SVCB
	msgSVCB := new(dns.Msg)
	msgSVCB.SetQuestion("app.example.org.", dns.TypeSVCB)
	wSVCB := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}

	code, err = g.ServeDNS(context.Background(), wSVCB, msgSVCB)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.Len(t, wSVCB.msg.Answer, 1)

	svcbRecord, ok := wSVCB.msg.Answer[0].(*dns.SVCB)
	assert.True(t, ok)
	assert.Equal(t, uint16(1), svcbRecord.Priority)
	assert.Equal(t, ".", svcbRecord.Target)

	// 3. Test Failover Policies when both are down
	backend1.Alive = false
	backend2.Alive = false

	// Failover Mode: fail-closed with NXDOMAIN
	record.FailoverPolicy = FailoverPolicy{
		Mode:  "fail-closed",
		Rcode: "NXDOMAIN",
	}
	wFailClosed := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
	_, err = g.ServeDNS(context.Background(), wFailClosed, msg)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeNameError, wFailClosed.msg.Rcode)

	// Failover Mode: fail-specific with fallback IPs
	record.FailoverPolicy = FailoverPolicy{
		Mode:        "fail-specific",
		FallbackIPs: []string{"192.168.5.5", "2001:db8::5"},
	}
	wFailSpecific := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
	code, err = g.ServeDNS(context.Background(), wFailSpecific, msg)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.Len(t, wFailSpecific.msg.Answer, 1)

	httpsRecordFailSpecific := wFailSpecific.msg.Answer[0].(*dns.HTTPS)
	var foundV4HintsSpecific []net.IP
	var foundV6HintsSpecific []net.IP
	for _, kv := range httpsRecordFailSpecific.Value {
		switch kv.Key() {
		case dns.SVCB_IPV4HINT:
			foundV4HintsSpecific = kv.(*dns.SVCBIPv4Hint).Hint
		case dns.SVCB_IPV6HINT:
			foundV6HintsSpecific = kv.(*dns.SVCBIPv6Hint).Hint
		}
	}
	assert.Equal(t, []net.IP{net.ParseIP("192.168.5.5")}, foundV4HintsSpecific)
	assert.Equal(t, []net.IP{net.ParseIP("2001:db8::5")}, foundV6HintsSpecific)
}

func TestCNAMEBackendResolution(t *testing.T) {
	backend := &MockBackend{Backend: &Backend{Address: "some-alb.aws.com", Enable: true, Priority: 10}}
	backend.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:      "cname.example.org.",
		Mode:      "failover",
		Backends:  []BackendInterface{backend},
		RecordTTL: 60,
	}

	g := &GSLB{
		Zones:   map[string]string{"example.org.": "dummy.yml"},
		Records: map[string]map[string]*Record{"example.org.": {"cname.example.org.": record}},
	}

	// 1. Query TypeA
	msgA := new(dns.Msg)
	msgA.SetQuestion("cname.example.org.", dns.TypeA)
	wA := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}

	code, err := g.ServeDNS(context.Background(), wA, msgA)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.Len(t, wA.msg.Answer, 1)

	cnameA, ok := wA.msg.Answer[0].(*dns.CNAME)
	assert.True(t, ok, "Expected CNAME record")
	assert.Equal(t, "cname.example.org.", cnameA.Hdr.Name)
	assert.Equal(t, dns.TypeCNAME, cnameA.Hdr.Rrtype)
	assert.Equal(t, uint32(60), cnameA.Hdr.Ttl)
	assert.Equal(t, "some-alb.aws.com.", cnameA.Target)

	// 2. Query TypeAAAA
	msgAAAA := new(dns.Msg)
	msgAAAA.SetQuestion("cname.example.org.", dns.TypeAAAA)
	wAAAA := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}

	code, err = g.ServeDNS(context.Background(), wAAAA, msgAAAA)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.Len(t, wAAAA.msg.Answer, 1)

	cnameAAAA, ok := wAAAA.msg.Answer[0].(*dns.CNAME)
	assert.True(t, ok, "Expected CNAME record")
	assert.Equal(t, "cname.example.org.", cnameAAAA.Hdr.Name)
	assert.Equal(t, dns.TypeCNAME, cnameAAAA.Hdr.Rrtype)
	assert.Equal(t, uint32(60), cnameAAAA.Hdr.Ttl)
	assert.Equal(t, "some-alb.aws.com.", cnameAAAA.Target)
}

type mockNextPlugin struct {
	called bool
}

func (n *mockNextPlugin) Name() string { return "mocknext" }

func (n *mockNextPlugin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	n.called = true
	response := new(dns.Msg)
	response.SetReply(r)

	q := r.Question[0]
	switch {
	case q.Qtype == dns.TypeMX:
		mx := &dns.MX{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeMX,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			Preference: 10,
			Mx:         "mail.example.org.",
		}
		response.Answer = append(response.Answer, mx)
	case q.Name == "nope.example.org.":
		response.Rcode = dns.RcodeNameError
	default:
		a := &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP("1.1.1.1"),
		}
		response.Answer = append(response.Answer, a)
	}

	err := w.WriteMsg(response)
	return response.Rcode, err
}

func TestServeDNS_FallbackECS(t *testing.T) {
	n := &mockNextPlugin{}
	g := &GSLB{
		Zones:          map[string]string{"example.org.": "dummy.yml"},
		UseEDNSCSubnet: true,
		Next:           n,
	}

	getECS := func(msg *dns.Msg) *dns.EDNS0_SUBNET {
		if msg == nil {
			return nil
		}
		opt := msg.IsEdns0()
		if opt == nil {
			return nil
		}
		for _, o := range opt.Option {
			if ecs, ok := o.(*dns.EDNS0_SUBNET); ok {
				return ecs
			}
		}
		return nil
	}

	// 1. MX Query to authoritative zone. Falls through to next plugin because GSLB doesn't handle MX.
	msgMX := new(dns.Msg)
	msgMX.SetQuestion("registry.example.org.", dns.TypeMX)
	optMX := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	ecsMX := &dns.EDNS0_SUBNET{
		Code:          dns.EDNS0SUBNET,
		Address:       net.ParseIP("203.0.113.0"),
		SourceNetmask: 24,
		Family:        1,
	}
	optMX.Option = append(optMX.Option, ecsMX)
	msgMX.Extra = append(msgMX.Extra, optMX)
	wMX := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}

	_, err := g.ServeDNS(context.Background(), wMX, msgMX)
	assert.NoError(t, err)
	assert.True(t, n.called)

	respEcsMX := getECS(wMX.msg)
	assert.NotNil(t, respEcsMX, "Expected ECS option in MX response")
	if respEcsMX != nil {
		assert.Equal(t, "203.0.113.0", respEcsMX.Address.String())
		assert.Equal(t, uint8(24), respEcsMX.SourceNetmask)
	}

	// Reset next plugin called status
	n.called = false

	// 2. A Query for a domain not in GSLB records but in authoritative zone (e.g. static.example.org.)
	msgStatic := new(dns.Msg)
	msgStatic.SetQuestion("static.example.org.", dns.TypeA)
	optStatic := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	ecsStatic := &dns.EDNS0_SUBNET{
		Code:          dns.EDNS0SUBNET,
		Address:       net.ParseIP("203.0.113.0"),
		SourceNetmask: 24,
		Family:        1,
	}
	optStatic.Option = append(optStatic.Option, ecsStatic)
	msgStatic.Extra = append(msgStatic.Extra, optStatic)
	wStatic := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}

	_, err = g.ServeDNS(context.Background(), wStatic, msgStatic)
	assert.NoError(t, err)
	assert.True(t, n.called)

	respEcsStatic := getECS(wStatic.msg)
	assert.NotNil(t, respEcsStatic, "Expected ECS option in static A response")
	if respEcsStatic != nil {
		assert.Equal(t, "203.0.113.0", respEcsStatic.Address.String())
		assert.Equal(t, uint8(24), respEcsStatic.SourceNetmask)
	}

	// Reset next plugin called status
	n.called = false

	// 3. A Query for a non-existent domain in authoritative zone (e.g. nope.example.org.) producing NXDOMAIN
	msgNope := new(dns.Msg)
	msgNope.SetQuestion("nope.example.org.", dns.TypeA)
	optNope := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	ecsNope := &dns.EDNS0_SUBNET{
		Code:          dns.EDNS0SUBNET,
		Address:       net.ParseIP("203.0.113.0"),
		SourceNetmask: 24,
		Family:        1,
	}
	optNope.Option = append(optNope.Option, ecsNope)
	msgNope.Extra = append(msgNope.Extra, optNope)
	wNope := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}

	code, err := g.ServeDNS(context.Background(), wNope, msgNope)
	assert.NoError(t, err)
	assert.True(t, n.called)
	assert.Equal(t, dns.RcodeNameError, code)

	respEcsNope := getECS(wNope.msg)
	assert.NotNil(t, respEcsNope, "Expected ECS option in NXDOMAIN response")
	if respEcsNope != nil {
		assert.Equal(t, "203.0.113.0", respEcsNope.Address.String())
		assert.Equal(t, uint8(24), respEcsNope.SourceNetmask)
	}
}

// TestServeDNS_FallthroughECSScopeMustBeZero verifies that when a domain has a
// geoip record in GSLB but the query type (MX, NS, SOA…) falls through to the
// next plugin, the ECS scope in the response MUST be /0 (global), not the
// client's prefix length. Static answers are identical for the whole world;
// stamping them with a non-zero scope causes Google DNS to cache them per-subnet,
// leading to overlapping-scope cache hazards (RFC 7871 §7.3.1).
func TestServeDNS_FallthroughECSScopeMustBeZero(t *testing.T) {
	// A geoip record exists for this domain — A queries are geo-routed
	backend := &Backend{Address: "192.168.1.1", Enable: true, Alive: true, Priority: 1, Location: "eu-west"}
	record := &Record{
		Fqdn:      "registry.example.org.",
		Mode:      "geoip",
		Backends:  []BackendInterface{backend},
		RecordTTL: 60,
	}

	n := &mockNextPlugin{}
	g := &GSLB{
		Zones:          map[string]string{"example.org.": "dummy.yml"},
		Records:        map[string]map[string]*Record{"example.org.": {"registry.example.org.": record}},
		UseEDNSCSubnet: true,
		Next:           n,
	}

	getECS := func(msg *dns.Msg) *dns.EDNS0_SUBNET {
		if msg == nil {
			return nil
		}
		opt := msg.IsEdns0()
		if opt == nil {
			return nil
		}
		for _, o := range opt.Option {
			if ecs, ok := o.(*dns.EDNS0_SUBNET); ok {
				return ecs
			}
		}
		return nil
	}

	makeECSQuery := func(name string, qtype uint16) *dns.Msg {
		msg := new(dns.Msg)
		msg.SetQuestion(name, qtype)
		opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
		ecs := &dns.EDNS0_SUBNET{
			Code:          dns.EDNS0SUBNET,
			Address:       net.ParseIP("203.0.113.0"),
			SourceNetmask: 24,
			Family:        1,
		}
		opt.Option = append(opt.Option, ecs)
		msg.Extra = append(msg.Extra, opt)
		return msg
	}

	// 1. Sanity check: A query for the geoip domain — GSLB handles it directly,
	//    scope SHOULD be /24 (geo-specific answer)
	msgA := makeECSQuery("registry.example.org.", dns.TypeA)
	wA := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
	_, err := g.ServeDNS(context.Background(), wA, msgA)
	assert.NoError(t, err)

	respEcsA := getECS(wA.msg)
	assert.NotNil(t, respEcsA, "A response must have ECS")
	if respEcsA != nil {
		assert.Equal(t, uint8(24), respEcsA.SourceScope, "A response is geo-specific, scope must be /24")
	}

	// 2. MX query for the SAME geoip domain — falls through to next plugin.
	//    The MX answer is static/global, scope MUST be /0.
	n.called = false
	msgMX := makeECSQuery("registry.example.org.", dns.TypeMX)
	wMX := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
	_, err = g.ServeDNS(context.Background(), wMX, msgMX)
	assert.NoError(t, err)
	assert.True(t, n.called, "MX must fall through to next plugin")

	respEcsMX := getECS(wMX.msg)
	assert.NotNil(t, respEcsMX, "MX response must have ECS")
	if respEcsMX != nil {
		assert.Equal(t, "203.0.113.0", respEcsMX.Address.String())
		assert.Equal(t, uint8(24), respEcsMX.SourceNetmask, "SourceNetmask must echo the query")
		assert.Equal(t, uint8(0), respEcsMX.SourceScope,
			"MX is static/global, scope must be /0 — not the client prefix")
	}

	// 3. SOA query for the SAME geoip domain — also falls through, scope MUST be /0.
	n.called = false
	msgSOA := makeECSQuery("registry.example.org.", dns.TypeSOA)
	wSOA := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
	_, err = g.ServeDNS(context.Background(), wSOA, msgSOA)
	assert.NoError(t, err)
	assert.True(t, n.called, "SOA must fall through to next plugin")

	respEcsSOA := getECS(wSOA.msg)
	assert.NotNil(t, respEcsSOA, "SOA response must have ECS")
	if respEcsSOA != nil {
		assert.Equal(t, uint8(0), respEcsSOA.SourceScope,
			"SOA is static/global, scope must be /0")
	}

	// 4. AAAA query for the SAME domain (which only has IPv4/A backend configured).
	//    GSLB handles it directly (no fallthrough) but returns NODATA (empty answer).
	//    Since the type is not configured/supported, it is a static NODATA response
	//    and scope MUST be /0.
	n.called = false
	msgAAAA := makeECSQuery("registry.example.org.", dns.TypeAAAA)
	wAAAA := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
	_, err = g.ServeDNS(context.Background(), wAAAA, msgAAAA)
	assert.NoError(t, err)
	assert.False(t, n.called, "AAAA should be handled by GSLB directly (returning NODATA)")

	respEcsAAAA := getECS(wAAAA.msg)
	assert.NotNil(t, respEcsAAAA, "AAAA response must have ECS")
	if respEcsAAAA != nil {
		assert.Equal(t, uint8(0), respEcsAAAA.SourceScope,
			"AAAA NODATA for A-only record must have scope /0")
	}
}

func TestGSLB_ServeDNS_Concurrency(t *testing.T) {
	backend1 := &Backend{Address: "192.168.1.1", Enable: true}
	record := &Record{
		Fqdn:      "race.example.org.",
		Mode:      "failover",
		Backends:  []BackendInterface{backend1},
		RecordTTL: 60,
	}

	g := &GSLB{
		Zones:   map[string]string{"example.org.": "dummy.yml"},
		Records: map[string]map[string]*Record{"example.org.": {"race.example.org.": record}},
	}

	stopCh := make(chan struct{})
	var wg sync.WaitGroup

	// Concurrently read via ServeDNS
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					msg := new(dns.Msg)
					msg.SetQuestion("race.example.org.", dns.TypeA)
					w := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}
					_, _ = g.ServeDNS(context.Background(), w, msg)
				}
			}
		}()
	}

	// Concurrently write simulating reloadConfig writing under lock
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			g.Mutex.Lock()
			newRecords := map[string]map[string]*Record{
				"example.org.": {
					"race.example.org.": record,
				},
			}
			g.Records = newRecords
			g.Mutex.Unlock()
			time.Sleep(1 * time.Millisecond)
		}
		close(stopCh)
	}()

	wg.Wait()
}

func TestServeDNS_SRV(t *testing.T) {
	backend1 := &Backend{
		Address:  "srv1.example.org",
		Port:     5060,
		Priority: 10,
		Weight:   50,
		Enable:   true,
		Alive:    true,
	}
	backend2 := &Backend{
		Address:  "srv2.example.org",
		Port:     5061,
		Priority: 20,
		Weight:   100,
		Enable:   true,
		Alive:    true,
	}
	record := &Record{
		Fqdn:      "sip.example.org.",
		Mode:      "failover", // will pick backend1 first since priority 10 < 20
		Backends:  []BackendInterface{backend1, backend2},
		RecordTTL: 60,
	}

	g := &GSLB{
		Zones:   map[string]string{"example.org.": "dummy.yml"},
		Records: map[string]map[string]*Record{"example.org.": {"sip.example.org.": record}},
	}

	msg := new(dns.Msg)
	msg.SetQuestion("sip.example.org.", dns.TypeSRV)
	w := &mockResponseWriter{msg: new(dns.Msg), ip: net.ParseIP("127.0.0.1")}

	code, err := g.ServeDNS(context.Background(), w, msg)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, code)
	assert.NotNil(t, w.msg)
	assert.Len(t, w.msg.Answer, 1)

	srvRecord, ok := w.msg.Answer[0].(*dns.SRV)
	assert.True(t, ok)
	assert.Equal(t, "sip.example.org.", srvRecord.Hdr.Name)
	assert.Equal(t, uint16(10), srvRecord.Priority)
	assert.Equal(t, uint16(50), srvRecord.Weight)
	assert.Equal(t, uint16(5060), srvRecord.Port)
	assert.Equal(t, "srv1.example.org.", srvRecord.Target)
}
