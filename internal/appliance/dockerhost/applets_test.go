package dockerhost

import (
	"slices"
	"testing"
)

// TestAppletLinksStayInTheTmpfsDirs feeds a `busybox --list-full` listing
// that names an applet at the root (linuxrc) and asserts only the applets in
// the tmpfs-covered directories are linked.
//
// VALIDATES: spec Phase 1 boot step 8: busybox links land only where the boot
// mounted a writable tmpfs.
// PREVENTS: the boot failing on `symlink /linuxrc: read-only file system`,
// seen on the first vz boot (2026-10-21).
func TestAppletLinksStayInTheTmpfsDirs(t *testing.T) {
	list := "bin/sh\nlinuxrc\nsbin/ip\nusr/bin/awk\nusr/sbin/chpasswd\n\nusr/local/bin/x\n"
	got := appletLinks(list)
	want := []string{"/bin/sh", "/sbin/ip", "/usr/bin/awk", "/usr/sbin/chpasswd"}
	if !slices.Equal(got, want) {
		t.Errorf("appletLinks = %q, want %q", got, want)
	}
}
