package gslb

import (
	"fmt"
	"net"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// UnmarshalYAML implements custom YAML unmarshaling to handle healthcheck_profiles
func (g *GSLB) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw struct {
		Records             map[string]interface{}  `yaml:"records"`
		HealthcheckProfiles map[string]*HealthCheck `yaml:"healthcheck_profiles"`
	}

	if err := unmarshal(&raw); err != nil {
		return err
	}

	// Store healthcheck profiles
	if raw.HealthcheckProfiles != nil {
		g.HealthcheckProfiles = raw.HealthcheckProfiles
	}

	// Process records with healthcheck profile resolution
	if raw.Records != nil {
		if g.Records == nil {
			g.Records = make(map[string]map[string]*Record)
		}
		zone := g.Zone // zone attendue, ex: ".example.org."
		if g.Records[zone] == nil {
			g.Records[zone] = make(map[string]*Record)
		}
		for fqdn, recordData := range raw.Records {
			if zone != "" && !strings.HasSuffix(fqdn, zone) {
				return fmt.Errorf("record %s does not match zone %s", fqdn, zone)
			}
			// Pre-process the record data to resolve healthcheck profiles
			processedRecordData, err := g.processRecordHealthchecks(recordData)
			if err != nil {
				return fmt.Errorf("error processing record %s: %w", fqdn, err)
			}

			// Marshal and unmarshal the processed data to create the Record
			recordBytes, err := yaml.Marshal(processedRecordData)
			if err != nil {
				return fmt.Errorf("failed to marshal processed record %s: %w", fqdn, err)
			}

			var record Record
			if err := yaml.Unmarshal(recordBytes, &record); err != nil {
				return fmt.Errorf("failed to unmarshal record %s: %w", fqdn, err)
			}

			record.Fqdn = fqdn
			g.Records[zone][fqdn] = &record
		}
	}

	return nil
}

// processRecordHealthchecks processes a record to resolve healthcheck profile references
func (g *GSLB) processRecordHealthchecks(recordData interface{}) (interface{}, error) {
	recordMap, ok := recordData.(map[string]interface{})
	if !ok {
		return recordData, nil
	}

	backends, exists := recordMap["backends"]
	if !exists {
		return recordData, nil
	}

	backendsList, ok := backends.([]interface{})
	if !ok {
		return recordData, nil
	}

	// Process each backend
	for i, backend := range backendsList {
		backendMap, ok := backend.(map[string]interface{})
		if !ok {
			continue
		}

		healthchecks, exists := backendMap["healthchecks"]
		if !exists {
			continue
		}

		processedHealthchecks, err := g.processHealthchecks(healthchecks)
		if err != nil {
			return nil, err
		}

		backendMap["healthchecks"] = processedHealthchecks
		backendsList[i] = backendMap
	}

	recordMap["backends"] = backendsList
	return recordMap, nil
}

// processHealthchecks processes healthchecks to resolve profile references
func (g *GSLB) processHealthchecks(healthchecks interface{}) ([]interface{}, error) {
	var result []interface{}

	switch hc := healthchecks.(type) {
	case []interface{}:
		for _, item := range hc {
			switch v := item.(type) {
			case string:
				// It's a profile reference
				profile, err := ResolveHealthcheckProfile(v, g.HealthcheckProfiles)
				if err != nil {
					return nil, err
				}
				result = append(result, map[string]interface{}{
					"type":   profile.Type,
					"params": profile.Params,
				})
			default:
				// It's a full healthcheck object
				result = append(result, item)
			}
		}
	default:
		return nil, fmt.Errorf("healthchecks must be an array")
	}

	return result, nil
}

func (g *GSLB) rebuildLocationMapIPNet() {
	g.LocationMapIPNet = make([]CustomSubnet, 0, len(g.LocationMap))
	for subnet, location := range g.LocationMap {
		_, ipnet, err := net.ParseCIDR(subnet)
		if err == nil {
			g.LocationMapIPNet = append(g.LocationMapIPNet, CustomSubnet{
				Subnet:   subnet,
				IPNet:    ipnet,
				Location: location,
			})
		}
	}
}

func (g *GSLB) loadCustomLocationsMap(path string) error {
	g.Mutex.Lock()
	defer g.Mutex.Unlock()
	if path == "" {
		g.LocationMap = nil
		g.LocationMapIPNet = nil
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read location map: %w", err)
	}
	var parsed struct {
		Subnets []struct {
			Subnet   string `yaml:"subnet"`
			Location string `yaml:"location"`
		} `yaml:"subnets"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("failed to parse location map: %w", err)
	}
	m := make(map[string]string)
	for _, s := range parsed.Subnets {
		m[s.Subnet] = s.Location
	}
	g.LocationMap = m
	g.rebuildLocationMapIPNet()
	return nil
}

func loadConfigFile(gslb *GSLB, fileName string, zone string) error {
	if !strings.HasSuffix(zone, ".") {
		zone += "."
	}
	data, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("failed to read YAML configuration: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("failed to read YAML configuration: file empty")
	}
	var raw struct {
		Defaults            map[string]interface{}  `yaml:"defaults"`
		Records             map[string]interface{}  `yaml:"records"`
		HealthcheckProfiles map[string]*HealthCheck `yaml:"healthcheck_profiles"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to parse YAML configuration: %w", err)
	}
	gslb.HealthcheckProfiles = raw.HealthcheckProfiles
	if gslb.Records == nil {
		gslb.Records = make(map[string]map[string]*Record)
	}
	if gslb.Records[zone] == nil {
		gslb.Records[zone] = make(map[string]*Record)
	}

	for fqdn, recordData := range raw.Records {
		if zone != "" && !strings.HasSuffix(fqdn, zone) {
			return fmt.Errorf("record %s does not match zone %s", fqdn, zone)
		}
		var merged map[string]interface{}

		// handle defaults
		if raw.Defaults != nil {
			recordMap, ok := recordData.(map[string]interface{})
			if !ok {
				return fmt.Errorf("record %s is not a map", fqdn)
			}
			merged = make(map[string]interface{})
			// copy defaults
			for k, v := range raw.Defaults {
				merged[k] = v
			}
			// copy record data
			for k, v := range recordMap {
				merged[k] = v
			}
		} else {
			var ok bool
			merged, ok = recordData.(map[string]interface{})
			if !ok {
				return fmt.Errorf("record %s is not a map", fqdn)
			}
		}
		processedRecordData, err := (&GSLB{HealthcheckProfiles: raw.HealthcheckProfiles}).processRecordHealthchecks(merged)
		if err != nil {
			return fmt.Errorf("error processing record %s: %w", fqdn, err)
		}
		recordBytes, err := yaml.Marshal(processedRecordData)
		if err != nil {
			return fmt.Errorf("failed to marshal processed record %s: %w", fqdn, err)
		}
		var record Record
		if err := yaml.Unmarshal(recordBytes, &record); err != nil {
			return fmt.Errorf("failed to unmarshal record %s: %w", fqdn, err)
		}
		record.Fqdn = fqdn
		gslb.Records[zone][fqdn] = &record
	}
	return nil
}

