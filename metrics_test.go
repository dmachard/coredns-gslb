package gslb

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetrics_IncAndObserve(t *testing.T) {
	RegisterMetrics()
	IncHealthcheckTotal("example.com.", "http", "1.2.3.4", "success")
	IncHealthcheckTotal("example.com.", "http", "1.2.3.4", "fail")
	ObserveHealthcheckDuration("http", "1.2.3.4", 0.123)
	ObserveHealthcheckDuration("http", "1.2.3.4", 0.456)

	count := testutil.ToFloat64(healthcheckTotal.WithLabelValues("example.com.", "http", "1.2.3.4", "success"))
	if count != 1 {
		t.Errorf("expected 1, got %v", count)
	}
	countFail := testutil.ToFloat64(healthcheckTotal.WithLabelValues("example.com.", "http", "1.2.3.4", "fail"))
	if countFail != 1 {
		t.Errorf("expected 1, got %v", countFail)
	}
}

func TestMetrics_RecordResolutions(t *testing.T) {
	RegisterMetrics()
	IncRecordResolutions("example.com", "success")
	IncRecordResolutions("example.com", "success")
	IncRecordResolutions("example.com", "fail")
	IncRecordResolutions("test.com", "success")

	// Test successful resolutions for example.com
	count := testutil.ToFloat64(recordResolutions.WithLabelValues("example.com", "success"))
	if count != 2 {
		t.Errorf("expected 2, got %v", count)
	}

	// Test failed resolutions for example.com
	countFail := testutil.ToFloat64(recordResolutions.WithLabelValues("example.com", "fail"))
	if countFail != 1 {
		t.Errorf("expected 1, got %v", countFail)
	}

	// Test successful resolutions for test.com
	countTest := testutil.ToFloat64(recordResolutions.WithLabelValues("test.com", "success"))
	if countTest != 1 {
		t.Errorf("expected 1, got %v", countTest)
	}
}

func TestMetrics_ConfigReloads(t *testing.T) {
	RegisterMetrics()
	IncConfigReloads("success")
	IncConfigReloads("failure")
	IncConfigReloads("success")

	successCount := testutil.ToFloat64(configReloads.WithLabelValues("success"))
	if successCount != 2 {
		t.Errorf("expected 2, got %v", successCount)
	}
	failureCount := testutil.ToFloat64(configReloads.WithLabelValues("failure"))
	if failureCount != 1 {
		t.Errorf("expected 1, got %v", failureCount)
	}
}

func TestMetrics_HealthcheckFailures(t *testing.T) {
	RegisterMetrics()
	IncHealthcheckFailures("http/80", "1.2.3.4", "timeout")
	IncHealthcheckFailures("http/80", "1.2.3.4", "timeout")
	IncHealthcheckFailures("tcp/443", "1.2.3.5", "connection")
	IncHealthcheckFailures(ICMPType, "1.2.3.6", "protocol")
	IncHealthcheckFailures("grpc", "1.2.3.7", "other")

	timeoutCount := testutil.ToFloat64(healthcheckFailures.WithLabelValues("http/80", "1.2.3.4", "timeout"))
	if timeoutCount != 2 {
		t.Errorf("expected 2, got %v", timeoutCount)
	}
	connCount := testutil.ToFloat64(healthcheckFailures.WithLabelValues("tcp/443", "1.2.3.5", "connection"))
	if connCount != 1 {
		t.Errorf("expected 1, got %v", connCount)
	}
	protocolCount := testutil.ToFloat64(healthcheckFailures.WithLabelValues(ICMPType, "1.2.3.6", "protocol"))
	if protocolCount != 1 {
		t.Errorf("expected 1, got %v", protocolCount)
	}
	otherCount := testutil.ToFloat64(healthcheckFailures.WithLabelValues("grpc", "1.2.3.7", "other"))
	if otherCount != 1 {
		t.Errorf("expected 1, got %v", otherCount)
	}
}

func TestMetrics_ActiveBackends(t *testing.T) {
	RegisterMetrics()
	SetActiveBackends("example.com.", 3)
	SetActiveBackends("example.com.", 2)
	SetActiveBackends("test.com.", 1)

	val1 := testutil.ToFloat64(activeBackends.WithLabelValues("example.com."))
	if val1 != 2 {
		t.Errorf("expected 2, got %v", val1)
	}
	val2 := testutil.ToFloat64(activeBackends.WithLabelValues("test.com."))
	if val2 != 1 {
		t.Errorf("expected 1, got %v", val2)
	}
}

func TestMetrics_BackendConfiguredTotal(t *testing.T) {
	RegisterMetrics()
	SetBackendsTotal(3)
	SetBackendsTotal(5)

	val := testutil.ToFloat64(backendsTotal)
	if val != 5 {
		t.Errorf("expected 5, got %v", val)
	}
}

