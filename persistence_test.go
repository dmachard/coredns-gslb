package gslb

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/stretchr/testify/assert"
)

func TestSaveAndLoadState(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// 1. Initialize GSLB with records and backends
	g := &GSLB{
		Records:              make(map[string]map[string]*Record),
		StatePersistEnable:   true,
		StatePersistPath:     statePath,
		StatePersistInterval: "30s",
		StateMaxAge:          "60s",
	}

	zone := "example.com."
	g.Records[zone] = make(map[string]*Record)

	backend1 := &Backend{Address: "192.168.1.1", Enable: true}
	backend2 := &Backend{Address: "192.168.1.2", Enable: true}

	// Set initial states
	backend1.SetAlive(true)
	backend1.SetLastHealthcheck(time.Now().Add(-5 * time.Second))
	backend2.SetAlive(false)
	backend2.SetLastHealthcheck(time.Now().Add(-10 * time.Second))

	record := &Record{
		Fqdn:     "webapp.example.com.",
		Backends: []BackendInterface{backend1, backend2},
	}
	g.Records[zone]["webapp.example.com."] = record

	// Store last resolution time
	resTime := time.Now().Add(-2 * time.Second).Truncate(time.Second)
	g.LastResolution.Store("webapp.example.com.", resTime)

	// 2. Save state
	err := g.SaveState()
	assert.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(statePath)
	assert.NoError(t, err)

	// Verify content structure
	data, err := os.ReadFile(statePath)
	assert.NoError(t, err)
	var serialized map[string]BackendState
	err = json.Unmarshal(data, &serialized)
	assert.NoError(t, err)
	assert.Contains(t, serialized, "webapp.example.com.|192.168.1.1")
	assert.Contains(t, serialized, "webapp.example.com.|192.168.1.2")
	assert.Equal(t, "healthy", serialized["webapp.example.com.|192.168.1.1"].Status)
	assert.Equal(t, "unhealthy", serialized["webapp.example.com.|192.168.1.2"].Status)

	// 3. Load state into a fresh GSLB structure
	g2 := &GSLB{
		Records:              make(map[string]map[string]*Record),
		StatePersistEnable:   true,
		StatePersistPath:     statePath,
		StatePersistInterval: "30s",
		StateMaxAge:          "60s",
	}
	g2.Records[zone] = make(map[string]*Record)

	b1 := &Backend{Address: "192.168.1.1", Enable: true}
	b2 := &Backend{Address: "192.168.1.2", Enable: true}
	// Initialized to false/zero
	b1.SetAlive(false)
	b2.SetAlive(true)

	r2 := &Record{
		Fqdn:     "webapp.example.com.",
		Backends: []BackendInterface{b1, b2},
	}
	g2.Records[zone]["webapp.example.com."] = r2

	err = g2.LoadState()
	assert.NoError(t, err)

	// Verify loaded states
	assert.True(t, b1.IsAlive())
	assert.False(t, b2.IsAlive())
	assert.WithinDuration(t, backend1.GetLastHealthcheck(), b1.GetLastHealthcheck(), time.Second)

	// Verify LastResolution time
	val, ok := g2.LastResolution.Load("webapp.example.com.")
	assert.True(t, ok)
	loadedResTime := val.(time.Time)
	assert.True(t, loadedResTime.Equal(resTime) || loadedResTime.Sub(resTime) < time.Second)
}

