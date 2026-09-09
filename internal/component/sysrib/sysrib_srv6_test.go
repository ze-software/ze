// VALIDATES: the one site that reads an SRv6 Service SID's resolvability --
// fibEntry, which also covers the ECMP promotion below it -- and four of the
// five call sites that act on the verdict it answers with: the two refusals in
// recomputeBest, the replay, and the permission sweep. The fifth is
// cascadeRecompute, fibEntry's fourth caller, which no test here drives. A SID
// the Loc-RIB does not cover leaves the prefix in the RIB with nothing
// programmed, and a SID it covers is programmed like any other route.
// PREVENTS: the three ways a forwarding entry outlives the SID it encapsulates
// to -- a prefix installed over a SID no locator route reaches, a prefix
// promoted onto an equal-cost member while its own SID is unreachable, and a
// replay handing a reconnecting FIB plugin a prefix whose SID stopped
// resolving. The replay re-reads resolvability rather than trusting the state
// it stored, and that is a DEFENSIVE read rather than the only route:
// trackNextHops tracks the SID, so a covering-route change reaches the prefix
// through a cascade as well. What the re-read costs is one lookup per replayed
// prefix, and what it buys is that a replay cannot be the path that reinstates
// an entry encapsulating to a SID nothing reaches.

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

// addSRv6Path hands sysrib one BGP route over eth0 carrying an SRv6 Service
// SID, at the eBGP distance 20, and publishes whatever the arbitration decided.
// That is the shape a VPN prefix over an SRv6 core reaches the system RIB in.
//
// The SID is an IPv6 address whichever family the prefix belongs to, so its
// reachability is a question the Loc-RIB answers separately from the
// next-hop's, and every test below reads the two apart.
func addSRv6Path(s *sysRIB, prefix netip.Prefix, nextHop, sid netip.Addr) {
	fam, changes := s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{{
		Action:    routeaction.Add,
		Prefix:    prefix,
		NextHop:   nextHop,
		SRv6SID:   sid,
		Interface: "eth0",
		Priority:  20,
	}}))
	if len(changes) > 0 {
		publishChanges(changes, fam)
	}
}

// srv6Locator is the SRv6 locator every SID below is allocated from. A test
// gives a SID its reachability by inserting this prefix into the Loc-RIB and
// takes it away by removing it, which is the state RFC 9252 Section 5 makes the
// ingress PE read.
var srv6Locator = netip.MustParsePrefix("2001:db8:cafe::/48")

// TestUnresolvableSRv6SIDIsNotProgrammed is the first refusal in recomputeBest:
// a prefix with no previous winner whose SID the Loc-RIB does not cover. The
// route wins selection and enters the RIB, because withholding the FIB write is
// not withdrawing the route, and nothing is published. Programming it would
// give the kernel a SEG6 encap to a destination no route reaches, which
// blackholes the traffic hashed onto it with no counter to show it.
func TestUnresolvableSRv6SIDIsNotProgrammed(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	// The next-hop resolves. The SID is the only thing that does not, so the
	// refusal below can come from nothing else.
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), locrib.Path{Source: connectedID})

	pfx := netip.MustParsePrefix("10.50.0.0/24")
	sid := netip.MustParseAddr("2001:db8:cafe::1")
	addSRv6Path(s, pfx, netip.MustParseAddr("192.0.2.1"), sid)

	if changes := publishedChanges(t, bus); len(changes) != 0 {
		t.Fatalf("published %+v, want nothing: %s is not covered by any route in the Loc-RIB",
			changes, sid)
	}

	key := prefixKey{family: family.IPv4Unicast, prefix: pfx}
	s.mu.RLock()
	best, programmed := s.best[key], s.programmedByZe(key)
	s.mu.RUnlock()

	if best == nil {
		t.Errorf("%s has no best route: an unresolvable SID declines the FIB write and the route "+
			"stays in the RIB, so `show rib` reports it and redistribution can offer it onward", pfx)
	}
	if programmed {
		t.Errorf("%s counts as programmed: nothing was published for it, so a later withdraw would "+
			"tell a FIB writer to remove an entry it never made", pfx)
	}
}

