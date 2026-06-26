package gslb

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/miekg/dns"
)

var fqdnRegex = regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9]*(\.[a-zA-Z0-9][-a-zA-Z0-9]*)+\.?$`)

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
			if isAddressTypeCompatible(ip, recordType) {
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
			if isAddressTypeCompatible(ip, recordType) {
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
			if isAddressTypeCompatible(ip, recordType) {
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

// pickBackendWithWeighted returns one healthy backend, selected proportionally to its weight.
func (g *GSLB) pickBackendWithWeighted(record *Record, recordType uint16) ([]string, error) {
	var weightedBackends []BackendInterface
	var totalWeight int
	for _, backend := range record.Backends {
		if backend.IsHealthy() && backend.IsEnabled() {
			ip := backend.GetAddress()
			if isAddressTypeCompatible(ip, recordType) {
				w := backend.GetWeight()
				if w > 0 {
					weightedBackends = append(weightedBackends, backend)
					totalWeight += w
				}
			}
		}
	}
	if len(weightedBackends) == 0 || totalWeight == 0 {
		return nil, fmt.Errorf("no healthy backends with weight > 0 for type %d", recordType)
	}
	// Roulette wheel selection
	randVal := rand.Intn(totalWeight)
	cumulative := 0
	for _, backend := range weightedBackends {
		cumulative += backend.GetWeight()
		if randVal < cumulative {
			IncBackendSelected(record.Fqdn, backend.GetAddress())
			return []string{backend.GetAddress()}, nil
		}
	}
	// Should not reach here
	return nil, fmt.Errorf("weighted selection failed")
}

// pickBackendWithGeoIP implements advanced GeoIP routing: country, city, ASN, custom location, with fallback to failover.
func (g *GSLB) pickBackendWithGeoIP(record *Record, recordType uint16, clientIP net.IP) ([]string, error) {
	// 1. Geo hierarchy with city DB (city -> subdivision -> country -> continent)
	if g.GeoIPCityDB != nil {
		recordCity, err := g.GeoIPCityDB.City(clientIP)
		if err == nil && recordCity != nil {
			clientLatitude := recordCity.Location.Latitude
			clientLongitude := recordCity.Location.Longitude
			if nearestIPs, ok := g.pickNearestBackendByCoordinates(record, recordType, clientLatitude, clientLongitude); ok {
				for _, ip := range nearestIPs {
					IncBackendSelected(record.Fqdn, ip)
				}
				return nearestIPs, nil
			}

			cityName := ""
			if recordCity.City.Names != nil {
				cityName = recordCity.City.Names["en"]
			}
			subdivisionCode := ""
			if len(recordCity.Subdivisions) > 0 {
				subdivisionCode = strings.ToUpper(recordCity.Subdivisions[0].IsoCode)
			}
			continentCode := recordCity.Continent.Code
			countryCode := strings.ToUpper(recordCity.Country.IsoCode)

			// 1.a city
			if cityName != "" {
				var cityCountrySubdivisionIPs []string
				var cityCountryIPs []string
				var cityOnlyIPs []string
				for _, backend := range record.Backends {
					if !backend.IsHealthy() || !backend.IsEnabled() || !isAddressTypeCompatible(backend.GetAddress(), recordType) {
						continue
					}

					if !strings.EqualFold(backend.GetCity(), cityName) {
						continue
					}

					backendContinent := backend.GetContinent()
					backendCountry := strings.ToUpper(backend.GetCountry())
					backendSubdivision := strings.ToUpper(backend.GetSubdivision())

					// If backend provides a continent hint, it must match client geo.
					if backendContinent != "" && backendContinent != continentCode {
						continue
					}

					// If backend provides country/subdivision hints, they must match client geo.
					if backendCountry != "" && backendCountry != countryCode {
						continue
					}
					if backendSubdivision != "" && backendSubdivision != subdivisionCode {
						continue
					}

					switch {
					case backendCountry != "" && backendSubdivision != "":
						cityCountrySubdivisionIPs = append(cityCountrySubdivisionIPs, backend.GetAddress())
					case backendCountry != "":
						cityCountryIPs = append(cityCountryIPs, backend.GetAddress())
					default:
						cityOnlyIPs = append(cityOnlyIPs, backend.GetAddress())
					}
				}

				switch {
				case len(cityCountrySubdivisionIPs) > 0:
					for _, ip := range cityCountrySubdivisionIPs {
						IncBackendSelected(record.Fqdn, ip)
					}
					return cityCountrySubdivisionIPs, nil
				case len(cityCountryIPs) > 0:
					for _, ip := range cityCountryIPs {
						IncBackendSelected(record.Fqdn, ip)
					}
					return cityCountryIPs, nil
				case len(cityOnlyIPs) > 0:
					for _, ip := range cityOnlyIPs {
						IncBackendSelected(record.Fqdn, ip)
					}
					return cityOnlyIPs, nil
				}
			}

			// 1.b subdivision
			if subdivisionCode != "" {
				var subdivisionMatchedIPs []string
				for _, backend := range record.Backends {
					if !backend.IsHealthy() || !backend.IsEnabled() || !isAddressTypeCompatible(backend.GetAddress(), recordType) {
						continue
					}
					backendContinent := backend.GetContinent()
					if backendContinent != "" && backendContinent != continentCode {
						continue
					}
					if strings.ToUpper(backend.GetSubdivision()) != subdivisionCode {
						continue
					}
					// If backend declares a country, it must match client country.
					if backend.GetCountry() != "" && !strings.EqualFold(backend.GetCountry(), countryCode) {
						continue
					}
					subdivisionMatchedIPs = append(subdivisionMatchedIPs, backend.GetAddress())
				}
				if len(subdivisionMatchedIPs) > 0 {
					for _, ip := range subdivisionMatchedIPs {
						IncBackendSelected(record.Fqdn, ip)
					}
					return subdivisionMatchedIPs, nil
				}
			}

			// 1.c country
			if countryCode != "" {
				var countryMatchedIPs []string
				for _, backend := range record.Backends {
					if !backend.IsHealthy() || !backend.IsEnabled() || !isAddressTypeCompatible(backend.GetAddress(), recordType) {
						continue
					}
					backendContinent := backend.GetContinent()
					if backendContinent != "" && backendContinent != continentCode {
						continue
					}
					if strings.EqualFold(backend.GetCountry(), countryCode) {
						countryMatchedIPs = append(countryMatchedIPs, backend.GetAddress())
					}
				}
				if len(countryMatchedIPs) > 0 {
					for _, ip := range countryMatchedIPs {
						IncBackendSelected(record.Fqdn, ip)
					}
					return countryMatchedIPs, nil
				}
			}

			// 1.d continent
			if continentCode != "" {
				var continentMatchedIPs []string
				for _, backend := range record.Backends {
					if !backend.IsHealthy() || !backend.IsEnabled() || !isAddressTypeCompatible(backend.GetAddress(), recordType) {
						continue
					}
					if backend.GetContinent() == continentCode {
						continentMatchedIPs = append(continentMatchedIPs, backend.GetAddress())
					}
				}
				if len(continentMatchedIPs) > 0 {
					for _, ip := range continentMatchedIPs {
						IncBackendSelected(record.Fqdn, ip)
					}
					return continentMatchedIPs, nil
				}
			}
		}
	}

	// 2. Country-based routing with country DB (for country-only setups)
	if g.GeoIPCountryDB != nil {
		recordCountry, err := g.GeoIPCountryDB.Country(clientIP)
		if err == nil && recordCountry != nil {
			countryCode := strings.ToUpper(recordCountry.Country.IsoCode)
			continentCode := recordCountry.Continent.Code
			var matchedIPs []string
			if countryCode != "" {
				for _, backend := range record.Backends {
					if backend.IsHealthy() && backend.IsEnabled() && isAddressTypeCompatible(backend.GetAddress(), recordType) {
						backendContinent := backend.GetContinent()
						if backendContinent != "" && backendContinent != continentCode {
							continue
						}
						if strings.EqualFold(backend.GetCountry(), countryCode) {
							matchedIPs = append(matchedIPs, backend.GetAddress())
						}
					}
				}
			}
			if len(matchedIPs) > 0 {
				for _, ip := range matchedIPs {
					IncBackendSelected(record.Fqdn, ip)
				}
				return matchedIPs, nil
			}

			// 2.b continent (country DB also provides continent metadata)
			if continentCode != "" {
				for _, backend := range record.Backends {
					if backend.IsHealthy() && backend.IsEnabled() && isAddressTypeCompatible(backend.GetAddress(), recordType) {
						if backend.GetContinent() == continentCode {
							matchedIPs = append(matchedIPs, backend.GetAddress())
						}
					}
				}
			}
			if len(matchedIPs) > 0 {
				for _, ip := range matchedIPs {
					IncBackendSelected(record.Fqdn, ip)
				}
				return matchedIPs, nil
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
				if backend.IsHealthy() && backend.IsEnabled() && isAddressTypeCompatible(backend.GetAddress(), recordType) {
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
	needsRebuild := len(g.LocationMap) != len(g.LocationMapIPNet)
	g.Mutex.RUnlock()

	if needsRebuild {
		g.Mutex.Lock()
		if len(g.LocationMap) != len(g.LocationMapIPNet) {
			g.rebuildLocationMapIPNet()
		}
		g.Mutex.Unlock()
	}

	g.Mutex.RLock()
	locationMapIPNet := g.LocationMapIPNet
	g.Mutex.RUnlock()

	if len(locationMapIPNet) > 0 {
		var matchedIPs []string
		for _, backend := range record.Backends {
			if backend.IsHealthy() && backend.IsEnabled() && isAddressTypeCompatible(backend.GetAddress(), recordType) {
				loc := backend.GetLocation()
				for _, entry := range locationMapIPNet {
					if entry.IPNet.Contains(clientIP) {
						if loc == entry.Location {
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

func isAddressTypeCompatible(ip string, recordType uint16) bool {
	if ip == "" {
		return false
	}
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		if !fqdnRegex.MatchString(ip) {
			return false
		}
		return recordType == dns.TypeA || recordType == dns.TypeAAAA || recordType == dns.TypeCNAME
	}
	if recordType == dns.TypeA {
		return parsedIP.To4() != nil
	}
	if recordType == dns.TypeAAAA {
		return parsedIP.To16() != nil && parsedIP.To4() == nil
	}
	return false
}

func (g *GSLB) pickNearestBackendFromSubset(backends []BackendInterface, recordType uint16, clientLatitude, clientLongitude float64) ([]string, bool) {
	clientLatitudeRad := clientLatitude * math.Pi / 180
	clientLongitudeRad := clientLongitude * math.Pi / 180

	var bestAddresses []string
	bestDistance := 0.0
	bestPriority := 0
	found := false

	for _, backend := range backends {
		if !backend.IsHealthy() || !backend.IsEnabled() || !backend.HasGeoCoordinates() {
			continue
		}
		address := backend.GetAddress()
		if !isAddressTypeCompatible(address, recordType) {
			continue
		}

		distance := haversineDistanceRad(clientLatitudeRad, clientLongitudeRad, backend.GetLatitudeRad(), backend.GetLongitudeRad())
		priority := backend.GetPriority()
		if !found {
			bestAddresses = []string{address}
			bestDistance = distance
			bestPriority = priority
			found = true
			continue
		}
		if distance < bestDistance {
			bestAddresses = []string{address}
			bestDistance = distance
			bestPriority = priority
		} else if distance == bestDistance {
			if priority < bestPriority {
				bestAddresses = []string{address}
				bestPriority = priority
			} else if priority == bestPriority {
				bestAddresses = append(bestAddresses, address)
			}
		}
	}
	return bestAddresses, found
}

func (g *GSLB) pickNearestBackendByCoordinates(record *Record, recordType uint16, clientLatitude, clientLongitude float64) ([]string, bool) {
	return g.pickNearestBackendFromSubset(record.Backends, recordType, clientLatitude, clientLongitude)
}

type clientGeoInfo struct {
	City        string
	Subdivision string
	Country     string
	Continent   string
	Latitude    *float64
	Longitude   *float64
}

func (g *GSLB) getClientGeoInfo(clientIP net.IP) (*clientGeoInfo, error) {
	if g.GeoIPCityDB != nil {
		recordCity, err := g.GeoIPCityDB.City(clientIP)
		if err == nil && recordCity != nil {
			info := &clientGeoInfo{
				Continent: recordCity.Continent.Code,
				Country:   strings.ToUpper(recordCity.Country.IsoCode),
			}
			if recordCity.City.Names != nil {
				info.City = recordCity.City.Names["en"]
			}
			if len(recordCity.Subdivisions) > 0 {
				info.Subdivision = strings.ToUpper(recordCity.Subdivisions[0].IsoCode)
			}
			if recordCity.Location.Latitude != 0 || recordCity.Location.Longitude != 0 {
				lat := recordCity.Location.Latitude
				lon := recordCity.Location.Longitude
				info.Latitude = &lat
				info.Longitude = &lon
			}
			return info, nil
		}
	}
	if g.GeoIPCountryDB != nil {
		recordCountry, err := g.GeoIPCountryDB.Country(clientIP)
		if err == nil && recordCountry != nil {
			info := &clientGeoInfo{
				Continent: recordCountry.Continent.Code,
				Country:   strings.ToUpper(recordCountry.Country.IsoCode),
			}
			return info, nil
		}
	}
	return nil, fmt.Errorf("no geoip database or no match")
}

func (g *GSLB) pickBackendWithFailoverFromSubset(backends []BackendInterface, recordType uint16, fqdn string) ([]string, error) {
	sortedBackends := make([]BackendInterface, len(backends))
	copy(sortedBackends, backends)
	sort.Slice(sortedBackends, func(i, j int) bool {
		return sortedBackends[i].GetPriority() < sortedBackends[j].GetPriority()
	})

	minPriority := -1
	var healthyIPs []string
	for _, backend := range sortedBackends {
		if backend.IsHealthy() && backend.IsEnabled() {
			ip := backend.GetAddress()
			if isAddressTypeCompatible(ip, recordType) {
				if minPriority == -1 {
					minPriority = backend.GetPriority()
				}
				if backend.GetPriority() == minPriority {
					healthyIPs = append(healthyIPs, ip)
					IncBackendSelected(fqdn, ip)
				} else {
					break // stop at first higher priority
				}
			}
		}
	}

	if len(healthyIPs) == 0 {
		return nil, fmt.Errorf("no healthy backends in subset for type %d", recordType)
	}

	return healthyIPs, nil
}

func (g *GSLB) pickBackendWithGeoIPAffinity(record *Record, recordType uint16, clientIP net.IP) ([]string, error) {
	var candidates []BackendInterface

	// Step 1: Subnet Pinning
	g.Mutex.RLock()
	needsRebuildAffinity := len(g.LocationMap) != len(g.LocationMapIPNet)
	g.Mutex.RUnlock()

	if needsRebuildAffinity {
		g.Mutex.Lock()
		if len(g.LocationMap) != len(g.LocationMapIPNet) {
			g.rebuildLocationMapIPNet()
		}
		g.Mutex.Unlock()
	}

	g.Mutex.RLock()
	locationMapIPNet := g.LocationMapIPNet
	g.Mutex.RUnlock()

	var matchedLocation string
	if len(locationMapIPNet) > 0 {
		for _, entry := range locationMapIPNet {
			if entry.IPNet.Contains(clientIP) {
				matchedLocation = entry.Location
				break
			}
		}
	}

	if matchedLocation != "" {
		for _, backend := range record.Backends {
			if backend.IsHealthy() && backend.IsEnabled() && isAddressTypeCompatible(backend.GetAddress(), recordType) {
				if backend.GetLocation() == matchedLocation {
					candidates = append(candidates, backend)
				}
			}
		}
	}

	// Step 2: Geo Hierarchy Narrowing
	var geo *clientGeoInfo
	var geoErr error
	if len(candidates) == 0 {
		geo, geoErr = g.getClientGeoInfo(clientIP)
		if geoErr == nil && geo != nil {
			// City level
			if geo.City != "" {
				for _, backend := range record.Backends {
					if backend.IsHealthy() && backend.IsEnabled() && isAddressTypeCompatible(backend.GetAddress(), recordType) {
						if strings.EqualFold(backend.GetCity(), geo.City) {
							if backend.GetContinent() != "" && backend.GetContinent() != geo.Continent {
								continue
							}
							if backend.GetCountry() != "" && !strings.EqualFold(backend.GetCountry(), geo.Country) {
								continue
							}
							if backend.GetSubdivision() != "" && !strings.EqualFold(backend.GetSubdivision(), geo.Subdivision) {
								continue
							}
							candidates = append(candidates, backend)
						}
					}
				}
			}

			// Subdivision level
			if len(candidates) == 0 && geo.Subdivision != "" && geo.Country != "" {
				for _, backend := range record.Backends {
					if backend.IsHealthy() && backend.IsEnabled() && isAddressTypeCompatible(backend.GetAddress(), recordType) {
						if strings.EqualFold(backend.GetSubdivision(), geo.Subdivision) && strings.EqualFold(backend.GetCountry(), geo.Country) {
							if backend.GetContinent() != "" && backend.GetContinent() != geo.Continent {
								continue
							}
							candidates = append(candidates, backend)
						}
					}
				}
			}

			// Country level
			if len(candidates) == 0 && geo.Country != "" {
				for _, backend := range record.Backends {
					if backend.IsHealthy() && backend.IsEnabled() && isAddressTypeCompatible(backend.GetAddress(), recordType) {
						if strings.EqualFold(backend.GetCountry(), geo.Country) {
							if backend.GetContinent() != "" && backend.GetContinent() != geo.Continent {
								continue
							}
							candidates = append(candidates, backend)
						}
					}
				}
			}

			// Continent level
			if len(candidates) == 0 && geo.Continent != "" {
				for _, backend := range record.Backends {
					if backend.IsHealthy() && backend.IsEnabled() && isAddressTypeCompatible(backend.GetAddress(), recordType) {
						if strings.EqualFold(backend.GetContinent(), geo.Continent) {
							candidates = append(candidates, backend)
						}
					}
				}
			}
		}
	}

	// Step 3: Pick from candidate set using coordinate distance
	if len(candidates) > 0 {
		if geo == nil {
			geo, _ = g.getClientGeoInfo(clientIP)
		}
		if geo != nil && geo.Latitude != nil && geo.Longitude != nil {
			if nearestIPs, ok := g.pickNearestBackendFromSubset(candidates, recordType, *geo.Latitude, *geo.Longitude); ok {
				for _, ip := range nearestIPs {
					IncBackendSelected(record.Fqdn, ip)
				}
				return nearestIPs, nil
			}
		}
		ips, err := g.pickBackendWithFailoverFromSubset(candidates, recordType, record.Fqdn)
		if err == nil && len(ips) > 0 {
			return ips, nil
		}
	}

	// Step 4: Global Fallback
	if geo == nil {
		geo, _ = g.getClientGeoInfo(clientIP)
	}
	if geo != nil && geo.Latitude != nil && geo.Longitude != nil {
		if nearestIPs, ok := g.pickNearestBackendByCoordinates(record, recordType, *geo.Latitude, *geo.Longitude); ok {
			for _, ip := range nearestIPs {
				IncBackendSelected(record.Fqdn, ip)
			}
			return nearestIPs, nil
		}
	}

	return g.pickBackendWithFailover(record, recordType)
}

func haversineDistanceRad(lat1Rad, lon1Rad, lat2Rad, lon2Rad float64) float64 {
	const earthRadiusKm = 6371.0

	deltaLat := lat2Rad - lat1Rad
	deltaLon := lon2Rad - lon1Rad

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}
