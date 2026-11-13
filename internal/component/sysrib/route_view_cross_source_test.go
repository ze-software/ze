// Design: docs/architecture/rib/unified-locrib.md -- selected forwarding membership.
// VALIDATES: the authority's cross-source members retain their FIB ownership.

package sysrib

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func crossSourceCandidate(protocol, device string, label uint32) locrib.Path {
	return locrib.Path{Source: redistevents.RegisterProtocol(protocol), Interface: device,
		DistanceOverride: 20, HasDistanceOverride: true, Metric: 10, Labels: []uint32{label}}
}

// selectedFromAuthority takes only the authority's selected value. The
// projected branch crosses the real exporter, DTO and projection validators.
// It neither chooses candidates nor fabricates membership in the fixture.
func selectedFromAuthority(t *testing.T, rib *locrib.RIB, prefix netip.Prefix, projected bool) routeViewSelectedChange {
	t.Helper()
	var prepared routeViewSelectedChange
	var prepareErr error
	if projected {
		var entry rpc.RouteViewEntry
		rib.Inspect(family.IPv4Unicast, prefix, func(group locrib.PathGroup) {
			prepareErr = routeViewCopyEntry(family.IPv4Unicast, prefix, &group, &entry)
		})
		if prepareErr != nil {
			t.Fatal(prepareErr)
		}
		projection, err := newSelectedProjection(rpc.RouteViewOpenOutput{Session: "cross-source", Epoch: "daemon", Snapshot: "initial"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := projection.Apply(&rpc.RouteViewReadOutput{Session: "cross-source", Epoch: "daemon", Snapshot: "initial", Cursor: 2,
			Phase: rpc.RouteViewPhaseSnapshot, Complete: true, Entries: []rpc.RouteViewEntry{entry}}); err != nil {
			t.Fatal(err)
		}
		projection.Inspect(family.IPv4Unicast, prefix, func(value *selectedProjectionPath) {
			prepared, prepareErr = prepareRouteViewSelectedMetadata(&locrib.Change{Kind: locrib.ChangeAdd,
				Family: family.IPv4Unicast, Prefix: prefix, Best: value.path, ECMP: value.path.ECMP}, value.members)
		})
	} else {
		rib.Inspect(family.IPv4Unicast, prefix, func(group locrib.PathGroup) {
			prepared, prepareErr = prepareRouteViewLocal(family.IPv4Unicast, prefix, &group)
		})
	}
	if prepareErr != nil {
		t.Fatal(prepareErr)
	}
	if prepared == nil {
		t.Fatal("real authority produced no selected value")
	}
	return prepared
}

func TestRouteViewCrossSourceMembersRetainPermissions(t *testing.T) {
	for _, projected := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "projection"}[projected], func(t *testing.T) {
			rib := locrib.NewRIB()
			prefix := netip.MustParsePrefix("198.18.244.0/24")
			primary := crossSourceCandidate("bgp", "primary", 101)
			member := crossSourceCandidate("ospf", "member", 201)
			rib.Insert(family.IPv4Unicast, prefix, primary)
			state := newSysRIB()
			state.resolver.Store(newNHResolver(rib))
			state.applyFIBImport(map[string]bool{"bgp": true, "ospf": true})
			batches, err := state.applySelectedBatch([]routeViewSelectedChange{selectedFromAuthority(t, rib, prefix, projected)})
			if err != nil {
				t.Fatal(err)
			}
			initial := batches[family.IPv4Unicast]
			if len(initial) != 1 || initial[0].Action != routeaction.Add || initial[0].Protocol != "bgp" || initial[0].Interface != "primary" {
				t.Fatalf("healthy primary control: %+v", initial)
			}
			rib.Insert(family.IPv4Unicast, prefix, member)
			batches, err = state.applySelectedBatch([]routeViewSelectedChange{selectedFromAuthority(t, rib, prefix, projected)})
			if err != nil {
				t.Fatal(err)
			}
			withMember := batches[family.IPv4Unicast]
			if len(withMember) != 1 || withMember[0].Action != routeaction.Update || len(withMember[0].ECMPPaths) != 1 || withMember[0].ECMPPaths[0].Interface != "member" || !slices.Equal(withMember[0].ECMPPaths[0].Labels, []uint32{201}) {
				t.Fatalf("authoritative cross-source member never reached forwarding: %+v", withMember)
			}
			withheld := state.applyFIBImport(map[string]bool{"bgp": true, "ospf": false})[family.IPv4Unicast]
			if len(withheld) != 1 || withheld[0].Action != routeaction.Update || len(withheld[0].ECMPPaths) != 0 || withheld[0].Protocol != "bgp" {
				t.Fatalf("withholding a member changed prefix ownership or retained its target: %+v", withheld)
			}
			restored := state.applyFIBImport(map[string]bool{"bgp": true, "ospf": true})[family.IPv4Unicast]
			if len(restored) != 1 || restored[0].Action != routeaction.Update || len(restored[0].ECMPPaths) != 1 || restored[0].ECMPPaths[0].Interface != "member" {
				t.Fatalf("permitting the owner required producer reannouncement: %+v", restored)
			}
			replayed := state.replayBatches(&replay.Request{ReplayID: replay.Broadcast})
			if len(replayed) != 1 || len(replayed[0].Changes) != 1 || len(replayed[0].Changes[0].ECMPPaths) != 1 || replayed[0].Changes[0].ECMPPaths[0].Interface != "member" {
				t.Fatalf("replay lost permitted source ownership: %+v", replayed)
			}
			group, found := rib.Lookup(family.IPv4Unicast, prefix)
			if !found || len(group.Paths) != 2 {
				t.Fatal("FIB permission erased an authoritative candidate")
			}
		})
	}
}

