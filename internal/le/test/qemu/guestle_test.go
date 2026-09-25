package testqemu

import (
	"os"
	"path/filepath"
	"testing"
)

// VALIDATES: guestLeLink writes dir/le pointing at the running executable, and
// replaces a link a previous run left.
// PREVENTS: a guest suite running a stale harness, or a harness reached under a
// name that does not select the le personality.
func TestGuestLeLinkPointsAtTheRunningExecutable(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("/nowhere/le", filepath.Join(dir, "le")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	link, err := guestLeLink(dir)
	if err != nil {
		t.Fatalf("guestLeLink: %v", err)
	}
	if link != filepath.Join(dir, "le") {
		t.Errorf("the link is %q, want %q", link, filepath.Join(dir, "le"))
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if target, err := os.Readlink(link); err != nil || target != self {
		t.Errorf("the link points at %q (%v), want %q", target, err, self)
	}
}
