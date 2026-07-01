package gslb

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/miekg/dns"
)

func (g *GSLB) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	// Get domain and ensure it is fully qualified
	q := r.Question[0]
	domain := strings.ToLower(dns.Fqdn(strings.TrimSuffix(q.Name, ".")))

	g.Mutex.RLock()
	isAuth := g.isAuthoritative(domain)
	g.Mutex.RUnlock()

	// If the domain doesn't match any authoritative domain, pass to the next plugin
	if !isAuth {
		return plugin.NextOrFailure(g.Name(), g.Next, ctx, w, r)
	}

	// Determine the client IP and prefix length (ECS or RemoteAddr fallback)
	clientIP, clientPrefixLen := g.extractClientIP(w, r)
	if clientIP == nil {
		log.Error("Failed to determine client IP, responding with SERVFAIL")
		return dns.RcodeServerFailure, nil
	}
	ctx = WithClientInfo(ctx, clientIP, clientPrefixLen)

	if g.UseEDNSCSubnet {
		w = &ecsResponseWriter{ResponseWriter: w, g: g, reqMsg: r}
	}

	// Update the last resolution time for the domain
	// This is used to track when the last resolution was made for a domain
	g.updateLastResolutionTime(domain)

	g.Mutex.RLock()
	record, _ := g.findRecord(domain)
	if record == nil {
		g.Mutex.RUnlock()
		return plugin.NextOrFailure(g.Name(), g.Next, ctx, w, r)
	}

	switch q.Qtype {
	case dns.TypeA:
		res, err := g.handleIPRecord(ctx, w, r, domain, dns.TypeA)
		g.Mutex.RUnlock()
		return res, err
	case dns.TypeAAAA:
		res, err := g.handleIPRecord(ctx, w, r, domain, dns.TypeAAAA)
		g.Mutex.RUnlock()
		return res, err
	case dns.TypeCNAME:
		res, err := g.handleIPRecord(ctx, w, r, domain, dns.TypeCNAME)
		g.Mutex.RUnlock()
		return res, err
	case dns.TypeTXT:
		if g.DisableTXT {
			g.Mutex.RUnlock()
			return plugin.NextOrFailure(g.Name(), g.Next, ctx, w, r)
		}
		res, err := g.handleTXTRecord(ctx, w, r, domain)
		g.Mutex.RUnlock()
		return res, err
	case dns.TypeSVCB:
		res, err := g.handleSVCBRecord(ctx, w, r, domain, dns.TypeSVCB)
		g.Mutex.RUnlock()
		return res, err
	case dns.TypeHTTPS:
		res, err := g.handleSVCBRecord(ctx, w, r, domain, dns.TypeHTTPS)
		g.Mutex.RUnlock()
		return res, err
	case dns.TypeSRV:
		res, err := g.handleSRVRecord(ctx, w, r, domain)
		g.Mutex.RUnlock()
		return res, err
	default:
		g.Mutex.RUnlock()
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
	domainNorm := strings.ToLower(strings.TrimSuffix(domain, ".")) + "."
	for authZone := range g.Zones {
		if strings.HasSuffix(domainNorm, authZone) {
			return true
		}
	}
	return false
}

