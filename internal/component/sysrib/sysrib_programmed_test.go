// VALIDATES: the one test for "Ze has an install outstanding for this prefix"
// is the PRESENCE of the resolved next-hop, never a best route in the table and
// never the validity of the address the entry holds.
// PREVENTS: the two failures that reading either one produces -- a Withdraw for
// a prefix the FIB writer never installed, which it reports as a sync failure,
// and a group of dead next-hops left in the kernel because the prefix was filed
// under "Ze programmed nothing".

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

// TestCascadeWithdrawsAGroupPromotedToADeviceOnlyMember is the case a validity
// test cannot see. A member named by a device alone is programmed under an
// INVALID address, so a promotion writes one into the resolved table. When the
// group later loses that member and the winner's gateway is still unreachable,
// the prefix has no path left and the kernel must be told: reading the address
// rather than the entry files a programmed prefix under "Ze programmed
// nothing", and the kernel keeps forwarding over next-hops that are all gone.
func TestCascadeWithdrawsAGroupPromotedToADeviceOnlyMember(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	covering := netip.MustParsePrefix("192.0.2.0/24")
	loc.Insert(family.IPv4Unicast, covering, locrib.Path{Source: connectedID})

	pfx := netip.MustParsePrefix("10.20.0.0/24")
	winnerNH := netip.MustParseAddr("192.0.2.1")
	addPath(s, "bgp", pfx, winnerNH, "", 20)
	addPath(s, "ospf", pfx, netip.Addr{}, "eth0", 20)

	loc.Remove(family.IPv4Unicast, covering, connectedID, 0)
	s.processCascade([]netip.Addr{winnerNH})
	promoted := publishedChanges(t, bus)
	if len(promoted) != 3 || promoted[2].Interface != "eth0" {
		t.Fatalf("setup: published %+v, want the promotion of the device-only member", promoted)
	}

	// The member goes, and the winner's own gateway is still unreachable, so
	// the prefix has nothing left to forward over.
	withdrawPath(s, "ospf", pfx)
	s.processCascade([]netip.Addr{winnerNH})

	left := publishedChanges(t, bus)[3:]
	if len(left) != 1 || left[0].Action != routeaction.Withdraw || left[0].Prefix != pfx {
		t.Errorf("the cascade published %+v, want a withdraw of %s: every next-hop the group had is "+
			"gone, and the promoted member's address is invalid rather than absent", left, pfx)
	}
}

// TestReplaySkipsAPrefixZeDoesNotProgram is the replay side of the same test. A
// FIB plugin that reconnects asks for the table, and the table holds a best
// route for prefixes Ze programs nothing for. Answering with an Add for one
// installs the route the cascade withdrew.
func TestReplaySkipsAPrefixZeDoesNotProgram(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	covering := netip.MustParsePrefix("192.0.2.0/24")
	loc.Insert(family.IPv4Unicast, covering, locrib.Path{Source: connectedID})
	pfx := netip.MustParsePrefix("10.21.0.0/24")
	nextHop := netip.MustParseAddr("192.0.2.1")
	addPath(s, "bgp", pfx, nextHop, "", 20)

	loc.Remove(family.IPv4Unicast, covering, connectedID, 0)
	s.processCascade([]netip.Addr{nextHop})
	if changes := publishedChanges(t, bus); len(changes) != 2 || changes[1].Action != routeaction.Withdraw {
		t.Fatalf("setup: published %+v, want the add and the withdraw an unreachable next-hop owes", changes)
	}

	s.replayBest(&replay.Request{ReplayID: replay.Broadcast})

	if replayed := publishedChanges(t, bus)[2:]; len(replayed) != 0 {
		t.Errorf("the replay published %+v: %s is in the best table and Ze has no install "+
			"outstanding for it", replayed, pfx)
	}
}

// TestAddsAPrefixTakenFromAWithheldWinner is the action side of the same test.
// The prefix was held by a protocol the operator withholds, so no FIB writer
// holds an entry for it. When a permitted protocol takes it, what is owed is an
// ADD: an Update names an entry to replace, and there is none.
func TestAddsAPrefixTakenFromAWithheldWinner(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp")

	pfx := netip.MustParsePrefix("10.22.0.0/24")
	addPath(s, "bgp", pfx, netip.MustParseAddr("192.0.2.1"), "", 20)
	if changes := publishedChanges(t, bus); len(changes) != 0 {
		t.Fatalf("setup: published %+v, want nothing: bgp is withheld", changes)
	}

	addPath(s, "ospf", pfx, netip.MustParseAddr("192.0.2.2"), "eth0", 10)

	taken := publishedChanges(t, bus)
	if len(taken) != 1 {
		t.Fatalf("published %+v, want one change for %s", taken, pfx)
	}
	if taken[0].Action != routeaction.Add {
		t.Errorf("published %s for %s, want an add: Ze programmed nothing for the withheld winner, "+
			"so no FIB writer holds an entry to update", taken[0].Action, pfx)
	}
}
