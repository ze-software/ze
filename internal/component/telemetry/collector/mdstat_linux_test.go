// VALIDATES: the mdstat collector parses /proc/mdstat and emits per-array disk
// health, disk counts, and mismatch (unsynchronized) blocks on the right gauges.
// PREVENTS: mislabeled or mis-counted RAID array health reaching the metrics
// surface (e.g. counting active as total, or a stale sync figure).

//go:build linux

package collector

import "testing"

func TestMDStatCollect(t *testing.T) {
	fs := procFixture(t, map[string]string{
		"mdstat": "Personalities : [raid1]\n" +
			"md0 : active raid1 sda1[0] sdb1[1]\n" +
			"      248896 blocks [2/2] [UU]\n" +
			"\n" +
			"unused devices: <none>\n",
	})
	reg := newRecordingRegistry()
	c := newMDStatCollector(fs)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	const chart = "md.md0"
	if got := reg.gauge(t, "netdata_md_disks_disks_average", chart, "inuse", "md0"); got != 2 {
		t.Errorf("disks inuse = %v, want 2", got)
	}
	if got := reg.gauge(t, "netdata_md_disks_disks_average", chart, "down", "md0"); got != 0 {
		t.Errorf("disks down = %v, want 0", got)
	}
	if got := reg.gauge(t, "netdata_md_health_disks_average", chart, "total", "md0"); got != 2 {
		t.Errorf("health total = %v, want 2", got)
	}
	if got := reg.gauge(t, "netdata_md_health_disks_average", chart, "failed", "md0"); got != 0 {
		t.Errorf("health failed = %v, want 0", got)
	}
	// Fully synced array: unsynchronized blocks must be zero.
	if got := reg.gauge(t, "netdata_md_mismatch_cnt_unsynchronized_blocks_average", chart, "unsynchronized", "md0"); got != 0 {
		t.Errorf("unsynchronized = %v, want 0", got)
	}
}