func TestLoadStateMaxAge(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	g := &GSLB{
		Records:              make(map[string]map[string]*Record),
		StatePersistEnable:   true,
		StatePersistPath:     statePath,
		StatePersistInterval: "30s",
		StateMaxAge:          "1s", // very short max age
	}

	zone := "example.com."
	g.Records[zone] = make(map[string]*Record)

	backend := &Backend{Address: "192.168.1.1", Enable: true}
	backend.SetAlive(true)

	record := &Record{
		Fqdn:     "webapp.example.com.",
		Backends: []BackendInterface{backend},
	}
	g.Records[zone]["webapp.example.com."] = record

	// Save state
	err := g.SaveState()
	assert.NoError(t, err)

	// Sleep to let the state become stale
	time.Sleep(1500 * time.Millisecond)

	// Try loading state into fresh GSLB with same configuration
	g2 := &GSLB{
		Records:              make(map[string]map[string]*Record),
		StatePersistEnable:   true,
		StatePersistPath:     statePath,
		StatePersistInterval: "30s",
		StateMaxAge:          "1s",
	}
	g2.Records[zone] = make(map[string]*Record)

	b := &Backend{Address: "192.168.1.1", Enable: true}
	b.SetAlive(false) // default offline

	r := &Record{
		Fqdn:     "webapp.example.com.",
		Backends: []BackendInterface{b},
	}
	g2.Records[zone]["webapp.example.com."] = r

	err = g2.LoadState()
	assert.NoError(t, err)

	// Since state is stale, it should not have been loaded. Backend stays false.
	assert.False(t, b.IsAlive())
}

func TestStatePersistParsing(t *testing.T) {
	configStr := `gslb {
		zone app-x.gslb.example.com ./tests/db.app-x.gslb.example.com.yml
		state_persist_enable true
		state_persist_path "/tmp/custom_state.json"
		state_persist_interval "45s"
		state_max_age "90s"
	}`

	c := caddy.NewTestController("dns", configStr)
	err := setup(c)
	assert.NoError(t, err)

	// Retrieve the GSLB config and verify fields
	dnscfg := dnsserver.GetConfig(c)
	plugins := dnscfg.Plugin
	assert.NotEmpty(t, plugins)

	// Retrieve the GSLB instance by executing the plugin generator function
	handler := plugins[len(plugins)-1](nil)
	g, ok := handler.(*GSLB)
	assert.True(t, ok)
	assert.NotNil(t, g)

	assert.True(t, g.StatePersistEnable)
	assert.Equal(t, "/tmp/custom_state.json", g.StatePersistPath)
	assert.Equal(t, "45s", g.StatePersistInterval)
	assert.Equal(t, "90s", g.StateMaxAge)
}

func TestSaveState_DisabledOrRedis(t *testing.T) {
	// If StatePersistEnable is false, SaveState should do nothing and return nil
	g := &GSLB{
		StatePersistEnable: false,
	}
	err := g.SaveState()
	assert.NoError(t, err)

	// If RedisEnable is true, SaveState should do nothing and return nil
	g2 := &GSLB{
		StatePersistEnable: true,
		RedisEnable:        true,
	}
	err = g2.SaveState()
	assert.NoError(t, err)
}

func TestLoadState_DisabledOrRedisOrNonexistent(t *testing.T) {
	// If StatePersistEnable is false, LoadState should return nil
	g := &GSLB{
		StatePersistEnable: false,
	}
	err := g.LoadState()
	assert.NoError(t, err)

	// If RedisEnable is true, LoadState should return nil
	g2 := &GSLB{
		StatePersistEnable: true,
		RedisEnable:        true,
	}
	err = g2.LoadState()
	assert.NoError(t, err)

	// If state file does not exist, LoadState should return nil
	g3 := &GSLB{
		StatePersistEnable: true,
		StatePersistPath:   "/path/to/nonexistent/state.json",
	}
	err = g3.LoadState()
	assert.NoError(t, err)
}

func TestLoadState_InvalidJson(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	err := os.WriteFile(statePath, []byte("{invalid-json}"), 0644)
	assert.NoError(t, err)

	g := &GSLB{
		StatePersistEnable: true,
		StatePersistPath:   statePath,
		StateMaxAge:        "60s",
	}

	err = g.LoadState()
	assert.Error(t, err)
}

