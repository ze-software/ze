// VALIDATES: the btrfs collector walks a /sys/fs/btrfs UUID tree and emits disk
// allocation and per-type (data) used/free gauges in MiB (root injected via the
// c.root seam).
// PREVENTS: a wrong bytes-to-MiB conversion or a broken used/free/unallocated
// arithmetic on the btrfs.* metrics.

//go:build linux

package collector

import "testing"

func TestBtrfsCollect(t *testing.T) {
	dir := t.TempDir()
	const uuid = "11111111-2222-3333-4444-555555555555"
	// bytes. data: total 2 MiB, used 1 MiB. metadata: total 1 MiB, used 0.5 MiB.
	// system: total 1 MiB, used 0.
	for rel, content := range map[string]string{
		uuid + "/allocation/data/disk_total":     "2097152\n",
		uuid + "/allocation/data/disk_used":      "1048576\n",
		uuid + "/allocation/data/bytes_used":     "1048576\n",
		uuid + "/allocation/metadata/disk_total": "1048576\n",
		uuid + "/allocation/metadata/disk_used":  "524288\n",
		uuid + "/allocation/metadata/bytes_used": "524288\n",
		uuid + "/allocation/system/disk_total":   "1048576\n",
		uuid + "/allocation/system/disk_used":    "0\n",
		uuid + "/allocation/system/bytes_used":   "0\n",
	} {
		writeProcFile(t, dir, rel, content)
	}

	c := newBtrfsCollector()
	c.root = dir
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	// family = uuid[:8] since no label file.
	const fam = "11111111"
	const disk = "netdata_btrfs_disk_MiB_average"
	// diskUsed = 1 + 0.5 + 0 = 1.5 MiB
	if got := reg.gauge(t, disk, "btrfs."+fam, "used", fam); got != 1.5 {
		t.Errorf("disk used = %v, want 1.5", got)
	}
	// diskTotal = 2 + 1 + 1 = 4 MiB; unallocated = 4 - 1.5 = 2.5 MiB
	if got := reg.gauge(t, disk, "btrfs."+fam, "unallocated", fam); got != 2.5 {
		t.Errorf("disk unallocated = %v, want 2.5", got)
	}

	const data = "netdata_btrfs_data_MiB_average"
	if got := reg.gauge(t, data, "btrfs_data."+fam, "used", fam); got != 1 {
		t.Errorf("data used = %v, want 1", got)
	}
	// free = (2 MiB total - 1 MiB used) = 1 MiB
	if got := reg.gauge(t, data, "btrfs_data."+fam, "free", fam); got != 1 {
		t.Errorf("data free = %v, want 1", got)
	}
}