func (g *GSLB) hasRecordTypeConfigured(record *Record, recordType uint16) bool {
	if strings.ToLower(record.FailoverPolicy.Mode) == "fail-specific" && record.FailoverPolicy.FallbackCNAME != "" {
		if recordType == dns.TypeA || recordType == dns.TypeAAAA || recordType == dns.TypeCNAME {
			return true
		}
	}
	for _, backend := range record.Backends {
		if isAddressTypeCompatible(backend.GetAddress(), recordType) {
			return true
		}
	}
	for _, ip := range record.FailoverPolicy.FallbackIPs {
		if isAddressTypeCompatible(ip, recordType) {
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
	if !g.hasRecordTypeConfigured(record, recordType) {
		return g.sendRcodeResponse(w, r, domain, dns.RcodeSuccess, true)
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
		ObserveRecordResolutionDuration(domain, "fail", time.Since(start).Seconds())

		policyMode := strings.ToLower(record.FailoverPolicy.Mode)
		if policyMode == "" {
			policyMode = "fail-open"
		}

		g.logFailSafeWarning(domain, policyMode)

		switch policyMode {
		case "fail-closed":
			rcode := dns.RcodeServerFailure
			switch strings.ToUpper(record.FailoverPolicy.Rcode) {
			case "NXDOMAIN":
				rcode = dns.RcodeNameError
			case "REFUSED":
				rcode = dns.RcodeRefused
			case "NOERROR":
				rcode = dns.RcodeSuccess
			case "SERVFAIL":
				rcode = dns.RcodeServerFailure
			}
			return g.sendRcodeResponse(w, r, domain, rcode, true)

		case "fail-specific":
			if record.FailoverPolicy.FallbackCNAME != "" {
				return g.sendAddressRecordResponse(w, r, domain, []string{record.FailoverPolicy.FallbackCNAME}, record.RecordTTL, recordType)
			}
			fallbackIPs, err := g.pickFallbackAddresses(record, recordType)
			if err != nil {
				log.Debugf("Error retrieving fallback IPs for domain %s: %v", domain, err)
				return dns.RcodeServerFailure, nil
			}
			return g.sendAddressRecordResponse(w, r, domain, fallbackIPs, record.RecordTTL, recordType)

		case "fail-open":
			fallthrough
		default:
			// Fallback: get all IP addresses
			ipAddresses, err := g.pickAllAddresses(domain, recordType)
			if err != nil {
				log.Debugf("Error retrieving backends for domain %s: %v", domain, err)
				return dns.RcodeServerFailure, nil
			}
			return g.sendAddressRecordResponse(w, r, domain, ipAddresses, record.RecordTTL, recordType)
		}
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

	g.decorateWithECS(r, response, domain, true)

	// Send the DNS response with the multiple TXT records
	if err := w.WriteMsg(response); err != nil {
		log.Error("Failed to write DNS TXT response: ", err)
		return dns.RcodeServerFailure, err
	}

	// Return success
	return dns.RcodeSuccess, nil
}

func (g *GSLB) handleSVCBRecord(ctx context.Context, w dns.ResponseWriter, r *dns.Msg, domain string, recordType uint16) (int, error) {
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

	// Pick IPv4 and IPv6 hint addresses using the selected load-balancing/failover policy
	ipv4ips, errA := g.pickResponse(domain, dns.TypeA, ci.IP)
	ipv6ips, errAAAA := g.pickResponse(domain, dns.TypeAAAA, ci.IP)

	var ips []string
	var ipsV6 []string

	// Check if both failed
	if errA != nil && errAAAA != nil {
		log.Debugf("[%s] no backend available for SVCB/HTTPS: %v, %v", domain, errA, errAAAA)
		ObserveRecordResolutionDuration(domain, "fail", time.Since(start).Seconds())

		policyMode := strings.ToLower(record.FailoverPolicy.Mode)
		if policyMode == "" {
			policyMode = "fail-open"
		}

		g.logFailSafeWarning(domain, policyMode)

		switch policyMode {
		case "fail-closed":
			rcode := dns.RcodeServerFailure
			switch strings.ToUpper(record.FailoverPolicy.Rcode) {
			case "NXDOMAIN":
				rcode = dns.RcodeNameError
			case "REFUSED":
				rcode = dns.RcodeRefused
			case "NOERROR":
				rcode = dns.RcodeSuccess
			case "SERVFAIL":
				rcode = dns.RcodeServerFailure
			}
			return g.sendRcodeResponse(w, r, domain, rcode, true)

		case "fail-specific":
			if record.FailoverPolicy.FallbackCNAME != "" {
				return g.sendAddressRecordResponse(w, r, domain, []string{record.FailoverPolicy.FallbackCNAME}, record.RecordTTL, dns.TypeCNAME)
			}
			fallbackA, _ := g.pickFallbackAddresses(record, dns.TypeA)
			fallbackAAAA, _ := g.pickFallbackAddresses(record, dns.TypeAAAA)
			ips = fallbackA
			ipsV6 = fallbackAAAA

		case "fail-open":
			fallthrough
		default:
			allA, _ := g.pickAllAddresses(domain, dns.TypeA)
			allAAAA, _ := g.pickAllAddresses(domain, dns.TypeAAAA)
			ips = allA
			ipsV6 = allAAAA
		}
	} else {
		if errA == nil {
			ips = ipv4ips
		}
		if errAAAA == nil {
			ipsV6 = ipv6ips
		}
	}

	// If we still have no IPs at all, let's send a success response with no answers (NODATA)
	if len(ips) == 0 && len(ipsV6) == 0 {
		ObserveRecordResolutionDuration(domain, "success", time.Since(start).Seconds())
		return g.sendRcodeResponse(w, r, domain, dns.RcodeSuccess, true)
	}

	// Construct the SVCB / HTTPS record response
	response := new(dns.Msg)
	response.SetReply(r)

	var pairs []dns.SVCBKeyValue

	// 1. ALPN (defaulting to h3, h2 if not configured)
	alpnVals := record.ALPN
	if len(alpnVals) == 0 {
		alpnVals = []string{"h3", "h2"}
	}
	pairs = append(pairs, &dns.SVCBAlpn{Alpn: alpnVals})

	// 2. Port (map dynamically to the backend's configured/discovered port)
	var svcPort uint16 = 0
	for _, backend := range record.Backends {
		if backend.GetPort() > 0 {
			svcPort = uint16(backend.GetPort())
			break
		}
	}
	if svcPort > 0 {
		pairs = append(pairs, &dns.SVCBPort{Port: svcPort})
	}

	// 3. IPv4 Hints
	if len(ips) > 0 {
		var v4hints []net.IP
		for _, ipStr := range ips {
			if ip := net.ParseIP(ipStr); ip != nil {
				v4hints = append(v4hints, ip)
			}
		}
		if len(v4hints) > 0 {
			pairs = append(pairs, &dns.SVCBIPv4Hint{Hint: v4hints})
		}
	}

	// 4. IPv6 Hints
	if len(ipsV6) > 0 {
		var v6hints []net.IP
		for _, ipStr := range ipsV6 {
			if ip := net.ParseIP(ipStr); ip != nil {
				v6hints = append(v6hints, ip)
			}
		}
		if len(v6hints) > 0 {
			pairs = append(pairs, &dns.SVCBIPv6Hint{Hint: v6hints})
		}
	}

	// Sort SvcParams by key code to comply with RFC 9460
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Key() < pairs[j].Key()
	})

	var rr dns.RR
	hdr := dns.RR_Header{
		Name:   domain,
		Rrtype: recordType,
		Class:  dns.ClassINET,
		Ttl:    uint32(record.RecordTTL),
	}

	if recordType == dns.TypeSVCB {
		rr = &dns.SVCB{
			Hdr:      hdr,
			Priority: 1,
			Target:   ".",
			Value:    pairs,
		}
	} else {
		rr = &dns.HTTPS{
			SVCB: dns.SVCB{
				Hdr:      hdr,
				Priority: 1,
				Target:   ".",
				Value:    pairs,
			},
		}
	}

	response.Answer = append(response.Answer, rr)

	g.decorateWithECS(r, response, domain, false)

	err := w.WriteMsg(response)
	if err != nil {
		log.Error("Failed to write DNS SVCB/HTTPS response: ", err)
		IncRecordResolutions(domain, "fail")
		return dns.RcodeServerFailure, err
	}

	ObserveRecordResolutionDuration(domain, "success", time.Since(start).Seconds())
	IncRecordResolutions(domain, "success")
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
			if isAddressTypeCompatible(ip, recordType) {
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
	case "geoip_affinity":
		return g.pickBackendWithGeoIPAffinity(record, recordType, clientIP)
	case "weighted":
		return g.pickBackendWithWeighted(record, recordType)
	case "ip-hash":
		return g.pickBackendWithHash(record, recordType, clientIP)
	default:
		return nil, fmt.Errorf("unsupported mode: %s", record.Mode)
	}
}

func (g *GSLB) sendAddressRecordResponse(w dns.ResponseWriter, r *dns.Msg, domain string, ipAddresses []string, ttl int, recordType uint16) (int, error) {
	response := new(dns.Msg)
	response.SetReply(r)
	for _, ip := range ipAddresses {
		var rr dns.RR
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			rr = &dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   domain,
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    uint32(ttl),
				},
				Target: dns.Fqdn(ip),
			}
		} else {
			switch recordType {
			case dns.TypeA:
				rr = &dns.A{
					Hdr: dns.RR_Header{
						Name:   domain,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    uint32(ttl),
					},
					A: parsedIP,
				}
			case dns.TypeAAAA:
				rr = &dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   domain,
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    uint32(ttl),
					},
					AAAA: parsedIP,
				}
			}
		}
		if rr != nil {
			response.Answer = append(response.Answer, rr)
		}
	}

	g.decorateWithECS(r, response, domain, false)

	err := w.WriteMsg(response)
	if err != nil {
		log.Error("Failed to write DNS response: ", err)
		IncRecordResolutions(domain, "fail")
		return dns.RcodeServerFailure, err
	}
	IncRecordResolutions(domain, "success")
	return dns.RcodeSuccess, nil
}

