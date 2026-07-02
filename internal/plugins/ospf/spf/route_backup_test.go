// VALIDATES: AC-12/R-11 (a backup-only change re-installs via routeEqual/
// DiffRoutes) and AC-15 (FastRerouteSnapshot surfaces backup + class + repair
// stack while the base Snapshot is unchanged).
// PREVENTS: a stale backup after a topology change that did not move the primary.
package spf

import (
	"net/netip"
	"testing"
)

func TestBackupOnlyChangeReinstalls(t *testing.T) {
	pfx := netip.MustParsePrefix("192.0.2.0/24")
	base := RouteEntry{
		AreaID: testArea(), Prefix: pfx, Metric: 10, Type: RouteIntraArea, Origin: testRID(t, "2.2.2.2"),
		NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}},
		Backups:  []Backup{{NextHop: netip.MustParseAddr("10.0.0.3"), Kind: BackupLFA, LinkProtect: true}},
	}
	changed := base
	changed.Backups = []Backup{{NextHop: netip.MustParseAddr("10.0.0.4"), Kind: BackupLFA, LinkProtect: true}}

	if routeEqual(base, changed) {
		t.Fatal("routeEqual ignored a backup-only change")
	}
	d := DiffRoutes(IndexByPrefix([]RouteEntry{base}), IndexByPrefix([]RouteEntry{changed}))
	if len(d.Changed) != 1 {
		t.Fatalf("backup-only change not in delta: %+v", d)
	}

	// A repair-label-only change (same backup next-hop) also re-installs.
	relabel := base
	relabel.Backups = []Backup{{NextHop: netip.MustParseAddr("10.0.0.3"), RepairLabels: []uint32{16010}, Kind: BackupTILFA, LinkProtect: true}}
	if routeEqual(base, relabel) {
		t.Fatal("routeEqual ignored a repair-label change")
	}
}

func TestRouteEntryBackupSnapshot(t *testing.T) {
	r := RouteEntry{
		AreaID: testArea(), Prefix: netip.MustParsePrefix("192.0.2.0/24"), Metric: 11, Type: RouteIntraArea, Origin: testRID(t, "2.2.2.2"),
		NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.12.2")}},
		Backups:  []Backup{{NextHop: netip.MustParseAddr("10.0.13.3"), RepairLabels: []uint32{16010, 24003}, NodeProtect: true, LinkProtect: true, Kind: BackupTILFA}},
	}
	snap := FastRerouteSnapshot([]RouteEntry{r})
	if len(snap) != 1 || len(snap[0].NextHops) != 1 {
		t.Fatalf("snapshot shape = %+v", snap)
	}
	h := snap[0].NextHops[0]
	if !h.Protected || h.Backup != "10.0.13.3" || h.BackupClass != "node+link" {
		t.Fatalf("backup snapshot fields = %+v", h)
	}
	if len(h.RepairLabels) != 2 || h.RepairLabels[0] != 16010 || h.RepairLabels[1] != 24003 {
		t.Fatalf("repair labels = %+v", h.RepairLabels)
	}

	// The base `show ospf route` snapshot never carries backup fields.
	base := Snapshot([]RouteEntry{r})
	if len(base) != 1 || len(base[0].NextHops) != 1 {
		t.Fatalf("base snapshot shape = %+v", base)
	}
	if base[0].NextHops[0].NextHop != "10.0.12.2" {
		t.Fatalf("base snapshot next-hop = %q", base[0].NextHops[0].NextHop)
	}
}
