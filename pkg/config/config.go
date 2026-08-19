package config

import (
	"fmt"
	"net"
	"strings"

	"gopkg.in/yaml.v3"
)

// FallbackPolicyConfig represents the behavior of the record when all backends are unhealthy/disabled.
type FallbackPolicyConfig struct {
	Mode          string   `yaml:"mode"`
	FallbackIPs   []string `yaml:"fallback_ips"`
	FallbackCNAME string   `yaml:"fallback_cname"`
	Rcode         string   `yaml:"rcode"`
}

// DiscoveryConfig holds discovery configurations.
type DiscoveryConfig struct {
	Type     string `yaml:"type"`
	Endpoint string `yaml:"endpoint"`
	Service  string `yaml:"service"`
	Interval string `yaml:"interval"`
	Tag      string `yaml:"tag"`
}

// BackendConfig represents an individual backend with health check settings.
type BackendConfig struct {
	Description   string        `yaml:"description"`
	Address       string        `yaml:"address"`
	Port          int           `yaml:"port"`
	Priority      int           `yaml:"priority"`
	Weight        int           `yaml:"weight"`
	Enable        *bool         `yaml:"enable"` // Pointer to distinguish from default false
	Tags          []string      `yaml:"tags"`
	Timeout       string        `yaml:"timeout"`
	HealthChecks  []interface{} `yaml:"healthchecks"` // Inline healthchecks or profile names
	Continent     string        `yaml:"continent"`
	Country       string        `yaml:"country"`
	Subdivision   string        `yaml:"subdivision"`
	City          string        `yaml:"city"`
	ASN           string        `yaml:"asn"`
	Location      string        `yaml:"location"`
	Longitude     *float64      `yaml:"longitude"`
	Latitude      *float64      `yaml:"latitude"`
	AssumeHealthy bool          `yaml:"assume_healthy"`
	Rise          int           `yaml:"rise"`
	Fall          int           `yaml:"fall"`
}

// RecordConfig represents a GSLB record.
type RecordConfig struct {
	Mode           string               `yaml:"mode"`
	Backends       []BackendConfig      `yaml:"backends"`
	Owner          string               `yaml:"owner"`
	Description    string               `yaml:"description"`
	RecordTTL      int                  `yaml:"record_ttl"`
	ScrapeInterval string               `yaml:"scrape_interval"`
	ScrapeRetries  int                  `yaml:"scrape_retries"`
	ScrapeTimeout  string               `yaml:"scrape_timeout"`
	FallbackPolicy FallbackPolicyConfig `yaml:"fallback_policy"`
	Discovery      *DiscoveryConfig     `yaml:"discovery"`
	ALPN           []string             `yaml:"alpn"`
}

// HealthcheckProfile represents a reusable healthcheck profile.
type HealthcheckProfile struct {
	Type   string                 `yaml:"type"`
	Params map[string]interface{} `yaml:"params"`
	Rise   int                    `yaml:"rise"`
	Fall   int                    `yaml:"fall"`
}

// ZoneConfig represents the parsed YAML structure of a zone configuration file.
type ZoneConfig struct {
	Defaults            map[string]interface{}        `yaml:"defaults"`
	Records             map[string]RecordConfig       `yaml:"records"`
	HealthcheckProfiles map[string]HealthcheckProfile `yaml:"healthcheck_profiles"`
}

// LocationMapConfig represents custom subnets configuration.
type LocationMapConfig struct {
	Subnets []struct {
		Subnet   string `yaml:"subnet"`
		Location string `yaml:"location"`
	} `yaml:"subnets"`
}

