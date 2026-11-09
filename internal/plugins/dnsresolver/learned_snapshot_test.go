package dnsresolver

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// countingCounter is a metrics.Counter that remembers how often it stepped.
type countingCounter struct{ steps float64 }

func (c *countingCounter) Inc()          { c.steps++ }
func (c *countingCounter) Add(v float64) { c.steps += v }

// countingRegistry is a metrics registry whose every counter vector answers
// one countingCounter, so a test reads what a series stepped.
type countingRegistry struct {
	metrics.NopRegistry
	counter *countingCounter
}

func (r countingRegistry) CounterVec(_, _ string, _ []string) metrics.CounterVec {
	return countingVec{counter: r.counter}
}

type countingVec struct{ counter *countingCounter }

func (v countingVec) With(_ ...string) metrics.Counter { return v.counter }
func (v countingVec) Delete(_ ...string) bool          { return false }

// TestLearnedSnapshotBatchKeepsLaterSnapshot proves a system-dns snapshot the
// LAN resolver cannot apply does not cost it the valid snapshot delivered
// after it in the same batch. Method: the handler followLearnedResolvers
// installs is fed a batch of [undecodable body, valid snapshot] the way the
// SDK's batch loop feeds it (pkg/plugin/sdk OnEvent and activateBridge),
// which moves to the next event only on a nil answer; the test then reads
// the forward list the query path uses and the rejection counter.
//
// VALIDATES: spec-home-cpe-23b, a rejected snapshot is logged at WARN and
// counted, never returned to the SDK. PREVENTS: a bad snapshot ending the
// batch, so the resolver forwards to stale learned servers until the next
// lease change.
func TestLearnedSnapshotBatchKeepsLaterSnapshot(t *testing.T) {
	cfg := testConfig()
	res := newTestResolver(t, cfg, newTestClock())
	rejected := &countingCounter{}
	handler := learnedSnapshotHandler(res, func() metrics.Counter { return rejected }, slogutil.DiscardLogger())

	batch := []string{"{not a snapshot", learnedSnapshot(t, 1, []string{"192.0.2.53"})}
	for i, event := range batch {
		if err := handler(event); err != nil {
			t.Fatalf("event %d answered %v: the SDK's batch loop stops there and drops every later snapshot", i, err)
		}
	}

	if rejected.steps != 1 {
		t.Fatalf("rejection counter stepped %v times, want 1", rejected.steps)
	}
	want := []upstream{{IP: netip.MustParseAddr("192.0.2.53"), Port: 53}}
	if got := publishedForward(t, res); !slices.Equal(got, want) {
		t.Fatalf("forward = %v after the batch, want %v from the valid snapshot", got, want)
	}
}

// TestLearnedRejectedCountedAfterLateMetrics proves a snapshot rejected after
// the engine's metrics registry arrived is counted in that registry, even
// though the handler was built before it. Method: build the handler with the
// provider followLearnedResolvers passes while only the no-op metric set
// exists, as at plugin start, then call setMetricsRegistry with a counting
// registry, as the registration's deferred ConfigureMetrics does, and feed an
// undecodable snapshot.
//
// VALIDATES: spec-home-cpe-23b, ze_dns_source_snapshots_rejected_total counts
// the resolver's rejections. PREVENTS: the handler keeping the no-op counter
// rmetrics answered at start, so the exported series never moves.
func TestLearnedRejectedCountedAfterLateMetrics(t *testing.T) {
	setMetricsRegistry(metrics.NopRegistry{})
	t.Cleanup(func() { setMetricsRegistry(metrics.NopRegistry{}) })

	res := newTestResolver(t, testConfig(), newTestClock())
	handler := learnedSnapshotHandler(res, learnedRejectedCounter, slogutil.DiscardLogger())

	counted := &countingCounter{}
	setMetricsRegistry(countingRegistry{counter: counted})

	if err := handler("{not a snapshot"); err != nil {
		t.Fatalf("rejected snapshot answered %v, want nil", err)
	}
	if counted.steps != 1 {
		t.Fatalf("the registry set after the handler was built counted %v rejections, want 1", counted.steps)
	}
}
