package gslb

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coredns/caddy"
	"github.com/stretchr/testify/assert"
)

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
				zone app-x.gslb.example.com ./tests/db.app-x.gslb.example.com.yml
			}`,
			expectError: false,
		},

		// Test with valid configuration and additional options
		{
			name: "Valid config with additional options",
			config: `gslb {
				zone app-x.gslb.example.com ./tests/db.app-x.gslb.example.com.yml
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
				zone app-x.gslb.example.com ./tests/db.app-x.gslb.example.com.yml
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
				zone app-x.gslb.example.com ./tests/db.app-x.gslb.example.com.yml
				zone app-y.gslb.example.com ./tests/db.app-y.gslb.example.com.yml
			}`,
			expectError: false,
		},

		// Test with all main parameters set
		{
			name: "Valid config with all main parameters",
			config: `gslb {
				zone app-x.gslb.example.com ./tests/db.app-x.gslb.example.com.yml
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
				zone app-x.gslb.example.com ./tests/db.app-x.gslb.example.com.yml
				disable_txt
			}`,
			expectError: false,
		},
		{
			name: "use_edns_csubnet valid",
			config: `gslb {
				zone app-x.gslb.example.com ./tests/db.app-x.gslb.example.com.yml
				use_edns_csubnet
			}`,
			expectError: false,
		},
		{
			name: "Valid config with Redis options",
			config: `gslb {
				zone app-x.gslb.example.com ./tests/db.app-x.gslb.example.com.yml
				redis_enable true
				redis_addr "1.2.3.4:6379"
				redis_password "securepass"
				redis_db 2
				redis_key_prefix "gslb_prefix:"
				redis_sync_mode none
			}`,
			expectError: false,
		},
		{
			name: "Invalid redis_sync_mode value error",
			config: `gslb {
				zone app-x.gslb.example.com ./tests/db.app-x.gslb.example.com.yml
				redis_sync_mode invalid
			}`,
			expectError: true,
		},
		{
			name: "Invalid redis_db value error",
			config: `gslb {
				zone app-x.gslb.example.com ./tests/db.app-x.gslb.example.com.yml
				redis_db -1
			}`,
			expectError: true,
		},
		{
			name: "Unknown option error",
			config: `gslb {
				unknown_option
			}`,
			expectError: true,
		},
		{
			name: "Missing zone arg error",
			config: `gslb {
				zone
			}`,
			expectError: true,
		},
		{
			name: "Missing zone file error",
			config: `gslb {
				zone app.example.com
			}`,
			expectError: true,
		},
		{
			name: "Missing max_stagger_start arg error",
			config: `gslb {
				max_stagger_start
			}`,
			expectError: true,
		},
		{
			name: "Invalid max_stagger_start value error",
			config: `gslb {
				max_stagger_start invalid
			}`,
			expectError: true,
		},
		{
			name: "Missing batch_size_start arg error",
			config: `gslb {
				batch_size_start
			}`,
			expectError: true,
		},
		{
			name: "Invalid batch_size_start value error",
			config: `gslb {
				batch_size_start -10
			}`,
			expectError: true,
		},
		{
			name: "Missing resolution_idle_timeout arg error",
			config: `gslb {
				resolution_idle_timeout
			}`,
			expectError: true,
		},
		{
			name: "Invalid resolution_idle_timeout value error",
			config: `gslb {
				resolution_idle_timeout invalid
			}`,
			expectError: true,
		},
		{
			name: "Missing geoip_custom arg error",
			config: `gslb {
				geoip_custom
			}`,
			expectError: true,
		},
		{
			name: "Missing geoip_maxmind arg error",
			config: `gslb {
				geoip_maxmind
			}`,
			expectError: true,
		},
		{
			name: "Missing geoip_maxmind path error",
			config: `gslb {
				geoip_maxmind country_db
			}`,
			expectError: true,
		},
		{
			name: "Unknown geoip_maxmind type error",
			config: `gslb {
				geoip_maxmind unknown_db ./path
			}`,
			expectError: true,
		},
		{
			name: "Failed to open country MaxMind DB error",
			config: `gslb {
				geoip_maxmind country_db ./invalid.mmdb
			}`,
			expectError: true,
		},
		{
			name: "Missing healthcheck_idle_multiplier arg error",
			config: `gslb {
				healthcheck_idle_multiplier
			}`,
			expectError: true,
		},
		{
			name: "Invalid healthcheck_idle_multiplier value error",
			config: `gslb {
				healthcheck_idle_multiplier 0
			}`,
			expectError: true,
		},
		{
			name: "Missing api_enable arg error",
			config: `gslb {
				api_enable
			}`,
			expectError: true,
		},
		{
			name: "Missing api_tls_cert arg error",
			config: `gslb {
				api_tls_cert
			}`,
			expectError: true,
		},
		{
			name: "Missing api_tls_key arg error",
			config: `gslb {
				api_tls_key
			}`,
			expectError: true,
		},
		{
			name: "Missing api_listen_addr arg error",
			config: `gslb {
				api_listen_addr
			}`,
			expectError: true,
		},
		{
			name: "Missing api_listen_port arg error",
			config: `gslb {
				api_listen_port
			}`,
			expectError: true,
		},
		{
			name: "Missing api_basic_user arg error",
			config: `gslb {
				api_basic_user
			}`,
			expectError: true,
		},
		{
			name: "Missing api_basic_pass arg error",
			config: `gslb {
				api_basic_pass
			}`,
			expectError: true,
		},
		{
			name: "Missing healthcheck_profiles arg error",
			config: `gslb {
				healthcheck_profiles
			}`,
			expectError: true,
		},
		{
			name: "Nonexistent healthcheck_profiles file error",
			config: `gslb {
				healthcheck_profiles /path/to/nonexistent
			}`,
			expectError: true,
		},
		{
			name: "disable_txt with extra arg error",
			config: `gslb {
				disable_txt extra
			}`,
			expectError: true,
		},
		{
			name: "use_edns_csubnet with extra arg error",
			config: `gslb {
				use_edns_csubnet extra
			}`,
			expectError: true,
		},
		{
			name: "No zone directive error",
			config: `gslb {
				api_enable false
			}`,
			expectError: true,
		},
	}

	// Iterate over test cases
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create a new Caddy controller for each test case
			c := caddy.NewTestController("dns", test.config)
			err := setup(c)

			if test.expectError {
				assert.Error(t, err, "Expected error but got nil for test: %v", test.name)
			} else {
				assert.NoError(t, err, "Expected no error, but got: %v for test: %v", err, test.name)
			}
		})
	}
}
func TestSetupGSLB_ValidationErrors(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("duplicate backend addresses error", func(t *testing.T) {
		invalidConfig := `records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
      - address: 1.2.3.4
