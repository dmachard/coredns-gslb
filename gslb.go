package gslb

import (
	"context"
	"fmt"
	"net"
	"net/http"
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
	Next                plugin.Handler
	Zones               map[string]string             // List of authoritative domains
	Records             map[string]map[string]*Record // zone -> fqdn -> record
	HealthcheckProfiles map[string]*HealthCheck       `yaml:"healthcheck_profiles"`

	Zone                      string   // Zone attendue pour la vérification des records
	LastResolution            sync.Map // key: domain (string), value: time.Time
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
	APIEnable                 bool           // Enable/disable API HTTP server
	APICertPath               string         // TLS certificate path for API
	APIKeyPath                string         // TLS key path for API
	APIListenAddr             string         // API listen address (default 0.0.0.0)
	APIListenPort             string         // API listen port (default 8080)
	APIBasicUser              string         // HTTP Basic Auth username (optional)
	APIBasicPass              string         // HTTP Basic Auth password (optional)
	// DisableTXT disables TXT record resolution if set to true
	DisableTXT bool
}

func (g *GSLB) Name() string { return "gslb" }

// UnmarshalYAML implements custom YAML unmarshaling to handle healthcheck_profiles
func (g *GSLB) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw struct {
		Records             map[string]interface{}  `yaml:"records"`
		HealthcheckProfiles map[string]*HealthCheck `yaml:"healthcheck_profiles"`
	}

	if err := unmarshal(&raw); err != nil {
		return err
	}

	// Store healthcheck profiles
	if raw.HealthcheckProfiles != nil {
		g.HealthcheckProfiles = raw.HealthcheckProfiles
	}

	// Process records with healthcheck profile resolution
	if raw.Records != nil {
		if g.Records == nil {
			g.Records = make(map[string]map[string]*Record)
		}
		zone := g.Zone // zone attendue, ex: ".example.org."
		if g.Records[zone] == nil {
			g.Records[zone] = make(map[string]*Record)
		}
		for fqdn, recordData := range raw.Records {
			if zone != "" && !strings.HasSuffix(fqdn, zone) {
				return fmt.Errorf("record %s does not match zone %s", fqdn, zone)
			}
			// Pre-process the record data to resolve healthcheck profiles
			processedRecordData, err := g.processRecordHealthchecks(recordData)
			if err != nil {
				return fmt.Errorf("error processing record %s: %w", fqdn, err)
			}

			// Marshal and unmarshal the processed data to create the Record
			recordBytes, err := yaml.Marshal(processedRecordData)
			if err != nil {
				return fmt.Errorf("failed to marshal processed record %s: %w", fqdn, err)
			}

			var record Record
			if err := yaml.Unmarshal(recordBytes, &record); err != nil {
				return fmt.Errorf("failed to unmarshal record %s: %w", fqdn, err)
			}

			record.Fqdn = fqdn
			g.Records[zone][fqdn] = &record
		}
	}

	return nil
}

// processRecordHealthchecks processes a record to resolve healthcheck profile references
func (g *GSLB) processRecordHealthchecks(recordData interface{}) (interface{}, error) {
	recordMap, ok := recordData.(map[string]interface{})
	if !ok {
		return recordData, nil
	}

	backends, exists := recordMap["backends"]
	if !exists {
		return recordData, nil
	}

	backendsList, ok := backends.([]interface{})
	if !ok {
		return recordData, nil
	}

	// Process each backend
	for i, backend := range backendsList {
		backendMap, ok := backend.(map[string]interface{})
		if !ok {
			continue
		}

		healthchecks, exists := backendMap["healthchecks"]
		if !exists {
			continue
		}

		processedHealthchecks, err := g.processHealthchecks(healthchecks)
		if err != nil {
			return nil, err
		}

		backendMap["healthchecks"] = processedHealthchecks
		backendsList[i] = backendMap
	}

	recordMap["backends"] = backendsList
	return recordMap, nil
}

