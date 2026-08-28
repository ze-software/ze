// VALIDATES: spec-anomaly-6-as-enrichment AC-1 and AC-3 -- the flowexport
// producer stamps the origin AS it already holds onto every observation it
// publishes, and leaves the 0 sentinel when no enricher is attached.
// PREVENTS: a consumer in another plugin importing flowexport/enrich to fetch
// the AS itself, which fails `./le tier check`.
//
// These tests live beside exporter_test.go rather than inside it because that
// file carries RFC-tagged NetFlow and sFlow proofs; a new file keeps this
// spec's additions clear of them.

package flowexport

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/observation"
	"github.com/ze-software/ze/internal/plugins/flowexport/enrich"
)

// collectFlowObs subscribes to the process-wide observation feed and returns a
// reader for the flow observations whose source address is one of want. The
// feed is shared by every publisher in the process, so filtering on the source
// address keeps one test from reading another test's flows. The reader waits
// until it holds one observation per wanted address, or two seconds elapse.
func collectFlowObs(t *testing.T, want ...netip.Addr) func() []observation.Observation {
	t.Helper()
	feed := observation.Global()
	var mu sync.Mutex
	var got []observation.Observation
	subID := feed.Subscribe("srcas-test", func(obs observation.Observation) {
		if obs.Kind != observation.KindFlow {
			return
		}
		for _, w := range want {
			if obs.Flow.Src != w {
				continue
			}
			mu.Lock()
			got = append(got, obs)
			mu.Unlock()
			return
		}
	})
	t.Cleanup(func() { feed.Unsubscribe(subID) })

	return func() []observation.Observation {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			n := len(got)
			mu.Unlock()
			if n >= len(want) {
				break
			}
			time.Sleep(time.Millisecond)
		}
		mu.Lock()
		defer mu.Unlock()
		out := make([]observation.Observation, len(got))
		copy(out, got)
		return out
	}
}

// VALIDATES: child-6 AC-1 -- exportFlows copies the origin AS the enricher
// already put on the flow onto the observation it publishes, so the neutral
// facts surface carries it with no second lookup. 4294967295 is the top of the
// uint32 range (boundary row of the spec's Boundary Tests table).
func TestExportFlowsStampsSrcAS(t *testing.T) {
	exp := newTestExporter(t, "netflow9")

	tree := enrich.NewRadixTree()
	tree.Insert(netip.MustParsePrefix("192.0.2.0/24"), enrich.ASEntry{AS: 64500})
	tree.Insert(netip.MustParsePrefix("198.51.100.0/24"), enrich.ASEntry{AS: 4294967295})
	en := enrich.NewEnricher()
	en.UpdateTree(tree)
	exp.setEnricher(en)

	low := netip.MustParseAddr("192.0.2.5")
	high := netip.MustParseAddr("198.51.100.7")
	read := collectFlowObs(t, low, high)

	dst := netip.MustParseAddr("203.0.113.9")
	exp.exportFlows([]ConntrackFlow{
		{SrcAddr: low, DstAddr: dst, Protocol: 6, Bytes: 100},
		{SrcAddr: high, DstAddr: dst, Protocol: 6, Bytes: 200},
	})

	want := map[netip.Addr]uint32{low: 64500, high: 4294967295}
	got := read()
	if len(got) != 2 {
		t.Fatalf("received %d flow observations, want 2", len(got))
	}
	for _, obs := range got {
		if obs.SrcAS != want[obs.Flow.Src] {
			t.Errorf("observation for %v: SrcAS = %d, want %d", obs.Flow.Src, obs.SrcAS, want[obs.Flow.Src])
		}
	}
}

// VALIDATES: child-6 AC-3 -- with no enricher attached the flow carries no AS,
// so the published observation carries the 0 sentinel and a consumer degrades
// to prefix cohorts.
func TestExportFlowsSrcASZeroWhenNoEnricher(t *testing.T) {
	exp := newTestExporter(t, "netflow9") // no setEnricher call

	src := netip.MustParseAddr("192.0.2.77")
	read := collectFlowObs(t, src)

	exp.exportFlows([]ConntrackFlow{
		{SrcAddr: src, DstAddr: netip.MustParseAddr("203.0.113.9"), Protocol: 6, Bytes: 100},
	})

	got := read()
	if len(got) != 1 {
		t.Fatalf("received %d flow observations, want 1", len(got))
	}
	if got[0].SrcAS != 0 {
		t.Errorf("SrcAS = %d, want 0 (unknown)", got[0].SrcAS)
	}
}
