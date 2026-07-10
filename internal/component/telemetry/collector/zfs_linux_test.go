// VALIDATES: the zfs collector parses the ARC kstat file and emits ARC size (MiB)
// gauges plus ARC hit-rate deltas across two collects (path injected via the
// c.path seam).
// PREVENTS: a broken key lookup, bytes-to-MiB conversion, or hit-rate delta on
// the zfs.arc_size / zfs.hits_rate metrics.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func TestZFSCollect(t *testing.T) {
	// name type data. bytes: size 2 MiB, c 4 MiB, c_min 1 MiB, c_max 8 MiB.
	const first = "size 4 2097152\nc 4 4194304\nc_min 4 1048576\nc_max 4 8388608\n" +
		"hits 4 100\nmisses 4 20\n"
	// deltas: hits 200, misses 20.
	const second = "size 4 2097152\nc 4 4194304\nc_min 4 1048576\nc_max 4 8388608\n" +
		"hits 4 300\nmisses 4 40\n"

	dir, path := tmpFile(t, "arcstats", first)
	c := newZFSCollector(time.Second)
	c.path = path
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "arcstats", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	const size = "netdata_zfs_arc_size_MiB_average"
	if got := reg.gauge(t, size, "zfs.arc_size", "arcsz", "size"); got != 2 {
		t.Errorf("arcsz = %v, want 2", got)
	}
	if got := reg.gauge(t, size, "zfs.arc_size", "target", "size"); got != 4 {
		t.Errorf("target = %v, want 4", got)
	}
	if got := reg.gauge(t, size, "zfs.arc_size", "max", "size"); got != 8 {
		t.Errorf("max = %v, want 8", got)
	}
	if got := reg.gauge(t, "netdata_zfs_hits_rate_events_persec_average", "zfs.hits_rate", "hits", "hits"); got != 200 {
		t.Errorf("hits rate = %v, want 200", got)
	}
}