// LoadZoneConfig parses YAML data and resolves default options.
func LoadZoneConfig(data []byte) (*ZoneConfig, error) {
	var raw struct {
		Defaults            map[string]interface{}        `yaml:"defaults"`
		Records             map[string]interface{}        `yaml:"records"`
		HealthcheckProfiles map[string]HealthcheckProfile `yaml:"healthcheck_profiles"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML configuration: %w", err)
	}

	cfg := &ZoneConfig{
		Defaults:            raw.Defaults,
		HealthcheckProfiles: raw.HealthcheckProfiles,
		Records:             make(map[string]RecordConfig),
	}

	for fqdn, recordData := range raw.Records {
		var merged map[string]interface{}
		if raw.Defaults != nil {
			recordMap, ok := recordData.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("record %s is not a map", fqdn)
			}
			merged = make(map[string]interface{})
			for k, v := range raw.Defaults {
				merged[k] = v
			}
			for k, v := range recordMap {
				merged[k] = v
			}
		} else {
			var ok bool
			merged, ok = recordData.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("record %s is not a map", fqdn)
			}
		}

		recordBytes, err := yaml.Marshal(merged)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal record %s: %w", fqdn, err)
		}

		var recConfig RecordConfig
		// Set default values before unmarshaling
		recConfig.Mode = "failover"
		recConfig.RecordTTL = 30
		recConfig.ScrapeInterval = "10s"
		recConfig.ScrapeRetries = 1
		recConfig.ScrapeTimeout = "5s"

		if err := yaml.Unmarshal(recordBytes, &recConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal record %s: %w", fqdn, err)
		}

		// Apply backend default values
		for i := range recConfig.Backends {
			b := &recConfig.Backends[i]
			if b.Address == "" {
				b.Address = "127.0.0.1"
			}
			if b.Weight <= 0 {
				b.Weight = 1
			}
			if b.Timeout == "" {
				b.Timeout = "5s"
			}
			if b.Rise <= 0 {
				b.Rise = 2
			}
			if b.Fall <= 0 {
				b.Fall = 3
			}
			if b.Enable == nil {
				tru := true
				b.Enable = &tru
			}
		}

		cfg.Records[fqdn] = recConfig
	}

	return cfg, nil
}

// ParseAndValidateLocationMap parses the custom location map YAML and validates CIDRs.
func ParseAndValidateLocationMap(data []byte) (map[string]string, []string) {
	var parsed LocationMapConfig
	var errors []string
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		errors = append(errors, fmt.Sprintf("failed to parse location map: %v", err))
		return nil, errors
	}
	m := make(map[string]string)
	for _, s := range parsed.Subnets {
		if s.Subnet == "" {
			errors = append(errors, "empty subnet in location map")
			continue
		}
		_, _, err := net.ParseCIDR(s.Subnet)
		if err != nil {
			errors = append(errors, fmt.Sprintf("invalid CIDR notation '%s' in location map", s.Subnet))
		} else {
			m[s.Subnet] = s.Location
		}
	}
	return m, errors
}

// Validate performs semantic validation on the ZoneConfig.
func (z *ZoneConfig) Validate(validLocations map[string]bool, globalProfiles map[string]HealthcheckProfile) ([]string, []string) {
	var errs []string
	var warns []string

	for fqdn, record := range z.Records {
		// 1. Duplicate backend addresses within the same record
		seenAddresses := make(map[string]bool)
		for _, backend := range record.Backends {
			if backend.Address == "" {
				continue
			}
			if seenAddresses[backend.Address] {
				errs = append(errs, fmt.Sprintf("duplicate backend address '%s' in record '%s'", backend.Address, fqdn))
			}
			seenAddresses[backend.Address] = true
		}

		// 2. Backends referencing location values not pinned in the custom location map.
		// The location map only pins *specific* client subnets to a location, so it is
		// expected to be partial: a backend location that is not a pin target stays
		// usable through GeoIP affinity, failover and the API bulk operations. This is
		// therefore reported as a warning (typo hint), never as a fatal error.
		if len(validLocations) > 0 {
			reported := make(map[string]bool)
			for _, backend := range record.Backends {
				if backend.Location == "" || validLocations[backend.Location] || reported[backend.Location] {
					continue
				}
				reported[backend.Location] = true
				warns = append(warns, fmt.Sprintf("record '%s' references location '%s' which is not pinned to any subnet in the custom location map (subnet pinning will never select these backends)", fqdn, backend.Location))
			}
		}

		// 3. Duplicate priority values for backends in failover mode (ambiguous failover order)
		if strings.ToLower(record.Mode) == "failover" {
			seenPriorities := make(map[int]bool)
			for _, backend := range record.Backends {
				if seenPriorities[backend.Priority] {
					warns = append(warns, fmt.Sprintf("duplicate priority value '%d' for backends in failover mode for record '%s' (ambiguous failover order)", backend.Priority, fqdn))
				}
				seenPriorities[backend.Priority] = true
			}
		}

		// 4. Check backends healthchecks
		for _, backend := range record.Backends {
			// Warning: Backends configured without any healthchecks (implicitly always healthy)
			if len(backend.HealthChecks) == 0 && !backend.AssumeHealthy {
				warns = append(warns, fmt.Sprintf("backend '%s' in record '%s' configured without any healthchecks (implicitly always healthy)", backend.Address, fqdn))
			}

			// Validate healthchecks (inline or profile)
			for _, hc := range backend.HealthChecks {
				var hcType string
				var hcParams map[string]interface{}

				switch v := hc.(type) {
				case string:
					// Profile reference
					profileName := v
					profile, exists := z.HealthcheckProfiles[profileName]
					if !exists && globalProfiles != nil {
						profile, exists = globalProfiles[profileName]
					}
					if !exists {
						errs = append(errs, fmt.Sprintf("unresolved healthcheck profile '%s' in record '%s' backend '%s'", profileName, fqdn, backend.Address))
						continue
					}
					hcType = profile.Type
					hcParams = profile.Params
				case map[string]interface{}:
					// Inline healthcheck
					if t, ok := v["type"].(string); ok {
						hcType = t
					}
					if p, ok := v["params"].(map[string]interface{}); ok {
						hcParams = p
					}
				default:
					errs = append(errs, fmt.Sprintf("invalid healthcheck definition in record '%s' backend '%s'", fqdn, backend.Address))
					continue
				}

				// Warning: HTTP healthchecks configured on port 443 with enable_tls: false
				if strings.ToLower(hcType) == "http" && hcParams != nil {
					var port int
					var enableTLS bool

					// extract port
					if pVal, ok := hcParams["port"]; ok {
						switch p := pVal.(type) {
						case int:
							port = p
						case int64:
							port = int(p)
						case float64:
							port = int(p)
						case string:
							fmt.Sscanf(p, "%d", &port)
						}
					}

					// extract enable_tls
					if tVal, ok := hcParams["enable_tls"]; ok {
						if b, ok := tVal.(bool); ok {
							enableTLS = b
						}
					}

					if port == 443 && !enableTLS {
						warns = append(warns, fmt.Sprintf("HTTP healthcheck on port 443 configured with enable_tls: false in record '%s' backend '%s'", fqdn, backend.Address))
					}
				}
			}
		}
	}

	return errs, warns
}
