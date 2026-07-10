// VALIDATES: the diskstats collector parses /proc/diskstats for physical disks
// and emits I/O bandwidth (sectors→KiB), operation rates and a system.io
// aggregate across two collects (path injected via the c.path seam).
// PREVENTS: a broken sector-to-KiB conversion, delta, or the physical-disk
// filter leaking partitions into the metrics surface.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func TestDiskStatsCollect(t *testing.T) {
	// major minor name rdOps rdMerged rdSect rdMs wrOps wrMerged wrSect wrMs inFlight ioMs weightedMs
	const first = "   8       0 sda 100 0 200 0 50 0 100 0 0 0 0\n"
	// deltas: rdOps 50, rdSect 200, wrOps 40, wrSect 200, inFlight 5
	const second = "   8       0 sda 150 0 400 0 90 0 300 5 5 0 0\n"

	dir, path := tmpFile(t, "diskstats", first)
	c := newDiskStatsCollector(time.Second)
	c.path = path
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "diskstats", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	// 200 sectors / 2 = 100 KiB/s.
	if got := reg.gauge(t, "netdata_disk_io_KiB_persec_average", "disk.sda", "reads", "sda"); got != 100 {
		t.Errorf("io reads = %v, want 100", got)
	}
	const ops = "netdata_disk_ops_operations_persec_average"
	if got := reg.gauge(t, ops, "disk.sda", "reads", "sda"); got != 50 {
		t.Errorf("ops reads = %v, want 50", got)
	}
	if got := reg.gauge(t, ops, "disk.sda", "writes", "sda"); got != 40 {
		t.Errorf("ops writes = %v, want 40", got)
	}
	// system.io aggregate: only physical disk sda contributes.
	if got := reg.gauge(t, "netdata_system_io_KiB_persec_average", "system.io", "in", "disk"); got != 100 {
		t.Errorf("system.io in = %v, want 100", got)
	}
}
