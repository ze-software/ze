// Design: docs/architecture/rib/unified-locrib.md -- forwarding metadata through recursive resolution.
package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/nexthop"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

// A recursive gateway inherits the terminal adjacency's device and on-link
// flag, including when recursion crosses from IPv4 into IPv6.
func TestRecursiveForwardingKeepsAdjacency(t *testing.T) {
	loc := locrib.NewRIB()
	cover := netip.MustParsePrefix("192.0.2.0/24")
	adjacent := recursivePath(netip.MustParseAddr("fe80::2"), 10)
	adjacent.Interface, adjacent.OnLink = "eth7", true
	loc.Insert(family.IPv4Unicast, cover, adjacent)
	resolver := newNHResolver(loc)
	resolved := resolver.Resolve(netip.MustParseAddr("192.0.2.1"))
	if !resolved.Resolved || resolved.DirectNH != adjacent.NextHop || resolved.Interface != "eth7" || !resolved.OnLink {
		t.Fatalf("recursive adjacency = %+v", resolved)
	}
	adjacent.OnLink = false
	adjacent.NextHop = netip.MustParseAddr("2001:db8::2")
	loc.Insert(family.IPv4Unicast, cover, adjacent)
	terminal := connectedPath(0)
	terminal.Interface = "eth8"
	loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::/64"), terminal)
	resolved = resolver.Resolve(netip.MustParseAddr("192.0.2.1"))
	if !resolved.Resolved || resolved.DirectNH != adjacent.NextHop || resolved.Interface != "eth8" || resolved.OnLink {
		t.Fatalf("cross-family resolution = %+v", resolved)
	}
	terminal.RouteType = routetype.Unreachable
	loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::/64"), terminal)
	if resolved = resolver.Resolve(netip.MustParseAddr("192.0.2.1")); resolved.Resolved {
		t.Fatalf("discard route proved a forwarding adjacency: %+v", resolved)
	}
}

// Two recursive roots can resolve to the same link-local address on distinct
// devices. The FIB and replay must keep both scoped targets and their weights.
func TestRecursiveCrossFamilyGroupReachesFIB(t *testing.T) {
	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()
	gateway := netip.MustParseAddr("fe80::2")
	for index, cover := range []string{"192.0.2.1/32", "192.0.2.2/32"} {
		path := recursivePath(gateway, 10)
		path.Interface = []string{"eth1", "eth2"}[index]
		path.OnLink = true
		loc.Insert(family.IPv4Unicast, netip.MustParsePrefix(cover), path)
	}
	prefix := netip.MustParsePrefix("198.51.100.0/24")
	_, changes := s.processEvent(fromLocRIBBatch("static", family.IPv4Unicast, &incomingChange{
		Action: routeaction.Add, Prefix: prefix, NextHop: netip.MustParseAddr("192.0.2.1"),
		Weight: 3, Metric: 77,
		ECMPNextHops: []nexthop.NextHop{{Addr: netip.MustParseAddr("192.0.2.2"), Weight: 2}},
	}))
	check := func(changes []outgoingChange) {
		t.Helper()
		if len(changes) != 1 {
			t.Fatalf("FIB changes = %+v", changes)
		}
		entry := changes[0]
		if entry.NextHop != gateway || entry.Interface != "eth1" || !entry.OnLink || entry.Weight != 3 || entry.Metric != 77 {
			t.Fatalf("recursive primary lost its terminal adjacency: %+v", entry)
		}
		if len(entry.ECMPPaths) != 1 {
			t.Fatalf("scoped sibling was removed: %+v", entry)
		}
		member := entry.ECMPPaths[0]
		if member.NextHop != gateway || member.Interface != "eth2" || !member.OnLink || member.Weight != 2 {
			t.Fatalf("recursive sibling lost its terminal adjacency: %+v", member)
		}
	}
	check(changes)
	s.replayBest(&replay.Request{ReplayID: replay.Broadcast})
	check(publishedChanges(t, bus))
}