`
		filePath := filepath.Join(tmpDir, "invalid_zone.yml")
		err := os.WriteFile(filePath, []byte(invalidConfig), 0644)
		assert.NoError(t, err)

		configStr := `gslb {
			zone example.com ` + filePath + `
		}`
		c := caddy.NewTestController("dns", configStr)
		err = setup(c)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate backend address")
	})

	t.Run("invalid custom location map CIDR error", func(t *testing.T) {
		validZone := `records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
`
		filePathZone := filepath.Join(tmpDir, "valid_zone.yml")
		err := os.WriteFile(filePathZone, []byte(validZone), 0644)
		assert.NoError(t, err)

		invalidLocMap := `subnets:
  - subnet: 192.168.0.0/99
    location: eu-west-1
`
		filePathLoc := filepath.Join(tmpDir, "invalid_loc.yml")
		err = os.WriteFile(filePathLoc, []byte(invalidLocMap), 0644)
		assert.NoError(t, err)

		configStr := `gslb {
			zone example.com ` + filePathZone + `
			geoip_custom ` + filePathLoc + `
		}`
		c := caddy.NewTestController("dns", configStr)
		err = setup(c)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid custom location map")
	})

	t.Run("backend referencing undefined location error", func(t *testing.T) {
		invalidConfig := `records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
        location: us-west-2