// TestResolvableSRv6SIDIsProgrammed is the other half of the same decision. A
// SID a locator route covers is reachable, so the prefix is programmed and the
// entry carries the SID the FIB writer builds the SEG6 encap from. Without this
// case the refusal above would also pass against code that programs no SRv6
// route at all.
func TestResolvableSRv6SIDIsProgrammed(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), locrib.Path{Source: connectedID})
	loc.Insert(family.IPv6Unicast, srv6Locator, locrib.Path{Source: connectedID})

	pfx := netip.MustParsePrefix("10.51.0.0/24")
	sid := netip.MustParseAddr("2001:db8:cafe::1")
	addSRv6Path(s, pfx, netip.MustParseAddr("192.0.2.1"), sid)

	changes := publishedChanges(t, bus)
	if len(changes) != 1 {
		t.Fatalf("published %+v, want one add for %s: %s is covered by %s", changes, pfx, sid, srv6Locator)
	}
	if changes[0].Action != routeaction.Add || changes[0].Prefix != pfx {
		t.Errorf("published %s %s, want an add for %s", changes[0].Action, changes[0].Prefix, pfx)
	}
	if changes[0].SRv6SID != sid {
		t.Errorf("the entry carries the SID %s, want %s: the FIB writer builds the SEG6 encap from it",
			changes[0].SRv6SID, sid)
	}
}

// TestSRv6SIDLossWithdrawsTheProgrammedPrefix is the second refusal in
// recomputeBest, the one for a prefix Ze already holds an install for. A route
// with an unreachable SID takes the prefix from a route Ze programmed, so the
// kernel entry is now stale: the RIB says the prefix belongs to a path Ze
// cannot program, while the kernel still forwards over the path it beat.
func TestSRv6SIDLossWithdrawsTheProgrammedPrefix(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	gateway := netip.MustParseAddr("192.0.2.1")
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), locrib.Path{Source: connectedID})

	pfx := netip.MustParsePrefix("10.52.0.0/24")
	addPath(s, "ospf", pfx, gateway, "eth0", 110)

	installed := publishedChanges(t, bus)
	if len(installed) != 1 || installed[0].Action != routeaction.Add {
		t.Fatalf("setup: published %+v, want one add for %s", installed, pfx)
	}

	// BGP wins on distance and its SID reaches nothing.
	sid := netip.MustParseAddr("2001:db8:cafe::2")
	addSRv6Path(s, pfx, gateway, sid)

	taken := publishedChanges(t, bus)[1:]
	if len(taken) != 1 {
		t.Fatalf("published %+v, want one change for %s", taken, pfx)
	}
	if taken[0].Action != routeaction.Withdraw || taken[0].Prefix != pfx {
		t.Errorf("published %s %s, want a withdraw for %s: the winner's SID %s reaches nothing and "+
			"the entry Ze programmed for the route it beat is stale",
			taken[0].Action, taken[0].Prefix, pfx, sid)
	}

	key := prefixKey{family: family.IPv4Unicast, prefix: pfx}
	s.mu.RLock()
	programmed := s.programmedByZe(key)
	s.mu.RUnlock()
	if programmed {
		t.Errorf("%s still counts as programmed after the withdraw: the next change for it would be "+
			"an Update naming an entry the kernel no longer holds", pfx)
	}
}

// TestUnresolvableSRv6SIDIsNotPromotedOntoAnEqualCostMember is the claim
// fibEntry's own comment makes: the SID check covers the promotion below it. A
// winner whose gateway does not resolve hands the prefix to an equal-cost
// member that does, and a winner whose SID reaches nothing must not reach that
// branch at all. The two prefixes below differ in one thing, the reachability
// of the SID, so the promotion is proven to fire and proven to be refused.
func TestUnresolvableSRv6SIDIsNotPromotedOntoAnEqualCostMember(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	// Only the member's gateway is covered, so every winner below is unreachable
	// over its own gateway and the promotion is the branch that decides.
	memberNH := netip.MustParseAddr("198.51.100.5")
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), locrib.Path{Source: connectedID})
	loc.Insert(family.IPv6Unicast, srv6Locator, locrib.Path{Source: connectedID})
	winnerNH := netip.MustParseAddr("192.0.2.1")

	// The control: the SID resolves, so the member carries the prefix.
	reachable := netip.MustParsePrefix("10.53.0.0/24")
	addPath(s, "ospf", reachable, memberNH, "eth1", 20)
	addSRv6Path(s, reachable, winnerNH, netip.MustParseAddr("2001:db8:cafe::3"))

	promoted := publishedChanges(t, bus)
	if len(promoted) != 2 {
		t.Fatalf("setup: published %+v, want the ospf add and the bgp change for %s", promoted, reachable)
	}
	if promoted[1].NextHop != memberNH {
		t.Fatalf("setup: %s is programmed over %s, want the member %s: the promotion is the branch "+
			"this test needs to reach", reachable, promoted[1].NextHop, memberNH)
	}

	// The same shape with a SID no locator route covers.
	unreachable := netip.MustParsePrefix("10.54.0.0/24")
	sid := netip.MustParseAddr("2001:db8:beef::4")
	addPath(s, "ospf", unreachable, memberNH, "eth1", 20)
	addSRv6Path(s, unreachable, winnerNH, sid)

	after := publishedChanges(t, bus)[2:]
	if len(after) != 2 {
		t.Fatalf("published %+v, want the ospf add and one more change for %s", after, unreachable)
	}
	if after[1].Action != routeaction.Withdraw {
		t.Errorf("published %s %s over %s, want a withdraw for %s: %s reaches nothing, so the prefix "+
			"is not programmed over the equal-cost member either",
			after[1].Action, after[1].Prefix, after[1].NextHop, unreachable, sid)
	}
}

