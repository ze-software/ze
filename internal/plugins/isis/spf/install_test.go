// Design: docs/architecture/isis/isis-9-spf-rib.md TDD plan -- Loc-RIB insertion + wiring.
//
// VALIDATES: the SPF result is INSERTED into the shared Loc-RIB as
// locrib.Path{Source = IS-IS ProtocolID, Instance (per ECMP next-hop),
// AdminDistance = 115, Metric} via InsertForward (mirroring BGP
// rib_bestchange.go:813), with one Path per equal-cost next-hop (distinct
// Instance); a lost prefix is forward-removed; and the end-to-end wiring (SPF run
// -> Loc-RIB insertion) installs the expected paths (the umbrella Wiring Test row
// "SPF result change -> InsertForward").
// PREVENTS: a regression where IS-IS routes never reach the Loc-RIB, use the
// wrong Source/AdminDistance, collapse ECMP to one Path, or leave stale paths
// after withdrawal (R-4).

package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// lookupPaths returns the Loc-RIB PathGroup paths for a prefix (IPv4 unicast).
func lookupPaths(t *testing.T, loc *locrib.RIB, pfx netip.Prefix) []locrib.Path {
	t.Helper()
	g, ok := loc.Lookup(family.IPv4Unicast, pfx)
	if !ok {
		return nil
	}
	return g.Paths
}

// TestISISInstallPath verifies that Apply inserts one locrib.Path per equal-cost
// next-hop with the IS-IS Source and AdminDistance 115, and that a withdrawal
// forward-removes every inserted Path.
func TestISISInstallPath(t *testing.T) {
	loc := locrib.NewRIB()
	in := NewInstaller(loc)

	pfx := netip.MustParsePrefix("10.20.0.0/24")
	nh1 := netip.MustParseAddr("10.0.0.1")
	nh2 := netip.MustParseAddr("10.0.0.2")

	// Two equal-cost next-hops -> two Paths, distinct Instance.
	in.Apply([]RouteEntry{{
		Prefix: pfx, Metric: 25, Level: Level1,
		NextHops: []NextHop{{Addr: nh1, Interface: "eth0"}, {Addr: nh2, Interface: "eth1"}},
	}})

	paths := lookupPaths(t, loc, pfx)
	if len(paths) != 2 {
		t.Fatalf("inserted %d Paths, want 2 (one per ECMP next-hop)", len(paths))
	}
	wantNH := map[netip.Addr]bool{nh1: false, nh2: false}
	instances := map[uint32]bool{}
	for _, p := range paths {
		if name := redistevents.ProtocolName(p.Source); name != "isis" {
			t.Errorf("Path Source = %q, want isis", name)
		}
		if p.AdminDistance != DefaultAdminDistance {
			t.Errorf("Path AdminDistance = %d, want %d", p.AdminDistance, DefaultAdminDistance)
		}
		if p.Metric != 25 {
			t.Errorf("Path Metric = %d, want 25", p.Metric)
		}
		if _, ok := wantNH[p.NextHop]; !ok {
			t.Errorf("unexpected Path next-hop %s", p.NextHop)
		}
		wantNH[p.NextHop] = true
		if instances[p.Instance] {
			t.Errorf("duplicate Instance %d (ECMP next-hops must differ)", p.Instance)
		}
		instances[p.Instance] = true
	}
	for nh, seen := range wantNH {
		if !seen {
			t.Errorf("next-hop %s not inserted", nh)
		}
	}

	// Withdraw the prefix: both Paths forward-removed.
	in.Apply(nil)
	if paths := lookupPaths(t, loc, pfx); len(paths) != 0 {
		t.Errorf("after withdraw, %d Paths remain, want 0", len(paths))
	}
}