func (g *GSLB) logFailSafeWarning(domain string, policyMode string) {
	key := "logfail:" + domain + ":" + policyMode
	now := time.Now()
	if val, ok := g.LastResolution.Load(key); ok {
		if lastLog, ok := val.(time.Time); ok && now.Sub(lastLog) < 10*time.Second {
			return
		}
	}
	g.LastResolution.Store(key, now)
	log.Warningf("[%s] no healthy backends available, applying failover policy: %s", domain, policyMode)
}

func (g *GSLB) sendRcodeResponse(w dns.ResponseWriter, r *dns.Msg, domain string, rcode int, forceGlobalScope bool) (int, error) {
	response := new(dns.Msg)
	response.SetReply(r)
	response.Rcode = rcode
	g.decorateWithECS(r, response, domain, forceGlobalScope)

	err := w.WriteMsg(response)
	if err != nil {
		log.Error("Failed to write DNS rcode response: ", err)
		IncRecordResolutions(domain, "fail")
		return dns.RcodeServerFailure, err
	}
	IncRecordResolutions(domain, "success")
	return dns.RcodeSuccess, nil
}

func (g *GSLB) decorateWithECS(r *dns.Msg, response *dns.Msg, domain string, forceGlobalScope bool) {
	if !g.UseEDNSCSubnet {
		return
	}
	o := r.IsEdns0()
	if o == nil {
		return
	}
	var reqEcs *dns.EDNS0_SUBNET
	for _, option := range o.Option {
		if ecs, ok := option.(*dns.EDNS0_SUBNET); ok {
			reqEcs = ecs
			break
		}
	}
	if reqEcs == nil {
		return
	}

	// Determine if the response is geo-specific.
	// When forceGlobalScope is true, the response came from a downstream plugin
	// (fallthrough) and is static/global — scope must be /0 per RFC 7871 §7.3.1.
	sourceScope := uint8(0)
	if !forceGlobalScope {
		record, _ := g.findRecord(domain)
		if record != nil {
			if record.Mode == "geoip" || record.Mode == "geoip_affinity" || record.Mode == "hash" || record.Mode == "ip-hash" || record.Mode == "client-ip-hash" {
				sourceScope = reqEcs.SourceNetmask
			} else if len(g.LocationMap) > 0 {
				sourceScope = reqEcs.SourceNetmask
			}
		}
	}

	respEcs := &dns.EDNS0_SUBNET{
		Code:          dns.EDNS0SUBNET,
		Family:        reqEcs.Family,
		SourceNetmask: reqEcs.SourceNetmask,
		SourceScope:   sourceScope,
		Address:       reqEcs.Address,
	}

	// Create or find the OPT record in response.Extra
	var respOpt *dns.OPT
	for _, extra := range response.Extra {
		if opt, ok := extra.(*dns.OPT); ok {
			respOpt = opt
			break
		}
	}

	if respOpt == nil {
		respOpt = &dns.OPT{
			Hdr: dns.RR_Header{
				Name:   ".",
				Rrtype: dns.TypeOPT,
			},
		}
		respOpt.SetUDPSize(o.UDPSize())
		response.Extra = append(response.Extra, respOpt)
	}

	// Check if EDNS0_SUBNET is already in response options, if not, append it
	found := false
	for _, opt := range respOpt.Option {
		if _, ok := opt.(*dns.EDNS0_SUBNET); ok {
			found = true
			break
		}
	}
	if !found {
		respOpt.Option = append(respOpt.Option, respEcs)
	}
}

