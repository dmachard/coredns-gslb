package gslb

import (
	"math"
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
	"github.com/stretchr/testify/assert"
)

func TestGSLB_PickBackendWithFailover_IPv4(t *testing.T) {
	// Create mock backends with different priorities and health statuses
	backendHealthy := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true, Priority: 10}}
	backendUnhealthy := &MockBackend{Backend: &Backend{Address: "192.168.1.2", Enable: true, Priority: 20}}

	// Mock the behavior of the IsHealthy method
	backendHealthy.On("IsHealthy").Return(true)
	backendUnhealthy.On("IsHealthy").Return(false)

	// Create a record
	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "failover",
		Backends: []BackendInterface{backendHealthy, backendUnhealthy},
	}

	// Create the GSLB object
	g := &GSLB{}

	// Test the pickFailoverBackend method
	ipAddresses, err := g.pickBackendWithFailover(record, dns.TypeA)

	// Assert the results
	assert.NoError(t, err, "Expected pickFailoverBackend to succeed")
	assert.Equal(t, "192.168.1.1", ipAddresses[0], "Expected the healthy backend to be selected")
}

func TestGSLB_PickBackendWithFailover_IPv6(t *testing.T) {
	// Create mock backends with different priorities and health statuses
	backendHealthy := &MockBackend{Backend: &Backend{Address: "2001:db8::1", Enable: true, Priority: 10}}
	backendUnhealthy := &MockBackend{Backend: &Backend{Address: "2001:db8::2", Enable: true, Priority: 20}}

	// Mock the behavior of the IsHealthy method
	backendHealthy.On("IsHealthy").Return(true)
	backendUnhealthy.On("IsHealthy").Return(false)

	// Create a record
	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "failover",
		Backends: []BackendInterface{backendHealthy, backendUnhealthy},
	}

	// Create the GSLB object
	g := &GSLB{}

	// Test the pickFailoverBackend method
	ipAddresses, err := g.pickBackendWithFailover(record, dns.TypeAAAA)

	// Assert the results
	assert.NoError(t, err, "Expected pickFailoverBackend to succeed")
	assert.Equal(t, "2001:db8::1", ipAddresses[0], "Expected the healthy backend to be selected")
}

func TestGSLB_PickBackendWithFailover_MultipleSamePriority(t *testing.T) {
	// Deux backends healthy, même priorité
	backendHealthy1 := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true, Priority: 10}}
	backendHealthy2 := &MockBackend{Backend: &Backend{Address: "192.168.1.2", Enable: true, Priority: 10}}
	backendUnhealthy := &MockBackend{Backend: &Backend{Address: "192.168.1.3", Enable: true, Priority: 20}}

	backendHealthy1.On("IsHealthy").Return(true)
	backendHealthy2.On("IsHealthy").Return(true)
	backendUnhealthy.On("IsHealthy").Return(false)

	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "failover",
		Backends: []BackendInterface{backendHealthy1, backendHealthy2, backendUnhealthy},
	}

	g := &GSLB{}

	ipAddresses, err := g.pickBackendWithFailover(record, dns.TypeA)

	assert.NoError(t, err, "Expected pickBackendWithFailover to succeed")
	assert.Len(t, ipAddresses, 2, "Expected two healthy backends of same priority to be returned")
	assert.Contains(t, ipAddresses, "192.168.1.1")
	assert.Contains(t, ipAddresses, "192.168.1.2")
}

func TestGSLB_PickBackendWithRoundRobin_IPv4(t *testing.T) {
	// Create mock backends with IPv4 addresses
	backend1 := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true}}
	backend2 := &MockBackend{Backend: &Backend{Address: "192.168.1.2", Enable: true}}
	backend3 := &MockBackend{Backend: &Backend{Address: "192.168.1.3", Enable: true}}

	// Mock the behavior of the IsHealthy method
	backend1.On("IsHealthy").Return(true)
	backend2.On("IsHealthy").Return(true)
	backend3.On("IsHealthy").Return(true)

	// Create a record with healthy backends
	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "round-robin",
		Backends: []BackendInterface{backend1, backend2, backend3},
	}

	// Create the GSLB object
	g := &GSLB{}

	// Perform the first selection; index should be 0
	ipAddresses, err := g.pickBackendWithRoundRobin("example.com.", record, dns.TypeA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "192.168.1.1", ipAddresses[0], "Expected the first backend to be selected")

	// Perform the second selection; index should be 1
	ipAddresses, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "192.168.1.2", ipAddresses[0], "Expected the second backend to be selected")

	// Perform the third selection; index should be 2
	ipAddresses, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "192.168.1.3", ipAddresses[0], "Expected the third backend to be selected")

	// Perform the fourth selection; index should wrap back to 0
	ipAddresses, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "192.168.1.1", ipAddresses[0], "Expected the first backend to be selected again")
}

func TestGSLB_PickBackendWithRoundRobin_IPv6(t *testing.T) {
	// Create mock backends with IPv6 addresses
	backend1 := &MockBackend{Backend: &Backend{Address: "2001:db8::1", Enable: true}}
	backend2 := &MockBackend{Backend: &Backend{Address: "2001:db8::2", Enable: true}}
	backend3 := &MockBackend{Backend: &Backend{Address: "2001:db8::3", Enable: true}}

	// Mock the behavior of the IsHealthy method
	backend1.On("IsHealthy").Return(true)
	backend2.On("IsHealthy").Return(true)
	backend3.On("IsHealthy").Return(true)

	// Create a record with healthy backends
	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "round-robin",
		Backends: []BackendInterface{backend1, backend2, backend3},
	}

	// Create the GSLB object
	g := &GSLB{}

	// Perform the first selection; index should be 0
	ipAddresses, err := g.pickBackendWithRoundRobin("example.com.", record, dns.TypeAAAA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "2001:db8::1", ipAddresses[0], "Expected the first IPv6 backend to be selected")

	// Perform the second selection; index should be 1
	ipAddresses, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeAAAA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "2001:db8::2", ipAddresses[0], "Expected the second IPv6 backend to be selected")

	// Perform the third selection; index should be 2
	ipAddresses, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeAAAA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "2001:db8::3", ipAddresses[0], "Expected the third IPv6 backend to be selected")

	// Perform the fourth selection; index should wrap back to 0
	ipAddresses, err = g.pickBackendWithRoundRobin("example.com.", record, dns.TypeAAAA)
	assert.NoError(t, err, "Expected pickBackendWithRoundRobin to succeed")
	assert.Equal(t, "2001:db8::1", ipAddresses[0], "Expected the first IPv6 backend to be selected again")
}

