// VALIDATES: a foreground helper that is the last cmd= line is waited for, so
//            its output reaches its assertions and its exit status is its own,
//            and a helper that never exits fails the test with a timeout that
//            names it.
// PREVENTS: the 2026-09-14 journal row: runOrchestrated classed the last
//           foreground helper with the daemons, started it, never waited, and
//           tore it down, so its output reached the accumulators only if it won
//           the race, and a helper that never exited passed as a daemon.

package runner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// lastHelperRecord builds a runner and a one-command record whose only command
// is the foreground helper under test, so the run has no daemon to wait on.
func lastHelperRecord(t *testing.T, name, timeout string, cmd *RunCommand) (*Runner, *Record) {
	t.Helper()
	baseDir := t.TempDir()
	tests := NewEncodingTests(baseDir)
	r, err := NewRunner(tests, baseDir)
	require.NoError(t, err)
	t.Cleanup(r.Cleanup)

	rec := tests.Add(name)
	rec.Active = true
	rec.Conf = map[string]any{}
	rec.Extra = map[string]string{"timeout": timeout}
	rec.RunCommands = []RunCommand{*cmd}
	return r, rec
}

// TestLastForegroundHelperOutputIsWaitedFor runs a helper that writes only
// after a delay: torn down at once, it never writes, and the assertion fails.
func TestLastForegroundHelperOutputIsWaitedFor(t *testing.T) {
	r, rec := lastHelperRecord(t, "late-writer", "10s", &RunCommand{
		Mode: modeForeground, Seq: 1, Exec: `sh -c "sleep 0.3; echo helper-done"`,
		ExpectStdout: []string{"helper-done"},
	})
	require.True(t, r.runTest(t.Context(), rec, &RunOptions{}), "last helper must be waited for: %v", rec.Error)
}

// TestLastForegroundHelperThatNeverExitsTimesOut proves a helper still running
// at the deadline fails the test and the error names it.
func TestLastForegroundHelperThatNeverExitsTimesOut(t *testing.T) {
	r, rec := lastHelperRecord(t, "never-exits", "1s", &RunCommand{
		Mode: modeForeground, Seq: 1, Exec: `sh -c "echo started; sleep 30"`,
		ExpectStdout: []string{"started"},
	})
	require.False(t, r.runTest(t.Context(), rec, &RunOptions{}))
	require.ErrorContains(t, rec.Error, "did not exit")
	require.ErrorContains(t, rec.Error, "cmd=background")
	require.Equal(t, stateTimeout, rec.FailureType)
}

// TestLastForegroundHelperExitCodeIsItsOwn keeps expect=exit asserting the
// helper's exit status once the helper is no longer the daemon stand-in.
func TestLastForegroundHelperExitCodeIsItsOwn(t *testing.T) {
	code := 3
	r, rec := lastHelperRecord(t, "exits-three", "10s", &RunCommand{
		Mode: modeForeground, Seq: 1, Exec: `sh -c "echo bye; exit 3"`,
		ExpectStdout: []string{"bye"},
	})
	rec.ExpectExitCode = &code
	require.True(t, r.runTest(t.Context(), rec, &RunOptions{}), "%v", rec.Error)
}

// TestForegroundHelperWithAGrandchildTimesOutOnTime proves the named timeout
// fires on time when a foreground helper leaves a grandchild holding its
// stdout pipe, in both wait arms: the last cmd= line and a mid-file helper.
//
// VALIDATES: the deadline ends the helper's whole process group
//
//	(boundProcessDeadline), so proc.Wait returns, and the run reports the
//	helper by name as a timeout within a bound well under the grandchild's
//	sleep.
//
// PREVENTS: a grandchild holding the pipe blocking proc.Wait past the
//
//	deadline, so the test hangs until the grandchild exits on its own.
func TestForegroundHelperWithAGrandchildTimesOutOnTime(t *testing.T) {
	const grandchild = `sh -c "sleep 30 & sleep 30"`
	tests := []struct {
		name string
		cmds []RunCommand
	}{
		{"last helper", []RunCommand{{Mode: modeForeground, Seq: 1, Exec: grandchild}}},
		{"mid-file helper", []RunCommand{
			{Mode: modeForeground, Seq: 1, Exec: grandchild},
			{Mode: modeForeground, Seq: 2, Exec: `sh -c "echo after"`},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, rec := lastHelperRecord(t, "grandchild", "1s", &tt.cmds[0])
			rec.RunCommands = tt.cmds

			start := time.Now()
			require.False(t, r.runTest(t.Context(), rec, &RunOptions{}))
			elapsed := time.Since(start)

			require.ErrorContains(t, rec.Error, "cmd seq=1")
			require.ErrorContains(t, rec.Error, "did not exit within")
			require.Equal(t, stateTimeout, rec.FailureType)
			// Under the deadline plus processWaitDelay, the backstop for a
			// descendant that escaped the group, and far under the 30s sleep.
			require.Less(t, elapsed, time.Second+processWaitDelay, "the timeout did not fire on time")
		})
	}
}

// TestLastHelperRejectReadsToTheEndOfTheRun proves a reject under the last
// foreground helper still sees bytes written after the helper exited, while an
// expect under it reads only the helper's own span.
//
// VALIDATES: rejectSpan reads a last helper's streams from its start to the
//
//	end of the run, as it did when the last helper's span stayed open. The
//	background process writes the forbidden text only on teardown, after the
//	helper has exited.
//
// PREVENTS: closing the last helper's span at its exit narrowing its rejects,
//
//	so daemon teardown output a test forbids would pass unseen.
func TestLastHelperRejectReadsToTheEndOfTheRun(t *testing.T) {
	const daemon = `sh -c 'trap "echo late-forbidden; exit 0" TERM; sleep 10 >/dev/null 2>&1 & wait'`
	const helper = `sh -c "echo helper-done"`
	tests := []struct {
		name string
		last RunCommand
		pass bool
	}{
		{"reject sees teardown output", RunCommand{
			Mode: modeForeground, Seq: 2, Exec: helper, RejectStdout: []string{"late-forbidden"},
		}, false},
		{"expect does not", RunCommand{
			Mode: modeForeground, Seq: 2, Exec: helper, ExpectStdout: []string{"late-forbidden"},
		}, false},
		{"expect reads the helper span", RunCommand{
			Mode: modeForeground, Seq: 2, Exec: helper, ExpectStdout: []string{"helper-done"},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds := []RunCommand{{Mode: modeBackground, Seq: 1, Exec: daemon}, tt.last}
			r, rec := lastHelperRecord(t, "teardown-writer", "10s", &cmds[0])
			rec.RunCommands = cmds
			ok := r.runTest(t.Context(), rec, &RunOptions{})
			require.Contains(t, rec.ClientStdout, "late-forbidden", "the daemon's teardown write must reach the accumulator")
			require.Equal(t, tt.pass, ok, "%v", rec.Error)
		})
	}
}
