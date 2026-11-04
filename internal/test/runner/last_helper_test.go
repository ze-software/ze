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
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
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

// TestBackgroundSpanSurvivesIntermediateHelper releases a background writer
// only after an intermediate helper has published its output snapshot. Both
// streams must remain in the background span until all writers have joined.
func TestBackgroundSpanSurvivesIntermediateHelper(t *testing.T) {
	const writer = `sh -c 'mkfifo release complete; echo writer-ready; read reply < release; echo background-later; echo background-later-err >&2; echo done > complete'`
	const release = `sh -c 'echo go > release; read reply < complete; echo final-helper'`
	for _, reject := range []bool{false, true} {
		name := "expects later output"
		if reject {
			name = "rejects later output"
		}
		t.Run(name, func(t *testing.T) {
			cmds := []RunCommand{
				{Mode: modeBackground, Seq: 1, Exec: writer, Ready: "writer-ready",
					ExpectStdout: []string{"background-later"}, ExpectStderrHas: []string{"background-later-err"}},
				{Mode: modeForeground, Seq: 2, Exec: `sh -c 'echo intermediate-helper'`,
					ExpectStdout: []string{"intermediate-helper", "background-later"}},
				{Mode: modeForeground, Seq: 3, Exec: release, ExpectStdout: []string{"final-helper"}},
			}
			if reject {
				cmds[0].RejectStderrHas = []string{"background-later-err"}
			}
			r, rec := lastHelperRecord(t, "background-span", "10s", &cmds[0])
			rec.RunCommands = cmds
			ok := r.runTest(t.Context(), rec, &RunOptions{})
			require.Contains(t, rec.ClientStdout, "background-later")
			require.Contains(t, rec.ClientStderr, "background-later-err")
			require.Equal(t, !reject, ok, "%v", rec.Error)
			require.Contains(t, commandStdout(rec, &rec.RunCommands[0]), "background-later")
			require.Contains(t, commandStderr(rec, &rec.RunCommands[0]), "background-later-err")
			if reject {
				require.ErrorContains(t, rec.Error, "cmd seq=1")
				require.ErrorContains(t, rec.Error, "unexpectedly contains")
			}
		})
	}
}

// TestEarlyErrorFinalizesOutputSpans exercises returns before normal teardown:
// with and without a helper snapshot, launched spans close after owner cleanup
// and commands that never launched cannot borrow another command's output.
func TestEarlyErrorFinalizesOutputSpans(t *testing.T) {
	const writer = `sh -c 'mkfifo early-release; echo writer-out; echo writer-err >&2; echo writer-ready; read reply < early-release'`
	code := 0
	tests := []struct {
		name string
		step RunCommand
		want string
	}{
		{"helper exit mismatch", RunCommand{
			Mode: modeForeground, Seq: 2, Exec: `sh -c 'echo helper-out; echo helper-err >&2; exit 7'`, ExitCode: &code,
		}, "expected exit code 0, got 7"},
		{"unknown stop", RunCommand{Mode: modeStop, Seq: 2, Name: "missing"}, "missing"},
		{"failed launch", RunCommand{
			Mode: modeForeground, Seq: 2, Exec: "./missing-output-span-helper",
		}, "start"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds := []RunCommand{
				{Mode: modeBackground, Seq: 1, Exec: writer, Ready: "writer-ready"},
				tt.step,
				{Mode: modeForeground, Seq: 3, Exec: `sh -c 'echo never-launched'`},
			}
			r, rec := lastHelperRecord(t, "early-error-span", "10s", &cmds[0])
			rec.RunCommands = cmds
			require.False(t, r.runTest(t.Context(), rec, &RunOptions{}))
			require.ErrorContains(t, rec.Error, tt.want)
			require.Contains(t, rec.ClientStdout, "writer-out")
			require.Contains(t, rec.ClientStderr, "writer-err")
			require.Equal(t, rec.ClientStdout+rec.ClientStderr, rec.ClientOutput)
			background := &rec.RunCommands[0]
			require.Equal(t, len(rec.ClientStdout), background.stdoutTo)
			require.Equal(t, len(rec.ClientStderr), background.stderrTo)
			require.Empty(t, commandStdout(rec, &rec.RunCommands[2]))
			require.Empty(t, commandStderr(rec, &rec.RunCommands[2]))
			if tt.name == "helper exit mismatch" {
				require.Contains(t, commandStdout(rec, background), "helper-out")
				require.Contains(t, commandStderr(rec, background), "helper-err")
			}
			if tt.name == "failed launch" {
				require.Equal(t, spanOpen, rec.RunCommands[1].stdoutFrom)
				require.Equal(t, spanOpen, rec.RunCommands[1].stderrFrom)
				require.Empty(t, commandStdout(rec, &rec.RunCommands[1]))
				require.Empty(t, commandStderr(rec, &rec.RunCommands[1]))
			}
		})
	}
}