func TestGSLB_PickBackendWithRandom_IPv4(t *testing.T) {
	// Create mock backends
	backend1 := &MockBackend{Backend: &Backend{Address: "192.168.1.1", Enable: true}}
	backend2 := &MockBackend{Backend: &Backend{Address: "192.168.1.2", Enable: true}}
	backend3 := &MockBackend{Backend: &Backend{Address: "192.168.1.3", Enable: true}}

	// Mock the behavior of the IsHealthy method
	backend1.On("IsHealthy").Return(true)
	backend2.On("IsHealthy").Return(true)
	backend3.On("IsHealthy").Return(true)

	// Create a record
	record := &Record{
		Fqdn:     "example.com.",
		Mode:     "random",
		Backends: []BackendInterface{backend1, backend2, backend3},
	}

	// Create the GSLB object
	g := &GSLB{}

	// Perform the random selection multiple times
	selectedIPs := make(map[string]bool)
	for i := 0; i < 10; i++ {
		ipAddresses, err := g.pickBackendWithRandom(record, dns.TypeA)
		assert.NoError(t, err, "Expected pickBackendWithRandom to succeed")
		for _, ip := range ipAddresses {
			selectedIPs[ip] = true
		}
	}

	// Assert that the IPs are from the healthy backends
	assert.GreaterOrEqual(t, len(selectedIPs), 2, "Expected at least two different backends to be selected randomly")
	assert.Contains(t, selectedIPs, "192.168.1.1", "Expected IP 192.168.1.1 to be selected")
	assert.Contains(t, selectedIPs, "192.168.1.2", "Expected IP 192.168.1.2 to be selected")
	assert.Contains(t, selectedIPs, "192.168.1.3", "Expected IP 192.168.1.3 to be selected")
}

func TestGSLB_PickBackendWithGeoIP_CustomDB(t *testing.T) {
	locationMap := map[string]string{
		"10.0.0.0/24":    "eu-west",
		"192.168.1.0/24": "us-east",
	}

	backendEU := &MockBackend{Backend: &Backend{Address: "10.0.0.42", Enable: true, Priority: 10, Location: "eu-west"}}
	backendUS := &MockBackend{Backend: &Backend{Address: "192.168.1.42", Enable: true, Priority: 20, Location: "us-east"}}
	backendOther := &MockBackend{Backend: &Backend{Address: "172.16.0.1", Enable: true, Priority: 30, Location: "other"}}
	backendEU.On("IsHealthy").Return(true)
	backendUS.On("IsHealthy").Return(true)
	backendOther.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendEU, backendUS, backendOther},
	}

	g := &GSLB{
		LocationMap: locationMap,
	}

	testCases := []struct {
		name     string
		clientIP string
		expect   []string
	}{
		{"us-east subnet", "192.168.1.50", []string{"192.168.1.42"}},
		{"eu-west subnet", "10.0.0.50", []string{"10.0.0.42"}},
		{"us-east subnet 2", "192.168.1.100", []string{"192.168.1.42"}},
		{"eu-west subnet 2", "10.0.0.200", []string{"10.0.0.42"}},
		{"unmatched IP fallback", "8.8.8.8", []string{"10.0.0.42"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP(tc.clientIP))
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, ips)
		})
	}

	// Test fallback when LocationMap is nil
	g.LocationMap = nil
	t.Run("fallback no location map", func(t *testing.T) {
		ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP("8.8.8.8"))
		assert.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.42"}, ips)
	})
}

func TestGSLB_PickBackendWithGeoIP_Country_MaxMind(t *testing.T) {
	db, err := geoip2.Open("tests/GeoLite2-Country.mmdb")
	if err != nil {
		t.Skip("GeoLite2-Country.mmdb not found, skipping real MaxMind test")
	}
	defer db.Close()

	backendUS := &MockBackend{Backend: &Backend{Address: "20.0.0.1", Enable: true, Priority: 10, Country: "US"}}
	backendAU := &MockBackend{Backend: &Backend{Address: "30.0.0.1", Enable: true, Priority: 20, Country: "AU"}}
	backendOther := &MockBackend{Backend: &Backend{Address: "40.0.0.1", Enable: true, Priority: 30, Country: "DE"}}
	backendUS.On("IsHealthy").Return(true)
	backendAU.On("IsHealthy").Return(true)
	backendOther.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendUS, backendAU, backendOther},
	}

	g := &GSLB{
		GeoIPCountryDB: db,
	}

	testCases := []struct {
		name     string
		clientIP string
		expect   []string
	}{
		{"US IP", "8.8.8.8", []string{"20.0.0.1"}},
		{"AU IP", "1.144.110.23", []string{"30.0.0.1"}},
		{"Unknown country fallback", "127.0.0.1", []string{"20.0.0.1"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP(tc.clientIP))
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, ips)
		})
	}
}