`
		filePathZone := filepath.Join(tmpDir, "invalid_zone_loc.yml")
		err := os.WriteFile(filePathZone, []byte(invalidConfig), 0644)
		assert.NoError(t, err)

		locMap := `subnets:
  - subnet: 192.168.0.0/16
    location: eu-west-1
`
		filePathLoc := filepath.Join(tmpDir, "loc.yml")
		err = os.WriteFile(filePathLoc, []byte(locMap), 0644)
		assert.NoError(t, err)

		configStr := `gslb {
			zone example.com ` + filePathZone + `
			geoip_custom ` + filePathLoc + `
		}`
		c := caddy.NewTestController("dns", configStr)
		err = setup(c)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "references location 'us-west-2' not defined in custom location map")
	})

	t.Run("unresolved healthcheck profile error", func(t *testing.T) {
		invalidConfig := `records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
        healthchecks:
          - nonexistent_profile
`
		filePathZone := filepath.Join(tmpDir, "invalid_zone_profile.yml")
		err := os.WriteFile(filePathZone, []byte(invalidConfig), 0644)
		assert.NoError(t, err)

		configStr := `gslb {
			zone example.com ` + filePathZone + `
		}`
		c := caddy.NewTestController("dns", configStr)
		err = setup(c)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unresolved healthcheck profile 'nonexistent_profile'")
	})
}

func TestLoadRealConfig(t *testing.T) {
	// Test loading the appX config file with healthcheck profiles
	g := &GSLB{}
	zone := "app-x.gslb.example.com."
	err := loadConfigFile(g, "./tests/db.app-x.gslb.example.com.yml", zone)
	assert.NoError(t, err)

	// Verify healthcheck profiles were loaded
	assert.NotNil(t, g.HealthcheckProfiles)
	assert.Len(t, g.HealthcheckProfiles, 5) // https_default, mtls_default, icmp_default, grpc_default, lua_default

	expectedProfiles := []string{"https_default", "mtls_default", "icmp_default", "grpc_default", "lua_default"}
	for _, profileName := range expectedProfiles {
		assert.Contains(t, g.HealthcheckProfiles, profileName, "Should contain profile %s", profileName)
	}

	// Verify records were loaded and processed
	assert.NotNil(t, g.Records)
	assert.Len(t, g.Records, 1)
	for _, recs := range g.Records {
		assert.Len(t, recs, 7)
	}

	zone = "webapp.app-x.gslb.example.com."
	if g.Records[zone] == nil {
		// fallback: try to find the only zone key
		for z := range g.Records {
			zone = z
			break
		}
	}
	record, ok := g.Records[zone]["webapp.app-x.gslb.example.com."]
	assert.True(t, ok, "Record webapp.app-x.gslb.example.com. should exist in zone %s", zone)
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
	_, ok = g.Records[zone]["webapp-lua.app-x.gslb.example.com."]
	assert.True(t, ok, "Record webapp-lua.app-x.gslb.example.com. should exist in zone %s", zone)
	_, ok = g.Records[zone]["webapp-grpc.app-x.gslb.example.com."]
	assert.True(t, ok, "Record webapp-grpc.app-x.gslb.example.com. should exist in zone %s", zone)
}

func TestConfigWatcherDetectsChanges(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "gslb_watcher_test_")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a minimal test config file with YAML structure the loadConfigFile expects
	testConfigPath := filepath.Join(tmpDir, "test_config.yml")
	initialConfig := `defaults:
  owner: admin
  record_ttl: 30

records:
  app.test.example.com.:
    mode: failover
    backends:
      - address: 192.168.1.1
        priority: 1
      - address: 192.168.1.2
        priority: 2
