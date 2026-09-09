// VALIDATES: the outgoing device and the multipath share travel from the Loc-RIB
// Path through sysrib to the FIB event, on the route's own next-hop and on every
// equal-cost member; a producer that states no share still emits 1; and sysrib
// still populates no table id.
// PREVENTS: a configured route losing its device or its weight at the arbitration
// hop, which programs a route to nowhere or an equal share where the operator
// asked for a proportion; and a silent change to what an unweighted BGP, OSPF or
// IS-IS group programs.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/nexthop"
)

// TestBestChangeCarriesTheOutgoingDevice is AC-13 at the arbitration hop: a route
// whose only next-hop is a device has no gateway at all, so the device is the
// only thing that names where the packet goes.
func TestBestChangeCarriesTheOutgoingDevice(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	pfx := netip.MustParsePrefix("10.0.0.0/8")
	_, changes := s.processEvent(fromLocRIBBatch("static", family.IPv4Unicast, &incomingChange{
		Action:    routeaction.Add,
		Prefix:    pfx,
		Interface: "tun100",
		Priority:  10,
	}))

	if len(changes) != 1 {
		t.Fatalf("got %d best-changes, want 1", len(changes))
	}
	if changes[0].Interface != "tun100" {
		t.Errorf("Interface = %q, want tun100", changes[0].Interface)
	}
	if changes[0].NextHop.IsValid() {
		t.Errorf("NextHop = %v, want none: the route names a device and no gateway", changes[0].NextHop)
	}
}

// TestBestChangeCarriesTheDeclaredWeights is AC-12 at the arbitration hop.
func TestBestChangeCarriesTheDeclaredWeights(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	pfx := netip.MustParsePrefix("10.0.0.0/8")
	nh1 := netip.MustParseAddr("192.0.2.1")
	nh2 := netip.MustParseAddr("192.0.2.2")
	_, changes := s.processEvent(fromLocRIBBatch("static", family.IPv4Unicast, &incomingChange{
		Action:       routeaction.Add,
		Prefix:       pfx,
		NextHop:      nh1,
		Weight:       3,
		Priority:     10,
		ECMPNextHops: []nexthop.NextHop{{Addr: nh2, Weight: 1}},
	}))

	if len(changes) != 1 {
		t.Fatalf("got %d best-changes, want 1", len(changes))
	}
	if changes[0].Weight != 3 {
		t.Errorf("primary Weight = %d, want 3", changes[0].Weight)
	}
	if len(changes[0].ECMPPaths) != 1 || changes[0].ECMPPaths[0].Weight != 1 {
		t.Errorf("ECMPPaths = %+v, want one member of weight 1", changes[0].ECMPPaths)
	}
}

// TestECMPPathCarriesTheInterface proves a device-only member survives the group
// build. It is the case an address-only group cannot cover: every device-only
// member has the same invalid address, so a next-hop-keyed dedup or a
// validity filter would collapse or drop them.
func TestECMPPathCarriesTheInterface(t *testing.T) {
	winner := &protocolRoute{
		protocol:         "static",
		nextHop:          netip.MustParseAddr("192.0.2.1"),
		priority:         10,
		nextHopWeight:    2,
		ecmpNextHops:     []nexthop.NextHop{{Interface: "tun100", Weight: 4}, {Interface: "tun101", Weight: 1}},
		nextHopInterface: "",
	}
	paths := newSysRIB().ecmpCollect(map[string]*protocolRoute{"static": winner}, winner)

	if len(paths) != 2 {
		t.Fatalf("group has %d members, want the two device-only siblings: %+v", len(paths), paths)
	}
	got := map[string]uint8{}
	for _, p := range paths {
		got[p.Interface] = p.Weight
	}
	if got["tun100"] != 4 || got["tun101"] != 1 {
		t.Errorf("device shares = %+v, want tun100:4 and tun101:1", got)
	}
}

// TestEqualCostGroupKeepsWeightOneForProducersThatStateNone is R-7: BGP, OSPF and
// IS-IS state no share, and what they program must not change because a share can
// now be stated.
func TestEqualCostGroupKeepsWeightOneForProducersThatStateNone(t *testing.T) {
	for _, protocol := range []string{"bgp", "ospf", "isis"} {
		winner := &protocolRoute{
			protocol:     protocol,
			nextHop:      netip.MustParseAddr("192.0.2.1"),
			priority:     20,
			ecmpNextHops: []nexthop.NextHop{{Addr: netip.MustParseAddr("192.0.2.2")}},
		}
		paths := newSysRIB().ecmpCollect(map[string]*protocolRoute{protocol: winner}, winner)
		if len(paths) != 1 {
			t.Fatalf("%s: group has %d members, want 1", protocol, len(paths))
		}
		if paths[0].Weight != 1 {
			t.Errorf("%s: member weight = %d, want 1 for a producer that states none", protocol, paths[0].Weight)
		}
	}
}

// TestSysRIBEmitsNoTableID is A-3 and the main-table boundary: sysrib assigns no
// table id anywhere, so everything it publishes lands in the main table. The
// table dimension belongs to plan/immediate/spec-fib-depth.md, and a static route
// in a named table is kept out of the Loc-RIB for exactly this reason.
func TestSysRIBEmitsNoTableID(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	_, changes := s.processEvent(fromLocRIBBatch("static", family.IPv4Unicast, &incomingChange{
		Action:   routeaction.Add,
		Prefix:   netip.MustParsePrefix("10.0.0.0/8"),
		NextHop:  netip.MustParseAddr("192.0.2.1"),
		Priority: 10,
	}))

	if len(changes) != 1 {
		t.Fatalf("got %d best-changes, want 1", len(changes))
	}
	if changes[0].TableID != 0 {
		t.Errorf("TableID = %d, want 0: sysrib populates no table id", changes[0].TableID)
	}
}

// TestChangeToBatchCarriesTheDeviceAndWeight pins the one translation from a
// Loc-RIB Change into what sysrib consumes. A field dropped here is invisible to
// every in-process deployment.
func TestChangeToBatchCarriesTheDeviceAndWeight(t *testing.T) {
	batch := changeToBatch(locrib.Change{
		Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("10.0.0.0/8"),
		Kind:   locrib.ChangeAdd,
		Best: locrib.Path{
			Source:        redistevents.RegisterProtocol("static"),
			NextHop:       netip.MustParseAddr("192.0.2.1"),
			Interface:     "tun100",
			Weight:        7,
			AdminDistance: 10,
		},
	})
	if batch == nil {
		t.Fatal("changeToBatch returned no batch for an Add")
	}
	if batch.Changes[0].Interface != "tun100" {
		t.Errorf("Interface = %q, want tun100", batch.Changes[0].Interface)
	}
	if batch.Changes[0].Weight != 7 {
		t.Errorf("Weight = %d, want 7", batch.Changes[0].Weight)
	}
}
