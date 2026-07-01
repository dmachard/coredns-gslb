package gslb

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestLoadCustomLocationMap(t *testing.T) {
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
	err = g.loadCustomLocationsMap(tmpFile.Name())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if g.LocationMap["192.168.0.0/16"] != "eu-west-1" {
		t.Errorf("Expected eu-west-1, got %v", g.LocationMap["192.168.0.0/16"])
	}
	if g.LocationMap["10.0.0.0/8"] != "us-east-1" {
		t.Errorf("Expected us-east-1, got %v", g.LocationMap["10.0.0.0/8"])
	}

	// Verify LocationMapIPNet cache is populated
	if len(g.LocationMapIPNet) != 2 {
		t.Fatalf("Expected LocationMapIPNet length 2, got %d", len(g.LocationMapIPNet))
	}
	for _, entry := range g.LocationMapIPNet {
		if entry.Subnet == "192.168.0.0/16" {
			if entry.Location != "eu-west-1" {
				t.Errorf("Expected location eu-west-1 for 192.168.0.0/16, got %s", entry.Location)
			}
			if entry.IPNet == nil {
				t.Error("Expected IPNet not to be nil")
			}
		}
	}
}

func TestRebuildLocationMapIPNet(t *testing.T) {
	g := &GSLB{
		LocationMap: map[string]string{
			"81.185.159.0/24": "us-east",
			"invalid-subnet":  "nowhere",
		},
	}
	g.rebuildLocationMapIPNet()

	// "invalid-subnet" should be skipped because net.ParseCIDR fails on it
	if len(g.LocationMapIPNet) != 1 {
		t.Fatalf("Expected LocationMapIPNet length 1, got %d", len(g.LocationMapIPNet))
	}
	entry := g.LocationMapIPNet[0]
	if entry.Subnet != "81.185.159.0/24" {
		t.Errorf("Expected subnet 81.185.159.0/24, got %s", entry.Subnet)
	}
	if entry.Location != "us-east" {
		t.Errorf("Expected location us-east, got %s", entry.Location)
	}
	if entry.IPNet == nil {
		t.Error("Expected IPNet not to be nil")
	}
}

