package gslb

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"os"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
	"gopkg.in/yaml.v3"
)

var log = clog.NewWithPlugin("gslb")

type GSLB struct {
	Next                      plugin.Handler
	Zones                     map[string]string  // List of authoritative domains
	Records                   map[string]*Record `yaml:"records"`
	LastResolution            sync.Map           // key: domain (string), value: time.Time
	RoundRobinIndex           sync.Map
	MaxStaggerStart           string
	BatchSizeStart            int
	ResolutionIdleTimeout     string
	ResolutionIdleMultiplier  int // Multiplier for slow healthcheck interval
	HealthcheckIdleMultiplier int // Multiplier for slow healthcheck interval
	Mutex                     sync.RWMutex
	UseEDNSCSubnet            bool
	LocationMap               map[string]string
	GeoIPCountryDB            *geoip2.Reader // Loaded MaxMind DB (country)
	GeoIPCityDB               *geoip2.Reader // Loaded MaxMind DB (city)
	GeoIPASNDB                *geoip2.Reader // Loaded MaxMind DB (ASN)
}

func (g *GSLB) Name() string { return "gslb" }

func (g *GSLB) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	// Get domain and ensure it is fully qualified
	q := r.Question[0]
	domain := dns.Fqdn(strings.TrimSuffix(q.Name, "."))

	// If the domain doesn't match any authoritative domain, pass to the next plugin
	if !g.isAuthoritative(domain) {
		return plugin.NextOrFailure(g.Name(), g.Next, ctx, w, r)
	}

	// Determine the client IP and prefix length (ECS or RemoteAddr fallback)
	clientIP, clientPrefixLen := g.extractClientIP(w, r)
	if clientIP == nil {
		log.Error("Failed to determine client IP, responding with SERVFAIL")
		return dns.RcodeServerFailure, nil
	}
	ctx = WithClientInfo(ctx, clientIP, clientPrefixLen)

	// Update the last resolution time for the domain
	// This is used to track when the last resolution was made for a domain
	g.updateLastResolutionTime(domain)

	switch q.Qtype {
	case dns.TypeA:
		return g.handleIPRecord(ctx, w, r, domain, dns.TypeA)
	case dns.TypeAAAA:
		return g.handleIPRecord(ctx, w, r, domain, dns.TypeAAAA)
	case dns.TypeTXT:
		return g.handleTXTRecord(ctx, w, r, domain)
	default:
		return plugin.NextOrFailure(g.Name(), g.Next, ctx, w, r)
	}
}

func (g *GSLB) extractClientIP(w dns.ResponseWriter, r *dns.Msg) (net.IP, uint8) {
	var clientIP net.IP
	var prefixLen uint8 = 32 // Default for IPv4

	// Check for EDNS options
	if g.UseEDNSCSubnet {
		if o := r.IsEdns0(); o != nil {
			for _, option := range o.Option {
				if ecs, ok := option.(*dns.EDNS0_SUBNET); ok {
					log.Debugf("ECS Detected: IP=%s, PrefixLength=%d", ecs.Address, ecs.SourceNetmask)
					return ecs.Address, ecs.SourceNetmask
				}
			}
		}
	}

	// Fallback to remote address if ECS is not present
	remoteAddr := w.RemoteAddr()
	host, _, err := net.SplitHostPort(remoteAddr.String())
	if err != nil {
		log.Errorf("Failed to parse remote address %s: %v", remoteAddr, err)
		return nil, 0
	}
	clientIP = net.ParseIP(host)
	if clientIP == nil {
		log.Errorf("Invalid IP address extracted from remote address: %s", host)
		return nil, 0
	}

	// Determine the prefix length based on the IP type
	if clientIP.To4() == nil {
		prefixLen = 128 // Default for IPv6
	}
	return clientIP, prefixLen
}

func (g *GSLB) isAuthoritative(domain string) bool {
	for authDomain := range g.Zones {
		if strings.HasSuffix(domain, authDomain) {
			return true
		}
	}
	return false
}

