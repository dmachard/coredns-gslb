package gslb

import (
	"fmt"
	"math/rand"
	"net"
	"sort"
	"time"

	"github.com/miekg/dns"
)

// pickBackendWithFailover returns all healthy backends with the lowest priority.
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
					IncBackendSelected(record.Fqdn, ip)
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

// pickBackendWithRoundRobin returns one healthy backend in round-robin order.
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
	IncBackendSelected(record.Fqdn, selectedBackend.GetAddress())

	return []string{selectedBackend.GetAddress()}, nil
}

// pickBackendWithRandom returns all healthy backends in random order.
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
		IncBackendSelected(record.Fqdn, backend.GetAddress())
	}

	return addresses, nil
}

// pickBackendWithGeoIP implements advanced GeoIP routing: country, city, ASN, custom location, with fallback to failover.
func (g *GSLB) pickBackendWithGeoIP(record *Record, recordType uint16, clientIP net.IP) ([]string, error) {
	// 1. Country-based routing (highest priority)
	if g.GeoIPCountryDB != nil {
		recordCountry, err := g.GeoIPCountryDB.Country(clientIP)
		if err == nil && recordCountry != nil && recordCountry.Country.IsoCode != "" {
			countryCode := recordCountry.Country.IsoCode
			var matchedIPs []string
			for _, backend := range record.Backends {
				if backend.IsHealthy() && backend.IsEnabled() {
					if backend.GetCountry() == countryCode {
						matchedIPs = append(matchedIPs, backend.GetAddress())
						IncBackendSelected(record.Fqdn, backend.GetAddress())
						break
					}
				}
			}
			if len(matchedIPs) > 0 {
				return matchedIPs, nil
			}
		}
	}

	// 2. City-based routing (if city DB loaded)
	if g.GeoIPCityDB != nil {
		recordCity, err := g.GeoIPCityDB.City(clientIP)
		if err == nil && recordCity != nil && recordCity.City.Names != nil {
			cityName := recordCity.City.Names["en"]
			if cityName != "" {
				var matchedIPs []string
				for _, backend := range record.Backends {
					if backend.IsHealthy() && backend.IsEnabled() {
						if backend.GetCity() == cityName {
							matchedIPs = append(matchedIPs, backend.GetAddress())
							IncBackendSelected(record.Fqdn, backend.GetAddress())
							break
						}
					}
				}
				if len(matchedIPs) > 0 {
					return matchedIPs, nil
				}
			}
		}
	}

	// 3. ASN-based routing (if ASN DB loaded)
	if g.GeoIPASNDB != nil {
		recordASN, err := g.GeoIPASNDB.ASN(clientIP)
		if err == nil && recordASN != nil && recordASN.AutonomousSystemNumber != 0 {
			asn := fmt.Sprint(recordASN.AutonomousSystemNumber)
			var matchedIPs []string
			for _, backend := range record.Backends {
				if backend.IsHealthy() && backend.IsEnabled() {
					if backend.GetASN() == asn {
						matchedIPs = append(matchedIPs, backend.GetAddress())
						IncBackendSelected(record.Fqdn, backend.GetAddress())
						break
					}
				}
			}
			if len(matchedIPs) > 0 {
				return matchedIPs, nil
			}
		}
	}

	// 4. Custom location map (subnet to location string)
	g.Mutex.RLock()
	locationMap := g.LocationMap
	g.Mutex.RUnlock()
	if len(locationMap) > 0 {
		var matchedIPs []string
		for _, backend := range record.Backends {
			if backend.IsHealthy() && backend.IsEnabled() {
				loc := backend.GetLocation()
				for subnet, location := range locationMap {
					_, ipnet, err := net.ParseCIDR(subnet)
					if err == nil && ipnet.Contains(clientIP) {
						if loc == location {
							matchedIPs = append(matchedIPs, backend.GetAddress())
							IncBackendSelected(record.Fqdn, backend.GetAddress())
							break
						}
						break
					}
				}
			}
		}
		if len(matchedIPs) > 0 {
			return matchedIPs, nil
		}
	}

	// 5. Fallback: failover (priority order)
	return g.pickBackendWithFailover(record, recordType)
}
