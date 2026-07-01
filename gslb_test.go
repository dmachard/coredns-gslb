package gslb

import (
	"context"
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
