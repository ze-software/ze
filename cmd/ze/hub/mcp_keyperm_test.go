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
