package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAndValidateLocationMap(t *testing.T) {
	tests := []struct {
		name       string
		yamlData   string
		expectErrs int
		expectMap  map[string]string
	}{
		{
			name: "valid location map",
			yamlData: `subnets:
  - subnet: 192.168.0.0/16
    location: eu-west-1
  - subnet: 10.0.0.0/8
    location: us-east-1`,
			expectErrs: 0,
			expectMap: map[string]string{
				"192.168.0.0/16": "eu-west-1",
				"10.0.0.0/8":     "us-east-1",
			},
		},
		{
			name: "invalid CIDR notation",
			yamlData: `subnets:
  - subnet: 192.168.0.0/99
    location: eu-west-1
  - subnet: invalid-cidr
    location: us-east-1`,
			expectErrs: 2,
			expectMap:  map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, errs := ParseAndValidateLocationMap([]byte(tt.yamlData))
			assert.Len(t, errs, tt.expectErrs)
			if tt.expectErrs == 0 {
				assert.Equal(t, tt.expectMap, m)
			}
		})
	}
}

func TestZoneConfig_Validate(t *testing.T) {
	t.Run("duplicate backend addresses", func(t *testing.T) {
		yamlData := `records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
      - address: 1.2.3.4
`
		cfg, err := LoadZoneConfig([]byte(yamlData))
		assert.NoError(t, err)

		errs, warns := cfg.Validate(nil, nil)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0], "duplicate backend address '1.2.3.4'")
		assert.Len(t, warns, 3) // 2 for missing healthchecks, 1 for duplicate priority 0
	})

	t.Run("backend referencing unpinned location warns but does not fail", func(t *testing.T) {
		yamlData := `records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
        location: us-west-2
      - address: 1.2.3.5
        location: us-west-2
      - address: 1.2.3.6
        location: eu-west-1
`
		cfg, err := LoadZoneConfig([]byte(yamlData))
		assert.NoError(t, err)

		validLocations := map[string]bool{"eu-west-1": true}
		errs, warns := cfg.Validate(validLocations, nil)
		assert.Empty(t, errs)

		var locWarns []string
		for _, w := range warns {
			if strings.Contains(w, "not pinned to any subnet") {
				locWarns = append(locWarns, w)
			}
		}
		// One warning per distinct unpinned location, not per backend.
		assert.Len(t, locWarns, 1)
		assert.Contains(t, locWarns[0], "record 'test.example.com.' references location 'us-west-2'")
	})

	t.Run("duplicate priority values in failover mode", func(t *testing.T) {
		yamlData := `records:
  test.example.com.:
    mode: failover
    backends:
      - address: 1.2.3.4
        priority: 10
      - address: 1.2.3.5
        priority: 10
`
		cfg, err := LoadZoneConfig([]byte(yamlData))
		assert.NoError(t, err)

		_, warns := cfg.Validate(nil, nil)
		assert.Len(t, warns, 3) // 1 duplicate priority warn + 2 no healthchecks warns
		foundPriorityWarn := false
		for _, w := range warns {
			if assert.ObjectsAreEqualValues(w, "duplicate priority value '10' for backends in failover mode for record 'test.example.com.' (ambiguous failover order)") {
				foundPriorityWarn = true
			}
		}
		assert.True(t, foundPriorityWarn)
	})

	t.Run("backends without healthchecks", func(t *testing.T) {
		yamlData := `records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
`
		cfg, err := LoadZoneConfig([]byte(yamlData))
		assert.NoError(t, err)

		_, warns := cfg.Validate(nil, nil)
		assert.Len(t, warns, 1)
		assert.Contains(t, warns[0], "configured without any healthchecks (implicitly always healthy)")
	})

	t.Run("HTTP healthcheck on port 443 with enable_tls false", func(t *testing.T) {
		yamlData := `records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
        healthchecks:
          - type: http
            params:
              port: 443
              enable_tls: false
`
		cfg, err := LoadZoneConfig([]byte(yamlData))
		assert.NoError(t, err)

		_, warns := cfg.Validate(nil, nil)
		assert.Len(t, warns, 1)
		assert.Contains(t, warns[0], "HTTP healthcheck on port 443 configured with enable_tls: false")
	})

	t.Run("unresolved healthcheck profiles", func(t *testing.T) {
		yamlData := `records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
        healthchecks:
          - missing_profile
`
		cfg, err := LoadZoneConfig([]byte(yamlData))
		assert.NoError(t, err)

		errs, _ := cfg.Validate(nil, nil)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0], "unresolved healthcheck profile 'missing_profile'")
	})

	t.Run("resolved local profile", func(t *testing.T) {
		yamlData := `healthcheck_profiles:
  local_profile:
    type: http
    params:
      port: 80
records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
        healthchecks:
          - local_profile
`
		cfg, err := LoadZoneConfig([]byte(yamlData))
		assert.NoError(t, err)

		errs, warns := cfg.Validate(nil, nil)
		assert.Len(t, errs, 0)
		assert.Len(t, warns, 0)
	})
}

func TestLoadZoneConfig_GlobalHealthcheck(t *testing.T) {
	t.Run("valid global healthcheck rise and fall", func(t *testing.T) {
		yamlData := `
healthcheck:
  rise: 1
  fall: 2
records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
`
		cfg, err := LoadZoneConfig([]byte(yamlData))
		assert.NoError(t, err)
		assert.NotNil(t, cfg.Healthcheck.Rise)
		assert.NotNil(t, cfg.Healthcheck.Fall)
		assert.Equal(t, 1, *cfg.Healthcheck.Rise)
		assert.Equal(t, 2, *cfg.Healthcheck.Fall)
	})

	t.Run("invalid global healthcheck rise", func(t *testing.T) {
		yamlData := `
healthcheck:
  rise: -1
`
		_, err := LoadZoneConfig([]byte(yamlData))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid global healthcheck rise")
	})

	t.Run("invalid global healthcheck fall", func(t *testing.T) {
		yamlData := `
healthcheck:
  fall: -5
`
		_, err := LoadZoneConfig([]byte(yamlData))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid global healthcheck fall")
	})
}