func TestLoadLocationMap_FileNotFound(t *testing.T) {
	g := &GSLB{}
	err := g.loadCustomLocationsMap("/nonexistent/location_map.yml")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestLoadLocationMap_EmptyPath(t *testing.T) {
	g := &GSLB{}
	err := g.loadCustomLocationsMap("")
	if err != nil {
		t.Errorf("Expected no error for empty path, got: %v", err)
	}
	if g.LocationMap != nil {
		t.Errorf("Expected LocationMap to be nil for empty path")
	}
	if g.LocationMapIPNet != nil {
		t.Errorf("Expected LocationMapIPNet to be nil for empty path")
	}
}

// Test UnmarshalYAML with healthcheck profiles
func TestGSLB_UnmarshalYAML_WithHealthcheckProfiles(t *testing.T) {
	yamlData := `
healthcheck_profiles:
  http_profile:
    type: http
    params:
      enable_tls: true
      port: 443
      uri: /health
      expected_code: 200
  tcp_profile:
    type: tcp
    params:
      port: 80
      timeout: 5s

records:
  test.example.com.:
    backends:
      - address: 192.168.1.1
        healthchecks: [ http_profile ]
        priority: 1
      - address: 192.168.1.2
        healthchecks: [ http_profile, tcp_profile ]
        priority: 2
    mode: failover
    record_ttl: 30
`
	// Unmarshal into a raw map
	var raw struct {
		HealthcheckProfiles map[string]*HealthCheck `yaml:"healthcheck_profiles"`
		Records             map[string]interface{}  `yaml:"records"`
	}
	err := yaml.Unmarshal([]byte(yamlData), &raw)
	assert.NoError(t, err)

	gslb := &GSLB{
		HealthcheckProfiles: raw.HealthcheckProfiles,
		Records:             make(map[string]map[string]*Record),
	}
	zone := ".example.com."
	gslb.Records[zone] = make(map[string]*Record)

	for fqdn, recordData := range raw.Records {
		processedRecordData, err := gslb.processRecordHealthchecks(recordData)
		assert.NoError(t, err)
		recordBytes, err := yaml.Marshal(processedRecordData)
		assert.NoError(t, err)
		var record Record
		assert.NoError(t, yaml.Unmarshal(recordBytes, &record))
		record.Fqdn = fqdn
		gslb.Records[zone][fqdn] = &record
	}

	// Verify healthcheck profiles were loaded
	assert.NotNil(t, gslb.HealthcheckProfiles)
	assert.Len(t, gslb.HealthcheckProfiles, 2)
	assert.Contains(t, gslb.HealthcheckProfiles, "http_profile")
	assert.Contains(t, gslb.HealthcheckProfiles, "tcp_profile")

	// Verify profiles have correct configuration
	httpProfile := gslb.HealthcheckProfiles["http_profile"]
	assert.Equal(t, "http", httpProfile.Type)
	assert.Equal(t, true, httpProfile.Params["enable_tls"])
	assert.Equal(t, 443, httpProfile.Params["port"])

	tcpProfile := gslb.HealthcheckProfiles["tcp_profile"]
	assert.Equal(t, "tcp", tcpProfile.Type)
	assert.Equal(t, 80, tcpProfile.Params["port"])

	// Verify records were processed correctly
	assert.NotNil(t, gslb.Records)
	assert.Len(t, gslb.Records, 1)
	record, ok := gslb.Records[zone]["test.example.com."]
	assert.True(t, ok, "Record test.example.com. should exist in zone %s", zone)
	assert.NotNil(t, record)
	assert.Equal(t, "failover", record.Mode)
	assert.Len(t, record.Backends, 2)

	// Verify backend 1 has 1 healthcheck (http_profile)
	backend1 := record.Backends[0]
	assert.Equal(t, "192.168.1.1", backend1.GetAddress())
	healthchecks1 := backend1.GetHealthChecks()
	assert.Len(t, healthchecks1, 1)
	assert.Equal(t, "https/443", healthchecks1[0].GetType())

	// Verify backend 2 has 2 healthchecks (http_profile + tcp_profile)
	backend2 := record.Backends[1]
	assert.Equal(t, "192.168.1.2", backend2.GetAddress())
	healthchecks2 := backend2.GetHealthChecks()
	assert.Len(t, healthchecks2, 2)
}

// Test processRecordHealthchecks method
func TestGSLB_processRecordHealthchecks(t *testing.T) {
	gslb := &GSLB{
		HealthcheckProfiles: map[string]*HealthCheck{
			"test_profile": {
				Type: "http",
				Params: map[string]interface{}{
					"port": 80,
					"uri":  "/status",
				},
			},
		},
	}

	// Test with valid record data containing profile references
	recordData := map[string]interface{}{
		"mode": "failover",
		"backends": []interface{}{
			map[string]interface{}{
				"address": "1.2.3.4",
				"healthchecks": []interface{}{
					"test_profile",
				},
			},
		},
	}

	processedData, err := gslb.processRecordHealthchecks(recordData)
	assert.NoError(t, err)

	processedRecord := processedData.(map[string]interface{})
	backends := processedRecord["backends"].([]interface{})
	backend := backends[0].(map[string]interface{})
	healthchecks := backend["healthchecks"].([]interface{})

	assert.Len(t, healthchecks, 1)
	hc := healthchecks[0].(map[string]interface{})
	assert.Equal(t, "http", hc["type"])
	assert.Equal(t, map[string]interface{}{"port": 80, "uri": "/status"}, hc["params"])
}

// Test processHealthchecks method
func TestGSLB_processHealthchecks(t *testing.T) {
	gslb := &GSLB{
		HealthcheckProfiles: map[string]*HealthCheck{
			"profile1": {
				Type: "http",
				Params: map[string]interface{}{
					"port": 443,
					"uri":  "/health",
				},
			},
			"profile2": {
				Type: "tcp",
				Params: map[string]interface{}{
					"port":    80,
					"timeout": "5s",
				},
			},
		},
	}

	t.Run("Profile references only", func(t *testing.T) {
		healthchecks := []interface{}{"profile1", "profile2"}

		result, err := gslb.processHealthchecks(healthchecks)
		assert.NoError(t, err)
		assert.Len(t, result, 2)

		// Check first healthcheck
		hc1 := result[0].(map[string]interface{})
		assert.Equal(t, "http", hc1["type"])
		assert.Equal(t, map[string]interface{}{"port": 443, "uri": "/health"}, hc1["params"])

		// Check second healthcheck
		hc2 := result[1].(map[string]interface{})
		assert.Equal(t, "tcp", hc2["type"])
		assert.Equal(t, map[string]interface{}{"port": 80, "timeout": "5s"}, hc2["params"])
	})

	t.Run("Mixed profile references and inline definitions", func(t *testing.T) {
		healthchecks := []interface{}{
			"profile1",
			map[string]interface{}{
				"type": ICMPType,
				"params": map[string]interface{}{
					"count":   3,
					"timeout": "2s",
				},
			},
		}

		result, err := gslb.processHealthchecks(healthchecks)
		assert.NoError(t, err)
		assert.Len(t, result, 2)

		// Check profile reference
		hc1 := result[0].(map[string]interface{})
		assert.Equal(t, "http", hc1["type"])

		// Check inline definition (should be unchanged)
		hc2 := result[1].(map[string]interface{})
		assert.Equal(t, ICMPType, hc2["type"])
		params := hc2["params"].(map[string]interface{})
		assert.Equal(t, 3, params["count"])
	})

	t.Run("Invalid profile reference", func(t *testing.T) {
		healthchecks := []interface{}{"non_existent_profile"}

		result, err := gslb.processHealthchecks(healthchecks)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "healthcheck profile 'non_existent_profile' not found")
	})

	t.Run("No profiles defined", func(t *testing.T) {
		gslbNoProfiles := &GSLB{HealthcheckProfiles: nil}
		healthchecks := []interface{}{"some_profile"}

		result, err := gslbNoProfiles.processHealthchecks(healthchecks)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Invalid healthchecks format", func(t *testing.T) {
		// healthchecks should be an array, not a string
		healthchecks := "invalid_format"

		result, err := gslb.processHealthchecks(healthchecks)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "healthchecks must be an array")
	})
}

