package gslb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestRecord_UnmarshalYAML(t *testing.T) {
	yamlData := `
mode: "failover"
owner: "admin"
description: "Test record"
record_ttl: 60
scrape_interval: "15s"
scrape_retries: 3
scrape_timeout: "10s"
backends:
  - address: "192.168.1.1"
    enable: true
`

	var record Record
	err := yaml.Unmarshal([]byte(yamlData), &record)
	assert.NoError(t, err)
	assert.Nil(t, err)
	assert.Equal(t, "failover", record.Mode)
	assert.Equal(t, "admin", record.Owner)
	assert.Equal(t, 60, record.RecordTTL)
	assert.Equal(t, "15s", record.ScrapeInterval)
	assert.Equal(t, 3, record.ScrapeRetries)
	assert.Equal(t, "10s", record.ScrapeTimeout)
	assert.Len(t, record.Backends, 1)
	assert.Equal(t, "192.168.1.1", record.Backends[0].GetAddress())
}

func TestRecord_UpdateRecord(t *testing.T) {
	record := &Record{
		Fqdn:  "example.com",
		Mode:  "failover",
		Owner: "admin",
	}

	newRecord := &Record{
		Fqdn:  "example.com",
		Mode:  "round-robin",
		Owner: "admin",
	}

	record.updateRecord(newRecord)

	assert.Equal(t, "round-robin", record.Mode)
}

func TestRecord_ScrapeInterval(t *testing.T) {
	record := &Record{
		ScrapeInterval: "350s",
	}

	interval := record.GetScrapeInterval()
	assert.Equal(t, 350*time.Second, interval)
}

func TestRecord_ScrapeTimeout(t *testing.T) {
	record := &Record{
		ScrapeTimeout: "5s",
	}

	timeout := record.GetScrapeTimeout()
	assert.Equal(t, 5*time.Second, timeout)
}

func TestRecord_ScrapeBackends_Slowdown(t *testing.T) {
	idleTimeout := "1s"
	multiplier := 3

	g := &GSLB{
		ResolutionIdleTimeout:     idleTimeout,
		HealthcheckIdleMultiplier: multiplier,
	}
	rec := &Record{
		Fqdn:           "test.example.com.",
		ScrapeInterval: "100ms",
	}

	backend := &callCounter{}
	rec.Backends = []BackendInterface{backend}
	g.LastResolution.Store(rec.Fqdn, time.Now().Add(-2*time.Second))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go rec.scrapeBackends(ctx, g)

	time.Sleep(500 * time.Millisecond)
	cancel()

	if backend.calls < 1 || backend.calls > 2 {
		t.Errorf("expected 1 or 2 healthchecks, got %d", backend.calls)
	}
}

type callCounter struct {
	Backend
	calls int
}

func (b *callCounter) runHealthChecks(retries int, timeout time.Duration) {
	b.calls++
}
func (b *callCounter) GetFqdn() string                           { return "test.example.com." }
func (b *callCounter) SetFqdn(fqdn string)                       {}
func (b *callCounter) GetDescription() string                    { return "" }
func (b *callCounter) GetAddress() string                        { return "127.0.0.1" }
func (b *callCounter) GetPriority() int                          { return 1 }
func (b *callCounter) IsEnabled() bool                           { return true }
func (b *callCounter) GetHealthChecks() []GenericHealthCheck     { return nil }
func (b *callCounter) GetTimeout() string                        { return "" }
func (b *callCounter) GetLocation() string                       { return "" }
func (b *callCounter) GetCountry() string                        { return "" }
func (b *callCounter) IsHealthy() bool                           { return true }
func (b *callCounter) removeBackend()                            {}
func (b *callCounter) updateBackend(newBackend BackendInterface) {}
func (b *callCounter) Lock()                                     {}
func (b *callCounter) Unlock()                                   {}
