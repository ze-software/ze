// VALIDATES: AC-1 the anomaly event contract -- a distinct "anomaly-detect"
// namespace with source/entity-oriented events, and GradeSeverity's 1x/2x/5x
// tiering of an incident score against the emit threshold.
// PREVENTS: collision with the ddos-detect namespace, and a severity misgrade at
// the tier boundaries or on a non-positive threshold.

package anomalyevent

import (
	"net/netip"
	"testing"
)

func TestAnomalyEventRegisterAndGrade(t *testing.T) {
	if Namespace != "anomaly-detect" {
		t.Errorf("namespace = %q, want anomaly-detect (distinct from ddos-detect)", Namespace)
	}
	if Detected == nil || Ongoing == nil || Cleared == nil {
		t.Fatal("event handles must be registered at init")
	}

	cases := []struct {
		score, threshold float64
		want             Severity
	}{
		{0.5, 1, SeverityMedium},
		{1, 1, SeverityMedium},
		{1.9, 1, SeverityMedium},
		{2, 1, SeverityHigh},
		{4.9, 1, SeverityHigh},
		{5, 1, SeverityCritical},
		{100, 1, SeverityCritical},
		{30, 0, SeverityMedium}, // non-positive threshold -> medium, no divide-by-zero
	}
	for _, c := range cases {
		if got := GradeSeverity(c.score, c.threshold); got != c.want {
			t.Errorf("GradeSeverity(%v, %v) = %q, want %q", c.score, c.threshold, got, c.want)
		}
	}

	ev := &AnomalyDetected{
		Entity:        netip.MustParsePrefix("198.51.100.0/24"),
		FiredFeatures: []FeatureSignal{{Name: "out-in-ratio", Z: 6.2}},
		Score:         6.2,
		Severity:      SeverityCritical,
	}
	if !ev.Entity.IsValid() {
		t.Error("AnomalyDetected.Entity must be a valid source prefix")
	}
	if ev.Score != 6.2 || len(ev.FiredFeatures) != 1 || ev.Severity != SeverityCritical {
		t.Errorf("AnomalyDetected fields not carried: %+v", ev)
	}
}
