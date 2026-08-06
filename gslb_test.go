package gslb

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetResolutionIdleTimeout_WithCustomValue(t *testing.T) {
	r := &GSLB{
		ResolutionIdleTimeout: "100s",
	}

	timeout := r.GetResolutionIdleTimeout()

	assert.Equal(t, 100*time.Second, timeout)
}

func TestGetResolutionIdleTimeout_DefaultValue(t *testing.T) {
	r := &GSLB{}

	timeout := r.GetResolutionIdleTimeout()

	assert.Equal(t, 3600*time.Second, timeout)
}

func TestGSLB_UpdateLastResolutionTime(t *testing.T) {
	g := &GSLB{}
	domain := "test.example.com."
	g.updateLastResolutionTime(domain)
	v, ok := g.LastResolution.Load(domain)
	assert.True(t, ok)
	timeVal, ok := v.(time.Time)
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now(), timeVal, time.Second)
}

func TestGSLB_Name(t *testing.T) {
	g := &GSLB{}
	assert.Equal(t, "gslb", g.Name())
}

func TestGSLB_RemainingEdgeCases(t *testing.T) {
	// 1. GetMaxStaggerStart with invalid duration
	gStagger := &GSLB{MaxStaggerStart: "invalid"}
	assert.Equal(t, 60*time.Second, gStagger.GetMaxStaggerStart())

	// 2. GetResolutionIdleTimeout with invalid duration
	gIdle := &GSLB{ResolutionIdleTimeout: "invalid"}
	assert.Equal(t, 3600*time.Second, gIdle.GetResolutionIdleTimeout())

	// 3. UpdateRecords with zone that does not exist
	gUpdate := &GSLB{
		Records: map[string]map[string]*Record{
			"existing.zone.": {},
		},
	}
	newG := &GSLB{
		Records: map[string]map[string]*Record{
			"nonexistent.zone.": {},
		},
	}
	// Should log "Not yet implemented" and not crash or fail
	gUpdate.updateRecords(context.Background(), newG)
}

func TestGSLB_UpdateRecords_NewRecordGetsCancelFunc(t *testing.T) {
	g := &GSLB{
		Records: map[string]map[string]*Record{
			"example.org.": {},
		},
	}

	newG := &GSLB{
		Records: map[string]map[string]*Record{
			"example.org.": {
				"new.example.org.": {
					Mode:           "failover",
					ScrapeInterval: "1h",
					Backends: []BackendInterface{
						&Backend{Address: "192.168.1.1", Enable: true},
					},
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g.updateRecords(ctx, newG)

	rec := g.Records["example.org."]["new.example.org."]
	assert.NotNil(t, rec)
	assert.NotNil(t, rec.cancelFunc, "new record must have a cancel func so it can be stopped on reload/shutdown")
}

func TestGSLB_InitializeRecordsFromFiles(t *testing.T) {
	// Create a temp zone file
	tmpDir, err := os.MkdirTemp("", "gslb_test_init_*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	zoneFile := filepath.Join(tmpDir, "db.example.org.yml")
	content := `
records:
  api.example.org.:
    mode: failover
    backends:
      - address: "192.168.1.50"
        priority: 1
`
	err = os.WriteFile(zoneFile, []byte(content), 0644)
	assert.NoError(t, err)

	g := &GSLB{
		RedisEnable:    true,
		RedisAddr:      "127.0.0.1:6379",
		RedisPassword:  "",
		RedisDB:        0,
		RedisKeyPrefix: "initgslb:",
		RedisSyncMode:  "lock",
		Zones:          map[string]string{"example.org.": zoneFile},
		Records:        make(map[string]map[string]*Record),
	}

	err = g.ConnectRedis()
	if err != nil {
		t.Skipf("Skipping Redis integration part, connection failed: %v", err)
	}
	defer g.redisClient.Close()
	defer g.redisCancel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Set backend health in Redis to true
	err = g.SetRedisHealth(ctx, "example.org.", "api.example.org.", "192.168.1.50", true, 10*time.Second)
	assert.NoError(t, err)

	// Call initializeRecordsFromFiles
	g.initializeRecordsFromFiles(ctx, g.Zones)

	// Verify that the record was loaded and the backend is alive (loaded from Redis)
	assert.Contains(t, g.Records, "example.org.")
	zoneRecords := g.Records["example.org."]
	assert.Contains(t, zoneRecords, "api.example.org.")
	rec := zoneRecords["api.example.org."]
	assert.Len(t, rec.Backends, 1)
	assert.True(t, rec.Backends[0].IsAlive())

	// Stop any active scraper background goroutines started during initialization
	if rec.cancelFunc != nil {
		rec.cancelFunc()
	}
}

func TestGSLB_InitializeRecordsFromFiles_WithStatePersist(t *testing.T) {
	tmpDir := t.TempDir()
	zoneFile := filepath.Join(tmpDir, "db.example.org.yml")
	content := `
records:
  api.example.org.:
    mode: failover
    backends:
      - address: "192.168.1.50"
        priority: 1
`
	err := os.WriteFile(zoneFile, []byte(content), 0644)
	assert.NoError(t, err)

	stateFile := filepath.Join(tmpDir, "state.json")
	stateContent := `{
  "api.example.org.|192.168.1.50": {
    "status": "healthy",
    "last_check": "2026-07-07T09:00:00Z",
    "last_resolution": "2026-07-07T09:00:00Z"
  }
}`
	err = os.WriteFile(stateFile, []byte(stateContent), 0644)
	assert.NoError(t, err)

	g := &GSLB{
		StatePersistEnable:   true,
		StatePersistPath:     stateFile,
		StatePersistInterval: "30s",
		StateMaxAge:          "300s",
		Zones:                map[string]string{"example.org.": zoneFile},
		Records:              make(map[string]map[string]*Record),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = g.initializeRecordsFromFiles(ctx, g.Zones)
	assert.NoError(t, err)

	// Verify that the record was loaded and the backend is alive (loaded from the state file)
	assert.Contains(t, g.Records, "example.org.")
	zoneRecords := g.Records["example.org."]
	assert.Contains(t, zoneRecords, "api.example.org.")
	rec := zoneRecords["api.example.org."]
	assert.Len(t, rec.Backends, 1)
	assert.True(t, rec.Backends[0].IsAlive())

	// Stop any active scraper background goroutines started during initialization
	if rec.cancelFunc != nil {
		rec.cancelFunc()
	}
}
