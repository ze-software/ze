package runner

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestPublishDaemonPIDReplacesIdentity proves publication swaps a complete file
// into place rather than truncating the identity a fixture can already open.
// Retaining the old descriptor makes the distinction deterministic: an in-place
// write changes that reader's bytes, whereas an atomic replacement does not.
func TestPublishDaemonPIDReplacesIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.pid")
	if err := publishDaemonPID(path, 12345, false, 0, 0); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := previous.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := publishDaemonPID(path, 67890, false, 0, 0); err != nil {
		t.Fatal(err)
	}
	old, err := io.ReadAll(previous)
	if err != nil {
		t.Fatal(err)
	}
	if string(old) != "12345" {
		t.Errorf("publication changed an open identity to %q; want 12345", old)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != "67890" {
		t.Errorf("published identity = %q; want 67890", current)
	}
	assertOnlyDaemonPID(t, dir)
}

// TestPublishDaemonPIDModeOwnership checks both ordinary publication and the
// dropped-user path without requiring permission to take another user's files.
func TestPublishDaemonPIDModeOwnership(t *testing.T) {
	for _, dropped := range []bool{false, true} {
		name := "ordinary"
		if dropped {
			name = "dropped user"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "daemon.pid")
			if err := publishDaemonPID(path, 12345, dropped, os.Geteuid(), os.Getegid()); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Errorf("published permissions = %v; want 0600", info.Mode().Perm())
			}
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				t.Fatalf("file metadata has type %T; want *syscall.Stat_t", info.Sys())
			}
			if stat.Uid != uint32(os.Geteuid()) {
				t.Errorf("published uid = %d; want %d", stat.Uid, os.Geteuid())
			}
			if stat.Gid != uint32(os.Getegid()) {
				t.Errorf("published gid = %d; want %d", stat.Gid, os.Getegid())
			}
			assertOnlyDaemonPID(t, dir)
		})
	}
}

// TestPublishDaemonPIDRenameFailure removes its staged file and preserves the
// destination when a directory prevents the final publication.
func TestPublishDaemonPIDRenameFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.pid")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	err := publishDaemonPID(path, 12345, false, 0, 0)
	var renameErr *os.LinkError
	if !errors.As(err, &renameErr) {
		t.Fatalf("publication error = %v; want a rename failure", err)
	}
	if renameErr.Op != "rename" {
		t.Errorf("failed operation = %q; want rename", renameErr.Op)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("failed publication replaced the destination directory")
	}
	assertOnlyDaemonPID(t, dir)
}

// TestPublishDaemonPIDOwnershipFailure keeps the old public identity when the
// runner cannot give the staged file to the requested user.
func TestPublishDaemonPIDOwnershipFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can change ownership; this case requires an unprivileged runner")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.pid")
	if err := publishDaemonPID(path, 12345, false, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := publishDaemonPID(path, 67890, true, os.Geteuid()+1, os.Getegid()); err == nil {
		t.Fatal("publishing another user's PID succeeded without permission")
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != "12345" {
		t.Errorf("failed publication changed the identity to %q; want 12345", current)
	}
	assertOnlyDaemonPID(t, dir)
}

// TestPublishDaemonPIDCreateFailure reports an unavailable publication directory
// without creating a public identity or another directory entry.
func TestPublishDaemonPIDCreateFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing", "daemon.pid")
	if err := publishDaemonPID(path, 12345, false, 0, 0); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("publication error = %v; want missing directory", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("failed creation left %d entries; want none", len(entries))
	}
}

// TestPublishDaemonPIDRunnerFailure proves both launch arms report a failed PID
// handoff even when their child otherwise succeeds. The child's readiness file
// orders the destination-directory obstruction before the runner publishes.
func TestPublishDaemonPIDRunnerFailure(t *testing.T) {
	for _, mode := range []string{modeForeground, modeBackground} {
		t.Run(mode, func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "child.pid")
			script := filepath.Join(t.TempDir(), "daemon-stub")
			if err := os.WriteFile(script, []byte(`#!/bin/sh
set -eu
mkdir daemon.pid
printf '%s' "$$" > "$OWNED_PID_FILE"
: > "$ZE_READY_FILE"
echo daemon-stub-ready
`), 0o700); err != nil {
				t.Fatal(err)
			}
			r, rec := lastHelperRecord(t, "pid-publication-"+mode, "5s", &RunCommand{
				Mode: mode, Seq: 1, Exec: "ze start",
				ExpectStdout: []string{"daemon-stub-ready"},
			})
			r.zePath = script
			rec.EnvVars = append(rec.EnvVars, "OWNED_PID_FILE="+pidFile)
			zero := 0
			rec.ExpectExitCode = &zero
			if r.runTest(t.Context(), rec, &RunOptions{}) {
				t.Error("daemon launch passed despite the failed PID handoff")
			}
			if rec.Error == nil {
				t.Fatal("daemon launch lost its PID publication error")
			}
			if !strings.Contains(rec.Error.Error(), "publish daemon PID") {
				t.Errorf("daemon launch error = %v; want PID publication failure", rec.Error)
			}
			assertOwnedPIDReaped(t, pidFile)
		})
	}
}

func assertOnlyDaemonPID(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("publication left %d directory entries; want daemon.pid alone", len(entries))
	}
	if entries[0].Name() != "daemon.pid" {
		t.Errorf("publication left %q; want daemon.pid", entries[0].Name())
	}
}
