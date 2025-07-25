package gslb

import (
	"testing"

	"github.com/coredns/caddy"
	"github.com/stretchr/testify/assert"
)

// Test setup function for the GSLB plugin.
func TestSetupGSLB(t *testing.T) {
	// Define test cases
	tests := []struct {
		name        string
		config      string
		expectError bool
	}{
		// Test with basic valid configuration (explicit zone-to-file mapping)
		{
			name: "Valid config with explicit zone-to-file mapping",
			config: `gslb {
				zone example.org ./tests/db.app-x.gslb.example.com.yml
			}`,
			expectError: false,
		},

		// Test with valid configuration and additional options
		{
			name: "Valid config with additional options",
			config: `gslb {
				zone example.org ./tests/db.app-x.gslb.example.com.yml
				max_stagger_start 120s
				batch_size_start 50
				resolution_idle_timeout 1800s
			}`,
			expectError: false,
		},

		// Test with geoip_maxmind block (valid syntax, no files)
		{
			name: "Valid geoip_maxmind block syntax",
			config: `gslb {
				zone example.org ./tests/db.app-x.gslb.example.com.yml
				geoip_maxmind country_db ./tests/GeoLite2-Country.mmdb
				geoip_maxmind city_db ./tests/GeoLite2-City.mmdb
				geoip_maxmind asn_db ./tests/GeoLite2-ASN.mmdb
			}`,
			expectError: false,
		},

		// Test with multiple zones and files
		{
			name: "Valid config with multiple zones and files",
			config: `gslb {
				zone example.org ./tests/db.app-x.gslb.example.com.yml
				zone example.net ./tests/db.app-y.gslb.example.com.yml
			}`,
			expectError: false,
		},

		// Test with all main parameters set
		{
			name: "Valid config with all main parameters",
			config: `gslb {
				zone example.org ./tests/db.app-x.gslb.example.com.yml
				max_stagger_start 90s
				batch_size_start 42
				resolution_idle_timeout 1234s
				geoip_maxmind country_db ./tests/GeoLite2-Country.mmdb
				geoip_maxmind city_db ./tests/GeoLite2-City.mmdb
				geoip_maxmind asn_db ./tests/GeoLite2-ASN.mmdb
				geoip_custom ./tests/location_map.yml
				api_enable false
				api_tls_cert /tmp/cert.pem
				api_tls_key /tmp/key.pem
				api_listen_addr 127.0.0.1
				api_listen_port 9999
				api_basic_user testuser
				api_basic_pass testpass
				healthcheck_idle_multiplier 7
			}`,
			expectError: false,
		},
		// Test with disable_txt option
		{
			name: "Disable TXT option disables TXT queries",
			config: `gslb {
				zone example.org ./tests/db.app-x.gslb.example.com.yml
				disable_txt
			}`,
			expectError: false,
		},
	}

	// Iterate over test cases
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create a new Caddy controller for each test case
			c := caddy.NewTestController("dns", test.config)
			err := setup(c)

			// Only expect no error for all cases
			if err != nil {
				t.Fatalf("Expected no error, but got: %v for test: %v", err, test.name)
			}
		})
	}
}
func TestLoadRealConfig(t *testing.T) {
	// Test loading the appX config file with healthcheck profiles
	g := &GSLB{}
	err := loadConfigFile(g, "./tests/db.app-x.gslb.example.com.yml")
	assert.NoError(t, err)

	// Verify healthcheck profiles were loaded
	assert.NotNil(t, g.HealthcheckProfiles)
	assert.Len(t, g.HealthcheckProfiles, 4) // https_default, icmp_default, grpc_default, lua_default

	expectedProfiles := []string{"https_default", "icmp_default", "grpc_default", "lua_default"}
	for _, profileName := range expectedProfiles {
		assert.Contains(t, g.HealthcheckProfiles, profileName, "Should contain profile %s", profileName)
	}

	// Verify records were loaded and processed
	assert.NotNil(t, g.Records)
	assert.Len(t, g.Records, 3)

	record, ok := g.Records["webapp.app-x.gslb.example.com."]
	assert.True(t, ok, "Record webapp.app-x.gslb.example.com. should exist")
	assert.NotNil(t, record)
	assert.Equal(t, "failover", record.Mode)
	assert.Len(t, record.Backends, 2)

	// Check first backend - should have 1 healthcheck (https_default)
	backend1 := record.Backends[0]
	assert.Equal(t, "172.16.0.10", backend1.GetAddress())
	healthchecks1 := backend1.GetHealthChecks()
	assert.Len(t, healthchecks1, 1)
	assert.Equal(t, "https/443", healthchecks1[0].GetType())

	// Check second backend - should have 2 healthchecks (https_default + icmp_default)
	backend2 := record.Backends[1]
	assert.Equal(t, "172.16.0.11", backend2.GetAddress())
	healthchecks2 := backend2.GetHealthChecks()
	assert.Len(t, healthchecks2, 2)

	// Should have HTTPS and ICMP
	found_https := false
	found_icmp := false
	for _, hc := range healthchecks2 {
		if hc.GetType() == "https/443" {
			found_https = true
		}
		if hc.GetType() == ICMPType {
			found_icmp = true
		}
	}
	assert.True(t, found_https, "Should have HTTPS healthcheck")
	assert.True(t, found_icmp, "Should have ICMP healthcheck")

	// Vérifier la présence des autres records
	_, ok = g.Records["webapp-lua.app-x.gslb.example.com."]
	assert.True(t, ok, "Record webapp-lua.app-x.gslb.example.com. should exist")
	_, ok = g.Records["webapp-grpc.app-x.gslb.example.com."]
	assert.True(t, ok, "Record webapp-grpc.app-x.gslb.example.com. should exist")
}