`
	err = os.WriteFile(testConfigPath, []byte(initialConfig), 0644)
	assert.NoError(t, err)

	// Create GSLB instance and load initial config
	g := &GSLB{
		Zones:   make(map[string]string),
		Records: make(map[string]map[string]*Record),
	}
	zone := "test.example.com."
	g.Zones[zone] = testConfigPath

	err = loadConfigFile(g, testConfigPath, zone)
	assert.NoError(t, err)

	// Verify initial state - first backend should have priority 1
	recordKey := "app.test.example.com."
	assert.Contains(t, g.Records[zone], recordKey, "Record should exist initially")
	initialRecord := g.Records[zone][recordKey]
	assert.Len(t, initialRecord.Backends, 2, "Should have 2 backends")
	firstBackendInitial := initialRecord.Backends[0]
	assert.Equal(t, "192.168.1.1", firstBackendInitial.GetAddress(), "First backend address")
	assert.Equal(t, 1, firstBackendInitial.GetPriority(), "First backend initial priority should be 1")

	// Start the watcher in a goroutine
	go func() {
		// This will run forever, but we don't care - just let it run
		_ = startConfigWatcher(g, testConfigPath)
	}()

	// Give the watcher time to start and add the file to watch
	time.Sleep(300 * time.Millisecond)

	// Modify the config file - SWAP the priorities
	modifiedConfig := `defaults:
  owner: admin
  record_ttl: 30

records:
  app.test.example.com.:
    mode: failover
    backends:
      - address: 192.168.1.1
        priority: 2
      - address: 192.168.1.2
        priority: 1