// TestISISInstallShrinkECMP verifies that when an ECMP set shrinks from two
// next-hops to one, the dropped Instance's Path is removed (no stale next-hop).
func TestISISInstallShrinkECMP(t *testing.T) {
	loc := locrib.NewRIB()
	in := NewInstaller(loc)
	pfx := netip.MustParsePrefix("10.21.0.0/24")
	nh1 := netip.MustParseAddr("10.0.0.1")
	nh2 := netip.MustParseAddr("10.0.0.2")

	in.Apply([]RouteEntry{{
		Prefix: pfx, Metric: 10, Level: Level1,
		NextHops: []NextHop{{Addr: nh1}, {Addr: nh2}},
	}})
	if len(lookupPaths(t, loc, pfx)) != 2 {
		t.Fatalf("setup: want 2 Paths")
	}

	// Shrink to a single next-hop.
	in.Apply([]RouteEntry{{
		Prefix: pfx, Metric: 10, Level: Level1,
		NextHops: []NextHop{{Addr: nh1}},
	}})
	paths := lookupPaths(t, loc, pfx)
	if len(paths) != 1 {
		t.Fatalf("after shrink, %d Paths, want 1", len(paths))
	}
	if paths[0].NextHop != nh1 {
		t.Errorf("remaining next-hop = %s, want %s", paths[0].NextHop, nh1)
	}
}

// TestISISSPFRoute is the wiring test (umbrella Wiring Test row): an LSDB change
// -> SPF run -> Loc-RIB insertion. It drives a Computer over a two-node topology
// where node 2 originates a prefix, and asserts the prefix lands in the Loc-RIB
// with the IS-IS Source after Run.
func TestISISSPFRoute(t *testing.T) {
	loc := locrib.NewRIB()
	src := newStubSource()
	a, b := srcID(1), srcID(2)
	src.bidir(a, b, 10)
	// Node 2 originates 10.30.0.0/24 at prefix metric 5.
	for i := range src.byLevel[Level1] {
		if src.byLevel[Level1][i].Source == b {
			src.byLevel[Level1][i].LSP.TLVs = append(src.byLevel[Level1][i].LSP.TLVs,
				tlv135(netip.MustParsePrefix("10.30.0.0/24"), 5, false))
		}
	}

	c := NewComputer(Config{
		Source:    src,
		Resolver:  stubResolver{}, // node 2 -> 10.0.0.2 on eth0
		Root:      sysID(1),
		Levels:    []Level{Level1},
		Installer: NewInstaller(loc),
	})

	// Run SPF directly (the debounce timer would call this in production).
	delta := c.Run()
	if len(delta.Added) != 1 {
		t.Fatalf("delta.Added = %d, want 1 (the remote prefix)", len(delta.Added))
	}

	pfx := netip.MustParsePrefix("10.30.0.0/24")
	paths := lookupPaths(t, loc, pfx)
	if len(paths) != 1 {
		t.Fatalf("Loc-RIB has %d Paths for %s, want 1", len(paths), pfx)
	}
	if name := redistevents.ProtocolName(paths[0].Source); name != "isis" {
		t.Errorf("Source = %q, want isis", name)
	}
	if paths[0].NextHop.String() != "10.0.0.2" {
		t.Errorf("next-hop = %s, want 10.0.0.2 (node 2's address)", paths[0].NextHop)
	}
	// Total metric = node distance 10 + prefix metric 5 = 15.
	if paths[0].Metric != 15 {
		t.Errorf("Metric = %d, want 15 (10 + 5)", paths[0].Metric)
	}

	// Snapshot reflects the installed route.
	snap := c.Snapshot()
	if len(snap) != 1 || snap[0].Prefix != "10.30.0.0/24" || snap[0].Level != "l1" {
		t.Errorf("snapshot = %+v, want one l1 route for 10.30.0.0/24", snap)
	}

	// Stop forward-removes it.
	c.Stop()
	if len(lookupPaths(t, loc, pfx)) != 0 {
		t.Errorf("after Stop, Paths remain for %s", pfx)
	}
}

// TestISISInstallNilLocRIB verifies a nil Loc-RIB (forked subprocess) makes
// install a no-op without panicking, while the installed set is still tracked for
// the snapshot.
func TestISISInstallNilLocRIB(t *testing.T) {
	in := NewInstaller(nil)
	pfx := netip.MustParsePrefix("10.40.0.0/24")
	in.Apply([]RouteEntry{{
		Prefix: pfx, Metric: 10, Level: Level1,
		NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.1")}},
	}})
	if got := in.Installed(); len(got) != 1 {
		t.Errorf("installed set = %d, want 1 (tracked even with nil Loc-RIB)", len(got))
	}
}
