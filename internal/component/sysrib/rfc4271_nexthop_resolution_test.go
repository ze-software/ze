// Design: docs/architecture/core-design.md -- sysrib resolves a BGP NEXT_HOP before the FIB programs it
// Related: nhresolver.go -- Resolve walks the Loc-RIB to the directly connected next hop
// Related: sysrib.go -- fibEntry and resolveMember put the resolved address in the published entry

package sysrib

import (
	"context"
	"net/netip"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// rfc4271RecursiveTopology is a BGP route two resolution steps away from a
// connected subnet: 192.168.50.0/24 via BGP NEXT_HOP 172.16.5.5, 172.16.0.0/16
// via the IGP gateway 10.0.0.1, and 10.0.0.0/24 connected on eth1.
type rfc4271RecursiveTopology struct {
	bus       *testEventBus
	loc       *locrib.RIB
	igpSource redistevents.ProtocolID
	igpPrefix netip.Prefix
	bgpPrefix netip.Prefix
	bgpNH     netip.Addr
	directNH  netip.Addr
}

// startRFC4271RecursiveTopology runs a sysRIB over a fresh Loc-RIB and inserts
// the three routes in dependency order, so the BGP route is resolvable when it
// arrives. The sysRIB worker stops at test cleanup.
func startRFC4271RecursiveTopology(t *testing.T) *rfc4271RecursiveTopology {
	t.Helper()
	redistevents.ResetForTest()
	bgpID := redistevents.RegisterProtocol("bgp")
	igpID := redistevents.RegisterProtocol("ospf")
	connID := redistevents.RegisterProtocol("connected")

	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })

	s := newSysRIB()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go s.runUntilDone(ctx, done)
	t.Cleanup(func() {
		cancel()
		<-done
	})

	topo := &rfc4271RecursiveTopology{
		bus:       bus,
		loc:       loc,
		igpSource: igpID,
		igpPrefix: netip.MustParsePrefix("172.16.0.0/16"),
		bgpPrefix: netip.MustParsePrefix("192.168.50.0/24"),
		bgpNH:     netip.MustParseAddr("172.16.5.5"),
		directNH:  netip.MustParseAddr("10.0.0.1"),
	}
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), locrib.Path{
		Source: connID, Interface: "eth1", AdminDistance: 0, Metric: 0,
	})
	loc.Insert(family.IPv4Unicast, topo.igpPrefix, locrib.Path{
		Source: igpID, NextHop: topo.directNH, AdminDistance: 110, Metric: 20,
	})
	loc.Insert(family.IPv4Unicast, topo.bgpPrefix, locrib.Path{
		Source: bgpID, NextHop: topo.bgpNH, AdminDistance: 20, Metric: 0,
	})
	return topo
}

// runUntilDone runs the worker and closes done when it returns, so a test can
// wait for the stop without an anonymous goroutine.
func (s *sysRIB) runUntilDone(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	s.run(ctx)
}

// bgpEntries returns every published change for prefix, in publication order.
func (topo *rfc4271RecursiveTopology) bgpEntries() []sysribevents.BestChangeEntry {
	all := bestChangeEntries(topo.bus)
	var out []sysribevents.BestChangeEntry
	for i := range all {
		if all[i].Prefix == topo.bgpPrefix {
			out = append(out, all[i])
		}
	}
	return out
}