func (g *GSLB) pickFallbackAddresses(record *Record, recordType uint16) ([]string, error) {
	var ipAddresses []string
	for _, ip := range record.FailoverPolicy.FallbackIPs {
		if isAddressTypeCompatible(ip, recordType) {
			ipAddresses = append(ipAddresses, ip)
		}
	}
	if len(ipAddresses) == 0 {
		return nil, fmt.Errorf("no fallback backends exist for domain: %s", record.Fqdn)
	}
	return ipAddresses, nil
}

func (g *GSLB) findRecord(domain string) (*Record, string) {
	// Exact match first
	for zone, recs := range g.Records {
		if rec, ok := recs[domain]; ok {
			return rec, zone
		}
	}

	// Wildcard fallback: find the most specific authoritative zone for the domain
	var bestZone string
	for zone := range g.Records {
		if strings.HasSuffix(domain, zone) {
			if len(zone) > len(bestZone) {
				bestZone = zone
			}
		}
	}

	if bestZone != "" {
		recs := g.Records[bestZone]
		for i := 0; i < len(domain); i++ {
			if domain[i] == '.' {
				suffix := domain[i+1:]
				if !strings.HasSuffix(suffix, bestZone) {
					break
				}
				wildcardPattern := "*." + suffix
				if rec, ok := recs[wildcardPattern]; ok {
					return rec, bestZone
				}
			}
		}
	}

	return nil, ""
}

