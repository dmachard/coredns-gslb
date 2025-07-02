package gslb

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"os"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
	"gopkg.in/yaml.v3"
)

// Define log to be a logger with the plugin name in it. T
// his way we can just use log.Info and friends to log.
var log = clog.NewWithPlugin("gslb")

type GSLB struct {
	Next                  plugin.Handler
	Zones                 map[string]string  // List of authoritative domains
	Records               map[string]*Record `yaml:"records"`
	LastResolution        sync.Map           // key: domain (string), value: time.Time
	RoundRobinIndex       sync.Map
	MaxStaggerStart       string
	BatchSizeStart        int
	ResolutionIdleTimeout string
	Mutex                 sync.RWMutex
	UseEDNSCSubnet        bool
	LocationMap           map[string]string
}

func (g *GSLB) Name() string { return "gslb" }

// ServeDNS implements the plugin.Handler interface. This method gets called when example is used
// in a Server.
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
	log.Debugf("Client IP: %s, PrefixLength: %d", clientIP, clientPrefixLen)

	// Update the last resolution time
	g.updateLastResolutionTime(domain)

	// Process the DNS request if it's an authoritative domain
	switch q.Qtype {
	case dns.TypeA:
		return g.handleIPRecord(ctx, w, r, domain, dns.TypeA)
	case dns.TypeAAAA:
		return g.handleIPRecord(ctx, w, r, domain, dns.TypeAAAA)
	case dns.TypeTXT:
		return g.handleTXTRecord(ctx, w, r, domain)
	default:
		// Forward other requests to the next plugin
		return plugin.NextOrFailure(g.Name(), g.Next, ctx, w, r)
	}
}

// extractClientIP extracts the client's IP and prefix length from EDNS or the remote address.
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

// handleAddressRecord handles both A and AAAA record queries for the domain.
func (g *GSLB) handleIPRecord(ctx context.Context, w dns.ResponseWriter, r *dns.Msg, domain string, recordType uint16) (int, error) {
	record, exists := g.Records[domain]
	if !exists {
		return plugin.NextOrFailure(g.Name(), g.Next, ctx, w, r)
	}

	// Try to get an appropriate response
	ip, err := g.pickResponse(domain, recordType)
	if err != nil {
		log.Debugf("[%s] no backend available for type %d: %v", domain, recordType, err)

		// Fallback: get all IP addresses
		ipAddresses, err := g.pickAllAddresses(domain, recordType)
		if err != nil {
			log.Debugf("Error retrieving backends for domain %s: %v", domain, err)
			return dns.RcodeServerFailure, nil
		}

		return g.sendAddressRecordResponse(w, r, domain, ipAddresses, record.RecordTTL, recordType)
	}

	return g.sendAddressRecordResponse(w, r, domain, ip, record.RecordTTL, recordType)
}

// handleTXTRecord handles TXT record queries for the domain.
// It returns a TXT record for each backend, summarizing its address, priority, health, and enabled status.
// This is useful for debugging and monitoring: you can query the TXT record for a domain to see backend states.
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

// pickAllByType returns all the IP addresses of backends defined for a given domain, filtered by type (A or AAAA).
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

func (g *GSLB) pickResponse(domain string, recordType uint16) ([]string, error) {
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
		return g.pickBackendWithGeoIP(record, recordType)
	default:
		return nil, fmt.Errorf("unsupported mode: %s", record.Mode)
	}
}

func (g *GSLB) pickBackendWithFailover(record *Record, recordType uint16) ([]string, error) {
	sortedBackends := make([]BackendInterface, len(record.Backends))
	copy(sortedBackends, record.Backends)
	sort.Slice(sortedBackends, func(i, j int) bool {
		return sortedBackends[i].GetPriority() < sortedBackends[j].GetPriority()
	})

	minPriority := -1
	var healthyIPs []string
	for _, backend := range sortedBackends {
		if backend.IsHealthy() {
			ip := backend.GetAddress()
			if (recordType == dns.TypeA && net.ParseIP(ip).To4() != nil) ||
				(recordType == dns.TypeAAAA && net.ParseIP(ip).To16() != nil && net.ParseIP(ip).To4() == nil) {
				if minPriority == -1 {
					minPriority = backend.GetPriority()
				}
				if backend.GetPriority() == minPriority {
					healthyIPs = append(healthyIPs, ip)
				} else {
					break // stop at first higher priority
				}
			}
		}
	}

	if len(healthyIPs) == 0 {
		return nil, fmt.Errorf("no healthy backends in failover mode for type %d", recordType)
	}

	return healthyIPs, nil
}