func TestGSLB_PickBackendWithGeoIP_Country_MaxMind_ContinentOnly(t *testing.T) {
	db, err := geoip2.Open("tests/GeoLite2-Country.mmdb")
	if err != nil {
		t.Skip("GeoLite2-Country.mmdb not found, skipping real MaxMind test")
	}
	defer db.Close()

	backendEU := &MockBackend{Backend: &Backend{Address: "50.0.0.1", Enable: true, Priority: 10, Continent: "EU"}}
	backendNA := &MockBackend{Backend: &Backend{Address: "60.0.0.1", Enable: true, Priority: 20, Continent: "NA"}}
	backendEU.On("IsHealthy").Return(true)
	backendNA.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo-continent.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendEU, backendNA},
	}

	g := &GSLB{
		GeoIPCountryDB: db,
	}

	testCases := []struct {
		name     string
		clientIP string
		expect   []string
	}{
		{"EU IP", "81.185.159.80", []string{"50.0.0.1"}}, // France
		{"NA IP", "8.8.8.8", []string{"60.0.0.1"}},       // United States
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP(tc.clientIP))
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, ips)
		})
	}
}

func TestGSLB_PickBackendWithGeoIP_CitySubdivisionCountryHierarchy(t *testing.T) {
	db, err := geoip2.Open("tests/GeoLite2-City.mmdb")
	if err != nil {
		t.Skip("GeoLite2-City.mmdb not found, skipping city/subdivision hierarchy test")
	}
	defer db.Close()

	g := &GSLB{
		GeoIPCityDB: db,
	}

	clientIP := net.ParseIP("9.9.9.9") // US, subdivision CA, city Berkeley in tests DB

	t.Run("city match wins over subdivision and country", func(t *testing.T) {
		backendCity := &MockBackend{Backend: &Backend{Address: "10.0.0.1", Enable: true, Priority: 10, City: "Berkeley"}}
		backendSubdivision := &MockBackend{Backend: &Backend{Address: "10.0.0.2", Enable: true, Priority: 20, Country: "US", Subdivision: "CA"}}
		backendCountry := &MockBackend{Backend: &Backend{Address: "10.0.0.3", Enable: true, Priority: 30, Country: "US"}}
		backendCity.On("IsHealthy").Return(true)
		backendSubdivision.On("IsHealthy").Return(true)
		backendCountry.On("IsHealthy").Return(true)

		record := &Record{
			Fqdn:     "geo-hierarchy.example.com.",
			Mode:     "geoip",
			Backends: []BackendInterface{backendCity, backendSubdivision, backendCountry},
		}

		ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, clientIP)
		assert.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.1"}, ips)
	})

	t.Run("city match is geo-aware and prefers most specific backend", func(t *testing.T) {
		backendCityExact := &MockBackend{Backend: &Backend{Address: "10.0.0.1", Enable: true, Priority: 10, City: "Berkeley", Country: "US", Subdivision: "CA"}}
		backendCityCountry := &MockBackend{Backend: &Backend{Address: "10.0.0.2", Enable: true, Priority: 20, City: "Berkeley", Country: "US"}}
		backendCityWrongCountry := &MockBackend{Backend: &Backend{Address: "10.0.0.3", Enable: true, Priority: 30, City: "Berkeley", Country: "GB"}}
		backendCityWrongSubdivision := &MockBackend{Backend: &Backend{Address: "10.0.0.4", Enable: true, Priority: 40, City: "Berkeley", Country: "US", Subdivision: "NY"}}
		backendCityExact.On("IsHealthy").Return(true)
		backendCityCountry.On("IsHealthy").Return(true)
		backendCityWrongCountry.On("IsHealthy").Return(true)
		backendCityWrongSubdivision.On("IsHealthy").Return(true)

		record := &Record{
			Fqdn: "geo-hierarchy.example.com.",
			Mode: "geoip",
			Backends: []BackendInterface{
				backendCityExact,
				backendCityCountry,
				backendCityWrongCountry,
				backendCityWrongSubdivision,
			},
		}

		ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, clientIP)
		assert.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.1"}, ips)
	})

	t.Run("subdivision fallback wins when city has no match", func(t *testing.T) {
		backendSubdivision := &MockBackend{Backend: &Backend{Address: "10.0.0.2", Enable: true, Priority: 20, Country: "US", Subdivision: "CA"}}
		backendCountry := &MockBackend{Backend: &Backend{Address: "10.0.0.3", Enable: true, Priority: 30, Country: "US"}}
		backendSubdivision.On("IsHealthy").Return(true)
		backendCountry.On("IsHealthy").Return(true)

		record := &Record{
			Fqdn:     "geo-hierarchy.example.com.",
			Mode:     "geoip",
			Backends: []BackendInterface{backendSubdivision, backendCountry},
		}

		ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, clientIP)
		assert.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.2"}, ips)
	})

	t.Run("country fallback returns all country backends", func(t *testing.T) {
		backendCountry1 := &MockBackend{Backend: &Backend{Address: "10.0.0.3", Enable: true, Priority: 30, Country: "US"}}
		backendCountry2 := &MockBackend{Backend: &Backend{Address: "10.0.0.4", Enable: true, Priority: 40, Country: "US"}}
		backendCountry1.On("IsHealthy").Return(true)
		backendCountry2.On("IsHealthy").Return(true)

		record := &Record{
			Fqdn:     "geo-hierarchy.example.com.",
			Mode:     "geoip",
			Backends: []BackendInterface{backendCountry1, backendCountry2},
		}

		ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, clientIP)
		assert.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.3", "10.0.0.4"}, ips)
	})
}

func TestGSLB_PickBackendWithGeoIP_City_MaxMind(t *testing.T) {
	db, err := geoip2.Open("tests/GeoLite2-City.mmdb")
	if err != nil {
		t.Skip("GeoLite2-City.mmdb not found, skipping real MaxMind city test")
	}
	defer db.Close()

	backendParis := &MockBackend{Backend: &Backend{Address: "10.10.10.1", Enable: true, Priority: 10, City: "Paris"}}
	backendBerlin := &MockBackend{Backend: &Backend{Address: "20.20.20.1", Enable: true, Priority: 20, City: "Berlin"}}
	backendOther := &MockBackend{Backend: &Backend{Address: "30.30.30.1", Enable: true, Priority: 30, City: "OtherCity"}}
	backendParis.On("IsHealthy").Return(true)
	backendBerlin.On("IsHealthy").Return(true)
	backendOther.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendParis, backendBerlin, backendOther},
	}

	g := &GSLB{
		GeoIPCityDB: db,
	}

	testCases := []struct {
		name     string
		clientIP string
		expect   []string
	}{
		{"Paris IP", "81.185.159.80", []string{"10.10.10.1"}},        // IP in Paris
		{"Berlin IP", "141.20.20.1", []string{"20.20.20.1"}},         // IP in Berlin
		{"Unknown city fallback", "8.8.8.8", []string{"10.10.10.1"}}, // fallback to lowest priority
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP(tc.clientIP))
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, ips)
		})
	}
}