// Test UnmarshalYAML error cases
func TestGSLB_UnmarshalYAML_ErrorCases(t *testing.T) {
	t.Run("Invalid YAML", func(t *testing.T) {
		yamlData := `
healthcheck_profiles:
  invalid: [
records:
  test: {}
`
		var gslb GSLB
		err := yaml.Unmarshal([]byte(yamlData), &gslb)
		assert.Error(t, err)
	})

	t.Run("Invalid profile reference in record", func(t *testing.T) {
		yamlData := `
healthcheck_profiles:
  valid_profile:
    type: http
    params:
      port: 80

records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
        healthchecks: [invalid_profile]
    mode: failover
`
		var gslb GSLB
		err := yaml.Unmarshal([]byte(yamlData), &gslb)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "healthcheck profile 'invalid_profile' not found")
	})
}

// Test UnmarshalYAML with no profiles (backward compatibility)
func TestGSLB_UnmarshalYAML_NoProfiles(t *testing.T) {
	yamlData := `
records:
  test.example.com.:
    backends:
      - address: 192.168.1.1
        healthchecks:
          - type: http
            params:
              enable_tls: false
              port: 80
              uri: /health
    mode: failover
`

	var gslb GSLB
	err := yaml.Unmarshal([]byte(yamlData), &gslb)
	assert.NoError(t, err)

	// Should work without profiles
	assert.Nil(t, gslb.HealthcheckProfiles)
	assert.NotNil(t, gslb.Records)
	assert.Len(t, gslb.Records, 1)

	_, ok := gslb.Records[""]
	assert.True(t, ok, "Zone key should be empty string in gslb.Records when no explicit zone is set")

	record, ok := gslb.Records[""]["test.example.com."]
	assert.True(t, ok, "Record test.example.com. should exist in zone '' (empty string)")
	assert.NotNil(t, record)
	assert.Len(t, record.Backends, 1)

	backend := record.Backends[0]
	healthchecks := backend.GetHealthChecks()
	assert.Len(t, healthchecks, 1)
	assert.Equal(t, "http/80", healthchecks[0].GetType())
}

