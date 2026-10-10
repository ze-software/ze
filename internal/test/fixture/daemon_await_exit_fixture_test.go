package fixture

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startAwaitExitTarget starts a process that outlives the test unless it is
// killed, publishes its pid in daemon.pid in a fresh working directory, and
// makes that directory the current one, as the runner does for a .ci.
func startAwaitExitTarget(t *testing.T) *exec.Cmd {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	target := exec.CommandContext(t.Context(), "sleep", "30")
	require.NoError(t, target.Start())
	t.Cleanup(func() {
		_ = target.Process.Kill() //nolint:errcheck // the subtest may have killed it already
		_ = target.Wait()         //nolint:errcheck // reaping only; the exit status is the kill
	})
	pid := strconv.Itoa(target.Process.Pid)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "daemon.pid"), []byte(pid), 0o600))
	return target
}

// TestDaemonAwaitExit proves the barrier between two daemons of one .ci
// reports an exit only for a process that has exited. A .ci that starts a
// second daemon on the first one's file (test/plugin/api-peer-save.ci,
// api-peer-create-delete-rib.ci, session-editor-commit-confirmed-restart.ci)
// trusts its "OK: the first daemon exited" line, so a false exit would let the
// second daemon start beside a live first one.
//
// VALIDATES: a running process is never reported exited; a zombie (the runner
// does not reap a foreground daemon until the test ends) and a reaped process
// are; a missing daemon.pid is an error, never an exit.
// PREVENTS: an await-exit that reads a running daemon as gone.
// MUTATION: make daemonProcessState return ("", nil) and the "running" case
// reports an exit.
func TestDaemonAwaitExit(t *testing.T) {
	t.Run("running", func(t *testing.T) {
		startAwaitExitTarget(t)
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		err := daemonAwaitExit(ctx, nil)
		require.Error(t, err, "a running process was reported exited")
		assert.Contains(t, err.Error(), "never exited")
	})
	t.Run("zombie", func(t *testing.T) {
		target := startAwaitExitTarget(t)
		require.NoError(t, target.Process.Kill())
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		require.NoError(t, daemonAwaitExit(ctx, nil))
	})
	t.Run("reaped", func(t *testing.T) {
		target := startAwaitExitTarget(t)
		require.NoError(t, target.Process.Kill())
		_ = target.Wait() //nolint:errcheck // the exit status is the kill
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		require.NoError(t, daemonAwaitExit(ctx, nil))
	})
	t.Run("no pid file", func(t *testing.T) {
		t.Chdir(t.TempDir())
		err := daemonAwaitExit(t.Context(), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read daemon pid")
	})
}