func TestRouteViewCrossSourceDuplicateTargetsKeepPermittedOwner(t *testing.T) {
	for _, projected := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "projection"}[projected], func(t *testing.T) {
			rib := locrib.NewRIB()
			prefix := netip.MustParsePrefix("198.18.245.0/24")
			rib.Insert(family.IPv4Unicast, prefix, crossSourceCandidate("bgp", "primary", 101))
			rib.Insert(family.IPv4Unicast, prefix, crossSourceCandidate("ospf", "duplicate", 201))
			rib.Insert(family.IPv4Unicast, prefix, crossSourceCandidate("isis", "duplicate", 301))
			state := newSysRIB()
			state.resolver.Store(newNHResolver(rib))
			state.applyFIBImport(map[string]bool{"bgp": true, "ospf": false, "isis": true})
			batches, err := state.applySelectedBatch([]routeViewSelectedChange{selectedFromAuthority(t, rib, prefix, projected)})
			if err != nil {
				t.Fatal(err)
			}
			changes := batches[family.IPv4Unicast]
			if len(changes) != 1 || changes[0].Protocol != "bgp" || len(changes[0].ECMPPaths) != 1 || !slices.Equal(changes[0].ECMPPaths[0].Labels, []uint32{301}) {
				t.Fatalf("withheld duplicate hid the permitted owner's complete path: %+v", changes)
			}
			swapped := state.applyFIBImport(map[string]bool{"bgp": true, "ospf": true, "isis": false})[family.IPv4Unicast]
			if len(swapped) != 1 || swapped[0].Action != routeaction.Update || len(swapped[0].ECMPPaths) != 1 || !slices.Equal(swapped[0].ECMPPaths[0].Labels, []uint32{201}) {
				t.Fatalf("owner-only permission change lost identical target replacement metadata: %+v", swapped)
			}
		})
	}
}

func TestRouteViewProjectionRefusesUnknownMemberOwner(t *testing.T) {
	redistevents.RegisterProtocol("static")
	projection, err := newSelectedProjection(rpc.RouteViewOpenOutput{Session: "member-owner", Epoch: "daemon", Snapshot: "initial"})
	if err != nil {
		t.Fatal(err)
	}
	prefix := netip.MustParsePrefix("198.18.246.0/24")
	entry := runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 10, Interface: "primary", ECMP: []rpc.RouteViewNextHop{{Interface: "member"}}})
	if _, err := projection.Apply(&rpc.RouteViewReadOutput{Session: "member-owner", Epoch: "daemon", Snapshot: "initial", Cursor: 2,
		Phase: rpc.RouteViewPhaseSnapshot, Complete: true, Entries: []rpc.RouteViewEntry{entry}}); err != nil {
		t.Fatalf("healthy primary-owned member refused: %v", err)
	}
	entry.Path.ECMP[0].Protocol = "unregistered-member-owner"
	if _, err := projection.Apply(&rpc.RouteViewReadOutput{Session: "member-owner", Epoch: "daemon", Cursor: 3,
		Phase: rpc.RouteViewPhaseDelta, Complete: true, Entries: []rpc.RouteViewEntry{entry}}); err == nil {
		t.Fatal("projection accepted a member owner absent from its registered protocol identities")
	}
	if projection.Cursor() != 2 {
		t.Fatal("refused member owner advanced the selected cursor")
	}
	if _, _, found := projection.LPM(family.IPv4Unicast, prefix.Addr()); !found {
		t.Fatal("member-owner refusal destroyed the healthy selected primary")
	}
}

