package pidfile

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPIDFileLocationXDG verifies XDG_RUNTIME_DIR is used when set and writable.
//
// VALIDATES: Priority 1 location uses XDG_RUNTIME_DIR/ze/<hash>.pid.
// PREVENTS: Ignoring XDG convention on modern Linux systems.
func TestPIDFileLocationXDG(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", xdgDir)

	configPath := "/etc/ze/router.conf"
	loc, err := Location(configPath)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(loc, filepath.Join(xdgDir, "ze")),
		"should use XDG_RUNTIME_DIR/ze/")
	assert.True(t, strings.HasSuffix(loc, ".pid"),
		"should end with .pid")
}

// TestPIDFileLocationTmpFallback verifies fallback to /tmp/ze/ when XDG is not set.
//
// VALIDATES: Priority 3 location uses os.TempDir()/ze/<hash>.pid.
// PREVENTS: Failure when XDG_RUNTIME_DIR is not set (same cascade as socket).
func TestPIDFileLocationTmpFallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")

	configPath := "/etc/ze/router.conf"
	loc, err := Location(configPath)
	require.NoError(t, err)

	hash := ConfigHash("/etc/ze/router.conf")
	expected := filepath.Join(os.TempDir(), "ze", hash+".pid")
	assert.Equal(t, expected, loc)
}

// TestPIDFileLocationAlwaysUsesHash verifies all paths use config hash naming.
//
// VALIDATES: PID file name is always <hash>.pid regardless of fallback level.
// PREVENTS: Inconsistent naming between XDG and fallback paths.
func TestPIDFileLocationAlwaysUsesHash(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")

	loc, err := Location("/etc/ze/router.conf")
	require.NoError(t, err)

	assert.True(t, strings.HasSuffix(loc, ".pid"))
	base := filepath.Base(loc)
	assert.Equal(t, ConfigHash("/etc/ze/router.conf")+".pid", base,
		"filename should be <config-hash>.pid")
}

// TestPIDFileCreate verifies PID file content format.
//
// VALIDATES: File contains PID, config path, and timestamp on separate lines.
// PREVENTS: Malformed PID file that can't be parsed back.
func TestPIDFileCreate(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	configPath := "/etc/ze/router.conf"

	pf, err := Acquire(pidPath, configPath)
	require.NoError(t, err)
	defer pf.Release()

	// Read and verify content
	data, err := os.ReadFile(pidPath)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 3, "PID file should have 3 lines")

	// Line 1: PID
	pid, err := strconv.Atoi(lines[0])
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), pid)

	// Line 2: config path
	assert.Equal(t, configPath, lines[1])

	// Line 3: timestamp (RFC 3339)
	_, err = time.Parse(time.RFC3339, lines[2])
	require.NoError(t, err, "line 3 should be RFC 3339 timestamp")
}

// TestPIDFileAcquireLock verifies flock mutual exclusion.
//
// VALIDATES: Second Acquire fails when first holds the lock.
// PREVENTS: Two instances running with same config.
func TestPIDFileAcquireLock(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	configPath := "/etc/ze/router.conf"

	pf1, err := Acquire(pidPath, configPath)
	require.NoError(t, err)
	defer pf1.Release()

	// Second acquire should fail
	_, err = Acquire(pidPath, configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

// TestPIDFileRelease verifies lock release and file removal.
//
// VALIDATES: After Release, file is removed and lock is freed.
// PREVENTS: Stale PID files after graceful shutdown.
func TestPIDFileRelease(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	configPath := "/etc/ze/router.conf"

	pf, err := Acquire(pidPath, configPath)
	require.NoError(t, err)

	pf.Release()

	// File should be removed
	_, err = os.Stat(pidPath)
	assert.True(t, os.IsNotExist(err), "PID file should be removed after Release")

	// Should be able to acquire again
	pf2, err := Acquire(pidPath, configPath)
	require.NoError(t, err)
	pf2.Release()
}

// TestPIDFileStaleDetection verifies stale PID file detection.
//
// VALIDATES: ReadInfo returns Locked=false for stale PID file (no lock holder).
// PREVENTS: Refusing to start when previous instance crashed.
func TestPIDFileStaleDetection(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	configPath := "/etc/ze/router.conf"

	// Write a fake PID file (no flock held)
	content := "99999\n" + configPath + "\n2026-01-31T10:30:00Z\n"
	err := os.WriteFile(pidPath, []byte(content), 0o644)
	require.NoError(t, err)

	// Should detect as stale (no lock)
	info, err := ReadInfo(pidPath)
	require.NoError(t, err)
	assert.Equal(t, 99999, info.PID)
	assert.Equal(t, configPath, info.ConfigPath)
	assert.False(t, info.Locked, "should not be locked (stale)")

	// Acquire should succeed (overwriting stale file)
	pf, err := Acquire(pidPath, configPath)
	require.NoError(t, err)
	defer pf.Release()

	// Now ReadInfo should show locked
	info2, err := ReadInfo(pidPath)
	require.NoError(t, err)
	assert.True(t, info2.Locked, "should be locked after Acquire")
}

// TestPIDFileConfigHash verifies consistent hash computation.
//
// VALIDATES: Same config path always produces same hash.
// PREVENTS: Inconsistent PID file lookup between signal sender and daemon.
func TestPIDFileConfigHash(t *testing.T) {
	h1 := ConfigHash("/etc/ze/router.conf")
	h2 := ConfigHash("/etc/ze/router.conf")
	assert.Equal(t, h1, h2, "same path should produce same hash")

	h3 := ConfigHash("/etc/ze/other.conf")
	assert.NotEqual(t, h1, h3, "different paths should produce different hashes")

	assert.Len(t, h1, 8, "hash should be 8 characters")
}

// TestPIDFileBoundaryPID verifies PID parsing handles edge cases.
//
// VALIDATES: PID 1 is valid, PID 0 is invalid.
// PREVENTS: Off-by-one in PID validation.
// BOUNDARY: PID range 1-4194304.
func TestPIDFileBoundaryPID(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"valid_pid_1", "1\n/etc/ze/r.conf\n2026-01-31T10:30:00Z\n", false},
		{"invalid_pid_0", "0\n/etc/ze/r.conf\n2026-01-31T10:30:00Z\n", true},
		{"invalid_pid_negative", "-1\n/etc/ze/r.conf\n2026-01-31T10:30:00Z\n", true},
		{"invalid_pid_not_number", "abc\n/etc/ze/r.conf\n2026-01-31T10:30:00Z\n", true},
		{"valid_pid_max", "4194304\n/etc/ze/r.conf\n2026-01-31T10:30:00Z\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			pidPath := filepath.Join(dir, "test.pid")
			err := os.WriteFile(pidPath, []byte(tt.content), 0o644)
			require.NoError(t, err)

			info, err := ReadInfo(pidPath)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Greater(t, info.PID, 0)
			}
		})
	}
}
