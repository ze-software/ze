// Tests that `request data restore` cannot restore the daemon's config inside
// a confirmed-commit window.
//
// VALIDATES: spec-session-editor-file-mode-parity owner decision (e) for the
// data restore (handleDataRestore).
// PREVENTS: a restored config going active inside a window and being wiped,
// silently, by the window's revert at the deadline.
package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
)

// confirmNopRecorder persists nothing: the test never restarts the daemon.
type confirmNopRecorder struct{}

func (confirmNopRecorder) Save(confirm.Pending) error { return nil }
func (confirmNopRecorder) Clear() error               { return nil }

// TestRestoreConfigRefusedDuringConfirmWindow restores while alice's window is
// open and expects the window's own refusal naming alice, with nothing staged,
// nothing reloaded and the active config unchanged. Closing the window lets
// the same restore through, so the refusal is the window and nothing else.
func TestRestoreConfigRefusedDuringConfirmWindow(t *testing.T) {
	var store storage.Storage
	var configPath string
	reloaded := false
	store, configPath = dataRPCStore(t, func(context.Context) error {
		reloaded = true
		return storage.PromoteCandidate(store, configPath)
	})
	window := confirm.NewWindow(func([]byte) error { return nil }, confirmNopRecorder{})
	t.Cleanup(window.Stop)
	target := *dataRPC.Load()
	target.Window = func() *confirm.Window { return window }
	InstallDataRPC(&target)
	opened := confirm.Commit{
		Snapshot: func() ([]byte, error) { return []byte("current"), nil },
		Apply:    func() error { return nil },
	}
	require.NoError(t, window.Confirmed("alice", time.Minute, false, opened))
	source := dataArtifact(t, "ze.conf", "restored")

	response := dataCall(t, handleDataRestore, "path", source, "config")
	assert.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "alice")
	assert.Contains(t, response.Error, "seconds left")
	assert.False(t, reloaded, "the refused restore reloaded the daemon")
	_, _, staged, err := storage.ReadCandidateConfig(store, configPath)
	require.NoError(t, err)
	assert.False(t, staged, "the refused restore staged a candidate")
	active, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "current", string(active))

	require.NoError(t, window.Accept("alice"))
	response = dataCall(t, handleDataRestore, "path", source, "config")
	require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
	assert.True(t, reloaded)
}
