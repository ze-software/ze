package runner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPortLockDirectoryOthersCanWriteIsRefusedByName proves that an existing
// lock directory another user could write is refused, and that the refusal
// names the directory, its owner and its mode.
//
// VALIDATES: reservePortLocksIn checks an existing directory before using it.
// PREVENTS: the 2026-10-07 lockout, read as "open port lock 2020: permission
// denied" with no word on which directory was wrong or why
// (plan/journal/gate-verdict-depends-on-the-machine.md). A directory others
// can write lets them unlink a held lock file, and a replacement inode then
// admits a second holder.
func TestPortLockDirectoryOthersCanWriteIsRefusedByName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "locks")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	reservation, ok, err := reservePortLocksIn(dir, 41000, 1)
	if reservation != nil {
		reservation.Release()
	}
	if err == nil || ok {
		t.Fatalf("a world-writable lock directory was used: ok=%t err=%v", ok, err)
	}
	for _, want := range []string{dir, "uid " + strconv.Itoa(os.Geteuid()), "drwxrwxrwx"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}
}

// TestPortLockDirectorySymlinkIsRefused proves the directory is judged as
// itself, never through a link another user planted at the fixed path.
func TestPortLockDirectorySymlinkIsRefused(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "locks")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	reservation, ok, err := reservePortLocksIn(link, 41002, 1)
	if reservation != nil {
		reservation.Release()
	}
	if err == nil || ok || !strings.Contains(err.Error(), link) {
		t.Fatalf("a symlinked lock directory was used: ok=%t err=%v", ok, err)
	}
}

// TestPortLockDirectoryOwnPrivateIsUsed proves the check admits the directory
// the runner itself creates.
func TestPortLockDirectoryOwnPrivateIsUsed(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "locks")
	reservation, ok, err := reservePortLocksIn(dir, 41004, 2)
	if err != nil || !ok {
		t.Fatalf("own private lock directory refused: ok=%t err=%v", ok, err)
	}
	reservation.Release()
}
