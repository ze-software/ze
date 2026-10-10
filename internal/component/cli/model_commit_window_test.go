// Tests for the session editor's commit subcommands routed through the
// daemon's confirmed-commit window.
//
// VALIDATES: spec-session-editor-file-mode-parity AC-13, AC-16, AC-17, AC-18,
// AC-23, AC-24 at the Model: a session `commit confirmed` applies and opens
// the daemon window for its user, `commit abort` reverts the running config,
// the owner's `commit now` and plain nested `commit confirmed` are refused,
// another user is refused naming the owner, and a refused or failed commit
// opens no window.
// METHOD: two session editors over one store, a real confirm.Window whose
// revert and reload promote the candidate, so the active config is what a
// running daemon would run.
package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// memoryRecorder keeps the window's record in memory.
type memoryRecorder struct{ saved int }

func (r *memoryRecorder) Save(confirm.Pending) error { r.saved++; return nil }
func (r *memoryRecorder) Clear() error               { return nil }

// windowFixture is one store, its daemon window, and a Model per user.
type windowFixture struct {
	store      storage.Storage
	configPath string
	window     *confirm.Window
	reloadErr  error
}

func newWindowFixture(t *testing.T) *windowFixture {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600))
	f := &windowFixture{store: newTestTreeStore(t, configPath), configPath: configPath}
	revert := func(rollback []byte) error {
		if _, err := storage.WriteCandidateVersion(f.store, configPath, rollback, time.Now()); err != nil {
			return err
		}
		return storage.PromoteCandidate(f.store, configPath)
	}
	f.window = confirm.NewWindow(revert, &memoryRecorder{})
	t.Cleanup(f.window.Stop)
	return f
}

// model builds a session Model for user whose commits reach the fixture's
// daemon: the reload promotes the candidate, or fails with f.reloadErr.
func (f *windowFixture) model(t *testing.T, user string) *Model {
	t.Helper()
	ed, err := NewEditorWithStorage(f.store, f.configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // test cleanup
	ed.SetReloadNotifier(func() error {
		if f.reloadErr != nil {
			return f.reloadErr
		}
		return storage.PromoteCandidate(f.store, f.configPath)
	})
	ed.SetConfirmWindow(func() *confirm.Window { return f.window })
	ed.SetSession(NewEditSession(user, "ssh"))
	m, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	return &m
}

func (f *windowFixture) active(t *testing.T) string {
	t.Helper()
	data, err := storage.ReadActiveConfig(f.store, f.configPath)
	require.NoError(t, err)
	return string(data)
}

func confirmedRequest(seconds int, force bool) contract.CommitRequest {
	return contract.CommitRequest{Action: contract.CommitConfirmed, Seconds: seconds, Force: force}
}

// TestSessionCommitConfirmedOpensDaemonWindow: AC-13 and AC-16. The commit
// reaches the running config at once and opens the window for the user;
// `commit abort` restores the config from before it.
func TestSessionCommitConfirmedOpensDaemonWindow(t *testing.T) {
	f := newWindowFixture(t)
	alice := f.model(t, "alice")
	before := f.active(t)

	require.NoError(t, alice.editor.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))
	result, err := alice.cmdCommitRequest(confirmedRequest(60, false))
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Confirm within 60s")
	assert.Contains(t, f.active(t), "9.9.9.9", "the commit applies at once")
	status, open := f.window.Status()
	require.True(t, open, "the daemon window is open")
	assert.Equal(t, "alice", status.User)

	result, err = alice.cmdCommitRequest(contract.CommitRequest{Action: contract.CommitAbort})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Changes rolled back to previous configuration.")
	assert.Equal(t, before, f.active(t), "abort restores the config from before the commit")
	_, open = f.window.Status()
	assert.False(t, open)
	assert.NotContains(t, alice.editor.WorkingContent(), "9.9.9.9", "the owner's view follows the revert")
}

// TestSessionCommitWindowRefusals: AC-18 (a) and (b), AC-17. The owner's
// `commit now [force]` and plain nested `commit confirmed` are refused with
// confirm.ErrPending; another user is refused naming the owner; the window
// and the running config stay as they were.
func TestSessionCommitWindowRefusals(t *testing.T) {
	f := newWindowFixture(t)
	alice := f.model(t, "alice")
	bob := f.model(t, "bob")

	_, err := alice.cmdCommitRequest(contract.CommitRequest{Action: contract.CommitAccept})
	require.ErrorIs(t, err, confirm.ErrNoWindow, "accept with no window open")

	require.NoError(t, alice.editor.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))
	_, err = alice.cmdCommitRequest(confirmedRequest(60, false))
	require.NoError(t, err)
	applied := f.active(t)

	require.NoError(t, alice.editor.SetValue([]string{"bgp"}, "router-id", "8.8.8.8"))
	for _, req := range []contract.CommitRequest{
		{Action: contract.CommitNow},
		{Action: contract.CommitNow, Force: true},
		confirmedRequest(30, false),
	} {
		_, err = alice.cmdCommitRequest(req)
		require.ErrorIs(t, err, confirm.ErrPending, "owner %+v", req)
	}

	require.NoError(t, bob.editor.SetValue([]string{"bgp"}, "router-id", "7.7.7.7"))
	for _, req := range []contract.CommitRequest{
		{Action: contract.CommitNow},
		{Action: contract.CommitNow, Force: true},
		confirmedRequest(30, false),
		confirmedRequest(30, true),
		{Action: contract.CommitAccept},
		{Action: contract.CommitAbort},
	} {
		_, err = bob.cmdCommitRequest(req)
		var other *confirm.OtherUserError
		require.True(t, errors.As(err, &other), "bob %+v: %v", req, err)
		assert.Equal(t, "alice", other.Owner)
	}
	assert.Equal(t, applied, f.active(t), "no refused commit reached the running config")

	// A second session of alice is the owner (AC-17).
	again := f.model(t, "alice")
	_, err = again.cmdCommitRequest(contract.CommitRequest{Action: contract.CommitAccept})
	require.NoError(t, err)
	_, open := f.window.Status()
	assert.False(t, open)
	assert.Contains(t, f.active(t), "9.9.9.9", "accept keeps the applied config")
}