func TestGSLB_PickBackendWithGeoIP_City_MaxMind_ContinentOnly(t *testing.T) {
	db, err := geoip2.Open("tests/GeoLite2-City.mmdb")
	if err != nil {
		t.Skip("GeoLite2-City.mmdb not found, skipping real MaxMind city test")
	}
	defer db.Close()

	backendEU := &MockBackend{Backend: &Backend{Address: "70.0.0.1", Enable: true, Priority: 10, Continent: "EU"}}
	backendNA := &MockBackend{Backend: &Backend{Address: "80.0.0.1", Enable: true, Priority: 20, Continent: "NA"}}
	backendEU.On("IsHealthy").Return(true)
	backendNA.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo-continent-city.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendEU, backendNA},
	}

	g := &GSLB{
		GeoIPCityDB: db,
	}

	testCases := []struct {
		name     string
		clientIP string
		expect   []string
	}{
		{"EU city IP", "141.20.20.1", []string{"70.0.0.1"}}, // Berlin
		{"NA city IP", "9.9.9.9", []string{"80.0.0.1"}},     // Berkeley
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP(tc.clientIP))
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, ips)
		})
	}
}

func TestGSLB_PickBackendWithGeoIP_CoordinatesNearest_Backend(t *testing.T) {
	db, err := geoip2.Open("tests/GeoLite2-City.mmdb")
	if err != nil {
		t.Skip("GeoLite2-City.mmdb not found, skipping coordinate distance test")
	}
	defer db.Close()

	backendNear := &MockBackend{Backend: &Backend{
		Address:        "90.0.0.1",
		Enable:         true,
		Priority:       20,
		Longitude:      -122.2727, // Oakland, CA
		Latitude:       37.8044,
		LongitudeRad:   -122.2727 * math.Pi / 180,
		LatitudeRad:    37.8044 * math.Pi / 180,
		HasCoordinates: true,
	}}
	backendFar := &MockBackend{Backend: &Backend{
		Address:        "91.0.0.1",
		Enable:         true,
		Priority:       10,
		Longitude:      2.3522, // Paris, FR
		Latitude:       48.8566,
		LongitudeRad:   2.3522 * math.Pi / 180,
		LatitudeRad:    48.8566 * math.Pi / 180,
		HasCoordinates: true,
	}}
	backendNear.On("IsHealthy").Return(true)
	backendFar.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo-coordinates.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendNear, backendFar},
	}

	g := &GSLB{
		GeoIPCityDB: db,
	}

	ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP("9.9.9.9")) // Berkeley, CA in test DB
	assert.NoError(t, err)
	assert.Equal(t, []string{"90.0.0.1"}, ips)
}

func TestGSLB_PickBackendWithGeoIP_MultipleNearest_Coordinates(t *testing.T) {
	db, err := geoip2.Open("tests/GeoLite2-City.mmdb")
	if err != nil {
		t.Skip("GeoLite2-City.mmdb not found, skipping multiple nearest test")
	}
	defer db.Close()

	backendNear1 := &MockBackend{Backend: &Backend{
		Address:        "90.0.0.1",
		Enable:         true,
		Priority:       20,
		Longitude:      -122.2727, // Oakland, CA
		Latitude:       37.8044,
		LongitudeRad:   -122.2727 * math.Pi / 180,
		LatitudeRad:    37.8044 * math.Pi / 180,
		HasCoordinates: true,
	}}
	backendNear2 := &MockBackend{Backend: &Backend{
		Address:        "90.0.0.2",
		Enable:         true,
		Priority:       20,
		Longitude:      -122.2727, // Oakland, CA
		Latitude:       37.8044,
		LongitudeRad:   -122.2727 * math.Pi / 180,
		LatitudeRad:    37.8044 * math.Pi / 180,
		HasCoordinates: true,
	}}
	backendFar := &MockBackend{Backend: &Backend{
		Address:        "91.0.0.1",
		Enable:         true,
		Priority:       10,
		Longitude:      2.3522, // Paris, FR
		Latitude:       48.8566,
		LongitudeRad:   2.3522 * math.Pi / 180,
		LatitudeRad:    48.8566 * math.Pi / 180,
		HasCoordinates: true,
	}}
	backendNear1.On("IsHealthy").Return(true)
	backendNear2.On("IsHealthy").Return(true)
	backendFar.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo-coordinates-multiple.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendNear1, backendNear2, backendFar},
	}

	g := &GSLB{
		GeoIPCityDB: db,
	}

	ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP("9.9.9.9")) // Berkeley, CA in test DB
	assert.NoError(t, err)
	assert.Len(t, ips, 2)
	assert.Contains(t, ips, "90.0.0.1")
	assert.Contains(t, ips, "90.0.0.2")
}

