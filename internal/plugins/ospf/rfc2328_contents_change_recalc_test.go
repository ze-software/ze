// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md -- LSDB install drives the SPF recalculation.
// Related: spf_wiring.go -- initSPF, which wires lsdb.SetOnChange to the SPF computer's TriggerArea.

package ospf

// VALIDATES: RFC 2328 Section 13.2, an installed LSA with changed contents recalculates every area.
// PREVENTS: an install that never reaches the SPF computer, or a recalculation of the dirty area only.

import (
	"maps"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// recalcArea1 is the non-backbone area whose database changes in the 13-6 units.
var recalcArea1 = types.AreaID{0, 0, 0, 1}

// recalcEngine builds an engine whose SPF computer (built by initSPF, wired to the LSDB by
// SetOnChange) runs a millisecond after a trigger and calculates areas 0.0.0.0 and 0.0.0.1.
func recalcEngine(t *testing.T, self types.RouterID) *engine {
	t.Helper()
	e := newEngine(nil)
	e.spf.SetRoot(self)
	e.spf.SetAreas([]types.AreaID{types.BackboneArea, recalcArea1})
	e.spf.SetTimers(time.Millisecond, time.Millisecond, time.Millisecond)
	// A not-advertised range hides the neighbor's stub from the backbone. Without it, the summary
	// this ABR originates for the stub would change in area 0.0.0.0 too, and that backbone
	// install would trigger the backbone recalculation on its own.
	e.spf.SetAreaConfigs([]ospfspf.AreaConfig{{AreaID: recalcArea1,
		Ranges: []ospfspf.AreaRange{
			{Prefix: netip.MustParsePrefix("203.0.113.0/24"), Advertise: false},
			{Prefix: netip.MustParsePrefix("10.0.0.0/8"), Advertise: false},
		}}})
	t.Cleanup(e.spf.Stop)
	// The backbone holds our router-LSA, so the baseline run calculates both areas and the
	// later change in area 0.0.0.1 is the only trigger left to recalculate the backbone.
	backboneStub := packet.RouterLink{Type: packet.RouterLinkTypeStub, LinkID: types.LinkStateID{192, 0, 2, 0},
		LinkData: ip4ForTest("255.255.255.0"), Metric: 1}
	if !e.lsdb.Install(types.BackboneArea, bgplsReachabilityRouter(self, backboneStub)) {
		t.Fatal("install our backbone router-LSA")
	}
	return e
}

// recalcLastRuns returns each calculated area's last-run stamp from `show ospf spf`.
func recalcLastRuns(e *engine) map[string]string {
	out := make(map[string]string)
	for _, row := range e.spf.SPFSnapshot() {
		out[row.Area] = row.LastRun
	}
	return out
}

// recalcPending reports whether an SPF run is armed and has not started.
func recalcPending(e *engine) bool {
	for _, row := range e.spf.SPFSnapshot() {
		if row.Pending {
			return true
		}
	}
	return false
}

// recalcSettle waits until ready holds, no run is armed, and the last-run stamps stay the same
// for 20 ms, so no run still in flight can move a stamp after the caller reads them. It fails
// the test after two seconds and returns the settled stamps.
func recalcSettle(t *testing.T, e *engine, ready func() bool, why string) map[string]string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ready() && !recalcPending(e) {
			stamps := recalcLastRuns(e)
			time.Sleep(20 * time.Millisecond)
			if !recalcPending(e) && maps.Equal(stamps, recalcLastRuns(e)) {
				return stamps
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s: the SPF computer never settled with the expected routing table (last runs %v)", why, recalcLastRuns(e))
	return nil
}

// recalcRequireEveryArea fails unless both areas were calculated again after the baseline.
func recalcRequireEveryArea(t *testing.T, before, after map[string]string, why string) {
	t.Helper()
	for _, area := range []types.AreaID{types.BackboneArea, recalcArea1} {
		name := area.String()
		if before[name] == "" {
			t.Fatalf("baseline: area %s was never calculated; the setup is void", name)
		}
		if after[name] == before[name] {
			t.Fatalf("%s: area %s was not recalculated (last run %s unchanged)", why, name, before[name])
		}
	}
}

// recalcHasRoute reports whether the computer's routing table holds prefix as an area route.
func recalcHasRoute(e *engine, area types.AreaID, prefix netip.Prefix) bool {
	for _, r := range e.spf.Routes() {
		if r.AreaID == area && r.Prefix == prefix {
			return true
		}
	}
	return false
}

// RFC requirement: RFC2328-13-6 positive -- installing an LSA whose contents differ from the database copy recalculates the entire routing table, starting with the shortest-path calculation of each area, not only the area whose database changed: a changed router-LSA, and a changed network-LSA, installed in area 0.0.0.1 through the engine's LSDB each trigger an SPF run that recalculates area 0.0.0.0 as well as area 0.0.0.1, and the routing table follows the new contents (LSDB.Install -> SetOnChange -> Computer.TriggerArea -> Computer.Run, spf_wiring.go).
//
// The test drives the production wiring: nothing calls Run or TriggerArea by hand, so an
// install that no longer reached the computer, or a computer that recalculated only the dirty
// area, leaves a last-run stamp unchanged and fails the wait. The router-LSA change replaces
// the neighbor's link back to us by a stub; the network-LSA change drops the neighbor from the
// attached routers. Either way the neighbor's stub 203.0.113.64/26 must leave the table.
func TestRFC2328ChangedContentsRecalculateEveryArea(t *testing.T) {
	self := mustRouterID(t, "1.1.1.1")
	peer := mustRouterID(t, "2.2.2.2")
	peerStub := netip.MustParsePrefix("203.0.113.64/26")
	stub := packet.RouterLink{Type: packet.RouterLinkTypeStub, LinkID: types.LinkStateID{203, 0, 113, 64},
		LinkData: ip4ForTest("255.255.255.192"), Metric: 5}

	t.Run("router-lsa", func(t *testing.T) {
		e := recalcEngine(t, self)
		toPeer := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(peer), LinkData: ip4ForTest("10.0.0.1"), Metric: 10}
		toSelf := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(self), LinkData: ip4ForTest("10.0.0.2"), Metric: 10}
		if !e.lsdb.Install(recalcArea1, bgplsReachabilityRouter(self, toPeer)) {
			t.Fatal("install our router-LSA")
		}
		if !e.lsdb.Install(recalcArea1, bgplsReachabilityRouter(peer, toSelf, stub)) {
			t.Fatal("install the neighbor's router-LSA")
		}
		before := recalcSettle(t, e, func() bool { return recalcHasRoute(e, recalcArea1, peerStub) }, "baseline: neighbor's stub routed")

		// The neighbor's next instance, aged one second by the flood: a different body (its only
		// link is the 10.0.0.0/24 stub).
		changed := routerLSAForTest(t, peer, types.InitialSequenceNumber+1, 1)
		if !e.lsdb.Install(recalcArea1, changed) {
			t.Fatal("install the changed router-LSA")
		}
		why := "changed router-LSA in area 0.0.0.1"
		after := recalcSettle(t, e, func() bool { return !recalcHasRoute(e, recalcArea1, peerStub) }, why+": neighbor's stub withdrawn")
		recalcRequireEveryArea(t, before, after, why)
	})

	t.Run("network-lsa", func(t *testing.T) {
		e := recalcEngine(t, self)
		dr := types.LinkStateID{10, 1, 0, 2}
		selfTransit := packet.RouterLink{Type: packet.RouterLinkTypeTransit, LinkID: dr, LinkData: ip4ForTest("10.1.0.1"), Metric: 10}
		peerTransit := packet.RouterLink{Type: packet.RouterLinkTypeTransit, LinkID: dr, LinkData: ip4ForTest("10.1.0.2"), Metric: 10}
		network := func(seq types.LSSequenceNumber, attached ...types.RouterID) packet.LSA {
			return packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeNetwork, LinkStateID: dr, AdvertisingRouter: peer, Sequence: seq},
				Network: &packet.NetworkLSA{NetworkMask: ip4ForTest("255.255.255.0"), AttachedRouters: attached}}
		}
		if !e.lsdb.Install(recalcArea1, bgplsReachabilityRouter(self, selfTransit)) {
			t.Fatal("install our router-LSA")
		}
		if !e.lsdb.Install(recalcArea1, bgplsReachabilityRouter(peer, peerTransit, stub)) {
			t.Fatal("install the neighbor's router-LSA")
		}
		if !e.lsdb.Install(recalcArea1, network(types.InitialSequenceNumber, peer, self)) {
			t.Fatal("install the network-LSA")
		}
		before := recalcSettle(t, e, func() bool { return recalcHasRoute(e, recalcArea1, peerStub) }, "baseline: neighbor's stub routed")

		// The DR's next instance no longer lists the neighbor as attached.
		if !e.lsdb.Install(recalcArea1, network(types.InitialSequenceNumber+1, peer)) {
			t.Fatal("install the changed network-LSA")
		}
		why := "changed network-LSA in area 0.0.0.1"
		after := recalcSettle(t, e, func() bool { return !recalcHasRoute(e, recalcArea1, peerStub) }, why+": neighbor's stub withdrawn")
		recalcRequireEveryArea(t, before, after, why)
	})
}
