// Design: docs/guide/config-editor.md -- structured daemon rollback failures.

package cli

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/confirm"
)

func TestModelWindowFailureClearsWithoutLosingCommandError(t *testing.T) {
	f := newWindowFixture(t)
	revertErr := errors.New("rollback device refused the write")
	failed := make(chan struct{}, 1)
	f.window = confirm.NewWindow(func([]byte) error {
		select {
		case failed <- struct{}{}:
		default:
		}
		return revertErr
	}, &memoryRecorder{})
	t.Cleanup(f.window.Stop)
	m := f.model(t, "operator")
	t.Cleanup(func() { require.NoError(t, m.Close()) })
	require.NoError(t, m.editor.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))
	result, err := m.cmdCommitRequest(confirmedRequest(1, false))
	require.NoError(t, err)
	m.applyResult(result)
	select {
	case <-failed:
	case <-time.After(5 * time.Second):
		t.Fatal("the real deadline did not attempt rollback")
	}
	// A worker request after the reverter signal waits until deadlineRevert
	// has published its failed Status, without a timing-dependent sleep.
	require.ErrorIs(t, f.window.Now("operator", confirm.Commit{
		Apply: func() error { return nil },
	}), confirm.ErrPending)
	commandErr := errors.New("an independent command refusal")
	m.err = commandErr
	_, noticed := m.pollDaemonWindow()
	require.True(t, noticed)
	require.ErrorIs(t, m.Error(), revertErr)
	require.ErrorIs(t, m.Error(), commandErr)
	_, open := f.window.Status()
	require.True(t, open)

	require.NoError(t, f.window.Accept("operator"))
	_, noticed = m.pollDaemonWindow()
	require.True(t, noticed)
	require.NotErrorIs(t, m.Error(), revertErr, "closure clears the watched failure")
	require.ErrorIs(t, m.Error(), commandErr, "polling must not clear an unrelated command error")
}

func TestWindowWatchClosedObservationClearsUninitializedFailure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		watched bool
		stopped bool
	}{
		{name: "fresh-closed"},
		{name: "fresh-stopped", stopped: true},
		{name: "watched-closed", watched: true},
		{name: "watched-stopped", watched: true, stopped: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			window := confirm.NewWindow(func([]byte) error { return nil }, &memoryRecorder{})
			t.Cleanup(window.Stop)
			if tc.stopped {
				window.Stop()
				require.Nil(t, WatchWindow(window), "a stopped window cannot initialize a watch")
			}
			m := NewCommandModel(FilesystemAuthorityOperatorLocal)
			t.Cleanup(func() { require.NoError(t, m.Close()) })
			failure := errors.New("failure read before the window stopped")
			commandErr := errors.New("independent command failure")
			m.err = commandErr
			// Status can read an open failure before Stop, then WatchWindow
			// can return nil after Stop. Poll still retains that failure while
			// open remains false. Seed exactly that reachable intermediate
			// state; no timing race or production interception hook is needed.
			m.windowWatch = WindowWatch{open: tc.watched, revertFailed: failure}
			require.ErrorIs(t, m.Error(), failure)
			_, noticed := m.windowWatch.Poll(window, "operator")
			require.Equal(t, tc.watched && !tc.stopped, noticed)
			require.NotErrorIs(t, m.Error(), failure, "every closed observation must clear the retained rollback failure")
			require.ErrorIs(t, m.Error(), commandErr)
			require.False(t, m.ConfirmTimerActive())
			_, noticed = m.windowWatch.Poll(window, "operator")
			require.False(t, noticed, "a later poll must not invent a closure notification")
			require.NotErrorIs(t, m.Error(), failure)
		})
	}
}
