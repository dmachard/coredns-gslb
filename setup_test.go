package gslb

import (
	"testing"

	"github.com/coredns/caddy"
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
					example.org ./tests/appX_records.yml
				}
			}`,
			expectError: false,
		},

		// Test with valid configuration and additional options
		{
			name: "Valid config with additional options",
			config: `gslb {
				zones {
					example.org ./tests/appX_records.yml
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
					example.org ./tests/appX_records.yml
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
					example.org ./tests/appX_records.yml
					example.net ./tests/appY_records.yml
				}
			}`,
			expectError: false,
		},

		// Test with all main parameters set
		{
			name: "Valid config with all main parameters",
			config: `gslb {
				zones {
					example.org ./tests/appX_records.yml
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
		{"Valid config", "./tests/appX_records.yml", false},
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
