// Design: docs/architecture/ospf/ospf-ext-6-ti-lfa.md -- Path backup carry-through (AC-13,
// A-2, R-10). The fast-reroute backup next-hop + repair labels are carry-through
// metadata: excluded from the arbitration key and from best-path selection
// (AdminDistance then Metric), but compared by Equal so a backup-only change
// re-programs the FIB (the same contract as Labels).
package locrib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: AC-13/R-10 -- a Path's backup never enters the arbitration key and
// never changes which path wins; it IS detected by Equal so the FIB re-programs.
func TestLocribPathBackupCarriedNotArbitrated(t *testing.T) {
	nh := netip.MustParseAddr("10.0.0.2")
	withBackup := Path{
		Source: 5, Instance: 0, NextHop: nh, AdminDistance: 110, Metric: 10,
		BackupNextHop: netip.MustParseAddr("10.0.0.9"), BackupRepairLabels: []uint32{16010},
	}
	noBackup := withBackup
	noBackup.BackupNextHop = netip.Addr{}
	noBackup.BackupRepairLabels = nil

	// Excluded from the (Source, Instance) key.
	assert.Equal(t, withBackup.key(), noBackup.key(), "backup must not enter the arbitration key")

	// Included in Equal so a backup-only change re-programs the FIB.
	assert.False(t, withBackup.Equal(noBackup), "a backup change must be a detected change")
	relabel := withBackup
	relabel.BackupRepairLabels = []uint32{16020}
	assert.False(t, withBackup.Equal(relabel), "a repair-label change must be detected")

	// Arbitration is AdminDistance then Metric, indifferent to the backup: a
	// lower-distance path from another protocol wins even though the OSPF path
	// carries a backup.
	static := Path{Source: 6, Instance: 0, NextHop: netip.MustParseAddr("10.0.0.1"), AdminDistance: 1, Metric: 99}
	assert.Equal(t, 1, selectBest([]Path{withBackup, static}), "backup must not change best-path selection")

	// Between two OSPF instances the lower Metric wins regardless of backups.
	a := Path{Source: 5, Instance: 0, NextHop: nh, AdminDistance: 110, Metric: 5, BackupNextHop: netip.MustParseAddr("10.0.0.9")}
	b := Path{Source: 5, Instance: 1, NextHop: nh, AdminDistance: 110, Metric: 3}
	assert.Equal(t, 1, selectBest([]Path{a, b}), "lower Metric wins; backup irrelevant to arbitration")
}