func TestGSLB_RecordsMatchZone(t *testing.T) {
	testCases := []struct {
		name      string
		yamlData  string
		zone      string
		shouldErr bool
	}{
		{
			name: "All records match zone",
			yamlData: `
records:
  valid1.example.org.:
    backends:
      - address: 1.1.1.1
  valid2.example.org.:
    backends:
      - address: 2.2.2.2
`,
			zone:      ".example.org.",
			shouldErr: false,
		},
		{
			name: "One record does not match zone",
			yamlData: `
records:
  valid1.example.org.:
    backends:
      - address: 1.1.1.1
  invalid.example.com.:
    backends:
      - address: 3.3.3.3
`,
			zone:      ".example.org.",
			shouldErr: true,
		},
		{
			name: "All records do not match zone",
			yamlData: `
records:
  invalid1.example.com.:
    backends:
      - address: 4.4.4.4
  invalid2.example.net.:
    backends:
      - address: 5.5.5.5
`,
			zone:      ".example.org.",
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gslb := &GSLB{Zone: tc.zone}
			err := yaml.Unmarshal([]byte(tc.yamlData), gslb)
			if tc.shouldErr {
				assert.Error(t, err, "Expected error for zone mismatch")
			} else {
				assert.NoError(t, err, "Expected no error for matching records")
				for fqdn := range gslb.Records {
					assert.True(t, strings.HasSuffix(fqdn, tc.zone), "Record %s does not match zone %s", fqdn, tc.zone)
				}
			}
		})
	}
}

func TestGSLB_YAMLDefaultsAreApplied(t *testing.T) {
	yamlData := `
defaults:
  owner: admin
  record_ttl: 30
  scrape_interval: 10s
  scrape_retries: 1
  scrape_timeout: 5s
records:
  test1.example.com.:
    mode: failover
  test2.example.com.:
    mode: failover
    owner: bob  # Should override default
    record_ttl: 60 # Should override default
`
	zone := ".example.com."
	gslb := &GSLB{}
	err := loadConfigFile(gslb, writeTempYAML(t, yamlData), zone)
	assert.NoError(t, err)
	assert.NotNil(t, gslb.Records[zone]["test1.example.com."])
	record1 := gslb.Records[zone]["test1.example.com."]
	assert.Equal(t, "admin", record1.Owner, "test1 should inherit owner=admin from defaults")
	assert.Equal(t, 30, record1.RecordTTL, "test1 should inherit record_ttl=30 from defaults")
	assert.Equal(t, "10s", record1.ScrapeInterval, "test1 should inherit scrape_interval=10s from defaults")
	assert.Equal(t, 1, record1.ScrapeRetries, "test1 should inherit scrape_retries=1 from defaults")
	assert.Equal(t, "5s", record1.ScrapeTimeout, "test1 should inherit scrape_timeout=5s from defaults")
	assert.Equal(t, "failover", record1.Mode)

	record2 := gslb.Records[zone]["test2.example.com."]
	assert.Equal(t, "bob", record2.Owner, "test2 should override owner")
	assert.Equal(t, 60, record2.RecordTTL, "test2 should override record_ttl")
	assert.Equal(t, "10s", record2.ScrapeInterval, "test2 should inherit scrape_interval=10s from defaults")
	assert.Equal(t, 1, record2.ScrapeRetries, "test2 should inherit scrape_retries=1 from defaults")
	assert.Equal(t, "5s", record2.ScrapeTimeout, "test2 should inherit scrape_timeout=5s from defaults")
	assert.Equal(t, "failover", record2.Mode)
}

