package gslb

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BackendState struct {
	Status         string    `json:"status"` // "healthy" or "unhealthy"
	LastCheck      time.Time `json:"last_check"`
	LastResolution time.Time `json:"last_resolution"`
}

// SaveState serializes the current health check state and last resolution times to disk.
func (g *GSLB) SaveState() error {
	if !g.StatePersistEnable || g.RedisEnable {
		return nil
	}

	state := make(map[string]BackendState)

	g.Mutex.RLock()
	for _, records := range g.Records {
		for fqdn, record := range records {
			record.mutex.RLock()
			// Get last resolution time for this FQDN
			var lastRes time.Time
			if val, ok := g.LastResolution.Load(fqdn); ok {
				if t, ok := val.(time.Time); ok {
					lastRes = t
				}
			}

			for _, backend := range record.Backends {
				status := "unhealthy"
				if backend.IsAlive() {
					status = "healthy"
				}
				key := fqdn + "|" + backend.GetAddress()
				state[key] = BackendState{
					Status:         status,
					LastCheck:      backend.GetLastHealthcheck(),
					LastResolution: lastRes,
				}
			}
			record.mutex.RUnlock()
		}
	}
	g.Mutex.RUnlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// Ensure the parent directory exists
	dir := filepath.Dir(g.StatePersistPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write to temporary file first, then rename for atomic write
	tmpFile := g.StatePersistPath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, g.StatePersistPath)
}

// LoadState reads the persisted state from disk and applies it.
func (g *GSLB) LoadState() error {
	if !g.StatePersistEnable || g.RedisEnable {
		return nil
	}

	info, err := os.Stat(g.StatePersistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No state file is fine
		}
		return err
	}

	// Check file age
	maxAge, err := time.ParseDuration(g.StateMaxAge)
	if err != nil {
		maxAge = 60 * time.Second // default
	}

	if time.Since(info.ModTime()) > maxAge {
		log.Infof("State file %s is stale (modified %v ago, max age %v), ignoring", g.StatePersistPath, time.Since(info.ModTime()), maxAge)
		return nil
	}

	data, err := os.ReadFile(g.StatePersistPath)
	if err != nil {
		return err
	}

	var state map[string]BackendState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	g.Mutex.Lock()
	defer g.Mutex.Unlock()

	for key, bState := range state {
		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 {
			continue
		}
		fqdn := parts[0]
		addr := parts[1]

		// Find backend and apply state
		found := false
		for _, records := range g.Records {
			if record, exists := records[fqdn]; exists {
				record.mutex.Lock()
				for _, backend := range record.Backends {
					if backend.GetAddress() == addr {
						backend.SetAlive(bState.Status == "healthy")
						backend.SetLastHealthcheck(bState.LastCheck)
						found = true
						break
					}
				}
				record.mutex.Unlock()
			}
		}

		if found {
			// Apply last resolution time if it is newer than what we currently have
			if !bState.LastResolution.IsZero() {
				currentVal, exists := g.LastResolution.Load(fqdn)
				if !exists {
					g.LastResolution.Store(fqdn, bState.LastResolution)
				} else if currentT, ok := currentVal.(time.Time); ok && bState.LastResolution.After(currentT) {
					g.LastResolution.Store(fqdn, bState.LastResolution)
				}
			}
		}
	}

	log.Infof("Loaded initial state for %d backend(s) from %s", len(state), g.StatePersistPath)
	return nil
}

// StartStatePersistLoop starts the periodic state saving background loop.
func (g *GSLB) StartStatePersistLoop(ctx context.Context) {
	if !g.StatePersistEnable || g.RedisEnable {
		return
	}

	interval, err := time.ParseDuration(g.StatePersistInterval)
	if err != nil {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := g.SaveState(); err != nil {
					log.Errorf("Failed to save state: %v", err)
				}
			case <-ctx.Done():
				// Save final state on shutdown
				if err := g.SaveState(); err != nil {
					log.Errorf("Failed to save state on shutdown: %v", err)
				}
				return
			}
		}
	}()
}
