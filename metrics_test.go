package gslb

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetrics_IncAndObserve(t *testing.T) {
	RegisterMetrics()
	IncHealthcheckTotal("http", "1.2.3.4", "success")
	IncHealthcheckTotal("http", "1.2.3.4", "fail")
	ObserveHealthcheckDuration("http", "1.2.3.4", 0.123)
	ObserveHealthcheckDuration("http", "1.2.3.4", 0.456)

	count := testutil.ToFloat64(healthcheckTotal.WithLabelValues("http", "1.2.3.4", "success"))
	if count != 1 {
		t.Errorf("expected 1, got %v", count)
	}
	countFail := testutil.ToFloat64(healthcheckTotal.WithLabelValues("http", "1.2.3.4", "fail"))
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
	IncHealthcheckFailures("icmp", "1.2.3.6", "protocol")
	IncHealthcheckFailures("grpc", "1.2.3.7", "other")

	timeoutCount := testutil.ToFloat64(healthcheckFailures.WithLabelValues("http/80", "1.2.3.4", "timeout"))
	if timeoutCount != 2 {
		t.Errorf("expected 2, got %v", timeoutCount)
	}
	connCount := testutil.ToFloat64(healthcheckFailures.WithLabelValues("tcp/443", "1.2.3.5", "connection"))
	if connCount != 1 {
		t.Errorf("expected 1, got %v", connCount)
	}
	protocolCount := testutil.ToFloat64(healthcheckFailures.WithLabelValues("icmp", "1.2.3.6", "protocol"))
	if protocolCount != 1 {
		t.Errorf("expected 1, got %v", protocolCount)
	}
	otherCount := testutil.ToFloat64(healthcheckFailures.WithLabelValues("grpc", "1.2.3.7", "other"))
	if otherCount != 1 {
		t.Errorf("expected 1, got %v", otherCount)
	}
}