// TestFailedOrchestratedRunSavesLateOutput drives helper errors through the
// production orchestration path. A FIFO releases the background writer after
// an intermediate snapshot; both saved streams must contain its later bytes
// even when a subsequent helper fails or times out before normal teardown.
func TestFailedOrchestratedRunSavesLateOutput(t *testing.T) {
	const writer = `sh -c 'mkfifo release complete blocked; echo writer-ready; read reply < release; echo background-late-out; echo background-late-err >&2; echo done > complete'`
	const release = `sh -c 'echo go > release; read reply < complete; echo helper-late-out; echo helper-late-err >&2; `
	zero := 0
	tests := []struct {
		name    string
		timeout string
		finish  string
		failure string
		cause   string
	}{
		{"helper-exit", "10s", "exit 7'", failTypeExitCodeMismatch, "expected exit code 0, got 7"},
		{"helper-timeout", "1s", "read reply < blocked'", stateTimeout, "foreground helper did not exit within"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds := []RunCommand{
				{Mode: modeBackground, Seq: 1, Exec: writer, Ready: "writer-ready"},
				{Mode: modeForeground, Seq: 2, Exec: `sh -c 'echo intermediate-snapshot'`},
				{Mode: modeForeground, Seq: 3, Exec: release + tt.finish, ExitCode: &zero},
				{Mode: modeForeground, Seq: 4, Exec: `sh -c 'echo never-launched'`},
			}
			r, rec := lastHelperRecord(t, tt.name, tt.timeout, &cmds[0])
			rec.RunCommands = cmds
			saveDir := t.TempDir()
			require.False(t, r.runTest(t.Context(), rec, &RunOptions{SaveDir: saveDir}))
			require.ErrorContains(t, rec.Error, tt.cause)
			require.Equal(t, tt.failure, rec.FailureType)

			saved := filepath.Join(saveDir, rec.Nick+"-"+tt.name)
			for _, stream := range []struct {
				file   string
				output string
				late   string
			}{
				{"client-stdout.log", rec.ClientStdout, "background-late-out"},
				{"client-stderr.log", rec.ClientStderr, "background-late-err"},
			} {
				content, err := os.ReadFile(filepath.Join(saved, stream.file))
				require.NoError(t, err)
				require.Equal(t, stream.output, string(content), stream.file)
				require.Contains(t, string(content), stream.late, stream.file)
				require.NotContains(t, string(content), "never-launched", stream.file)
			}
			require.Contains(t, rec.ClientStdout, "helper-late-out")
			require.Contains(t, rec.ClientStderr, "helper-late-err")
			require.Empty(t, commandStdout(rec, &rec.RunCommands[3]))
			require.Empty(t, commandStderr(rec, &rec.RunCommands[3]))
		})
	}
}

// TestEarlySaveWaitsForOwnedWriterJoin holds the real background child's
// sole reaper until error cleanup has signaled it. Saving before that owner
// joins is refused even if an earlier helper already published a snapshot.
func TestEarlySaveWaitsForOwnedWriterJoin(t *testing.T) {
	const writer = `mkfifo release complete; echo writer-ready; read reply < release; echo background-late-out; echo background-late-err >&2; echo done > complete`
	zero := 0
	cmds := []RunCommand{
		{Mode: modeBackground, Seq: 1, Exec: "sh -c '" + writer + "'", Ready: "writer-ready"},
		{Mode: modeForeground, Seq: 2, Exec: `sh -c 'echo intermediate-snapshot'`},
		{Mode: modeForeground, Seq: 3,
			Exec: `sh -c 'echo go > release; read reply < complete; exit 7'`, ExitCode: &zero},
	}
	r, rec := lastHelperRecord(t, "save-after-join", "10s", &cmds[0])
	rec.RunCommands = cmds
	release := make(chan struct{})
	attempted := make(chan struct{}, 1)
	var releaseOnce sync.Once
	r.observeProcessExit = func(p *ownedProcess) error {
		if p.cmd.Args[len(p.cmd.Args)-1] != writer {
			return p.beforeReap()
		}
		p.signalMu.Lock()
		signalGroup := p.groupSignal
		p.groupSignal = func(pid int, signal syscall.Signal) error {
			err := signalGroup(pid, signal)
			select {
			case attempted <- struct{}{}:
			default:
			}
			return err
		}
		p.signalMu.Unlock()
		<-release
		return p.beforeReap()
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	result := make(chan bool, 1)
	finished := make(chan struct{})
	saveDir := t.TempDir()
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		cancel()
		<-finished
	})
	go func() {
		result <- r.runTest(ctx, rec, &RunOptions{SaveDir: saveDir})
		close(finished)
	}()
	select {
	case <-attempted:
	case <-ctx.Done():
		t.Fatal("early error cleanup never reached the held background owner")
	}
	saved := filepath.Join(saveDir, rec.Nick+"-save-after-join")
	_, err := os.Stat(saved)
	require.ErrorIs(t, err, os.ErrNotExist, "output must not be saved before the owner joins")
	releaseOnce.Do(func() { close(release) })
	select {
	case ok := <-result:
		require.False(t, ok)
	case <-ctx.Done():
		t.Fatal("early error cleanup did not join the released owner")
	}
	require.ErrorContains(t, rec.Error, "expected exit code 0, got 7")
	require.Equal(t, failTypeExitCodeMismatch, rec.FailureType)
	for file, text := range map[string]string{
		"client-stdout.log": "background-late-out",
		"client-stderr.log": "background-late-err",
	} {
		content, err := os.ReadFile(filepath.Join(saved, file))
		require.NoError(t, err)
		require.Contains(t, string(content), text, file)
	}
}
