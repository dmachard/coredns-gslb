package gslb

import (
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
