package filter_modify

import (
	"testing"

	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// TestModifySetsIGPExportMetric: a modify instance at an IGP export sets the
// advertised metric and the OSPF metric type, and refuses a metric the OSPF
// AS-external LSA cannot carry or a metric type OSPF does not define. Method:
// compile the route evaluator of each instance and read the sets it answers.
func TestModifySetsIGPExportMetric(t *testing.T) {
	evaluate, err := compileRouteEvaluator("export-e1", map[string]any{
		"set": map[string]any{"metric": "50", "metric-type": "type-1"},
	}, nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	sets := evaluate(ribpolicy.Route{}).Sets
	if !sets.HasMetric || sets.Metric != 50 {
		t.Errorf("metric = %d (set %v), want 50", sets.Metric, sets.HasMetric)
	}
	if sets.MetricType != ribpolicy.ExternalMetricType1 {
		t.Errorf("metric-type = %d, want type-1", sets.MetricType)
	}

	evaluate, err = compileRouteEvaluator("export-e2", map[string]any{"set": map[string]any{"metric-type": "type-2"}}, nil)
	if err != nil {
		t.Fatalf("compile type-2: %v", err)
	}
	if got := evaluate(ribpolicy.Route{}).Sets; got.MetricType != ribpolicy.ExternalMetricType2 || got.HasMetric {
		t.Errorf("type-2 only: sets %+v, want metric-type 2 and no metric", got)
	}

	for name, set := range map[string]map[string]any{
		"metric above 24 bits": {"metric": "16777216"},
		"metric not a number":  {"metric": "fifty"},
		"unknown metric type":  {"metric-type": "type-3"},
	} {
		if _, err := compileRouteEvaluator(name, map[string]any{"set": set}, nil); err == nil {
			t.Errorf("%s: compiled, want a refusal", name)
		}
	}
}
