// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- the canonical ze_isis_* metric
// set assertion (AC-10). isis-13 registers NO metric series itself; it asserts
// that the full umbrella "Metrics (canonical)" table is registered by its owning
// subsystems (isis-3 transport, isis-5/6/7/8/9/10/11 engine subsystems) with the
// exact series names and labels, and that no bare isis_* name is used.
// Related: server.go -- engine.setMetrics wires the adjacency/dis/auth/lsdb/
//   flooder/spf series; transport.SetMetrics the frame series; consumer.SetMetrics
//   the redist series.
//
// VALIDATES: every row of the umbrella Metrics table is registered with its exact
// name and label set when the engine + transport + redistribution consumer are
// wired with a real registry; all series are ze_isis_*; isis-13 owns none.
// PREVENTS: a renamed/relabelled series, a bare isis_* name, or a canonical
// series that silently stops being registered after a refactor.

package isis

import (
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	isisredistribute "github.com/ze-software/ze/internal/plugins/isis/redistribute"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// recordingRegistry captures the name and label set of every metric registered
// through it, delegating the actual metric objects to NopRegistry (so With/Inc
// are harmless no-ops). It records the full Registry surface so the test can
// assert the exact canonical set, independent of whether a series ever takes a
// sample (a Prometheus scrape would only show series with children).
type recordingRegistry struct {
	metrics.NopRegistry
	labels map[string][]string // series name -> sorted label names ("" for none)
}

func newRecordingRegistry() *recordingRegistry {
	return &recordingRegistry{labels: make(map[string][]string)}
}

func (r *recordingRegistry) record(name string, labelNames []string) {
	ls := append([]string(nil), labelNames...)
	sort.Strings(ls)
	r.labels[name] = ls
}

func (r *recordingRegistry) Counter(name, help string) metrics.Counter {
	r.record(name, nil)
	return r.NopRegistry.Counter(name, help)
}

func (r *recordingRegistry) Gauge(name, help string) metrics.Gauge {
	r.record(name, nil)
	return r.NopRegistry.Gauge(name, help)
}

func (r *recordingRegistry) CounterVec(name, help string, labelNames []string) metrics.CounterVec {
	r.record(name, labelNames)
	return r.NopRegistry.CounterVec(name, help, labelNames)
}

func (r *recordingRegistry) GaugeVec(name, help string, labelNames []string) metrics.GaugeVec {
	r.record(name, labelNames)
	return r.NopRegistry.GaugeVec(name, help, labelNames)
}

func (r *recordingRegistry) Histogram(name, help string, buckets []float64) metrics.Histogram {
	r.record(name, nil)
	return r.NopRegistry.Histogram(name, help, buckets)
}

func (r *recordingRegistry) HistogramVec(name, help string, buckets []float64, labelNames []string) metrics.HistogramVec {
	r.record(name, labelNames)
	return r.NopRegistry.HistogramVec(name, help, buckets, labelNames)
}

// wireAllMetrics mirrors runISISEngine's metric wiring: it forwards a registry
// to every IS-IS subsystem that OWNS a canonical series. isis-13 owns none; this
// helper just drives the owners so the test can scrape the full set.
func wireAllMetrics(reg metrics.Registry) {
	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	eng.transport.SetMetrics(reg) // isis-3: frames + sockets
	eng.setMetrics(reg)           // isis-5/6/7/8/9/10/11 engine-side series
	consumer := isisredistribute.NewConsumer(eng)
	consumer.SetMetrics(reg) // isis-11: redist series
}

// TestISISMetricsRegistered asserts the full umbrella "Metrics (canonical)" set
// is registered with the exact name and label set, none bare isis_*.
func TestISISMetricsRegistered(t *testing.T) {
	reg := newRecordingRegistry()
	wireAllMetrics(reg)

	// The single source of truth: the umbrella Metrics-table rows. Label sets are
	// sorted for comparison (registration order is irrelevant).
	want := map[string][]string{
		// isis-3 transport
		"ze_isis_frames_sent_total":     {"interface"},
		"ze_isis_frames_received_total": {"interface"},
		"ze_isis_frames_dropped_total":  {"interface", "reason"},
		"ze_isis_sockets_open":          {},
		// isis-5 adjacency
		"ze_isis_adjacencies_up":    {"interface", "level"},
		"ze_isis_adjacencies_total": {"level"},
		// isis-6 lsdb
		"ze_isis_lsps":                   {"level"},
		"ze_isis_lsp_fragments":          {"level"},
		"ze_isis_lsp_originations_total": {"level"},
		"ze_isis_sequence_wraps_total":   {"level"},
		"ze_isis_purges_total":           {"level"},
		// isis-7 flooding
		"ze_isis_lsps_received_total":    {"level"},
		"ze_isis_lsps_transmitted_total": {"level"},
		"ze_isis_csnp_sent_total":        {"level"},
		"ze_isis_csnp_received_total":    {"level"},
		"ze_isis_psnp_sent_total":        {"level"},
		"ze_isis_psnp_received_total":    {"level"},
		"ze_isis_srm_resends_total":      {"level"},
		"ze_isis_lsps_dropped_total":     {"level", "reason"},
		// isis-8 dis
		"ze_isis_dis_elections_total": {"level"},
		"ze_isis_pseudonode_lsps":     {"level"},
		// isis-9 spf
		"ze_isis_spf_runs_total":       {"level"},
		"ze_isis_spf_duration_seconds": {"level"},
		"ze_isis_spf_nodes":            {"level"},
		"ze_isis_routes_installed":     {"afi", "level"},
		// isis-10 auth
		"ze_isis_auth_failures_total": {"interface", "level"},
		// isis-11 redistribution
		"ze_isis_redist_injected_total":        {"afi", "source"},
		"ze_isis_redist_withdrawn_total":       {"afi", "source"},
		"ze_isis_redist_inject_failures_total": {"source"},
		"ze_isis_lsp_reoriginations_total":     {"level"},
	}

	for name, wantLabels := range want {
		got, ok := reg.labels[name]
		if !ok {
			t.Errorf("canonical series %q not registered", name)
			continue
		}
		if !equalStrings(got, wantLabels) {
			t.Errorf("series %q labels = %v, want %v", name, got, wantLabels)
		}
	}

	// No bare isis_* names: every registered series must be ze_isis_*.
	for name := range reg.labels {
		if strings.HasPrefix(name, "isis_") {
			t.Errorf("bare isis_* series %q registered; must be ze_isis_*", name)
		}
		if strings.HasPrefix(name, "ze_isis_") {
			if _, expected := want[name]; !expected {
				t.Errorf("unexpected ze_isis_* series %q registered (not in the canonical table)", name)
			}
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
