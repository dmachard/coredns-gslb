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

	ctx := context.Background()
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