func TestRouteViewCrossSourceMemberAtPrimaryTarget(t *testing.T) {
	for _, projected := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "projection"}[projected], func(t *testing.T) {
			rib := locrib.NewRIB()
			prefix := netip.MustParsePrefix("198.18.250.0/24")
			primary := crossSourceCandidate("bgp", "shared", 101)
			member := crossSourceCandidate("ospf", "shared", 201)
			rib.Insert(family.IPv4Unicast, prefix, primary)
			state := newSysRIB()
			state.resolver.Store(newNHResolver(rib))
			state.applyFIBImport(map[string]bool{"bgp": true, "ospf": true})
			initial, err := state.applySelectedBatch([]routeViewSelectedChange{selectedFromAuthority(t, rib, prefix, projected)})
			if err != nil {
				t.Fatal(err)
			}
			if changes := initial[family.IPv4Unicast]; len(changes) != 1 || changes[0].Interface != "shared" || len(changes[0].ECMPPaths) != 0 || !slices.Equal(changes[0].Labels, primary.Labels) {
				t.Fatalf("healthy primary control: %+v", changes)
			}
			rib.Insert(family.IPv4Unicast, prefix, member)
			batches, err := state.applySelectedBatch([]routeViewSelectedChange{selectedFromAuthority(t, rib, prefix, projected)})
			if err != nil {
				t.Fatal(err)
			}
			changes := batches[family.IPv4Unicast]
			if len(changes) != 1 || changes[0].Action != routeaction.Update || changes[0].Protocol != "bgp" || len(changes[0].ECMPPaths) != 1 || changes[0].ECMPPaths[0].Interface != "shared" || !slices.Equal(changes[0].ECMPPaths[0].Labels, member.Labels) || !slices.Equal(changes[0].Labels, primary.Labels) {
				t.Fatalf("another owner at the primary target lost its path: %+v", changes)
			}
			withheld := state.applyFIBImport(map[string]bool{"bgp": true, "ospf": false})[family.IPv4Unicast]
			if len(withheld) != 1 || withheld[0].Action != routeaction.Update || len(withheld[0].ECMPPaths) != 0 || !slices.Equal(withheld[0].Labels, primary.Labels) {
				t.Fatalf("member permission changed the primary target's own metadata: %+v", withheld)
			}
			restored := state.applyFIBImport(map[string]bool{"bgp": true, "ospf": true})[family.IPv4Unicast]
			if len(restored) != 1 || restored[0].Action != routeaction.Update || len(restored[0].ECMPPaths) != 1 || !slices.Equal(restored[0].ECMPPaths[0].Labels, member.Labels) {
				t.Fatalf("primary-equivalent member needed reannouncement: %+v", restored)
			}
		})
	}
}

func TestRouteViewWithheldMemberReleasesDependencies(t *testing.T) {
	for _, projected := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "projection"}[projected], func(t *testing.T) {
			rib := locrib.NewRIB()
			prefix := netip.MustParsePrefix("198.18.251.0/24")
			primary := crossSourceCandidate("bgp", "primary", 101)
			member := crossSourceCandidate("ospf", "member", 201)
			member.NextHop = netip.MustParseAddr("192.0.2.251")
			member.SRv6SID = netip.MustParseAddr("2001:db8:251::1")
			rib.Insert(family.IPv4Unicast, prefix, primary)
			rib.Insert(family.IPv4Unicast, prefix, member)
			rib.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), crossSourceCandidate("connected", "member-link", 0))
			rib.Insert(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:251::/64"), crossSourceCandidate("connected", "sid-link", 0))
			state := newSysRIB()
			resolver := newNHResolver(rib)
			state.resolver.Store(resolver)
			state.applyFIBImport(map[string]bool{"bgp": true, "ospf": true})
			batches, err := state.applySelectedBatch([]routeViewSelectedChange{selectedFromAuthority(t, rib, prefix, projected)})
			if err != nil {
				t.Fatal(err)
			}
			if changes := batches[family.IPv4Unicast]; len(changes) != 1 || len(changes[0].ECMPPaths) != 1 {
				t.Fatalf("healthy permitted member control: %+v", changes)
			}
			for _, dependency := range []netip.Addr{member.NextHop, member.SRv6SID} {
				if !slices.Contains(resolver.Dependents(dependency), prefix) {
					t.Fatalf("permitted member has no dependency on %s", dependency)
				}
			}
			withheld := state.applyFIBImport(map[string]bool{"bgp": true, "ospf": false})[family.IPv4Unicast]
			if len(withheld) != 1 || len(withheld[0].ECMPPaths) != 0 || withheld[0].Interface != "primary" {
				t.Fatalf("withholding member did not preserve the primary: %+v", withheld)
			}
			for _, dependency := range []netip.Addr{member.NextHop, member.SRv6SID} {
				if slices.Contains(resolver.Dependents(dependency), prefix) {
					t.Errorf("withheld member retained dependency on %s", dependency)
				}
			}
			restored := state.applyFIBImport(map[string]bool{"bgp": true, "ospf": true})[family.IPv4Unicast]
			if len(restored) != 1 || len(restored[0].ECMPPaths) != 1 {
				t.Fatalf("permitting member did not recover forwarding: %+v", restored)
			}
			for _, dependency := range []netip.Addr{member.NextHop, member.SRv6SID} {
				if !slices.Contains(resolver.Dependents(dependency), prefix) {
					t.Errorf("permitting member did not restore dependency on %s", dependency)
				}
			}
			rib.Remove(family.IPv4Unicast, prefix, member.Source, member.Instance)
			if _, err := state.applySelectedBatch([]routeViewSelectedChange{selectedFromAuthority(t, rib, prefix, projected)}); err != nil {
				t.Fatal(err)
			}
			for _, dependency := range []netip.Addr{member.NextHop, member.SRv6SID} {
				if slices.Contains(resolver.Dependents(dependency), prefix) {
					t.Errorf("removed member retained dependency on %s", dependency)
				}
			}
		})
	}
}