func TestGSLB_PickBackendWithGeoIP_CoordinatesNearest_BackendUnavailableFallbackToNext(t *testing.T) {
	db, err := geoip2.Open("tests/GeoLite2-City.mmdb")
	if err != nil {
		t.Skip("GeoLite2-City.mmdb not found, skipping coordinate fallback test")
	}
	defer db.Close()

	backendNearUnavailable := &MockBackend{Backend: &Backend{
		Address:        "92.0.0.1",
		Enable:         true,
		Priority:       20,
		Longitude:      -122.2727, // Oakland, CA
		Latitude:       37.8044,
		LongitudeRad:   -122.2727 * math.Pi / 180,
		LatitudeRad:    37.8044 * math.Pi / 180,
		HasCoordinates: true,
	}}
	backendNext := &MockBackend{Backend: &Backend{
		Address:        "93.0.0.1",
		Enable:         true,
		Priority:       10,
		Longitude:      -74.0060, // New York, US
		Latitude:       40.7128,
		LongitudeRad:   -74.0060 * math.Pi / 180,
		LatitudeRad:    40.7128 * math.Pi / 180,
		HasCoordinates: true,
	}}
	backendNearUnavailable.On("IsHealthy").Return(false)
	backendNext.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo-coordinates-fallback.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendNearUnavailable, backendNext},
	}

	g := &GSLB{
		GeoIPCityDB: db,
	}

	ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP("9.9.9.9")) // Berkeley, CA in test DB
	assert.NoError(t, err)
	assert.Equal(t, []string{"93.0.0.1"}, ips)
}

func TestGSLB_PickBackendWithGeoIP_ASN_MaxMind(t *testing.T) {
	db, err := geoip2.Open("tests/GeoLite2-ASN.mmdb")
	if err != nil {
		t.Skip("GeoLite2-ASN.mmdb not found, skipping real MaxMind ASN test")
	}
	defer db.Close()

	backendGoogle := &MockBackend{Backend: &Backend{Address: "8.8.8.8", Enable: true, Priority: 10, ASN: "15169"}}     // Google ASN
	backendCloudflare := &MockBackend{Backend: &Backend{Address: "1.1.1.1", Enable: true, Priority: 20, ASN: "13335"}} // Cloudflare ASN
	backendOther := &MockBackend{Backend: &Backend{Address: "9.9.9.9", Enable: true, Priority: 30, ASN: "0"}}
	backendGoogle.On("IsHealthy").Return(true)
	backendCloudflare.On("IsHealthy").Return(true)
	backendOther.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendGoogle, backendCloudflare, backendOther},
	}

	g := &GSLB{
		GeoIPASNDB: db,
	}

	testCases := []struct {
		name     string
		clientIP string
		expect   []string
	}{
		{"Google ASN IP", "8.8.8.8", []string{"8.8.8.8"}},
		{"Cloudflare ASN IP", "1.1.1.1", []string{"1.1.1.1"}},
		{"Unknown ASN fallback", "9.9.9.9", []string{"8.8.8.8"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP(tc.clientIP))
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, ips)
		})
	}
}

func TestGSLB_PickBackendWithWeighted(t *testing.T) {
	backend1 := &MockBackend{Backend: &Backend{Address: "10.0.0.1", Enable: true, Weight: 5}}
	backend2 := &MockBackend{Backend: &Backend{Address: "10.0.0.2", Enable: true, Weight: 1}}
	backend3 := &MockBackend{Backend: &Backend{Address: "10.0.0.3", Enable: true, Weight: 4}}

	backend1.On("IsHealthy").Return(true)
	backend2.On("IsHealthy").Return(true)
	backend3.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "weighted.example.com.",
		Mode:     "weighted",
		Backends: []BackendInterface{backend1, backend2, backend3},
	}
	g := &GSLB{}

	// Simuler 10 000 sélections pour vérifier la répartition
	selections := map[string]int{}
	n := 10000
	for i := 0; i < n; i++ {
		ips, err := g.pickBackendWithWeighted(record, dns.TypeA)
		assert.NoError(t, err)
		assert.Len(t, ips, 1)
		selections[ips[0]]++
	}
	// Les proportions attendues sont 5:1:4
	// On tolère une marge de +/-10%
	expected := map[string]float64{
		"10.0.0.1": 0.5,
		"10.0.0.2": 0.1,
		"10.0.0.3": 0.4,
	}
	for addr, exp := range expected {
		frac := float64(selections[addr]) / float64(n)
		assert.InDelta(t, exp, frac, 0.05, "Backend %s: got %.2f, expected %.2f", addr, frac, exp)
	}
}