func (g *GSLB) handleIPRecord(ctx context.Context, w dns.ResponseWriter, r *dns.Msg, domain string, recordType uint16) (int, error) {
	record, exists := g.Records[domain]
	if !exists {
		return plugin.NextOrFailure(g.Name(), g.Next, ctx, w, r)
	}
	ci := GetClientInfo(ctx)
	if ci == nil || ci.IP == nil {
		log.Error("No client info in context")
		return dns.RcodeServerFailure, nil
	}
	start := time.Now()
	ip, err := g.pickResponse(domain, recordType, ci.IP)
	if err != nil {
		log.Debugf("[%s] no backend available for type %d: %v", domain, recordType, err)

		// Fallback: get all IP addresses
		ipAddresses, err := g.pickAllAddresses(domain, recordType)
		if err != nil {
			log.Debugf("Error retrieving backends for domain %s: %v", domain, err)
			ObserveRecordResolutionDuration(domain, "fail", time.Since(start).Seconds())
			return dns.RcodeServerFailure, nil
		}

		ObserveRecordResolutionDuration(domain, "fail", time.Since(start).Seconds())
		return g.sendAddressRecordResponse(w, r, domain, ipAddresses, record.RecordTTL, recordType)
	}

	ObserveRecordResolutionDuration(domain, "success", time.Since(start).Seconds())
	return g.sendAddressRecordResponse(w, r, domain, ip, record.RecordTTL, recordType)
}

func (g *GSLB) handleTXTRecord(ctx context.Context, w dns.ResponseWriter, r *dns.Msg, domain string) (int, error) {
	record, exists := g.Records[domain]
	if !exists {
		// If the domain is not found in the records, pass the request to the next plugin
		return plugin.NextOrFailure(g.Name(), g.Next, ctx, w, r)
	}

	// Prepare a list to store the backend summaries
	var summaries []string
	for _, backend := range record.Backends {
		// Determine the backend's health status
		status := "unhealthy"
		if backend.IsHealthy() {
			status = "healthy"
		}

		// Determine if the backend is enabled or not
		enabled := "true"
		if !backend.IsEnabled() {
			enabled = "false"
		}

		// Format the backend information as a summary string
		summary := fmt.Sprintf(
			"Backend: %s | Priority: %d | Status: %s | Enabled: %v",
			backend.GetAddress(), backend.GetPriority(), status, enabled,
		)
		// Add the summary to the list
		summaries = append(summaries, summary)
	}

	// Create the DNS response message
	response := new(dns.Msg)
	response.SetReply(r)

	// Add each chunk as a separate TXT record in the response
	for _, summary := range summaries {
		// Add the chunk as a TXT record
		txt := &dns.TXT{
			Hdr: dns.RR_Header{
				Name:   domain,
				Rrtype: dns.TypeTXT,
				Class:  dns.ClassINET,
				Ttl:    uint32(record.RecordTTL),
			},
			Txt: []string{summary},
		}
		// Append the TXT record to the response
		response.Answer = append(response.Answer, txt)
	}

	// Send the DNS response with the multiple TXT records
	if err := w.WriteMsg(response); err != nil {
		log.Error("Failed to write DNS TXT response: ", err)
		return dns.RcodeServerFailure, err
	}

	// Return success
	return dns.RcodeSuccess, nil
}

func (g *GSLB) pickAllAddresses(domain string, recordType uint16) ([]string, error) {
	record, exists := g.Records[domain]
	if !exists {
		return nil, fmt.Errorf("domain not found: %s", domain)
	}

	var ipAddresses []string
	for _, backend := range record.Backends {
		if backend.IsEnabled() {
			ip := backend.GetAddress()
			if (recordType == dns.TypeA && net.ParseIP(ip).To4() != nil) ||
				(recordType == dns.TypeAAAA && net.ParseIP(ip).To16() != nil && net.ParseIP(ip).To4() == nil) {
				ipAddresses = append(ipAddresses, ip)
			}
		}
	}

	if len(ipAddresses) == 0 {
		return nil, fmt.Errorf("no backends exist for domain: %s", domain)
	}

	return ipAddresses, nil
}

func (g *GSLB) pickResponse(domain string, recordType uint16, clientIP net.IP) ([]string, error) {
	record, exists := g.Records[domain]
	if !exists {
		return nil, fmt.Errorf("domain not found: %s", domain)
	}

	switch record.Mode {
	case "failover":
		return g.pickBackendWithFailover(record, recordType)
	case "roundrobin":
		return g.pickBackendWithRoundRobin(domain, record, recordType)
	case "random":
		return g.pickBackendWithRandom(record, recordType)
	case "geoip":
		return g.pickBackendWithGeoIP(record, recordType, clientIP)
	default:
		return nil, fmt.Errorf("unsupported mode: %s", record.Mode)
	}
}