// A device-only primary still re-evaluates recursive members. A covering-route
// device change reaches the live FIB and replay, and withdrawal removes only
// the unavailable member without reinstalling a withheld prefix.
func TestECMPRecursiveMemberLifecycle(t *testing.T) {
	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()
	prefix := netip.MustParsePrefix("198.51.100.0/24")
	cover := netip.MustParsePrefix("2001:db8::/64")
	gateway := netip.MustParseAddr("192.0.2.2")
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), recursivePath(netip.MustParseAddr("2001:db8::2"), 0))
	terminal := connectedPath(0)
	terminal.Interface = "eth1"
	loc.Insert(family.IPv6Unicast, cover, terminal)
	_, changes := s.processEvent(fromLocRIBBatch("static", family.IPv4Unicast, &incomingChange{
		Action: routeaction.Add, Prefix: prefix, Interface: "eth0",
		ECMPNextHops: []nexthop.NextHop{{Addr: gateway, Weight: 3}},
	}))
	if len(changes) != 1 || len(changes[0].ECMPPaths) != 1 || changes[0].ECMPPaths[0].Interface != "eth1" {
		t.Fatalf("initial member = %+v", changes)
	}
	terminal.Interface = "eth2"
	loc.Insert(family.IPv6Unicast, cover, terminal)
	s.processCascade(getNHResolver().CoveredNHs(cover))
	published := publishedChanges(t, bus)
	if len(published) != 1 || published[0].Action != routeaction.Update || len(published[0].ECMPPaths) != 1 || published[0].ECMPPaths[0].Interface != "eth2" {
		t.Fatalf("member device change = %+v", published)
	}
	s.replayBest(&replay.Request{ReplayID: replay.Broadcast})
	published = publishedChanges(t, bus)
	if len(published) != 2 || len(published[1].ECMPPaths) != 1 || published[1].ECMPPaths[0].Interface != "eth2" || published[1].ECMPPaths[0].Weight != 3 {
		t.Fatalf("member replay = %+v", published)
	}
	loc.Remove(family.IPv6Unicast, cover, testProto, 0)
	s.processCascade(getNHResolver().CoveredNHs(cover))
	published = publishedChanges(t, bus)
	if len(published) != 3 || published[2].Action != routeaction.Update || len(published[2].ECMPPaths) != 0 || published[2].Interface != "eth0" {
		t.Fatalf("member withdrawal = %+v", published)
	}
	withheld := s.applyFIBImport(map[string]bool{"static": false})[family.IPv4Unicast]
	if len(withheld) != 1 || withheld[0].Action != routeaction.Withdraw {
		t.Fatalf("withhold = %+v", withheld)
	}
	loc.Insert(family.IPv6Unicast, cover, terminal)
	s.processCascade(getNHResolver().CoveredNHs(cover))
	if got := len(publishedChanges(t, bus)); got != len(published) {
		t.Fatalf("withheld prefix reinstalled: %d events", got)
	}
	permitted := s.applyFIBImport(map[string]bool{"static": true})[family.IPv4Unicast]
	if len(permitted) != 1 || permitted[0].Action != routeaction.Add || len(permitted[0].ECMPPaths) != 1 || permitted[0].ECMPPaths[0].Interface != "eth2" {
		t.Fatalf("permit = %+v", permitted)
	}
}

