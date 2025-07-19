package gslb

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	healthcheckTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gslb_healthcheck_total",
			Help: "Total number of healthchecks performed.",
		},
		[]string{"type", "address", "result"},
	)

	healthcheckDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gslb_healthcheck_duration_seconds",
			Help:    "Duration of healthchecks in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"type", "address"},
	)

	recordResolutions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gslb_record_resolution_total",
			Help: "Total number of GSLB record resolutions, labeled by record name and result",
		},
		[]string{"name", "result"},
	)

	configReloads = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gslb_config_reload_total",
			Help: "Total number of config reloads, labeled by result",
		},
		[]string{"result"},
	)

	healthcheckFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gslb_healthcheck_failures_total",
			Help: "Total number of healthcheck failures, labeled by type, address and reason",
		},
		[]string{"type", "address", "reason"},
	)

	activeBackends = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gslb_backend_active",
			Help: "Number of active (healthy) backends per record",
		},
		[]string{"name"},
	)
)

var metricsOnce sync.Once

func RegisterMetrics() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(healthcheckTotal)
		prometheus.MustRegister(healthcheckDuration)
		prometheus.MustRegister(recordResolutions)
		prometheus.MustRegister(configReloads)
		prometheus.MustRegister(healthcheckFailures)
		prometheus.MustRegister(activeBackends)
	})
}

func IncHealthcheckTotal(typ, address, result string) {
	healthcheckTotal.WithLabelValues(typ, address, result).Inc()
}

func ObserveHealthcheckDuration(typ, address string, duration float64) {
	healthcheckDuration.WithLabelValues(typ, address).Observe(duration)
}

func IncRecordResolutions(name, result string) {
	recordResolutions.WithLabelValues(name, result).Inc()
}

func IncConfigReloads(result string) {
	configReloads.WithLabelValues(result).Inc()
}

func IncHealthcheckFailures(typ, address, reason string) {
	healthcheckFailures.WithLabelValues(typ, address, reason).Inc()
}

func SetActiveBackends(name string, value float64) {
	activeBackends.WithLabelValues(name).Set(value)
}

func ObserveHealthcheck(typeStr, address string, start time.Time, result bool) {
	// Log the health check result
	// log.Debugf("Record health check for metrics: type=%s, address=%s, result=%t", typeStr, address, result)
	dur := time.Since(start).Seconds()
	if result {
		IncHealthcheckTotal(typeStr, address, "success")
	} else {
		IncHealthcheckTotal(typeStr, address, "fail")
	}
	ObserveHealthcheckDuration(typeStr, address, dur)
}
