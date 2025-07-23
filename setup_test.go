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
				zones {
					example.org ./tests/db.app-x.gslb.example.com.yml
				}
			}`,
			expectError: false,
		},

		// Test with valid configuration and additional options
		{
			name: "Valid config with additional options",
			config: `gslb {
				zones {
					example.org ./tests/db.app-x.gslb.example.com.yml
				}
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
				zones {
					example.org ./tests/db.app-x.gslb.example.com.yml
				}
				geoip_maxmind {
				}
			}`,
			expectError: false,
		},

		// Test with multiple zones and files
		{
			name: "Valid config with multiple zones and files",
			config: `gslb {
				zones {
					example.org ./tests/db.app-x.gslb.example.com.yml
					example.net ./tests/db.app-y.gslb.example.com.yml
				}
			}`,
			expectError: false,
		},

		// Test with all main parameters set
		{
			name: "Valid config with all main parameters",
			config: `gslb {
				zones {
					example.org ./tests/db.app-x.gslb.example.com.yml
				}
				max_stagger_start 90s
				batch_size_start 42
				resolution_idle_timeout 1234s
				geoip_maxmind {
					country_db /tmp/country.mmdb
					city_db /tmp/city.mmdb
					asn_db /tmp/asn.mmdb
				}
				geoip_custom /tmp/location_map.yml
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
				zones {
					example.org ./tests/db.app-x.gslb.example.com.yml
				}
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

// Test loadConfigFile function for handling invalid configurations
func TestLoadConfigFile(t *testing.T) {
	// Define test cases
	tests := []struct {
		name        string
		filePath    string
		expectError bool
	}{
		{"Valid config", "./tests/db.app-x.gslb.example.com.yml", false},
		{"Non-existent file", "./tests/non_existent_config.yml", true},
	}

	// Iterate over test cases
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create a new GSLB instance for each test case
			g := &GSLB{}
			err := loadConfigFile(g, test.filePath)

			// Check if we expect an error or not
			if test.expectError && err == nil {
				t.Fatalf("Expected error, but got none for test: %v", test.name)
			}
			if !test.expectError && err != nil {
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

	record := g.Records["webapp.app-y.gslb.example.com."]
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
}