func TestGSLB_PickBackendWithGeoIPAffinity(t *testing.T) {
	cityDB, err := geoip2.Open("tests/GeoLite2-City.mmdb")
	if err != nil {
		t.Skip("GeoLite2-City.mmdb not found, skipping TestGSLB_PickBackendWithGeoIPAffinity")
	}
	defer cityDB.Close()

	g := &GSLB{
		GeoIPCityDB: cityDB,
	}

	// 1. Test case: Subnet Pinning wins over global distance.
	// Paris client IP is 81.185.159.80.
	// We have two backends:
	// - backendUS: us-east, New York coordinates (40.7128, -74.0060), Address 1.2.3.4
	// - backendEU: eu-west, Berlin coordinates (52.5200, 13.4050), Address 5.6.7.8
	// Berlin is closer to Paris than New York is.
	// BUT, if we have a location map mapping Paris client IP's subnet to "us-east":
	// "81.185.159.0/24": "us-east",
	// Then geoip_affinity should select backendUS!
	t.Run("Subnet Pinning wins", func(t *testing.T) {
		g.Mutex.Lock()
		g.LocationMap = map[string]string{
			"81.185.159.0/24": "us-east",
		}
		g.Mutex.Unlock()

		backendUS := &MockBackend{Backend: &Backend{
			Address: "1.2.3.4", Enable: true, Priority: 10, Location: "us-east",
			Latitude: 40.7128, Longitude: -74.0060, HasCoordinates: true,
		}}
		backendEU := &MockBackend{Backend: &Backend{
			Address: "5.6.7.8", Enable: true, Priority: 20, Location: "eu-west",
			Latitude: 52.52, Longitude: 13.405, HasCoordinates: true,
		}}
		backendUS.recomputeCoordinateRadians()
		backendEU.recomputeCoordinateRadians()

		backendUS.On("IsHealthy").Return(true)
		backendEU.On("IsHealthy").Return(true)

		record := &Record{
			Fqdn:     "affinity.example.com.",
			Mode:     "geoip_affinity",
			Backends: []BackendInterface{backendUS, backendEU},
		}

		ips, err := g.pickBackendWithGeoIPAffinity(record, dns.TypeA, net.ParseIP("81.185.159.80"))
		assert.NoError(t, err)
		assert.Equal(t, []string{"1.2.3.4"}, ips)
	})

	// 2. Test case: Country Preference narrowing.
	// Client IP: 81.185.159.80 (FR).
	// Backends:
	// - backendFR: Country FR, coordinates in Paris (48.8566, 2.3522), Address 10.0.0.1
	// - backendDE: Country DE, coordinates in Berlin (52.5200, 13.4050), Address 20.0.0.1
	// - backendUS: Country US, coordinates in NY (40.7128, -74.0060), Address 30.0.0.1
	// If we query, since client is in FR, candidate set should narrow to backendFR and pick it.
	t.Run("Country preference narrow", func(t *testing.T) {
		g.Mutex.Lock()
		g.LocationMap = nil
		g.Mutex.Unlock()

		backendFR := &MockBackend{Backend: &Backend{
			Address: "10.0.0.1", Enable: true, Priority: 10, Country: "FR",
			Latitude: 48.8566, Longitude: 2.3522, HasCoordinates: true,
		}}
		backendDE := &MockBackend{Backend: &Backend{
			Address: "20.0.0.1", Enable: true, Priority: 20, Country: "DE",
			Latitude: 52.52, Longitude: 13.405, HasCoordinates: true,
		}}
		backendUS := &MockBackend{Backend: &Backend{
			Address: "30.0.0.1", Enable: true, Priority: 30, Country: "US",
			Latitude: 40.7128, Longitude: -74.006, HasCoordinates: true,
		}}
		backendFR.recomputeCoordinateRadians()
		backendDE.recomputeCoordinateRadians()
		backendUS.recomputeCoordinateRadians()

		backendFR.On("IsHealthy").Return(true)
		backendDE.On("IsHealthy").Return(true)
		backendUS.On("IsHealthy").Return(true)

		record := &Record{
			Fqdn:     "affinity.example.com.",
			Mode:     "geoip_affinity",
			Backends: []BackendInterface{backendFR, backendDE, backendUS},
		}

		ips, err := g.pickBackendWithGeoIPAffinity(record, dns.TypeA, net.ParseIP("81.185.159.80"))
		assert.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.1"}, ips)
	})

	// 3. Test case: Distance picking within the narrowed candidates.
	// Client IP: 8.8.8.8 (US).
	// Backends:
	// - backendUS_West: Country US, coordinates in Seattle (47.6062, -122.3321), Address 10.0.0.1
	// - backendUS_East: Country US, coordinates in New York (40.7128, -74.0060), Address 10.0.0.2
	// - backendFR: Country FR, coordinates in Paris (48.8566, 2.3522), Address 10.0.0.3
	// US client IP (8.8.8.8) is closer to US_East (NY) than US_West (Seattle).
	// Candidate pool should narrow to both US backends, and coordinate distance should select backendUS_East.
	t.Run("Distance pick within narrowed country subset", func(t *testing.T) {
		g.Mutex.Lock()
		g.LocationMap = nil
		g.Mutex.Unlock()

		backendUS_West := &MockBackend{Backend: &Backend{
			Address: "10.0.0.1", Enable: true, Priority: 10, Country: "US",
			Latitude: 47.6062, Longitude: -122.3321, HasCoordinates: true,
		}}
		backendUS_East := &MockBackend{Backend: &Backend{
			Address: "10.0.0.2", Enable: true, Priority: 20, Country: "US",
			Latitude: 40.7128, Longitude: -74.0060, HasCoordinates: true,
		}}
		backendFR := &MockBackend{Backend: &Backend{
			Address: "10.0.0.3", Enable: true, Priority: 30, Country: "FR",
			Latitude: 48.8566, Longitude: 2.3522, HasCoordinates: true,
		}}
		backendUS_West.recomputeCoordinateRadians()
		backendUS_East.recomputeCoordinateRadians()
		backendFR.recomputeCoordinateRadians()

		backendUS_West.On("IsHealthy").Return(true)
		backendUS_East.On("IsHealthy").Return(true)
		backendFR.On("IsHealthy").Return(true)

		record := &Record{
			Fqdn:     "affinity.example.com.",
			Mode:     "geoip_affinity",
			Backends: []BackendInterface{backendUS_West, backendUS_East, backendFR},
		}

		ips, err := g.pickBackendWithGeoIPAffinity(record, dns.TypeA, net.ParseIP("8.8.8.8"))
		assert.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.2"}, ips)
	})

	// 4. Test case: Fallback to global coordinates if all affinity candidates are unhealthy.
	// Client IP: 81.185.159.80 (FR).
	// Backends:
	// - backendFR: Country FR, Paris coordinates (48.8566, 2.3522), Address 10.0.0.1 (UNHEALTHY!)
	// - backendDE: Country DE, Berlin coordinates (52.5200, 13.4050), Address 20.0.0.1 (HEALTHY)
	// - backendUS: Country US, NY coordinates (40.7128, -74.0060), Address 30.0.0.1 (HEALTHY)
	// Since backendFR is unhealthy, we fall back to all healthy backends globally (closest first).
	// Berlin (DE) is closer to Paris than NY, so it should select backendDE!
	t.Run("Fallback to global coordinates", func(t *testing.T) {
		g.Mutex.Lock()
		g.LocationMap = nil
		g.Mutex.Unlock()

		backendFR := &MockBackend{Backend: &Backend{
			Address: "10.0.0.1", Enable: true, Priority: 10, Country: "FR",
			Latitude: 48.8566, Longitude: 2.3522, HasCoordinates: true,
		}}
		backendDE := &MockBackend{Backend: &Backend{
			Address: "20.0.0.1", Enable: true, Priority: 20, Country: "DE",
			Latitude: 52.52, Longitude: 13.405, HasCoordinates: true,
		}}
		backendUS := &MockBackend{Backend: &Backend{
			Address: "30.0.0.1", Enable: true, Priority: 30, Country: "US",
			Latitude: 40.7128, Longitude: -74.006, HasCoordinates: true,
		}}
		backendFR.recomputeCoordinateRadians()
		backendDE.recomputeCoordinateRadians()
		backendUS.recomputeCoordinateRadians()

		backendFR.On("IsHealthy").Return(false)
		backendDE.On("IsHealthy").Return(true)
		backendUS.On("IsHealthy").Return(true)

		record := &Record{
			Fqdn:     "affinity.example.com.",
			Mode:     "geoip_affinity",
			Backends: []BackendInterface{backendFR, backendDE, backendUS},
		}

		ips, err := g.pickBackendWithGeoIPAffinity(record, dns.TypeA, net.ParseIP("81.185.159.80"))
		assert.NoError(t, err)
		assert.Equal(t, []string{"20.0.0.1"}, ips)
	})
}