// TestSessionCommitConfirmedForceNests: AC-23 and AC-24. A nested `commit
// confirmed <s> force` applies the new change and keeps the first rollback,
// so abort restores the config from before the FIRST commit; accept leaves
// uncommitted edits pending and out of the running config.
func TestSessionCommitConfirmedForceNests(t *testing.T) {
	f := newWindowFixture(t)
	alice := f.model(t, "alice")
	before := f.active(t)

	require.NoError(t, alice.editor.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))
	_, err := alice.cmdCommitRequest(confirmedRequest(60, false))
	require.NoError(t, err)
	require.NoError(t, alice.editor.SetValue([]string{"bgp"}, "router-id", "8.8.8.8"))
	result, err := alice.cmdCommitRequest(confirmedRequest(30, true))
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Confirm within 30s")
	assert.Contains(t, f.active(t), "8.8.8.8")
	status, open := f.window.Status()
	require.True(t, open)
	assert.LessOrEqual(t, status.Left(), 30*time.Second, "the countdown restarted at 30 s")

	_, err = alice.cmdCommitRequest(contract.CommitRequest{Action: contract.CommitAbort})
	require.NoError(t, err)
	assert.Equal(t, before, f.active(t), "abort restores the config from before the first commit")

	require.NoError(t, alice.editor.SetValue([]string{"bgp"}, "router-id", "6.6.6.6"))
	_, err = alice.cmdCommitRequest(confirmedRequest(60, false))
	require.NoError(t, err)
	require.NoError(t, alice.editor.SetValue([]string{"bgp"}, "router-id", "5.5.5.5"))
	_, err = alice.cmdCommitRequest(contract.CommitRequest{Action: contract.CommitAccept})
	require.NoError(t, err)
	assert.Contains(t, f.active(t), "6.6.6.6", "accept keeps the applied commit")
	assert.NotContains(t, f.active(t), "5.5.5.5", "accept applies nothing more")
	assert.Contains(t, alice.editor.WorkingContent(), "5.5.5.5", "the uncommitted edit stays pending")
}

// TestSessionCommitConfirmedFailedOpensNoWindow: a commit the daemon refuses
// opens no window, and the edit stays pending.
func TestSessionCommitConfirmedFailedOpensNoWindow(t *testing.T) {
	f := newWindowFixture(t)
	alice := f.model(t, "alice")
	f.reloadErr = errors.New("connection refused")

	require.NoError(t, alice.editor.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))
	result, err := alice.cmdCommitRequest(confirmedRequest(60, false))
	require.NoError(t, err, "a failed commit is reported as status")
	assert.Contains(t, result.statusMessage, "commit failed")
	_, open := f.window.Status()
	assert.False(t, open, "a failed commit opens no window")
	assert.True(t, alice.editor.Dirty())
}

// TestSessionWindowPollReportsTheEnd: AC-14 and AC-17 at the draft poll. The
// window a session saw open is carried through the command result (a
// command runs on a copy of the Model), so the poll stays quiet after the
// session's own accept, names another session's accept, and says a deadline
// revert timed out.
func TestSessionWindowPollReportsTheEnd(t *testing.T) {
	f := newWindowFixture(t)
	alice := f.model(t, "alice")
	other := f.model(t, "alice")

	require.NoError(t, alice.editor.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))
	result, err := alice.cmdCommitRequest(confirmedRequest(60, false))
	require.NoError(t, err)
	require.NotNil(t, result.windowWatch, "the opened window travels in the result")
	alice.applyResult(result)
	notice, ok := other.pollDaemonWindow()
	require.True(t, ok)
	assert.Contains(t, notice, "Confirm within", "another session of the owner sees the countdown")

	result, err = alice.cmdCommitRequest(contract.CommitRequest{Action: contract.CommitAccept})
	require.NoError(t, err)
	alice.applyResult(result)
	_, ok = alice.pollDaemonWindow()
	assert.False(t, ok, "the session that accepted has nothing more to say")
	notice, ok = other.pollDaemonWindow()
	require.True(t, ok)
	assert.Contains(t, notice, "closed by another session")

	require.NoError(t, alice.editor.SetValue([]string{"bgp"}, "router-id", "8.8.8.8"))
	result, err = alice.cmdCommitRequest(confirmedRequest(1, false))
	require.NoError(t, err)
	alice.applyResult(result)
	require.Eventually(t, func() bool { _, open := f.window.Status(); return !open }, 5*time.Second, 50*time.Millisecond)
	notice, ok = alice.pollDaemonWindow()
	require.True(t, ok)
	assert.Contains(t, notice, "Timeout: configuration automatically rolled back.")
	assert.NotContains(t, f.active(t), "8.8.8.8")
}