func (g *GSLB) handleSRVRecord(ctx context.Context, w dns.ResponseWriter, r *dns.Msg, domain string) (int, error) {
	record, _ := g.findRecord(domain)
	if record == nil {
		return plugin.NextOrFailure(g.Name(), g.Next, ctx, w, r)
	}
	if !g.hasRecordTypeConfigured(record, dns.TypeSRV) {
		return g.sendRcodeResponse(w, r, domain, dns.RcodeSuccess, true)
	}
	ci := GetClientInfo(ctx)
	if ci == nil || ci.IP == nil {
		log.Error("No client info in context")
		return dns.RcodeServerFailure, nil
	}
	start := time.Now()

	selectedAddresses, err := g.pickResponse(domain, dns.TypeSRV, ci.IP)
	if err != nil {
		log.Debugf("[%s] no backend available for SRV: %v", domain, err)
		ObserveRecordResolutionDuration(domain, "fail", time.Since(start).Seconds())

		policyMode := strings.ToLower(record.FailoverPolicy.Mode)
		if policyMode == "" {
			policyMode = "fail-open"
		}

		g.logFailSafeWarning(domain, policyMode)

		switch policyMode {
		case "fail-closed":
			rcode := dns.RcodeServerFailure
			switch strings.ToUpper(record.FailoverPolicy.Rcode) {
			case "NXDOMAIN":
				rcode = dns.RcodeNameError
			case "REFUSED":
				rcode = dns.RcodeRefused
			case "NOERROR":
				rcode = dns.RcodeSuccess
			case "SERVFAIL":
				rcode = dns.RcodeServerFailure
			}
			return g.sendRcodeResponse(w, r, domain, rcode, true)

		case "fail-specific":
			fallbackIPs, err := g.pickFallbackAddresses(record, dns.TypeSRV)
			if err != nil {
				log.Debugf("Error retrieving fallback addresses for domain %s: %v", domain, err)
				return dns.RcodeServerFailure, nil
			}
			selectedAddresses = fallbackIPs

		case "fail-open":
			fallthrough
		default:
			allAddresses, err := g.pickAllAddresses(domain, dns.TypeSRV)
			if err != nil {
				log.Debugf("Error retrieving backends for domain %s: %v", domain, err)
				return dns.RcodeServerFailure, nil
			}
			selectedAddresses = allAddresses
		}
	}

	response := new(dns.Msg)
	response.SetReply(r)

	for _, addr := range selectedAddresses {
		var port, priority, weight int
		var target string = addr

		var found BackendInterface
		for _, b := range record.Backends {
			if b.GetAddress() == addr {
				found = b
				break
			}
		}

		if found != nil {
			port = found.GetPort()
			priority = found.GetPriority()
			weight = found.GetWeight()
		} else {
			port = 0
			priority = 0
			weight = 1
		}

		srv := &dns.SRV{
			Hdr: dns.RR_Header{
				Name:   domain,
				Rrtype: dns.TypeSRV,
				Class:  dns.ClassINET,
				Ttl:    uint32(record.RecordTTL),
			},
			Priority: uint16(priority),
			Weight:   uint16(weight),
			Port:     uint16(port),
			Target:   dns.Fqdn(target),
		}
		response.Answer = append(response.Answer, srv)
	}

	g.decorateWithECS(r, response, domain, false)

	err = w.WriteMsg(response)
	if err != nil {
		log.Error("Failed to write DNS SRV response: ", err)
		IncRecordResolutions(domain, "fail")
		return dns.RcodeServerFailure, err
	}

	ObserveRecordResolutionDuration(domain, "success", time.Since(start).Seconds())
	IncRecordResolutions(domain, "success")
	return dns.RcodeSuccess, nil
}
