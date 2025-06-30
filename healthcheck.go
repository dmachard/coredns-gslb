package gslb

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// GenericHealthCheck defines a common interface for health checks
type GenericHealthCheck interface {
	// PerformCheck executes the health check for a backend.
	PerformCheck(backend *Backend, fdqn string, maxRetries int) bool

	// GetType returns the type of the health check (e.g., "http/80").
	GetType() string

	// Equals compares the current health check with another instance for equality.
	Equals(other GenericHealthCheck) bool
}

// healthChecksEqual compares two slices of GenericHealthCheck for equality.
func healthChecksEqual(h1, h2 []GenericHealthCheck) bool {
	if len(h1) != len(h2) {
		return false
	}

	for i := range h1 {
		if !h1[i].Equals(h2[i]) {
			return false
		}
	}

	return true
}

type HealthCheck struct {
	Type   string                 `yaml:"type"`
	Params map[string]interface{} `yaml:"params"`
}

func (hc *HealthCheck) ToSpecificHealthCheck() (GenericHealthCheck, error) {
	switch hc.Type {
	case "http":
		var httpCheck HTTPHealthCheck
		httpCheck.SetDefault()

		paramsYaml, err := yaml.Marshal(hc.Params) // Serialize `hc.Params` to YAML
		if err != nil {
			return nil, fmt.Errorf("failed to serialize healthcheck params: %v", err)
		}
		err = yaml.Unmarshal(paramsYaml, &httpCheck) // Deserialize into `HTTPHealthCheck`
		if err != nil {
			return nil, fmt.Errorf("failed to decode HTTP params: %v", err)
		}
		return &httpCheck, nil

	case "icmp":
		var icmpCheck ICMPHealthCheck
		icmpCheck.SetDefault()

		paramsYaml, err := yaml.Marshal(hc.Params) // Serialize `hc.Params` to YAML
		if err != nil {
			return nil, fmt.Errorf("failed to serialize healthcheck params: %v", err)
		}
		err = yaml.Unmarshal(paramsYaml, &icmpCheck) // Deserialize into `ICMPHealthCheck`
		if err != nil {
			return nil, fmt.Errorf("failed to decode ICMP params: %v", err)
		}
		return &icmpCheck, nil

	case "tcp":
		var tcpCheck TCPHealthCheck
		tcpCheck.SetDefault()

		paramsYaml, err := yaml.Marshal(hc.Params) // Serialize `hc.Params` to YAML
		if err != nil {
			return nil, fmt.Errorf("failed to serialize healthcheck params: %v", err)
		}
		err = yaml.Unmarshal(paramsYaml, &tcpCheck) // Deserialize into `ICMPHealthCheck`
		if err != nil {
			return nil, fmt.Errorf("failed to decode TCP params: %v", err)
		}
		return &tcpCheck, nil

	case "custom":
		var customCheck CustomHealthCheck
		customCheck.SetDefault()
		paramsYaml, err := yaml.Marshal(hc.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize healthcheck params: %v", err)
		}
		err = yaml.Unmarshal(paramsYaml, &customCheck)
		if err != nil {
			return nil, fmt.Errorf("failed to decode custom params: %v", err)
		}
		return &customCheck, nil

	default:
		return nil, fmt.Errorf("unsupported healthcheck type: %s", hc.Type)
	}
}

// Mock a health check that always returns true (successful)
// For testing purpose
type MockHealthCheck struct{}

func (hc *MockHealthCheck) PerformCheck(backend *Backend, fqdn string, maxRetries int) bool {
	return true
}
func (hc *MockHealthCheck) GetType() string {
	return "mock"
}
func (hc *MockHealthCheck) Equals(other GenericHealthCheck) bool {
	_, ok := other.(*MockHealthCheck)
	return ok
}
