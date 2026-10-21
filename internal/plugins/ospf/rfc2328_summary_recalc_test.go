// Design: docs/architecture/ospf/ospf-8-spf-rib.md -- inter-area routes from summary-LSAs.
// Related: spf_wiring.go -- initSPF wires lsdb.SetOnChange to the SPF computer.
// Related: spf/interarea.go -- ComputeInterAreaWith, RFC 2328 Section 16.2.
//
// VALIDATES: RFC 2328 Section 13.2, "The best route to the destination described by the
// summary-LSA must be recalculated (see Section 16.5).", through the engine's own wiring:
// installing a new summary-LSA instance moves the inter-area route it describes.
// PREVENTS: a routing table that keeps the cost or the existence an earlier summary-LSA
// instance gave a destination.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

var summaryRecalcPrefix = netip.MustParsePrefix("198.51.100.0/24")

// summaryRecalcEngine is an internal router 1.1.1.1 in the backbone only, with a
// point-to-point link of cost 10 to the area border router 3.3.3.3.
func summaryRecalcEngine(t *testing.T) (*engine, types.RouterID) {
	t.Helper()
	self := mustRouterID(t, "1.1.1.1")
	abr := mustRouterID(t, "3.3.3.3")
	e := newEngine(nil)
	e.spf.SetRoot(self)
	e.spf.SetAreas([]types.AreaID{types.BackboneArea})
	e.spf.SetTimers(time.Millisecond, time.Millisecond, time.Millisecond)
	t.Cleanup(e.spf.Stop)
	toABR := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(abr), LinkData: ip4ForTest("10.0.0.1"), Metric: 10}
	toSelf := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(self), LinkData: ip4ForTest("10.0.0.2"), Metric: 10}
	if !e.lsdb.Install(types.BackboneArea, bgplsReachabilityRouter(self, toABR)) {
		t.Fatal("install our router-LSA")
	}
	abrLSA := bgplsReachabilityRouter(abr, toSelf)
	abrLSA.Router.Flags = packet.RouterFlagB
	if !e.lsdb.Install(types.BackboneArea, abrLSA) {
		t.Fatal("install the area border router's router-LSA")
	}
	return e, abr
}

// summaryRecalcLSA is 3.3.3.3's summary-LSA for 198.51.100.0/24 at seq with metric.
func summaryRecalcLSA(abr types.RouterID, seq types.LSSequenceNumber, metric uint32) packet.LSA {
	return packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeSummaryNetwork, LinkStateID: types.LinkStateID{198, 51, 100, 0}, AdvertisingRouter: abr, Sequence: seq},
		Summary: &packet.SummaryLSA{NetworkMask: [4]byte{255, 255, 255, 0}, Metric: metric}}
}

// summaryRecalcRoute returns the computer's route to 198.51.100.0/24, if any.
func summaryRecalcRoute(e *engine) (uint64, bool) {
	for _, r := range e.spf.Routes() {
		if r.Prefix == summaryRecalcPrefix {
			return r.Metric, true
		}
	}
	return 0, false
}

// summaryRecalcWait waits up to two seconds for want to hold of the route to
// 198.51.100.0/24, and fails the test with why when it never does.
func summaryRecalcWait(t *testing.T, e *engine, want func(metric uint64, ok bool) bool, why string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if want(summaryRecalcRoute(e)) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	metric, ok := summaryRecalcRoute(e)
	t.Fatalf("%s: route to %s is (metric %d, present %v)", why, summaryRecalcPrefix, metric, ok)
}

// RFC requirement: RFC2328-13.2-1 positive -- installing a new instance of the area border router's summary-LSA for 198.51.100.0/24 with a changed metric recalculates the route to that destination: the inter-area route's cost moves from 15 (link 10 + metric 5) to 35 (link 10 + metric 25) with no other trigger (LSDB.Install, SetOnChange, ComputeInterAreaWith).
func TestRFC2328ChangedSummaryRecalculatesRoute(t *testing.T) {
	e, abr := summaryRecalcEngine(t)
	if !e.lsdb.Install(types.BackboneArea, summaryRecalcLSA(abr, types.InitialSequenceNumber, 5)) {
		t.Fatal("install the summary-LSA")
	}
	summaryRecalcWait(t, e, func(m uint64, ok bool) bool { return ok && m == 15 }, "baseline: route at cost 15")
	if !e.lsdb.Install(types.BackboneArea, summaryRecalcLSA(abr, types.InitialSequenceNumber+1, 25)) {
		t.Fatal("install the changed summary-LSA")
	}
	summaryRecalcWait(t, e, func(m uint64, ok bool) bool { return ok && m == 35 }, "changed summary-LSA: route at cost 35")
}

// RFC requirement: RFC2328-13.2-1 negative -- the route an earlier summary-LSA instance produced is not kept: once a new instance advertises 198.51.100.0/24 at LSInfinity, the recalculation removes the route to it from the routing table (LSDB.Install, SetOnChange, ComputeInterAreaWith).
func TestRFC2328SummaryAtInfinityRemovesRoute(t *testing.T) {
	e, abr := summaryRecalcEngine(t)
	if !e.lsdb.Install(types.BackboneArea, summaryRecalcLSA(abr, types.InitialSequenceNumber, 5)) {
		t.Fatal("install the summary-LSA")
	}
	summaryRecalcWait(t, e, func(m uint64, ok bool) bool { return ok && m == 15 }, "baseline: route at cost 15")
	if !e.lsdb.Install(types.BackboneArea, summaryRecalcLSA(abr, types.InitialSequenceNumber+1, 0xFFFFFF)) {
		t.Fatal("install the summary-LSA at LSInfinity")
	}
	summaryRecalcWait(t, e, func(_ uint64, ok bool) bool { return !ok }, "summary-LSA at LSInfinity: route removed")
}
