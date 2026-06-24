package gslb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type DiscoveredEndpoint struct {
	Address string
	Port    int
}

type DiscoveryConfig struct {
	Type     string `yaml:"type"`
	Endpoint string `yaml:"endpoint"`
	Service  string `yaml:"service"`
	Interval string `yaml:"interval"`
}

func (d *DiscoveryConfig) FetchEndpoints() ([]DiscoveredEndpoint, error) {
	switch strings.ToLower(d.Type) {
	case "consul":
		return d.fetchConsulEndpoints()
	case "http":
		return d.fetchHTTPEndpoints()
	case "dns_svcb", "dns_https":
		return d.fetchDNSEndpoints()
	default:
		return nil, fmt.Errorf("unsupported discovery type: %s", d.Type)
	}
}

func (d *DiscoveryConfig) fetchConsulEndpoints() ([]DiscoveredEndpoint, error) {
	url := fmt.Sprintf("%s/v1/catalog/service/%s", strings.TrimSuffix(d.Endpoint, "/"), d.Service)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("consul returned status %d", resp.StatusCode)
	}

	var services []struct {
		ServiceAddress string `json:"ServiceAddress"`
		Address        string `json:"Address"`
		ServicePort    int    `json:"ServicePort"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return nil, err
	}

	var endpoints []DiscoveredEndpoint
	for _, s := range services {
		addr := s.ServiceAddress
		if addr == "" {
			addr = s.Address
		}
		if addr == "" {
			continue
		}
		endpoints = append(endpoints, DiscoveredEndpoint{
			Address: addr,
			Port:    s.ServicePort,
		})
	}
	return endpoints, nil
}

func (d *DiscoveryConfig) fetchHTTPEndpoints() ([]DiscoveredEndpoint, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(d.Endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Try parsing simple string array first
	var ipStrings []string
	if err := json.Unmarshal(body, &ipStrings); err == nil {
		var endpoints []DiscoveredEndpoint
		for _, ip := range ipStrings {
			endpoints = append(endpoints, DiscoveredEndpoint{
				Address: ip,
				Port:    0,
			})
		}
		return endpoints, nil
	}

	// Try parsing detailed object array
	var detailed []struct {
		Address string `json:"address"`
		Port    int    `json:"port"`
	}
	if err := json.Unmarshal(body, &detailed); err != nil {
		return nil, fmt.Errorf("failed to parse HTTP response: %w", err)
	}

	var endpoints []DiscoveredEndpoint
	for _, det := range detailed {
		endpoints = append(endpoints, DiscoveredEndpoint{
			Address: det.Address,
			Port:    det.Port,
		})
	}
	return endpoints, nil
}

func (d *DiscoveryConfig) fetchDNSEndpoints() ([]DiscoveredEndpoint, error) {
	m := new(dns.Msg)
	qtype := dns.TypeSVCB
	if strings.ToLower(d.Type) == "dns_https" {
		qtype = dns.TypeHTTPS
	}
	m.SetQuestion(dns.Fqdn(d.Service), qtype)

	client := &dns.Client{Timeout: 5 * time.Second}
	r, _, err := client.Exchange(m, d.Endpoint)
	if err != nil {
		return nil, err
	}

	var endpoints []DiscoveredEndpoint
	for _, rr := range r.Answer {
		var svcb *dns.SVCB
		if qtype == dns.TypeSVCB {
			if s, ok := rr.(*dns.SVCB); ok {
				svcb = s
			}
		} else {
			if h, ok := rr.(*dns.HTTPS); ok {
				svcb = &h.SVCB
			}
		}

		if svcb == nil {
			continue
		}

		port := 0
		var ips []string

		for _, kv := range svcb.Value {
			switch kv.Key() {
			case dns.SVCB_PORT:
				if p, ok := kv.(*dns.SVCBPort); ok {
					port = int(p.Port)
				}
			case dns.SVCB_IPV4HINT:
				if h, ok := kv.(*dns.SVCBIPv4Hint); ok {
					for _, ip := range h.Hint {
						ips = append(ips, ip.String())
					}
				}
			case dns.SVCB_IPV6HINT:
				if h, ok := kv.(*dns.SVCBIPv6Hint); ok {
					for _, ip := range h.Hint {
						ips = append(ips, ip.String())
					}
				}
			}
		}

		// If no IPv4/IPv6 hints were provided but a Target is set, resolve target
		if len(ips) == 0 && svcb.Target != "." && svcb.Target != "" {
			// Resolve Target using A query to the same server
			aMsg := new(dns.Msg)
			aMsg.SetQuestion(dns.Fqdn(svcb.Target), dns.TypeA)
			aResp, _, err := client.Exchange(aMsg, d.Endpoint)
			if err == nil {
				for _, arr := range aResp.Answer {
					if aRR, ok := arr.(*dns.A); ok {
						ips = append(ips, aRR.A.String())
					}
				}
			}
			// Resolve Target using AAAA query
			aaaaMsg := new(dns.Msg)
			aaaaMsg.SetQuestion(dns.Fqdn(svcb.Target), dns.TypeAAAA)
			aaaaResp, _, err := client.Exchange(aaaaMsg, d.Endpoint)
			if err == nil {
				for _, aaaaRR := range aaaaResp.Answer {
					if aaaa, ok := aaaaRR.(*dns.AAAA); ok {
						ips = append(ips, aaaa.AAAA.String())
					}
				}
			}
		}

		for _, ip := range ips {
			endpoints = append(endpoints, DiscoveredEndpoint{
				Address: ip,
				Port:    port,
			})
		}
	}
	return endpoints, nil
}
