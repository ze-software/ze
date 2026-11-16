// Design: docs/architecture/decisions/001-pull-model-metrics.md -- native series admission
package metrics

import (
	"strings"
	"testing"
)

// Retention refuses labels that native With cannot accept, without creating a
// sample; a valid Unicode measurement still publishes and its last lease cleans up.
func TestPrometheusSeriesCustodyRefusesInvalidLabels(t *testing.T) {
	for _, kind := range []string{"gauge", "histogram"} {
		t.Run(kind, func(t *testing.T) {
			for _, refusal := range []struct {
				name   string
				labels []string
				cause  string
			}{
				{name: "invalid-utf8", labels: []string{string([]byte{0xff})}, cause: "UTF-8"},
				{name: "wrong-count", labels: []string{"alice", "extra"}, cause: "declared label count"},
			} {
				t.Run(refusal.name, func(t *testing.T) {
					reg := NewPrometheusRegistry()
					var retainer SeriesRetainer
					var write func()
					var measurement string
					if kind == "gauge" {
						vector := reg.GaugeVec("custody_validation_gauge", "Native admission.", []string{"login"})
						retainer = metricTestSeriesRetainer(t, vector)
						write = func() { vector.With("alice✓").Set(31) }
						measurement = `custody_validation_gauge{login="alice✓"} 31`
					} else {
						vector := reg.HistogramVec("custody_validation_histogram", "Native admission.", []float64{1}, []string{"login"})
						retainer = metricTestSeriesRetainer(t, vector)
						write = func() { vector.With("alice✓").Observe(0.5) }
						measurement = `custody_validation_histogram_sum{login="alice✓"} 0.5`
					}
					refused, err := retainer.RetainSeries(refusal.labels...)
					t.Cleanup(refused.Release)
					if err == nil {
						t.Fatal("native-incompatible labels acquired a series lease")
					}
					if !strings.Contains(err.Error(), refusal.cause) {
						t.Fatalf("refusal lost its actual cause: %v", err)
					}
					if refused.state != nil {
						t.Fatal("refused retention returned a nonzero lease")
					}
					refused.Release()
					refused.Release()
					if scrape := custodyScrape(t, reg); strings.Contains(scrape, "custody_validation_") {
						t.Fatal("refused retention or zero release exported a series", scrape)
					}
					valid := retainMetricTestSeries(t, retainer, "alice✓")
					t.Cleanup(valid.Release)
					if scrape := custodyScrape(t, reg); strings.Contains(scrape, "custody_validation_") {
						t.Fatal("retention exported a guessed value before measurement", scrape)
					}
					write()
					refused.Release()
					if scrape := custodyScrape(t, reg); !strings.Contains(scrape, measurement) {
						t.Fatal("valid native measurement was lost after refusal or zero release", scrape)
					}
					valid.Release()
					if scrape := custodyScrape(t, reg); strings.Contains(scrape, "custody_validation_") {
						t.Fatal("last release left the valid native series", scrape)
					}
				})
			}
		})
	}
}
