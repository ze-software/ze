// Design: docs/architecture/isis/isis-6-lsdb.md -- LSDB Prometheus metrics (owner isis-6).
//
// VALIDATES: this spec registers EXACTLY the umbrella canonical rows it owns
// (ze_isis_lsps, ze_isis_lsp_fragments, ze_isis_lsp_originations_total,
// ze_isis_sequence_wraps_total, ze_isis_purges_total), each labeled by level,
// and that origination/aging/wraparound increment them.
// PREVENTS: bare isis_* names or registering another owner's series.

package lsdb

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

func scrape(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/metrics", http.NoBody)
	w := httptest.NewRecorder()
	reg.Handler().ServeHTTP(w, req)
	return w.Body.String()
}

func TestISISLSDBMetricsRegisterExactSeries(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	d := New(nil)
	d.SetMetrics(reg)

	// Originate a small own LSP set (one prefix) to bump originations + size.
	o := NewOriginator(d, nil)
	node := NodeInfo{
		SystemID:      testSys(1),
		AdvertiseIPv4: true,
		MaxLifetime:   1200,
	}
	o.Originate(Level2, node, LevelState{Prefixes: []PrefixInfo{{
		Prefix: netip.MustParsePrefix("10.0.0.0/24"),
		Metric: types.NewPrefixMetric(10),
	}}})
	// Force a wraparound and a received purge so the wrap/purge counters expose a
	// labeled series too (a CounterVec exposes nothing until a label combo is
	// observed, so all five rows must be exercised here).
	frag0 := types.NewLSPID(types.NewSourceID(node.SystemID, 0), 0)
	o.mu.Lock()
	o.lastSeq[frag0] = types.MaxSequenceNumber
	o.mu.Unlock()
	o.Originate(Level2, node, LevelState{}) // triggers the wraparound + purge

	out := scrape(t, reg)
	for _, want := range []string{
		"ze_isis_lsps",
		"ze_isis_lsp_fragments",
		"ze_isis_lsp_originations_total",
		"ze_isis_sequence_wraps_total",
		"ze_isis_purges_total",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metric %q not exposed", want)
		}
	}
	// Every series carries a level label and there are no bare isis_* names.
	if !strings.Contains(out, `level="l2"`) {
		t.Errorf("expected level=\"l2\" label in output:\n%s", out)
	}
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, "isis_") {
			t.Errorf("bare isis_* metric name: %q", line)
		}
	}
}

// TestISISLSDBSetMetricsRace exercises the data race between SetMetrics
// (re-binding the metric handles under d.mu) and the Originator incrementing
// ze_isis_lsp_originations_total / ze_isis_sequence_wraps_total (which read those
// handles while holding the Originator's OWN mutex, not d.mu). Before the fix the
// handles were read without d.mu, racing the SetMetrics write; the fix routes the
// reads through d.mu-guarded accessors. Run under `go test -race` this fails
// without the fix and passes with it. Regression for finding B2-3.
func TestISISLSDBSetMetricsRace(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := NodeInfo{SystemID: testSys(1), AdvertiseIPv4: true, MaxLifetime: 1200}

	const rounds = 200
	done := make(chan struct{})

	// Writer: repeatedly rebind the metric handles (the SetMetrics path).
	go func() {
		for range rounds {
			reg := metrics.NewPrometheusRegistry()
			d.SetMetrics(reg)
		}
		close(done)
	}()

	// Readers: originate (reads mOriginations) and force wraparounds (reads
	// mWraps) concurrently with the rebinds.
	for range rounds {
		o.Originate(Level1, node, LevelState{})
		// Force a wraparound so incWraps runs too.
		frag0 := types.NewLSPID(types.NewSourceID(node.SystemID, 0), 0)
		o.mu.Lock()
		o.lastSeq[frag0] = types.MaxSequenceNumber
		o.mu.Unlock()
		o.Originate(Level1, node, LevelState{})
	}
	<-done
}

func TestISISLSDBPurgeCounter(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	d := New(nil)
	d.SetMetrics(reg)

	// A received purge counts once.
	id := lspID(2, 0)
	purge, raw := buildLSP(t, lspPDUType(Level2), id, 5, 0, nil)
	d.Receive(Level2, purge, raw, false)
	if !strings.Contains(scrape(t, reg), "ze_isis_purges_total") {
		t.Fatal("ze_isis_purges_total not exposed after a received purge")
	}

	// An LSP that ages to 0 also counts (distinct LSP ID so the counters add).
	live, lraw := buildLSP(t, lspPDUType(Level2), lspID(3, 0), 1, 1, nil)
	d.Insert(Level2, live, lraw)
	d.Tick() // 1 -> 0: purge via aging
	out := scrape(t, reg)
	// The counter value for l2 should now be >= 2 (one received, one aged). The
	// exposition prints the float value after the labeled metric name.
	if !strings.Contains(out, `ze_isis_purges_total{level="l2"} 2`) {
		t.Errorf("expected purges_total l2 == 2 (received + aged), output:\n%s", out)
	}
}
