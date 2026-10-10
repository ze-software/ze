// Tests that an API config session cannot commit inside a confirmed-commit
// window.
//
// VALIDATES: spec-session-editor-file-mode-parity owner decision (e) for the
// REST, gRPC and gNMI config sessions (ConfigSessionManager.Commit).
// PREVENTS: an API commit landing inside a window and being wiped, silently,
// by the window's revert at the deadline.
package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/confirm"
)

// confirmNopRecorder persists nothing: the test never restarts the daemon.
type confirmNopRecorder struct{}

func (confirmNopRecorder) Save(confirm.Pending) error { return nil }
func (confirmNopRecorder) Clear() error               { return nil }

// openWindow returns a window alice holds open for a minute.
func openWindow(t *testing.T) *confirm.Window {
	t.Helper()
	window := confirm.NewWindow(func([]byte) error { return nil }, confirmNopRecorder{})
	t.Cleanup(window.Stop)
	opened := confirm.Commit{
		Snapshot: func() ([]byte, error) { return []byte("# original\n"), nil },
		Apply:    func() error { return nil },
	}
	require.NoError(t, window.Confirmed("alice", time.Minute, false, opened))
	return window
}

// TestConfigSessionCommitRefusedDuringConfirmWindow commits a session while
// alice's window is open, through both commit paths (the runtime reload hook
// the daemon sets, and the plain save with no hook), and expects the window's
// own refusal naming alice, with nothing staged, saved or reloaded and the
// session kept for a retry.
func TestConfigSessionCommitRefusedDuringConfirmWindow(t *testing.T) {
	for _, withHook := range []bool{true, false} {
		name := "save"
		if withHook {
			name = "reload_hook"
		}
		t.Run(name, func(t *testing.T) {
			editor := newFakeEditor()
			mgr := NewConfigSessionManager(func() (ConfigEditor, error) { return editor, nil })
			reloaded := false
			if withHook {
				mgr.SetCommitHook(func() error {
					reloaded = true
					return nil
				})
			}
			window := openWindow(t)
			mgr.SetConfirmWindow(func() *confirm.Window { return window })

			id, err := mgr.Enter("bob")
			require.NoError(t, err)
			require.NoError(t, mgr.Set(&ConfigSetRequest{Username: "bob", SessionID: id, Path: "bgp.router-id", Value: "10.0.0.1"}))

			err = mgr.Commit(&ConfigCommitRequest{Username: "bob", SessionID: id})
			var other *confirm.OtherUserError
			require.ErrorAs(t, err, &other)
			assert.Equal(t, "alice", other.Owner)
			assert.False(t, reloaded, "the refused commit reloaded the daemon")
			assert.False(t, editor.saved, "the refused commit saved the config")
			assert.Empty(t, editor.stagedContent, "the refused commit staged a candidate")

			_, err = mgr.Diff(&ConfigDiffRequest{Username: "bob", SessionID: id})
			assert.NoError(t, err, "the refused session stays open for a retry")
		})
	}
}

// TestConfigSessionCommitWithoutWindowApplies proves the gate is the open
// window and not the presence of one: a daemon with no window (nil) and a
// window nobody opened both let the commit through.
func TestConfigSessionCommitWithoutWindowApplies(t *testing.T) {
	idle := confirm.NewWindow(func([]byte) error { return nil }, confirmNopRecorder{})
	t.Cleanup(idle.Stop)
	for name, window := range map[string]*confirm.Window{"no_daemon_window": nil, "idle_window": idle} {
		t.Run(name, func(t *testing.T) {
			editor := newFakeEditor()
			mgr := NewConfigSessionManager(func() (ConfigEditor, error) { return editor, nil })
			mgr.SetCommitHook(func() error { return nil })
			mgr.SetConfirmWindow(func() *confirm.Window { return window })

			id, err := mgr.Enter("bob")
			require.NoError(t, err)
			require.NoError(t, mgr.Set(&ConfigSetRequest{Username: "bob", SessionID: id, Path: "bgp.router-id", Value: "10.0.0.1"}))
			require.NoError(t, mgr.Commit(&ConfigCommitRequest{Username: "bob", SessionID: id}))
			assert.Equal(t, "# config\n", editor.committedContent)
		})
	}
}
