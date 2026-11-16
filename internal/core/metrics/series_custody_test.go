// Design: docs/architecture/decisions/001-pull-model-metrics.md -- exact shared series lifetime
package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func metricTestSeriesRetainer(t *testing.T, vector any) SeriesRetainer {
	t.Helper()
	retainer, ok := vector.(SeriesRetainer)
	if !ok {
		t.Fatal("metric vector lacks the declared series retention capability")
	}
	return retainer
}

func retainMetricTestSeries(t *testing.T, vector any, labels ...string) SeriesLease {
	t.Helper()
	lease, err := metricTestSeriesRetainer(t, vector).RetainSeries(labels...)
	if err != nil {
		t.Fatal(err)
	}
	return lease
}

func custodyScrape(t *testing.T, reg *PrometheusRegistry) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	reg.Handler().ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), "GET", "/metrics", http.NoBody))
	return recorder.Body.String()
}

// MUTATION: deleting on the first release erases the current holder's value.
func TestPrometheusSeriesCustodyKeepsLastHolder(t *testing.T) {
	for _, kind := range []string{"gauge", "histogram"} {
		t.Run(kind, func(t *testing.T) {
			reg := NewPrometheusRegistry()
			var retainer SeriesRetainer
			var remove func(...string) bool
			var write func()
			if kind == "gauge" {
				vector := reg.GaugeVec("custody_gauge", "Series lifetime.", []string{"login"})
				retainer = metricTestSeriesRetainer(t, vector)
				remove = vector.Delete
				write = func() { vector.With("alice").Set(7) }
			} else {
				vector := reg.HistogramVec("custody_histogram", "Series lifetime.", []float64{1}, []string{"login"})
				retainer = metricTestSeriesRetainer(t, vector)
				remove = vector.Delete
				write = func() { vector.With("alice").Observe(0.5) }
			}
			old := retainMetricTestSeries(t, retainer, "alice")
			current := retainMetricTestSeries(t, retainer, "alice")
			write()
			before := custodyScrape(t, reg)
			if !strings.Contains(before, `login="alice"`) {
				t.Fatal("positive measurement is absent")
			}
			if remove("alice") {
				t.Fatal("ordinary Delete removed an actively retained series")
			}
			oldCopy := old
			old.Release()
			oldCopy.Release()
			if after := custodyScrape(t, reg); after != before {
				t.Fatal("old holder or copied release changed the survivor", after)
			}
			current.Release()
			if strings.Contains(custodyScrape(t, reg), `login="alice"`) {
				t.Fatal("last release left the original series")
			}
			successor := retainMetricTestSeries(t, retainer, "alice")
			write()
			oldCopy.Release()
			if !strings.Contains(custodyScrape(t, reg), `login="alice"`) {
				t.Fatal("ended lease removed a recreated series")
			}
			successor.Release()
			if strings.Contains(custodyScrape(t, reg), `login="alice"`) {
				t.Fatal("successor's last release failed")
			}
		})
	}
}

// MUTATION: a registry-global/name-global custody map can delete foreign series.
func TestPrometheusSeriesCustodySeparatesRegistries(t *testing.T) {
	oldRegistry, currentRegistry := NewPrometheusRegistry(), NewPrometheusRegistry()
	oldVector := oldRegistry.GaugeVec("custody_registry", "Original registry.", []string{"login"})
	currentVector := currentRegistry.GaugeVec("custody_registry", "Original registry.", []string{"login"})
	old := retainMetricTestSeries(t, oldVector, "alice")
	current := retainMetricTestSeries(t, currentVector, "alice")
	// Re-registering a name in one registry reaches its same physical vector.
	same := retainMetricTestSeries(t, oldRegistry.GaugeVec("custody_registry", "Original registry.", []string{"login"}), "alice")
	oldVector.With("alice").Set(11)
	currentVector.With("alice").Set(29)
	old.Release()
	if !strings.Contains(custodyScrape(t, oldRegistry), `custody_registry{login="alice"} 11`) {
		t.Fatal("same-registration holder was lost")
	}
	same.Release()
	if strings.Contains(custodyScrape(t, oldRegistry), `login="alice"`) {
		t.Fatal("old registration was not cleaned")
	}
	if !strings.Contains(custodyScrape(t, currentRegistry), `custody_registry{login="alice"} 29`) {
		t.Fatal("old release erased the replacement registry")
	}
	current.Release()
	if strings.Contains(custodyScrape(t, currentRegistry), `login="alice"`) {
		t.Fatal("current registration was not cleaned")
	}
}

func TestSeriesCustodyRetainsExactLabelsAndNoReleasedEntries(t *testing.T) {
	var custody SeriesCustody
	labels := []string{"a", "b:c"}
	removed := make(map[string]int)
	remove := func(values ...string) bool { removed[seriesLabelsKey(values)]++; return true }
	firstKey := seriesLabelsKey(labels)
	first, err := custody.Retain(labels, nil, remove)
	if err != nil {
		t.Fatal(err)
	}
	labels[0] = "changed"
	secondLabels := []string{"a:b", "c"}
	secondKey := seriesLabelsKey(secondLabels)
	second, err := custody.Retain(secondLabels, nil, remove)
	if err != nil {
		t.Fatal(err)
	}
	if firstKey == secondKey {
		t.Fatal("different label components alias")
	}
	first.Release()
	if removed[firstKey] != 1 || removed[secondKey] != 0 {
		t.Fatal("release changed borrowed or foreign labels", removed)
	}
	second.Release()
	if removed[secondKey] != 1 || len(custody.interests) != 0 {
		t.Fatal("last release retained custody history", removed, len(custody.interests))
	}
	SeriesLease{}.Release()
}

func TestNopVectorsExplicitlySupportSeriesCustody(t *testing.T) {
	reg := NopRegistry{}
	for _, vector := range []any{reg.GaugeVec("g", "", []string{"login"}), reg.HistogramVec("h", "", nil, []string{"login"})} {
		retainer, ok := vector.(SeriesRetainer)
		if !ok {
			t.Fatal("disabled metrics lacks the declared capability")
		}
		lease := retainMetricTestSeries(t, retainer, "alice")
		lease.Release()
		lease.Release()
	}
}
