package hub

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// TestPIDFileWriteRemove verifies writePIDFile + removePIDFile cooperate.
//
// VALIDATES: AC-10 (ZE_PID_FILE path causes write on start, remove on shutdown).
// PREVENTS: PID file leaking across daemon restarts.
func TestPIDFileWriteRemove(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	dir := t.TempDir()
	path := filepath.Join(dir, "ze.pid")
	t.Setenv("ze.pid.file", path)
	env.ResetCache()

	wrote, err := writePIDFile()
	if err != nil {
		t.Fatalf("writePIDFile error: %v", err)
	}
	if wrote != path {
		t.Fatalf("writePIDFile returned %q, want %q", wrote, path)
	}

	data, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse pid %q: %v", string(data), err)
	}
	if pid != os.Getpid() {
		t.Errorf("pid file has %d, want %d", pid, os.Getpid())
	}

	removePIDFile(path)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected pid file removed, stat err = %v", err)
	}
}

// TestPIDFileUnset verifies writePIDFile is a no-op when ze.pid.file is empty.
//
// VALIDATES: PID file is opt-in (no file when env unset).
func TestPIDFileUnset(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	t.Setenv("ze.pid.file", "")
	env.ResetCache()

	wrote, err := writePIDFile()
	if err != nil {
		t.Fatalf("writePIDFile error: %v", err)
	}
	if wrote != "" {
		t.Errorf("writePIDFile returned %q, want empty", wrote)
	}
}

// TestPIDFileRefusesExisting verifies writePIDFile fails closed when the file
// already exists (symlink-attack defense).
//
// VALIDATES: Security review item -- symlink attack prevention.
func TestPIDFileRefusesExisting(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	dir := t.TempDir()
	path := filepath.Join(dir, "ze.pid")
	if err := os.WriteFile(path, []byte("99999\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ze.pid.file", path)
	env.ResetCache()

	if _, err := writePIDFile(); err == nil {
		t.Error("expected error on existing pid file, got nil")
	}
}
