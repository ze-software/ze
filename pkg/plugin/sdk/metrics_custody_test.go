// Design: docs/plugin-development/metrics.md -- original publisher family custody
package sdk

import (
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func publisherTestSeriesRetainer(t *testing.T, vector any) metrics.SeriesRetainer {
	t.Helper()
	retainer, ok := vector.(metrics.SeriesRetainer)
	if !ok {
		t.Fatal("publisher vector lacks the declared series retention capability")
	}
	return retainer
}

func retainPublisherTestSeries(t *testing.T, vector any, labels ...string) metrics.SeriesLease {
	t.Helper()
	lease, err := publisherTestSeriesRetainer(t, vector).RetainSeries(labels...)
	if err != nil {
		t.Fatal(err)
	}
	return lease
}

func TestPublisherSeriesCustodyUsesOriginalFamily(t *testing.T) {
	for _, kind := range []string{"gauge", "histogram"} {
		t.Run(kind, func(t *testing.T) {
			publisher := NewMetricsPublisher()
			var first, second metrics.SeriesRetainer
			var remove func(...string) bool
			if kind == "gauge" {
				a := publisher.GaugeVec("ze_custody_gauge", "Family custody.", []string{"login"})
				b := publisher.GaugeVec("ze_custody_gauge", "Family custody.", []string{"login"})
				first, second = publisherTestSeriesRetainer(t, a), publisherTestSeriesRetainer(t, b)
				remove = a.Delete
				a.With("alice").Set(17)
			} else {
				a := publisher.HistogramVec("ze_custody_histogram", "Family custody.", []float64{1}, []string{"login"})
				b := publisher.HistogramVec("ze_custody_histogram", "Family custody.", []float64{1}, []string{"login"})
				first, second = publisherTestSeriesRetainer(t, a), publisherTestSeriesRetainer(t, b)
				remove = a.Delete
				a.With("alice").Observe(0.5)
			}
			old := retainPublisherTestSeries(t, first, "alice")
			current := retainPublisherTestSeries(t, second, "alice")
			if remove("alice") {
				t.Fatal("Delete removed a retained publisher series")
			}
			old.Release()
			old.Release()
			if remove("alice") {
				t.Fatal("different wrapper did not share original family custody")
			}
			publisher.mu.Lock()
			var live int
			for _, family := range publisher.byName {
				live += len(family.series)
			}
			publisher.mu.Unlock()
			if live != 1 {
				t.Fatal("survivor has no useful original measurement", live)
			}
			current.Release()
			publisher.mu.Lock()
			live = 0
			for _, family := range publisher.byName {
				live += len(family.series)
			}
			publisher.mu.Unlock()
			if live != 0 {
				t.Fatal("last publisher holder left its series", live)
			}
		})
	}
}

func TestPublisherSeriesCustodyRefusesInvalidLabelBytes(t *testing.T) {
	publisher := NewMetricsPublisher()
	vector := publisher.GaugeVec("ze_custody_valid", "Label custody.", []string{"login"})
	retainer := publisherTestSeriesRetainer(t, vector)
	invalid, err := retainer.RetainSeries(string([]byte{0xff}))
	if err == nil {
		t.Fatal("invalid label bytes admitted a lease")
	}
	invalid.Release()
	valid := retainPublisherTestSeries(t, retainer, "alice")
	vector.With("alice").Set(31)
	publisher.mu.Lock()
	family := publisher.byName["ze_custody_valid"]
	live := len(family.series)
	publisher.mu.Unlock()
	if live != 1 {
		t.Fatal("invalid bytes created a series or valid value was refused", live)
	}
	if vector.Delete("alice") {
		t.Fatal("valid positive holder was not retained")
	}
	valid.Release()
	if vector.Delete("alice") {
		t.Fatal("final release did not remove its valid series")
	}
}

func TestPublisherSeriesCustodyZeroLabelsKeepSeriesLimit(t *testing.T) {
	publisher := NewMetricsPublisher()
	vector := publisher.GaugeVec("ze_custody_unlabeled", "Unlabeled vector.", nil)
	lease, err := publisherTestSeriesRetainer(t, vector).RetainSeries()
	if err == nil {
		t.Fatal("zero-label vector bypassed the native declaration refusal")
	}
	lease.Release()
	vector.With().Set(7)
	if vector.Delete() {
		t.Fatal("refused zero-label vector created a deletable series")
	}
	publisher.mu.Lock()
	remaining := publisher.series
	familyRows := len(publisher.byName["ze_custody_unlabeled"].series)
	publisher.mu.Unlock()
	if remaining != 0 {
		t.Fatal("refused vector changed the labeled-series limit", remaining)
	}
	if familyRows != 0 {
		t.Fatal("refused vector retained a series", familyRows)
	}
	labeled := publisher.GaugeVec("ze_custody_labeled", "Positive labeled vector.", []string{"login"})
	held := retainPublisherTestSeries(t, labeled, "alice")
	labeled.With("alice").Set(11)
	publisher.mu.Lock()
	remaining = publisher.series
	publisher.mu.Unlock()
	if remaining != 1 {
		t.Fatal("labeled positive control did not reserve one series", remaining)
	}
	held.Release()
	publisher.mu.Lock()
	remaining = publisher.series
	publisher.mu.Unlock()
	if remaining != 0 {
		t.Fatal("labeled last release lost its series limit", remaining)
	}
}

// The same producer that bounds ordinary With also bounds retained reservations.
func TestPublisherSeriesCustodyUsesNativeAdmissionLimit(t *testing.T) {
	publisher := NewMetricsPublisher()
	vector := publisher.GaugeVec("ze_custody_limit", "Bounded family.", []string{"login"})
	for i := range rpc.MetricsSeriesMax {
		vector.With("held-" + strconv.Itoa(i)).Set(1)
	}
	retainer := publisherTestSeriesRetainer(t, vector)
	refused, err := retainer.RetainSeries("excess")
	if err == nil {
		t.Fatal("custody bypassed actual native series limit")
	}
	refused.Release()
	// Existing admitted labels remain useful when the limit is full.
	existing := retainPublisherTestSeries(t, retainer, "held-0")
	existing.Release()
	replacement := retainPublisherTestSeries(t, retainer, "replacement")
	publisher.mu.Lock()
	family := publisher.byName["ze_custody_limit"]
	reserved := family.series["replacement"]
	count := publisher.series
	visible := reserved.dirty
	pending := reserved.awaitingHandle
	publisher.mu.Unlock()
	if count != rpc.MetricsSeriesMax || visible || !pending {
		t.Fatal("reservation was unbounded or published a guessed zero", count, visible, pending)
	}
	vector.With("replacement").Set(7)
	publisher.mu.Lock()
	visible = reserved.dirty
	pending = reserved.awaitingHandle
	publisher.mu.Unlock()
	if !visible || pending {
		t.Fatal("ordinary useful update did not publish its admitted series")
	}
	replacement.Release()
}
