//go:build unix

// Design: docs/architecture/cli/plugin-modes.md — ze local install: binary copy is inode-replacing

package local

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statOf returns the raw stat for path, or skips when the platform does not
// expose inode numbers. The *syscall.Stat_t is returned rather than a widened
// integer so callers compare Ino fields of identical type on every GOARCH.
func statOf(t *testing.T, path string) *syscall.Stat_t {
	t.Helper()
	fi, err := os.Stat(path)
	require.NoError(t, err)
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		t.Skip("inode numbers unavailable on this platform")
	}
	return st
}

// VALIDATES: copyFile replaces an existing destination by renaming a new inode
// over it, so a process executing the old binary keeps a valid mapping.
// PREVENTS: regression to os.OpenFile(dst, O_WRONLY|O_CREATE|O_TRUNC), which
// truncates the SAME inode a running executable is mapped from — SIGBUS on the
// running process (ETXTBSY on Linux, unprotected on macOS). Same hazard as the
// zefs store fix in pkg/zefs/store.go.
func TestCopyFileReplacesInodeInsteadOfTruncating(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "ze")
	src := filepath.Join(dir, "ze-new")

	const oldContent = "OLD-BINARY-CONTENT-THAT-MUST-SURVIVE"
	const newContent = "NEW"

	require.NoError(t, os.WriteFile(dst, []byte(oldContent), 0o755)) // #nosec G306 - test fixture stands in for an installed binary
	require.NoError(t, os.WriteFile(src, []byte(newContent), 0o644)) // #nosec G306 - test fixture

	// A hard link keeps the ORIGINAL inode reachable by path after the copy.
	// It is the stand-in for a running process's mapping of that inode: if
	// copyFile truncates in place, this link observes the damage.
	pinned := filepath.Join(dir, "ze-pinned")
	require.NoError(t, os.Link(dst, pinned))

	before := statOf(t, dst)

	require.NoError(t, copyFile(src, dst))

	after := statOf(t, dst)

	// The destination path now names a DIFFERENT inode.
	assert.NotEqual(t, before.Ino, after.Ino, "copyFile must rename a new inode over dst, not truncate dst in place")

	// The old inode is untouched: same bytes, same length. Under O_TRUNC this
	// would have become newContent (or an empty/partial file).
	pinnedData, err := os.ReadFile(pinned) // #nosec G304 - path built by this test
	require.NoError(t, err)
	assert.Equal(t, oldContent, string(pinnedData), "the original inode was written through; a running process would have taken SIGBUS")

	// The destination really did get the new content.
	dstData, err := os.ReadFile(dst) // #nosec G304 - path built by this test
	require.NoError(t, err)
	assert.Equal(t, newContent, string(dstData))

	// os.CreateTemp makes 0600; the installed binary must still be executable.
	fi, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), fi.Mode().Perm(), "installed binary must keep its executable mode")

	assertNoTempLeftovers(t, dir)
}

// VALIDATES: a fresh install (no existing destination) still lands the binary
// with the requested executable mode.
// PREVENTS: the 0600 that os.CreateTemp assigns leaking through to an install
// that never had a destination file to inherit a mode from.
func TestCopyFileFreshInstallIsExecutable(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "ze")
	src := filepath.Join(dir, "ze-new")

	require.NoError(t, os.WriteFile(src, []byte("BINARY"), 0o644)) // #nosec G306 - test fixture
	require.NoError(t, copyFile(src, dst))

	data, err := os.ReadFile(dst) // #nosec G304 - path built by this test
	require.NoError(t, err)
	assert.Equal(t, "BINARY", string(data))

	fi, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), fi.Mode().Perm())

	assertNoTempLeftovers(t, dir)
}

// VALIDATES: a copy that fails after the temp file exists removes it.
// PREVENTS: leaking .ze-install-*.tmp files into the install directory on every
// failed install. Reading a directory fd fails (EISDIR), so passing a directory
// as src drives the io.Copy error path with the temp file already created.
func TestCopyFileRemovesTempOnCopyFailure(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "ze")
	srcDir := filepath.Join(dir, "not-a-file")
	require.NoError(t, os.Mkdir(srcDir, 0o755))

	require.Error(t, copyFile(srcDir, dst))

	_, err := os.Stat(dst)
	assert.True(t, os.IsNotExist(err), "failed copy must not leave a destination behind")
	assertNoTempLeftovers(t, dir)
}

func assertNoTempLeftovers(t *testing.T, dir string) {
	t.Helper()
	leftovers, err := filepath.Glob(filepath.Join(dir, ".ze-install-*.tmp"))
	require.NoError(t, err)
	assert.Empty(t, leftovers, "temp file must be cleaned up")
}
