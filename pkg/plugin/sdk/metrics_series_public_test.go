// Design: docs/plugin-development/metrics.md -- public series retention contract
// Related: metrics_series.go -- canonical contract exposed by the SDK
package sdk_test

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestPublisherSeriesCustodyPublicContract uses only the public SDK to acquire
// and release gauge and histogram holders, including copies and successors.
// Measurement and transport assertions belong to the native publisher tests.
func TestPublisherSeriesCustodyPublicContract(t *testing.T) {
	for _, kind := range []string{"gauge", "histogram"} {
		t.Run(kind, func(t *testing.T) {
			publisher := sdk.NewMetricsPublisher()
			var vector any
			var update func(float64)
			var remove func(...string) bool
			if kind == "gauge" {
				gauge := publisher.GaugeVec("ze_public_custody_gauge", "Public holders.", []string{"login"})
				vector = gauge
				update = func(value float64) { gauge.With("alice").Set(value) }
				remove = gauge.Delete
			} else {
				histogram := publisher.HistogramVec("ze_public_custody_histogram", "Public holders.", []float64{1}, []string{"login"})
				vector = histogram
				update = func(value float64) { histogram.With("alice").Observe(value) }
				remove = histogram.Delete
			}
			retainer, ok := vector.(sdk.SeriesRetainer)
			if !ok {
				t.Fatal("SDK vector does not expose sdk.SeriesRetainer")
			}
			var invalid sdk.SeriesLease
			invalid, err := retainer.RetainSeries("alice", "extra")
			if err == nil {
				t.Fatal("public retention accepted the wrong label count")
			}
			invalid.Release()

			first, err := retainer.RetainSeries("alice")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(first.Release)
			copied := first
			update(0.25)
			if remove("alice") {
				t.Fatal("Delete removed a series with its first public holder")
			}
			second, err := retainer.RetainSeries("alice")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(second.Release)
			first.Release()
			copied.Release()
			update(0.5)
			if remove("alice") {
				t.Fatal("copied release removed a series with another public holder")
			}
			second.Release()
			if remove("alice") {
				t.Fatal("final public release left its series present")
			}

			successor, err := retainer.RetainSeries("alice")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(successor.Release)
			update(0.75)
			copied.Release()
			second.Release()
			if remove("alice") {
				t.Fatal("Delete removed a retained successor")
			}
			successor.Release()
			if remove("alice") {
				t.Fatal("successor release left its series present")
			}
			update(1.25)
			if !remove("alice") {
				t.Fatal("unretained public series could not be recreated and deleted")
			}
		})
	}
}
