// Design: docs/architecture/chaos-web-dashboard.md -- ze daemon selection unit tests

package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFakeZe writes an executable file named ze into dir and returns its path.
func writeFakeZe(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "ze")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700)) //nolint:gosec // test fixture must be executable
	return path
}

// TestResolveZeDaemonExplicitBinary: a --binary value that names another
// program is the daemon, whatever PATH holds.
func TestResolveZeDaemonExplicitBinary(t *testing.T) {
	t.Setenv("PATH", "")
	fake := writeFakeZe(t, t.TempDir())

	got, err := resolveZeDaemon(fake)
	require.NoError(t, err)
	assert.Equal(t, fake, got)
}

// TestResolveZeDaemonFromPath: with no --binary, the ze on PATH is the daemon.
func TestResolveZeDaemonFromPath(t *testing.T) {
	dir := t.TempDir()
	fake := writeFakeZe(t, dir)
	t.Setenv("PATH", dir)

	got, err := resolveZeDaemon("")
	require.NoError(t, err)
	assert.Equal(t, fake, got)
}

// TestResolveZeDaemonRefusesNone: no --binary and no ze on PATH is refused
// with a message naming both, never answered with a guess.
// VALIDATES: a missing daemon is refused with a clear message.
// PREVENTS: a silent fallback to a binary found beside the running program.
func TestResolveZeDaemonRefusesNone(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := resolveZeDaemon("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--binary is not set and ze is not in PATH")
}

// TestResolveZeDaemonRefusesMissingBinary: a --binary path that does not
// exist is refused before any fork.
func TestResolveZeDaemonRefusesMissingBinary(t *testing.T) {
	_, err := resolveZeDaemon(filepath.Join(t.TempDir(), "ze"))
	require.Error(t, err)
}

// TestResolveZeDaemonRefusesSelf: the orchestrator runs inside le, so a
// daemon path that is the running executable (here, the test binary) is
// refused, whether it arrives through --binary or through PATH.
// VALIDATES: the fork-mode daemon is never the running le process.
// PREVENTS: `le chaos run` forking `le -`, which answers "unknown command: -".
func TestResolveZeDaemonRefusesSelf(t *testing.T) {
	self, err := os.Executable()
	require.NoError(t, err)

	_, err = resolveZeDaemon(self)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is this program, not ze")

	dir := t.TempDir()
	require.NoError(t, os.Symlink(self, filepath.Join(dir, "ze")))
	t.Setenv("PATH", dir)
	_, err = resolveZeDaemon("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is this program, not ze")
}

// TestCLIRunRefusesSelfAsDaemon drives the refusal from the entry point:
// fork mode with --binary naming the running program exits 1 before it
// starts anything.
func TestCLIRunRefusesSelfAsDaemon(t *testing.T) {
	self, err := os.Executable()
	require.NoError(t, err)

	code := CLIRun([]string{"--binary", self, "--seed", "1", "--peers", "1", "--duration", "1s", "--quiet"})
	assert.Equal(t, 1, code)
}
