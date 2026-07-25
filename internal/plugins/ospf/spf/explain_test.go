// VALIDATES: spec-ospf-ext-14 AC-9, A-3, R-3 -- the SPF-explain snapshot lists the
// candidate paths considered per prefix, the winner, its cost, and the RFC 2328 Section 16.4
// path-preference tie-break that selected it, read-only from the LAST result WITHOUT a
// recompute (the route table and run count are unchanged).
// PREVENTS: an explain view that re-runs SPF, mutates the installed routes, or reports the
// wrong tie-break rationale.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func route(prefix string, metric uint64, rt RouteType, origin types.RouterID) RouteEntry {
	return RouteEntry{
		AreaID: types.BackboneArea,
		Prefix: netip.MustParsePrefix(prefix),
		Metric: metric,
		Type:   rt,
		Origin: origin,
	}
}

func TestSPFExplainCandidateList(t *testing.T) {
	o1 := testRID(t, "1.1.1.1")
	o2 := testRID(t, "2.2.2.2")
	winner := route("10.0.0.0/24", 10, RouteIntraArea, o1)
	loser := route("10.0.0.0/24", 20, RouteInterArea, o2)

	c := &Computer{}
	c.last = []RouteEntry{winner}
	c.lastCandidates = []RouteEntry{winner, loser}

	entries := c.ExplainSnapshot()
	if len(entries) != 1 {
		t.Fatalf("explain entries = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Prefix != "10.0.0.0/24" || e.WinningType != "intra-area" || e.WinningMetric != 10 {
		t.Fatalf("winner fields = %+v", e)
	}
	if len(e.Candidates) != 2 {
		t.Fatalf("candidates = %d, want 2", len(e.Candidates))
	}
	winners := 0
	for _, cand := range e.Candidates {
		if cand.Winner {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one candidate should be the winner, got %d", winners)
	}
}

func TestSPFExplainTieBreak(t *testing.T) {
	o1 := testRID(t, "1.1.1.1")
	o2 := testRID(t, "2.2.2.2")
	// Same prefix, intra-area (rank 0) beats inter-area (rank 1) by path preference.
	c := &Computer{
		last:           []RouteEntry{route("10.0.0.0/24", 100, RouteIntraArea, o1)},
		lastCandidates: []RouteEntry{route("10.0.0.0/24", 100, RouteIntraArea, o1), route("10.0.0.0/24", 5, RouteInterArea, o2)},
	}
	e := c.ExplainSnapshot()[0]
	if e.Reason == "" {
		t.Fatalf("tie-break reason must be set")
	}
	// Even though the inter-area path has a lower metric, the intra-area path wins by type.
	if e.WinningType != "intra-area" {
		t.Fatalf("winning type = %q, want intra-area", e.WinningType)
	}
}

func TestSPFExplainNoRecompute(t *testing.T) {
	o1 := testRID(t, "1.1.1.1")
	c := &Computer{
		last:           []RouteEntry{route("10.9.0.0/24", 10, RouteIntraArea, o1)},
		lastCandidates: []RouteEntry{route("10.9.0.0/24", 10, RouteIntraArea, o1)},
		runs:           7,
	}
	beforeRuns := c.runCount()
	beforeRoutes := len(c.Routes())

	_ = c.ExplainSnapshot()
	_ = c.ExplainSnapshot()

	if c.runCount() != beforeRuns {
		t.Fatalf("explain changed run count: %d -> %d", beforeRuns, c.runCount())
	}
	if len(c.Routes()) != beforeRoutes {
		t.Fatalf("explain changed the installed route set")
	}
}
