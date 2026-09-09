// VALIDATES: a replay hands a reconnecting FIB plugin the forwarding entry Ze
// has outstanding for a prefix -- the next-hop, the device, the share and the
// multipath group the rest of the package computes for it.
// PREVENTS: a replay that re-programs the winner's own gateway after a
// promotion took the prefix onto an equal-cost member, which puts the prefix
// back over a gateway the resolver declared unreachable and drops the member
// that was carrying the traffic.

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

// TestReplayCarriesThePromotedMember is the promotion case. The winner's
// gateway stopped resolving, an equal-cost member took the prefix, and the FIB
// holds the MEMBER's address over the MEMBER's device. A replay that reports
// the winner's own next-hop and device instead re-programs the prefix over the
// dead gateway, and `fib-kernel` asks for a replay on every start.
func TestReplayCarriesThePromotedMember(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	winnerPath := netip.MustParsePrefix("192.0.2.0/24")
	memberPath := netip.MustParsePrefix("198.51.100.0/24")
	loc.Insert(family.IPv4Unicast, winnerPath, locrib.Path{Source: connectedID})
	loc.Insert(family.IPv4Unicast, memberPath, locrib.Path{Source: connectedID})

	pfx := netip.MustParsePrefix("10.30.0.0/24")
	winnerNH := netip.MustParseAddr("192.0.2.1")
	memberNH := netip.MustParseAddr("198.51.100.5")
	addPath(s, "bgp", pfx, winnerNH, "eth0", 20)
	addPath(s, "ospf", pfx, memberNH, "eth1", 20)

	loc.Remove(family.IPv4Unicast, winnerPath, connectedID, 0)
	s.processCascade([]netip.Addr{winnerNH})
	promoted := publishedChanges(t, bus)
	if len(promoted) != 3 || promoted[2].NextHop != memberNH {
		t.Fatalf("setup: published %+v, want the promotion of the member over %s", promoted, memberNH)
	}

	s.replayBest(&replay.Request{ReplayID: replay.Broadcast})

	replayed := publishedChanges(t, bus)[3:]
	if len(replayed) != 1 {
		t.Fatalf("the replay published %+v, want one change for %s", replayed, pfx)
	}
	entry := replayed[0]
	if entry.NextHop != memberNH || entry.Interface != "eth1" || entry.Weight != 1 {
		t.Errorf("the replay carries next-hop %s over %q with share %d, want %s over \"eth1\" with "+
			"share 1: the promoted member holds the prefix and the winner's gateway does not resolve",
			entry.NextHop, entry.Interface, entry.Weight, memberNH)
	}
	if len(entry.ECMPPaths) != 0 {
		t.Errorf("the replay carries the group %+v, want none: the one member there was is the "+
			"primary next-hop now", entry.ECMPPaths)
	}
}

// TestReplayCarriesTheProgrammedEntry is the ordinary case, and it is the
// POSITIVE half: a replay of a prefix Ze programs normally carries the resolved
// next-hop, the winner's device and the equal-cost group under it. Without it
// the replay is proven only by what it declines to publish, so deleting the
// whole loop body keeps the package green.
//
// The member is RECURSIVE, so its gateway and the address the FIB programs for
// it are different addresses. A group collected without resolution reads the
// same as a resolved one when every member is directly connected, which is what
// left the live change and the replay free to publish two different multipaths
// for one prefix. The live change is compared against the replay here for that
// reason: one prefix, one answer.
func TestReplayCarriesTheProgrammedEntry(t *testing.T) {
	bgpID := redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	// 198.51.100.5 is reached over 192.0.2.9, which is on the connected link.
	transit := netip.MustParseAddr("192.0.2.9")
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), locrib.Path{Source: connectedID})
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"),
		locrib.Path{Source: bgpID, NextHop: transit})

	pfx := netip.MustParsePrefix("10.31.0.0/24")
	winnerNH := netip.MustParseAddr("192.0.2.1")
	memberNH := netip.MustParseAddr("198.51.100.5")
	addPath(s, "bgp", pfx, winnerNH, "eth0", 20)
	addPath(s, "ospf", pfx, memberNH, "eth1", 20)

	live := publishedChanges(t, bus)
	if len(live) != 2 {
		t.Fatalf("setup: published %+v, want the add and the group update for %s", live, pfx)
	}

	s.replayBest(&replay.Request{ReplayID: replay.Broadcast})

	replayed := publishedChanges(t, bus)[2:]
	if len(replayed) != 1 {
		t.Fatalf("the replay published %+v, want one change for %s", replayed, pfx)
	}
	entry := replayed[0]
	if entry.Action != routeaction.Add || entry.Prefix != pfx {
		t.Errorf("the replay published %s for %s, want an add for %s", entry.Action, entry.Prefix, pfx)
	}
	if entry.NextHop != winnerNH || entry.Interface != "eth0" || entry.Protocol != "bgp" {
		t.Errorf("the replay carries next-hop %s over %q from %q, want %s over \"eth0\" from bgp",
			entry.NextHop, entry.Interface, entry.Protocol, winnerNH)
	}
	if len(entry.ECMPPaths) != 1 || entry.ECMPPaths[0].NextHop != transit ||
		entry.ECMPPaths[0].Interface != "eth1" || entry.ECMPPaths[0].Weight != 1 {
		t.Errorf("the replay carries the group %+v, want the one member over \"eth1\" with share 1 "+
			"at %s, the address %s resolves to", entry.ECMPPaths, transit, memberNH)
	}
	if ecmpChanged(live[1].ECMPPaths, entry.ECMPPaths) {
		t.Errorf("the live change carried the group %+v and the replay carries %+v: the FIB is told "+
			"two different things about %s", live[1].ECMPPaths, entry.ECMPPaths, pfx)
	}
}
