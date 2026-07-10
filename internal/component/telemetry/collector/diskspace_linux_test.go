// VALIDATES: the diskspace collector parses /proc/mounts (filtering to real
// filesystems), statfs's each mountpoint, and emits avail/used/reserved GiB
// gauges keyed by the sanitized mountpoint (mounts path injected via seam).
// PREVENTS: the real-FS filter dropping a valid mount, or the used/avail/reserved
// arithmetic going negative or unrecorded.

//go:build linux

package collector

import "testing"

func TestDiskSpaceCollect(t *testing.T) {
	mp := t.TempDir() // a real, statfs-able mountpoint
	// One real (ext4) mount plus a pseudo-fs that must be filtered out.
	_, mounts := tmpFile(t, "mounts",
		"/dev/sda1 "+mp+" ext4 rw,relatime 0 0\n"+
			"proc /proc proc rw 0 0\n")

	c := newDiskSpaceCollector()
	c.mountsPath = mounts
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	const name = "netdata_disk_space_GiB_average"
	chart := "disk_space." + sanitizeMountpoint(mp)

	// statfs returns the real host FS numbers (non-deterministic), so assert the
	// gauges are recorded and internally consistent rather than exact values.
	used, ok := reg.gaugeOK(name, chart, "used", mp)
	if !ok {
		t.Fatalf("used gauge for %q not recorded", mp)
	}
	if used < 0 {
		t.Errorf("used = %v, want >= 0", used)
	}
	if avail, ok := reg.gaugeOK(name, chart, "avail", mp); !ok || avail < 0 {
		t.Errorf("avail gauge missing or negative: %v (ok=%v)", avail, ok)
	}
	if reserved, ok := reg.gaugeOK(name, chart, "reserved_for_root", mp); !ok || reserved < 0 {
		t.Errorf("reserved gauge missing or negative: %v (ok=%v)", reserved, ok)
	}

	// The proc pseudo-fs must be filtered out (never statfs'd / recorded).
	if _, ok := reg.gaugeOK(name, "disk_space._proc", "used", "/proc"); ok {
		t.Error("pseudo-fs /proc should be filtered out")
	}
}
