package dockerhost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestModprobeIndexDirIsWritable runs mountWritableTrees over a temp root and
// asserts /lib/modules is covered by an overlay whose upper layer sits on the
// /run tmpfs, beside the /usr/local one, with both layers' directories made.
//
// VALIDATES: spec R-1 (handover 13, open item 1). The image's busybox
// modprobe (modprobe-small) reads every module under /lib/modules unless it
// finds its modules.dep.bb index, and writes that index on its first call.
// On the read-only squashfs the write failed, so each of the 21 calls in
// Lima's 00-modprobe.sh scanned again: 64 to 89 s before the guest agent.
// Busybox ash runs its own modprobe applet in-process, whatever /sbin holds,
// so the index is the fix and a modprobe in front of busybox is not.
// PREVENTS: the module tree going back to read-only, which no unit test of
// busybox would show.
func TestModprobeIndexDirIsWritable(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "perm"), 0o755); err != nil {
		t.Fatal(err)
	}
	sys := &recordingSystem{root: root}
	if err := mountWritableTrees(sys, root); err != nil {
		t.Fatalf("mountWritableTrees: %v", err)
	}
	for _, dir := range []string{"/usr/local", "/lib/modules"} {
		m, ok := sys.mounted(dir)
		if !ok {
			t.Errorf("no mount on %s; mounts %+v", dir, sys.mounts)
			continue
		}
		if m.FSType != "overlay" {
			t.Errorf("%s mounted as %q, want overlay", dir, m.FSType)
		}
		if !strings.Contains(m.Data, "lowerdir="+dir+",") {
			t.Errorf("%s overlay data %q does not keep the squashfs tree as its lower layer", dir, m.Data)
		}
		for _, layer := range []string{"upperdir=/run/", "workdir=/run/"} {
			if !strings.Contains(m.Data, layer) {
				t.Errorf("%s overlay data %q has no %s layer on the /run tmpfs", dir, m.Data, layer)
			}
		}
		for option := range strings.SplitSeq(m.Data, ",") {
			layer, path, found := strings.Cut(option, "=")
			if !found || layer == "lowerdir" {
				continue
			}
			if info, err := os.Stat(filepath.Join(root, path)); err != nil || !info.IsDir() {
				t.Errorf("%s overlay %s %s was not created: %v", dir, layer, path, err)
			}
		}
	}
}
