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
