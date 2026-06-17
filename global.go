package gslb

import "sync"

// Global map for global healthcheck profiles loaded from Corefile
var (
	GlobalHealthcheckProfiles map[string]*HealthCheck
	ProfilesMutex             sync.RWMutex
)