// Helper to write a temporary YAML file
func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "gslb-defaults-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	_, err = f.WriteString(content)
	if err != nil {
		f.Close()
		t.Fatalf("failed to write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestGSLB_LoadConfigFile_ErrorCases(t *testing.T) {
	gUpdate := &GSLB{
		Records: map[string]map[string]*Record{
			"existing.zone.": {},
		},
	}

	// loadConfigFile error: file empty
	tmpFileEmpty, err := os.CreateTemp("", "empty-*.yml")
	assert.NoError(t, err)
	defer os.Remove(tmpFileEmpty.Name())
	tmpFileEmpty.Close()

	err = loadConfigFile(gUpdate, tmpFileEmpty.Name(), "existing.zone.")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file empty")

	// loadConfigFile error: invalid YAML
	tmpFileInvalid, err := os.CreateTemp("", "invalid-*.yml")
	assert.NoError(t, err)
	defer os.Remove(tmpFileInvalid.Name())
	_, _ = tmpFileInvalid.Write([]byte("{invalid-yaml"))
	tmpFileInvalid.Close()

	err = loadConfigFile(gUpdate, tmpFileInvalid.Name(), "existing.zone.")
	assert.Error(t, err)

	// loadConfigFile error: record zone mismatch
	tmpFileMismatch, err := os.CreateTemp("", "mismatch-*.yml")
	assert.NoError(t, err)
	defer os.Remove(tmpFileMismatch.Name())
	mismatchContent := `
records:
  mismatch.other.com.:
    backends: []
`
	_, _ = tmpFileMismatch.Write([]byte(mismatchContent))
	tmpFileMismatch.Close()

	err = loadConfigFile(gUpdate, tmpFileMismatch.Name(), "existing.zone.")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match zone")
}

func TestGSLB_RiseFallConfigParsing(t *testing.T) {
	content := `
healthcheck_profiles:
  profile1:
    type: http
    rise: 4
    fall: 5
    params:
      port: 80
  profile2:
    type: http
    params:
      port: 80

records:
  test.example.com.:
    backends:
      - address: "1.1.1.1"
        healthchecks: [ profile1 ] # Should inherit rise=4, fall=5
      - address: "2.2.2.2"
        healthchecks: [ profile1 ]
        rise: 6                    # Should override rise=6, fall=5
        fall: 7
      - address: "3.3.3.3"
        healthchecks: [ profile2 ] # Should use default rise=2, fall=3
      - address: "4.4.4.4"
        healthchecks:
          - type: http
            rise: 8                # Inline healthcheck with rise/fall
            fall: 9
            params:
              port: 80
`
	tmpName := writeTempYAML(t, content)
	defer os.Remove(tmpName)

	g := &GSLB{}
	err := loadConfigFile(g, tmpName, "example.com.")
	assert.NoError(t, err)

	record, ok := g.Records["example.com."]["test.example.com."]
	assert.True(t, ok)
	assert.Len(t, record.Backends, 4)

	// Backend 1: address 1.1.1.1
	b1 := record.Backends[0]
	assert.Equal(t, "1.1.1.1", b1.GetAddress())
	assert.Equal(t, 4, b1.GetRise())
	assert.Equal(t, 5, b1.GetFall())

	// Backend 2: address 2.2.2.2
	b2 := record.Backends[1]
	assert.Equal(t, "2.2.2.2", b2.GetAddress())
	assert.Equal(t, 6, b2.GetRise())
	assert.Equal(t, 7, b2.GetFall())

	// Backend 3: address 3.3.3.3
	b3 := record.Backends[2]
	assert.Equal(t, "3.3.3.3", b3.GetAddress())
	assert.Equal(t, 2, b3.GetRise())
	assert.Equal(t, 3, b3.GetFall())

	// Backend 4: address 4.4.4.4
	b4 := record.Backends[3]
	assert.Equal(t, "4.4.4.4", b4.GetAddress())
	assert.Equal(t, 8, b4.GetRise())
	assert.Equal(t, 9, b4.GetFall())
}