// TestReplayRereadsSRv6SIDResolvability is why replayBest asks fibEntry rather
// than reading the state it stored. A SID stops resolving when the locator
// route leaves the Loc-RIB, and no producer sends sysrib a change for the
// prefix that carries it. A replay that trusted the stored state would hand a
// reconnecting FIB plugin a prefix whose SEG6 encap reaches nothing, and the
// plugin would program it.
func TestReplayRereadsSRv6SIDResolvability(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), locrib.Path{Source: connectedID})
	loc.Insert(family.IPv6Unicast, srv6Locator, locrib.Path{Source: connectedID})

	pfx := netip.MustParsePrefix("10.55.0.0/24")
	sid := netip.MustParseAddr("2001:db8:cafe::5")
	addSRv6Path(s, pfx, netip.MustParseAddr("192.0.2.1"), sid)

	if changes := publishedChanges(t, bus); len(changes) != 1 {
		t.Fatalf("setup: published %+v, want one add for %s", changes, pfx)
	}

	s.replayBest(&replay.Request{ReplayID: replay.Broadcast})
	replayed := publishedChanges(t, bus)[1:]
	if len(replayed) != 1 || replayed[0].Prefix != pfx {
		t.Fatalf("setup: the replay published %+v, want %s while its SID resolves", replayed, pfx)
	}

	// The locator route goes and no cascade runs, which is the state a replay
	// is the only reader of: sysrib holds the prefix as programmed.
	loc.Remove(family.IPv6Unicast, srv6Locator, connectedID, 0)

	s.replayBest(&replay.Request{ReplayID: replay.Broadcast})
	after := publishedChanges(t, bus)[2:]
	if len(after) != 0 {
		t.Errorf("the replay published %+v, want nothing: %s no longer resolves, so a FIB plugin "+
			"taking this answer would program a SEG6 encap to a destination no route reaches",
			after, sid)
	}
}

// TestPermissionSweepDeclinesAnUnresolvableSRv6SID is the call site in the
// `rib { fib-withhold }` sweep. Giving a protocol its permission back must not
// install a route whose SID reaches nothing. The sweep programs every other
// verdict the way the live path does, so fibPathForbidden is the one verdict
// that still refuses it, and this is what proves the refusal survived that
// rule (fibimport.go, fibStateChange).
func TestPermissionSweepDeclinesAnUnresolvableSRv6SID(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp")

	loc.Insert(family.IPv6Unicast, srv6Locator, locrib.Path{Source: connectedID})

	// One prefix whose SID the locator covers and one whose SID it does not.
	// Both are named by a device alone, which is the branch under test.
	reachable := netip.MustParsePrefix("10.56.0.0/24")
	unreachable := netip.MustParsePrefix("10.57.0.0/24")
	sid := netip.MustParseAddr("2001:db8:beef::6")
	addSRv6Path(s, reachable, netip.Addr{}, netip.MustParseAddr("2001:db8:cafe::6"))
	addSRv6Path(s, unreachable, netip.Addr{}, sid)

	if changes := publishedChanges(t, bus); len(changes) != 0 {
		t.Fatalf("setup: published %+v, want nothing while bgp is withheld", changes)
	}

	reconfigure(t, s)

	sweep := publishedChanges(t, bus)
	if len(sweep) != 1 {
		t.Fatalf("the sweep published %+v, want the one prefix whose SID resolves", sweep)
	}
	if sweep[0].Prefix != reachable {
		t.Errorf("the sweep published %s, want %s: %s reaches nothing, so giving bgp its permission "+
			"back must not install it", sweep[0].Prefix, reachable, sid)
	}
}