// A service SID must survive the authoritative Loc-RIB path and control both
// installation and withdrawal when SID reachability changes.
func TestLocRIBServiceSIDLifecycle(t *testing.T) {
	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()
	prefix := netip.MustParsePrefix("198.51.100.0/24")
	cover := netip.MustParsePrefix("2001:db8::/64")
	path := connectedPath(0)
	path.Interface = "eth0"
	path.SRv6SID = netip.MustParseAddr("2001:db8::1234")
	var change locrib.Change
	unsubscribe := loc.OnChange(func(c locrib.Change) { change = c })
	defer unsubscribe()
	loc.Insert(family.IPv4Unicast, prefix, path)
	s.processLocRIBChange(&change)
	if got := publishedChanges(t, bus); len(got) != 0 {
		t.Fatalf("unreachable SID installed: %+v", got)
	}
	loc.Insert(family.IPv6Unicast, cover, connectedPath(0))
	s.processCascade(getNHResolver().CoveredNHs(cover))
	got := publishedChanges(t, bus)
	if len(got) != 1 || got[0].SRv6SID != path.SRv6SID || got[0].Action != routeaction.Add {
		t.Fatalf("reachable SID not installed: %+v", got)
	}
	s.replayBest(&replay.Request{ReplayID: replay.Broadcast})
	got = publishedChanges(t, bus)
	if len(got) != 2 || got[1].SRv6SID != path.SRv6SID {
		t.Fatalf("SID lost on replay: %+v", got)
	}
	loc.Remove(family.IPv6Unicast, cover, testProto, 0)
	s.processCascade(getNHResolver().CoveredNHs(cover))
	got = publishedChanges(t, bus)
	if len(got) != 3 || got[2].Action != routeaction.Withdraw {
		t.Fatalf("unreachable SID not withdrawn: %+v", got)
	}
}

// Repeated updates to an unreachable covering prefix must not turn a withdrawn
// path back into an unproved install. Only renewed reachability restores it.
func TestCascadeKeepsLostPathWithdrawn(t *testing.T) {
	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()
	cover := netip.MustParsePrefix("192.0.2.0/24")
	gateway := netip.MustParseAddr("192.0.2.1")
	prefix := netip.MustParsePrefix("198.51.100.0/24")
	loc.Insert(family.IPv4Unicast, cover, connectedPath(0))
	_, initial := s.processEvent(fromLocRIBBatch("static", family.IPv4Unicast, &incomingChange{
		Action: routeaction.Add, Prefix: prefix, NextHop: gateway,
	}))
	if len(initial) != 1 || initial[0].Action != routeaction.Add {
		t.Fatalf("initial install = %+v", initial)
	}
	loc.Remove(family.IPv4Unicast, cover, testProto, 0)
	for range 2 {
		s.processCascade(getNHResolver().CoveredNHs(cover))
	}
	got := publishedChanges(t, bus)
	if len(got) != 1 || got[0].Action != routeaction.Withdraw {
		t.Fatalf("unreachable cascade resurrected the route: %+v", got)
	}
	loc.Insert(family.IPv4Unicast, cover, connectedPath(0))
	s.processCascade(getNHResolver().CoveredNHs(cover))
	got = publishedChanges(t, bus)
	if len(got) != 2 || got[1].Action != routeaction.Add {
		t.Fatalf("restored covering route did not reinstall: %+v", got)
	}
}

func TestPromotedPrimaryMetadataChangesReachFIB(t *testing.T) {
	state := newPromotionState(t, netip.MustParsePrefix("10.63.0.0/24"))
	state.member.labels = []uint32{100}
	state.promote(t)
	state.member.labels = []uint32{200}
	addProducerRoute(state.sysrib, state.prefix, state.member)
	got := publishedChanges(t, state.bus)
	if len(got) != 3 || got[2].Action != routeaction.Update || len(got[2].Labels) != 1 || got[2].Labels[0] != 200 {
		t.Fatalf("same-address promoted relabel was suppressed: %+v", got)
	}
	state.member.iface = "eth2"
	addProducerRoute(state.sysrib, state.prefix, state.member)
	got = publishedChanges(t, state.bus)
	if len(got) != 4 || got[3].Action != routeaction.Update || got[3].Interface != "eth2" {
		t.Fatalf("same-address promoted device change was suppressed: %+v", got)
	}
	state.sysrib.replayBest(&replay.Request{ReplayID: replay.Broadcast})
	got = publishedChanges(t, state.bus)
	if len(got) != 5 || got[4].NextHop != state.member.nextHop || got[4].Interface != "eth2" || len(got[4].Labels) != 1 || got[4].Labels[0] != 200 {
		t.Fatalf("replay differs from the installed promoted path: %+v", got)
	}
}