// processHealthchecks processes healthchecks to resolve profile references
func (g *GSLB) processHealthchecks(healthchecks interface{}) ([]interface{}, error) {
	var result []interface{}

	switch hc := healthchecks.(type) {
	case []interface{}:
		for _, item := range hc {
			switch v := item.(type) {
			case string:
				// It's a profile reference
				profile, err := ResolveHealthcheckProfile(v, g.HealthcheckProfiles)
				if err != nil {
					return nil, err
				}
				result = append(result, map[string]interface{}{
					"type":   profile.Type,
					"params": profile.Params,
				})
			default:
				// It's a full healthcheck object
				result = append(result, item)
			}
		}
	default:
		return nil, fmt.Errorf("healthchecks must be an array")
	}

	return result, nil
}

func (g *GSLB) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	// Get domain and ensure it is fully qualified
	q := r.Question[0]
	domain := strings.ToLower(dns.Fqdn(strings.TrimSuffix(q.Name, ".")))

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
		if g.DisableTXT {
			return plugin.NextOrFailure(g.Name(), g.Next, ctx, w, r)
		}
		return g.handleTXTRecord(ctx, w, r, domain)
	default:
		return plugin.NextOrFailure(g.Name(), g.Next, ctx, w, r)
	}
}

func (g *GSLB) ServeAPI() {
	mux := http.NewServeMux()
	g.RegisterAPIHandlers(mux)
	listenAddr := g.APIListenAddr + ":" + g.APIListenPort
	if g.APICertPath != "" && g.APIKeyPath != "" {
		go func() {
			_ = http.ListenAndServeTLS(listenAddr, g.APICertPath, g.APIKeyPath, mux)
		}()
	} else {
		go func() {
			_ = http.ListenAndServe(listenAddr, mux)
		}()
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
	domainNorm := strings.ToLower(strings.TrimSuffix(domain, ".")) + "."
	for authZone := range g.Zones {
		if strings.HasSuffix(domainNorm, authZone) {
			return true
		}
	}
	return false
}

func (g *GSLB) handleIPRecord(ctx context.Context, w dns.ResponseWriter, r *dns.Msg, domain string, recordType uint16) (int, error) {
	record, _ := g.findRecord(domain)
	if record == nil {
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
	record, _ := g.findRecord(domain)
	if record == nil {
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

		// Add last healthcheck timestamp if available
		lastHealthcheck := ""
		if b, ok := backend.(*Backend); ok {
			if !b.LastHealthcheck.IsZero() {
				lastHealthcheck = b.LastHealthcheck.Format(time.RFC3339)
			}
		}

		summary := fmt.Sprintf(
			"Backend: %s | Priority: %d | Status: %s | Enabled: %v | LastHealthcheck: %s",
			backend.GetAddress(), backend.GetPriority(), status, enabled, lastHealthcheck,
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
	record, _ := g.findRecord(domain)
	if record == nil {
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
	record, _ := g.findRecord(domain)
	if record == nil {
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
	for zone, newRecords := range newGSLB.Records {
		oldRecords, exists := g.Records[zone]
		if !exists {
			log.Infof("Not yet implemented: new zone %s", zone)
			continue
		}
		// This zone exists, update existing records
		for fqdn, newRecord := range newRecords {
			oldRecord, exists := oldRecords[fqdn]
			if !exists {
				newRecord.Fqdn = fqdn
				g.Records[zone][fqdn] = newRecord
				log.Infof("Added new record for zone %s: %s", zone, fqdn)
				newRecord.updateRecordHealthStatus()
				go newRecord.scrapeBackends(ctx, g)
			} else {
				log.Infof("Reloading record %s in zone %s", fqdn, zone)
				oldRecord.updateRecord(newRecord)
				oldRecord.updateRecordHealthStatus()
			}
		}
		// Remove records from old zone that are no longer present in newGSLB.Records
		for fqdn := range oldRecords {
			if _, exists := newRecords[fqdn]; !exists {
				if record := oldRecords[fqdn]; record.cancelFunc != nil {
					record.cancelFunc()
				}
				delete(g.Records[zone], fqdn)
				log.Infof("Records [%s] removed from zone %s", fqdn, zone)
			}
		}
	}

	// Update metrics
	g.updateMetrics()
}

func (g *GSLB) initializeRecordsFromFiles(ctx context.Context, zoneFiles map[string]string) {
	g.Records = make(map[string]map[string]*Record)
	for zone, file := range zoneFiles {
		log.Infof("Loading records for zone %s from %s", zone, file)
		if err := loadConfigFile(g, file, zone); err != nil {
			log.Errorf("Failed to load records for zone %s from %s: %v", zone, file, err)
			continue
		}
		log.Infof("Loaded %d records for zone %s", len(g.Records[zone]), zone)
	}
	groups := g.batchRecords(g.BatchSizeStart)
	for i, group := range groups {
		go func(group []*Record, delay time.Duration) {
			time.Sleep(delay)
			for _, record := range group {
				domain := record.Fqdn
				recordCtx, cancel := context.WithCancel(ctx)
				record.cancelFunc = cancel
				log.Debugf("[%s] Starting health checks for backends", domain)
				// Initialize health status for existing record
				record.updateRecordHealthStatus()
				go record.scrapeBackends(recordCtx, g)
			}
		}(group, time.Duration(i)*g.staggerDelay(len(groups)))
	}

	// Update metrics
	g.updateMetrics()
}

func (g *GSLB) updateMetrics() {
	SetZonesTotal(float64(len(g.Records)))

	// Set total records configured
	totalRecords := 0
	for _, records := range g.Records {
		totalRecords += len(records)
	}
	SetRecordsTotal(float64(totalRecords))

	// Set total backends configured
	totalBackends := 0
	for _, records := range g.Records {
		for _, record := range records {
			totalBackends += len(record.Backends)
		}
	}
	SetBackendsTotal(float64(totalBackends))

	// Set total healthchecks configured
	totalHealthchecks := 0
	for _, records := range g.Records {
		for _, record := range records {
			for _, backend := range record.Backends {
				totalHealthchecks += len(backend.GetHealthChecks())
			}
		}
	}
	SetHealthchecksTotal(float64(totalHealthchecks))
}

func (g *GSLB) batchRecords(batchSize int) [][]*Record {
	var groups [][]*Record
	var current []*Record

	for _, records := range g.Records {
		for domain, record := range records {
			record.Fqdn = domain
			current = append(current, record)
			if len(current) == batchSize {
				groups = append(groups, current)
				current = nil
			}
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
		return fmt.Errorf("failed to read location map: %w", err)
	}
	var parsed struct {
		Subnets []struct {
			Subnet   string `yaml:"subnet"`
			Location string `yaml:"location"`
		} `yaml:"subnets"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("failed to parse location map: %w", err)
	}
	m := make(map[string]string)
	for _, s := range parsed.Subnets {
		m[s.Subnet] = s.Location
	}
	g.LocationMap = m
	return nil
}

func (g *GSLB) findRecord(domain string) (*Record, string) {
	for zone, recs := range g.Records {
		if rec, ok := recs[domain]; ok {
			return rec, zone
		}
	}
	return nil, ""
}

func loadConfigFile(gslb *GSLB, fileName string, zone string) error {

	if !strings.HasSuffix(zone, ".") {
		zone = zone + "."
	}
	data, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("failed to read YAML configuration: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("failed to read YAML configuration: file empty")
	}
	var raw struct {
		Records             map[string]interface{}  `yaml:"records"`
		HealthcheckProfiles map[string]*HealthCheck `yaml:"healthcheck_profiles"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to parse YAML configuration: %w", err)
	}
	gslb.HealthcheckProfiles = raw.HealthcheckProfiles
	if gslb.Records == nil {
		gslb.Records = make(map[string]map[string]*Record)
	}
	if gslb.Records[zone] == nil {
		gslb.Records[zone] = make(map[string]*Record)
	}
	for fqdn, recordData := range raw.Records {
		if zone != "" && !strings.HasSuffix(fqdn, zone) {
			return fmt.Errorf("record %s does not match zone %s", fqdn, zone)
		}
		processedRecordData, err := (&GSLB{HealthcheckProfiles: raw.HealthcheckProfiles}).processRecordHealthchecks(recordData)
		if err != nil {
			return fmt.Errorf("error processing record %s: %w", fqdn, err)
		}
		recordBytes, err := yaml.Marshal(processedRecordData)
		if err != nil {
			return fmt.Errorf("failed to marshal processed record %s: %w", fqdn, err)
		}
		var record Record
		if err := yaml.Unmarshal(recordBytes, &record); err != nil {
			return fmt.Errorf("failed to unmarshal record %s: %w", fqdn, err)
		}
		record.Fqdn = fqdn
		gslb.Records[zone][fqdn] = &record
	}
	return nil
}
