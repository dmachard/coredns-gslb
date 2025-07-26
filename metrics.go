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
		[]string{"name", "type", "address", "result"},
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

	backendSelected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gslb_backend_selected_total",
			Help: "Total number of times a backend was selected for a record",
		},
		[]string{"name", "address"},
	)

	recordResolutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gslb_record_resolution_duration_seconds",
			Help:    "Duration of GSLB record resolution in seconds, labeled by record name and result",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"name", "result"},
	)

	versionInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gslb_version_info",
			Help: "GSLB build version info (label: version)",
		},
		[]string{"version"},
	)

	healthchecksTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gslb_healthchecks_total",
			Help: "Number of healthchecks configured (total for all records/backends)",
		},
	)

	backendsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gslb_backends_total",
			Help: "Total number of backends configured (all records)",
		},
	)

	zonesTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gslb_zones_total",
			Help: "Total number of DNS zones configured.",
		},
	)

	recordsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gslb_records_total",
			Help: "Total number of GSLB records configured.",
		},
	)
	recordHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gslb_record_health_status",
			Help: "Health status per record (1 = healthy, 0 = unhealthy).",
		},
		[]string{"name", "status"},
	)
	backendHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gslb_backend_health_status",
			Help: "Health status per backend (1 = healthy, 0 = unhealthy).",
		},
		[]string{"name", "address", "status"},
	)
	backendHealthcheckStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gslb_backend_healthcheck_status",
			Help: "Healthcheck status per backend and type (1 = success, 0 = fail).",
		},
		[]string{"name", "address", "type", "status"},
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
		prometheus.MustRegister(backendSelected)
		prometheus.MustRegister(recordResolutionDuration)
		prometheus.MustRegister(versionInfo)
		prometheus.MustRegister(healthchecksTotal)
		prometheus.MustRegister(backendsTotal)
		prometheus.MustRegister(zonesTotal)
		prometheus.MustRegister(recordsTotal)
		prometheus.MustRegister(recordHealthStatus)
		prometheus.MustRegister(backendHealthStatus)
		prometheus.MustRegister(backendHealthcheckStatus)
	})
}

func IncHealthcheckTotal(name, typ, address, result string) {
	healthcheckTotal.WithLabelValues(name, typ, address, result).Inc()
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

func IncBackendSelected(name, address string) {
	backendSelected.WithLabelValues(name, address).Inc()
}

func SetVersionInfo(version string) {
	versionInfo.WithLabelValues(version).Set(1)
}

func SetHealthchecksTotal(value float64) {
	healthchecksTotal.Set(value)
}
func SetBackendsTotal(value float64) {
	backendsTotal.Set(value)
}

func SetZonesTotal(value float64) {
	zonesTotal.Set(value)
}

func SetRecordsTotal(value float64) {
	recordsTotal.Set(value)
}
func SetRecordHealthStatus(name, status string, value float64) {
	recordHealthStatus.WithLabelValues(name, status).Set(value)
}
func SetBackendHealthStatus(name, address, status string, value float64) {
	backendHealthStatus.WithLabelValues(name, address, status).Set(value)
}
func SetBackendHealthcheckStatus(name, address, typeStr, status string, value float64) {
	backendHealthcheckStatus.WithLabelValues(name, address, typeStr, status).Set(value)
}

func ObserveHealthcheck(name, typeStr, address string, start time.Time, result bool) {
	// Log the health check result
	// log.Debugf("Record health check for metrics: type=%s, address=%s, result=%t", typeStr, address, result)
	dur := time.Since(start).Seconds()
	if result {
		IncHealthcheckTotal(name, typeStr, address, "success")
	} else {
		IncHealthcheckTotal(name, typeStr, address, "fail")
	}
	ObserveHealthcheckDuration(typeStr, address, dur)
}

func ObserveRecordResolutionDuration(name, result string, duration float64) {
	recordResolutionDuration.WithLabelValues(name, result).Observe(duration)
}