`
	err = os.WriteFile(testConfigPath, []byte(modifiedConfig), 0644)
	assert.NoError(t, err)

	// Wait for the debounce timer (500ms) + processing time
	time.Sleep(1000 * time.Millisecond)

	// Verify config was reloaded AND priorities changed
	assert.NotNil(t, g.Records[zone], "Records should exist after reload")
	assert.Contains(t, g.Records[zone], recordKey, "Expected record should exist after reload")

	reloadedRecord := g.Records[zone][recordKey]
	assert.Len(t, reloadedRecord.Backends, 2, "Should still have 2 backends after reload")

	// Check that priorities were actually updated
	firstBackendAfterReload := reloadedRecord.Backends[0]
	assert.Equal(t, "192.168.1.1", firstBackendAfterReload.GetAddress(), "First backend address should stay the same")
	assert.Equal(t, 2, firstBackendAfterReload.GetPriority(), "First backend priority should be CHANGED to 2")

	secondBackendAfterReload := reloadedRecord.Backends[1]
	assert.Equal(t, "192.168.1.2", secondBackendAfterReload.GetAddress(), "Second backend address")
	assert.Equal(t, 1, secondBackendAfterReload.GetPriority(), "Second backend priority should be CHANGED to 1")
}

func TestCustomLocationMapWatcherDetectsChanges(t *testing.T) {
	// Create a temporary directory and file
	tmpDir := t.TempDir()
	locationMapFile := filepath.Join(tmpDir, "location_map.yml")

	// Write initial location map with correct YAML structure
	// The format must match what loadCustomLocationsMap expects: subnets array
	initialMap := `subnets:
  - subnet: 192.168.1.0/24
    location: datacenter1
  - subnet: 10.0.0.0/8
    location: datacenter2`

	err := os.WriteFile(locationMapFile, []byte(initialMap), 0644)
	assert.NoError(t, err)

	// Create GSLB instance with proper initialization
	g := &GSLB{
		LocationMap: make(map[string]string),
	}

	// Load initial location map
	err = g.loadCustomLocationsMap(locationMapFile)
	assert.NoError(t, err, "Should load initial location map without error")
	assert.Len(t, g.LocationMap, 2, "Should have 2 locations initially")
	assert.Equal(t, "datacenter1", g.LocationMap["192.168.1.0/24"])
	assert.Equal(t, "datacenter2", g.LocationMap["10.0.0.0/8"])

	t.Logf("Initial LocationMap loaded with %d entries", len(g.LocationMap))

	// Start watcher in goroutine
	go watchCustomLocationMap(g, locationMapFile)

	// Give watcher time to start
	time.Sleep(200 * time.Millisecond)

	// Modify location map using vi-style rename (create temp, then rename)
	updatedMap := `subnets:
  - subnet: 192.168.1.0/24
    location: datacenter1
  - subnet: 10.0.0.0/8
    location: datacenter2
  - subnet: 172.16.0.0/12
    location: datacenter3`

	tmpFile := locationMapFile + ".tmp"
	err = os.WriteFile(tmpFile, []byte(updatedMap), 0644)
	assert.NoError(t, err)

	err = os.Rename(tmpFile, locationMapFile)
	assert.NoError(t, err)

	t.Logf("File modified via rename, waiting for watcher to detect...")

	// Wait for watcher to detect change and reload (500ms debounce + processing)
	time.Sleep(1200 * time.Millisecond)

	// Verify the location map was reloaded
	g.Mutex.RLock()
	newLen := len(g.LocationMap)
	hasDatacenter3 := g.LocationMap["172.16.0.0/12"]
	g.Mutex.RUnlock()

	t.Logf("After reload: LocationMap has %d entries", newLen)

	// The new map should have 3 entries including the new datacenter3
	assert.Len(t, g.LocationMap, 3, "Should have 3 locations after reload")
	assert.Equal(t, "datacenter3", hasDatacenter3, "Should have datacenter3 entry")
	assert.Equal(t, "datacenter1", g.LocationMap["192.168.1.0/24"], "Should still have datacenter1")
	assert.Equal(t, "datacenter2", g.LocationMap["10.0.0.0/8"], "Should still have datacenter2")
}

func TestHealthcheckProfilesWatcherDetectsChanges(t *testing.T) {
	// Save original global profiles and restore at the end
	ProfilesMutex.Lock()
	originalProfiles := GlobalHealthcheckProfiles
	ProfilesMutex.Unlock()
	defer func() {
		ProfilesMutex.Lock()
		GlobalHealthcheckProfiles = originalProfiles
		ProfilesMutex.Unlock()
	}()

	// Reset global profiles for this test
	ProfilesMutex.Lock()
	GlobalHealthcheckProfiles = make(map[string]*HealthCheck)
	ProfilesMutex.Unlock()

	// Create a temporary directory and file
	tmpDir := t.TempDir()
	profilesFile := filepath.Join(tmpDir, "healthcheck_profiles.yml")

	// Write initial healthcheck profiles
	initialProfiles := `healthcheck_profiles:
  https_default:
    type: http
    params:
      port: 443
      tls: true
  icmp_default:
    type: icmp
    params:
      count: 3`

	err := os.WriteFile(profilesFile, []byte(initialProfiles), 0644)
	assert.NoError(t, err)

	// Create GSLB instance (needed for watcher signature)
	g := &GSLB{
		Zones:   make(map[string]string),
		Records: make(map[string]map[string]*Record),
	}

	// Load initial profiles
	err = reloadHealthcheckProfiles(profilesFile)
	assert.NoError(t, err, "Should load initial profiles without error")
	assert.NotNil(t, GlobalHealthcheckProfiles, "GlobalHealthcheckProfiles should not be nil")
	assert.Len(t, GlobalHealthcheckProfiles, 2, "Should have 2 profiles initially")

	// Verify initial profiles exist and have correct types
	httpsProfile, hasHttps := GlobalHealthcheckProfiles["https_default"]
	icmpProfile, hasIcmp := GlobalHealthcheckProfiles["icmp_default"]
	assert.True(t, hasHttps, "Should have https_default profile")
	assert.True(t, hasIcmp, "Should have icmp_default profile")
	assert.Equal(t, "http", httpsProfile.Type)
	assert.Equal(t, "icmp", icmpProfile.Type)

	t.Logf("Initial healthcheck profiles loaded: %d profiles", len(GlobalHealthcheckProfiles))

	// Start watcher in goroutine
	go watchHealthcheckProfiles(g, profilesFile)

	// Give watcher time to start
	time.Sleep(200 * time.Millisecond)

	// Modify profiles using vi-style rename (create temp, then rename)
	updatedProfiles := `healthcheck_profiles:
  https_default:
    type: http
    params:
      port: 443
      tls: true
  icmp_default:
    type: icmp
    params:
      count: 3
  grpc_default:
    type: grpc
    params:
      port: 50051`

	tmpFile := profilesFile + ".tmp"
	err = os.WriteFile(tmpFile, []byte(updatedProfiles), 0644)
	assert.NoError(t, err)

	err = os.Rename(tmpFile, profilesFile)
	assert.NoError(t, err)

	t.Logf("Profiles file modified via rename, waiting for watcher to detect...")

	// Wait for watcher to detect change and reload (500ms debounce + processing)
	time.Sleep(1200 * time.Millisecond)

	// Verify the profiles were reloaded
	ProfilesMutex.RLock()
	assert.NotNil(t, GlobalHealthcheckProfiles, "GlobalHealthcheckProfiles should not be nil after reload")
	assert.Len(t, GlobalHealthcheckProfiles, 3, "Should have 3 profiles after reload")

	// Verify all three profiles exist
	_, hasHttpsAfter := GlobalHealthcheckProfiles["https_default"]
	_, hasIcmpAfter := GlobalHealthcheckProfiles["icmp_default"]
	grpcProfile, hasGrpc := GlobalHealthcheckProfiles["grpc_default"]
	ProfilesMutex.RUnlock()

	assert.True(t, hasHttpsAfter, "Should still have https_default")
	assert.True(t, hasIcmpAfter, "Should still have icmp_default")
	assert.True(t, hasGrpc, "Should have new grpc_default")

	// Verify the new profile properties
	if hasGrpc && grpcProfile != nil {
		ProfilesMutex.RLock()
		assert.NotNil(t, grpcProfile, "grpc_default profile should not be nil")
		assert.Equal(t, "grpc", grpcProfile.Type, "grpc_default should have type 'grpc'")
		assert.NotNil(t, grpcProfile.Params, "grpc_default should have params")
		ProfilesMutex.RUnlock()
	}

	ProfilesMutex.RLock()
	numProfiles := len(GlobalHealthcheckProfiles)
	ProfilesMutex.RUnlock()
	t.Logf("Test passed - healthcheck profiles reloaded successfully with %d profiles", numProfiles)
}

func TestReloadHealthcheckProfilesAndZones_SuccessAndFail(t *testing.T) {
	// Setup temporary profiles file
	tmpDir := t.TempDir()
	profilesFile := filepath.Join(tmpDir, "healthcheck_profiles.yml")
	initialProfiles := `healthcheck_profiles:
  https_default:
    type: http
    params:
      port: 443`
	err := os.WriteFile(profilesFile, []byte(initialProfiles), 0644)
	assert.NoError(t, err)

	// Create GSLB with a mock zone and config file
	g := &GSLB{
		Zones:   make(map[string]string),
		Records: make(map[string]map[string]*Record),
	}

	// Case 1: Zones mapping points to a valid file
	validZoneFile := filepath.Join(tmpDir, "valid_zone.yml")
	initialConfig := `records:
  app.test.local.:
    mode: failover`
	err = os.WriteFile(validZoneFile, []byte(initialConfig), 0644)
	assert.NoError(t, err)

	zoneName := "test.local."
	g.Zones[zoneName] = validZoneFile

	err = reloadHealthcheckProfilesAndZones(g, profilesFile)
	assert.NoError(t, err)

	// Case 2: Zones mapping points to an invalid/non-existent file
	g.Zones["invalid.local."] = "/path/to/nonexistent/file.yml"
	err = reloadHealthcheckProfilesAndZones(g, profilesFile)
	assert.Error(t, err) // Should return error because reloading invalid zone fails

	// Case 3: Invalid profiles file path
	err = reloadHealthcheckProfilesAndZones(g, "/path/to/nonexistent/profiles.yml")
	assert.Error(t, err)
}

func TestFindZoneByFile_NotFound(t *testing.T) {
	g := &GSLB{
		Zones: map[string]string{
			"test.local.": "/path/to/valid.yml",
		},
	}
	// Found
	assert.Equal(t, "test.local.", findZoneByFile(g, "/path/to/valid.yml"))
	// Not found
	assert.Equal(t, "", findZoneByFile(g, "/path/to/invalid.yml"))
}
