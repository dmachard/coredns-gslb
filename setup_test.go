package gslb

import (
	"os"
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
		// Test with basic valid configuration
		{"Valid config", `gslb ./coredns/gslb_config.example.com.yml`, false},

		// Test with valid configuration and additional options
		{"Valid config with additional options", `gslb ./coredns/gslb_config.example.com.yml {
			max_stagger_start 120s
			batch_size_start 50
			resolution_idle_timeout 1800s
		}`, false},

		// Test with valid configuration and a single zone
		{"Valid config with single zone", `gslb ./coredns/gslb_config.example.com.yml example.org {
			max_stagger_start 120s
			batch_size_start 50
		}`, false},

		// Test with valid configuration and multiple zones
		{"Valid config with multiple zones", `gslb ./coredns/gslb_config.example.com.yml example.org example.com {
			resolution_idle_timeout 1800s
		}`, false},

		// Test with invalid `max_stagger_start` (non-duration value)
		{"Invalid max_stagger_start", `gslb ./coredns/gslb_config.example.com.yml {
			max_stagger_start invalid
		}`, true},

		// Test with invalid `batch_size_start` (non-integer value)
		{"Invalid batch_size_start", `gslb ./coredns/gslb_config.example.com.yml{
			batch_size_start invalid
		}`, true},

		// Test with an invalid configuration file path (non-existent file)
		{"Non-existent config file", `gslb ./non_existent_config.yml`, true},

		// Test with unknown block option
		{"Unknown block option", `gslb ./coredns/gslb_config.example.com.yml {
			unknown_option
		}`, true},
	}

	// Iterate over test cases
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create a new Caddy controller for each test case
			c := caddy.NewTestController("dns", test.config)
			err := setup(c)

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

// Test loadConfigFile function for handling invalid configurations
func TestLoadConfigFile(t *testing.T) {
	// Define test cases
	tests := []struct {
		name        string
		filePath    string
		expectError bool
	}{
		{"Valid config", "./coredns/gslb_config.example.com.yml", false},
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

func TestLoadLocationMap(t *testing.T) {
	// Create a temporary YAML file for the location map
	tmpFile, err := os.CreateTemp("", "location_map_test_*.yml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `subnets:
  - subnet: "192.168.0.0/16"
    location: "eu-west-1"
  - subnet: "10.0.0.0/8"
    location: "us-east-1"
`
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	g := &GSLB{}
	err = g.loadLocationMap(tmpFile.Name())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if g.LocationMap["192.168.0.0/16"] != "eu-west-1" {
		t.Errorf("Expected eu-west-1, got %v", g.LocationMap["192.168.0.0/16"])
	}
	if g.LocationMap["10.0.0.0/8"] != "us-east-1" {
		t.Errorf("Expected us-east-1, got %v", g.LocationMap["10.0.0.0/8"])
	}
}

func TestLoadLocationMap_FileNotFound(t *testing.T) {
	g := &GSLB{}
	err := g.loadLocationMap("/nonexistent/location_map.yml")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestLoadLocationMap_EmptyPath(t *testing.T) {
	g := &GSLB{}
	err := g.loadLocationMap("")
	if err != nil {
		t.Errorf("Expected no error for empty path, got: %v", err)
	}
	if g.LocationMap != nil {
		t.Errorf("Expected LocationMap to be nil for empty path")
	}
}