func (g *GSLB) sendAddressRecordResponse(w dns.ResponseWriter, r *dns.Msg, domain string, ipAddresses []string, ttl int, recordType uint16) (int, error) {
	response := new(dns.Msg)
	response.SetReply(r)
	for _, ip := range ipAddresses {
		var rr dns.RR
		switch recordType {
		case dns.TypeA:
			rr = &dns.A{
				Hdr: dns.RR_Header{
					Name:   domain,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    uint32(ttl),
				},
				A: net.ParseIP(ip),
			}
		case dns.TypeAAAA:
			rr = &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   domain,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    uint32(ttl),
				},
				AAAA: net.ParseIP(ip),
			}
		}
		response.Answer = append(response.Answer, rr)
	}

	err := w.WriteMsg(response)
	if err != nil {
		log.Error("Failed to write DNS response: ", err)
		IncRecordResolutions(domain, "fail")
		return dns.RcodeServerFailure, err
	}
	IncRecordResolutions(domain, "success")
	return dns.RcodeSuccess, nil
}

func (g *GSLB) updateRecords(ctx context.Context, newGSLB *GSLB) {
	for domain, newRecord := range newGSLB.Records {
		oldRecord, exists := g.Records[domain]
		if !exists {
			recordCtx, cancel := context.WithCancel(ctx)
			newRecord.cancelFunc = cancel
			newRecord.Fqdn = dns.Fqdn(domain)

			g.Records[domain] = newRecord
			log.Infof("Added new record for domain: %s", domain)
			go newRecord.scrapeBackends(recordCtx, g)
		} else {
			oldRecord.updateRecord(newRecord)
		}
	}

	for domain := range g.Records {
		if _, exists := newGSLB.Records[domain]; !exists {
			// cancel context to terminate the goroutine
			if record := g.Records[domain]; record.cancelFunc != nil {
				record.cancelFunc()
			}

			// delete records
			delete(g.Records, domain)
			log.Infof("Records [%s] removed", domain)
		}
	}
	SetRecordsConfiguredTotal(float64(len(g.Records)))
}

func (g *GSLB) initializeRecords(ctx context.Context) {
	groups := g.batchRecords(g.BatchSizeStart)

	for i, group := range groups {
		go func(group []*Record, delay time.Duration) {
			time.Sleep(delay)
			for _, record := range group {
				domain := record.Fqdn
				recordCtx, cancel := context.WithCancel(ctx)
				record.cancelFunc = cancel

				log.Debugf("[%s] Starting health checks for backends", domain)
				go record.scrapeBackends(recordCtx, g)
			}
		}(group, time.Duration(i)*g.staggerDelay(len(groups)))
	}
	SetRecordsConfiguredTotal(float64(len(g.Records)))
}

func (g *GSLB) batchRecords(batchSize int) [][]*Record {
	var groups [][]*Record
	var current []*Record

	for domain, record := range g.Records {
		record.Fqdn = domain
		current = append(current, record)
		if len(current) == batchSize {
			groups = append(groups, current)
			current = nil
		}
	}

	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

func (g *GSLB) staggerDelay(totalBatches int) time.Duration {
	if totalBatches == 0 {
		return 0
	}
	return g.GetMaxStaggerStart() / time.Duration(totalBatches)
}

func (g *GSLB) updateLastResolutionTime(domain string) {
	g.LastResolution.Store(domain, time.Now())
}

func (g *GSLB) GetMaxStaggerStart() time.Duration {
	d, err := time.ParseDuration(g.MaxStaggerStart)
	if err != nil {
		d, _ = time.ParseDuration("60s")
	}
	return d
}

func (g *GSLB) GetResolutionIdleTimeout() time.Duration {
	d, err := time.ParseDuration(g.ResolutionIdleTimeout)
	if err != nil {
		d, _ = time.ParseDuration("3600s")
	}
	return d
}

func (g *GSLB) loadCustomLocationsMap(path string) error {
	g.Mutex.Lock()
	defer g.Mutex.Unlock()
	if path == "" {
		g.LocationMap = nil
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read location map: %v", err)
	}
	var parsed struct {
		Subnets []struct {
			Subnet   string `yaml:"subnet"`
			Location string `yaml:"location"`
		} `yaml:"subnets"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("failed to parse location map: %v", err)
	}
	m := make(map[string]string)
	for _, s := range parsed.Subnets {
		m[s.Subnet] = s.Location
	}
	g.LocationMap = m
	return nil
}
