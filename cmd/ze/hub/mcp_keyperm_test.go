//go:build ze_mcp

package hub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckKeyFilePermissions_RejectsSymlink verifies that a symlink at the
// key-file path is refused even when the target has acceptable 0o600 perms.
// A parent-directory writer could otherwise swap a strict-perm symlink
// pointing at a world-readable target to bypass the 0o077 mask.
func TestCheckKeyFilePermissions_RejectsSymlink(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("perm check disabled when running as root")
	}
	dir := t.TempDir()
	realKey := filepath.Join(dir, "real.key")
	if err := os.WriteFile(realKey, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write real key: %v", err)
	}
	link := filepath.Join(dir, "link.key")
	if err := os.Symlink(realKey, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	err := checkKeyFilePermissions(link)
	if err == nil {
		t.Fatal("expected rejection for symlink key file")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error should mention symlink, got: %v", err)
	}
}

// TestCheckKeyFilePermissions_RejectsWorldReadable verifies the 0o077 mask.
func TestCheckKeyFilePermissions_RejectsWorldReadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("perm check disabled when running as root")
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "loose.key")
	if err := os.WriteFile(key, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := checkKeyFilePermissions(key)
	if err == nil {
		t.Fatal("expected rejection for 0o644 key file")
	}
	if !strings.Contains(err.Error(), "group/other permissions") {
		t.Fatalf("error should mention group/other perms, got: %v", err)
	}
}

// TestCheckKeyFilePermissions_AcceptsStrict verifies the happy path.
func TestCheckKeyFilePermissions_AcceptsStrict(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("perm check disabled when running as root")
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "tight.key")
	if err := os.WriteFile(key, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := checkKeyFilePermissions(key); err != nil {
		t.Fatalf("0o600 key rejected: %v", err)
	}
}

// TestCheckKeyFilePermissions_RejectsNonRegular ensures non-regular files
// are refused. A directory exercises the `!IsRegular()` branch without a
// platform-specific mkfifo / mknod call; a FIFO or block device would hit
// the same branch.
func TestCheckKeyFilePermissions_RejectsNonRegular(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("perm check disabled when running as root")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sub := filepath.Join(dir, "notakey")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	err := checkKeyFilePermissions(sub)
	if err == nil {
		t.Fatal("expected rejection for non-regular key file (directory)")
	}
	if !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("error should mention regular file, got: %v", err)
	}
}

// TestCheckKeyFilePermissions_EnforcedForAllUsers runs unconditionally (no
// root skip) to verify symlink, non-regular, and group/other permission checks
// apply regardless of process UID.
//
// VALIDATES: S7 -- checkKeyFilePermissions enforces symlink and 0o077 mode
// checks even when running as root.
// PREVENTS: root daemons accepting world- or group-readable private keys or
// attacker-controlled symlinks.
func TestCheckKeyFilePermissions_EnforcedForAllUsers(t *testing.T) {
	dir := t.TempDir()

	strictModes := []os.FileMode{0o600, 0o400}
	for _, mode := range strictModes {
		path := filepath.Join(dir, "strict-"+mode.String()+".key")
		if err := os.WriteFile(path, []byte("dummy"), mode); err != nil {
			t.Fatalf("write %04o key: %v", mode, err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatalf("chmod %04o key: %v", mode, err)
		}
		if err := checkKeyFilePermissions(path); err != nil {
			t.Fatalf("strict mode %04o rejected: %v", mode, err)
		}
	}

	looseModes := []os.FileMode{0o640, 0o604, 0o644, 0o610}
	for _, mode := range looseModes {
		path := filepath.Join(dir, "loose-"+mode.String()+".key")
		if err := os.WriteFile(path, []byte("dummy"), 0o600); err != nil {
			t.Fatalf("write %04o key: %v", mode, err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatalf("chmod %04o key: %v", mode, err)
		}
		err := checkKeyFilePermissions(path)
		if err == nil {
			t.Fatalf("expected rejection for mode %04o key file", mode)
		}
		if !strings.Contains(err.Error(), "group/other permissions") {
			t.Fatalf("mode %04o error = %v, want group/other permissions", mode, err)
		}
	}

	realKey := filepath.Join(dir, "target.key")
	if err := os.WriteFile(realKey, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write target key: %v", err)
	}
	link := filepath.Join(dir, "symlink.key")
	if err := os.Symlink(realKey, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := checkKeyFilePermissions(link); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink error = %v, want symlink rejection", err)
	}

	sub := filepath.Join(dir, "dirkey")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := checkKeyFilePermissions(sub); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("directory error = %v, want regular file rejection", err)
	}
}