func TestLoadState_InvalidMaxAgeFormat(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	stateContent := `{"api.example.com.|192.168.1.10": {"status": "healthy"}}`
	err := os.WriteFile(statePath, []byte(stateContent), 0644)
	assert.NoError(t, err)

	g := &GSLB{
		Records:            make(map[string]map[string]*Record),
		StatePersistEnable: true,
		StatePersistPath:   statePath,
		StateMaxAge:        "invalid-duration", // should fallback to 60s default
	}

	zone := "example.com."
	g.Records[zone] = make(map[string]*Record)
	backend := &Backend{Address: "192.168.1.10"}
	backend.SetAlive(false)
	g.Records[zone]["api.example.com."] = &Record{
		Fqdn:     "api.example.com.",
		Backends: []BackendInterface{backend},
	}

	err = g.LoadState()
	assert.NoError(t, err)
	// Should successfully fallback and load the healthy status
	assert.True(t, backend.IsAlive())
}

func TestLoadState_MalformedKeysAndNoMatches(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// State contains a key with no pipe, a key that has no matches, and a valid key
	stateContent := `{
		"malformedkey": {"status": "healthy"},
		"nonexistent.com.|192.168.1.1": {"status": "healthy"},
		"api.example.com.|192.168.1.10": {"status": "healthy", "last_resolution": "2026-07-07T09:00:00Z"}
	}`
	err := os.WriteFile(statePath, []byte(stateContent), 0644)
	assert.NoError(t, err)

	g := &GSLB{
		Records:            make(map[string]map[string]*Record),
		StatePersistEnable: true,
		StatePersistPath:   statePath,
		StateMaxAge:        "60s",
	}

	zone := "example.com."
	g.Records[zone] = make(map[string]*Record)
	backend := &Backend{Address: "192.168.1.10"}
	backend.SetAlive(false)
	g.Records[zone]["api.example.com."] = &Record{
		Fqdn:     "api.example.com.",
		Backends: []BackendInterface{backend},
	}

	err = g.LoadState()
	assert.NoError(t, err)

	// Valid backend should be updated to true
	assert.True(t, backend.IsAlive())

	// LastResolution should be updated
	val, ok := g.LastResolution.Load("api.example.com.")
	assert.True(t, ok)
	assert.False(t, val.(time.Time).IsZero())
}

func TestSaveState_WriteFileError(t *testing.T) {
	// Try saving state to a read-only directory path to trigger write error
	g := &GSLB{
		StatePersistEnable: true,
		StatePersistPath:   "/nonexistent-dir-12345/state.json",
	}
	err := g.SaveState()
	assert.Error(t, err)
}

func TestStartStatePersistLoop_Execution(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	g := &GSLB{
		Records:              make(map[string]map[string]*Record),
		StatePersistEnable:   true,
		StatePersistPath:     statePath,
		StatePersistInterval: "20ms",
	}

	zone := "example.com."
	g.Records[zone] = make(map[string]*Record)
	backend := &Backend{Address: "192.168.1.10"}
	backend.SetAlive(true)
	g.Records[zone]["api.example.com."] = &Record{
		Fqdn:     "api.example.com.",
		Backends: []BackendInterface{backend},
	}

	ctx, cancel := context.WithCancel(context.Background())
	g.StartStatePersistLoop(ctx)

	// Wait for loop to tick at least once and write to file
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop loop and trigger shutdown save
	cancel()

	// Wait a bit to ensure shutdown finish
	time.Sleep(50 * time.Millisecond)

	// Verify file was written
	_, err := os.Stat(statePath)
	assert.NoError(t, err)

	data, err := os.ReadFile(statePath)
	assert.NoError(t, err)
	var state map[string]BackendState
	err = json.Unmarshal(data, &state)
	assert.NoError(t, err)
	assert.Contains(t, state, "api.example.com.|192.168.1.10")
	assert.Equal(t, "healthy", state["api.example.com.|192.168.1.10"].Status)
}

func TestStartStatePersistLoop_DisabledAndInvalidInterval(t *testing.T) {
	// If disabled, should do nothing
	g := &GSLB{
		StatePersistEnable: false,
	}
	ctx := context.Background()
	g.StartStatePersistLoop(ctx) // should return instantly

	// If invalid interval, should fallback to default 30s
	g2 := &GSLB{
		StatePersistEnable:   true,
		StatePersistInterval: "invalid-duration",
	}
	g2.StartStatePersistLoop(ctx)
}