// VALIDATES: a BGP route whose NEXT_HOP is not directly connected reaches the
// FIB with the immediate, directly connected next hop the NEXT_HOP resolves to,
// never with the NEXT_HOP address itself.
// METHOD: run the sysRIB over a Loc-RIB holding a connected /24 on eth1, an IGP
// /16 via 10.0.0.1 inside it, and a BGP /24 whose NEXT_HOP 172.16.5.5 sits in
// the IGP /16. Read the (system-rib, best-change) entry the FIB writer programs.
//
// RFC requirement: RFC4271-9.1.2.1-1 positive -- the BGP route's NEXT_HOP 172.16.5.5
// is resolved through the Loc-RIB before the route is published to the FIB, and the
// published Add carries the directly connected address 10.0.0.1 and its device eth1,
// which is the address the FIB programs for forwarding; no entry for the prefix ever
// carries the unresolved NEXT_HOP (internal/component/sysrib/sysrib.go resolveMember).
func TestRFC4271BGPNextHopResolvedToTheImmediateNextHop(t *testing.T) {
	topo := startRFC4271RecursiveTopology(t)

	add, ok := waitForEntry(t, topo.bus, func(e sysribevents.BestChangeEntry) bool {
		return e.Prefix == topo.bgpPrefix && e.Action == routeaction.Add
	})
	if !ok {
		t.Fatalf("no Add published for %v; entries=%+v", topo.bgpPrefix, bestChangeEntries(topo.bus))
	}
	if add.Protocol != "bgp" {
		t.Errorf("Add protocol = %q, want bgp", add.Protocol)
	}
	if add.NextHop != topo.directNH {
		t.Errorf("Add next-hop = %v, want the directly connected %v (BGP NEXT_HOP %v)", add.NextHop, topo.directNH, topo.bgpNH)
	}
	if add.Interface != "eth1" {
		t.Errorf("Add interface = %q, want eth1", add.Interface)
	}
	entries := topo.bgpEntries()
	for i := range entries {
		if entries[i].NextHop == topo.bgpNH {
			t.Errorf("entry %+v carries the unresolved BGP NEXT_HOP", entries[i])
		}
	}
}

// VALIDATES: a BGP route whose NEXT_HOP stops resolving to a directly connected
// next hop is taken out of the FIB, and is not programmed toward the NEXT_HOP
// address instead.
// METHOD: the topology of the positive; once its Add is published, remove the
// IGP /16 that carried the resolution, then read what the FIB writer is told
// about the BGP prefix after that point.
//
// RFC requirement: RFC4271-9.1.2.1-1 negative -- when the NEXT_HOP 172.16.5.5 no longer
// resolves (the IGP route covering it is removed), the sysRIB publishes a Withdraw for
// the BGP prefix and no later Add or Update for it, so no packet is forwarded along a
// route whose NEXT_HOP is unresolved (internal/component/sysrib/sysrib.go
// cascadeRecompute).
func TestRFC4271BGPRouteWithAnUnresolvedNextHopLeavesTheFIB(t *testing.T) {
	topo := startRFC4271RecursiveTopology(t)

	if _, ok := waitForEntry(t, topo.bus, func(e sysribevents.BestChangeEntry) bool {
		return e.Prefix == topo.bgpPrefix && e.Action == routeaction.Add
	}); !ok {
		t.Fatalf("no Add published for %v; entries=%+v", topo.bgpPrefix, bestChangeEntries(topo.bus))
	}
	before := len(topo.bgpEntries())

	topo.loc.Remove(family.IPv4Unicast, topo.igpPrefix, topo.igpSource, 0)

	if _, ok := waitForEntry(t, topo.bus, func(e sysribevents.BestChangeEntry) bool {
		return e.Prefix == topo.bgpPrefix && e.Action.Verb() == routeaction.VerbRemove
	}); !ok {
		t.Fatalf("no Withdraw published for %v after its NEXT_HOP became unresolved; entries=%+v", topo.bgpPrefix, topo.bgpEntries())
	}
	// A late re-install toward the unresolved NEXT_HOP would arrive after the
	// Withdraw, so the window stays open before the verdict.
	time.Sleep(200 * time.Millisecond)
	after := topo.bgpEntries()[before:]
	for i := range after {
		if after[i].Action.Verb() != routeaction.VerbRemove {
			t.Errorf("after the NEXT_HOP became unresolved the FIB was told %+v", after[i])
		}
	}
}
