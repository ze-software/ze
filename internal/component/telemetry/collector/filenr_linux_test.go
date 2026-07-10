// VALIDATES: the filenr collector parses /proc/sys/fs/file-nr and emits the
// used-file-descriptor count (allocated minus free) on the system.file_nr_used
// gauge (path injected via the c.path seam).
// PREVENTS: an off-by-a-column parse (e.g. reporting allocated or free instead
// of used) reaching the metrics surface.

//go:build linux

package collector

import "testing"

func TestFileNRCollect(t *testing.T) {
	// allocated=9600 free=64 max=100000 → used = 9600 - 64 = 9536
	_, path := tmpFile(t, "file-nr", "9600\t64\t100000\n")
	c := newFileNRCollector()
	c.path = path
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if got := reg.gauge(t, "netdata_system_file_nr_used_files_average", "system.file_nr_used", "used", "files"); got != 9536 {
		t.Errorf("used = %v, want 9536", got)
	}
}

func TestFileNRCollectMalformed(t *testing.T) {
	_, path := tmpFile(t, "file-nr", "9600\n") // too few fields
	c := newFileNRCollector()
	c.path = path
	c.Init(newRecordingRegistry(), "netdata")
	if err := c.Collect(); err == nil {
		t.Fatal("expected error on malformed file-nr")
	}
}
