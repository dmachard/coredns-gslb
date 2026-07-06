package gslb

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

const testFqdn = "test.example.com."

func TestRecord_UnmarshalYAML(t *testing.T) {
	yamlData := `
mode: "failover"
owner: "admin"
description: "Test record"
record_ttl: 60
scrape_interval: "15s"
scrape_retries: 3
scrape_timeout: "10s"
alpn:
  - "h3"
  - "h2"
fallback_policy:
  mode: "fail-closed"
  rcode: "NXDOMAIN"
  fallback_ips:
    - "1.2.3.4"
  fallback_cname: "backup.cdn.cloudflare.net"
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
	assert.Equal(t, "fail-closed", record.FallbackPolicy.Mode)
	assert.Equal(t, "NXDOMAIN", record.FallbackPolicy.Rcode)
	assert.Len(t, record.FallbackPolicy.FallbackIPs, 1)
	assert.Equal(t, "1.2.3.4", record.FallbackPolicy.FallbackIPs[0])
	assert.Equal(t, "backup.cdn.cloudflare.net", record.FallbackPolicy.FallbackCNAME)
	assert.Len(t, record.Backends, 1)
	assert.Equal(t, "192.168.1.1", record.Backends[0].GetAddress())
	assert.Equal(t, []string{"h3", "h2"}, record.ALPN)
}

func TestRecord_UpdateRecord(t *testing.T) {
	record := &Record{
		Fqdn:  "example.com",
		Mode:  "failover",
		Owner: "admin",
		FallbackPolicy: FallbackPolicy{
			Mode: "fail-open",
		},
	}

	newRecord := &Record{
		Fqdn:  "example.com",
		Mode:  "round-robin",
		Owner: "admin",
		FallbackPolicy: FallbackPolicy{
			Mode:  "fail-closed",
			Rcode: "NXDOMAIN",
		},
	}

	record.updateRecord(newRecord)

	assert.Equal(t, "round-robin", record.Mode)
	assert.Equal(t, "fail-closed", record.FallbackPolicy.Mode)
	assert.Equal(t, "NXDOMAIN", record.FallbackPolicy.Rcode)
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
		Fqdn:           testFqdn,
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

	if atomic.LoadInt32(&backend.calls) < 1 || atomic.LoadInt32(&backend.calls) > 2 {
		t.Errorf("expected 1 or 2 healthchecks, got %d", atomic.LoadInt32(&backend.calls))
	}
}

type callCounter struct {
	Backend
	calls int32 // use atomic for thread safety
}

func (b *callCounter) runHealthChecks(retries int, timeout time.Duration) bool {
	atomic.AddInt32(&b.calls, 1)
	return true
}
func (b *callCounter) GetFqdn() string                           { return testFqdn }
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

func TestRecord_UnmarshalYAML_Discovery(t *testing.T) {
	yamlData := `
mode: "failover"
discovery:
  type: "consul"
  endpoint: "http://consul:8500"
  service: "my-service"
  interval: "10s"
  tag: "prod"
backends:
  - address: "192.168.1.1"
`

	var record Record
	err := yaml.Unmarshal([]byte(yamlData), &record)
	assert.NoError(t, err)
	assert.NotNil(t, record.Discovery)
	assert.Equal(t, "consul", record.Discovery.Type)
	assert.Equal(t, "http://consul:8500", record.Discovery.Endpoint)
	assert.Equal(t, "my-service", record.Discovery.Service)
	assert.Equal(t, "10s", record.Discovery.Interval)
	assert.Equal(t, "prod", record.Discovery.Tag)
}

func TestRecord_UpdateRecord_Discovery(t *testing.T) {
	record := &Record{
		Fqdn: "example.com",
		Discovery: &DiscoveryConfig{
			Type: "http",
		},
	}

	newRecord := &Record{
		Fqdn: "example.com",
		Discovery: &DiscoveryConfig{
			Type: "consul",
		},
	}

	record.updateRecord(newRecord)

	assert.NotNil(t, record.Discovery)
	assert.Equal(t, "consul", record.Discovery.Type)
}

func TestRecord_UpdateRecord_Full(t *testing.T) {
	b1 := &Backend{Address: "192.168.1.1", Enable: true}
	b2 := &Backend{Address: "192.168.1.2", Enable: true}

	rec := &Record{
		Fqdn:           "test.local",
		Mode:           "failover",
		Owner:          "admin",
		RecordTTL:      30,
		Description:    "old desc",
		ScrapeInterval: "10s",
		ScrapeRetries:  1,
		ScrapeTimeout:  "5s",
		FallbackPolicy: FallbackPolicy{Mode: "fail-open", Rcode: "NOERROR", FallbackIPs: []string{"1.1.1.1"}},
		Discovery:      &DiscoveryConfig{Type: "consul"},
		ALPN:           []string{"h3"},
		Backends:       []BackendInterface{b1, b2},
		ticker:         time.NewTicker(10 * time.Second),
	}

	newB2 := &Backend{Address: "192.168.1.2", Enable: true, Weight: 5} // to be updated
	newB3 := &Backend{Address: "192.168.1.3", Enable: true}            // to be added
	// b1 will be removed

	newRec := &Record{
		Fqdn:           "test.local",
		Mode:           "round-robin",
		Owner:          "user",
		RecordTTL:      60,
		Description:    "new desc",
		ScrapeInterval: "5s",
		ScrapeRetries:  2,
		ScrapeTimeout:  "3s",
		FallbackPolicy: FallbackPolicy{Mode: "fail-closed", Rcode: "NXDOMAIN", FallbackIPs: []string{"1.1.1.1", "2.2.2.2"}},
		Discovery:      &DiscoveryConfig{Type: "http"},
		ALPN:           []string{"h3", "h2"},
		Backends:       []BackendInterface{newB2, newB3},
	}

	rec.updateRecord(newRec)

	assert.Equal(t, "round-robin", rec.Mode)
	assert.Equal(t, "user", rec.Owner)
	assert.Equal(t, 60, rec.RecordTTL)
	assert.Equal(t, "new desc", rec.Description)
	assert.Equal(t, "5s", rec.ScrapeInterval)
	assert.Equal(t, 2, rec.ScrapeRetries)
	assert.Equal(t, "3s", rec.ScrapeTimeout)
	assert.Equal(t, "fail-closed", rec.FallbackPolicy.Mode)
	assert.Equal(t, "NXDOMAIN", rec.FallbackPolicy.Rcode)
	assert.Len(t, rec.FallbackPolicy.FallbackIPs, 2)
	assert.Equal(t, "http", rec.Discovery.Type)
	assert.Equal(t, []string{"h3", "h2"}, rec.ALPN)

	// Backends check
	assert.Len(t, rec.Backends, 2)
	assert.Equal(t, "192.168.1.2", rec.Backends[0].GetAddress())
	assert.Equal(t, "192.168.1.3", rec.Backends[1].GetAddress())

	// Call UpdateRecord to cover it
	rec.UpdateRecord()
}

func TestAlpnEqual(t *testing.T) {
	assert.True(t, alpnEqual(nil, nil))
	assert.True(t, alpnEqual([]string{}, []string{}))
	assert.True(t, alpnEqual([]string{"h3", "h2"}, []string{"h3", "h2"}))

	assert.False(t, alpnEqual([]string{"h3"}, []string{"h3", "h2"}))
	assert.False(t, alpnEqual([]string{"h3", "h2"}, []string{"h3", "h1"}))
}
