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
)

var metricsOnce sync.Once

func RegisterMetrics() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(healthcheckTotal)
		prometheus.MustRegister(healthcheckDuration)
	})
}

func IncHealthcheckTotal(typ, address, result string) {
	healthcheckTotal.WithLabelValues(typ, address, result).Inc()
}

func ObserveHealthcheckDuration(typ, address string, duration float64) {
	healthcheckDuration.WithLabelValues(typ, address).Observe(duration)
}

func ObserveHealthcheck(typeStr, address string, start time.Time, result bool) {
	// Log the health check result
	log.Debugf("Record health check for metrics: type=%s, address=%s, result=%t", typeStr, address, result)
	dur := time.Since(start).Seconds()
	if result {
		IncHealthcheckTotal(typeStr, address, "success")
	} else {
		IncHealthcheckTotal(typeStr, address, "fail")
	}
	ObserveHealthcheckDuration(typeStr, address, dur)
}