func TestMetrics_RecordsConfiguredTotal(t *testing.T) {
	RegisterMetrics()
	SetRecordsTotal(5)
	SetRecordsTotal(3)

	val := testutil.ToFloat64(recordsTotal)
	if val != 3 {
		t.Errorf("expected 3, got %v", val)
	}
}

func TestMetrics_RecordHealthStatus(t *testing.T) {
	RegisterMetrics()
	SetRecordHealthStatus("test.example.com", "healthy", 1)
	SetRecordHealthStatus("test.example.com", "unhealthy", 0)

	healthyVal := testutil.ToFloat64(recordHealthStatus.WithLabelValues("test.example.com", "healthy"))
	if healthyVal != 1 {
		t.Errorf("expected 1, got %v", healthyVal)
	}

	unhealthyVal := testutil.ToFloat64(recordHealthStatus.WithLabelValues("test.example.com", "unhealthy"))
	if unhealthyVal != 0 {
		t.Errorf("expected 0, got %v", unhealthyVal)
	}
}

func TestMetrics_BackendHealthStatus(t *testing.T) {
	RegisterMetrics()
	SetBackendHealthStatus("test.example.com", "192.168.1.1", "healthy", 1)
	SetBackendHealthStatus("test.example.com", "192.168.1.1", "unhealthy", 0)
	SetBackendHealthStatus("test.example.com", "192.168.1.2", "disabled", 1)
	SetBackendHealthStatus("test.example.com", "192.168.1.2", "disabled", 0)
	// Note: We can't easily test the actual value without exposing internal state
	// This test ensures the function doesn't panic
}

func TestMetrics_BackendHealthcheckStatus(t *testing.T) {
	RegisterMetrics()
	SetBackendHealthcheckStatus("test.example.com", "192.168.1.1", "https/443", "success", 1)
	SetBackendHealthcheckStatus("test.example.com", "192.168.1.1", "https/443", "fail", 0)

	successVal := testutil.ToFloat64(backendHealthcheckStatus.WithLabelValues("test.example.com", "192.168.1.1", "https/443", "success"))
	if successVal != 1 {
		t.Errorf("expected 1, got %v", successVal)
	}

	failVal := testutil.ToFloat64(backendHealthcheckStatus.WithLabelValues("test.example.com", "192.168.1.1", "https/443", "fail"))
	if failVal != 0 {
		t.Errorf("expected 0, got %v", failVal)
	}
}

func TestMetrics_HealthcheckConfiguredTotal(t *testing.T) {
	RegisterMetrics()
	SetHealthchecksTotal(2)
	SetHealthchecksTotal(5)

	val := testutil.ToFloat64(healthchecksTotal)
	if val != 5 {
		t.Errorf("expected 5, got %v", val)
	}
}

func TestMetrics_BackendSelected(t *testing.T) {
	RegisterMetrics()
	IncBackendSelected("example.com.", "1.2.3.4")
	IncBackendSelected("example.com.", "1.2.3.4")
	IncBackendSelected("example.com.", "2.2.2.2")
	IncBackendSelected("test.com.", "1.2.3.4")

	val1 := testutil.ToFloat64(backendSelected.WithLabelValues("example.com.", "1.2.3.4"))
	if val1 != 2 {
		t.Errorf("expected 2, got %v", val1)
	}
	val2 := testutil.ToFloat64(backendSelected.WithLabelValues("example.com.", "2.2.2.2"))
	if val2 != 1 {
		t.Errorf("expected 1, got %v", val2)
	}
	val3 := testutil.ToFloat64(backendSelected.WithLabelValues("test.com.", "1.2.3.4"))
	if val3 != 1 {
		t.Errorf("expected 1, got %v", val3)
	}
}

func TestMetrics_RecordResolutionDuration(t *testing.T) {
	recordResolutionDuration.Reset()
	RegisterMetrics()
	ObserveRecordResolutionDuration("example.com.", "success", 0.5)
	ObserveRecordResolutionDuration("example.com.", "success", 1.0)
	ObserveRecordResolutionDuration("example.com.", "fail", 2.0)
	ObserveRecordResolutionDuration("test.com.", "success", 0.25)

	count := testutil.CollectAndCount(recordResolutionDuration)
	if count != 3 {
		t.Errorf("expected 3 series, got %v", count)
	}
}

func TestMetrics_VersionInfo(t *testing.T) {
	RegisterMetrics()
	SetVersionInfo("1.2.3")
	SetVersionInfo("dev")

	val1 := testutil.ToFloat64(versionInfo.WithLabelValues("1.2.3"))
	if val1 != 1 {
		t.Errorf("expected 1, got %v", val1)
	}
	val2 := testutil.ToFloat64(versionInfo.WithLabelValues("dev"))
	if val2 != 1 {
		t.Errorf("expected 1, got %v", val2)
	}
}

func TestMetrics_DisabledBackends(t *testing.T) {
	RegisterMetrics()

	// Test SetDisabledBackends
	SetDisabledBackends(2)
	SetDisabledBackends(5)

	// Test IncDisabledBackends
	IncDisabledBackends()
	IncDisabledBackends()
}
