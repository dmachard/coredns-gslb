package gslb

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

func TestDiscovery_Consul(t *testing.T) {
	// Mock Consul catalog API response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/catalog/service/my-service", r.URL.Path)
		res := []map[string]interface{}{
			{
				"ServiceAddress": "192.168.1.10",
				"ServicePort":    8080,
			},
			{
				"Address":     "192.168.1.11",
				"ServicePort": 8081,
			},
		}
		json.NewEncoder(w).Encode(res)
	}))
	defer server.Close()

	d := &DiscoveryConfig{
		Type:     "consul",
		Endpoint: server.URL,
		Service:  "my-service",
	}

	endpoints, err := d.FetchEndpoints()
	assert.NoError(t, err)
	assert.Len(t, endpoints, 2)
	assert.Equal(t, "192.168.1.10", endpoints[0].Address)
	assert.Equal(t, 8080, endpoints[0].Port)
	assert.Equal(t, "192.168.1.11", endpoints[1].Address)
	assert.Equal(t, 8081, endpoints[1].Port)
}

func TestDiscovery_HTTP_Simple(t *testing.T) {
	// Mock HTTP API response returning string array of IPs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := []string{"10.0.0.1", "10.0.0.2"}
		json.NewEncoder(w).Encode(res)
	}))
	defer server.Close()

	d := &DiscoveryConfig{
		Type:     "http",
		Endpoint: server.URL,
	}

	endpoints, err := d.FetchEndpoints()
	assert.NoError(t, err)
	assert.Len(t, endpoints, 2)
	assert.Equal(t, "10.0.0.1", endpoints[0].Address)
	assert.Equal(t, 0, endpoints[0].Port)
	assert.Equal(t, "10.0.0.2", endpoints[1].Address)
	assert.Equal(t, 0, endpoints[1].Port)
}

func TestDiscovery_HTTP_Detailed(t *testing.T) {
	// Mock HTTP API response returning detailed objects
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := []map[string]interface{}{
			{"address": "10.0.0.1", "port": 9000},
			{"address": "10.0.0.2", "port": 9001},
		}
		json.NewEncoder(w).Encode(res)
	}))
	defer server.Close()

	d := &DiscoveryConfig{
		Type:     "http",
		Endpoint: server.URL,
	}

	endpoints, err := d.FetchEndpoints()
	assert.NoError(t, err)
	assert.Len(t, endpoints, 2)
	assert.Equal(t, "10.0.0.1", endpoints[0].Address)
	assert.Equal(t, 9000, endpoints[0].Port)
	assert.Equal(t, "10.0.0.2", endpoints[1].Address)
	assert.Equal(t, 9001, endpoints[1].Port)
}

func TestRecord_UpdateBackendsFromDiscovery(t *testing.T) {
	r := &Record{
		Fqdn: "test.example.com.",
		Backends: []BackendInterface{
			&Backend{
				Address: "10.0.0.1",
				Port:    80,
				Enable:  true,
			},
		},
	}

	// 10.0.0.1 stays (port 80), new 10.0.0.2 (port 80) is added
	discovered := []DiscoveredEndpoint{
		{Address: "10.0.0.1", Port: 80},
		{Address: "10.0.0.2", Port: 80},
	}

	r.updateBackendsFromDiscovery(discovered)

	assert.Len(t, r.Backends, 2)
	b1 := r.Backends[0].(*Backend)
	assert.Equal(t, "10.0.0.1", b1.Address)
	assert.Equal(t, 80, b1.Port)

	b2 := r.Backends[1].(*Backend)
	assert.Equal(t, "10.0.0.2", b2.Address)
	assert.Equal(t, 80, b2.Port)
	assert.True(t, b2.Enable)
}

func TestDiscovery_SVCB_DNS(t *testing.T) {
	// Start a local mock DNS server using miekg/dns
	dns.HandleFunc("service.internal.", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)

		svcb := &dns.SVCB{
			Hdr: dns.RR_Header{
				Name:   "service.internal.",
				Rrtype: dns.TypeSVCB,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			Priority: 1,
			Target:   "target.service.internal.",
			Value: []dns.SVCBKeyValue{
				&dns.SVCBPort{Port: 8080},
				&dns.SVCBIPv4Hint{Hint: []net.IP{net.ParseIP("10.0.0.5")}},
			},
		}
		m.Answer = append(m.Answer, svcb)
		w.WriteMsg(m)
	})

	dnsServer := &dns.Server{Addr: "127.0.0.1:0", Net: "udp"}

	// Start the DNS server
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	dnsServer.PacketConn = pc

	go dnsServer.ActivateAndServe()
	defer dnsServer.Shutdown()

	d := &DiscoveryConfig{
		Type:     "dns_svcb",
		Endpoint: pc.LocalAddr().String(),
		Service:  "service.internal.",
	}

	endpoints, err := d.FetchEndpoints()
	assert.NoError(t, err)
	assert.Len(t, endpoints, 1)
	assert.Equal(t, "10.0.0.5", endpoints[0].Address)
	assert.Equal(t, 8080, endpoints[0].Port)
}

func TestDiscovery_HTTPS_DNS(t *testing.T) {
	// Start a local mock DNS server using miekg/dns
	dns.HandleFunc("https.service.internal.", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)

		https := &dns.HTTPS{
			SVCB: dns.SVCB{
				Hdr: dns.RR_Header{
					Name:   "https.service.internal.",
					Rrtype: dns.TypeHTTPS,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				Priority: 1,
				Target:   "target.service.internal.",
				Value: []dns.SVCBKeyValue{
					&dns.SVCBPort{Port: 8443},
					&dns.SVCBIPv4Hint{Hint: []net.IP{net.ParseIP("10.0.0.6")}},
				},
			},
		}
		m.Answer = append(m.Answer, https)
		w.WriteMsg(m)
	})

	dnsServer := &dns.Server{Addr: "127.0.0.1:0", Net: "udp"}

	// Start the DNS server
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	dnsServer.PacketConn = pc

	go dnsServer.ActivateAndServe()
	defer dnsServer.Shutdown()

	d := &DiscoveryConfig{
		Type:     "dns_https",
		Endpoint: pc.LocalAddr().String(),
		Service:  "https.service.internal.",
	}

	endpoints, err := d.FetchEndpoints()
	assert.NoError(t, err)
	assert.Len(t, endpoints, 1)
	assert.Equal(t, "10.0.0.6", endpoints[0].Address)
	assert.Equal(t, 8443, endpoints[0].Port)
}
