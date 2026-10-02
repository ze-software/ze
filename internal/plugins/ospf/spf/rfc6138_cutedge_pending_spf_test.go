// Design: docs/architecture/wire/ospf.md -- OSPF SPF and LDP-IGP synchronization.
// Related: cutedge.go -- IsCutEdge and flushPendingSPF, the producer.
//
// VALIDATES: RFC 6138 Appendix A: "If an SPF run was scheduled but is pending execution,
// that SPF MUST be executed immediately before any procedure checks whether an interface
// is a 'cut-edge'." The topology changes after an SPF run and a new run is left pending;
// IsCutEdge answers from the graph of the pending run, which differs from the stale one.
// PREVENTS: a cut-edge answer read from the graph of the previous SPF while the run that
// reflects the current LSDB is still armed.
package spf

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc6138PendingComputer builds root 1.1.1.1 and 2.2.2.2 on the LAN 10.0.0.254, adding
// the point-to-point pair 1.1.1.1 <-> 2.2.2.2 when parallel is true, runs SPF once, then
// installs the other topology with sequence numbers one higher and arms a pending SPF
// (one-hour delay, so only a flush runs it). It returns the computer with that run
// pending and the SPF snapshot asserted unchanged by the arming.
func rfc6138PendingComputer(t *testing.T, parallel bool) *Computer {
	t.Helper()
	area := testArea()
	routers := func(withP2P, newer bool) []packet.LSA {
		one := []packet.RouterLink{transitLinkDR(t, "10.0.0.254", "10.0.0.1", 10)}
		two := []packet.RouterLink{transitLinkDR(t, "10.0.0.254", "10.0.0.2", 10)}
		if withP2P {
			one = append(one, p2pLink(t, "2.2.2.2", "172.16.0.1", 5))
			two = append(two, p2pLink(t, "1.1.1.1", "172.16.0.2", 5))
		}
		lsas := []packet.LSA{routerLSA(t, "1.1.1.1", one...), routerLSA(t, "2.2.2.2", two...)}
		if newer {
			for i := range lsas {
				lsas[i].Header.Sequence = types.InitialSequenceNumber.Next()
			}
		}
		return lsas
	}
	src := testSource(t, area, append(routers(parallel, false),
		networkLSA(t, "10.0.0.254", "1.1.1.1", "255.255.255.0", "1.1.1.1", "2.2.2.2"))...)
	c := NewComputer(Config{Source: src, Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area},
		SPFDelay: time.Hour, SPFHold: time.Hour, SPFMaxHold: time.Hour})
	c.Run()
	before := len(c.SPFSnapshot())
	for _, lsa := range routers(!parallel, true) {
		if !src.Install(area, lsa) {
			t.Fatalf("installing the newer Router-LSA of %s failed", lsa.Header.AdvertisingRouter)
		}
	}
	c.TriggerArea(area)
	if got := len(c.SPFSnapshot()); got != before {
		t.Fatalf("SPF ran on arming (%d states, was %d); the run should still be pending", got, before)
	}
	return c
}

// RFC requirement: RFC6138-x-1 positive -- after an SPF over a LAN with a parallel
// point-to-point path to 2.2.2.2, the point-to-point links are withdrawn and a new SPF is
// left pending; IsCutEdge on the LAN 10.0.0.254 answers true (a cut-edge), the answer of
// the pending run's graph, not false, the answer of the graph already computed.
func TestRFC6138PendingSPFRunBeforeCutEdgeAfterLinkLoss(t *testing.T) {
	// Goal: the pending SPF runs before the cut-edge check. Method: run, change the
	// LSDB, arm a one-hour SPF, query.
	c := rfc6138PendingComputer(t, true)
	if !c.IsCutEdge(testArea(), testLSID(t, "10.0.0.254")) {
		t.Fatal("IsCutEdge = false: answered from the SPF before the point-to-point links were withdrawn")
	}
}

// RFC requirement: RFC6138-x-1 negative -- after an SPF over a LAN that is root's only
// path to 2.2.2.2, a parallel point-to-point pair is added and a new SPF is left pending;
// IsCutEdge on the LAN 10.0.0.254 never answers true, the cut-edge answer of the graph
// already computed: it answers false, the pending run's graph.
func TestRFC6138PendingSPFNeverLeavesStaleCutEdge(t *testing.T) {
	// Goal: a stale cut-edge answer is not returned while an SPF is pending. Method: the
	// positive's network in the other order.
	c := rfc6138PendingComputer(t, false)
	if c.IsCutEdge(testArea(), testLSID(t, "10.0.0.254")) {
		t.Fatal("IsCutEdge = true: answered from the SPF before the point-to-point pair was added")
	}
}
