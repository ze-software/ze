// Design: docs/features/interfaces.md -- DNS resolver config tests

//go:build linux

package ifacedhcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteResolvConfSingle verifies that a single DNS server is written
// to resolv.conf as a nameserver line.
//
// VALIDATES: AC-6 - DHCP ACK with DNS option writes resolv.conf.
// PREVENTS: DNS from DHCP silently dropped.
func TestWriteResolvConfSingle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "resolv.conf")
	err := writeResolvConfTo(path, []string{"8.8.8.8"})
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "nameserver 8.8.8.8\n")
}

// TestWriteResolvConfMultiple verifies multiple DNS servers are all written.
//
// VALIDATES: AC-7 - Multiple DNS servers written to resolv.conf.
// PREVENTS: Only first DNS server written, rest dropped.
func TestWriteResolvConfMultiple(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "resolv.conf")
	err := writeResolvConfTo(path, []string{"8.8.8.8", "8.8.4.4", "1.1.1.1"})
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "nameserver 8.8.8.8\n")
	assert.Contains(t, content, "nameserver 8.8.4.4\n")
	assert.Contains(t, content, "nameserver 1.1.1.1\n")
	// Verify order preserved.
	i1 := strings.Index(content, "8.8.8.8")
	i2 := strings.Index(content, "8.8.4.4")
	i3 := strings.Index(content, "1.1.1.1")
	assert.Less(t, i1, i2)
	assert.Less(t, i2, i3)
}

// TestWriteResolvConfEmpty verifies that an empty server list is a no-op.
//
// VALIDATES: writeResolvConf with no servers does nothing.
// PREVENTS: Empty resolv.conf written on DHCP ACK without DNS option.
func TestWriteResolvConfEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "resolv.conf")
	err := writeResolvConfTo(path, nil)
	require.NoError(t, err)
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "file should not be created for empty list")
}

// TestClearResolvConf verifies that clearResolvConf removes the file.
//
// VALIDATES: AC-8 - On lease expiry, resolv.conf cleared.
// PREVENTS: Stale DNS config after DHCP lease expires.
func TestClearResolvConf(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "resolv.conf")
	require.NoError(t, os.WriteFile(path, []byte("nameserver 8.8.8.8\n"), 0o644))
	err := clearResolvConfAt(path)
	require.NoError(t, err)
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr))
}

// TestClearResolvConfMissing verifies that clearing a nonexistent file is not an error.
//
// VALIDATES: clearResolvConf handles missing file gracefully.
// PREVENTS: Error on double-clear or clear before any DHCP lease.
func TestClearResolvConfMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent")
	err := clearResolvConfAt(path)
	assert.NoError(t, err)
}