func (g *GSLB) pickBackendWithRoundRobin(domain string, record *Record, recordType uint16) ([]string, error) {
	g.Mutex.Lock()
	defer g.Mutex.Unlock()

	var index int
	value, exists := g.RoundRobinIndex.Load(domain)
	if exists {
		index = value.(int)
	}

	healthyBackends := []BackendInterface{}
	for _, backend := range record.Backends {
		if backend.IsHealthy() {
			ip := backend.GetAddress()
			if (recordType == dns.TypeA && net.ParseIP(ip).To4() != nil) ||
				(recordType == dns.TypeAAAA && net.ParseIP(ip).To16() != nil && net.ParseIP(ip).To4() == nil) {
				healthyBackends = append(healthyBackends, backend)
			}
		}
	}

	if len(healthyBackends) == 0 {
		return nil, fmt.Errorf("no healthy backends in round-robin mode for type %d", recordType)
	}

	selectedBackend := healthyBackends[index%len(healthyBackends)]
	g.RoundRobinIndex.Store(domain, (index+1)%len(healthyBackends))

	return []string{selectedBackend.GetAddress()}, nil
}

func (g *GSLB) pickBackendWithRandom(record *Record, recordType uint16) ([]string, error) {
	g.Mutex.Lock()
	defer g.Mutex.Unlock()

	healthyBackends := []BackendInterface{}
	for _, backend := range record.Backends {
		if backend.IsHealthy() {
			ip := backend.GetAddress()
			if (recordType == dns.TypeA && net.ParseIP(ip).To4() != nil) ||
				(recordType == dns.TypeAAAA && net.ParseIP(ip).To16() != nil && net.ParseIP(ip).To4() == nil) {
				healthyBackends = append(healthyBackends, backend)
			}
		}
	}

	if len(healthyBackends) == 0 {
		return nil, fmt.Errorf("no healthy backends in random mode for type %d", recordType)
	}

	// Shuffle healthy backends to create random order
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(healthyBackends), func(i, j int) {
		healthyBackends[i], healthyBackends[j] = healthyBackends[j], healthyBackends[i]
	})

	// Collect the shuffled IPs
	addresses := []string{}
	for _, backend := range healthyBackends {
		addresses = append(addresses, backend.GetAddress())
	}

	return addresses, nil
}

// pickBackendWithGeoIP selects the backend(s) based on the client's location using the LocationMap.
// It returns the IP(s) of the backend(s) matching the client's location, or falls back to healthy backends if no match.
func (g *GSLB) pickBackendWithGeoIP(record *Record, recordType uint16) ([]string, error) {
	// Try to get the client IP from the last request (thread-safe, but not ideal for concurrent queries)
	// In a real implementation, you would pass the client IP as a parameter or store it in context.
	// For now, we use the last resolution time as a proxy for the last client IP (not perfect, but works for plugin context).
	// This is a placeholder: you may want to refactor to pass client IP directly.
	// For now, we just pick the first healthy backend matching the location.

	g.Mutex.RLock()
	locationMap := g.LocationMap
	g.Mutex.RUnlock()

	if len(locationMap) == 0 {
		return nil, fmt.Errorf("location map is not loaded")
	}

	// This is a placeholder: in a real plugin, you should pass the client IP to this function.
	// Here, we just pick the first healthy backend with a location match.
	// For demo, we try all backends and match their address to a location.

	var matchedIPs []string
	for _, backend := range record.Backends {
		if backend.IsHealthy() && backend.IsEnabled() {
			ip := backend.GetAddress()
			if (recordType == dns.TypeA && net.ParseIP(ip).To4() != nil) ||
				(recordType == dns.TypeAAAA && net.ParseIP(ip).To16() != nil && net.ParseIP(ip).To4() == nil) {
				// Check if backend IP is in the location map (simulate match)
				for subnet := range locationMap {
					_, ipnet, err := net.ParseCIDR(subnet)
					if err == nil && ipnet.Contains(net.ParseIP(ip)) {
						matchedIPs = append(matchedIPs, ip)
						break
					}
				}
			}
		}
	}

	if len(matchedIPs) > 0 {
		return matchedIPs, nil
	}

	// Fallback: return all healthy backends
	return g.pickBackendWithFailover(record, recordType)
}

func (g *GSLB) sendAddressRecordResponse(w dns.ResponseWriter, r *dns.Msg, domain string, ipAddresses []string, ttl int, recordType uint16) (int, error) {
	response := new(dns.Msg)
	response.SetReply(r)
	for _, ip := range ipAddresses {
		var rr dns.RR
		if recordType == dns.TypeA {
			rr = &dns.A{
				Hdr: dns.RR_Header{
					Name:   domain,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    uint32(ttl),
				},
				A: net.ParseIP(ip),
			}
		} else if recordType == dns.TypeAAAA {
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
		return dns.RcodeServerFailure, err
	}

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

// loadLocationMap loads and parses the location map YAML file into the in-memory LocationMap.
// This method is thread-safe.
func (g *GSLB) loadLocationMap(path string) error {
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