// TestResponseWriter is a mock dns.ResponseWriter for testing
// It captures the DNS message sent by WriteMsg
type TestResponseWriter struct {
	Msg *dns.Msg
}

func (w *TestResponseWriter) WriteMsg(m *dns.Msg) error {
	w.Msg = m
	return nil
}
func (w *TestResponseWriter) LocalAddr() net.Addr       { return nil }
func (w *TestResponseWriter) RemoteAddr() net.Addr      { return nil }
func (w *TestResponseWriter) Close() error              { return nil }
func (w *TestResponseWriter) TsigStatus() error         { return nil }
func (w *TestResponseWriter) TsigTimersOnly(bool)       {}
func (w *TestResponseWriter) Hijack()                   {}
func (w *TestResponseWriter) Write([]byte) (int, error) { return 0, nil }

func TestIsAddressTypeCompatible(t *testing.T) {
	testCases := []struct {
		name       string
		ip         string
		recordType uint16
		expected   bool
	}{
		{"IPv4 compatible with A", "192.168.1.1", dns.TypeA, true},
		{"IPv4 not compatible with AAAA", "192.168.1.1", dns.TypeAAAA, false},
		{"IPv6 compatible with AAAA", "2001:db8::1", dns.TypeAAAA, true},
		{"IPv6 not compatible with A", "2001:db8::1", dns.TypeA, false},
		{"Invalid IP with A", "invalid-ip", dns.TypeA, false},
		{"Invalid IP with AAAA", "invalid-ip", dns.TypeAAAA, false},
		{"IPv4 compatible with other type", "192.168.1.1", dns.TypeTXT, false},
		{"CNAME compatible with A", "some-alb.aws.com", dns.TypeA, true},
		{"CNAME compatible with AAAA", "some-alb.aws.com", dns.TypeAAAA, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isAddressTypeCompatible(tc.ip, tc.recordType)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGSLB_PickBackendWithFailoverFromSubset(t *testing.T) {
	g := &GSLB{}
	b1 := &MockBackend{Backend: &Backend{Address: "10.0.0.1", Enable: true, Priority: 10}}
	b2 := &MockBackend{Backend: &Backend{Address: "10.0.0.2", Enable: true, Priority: 20}}
	b3 := &MockBackend{Backend: &Backend{Address: "10.0.0.3", Enable: true, Priority: 5}}
	b1.On("IsHealthy").Return(true)
	b2.On("IsHealthy").Return(true)
	b3.On("IsHealthy").Return(false) // unhealthy, should be skipped

	backends := []BackendInterface{b1, b2, b3}

	// Should pick b1 (lowest priority among healthy)
	ips, err := g.pickBackendWithFailoverFromSubset(backends, dns.TypeA, "test.local.")
	assert.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1"}, ips)

	// No healthy backends
	b1_unhealthy := &MockBackend{Backend: &Backend{Address: "10.0.0.1", Enable: true, Priority: 10}}
	b1_unhealthy.On("IsHealthy").Return(false)
	_, err = g.pickBackendWithFailoverFromSubset([]BackendInterface{b1_unhealthy}, dns.TypeA, "test.local.")
	assert.Error(t, err)
}

func TestGSLB_GeoIPAffinityAndHierarchy(t *testing.T) {
	db, err := geoip2.Open("tests/GeoLite2-City.mmdb")
	if err != nil {
		t.Skip("GeoLite2-City.mmdb not found, skipping TestGSLB_GeoIPAffinityAndHierarchy")
	}
	defer db.Close()

	g := &GSLB{
		GeoIPCityDB: db,
	}

	// 1. Subnet Pinning
	g.LocationMap = map[string]string{
		"10.0.0.0/24": "dc-us",
	}
	backendSubnetMatched := &Backend{Address: "192.168.1.1", Enable: true, Location: "dc-us", Alive: true}
	backendSubnetOther := &Backend{Address: "192.168.1.2", Enable: true, Location: "dc-eu", Alive: true}

	record := &Record{
		Backends: []BackendInterface{backendSubnetMatched, backendSubnetOther},
	}

	res, err := g.pickBackendWithGeoIPAffinity(record, dns.TypeA, net.ParseIP("10.0.0.5"))
	assert.NoError(t, err)
	assert.Equal(t, []string{"192.168.1.1"}, res)

	// 2. City Level Match with mismatches to cover continues
	g.LocationMap = nil // disable subnet pinning

	// Client IP from Talence (81.185.159.80): city "Talence", country "FR", continent "EU", subdivision "NAQ"
	clientIP := net.ParseIP("81.185.159.80")

	// Create backends with different mismatch attributes
	bCityMatch := &Backend{Address: "192.168.10.1", Enable: true, City: "Talence", Country: "FR", Continent: "EU", Alive: true}
	bCityContMismatch := &Backend{Address: "192.168.10.2", Enable: true, City: "Talence", Continent: "NA", Alive: true}                                  // Continent mismatch
	bCityCountryMismatch := &Backend{Address: "192.168.10.3", Enable: true, City: "Talence", Country: "US", Alive: true}                                 // Country mismatch
	bCitySubMismatch := &Backend{Address: "192.168.10.4", Enable: true, City: "Talence", Country: "FR", Continent: "EU", Subdivision: "US", Alive: true} // Subdivision mismatch

	recordCity := &Record{
		Backends: []BackendInterface{bCityMatch, bCityContMismatch, bCityCountryMismatch, bCitySubMismatch},
	}

	res, err = g.pickBackendWithGeoIPAffinity(recordCity, dns.TypeA, clientIP)
	assert.NoError(t, err)
	assert.Equal(t, []string{"192.168.10.1"}, res)

	// 3. Subdivision Level Match
	bSubdivisionMatch := &Backend{Address: "192.168.20.1", Enable: true, Country: "FR", Subdivision: "NAQ", Continent: "EU", Alive: true}
	bSubdivisionContMismatch := &Backend{Address: "192.168.20.2", Enable: true, Country: "FR", Subdivision: "NAQ", Continent: "NA", Alive: true}

	recordSub := &Record{
		Backends: []BackendInterface{bSubdivisionMatch, bSubdivisionContMismatch},
	}
	res, err = g.pickBackendWithGeoIPAffinity(recordSub, dns.TypeA, clientIP)
	assert.NoError(t, err)
	assert.Equal(t, []string{"192.168.20.1"}, res)

	// 4. Country Level Match
	bCountryMatch := &Backend{Address: "192.168.30.1", Enable: true, Country: "FR", Continent: "EU", Alive: true}
	bCountryContMismatch := &Backend{Address: "192.168.30.2", Enable: true, Country: "FR", Continent: "NA", Alive: true}

	recordCountry := &Record{
		Backends: []BackendInterface{bCountryMatch, bCountryContMismatch},
	}
	res, err = g.pickBackendWithGeoIPAffinity(recordCountry, dns.TypeA, clientIP)
	assert.NoError(t, err)
	assert.Equal(t, []string{"192.168.30.1"}, res)

	// 5. Continent Level Match
	bContinentMatch := &Backend{Address: "192.168.40.1", Enable: true, Continent: "EU", Alive: true}

	recordContinent := &Record{
		Backends: []BackendInterface{bContinentMatch},
	}
	res, err = g.pickBackendWithGeoIPAffinity(recordContinent, dns.TypeA, clientIP)
	assert.NoError(t, err)
	assert.Equal(t, []string{"192.168.40.1"}, res)

	// 6. Coordinate Distance Match
	bCoord := &Backend{
		Address: "192.168.50.1", Enable: true, Alive: true,
		Latitude: 48.8566, Longitude: 2.3522, HasCoordinates: true,
	}
	bCoord.recomputeCoordinateRadians()

	recordCoord := &Record{
		Backends: []BackendInterface{bCoord},
	}
	res, err = g.pickBackendWithGeoIPAffinity(recordCoord, dns.TypeA, clientIP)
	assert.NoError(t, err)
	assert.Equal(t, []string{"192.168.50.1"}, res)

	// 7. Country DB Fallback in getClientGeoInfo
	g.GeoIPCityDB = nil
	g.GeoIPCountryDB = db

	info, err := g.getClientGeoInfo(clientIP)
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "FR", info.Country)
	assert.Equal(t, "EU", info.Continent)

	// 8. No DB or no match error
	g.GeoIPCountryDB = nil
	_, err = g.getClientGeoInfo(clientIP)
	assert.Error(t, err)
}

func TestGeoIPLookupUsesPreparsedCache(t *testing.T) {
	backendEU := &MockBackend{Backend: &Backend{Address: "10.0.0.42", Enable: true, Location: "eu-west"}}
	backendEU.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo.example.com.",
		Mode:     "geoip",
		Backends: []BackendInterface{backendEU},
	}

	g := &GSLB{
		LocationMap: map[string]string{
			"10.0.0.0/8": "eu-west",
		},
	}

	// Before lookup, LocationMapIPNet is nil/empty
	assert.Nil(t, g.LocationMapIPNet)

	// Perform GeoIP lookup
	ips, err := g.pickBackendWithGeoIP(record, dns.TypeA, net.ParseIP("10.0.0.50"))
	assert.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.42"}, ips)

	// Verify that the lazy rebuild was triggered and populated LocationMapIPNet
	assert.Len(t, g.LocationMapIPNet, 1)
	assert.Equal(t, "10.0.0.0/8", g.LocationMapIPNet[0].Subnet)
	assert.Equal(t, "eu-west", g.LocationMapIPNet[0].Location)
	assert.NotNil(t, g.LocationMapIPNet[0].IPNet)
}

func TestGeoIPAffinityLookupUsesPreparsedCache(t *testing.T) {
	backendUS := &MockBackend{Backend: &Backend{Address: "192.168.1.42", Enable: true, Location: "us-east"}}
	backendUS.On("IsHealthy").Return(true)

	record := &Record{
		Fqdn:     "geo.example.com.",
		Mode:     "geoip_affinity",
		Backends: []BackendInterface{backendUS},
	}

	g := &GSLB{
		LocationMap: map[string]string{
			"192.168.1.0/24": "us-east",
		},
	}

	// Before lookup, LocationMapIPNet is nil/empty
	assert.Nil(t, g.LocationMapIPNet)

	// Perform GeoIP Affinity lookup (where subnet pinning is evaluated)
	ips, err := g.pickBackendWithGeoIPAffinity(record, dns.TypeA, net.ParseIP("192.168.1.100"))
	assert.NoError(t, err)
	assert.Equal(t, []string{"192.168.1.42"}, ips)

	// Verify that the lazy rebuild was triggered and populated LocationMapIPNet
	assert.Len(t, g.LocationMapIPNet, 1)
	assert.Equal(t, "192.168.1.0/24", g.LocationMapIPNet[0].Subnet)
	assert.Equal(t, "us-east", g.LocationMapIPNet[0].Location)
	assert.NotNil(t, g.LocationMapIPNet[0].IPNet)
}
