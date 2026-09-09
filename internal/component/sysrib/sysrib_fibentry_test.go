// VALIDATES: fibEntry is the ONE answer to "what does the FIB hold for this
// prefix": the live change, the cascade and the replay all emit it, so the
// address, the device, the share and the multipath group they publish for one
// prefix cannot disagree.
// PREVENTS: the three ways a second computation diverged -- a live change
// publishing a member's RAW gateway where the replay published the resolved
// one, a member the cascade dropped for being unreachable coming back into the
// kernel multipath on the next live change, and a replay refusing a prefix the
// live path programmed over a gateway the Loc-RIB does not cover.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// TestLiveChangeKeepsTheGroupTheCascadeNarrowed is the reachability filter at
// the LIVE emit. A cascade drops a member whose gateway stopped resolving, and
// the next change for the same prefix recollects the group. Collecting it
// without the filter puts the dead member back into the kernel multipath, and
// the traffic hashed onto it is blackholed until something else moves.
func TestLiveChangeKeepsTheGroupTheCascadeNarrowed(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	winnerCover := netip.MustParsePrefix("192.0.2.0/24")
	memberCover := netip.MustParsePrefix("198.51.100.0/24")
	loc.Insert(family.IPv4Unicast, winnerCover, locrib.Path{Source: connectedID})
	loc.Insert(family.IPv4Unicast, memberCover, locrib.Path{Source: connectedID})

	pfx := netip.MustParsePrefix("10.40.0.0/24")
	winnerNH := netip.MustParseAddr("192.0.2.1")
	memberNH := netip.MustParseAddr("198.51.100.5")
	addPath(s, "bgp", pfx, winnerNH, "eth0", 20)
	addPath(s, "ospf", pfx, memberNH, "eth1", 20)

	// The member's gateway loses its covering route. The prefix is re-evaluated
	// through the winner's next-hop, which is the one the resolver tracks.
	loc.Remove(family.IPv4Unicast, memberCover, connectedID, 0)
	s.processCascade([]netip.Addr{winnerNH})

	narrowed := publishedChanges(t, bus)
	if len(narrowed) != 3 || len(narrowed[2].ECMPPaths) != 0 {
		t.Fatalf("setup: published %+v, want the cascade to drop the unreachable member", narrowed)
	}

	// A live change for the prefix: the winner moved to another device.
	addPath(s, "bgp", pfx, winnerNH, "eth9", 20)

	live := publishedChanges(t, bus)[3:]
	if len(live) != 1 {
		t.Fatalf("published %+v, want one change for %s", live, pfx)
	}
	if live[0].Interface != "eth9" {
		t.Errorf("published the change over %q, want \"eth9\": the winner moved device",
			live[0].Interface)
	}
	if len(live[0].ECMPPaths) != 0 {
		t.Errorf("the live change carries the group %+v, want none: %s stopped resolving and the "+
			"cascade already took it out of the kernel multipath", live[0].ECMPPaths, memberNH)
	}
}

// TestReplayAgreesWithTheLiveAddForAnUnresolvedGateway is the state the Loc-RIB
// cannot answer for. A route whose gateway no path in the Loc-RIB covers is
// programmed over the target its producer named, because the Loc-RIB is not the
// router's whole picture of reachability: an OSPF or IS-IS next-hop sits on a
// link whose connected route no plugin inserted. A replay that refuses the same
// prefix hands a reconnecting FIB plugin an empty table where Ze holds an
// install, and the kernel entry survives with nothing left to withdraw it.
func TestReplayAgreesWithTheLiveAddForAnUnresolvedGateway(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	pfx := netip.MustParsePrefix("10.41.0.0/24")
	nextHop := netip.MustParseAddr("192.0.2.1")
	addPath(s, "bgp", pfx, nextHop, "eth0", 20)

	live := publishedChanges(t, bus)
	if len(live) != 1 || live[0].Action != routeaction.Add || live[0].NextHop != nextHop {
		t.Fatalf("setup: published %+v, want one add for %s over %s", live, pfx, nextHop)
	}

	s.replayBest(&replay.Request{ReplayID: replay.Broadcast})

	replayed := publishedChanges(t, bus)[1:]
	if len(replayed) != 1 {
		t.Fatalf("the replay published %+v, want the one prefix Ze has an install outstanding for",
			replayed)
	}
	if replayed[0].NextHop != live[0].NextHop || replayed[0].Interface != live[0].Interface ||
		replayed[0].Protocol != live[0].Protocol {
		t.Errorf("the replay carries %s over %q from %q and the live add carried %s over %q from %q: "+
			"one prefix, one answer", replayed[0].NextHop, replayed[0].Interface, replayed[0].Protocol,
			live[0].NextHop, live[0].Interface, live[0].Protocol)
	}
}

// TestResolvedNextHopIsASubsetOfBest pins the invariant recomputeBest's first
// Add relies on: a prefix with no previous winner has no install outstanding,
// so the change it owes is an Add and never an Update. Every path that writes
// the resolved next-hop writes the winner, and every path that takes a prefix
// out of the best table takes it out of both, so a key here is a key there. The
// four states that hold a best route with nothing programmed are driven below,
// because each is a chance for the two tables to part.
func TestResolvedNextHopIsASubsetOfBest(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")
	redistevents.RegisterProtocol("static")
	connectedID := redistevents.RegisterProtocol("connected")
	osProtocol := osInstalledProtocol(t)
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "static")

	covering := netip.MustParsePrefix("192.0.2.0/24")
	loc.Insert(family.IPv4Unicast, covering, locrib.Path{Source: connectedID})
	gateway := netip.MustParseAddr("192.0.2.1")

	addPath(s, "bgp", netip.MustParsePrefix("10.42.0.0/24"), gateway, "eth0", 20)
	addPath(s, "static", netip.MustParsePrefix("10.43.0.0/24"), gateway, "eth0", 10)
	addPath(s, osProtocol, netip.MustParsePrefix("10.44.0.0/24"), gateway, "eth0", 5)

	withdrawn := netip.MustParsePrefix("10.45.0.0/24")
	addPath(s, "ospf", withdrawn, gateway, "eth0", 110)
	withdrawPath(s, "ospf", withdrawn)

	// The gateway every prefix above shares loses its covering route, which is
	// the state that empties the resolved table for the ones Ze programmed.
	loc.Remove(family.IPv4Unicast, covering, connectedID, 0)
	s.processCascade([]netip.Addr{gateway})

	for key := range s.resolvedNH {
		if s.best[key] == nil {
			t.Errorf("%s has a resolved next-hop and no best route: recomputeBest reads a nil "+
				"previous winner as \"Ze programmed nothing\" and would send an Add for a prefix "+
				"the FIB already holds", key.prefix)
		}
	}
}
