// VALIDATES: spec-ospf-ext-11 -- RFC 6138 §4 / Appendix A cut-edge detection over the
// last SPF result, and the Appendix A MUST that a scheduled-but-pending SPF is executed
// before the cut-edge query.
// PREVENTS: withholding a cut-edge interface (network partition) and reading a stale
// cut-edge result across a topology change.
package spf

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// cutEdgeSource: root 1.1.1.1's ONLY link is the transit to the LAN pseudonode
// (10.0.0.254). Removing that link leaves no path to the LAN -> cut-edge.
func cutEdgeSource(t *testing.T) *Computer {
	t.Helper()
	area := testArea()
	src := testSource(t, area,
		routerLSA(t, "1.1.1.1", transitLinkDR(t, "10.0.0.254", "10.0.0.1", 10)),
		routerLSA(t, "2.2.2.2", transitLinkDR(t, "10.0.0.254", "10.0.0.2", 10)),
		networkLSA(t, "10.0.0.254", "1.1.1.1", "255.255.255.0", "1.1.1.1", "2.2.2.2"),
	)
	return NewComputer(Config{Source: src, Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area}})
}

// nonCutEdgeSource: root also has a P2P link to 2.2.2.2, which is itself attached to
// the LAN, so an alternate path to the LAN exists -> not a cut-edge.
func nonCutEdgeSource(t *testing.T) *Computer {
	t.Helper()
	area := testArea()
	src := testSource(t, area,
		routerLSA(t, "1.1.1.1", transitLinkDR(t, "10.0.0.254", "10.0.0.1", 10), p2pLink(t, "2.2.2.2", "172.16.0.1", 5)),
		routerLSA(t, "2.2.2.2", transitLinkDR(t, "10.0.0.254", "10.0.0.2", 10), p2pLink(t, "1.1.1.1", "172.16.0.2", 5)),
		networkLSA(t, "10.0.0.254", "1.1.1.1", "255.255.255.0", "1.1.1.1", "2.2.2.2"),
	)
	return NewComputer(Config{Source: src, Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area}})
}

func TestLDPSyncBroadcastCutEdgeDetected(t *testing.T) {
	c := cutEdgeSource(t)
	c.Run()
	if !c.IsCutEdge(testArea(), testLSID(t, "10.0.0.254")) {
		t.Fatal("expected cut-edge: the LAN has no alternate path from root")
	}
}

func TestLDPSyncBroadcastNonCutEdgeDetected(t *testing.T) {
	c := nonCutEdgeSource(t)
	c.Run()
	if c.IsCutEdge(testArea(), testLSID(t, "10.0.0.254")) {
		t.Fatal("expected non-cut-edge: an alternate path to the LAN exists via the P2P link")
	}
}

// RFC requirement: RFC6138-x-1 positive -- a scheduled-but-pending SPF is executed immediately
// before the cut-edge query reads the graph: IsCutEdge flushes the pending SPF (SPFSnapshot becomes
// non-empty) and answers from the fresh graph (RFC 6138 Appendix A).
// RFC requirement: RFC6138-x-1 negative -- the pending SPF is not run prematurely: before IsCutEdge
// is called the armed SPF has not executed (SPFSnapshot is empty), so it is specifically the
// cut-edge query that triggers the flush, not a background timer.
func TestLDPSyncCutEdgeUsesFreshSPF(t *testing.T) {
	// AC-8 / A-6 / R-8: a scheduled-but-pending SPF MUST be executed before the
	// cut-edge query reads the graph (RFC 6138 Appendix A).
	area := testArea()
	src := testSource(t, area,
		routerLSA(t, "1.1.1.1", transitLinkDR(t, "10.0.0.254", "10.0.0.1", 10), p2pLink(t, "2.2.2.2", "172.16.0.1", 5)),
		routerLSA(t, "2.2.2.2", transitLinkDR(t, "10.0.0.254", "10.0.0.2", 10), p2pLink(t, "1.1.1.1", "172.16.0.2", 5)),
		networkLSA(t, "10.0.0.254", "1.1.1.1", "255.255.255.0", "1.1.1.1", "2.2.2.2"),
	)
	// A long delay so the armed timer will not fire on its own during the test; the
	// only way SPF runs is the Appendix A flush inside IsCutEdge.
	c := NewComputer(Config{Source: src, Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area}, SPFDelay: time.Hour, SPFHold: time.Hour, SPFMaxHold: time.Hour})
	c.TriggerArea(area) // arms a pending SPF, does NOT run it

	if got := c.SPFSnapshot(); len(got) != 0 {
		t.Fatalf("SPF ran before the flush (%d states); the pending run should still be armed", len(got))
	}

	// The query must flush the pending SPF first, then answer from the fresh graph.
	if c.IsCutEdge(area, testLSID(t, "10.0.0.254")) {
		t.Fatal("expected non-cut-edge after the pending SPF is flushed")
	}
	if got := c.SPFSnapshot(); len(got) == 0 {
		t.Fatal("pending SPF was not flushed before the cut-edge query (RFC 6138 Appendix A MUST)")
	}
}
