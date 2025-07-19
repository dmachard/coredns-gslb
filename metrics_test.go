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
