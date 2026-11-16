// Design: docs/plugin-development/metrics.md -- precise retention admission refusal
package sdk

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPublisherSeriesCustodyReportsAdmissionCause drives the public retention
// method through each native refusal and checks useful admitted measurement.
// MUTATION: replacing a cause with a generic refusal loses its distinguishing text.
func TestPublisherSeriesCustodyReportsAdmissionCause(t *testing.T) {
	for _, test := range []struct {
		name   string
		labels []string
		values []string
		want   string
		full   bool
	}{
		{name: "refused declaration", values: []string{"alice"}, want: "needs at least one label name"},
		{name: "label count", labels: []string{"login"}, values: []string{"alice", "extra"}, want: "2 label values; expected 1"},
		{name: "UTF-8", labels: []string{"login"}, values: []string{string([]byte{0xff})}, want: "not valid UTF-8"},
		{name: "label size", labels: []string{"login"}, values: []string{strings.Repeat("a", rpc.MetricLabelValueMax+1)}, want: "exceeds " + strconv.Itoa(rpc.MetricLabelValueMax) + " octets"},
		{name: "series capacity", labels: []string{"login"}, values: []string{"excess"}, want: "holds " + strconv.Itoa(rpc.MetricsSeriesMax) + " labeled series, the limit", full: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			publisher := NewMetricsPublisher()
			const name = "ze_custody_diagnostic"
			vector := publisher.GaugeVec(name, "Admission refusal cause.", test.labels)
			if test.full {
				for index := range rpc.MetricsSeriesMax {
					vector.With("held-" + strconv.Itoa(index)).Set(1)
				}
			}
			publisher.mu.Lock()
			beforeSlots := publisher.series
			beforeRows := len(publisher.byName[name].series)
			publisher.mu.Unlock()
			lease, err := publisherTestSeriesRetainer(t, vector).RetainSeries(test.values...)
			if err == nil {
				t.Fatal("invalid admission acquired a lease")
			}
			lease.Release()
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("admission lost its actual cause: got %q, want %q", err.Error(), test.want)
			}
			publisher.mu.Lock()
			afterSlots := publisher.series
			afterRows := len(publisher.byName[name].series)
			publisher.mu.Unlock()
			if beforeSlots != afterSlots {
				t.Fatal("refusal consumed a series slot", beforeSlots, afterSlots)
			}
			if beforeRows != afterRows {
				t.Fatal("refusal retained a series row", beforeRows, afterRows)
			}

			// Capacity refusal still admits a holder of an existing series.
			// Other refusals cannot prevent an independent valid family.
			positive := vector
			positiveName, login := name, "held-0"
			if !test.full {
				positiveName, login = "ze_custody_diagnostic_positive", "alice"
				positive = publisher.GaugeVec(positiveName, "Useful admitted value.", []string{"login"})
			}
			held := retainPublisherTestSeries(t, positive, login)
			t.Cleanup(held.Release)
			positive.With(login).Set(31)
			publisher.mu.Lock()
			series := publisher.byName[positiveName].series[login]
			var value float64
			if series != nil {
				value = series.value
			}
			publisher.mu.Unlock()
			if series == nil {
				t.Fatal("admitted positive series is absent")
			}
			if value != 31 {
				t.Fatal("admitted measurement was not useful", value)
			}
			held.Release()
			publisher.mu.Lock()
			remaining := publisher.series
			publisher.mu.Unlock()
			if test.full {
				if remaining != rpc.MetricsSeriesMax-1 {
					t.Fatal("existing positive holder did not release its slot", remaining)
				}
			} else if remaining != 0 {
				t.Fatal("positive final release left a series slot", remaining)
			}
		})
	}
}

// TestPublisherSeriesCustodyReturnsRefusalWithoutLogging uses the existing
// nonparallel capture helper, which restores the default logger on cleanup.
// Retention returns a refusal to its owner; ordinary With still warns once.
// MUTATION: logging from RetainSeries violates the zero-record assertion.
func TestPublisherSeriesCustodyReturnsRefusalWithoutLogging(t *testing.T) {
	lines, seen := captureRefusalLines(t)
	publisher := NewMetricsPublisher()
	const name = "ze_custody_returned_refusal"
	vector := publisher.GaugeVec(name, "Returned admission refusal.", []string{"login"})
	retainer := publisherTestSeriesRetainer(t, vector)
	invalid := string([]byte{0xff})
	lease, err := retainer.RetainSeries(invalid)
	if err == nil {
		t.Fatal("invalid retention did not return its refusal")
	}
	lease.Release()
	if count := lines.Load(); count != 0 {
		t.Fatal("retention invoked a logging callback instead of returning its refusal", count)
	}
	if records := seen.all(); len(records) != 0 {
		t.Fatal("retention left a logging record", records)
	}

	vector.With(invalid).Set(1)
	if count := lines.Load(); count != 1 {
		t.Fatal("ordinary With did not preserve its first warning", count)
	}
	records := seen.all()
	if len(records) != 1 {
		t.Fatal("ordinary With did not produce exactly one record", records)
	}
	if records[0].message != "plugin metrics: update not forwarded to the engine" {
		t.Fatal("ordinary With warning message changed", records[0])
	}
	if records[0].metric != name {
		t.Fatal("ordinary With warning lost its metric", records[0])
	}
	if records[0].reason != "a label value is not valid UTF-8 and cannot be published without changing its identity" {
		t.Fatal("ordinary With warning lost its original reason", records[0])
	}
	vector.With(invalid).Set(2)
	if count := lines.Load(); count != 1 {
		t.Fatal("ordinary With repeated its once-per-family warning", count)
	}
	if records := seen.all(); len(records) != 1 {
		t.Fatal("ordinary With repeated its warning record", records)
	}

	held := retainPublisherTestSeries(t, vector, "alice")
	t.Cleanup(held.Release)
	vector.With("alice").Set(31)
	publisher.mu.Lock()
	series := publisher.byName[name].series["alice"]
	var value float64
	if series != nil {
		value = series.value
	}
	publisher.mu.Unlock()
	if series == nil {
		t.Fatal("valid measurement was lost after a refused value")
	}
	if value != 31 {
		t.Fatal("valid measurement changed after a refused value", value)
	}
	if count := lines.Load(); count != 1 {
		t.Fatal("valid retention or measurement added a warning", count)
	}
	held.Release()
}
