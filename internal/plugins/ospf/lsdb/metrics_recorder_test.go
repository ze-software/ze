// VALIDATES: the LSDB size-gauge publishing path in lsdb.go: SetMetrics registers the
// ze_ospf_lsdb_lsas gauge and, via publishAllSizeMetrics -> publishSizeMetric, sets the
// per-area and AS-wide series to the current LSA population counts.
// PREVENTS: a metrics wiring that registers the gauge but never seeds it, or that
// mislabels the AS-External count under an area label instead of "as".
package lsdb

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// gaugeStore records the last Set value for each (metric name, label values) series.
type gaugeStore struct {
	mu     sync.Mutex
	values map[string]float64
}

func (s *gaugeStore) set(key string, v float64) {
	s.mu.Lock()
	s.values[key] = v
	s.mu.Unlock()
}

func (s *gaugeStore) get(key string) (float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.values[key]
	return v, ok
}

// recordingRegistry embeds NopRegistry (so every non-gauge metric is a no-op) and records
// only the GaugeVec series the LSDB size gauge publishes.
type recordingRegistry struct {
	metrics.NopRegistry
	store *gaugeStore
}

func (r recordingRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	return &recordingGaugeVec{store: r.store, name: name}
}

type recordingGaugeVec struct {
	store *gaugeStore
	name  string
}

func (v *recordingGaugeVec) With(labels ...string) metrics.Gauge {
	return &recordingGauge{store: v.store, key: v.name + "|" + strings.Join(labels, ",")}
}
func (v *recordingGaugeVec) Delete(...string) bool { return false }

type recordingGauge struct {
	store *gaugeStore
	key   string
}

func (g *recordingGauge) Set(val float64) { g.store.set(g.key, val) }
func (g *recordingGauge) Inc()            { g.store.set(g.key, 0) }
func (g *recordingGauge) Dec()            { g.store.set(g.key, 0) }
func (g *recordingGauge) Add(float64)     {}

func TestSetMetricsSeedsSizeGauges(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	if !db.Install(a0, routerLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber, 10)) {
		t.Fatalf("install router LSA rejected")
	}
	if !db.Install(a0, externalLSA(t, rid("3.3.3.3"), types.InitialSequenceNumber)) {
		t.Fatalf("install external LSA rejected")
	}

	store := &gaugeStore{values: map[string]float64{}}
	db.SetMetrics(recordingRegistry{store: store})

	const gauge = "ze_ospf_lsdb_lsas"
	routerKey := gauge + "|" + a0.String() + "," + types.LSTypeRouter.String()
	if v, ok := store.get(routerKey); !ok || v != 1 {
		t.Fatalf("router size gauge %q = %v (present=%v), want 1", routerKey, v, ok)
	}
	// The Type 5 AS-External is counted under the "as" label, not an area label.
	asKey := gauge + "|as," + types.LSTypeASExternal.String()
	if v, ok := store.get(asKey); !ok || v != 1 {
		t.Fatalf("as-external size gauge %q = %v (present=%v), want 1", asKey, v, ok)
	}
	// It must NOT have been published under the installed-against area label.
	if _, ok := store.get(gauge + "|" + a0.String() + "," + types.LSTypeASExternal.String()); ok {
		t.Fatalf("AS-External size gauge mislabelled under an area label")
	}
}
