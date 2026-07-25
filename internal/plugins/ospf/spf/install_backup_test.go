// VALIDATES: AC-11 -- Installer.insert populates the locrib.Path backup fields
// (BackupNextHop + BackupRepairLabels) per primary next-hop, and clears them when
// the route loses its backup.
// PREVENTS: the FIB never learning the backup, or a stale backup after it is gone.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/rib/locrib"
)

func TestInstallerSetsBackupPath(t *testing.T) {
	loc := locrib.NewRIB()
	in := NewInstaller(loc)
	pfx := netip.MustParsePrefix("10.20.0.0/24")

	in.Apply([]RouteEntry{{
		AreaID: testArea(), Prefix: pfx, Metric: 42, Type: RouteIntraArea, Origin: testRID(t, "2.2.2.2"),
		NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}},
		Backups:  []Backup{{NextHop: netip.MustParseAddr("10.0.0.9"), RepairLabels: []uint32{16010, 24003}, Kind: BackupTILFA, NodeProtect: true, LinkProtect: true}},
	}})
	paths := lookupPaths(t, loc, pfx)
	if len(paths) != 1 {
		t.Fatalf("want 1 path, got %d", len(paths))
	}
	if paths[0].BackupNextHop != netip.MustParseAddr("10.0.0.9") {
		t.Fatalf("BackupNextHop = %v, want 10.0.0.9", paths[0].BackupNextHop)
	}
	if len(paths[0].BackupRepairLabels) != 2 || paths[0].BackupRepairLabels[0] != 16010 || paths[0].BackupRepairLabels[1] != 24003 {
		t.Fatalf("BackupRepairLabels = %v, want [16010 24003]", paths[0].BackupRepairLabels)
	}

	// Re-apply without a backup: the backup fields are cleared (a backup-only
	// change re-installs the path).
	in.Apply([]RouteEntry{{
		AreaID: testArea(), Prefix: pfx, Metric: 42, Type: RouteIntraArea, Origin: testRID(t, "2.2.2.2"),
		NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}},
	}})
	paths = lookupPaths(t, loc, pfx)
	if len(paths) != 1 || paths[0].BackupNextHop.IsValid() {
		t.Fatalf("backup not cleared: %+v", paths)
	}
}
