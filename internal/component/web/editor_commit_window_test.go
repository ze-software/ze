package web

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// nopRecorder persists nothing: these tests never restart the daemon.
type nopRecorder struct{}

func (nopRecorder) Save(confirm.Pending) error { return nil }
func (nopRecorder) Clear() error               { return nil }

// newWindowEditorManager is a promoting manager wired to a daemon window whose
// revert stages the rollback and promotes it, as the hub's confirmRevert does.
func newWindowEditorManager(t *testing.T) (*EditorManager, *config.Schema, *confirm.Window) {
	t.Helper()
	mgr, schema := newPromotingEditorManager(t, "# ze config\n")
	window := confirm.NewWindow(func(rollback []byte) error {
		if _, err := storage.WriteCandidateVersion(mgr.store, mgr.configPath, rollback, time.Now()); err != nil {
			return err
		}
		return storage.PromoteCandidate(mgr.store, mgr.configPath)
	}, nopRecorder{})
	t.Cleanup(window.Stop)
	mgr.SetConfirmWindow(func() *confirm.Window { return window })
	return mgr, schema, window
}

// webCommit types `commit <args>` into the web terminal as user.
func webCommit(schema *config.Schema, mgr *EditorManager, user string, args ...string) string {
	_, output := executeTerminalNav(schema, nil, mgr, user, nil, cliCommand{Verb: verbCommit, Args: args})
	return output
}

func requireCommitted(t *testing.T, mgr *EditorManager, want, lacks string) {
	t.Helper()
	committed, err := mgr.committedConfig()
	require.NoError(t, err)
	if want != "" {
		assert.Contains(t, string(committed), want)
	}
	if lacks != "" {
		assert.NotContains(t, string(committed), lacks)
	}
}

// TestWebTerminalCommitThroughDaemonWindow is item 4 of
// spec-session-editor-file-mode-parity: the web terminal commits through the
// daemon's confirmed-commit window, the one the SSH session editor uses.
//
// GOAL: a web user opens, extends, aborts and accepts a window, and the window
// refuses everyone else by its owner rules.
// METHOD: a promoting manager with a real confirm.Window whose revert promotes
// the rollback; each line goes through executeTerminalNav.
//
// VALIDATES: AC-13 (confirmed opens a window), AC-15 (accept keeps), AC-16
// (abort reverts), AC-18 (the window belongs to its user: commit now and
// commit confirmed refused for others, commit now refused for the owner),
// AC-23 (nested force adds and resets, abort reverts to before the first),
// AC-26 (verify applies nothing), from the web terminal.
// PREVENTS: the web terminal refusing confirmed as unsupported, or committing
// around a window another user owns.
func TestWebTerminalCommitThroughDaemonWindow(t *testing.T) {
	mgr, schema, window := newWindowEditorManager(t)

	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.1"))
	require.Equal(t, terminalOutputCommitSuccessful, webCommit(schema, mgr, "alice", "now"))

	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.2"))
	assert.Contains(t, webCommit(schema, mgr, "alice", "confirmed", "60"), "commit accept")
	requireCommitted(t, mgr, "10.0.0.2", "")
	status, open := window.Status()
	require.True(t, open, "commit confirmed opened the daemon window")
	assert.Equal(t, "alice", status.User)

	require.NoError(t, mgr.SetValue("bob", []string{"bgp", "session", "asn"}, "local", "65001"))
	assert.Contains(t, webCommit(schema, mgr, "bob", "now"), "a confirmed commit by alice")
	assert.Contains(t, webCommit(schema, mgr, "bob", "confirmed", "60"), "a confirmed commit by alice")
	assert.Contains(t, webCommit(schema, mgr, "bob", "accept"), "a confirmed commit by alice")
	requireCommitted(t, mgr, "", "65001")

	require.NoError(t, mgr.SetValue("alice", []string{"bgp", "session", "asn"}, "local", "65000"))
	assert.Contains(t, webCommit(schema, mgr, "alice", "now"), confirm.ErrPending.Error())
	assert.Contains(t, webCommit(schema, mgr, "alice", "confirmed", "60"), confirm.ErrPending.Error())
	assert.Contains(t, webCommit(schema, mgr, "alice", "confirmed", "30", "force"), "commit accept")
	requireCommitted(t, mgr, "65000", "")
	status, open = window.Status()
	require.True(t, open)
	assert.LessOrEqual(t, status.Left(), 30*time.Second, "the nested force reset the countdown")

	assert.Contains(t, webCommit(schema, mgr, "alice", "abort"), "rolled back")
	requireCommitted(t, mgr, "10.0.0.1", "10.0.0.2")
	requireCommitted(t, mgr, "", "65000")
	assert.Contains(t, mgr.ContentAtPath("alice", []string{"bgp"}), "10.0.0.1", "abort rebuilt alice's view")
	assert.Contains(t, webCommit(schema, mgr, "alice", "abort"), confirm.ErrNoWindow.Error())

	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.3"))
	assert.Contains(t, webCommit(schema, mgr, "alice", "confirmed", "60"), "commit accept")
	assert.Contains(t, webCommit(schema, mgr, "alice", "accept"), "accepted")
	_, open = window.Status()
	assert.False(t, open, "commit accept closed the window")
	requireCommitted(t, mgr, "10.0.0.3", "")

	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.4"))
	assert.Contains(t, webCommit(schema, mgr, "alice", "verify"), "valid")
	assert.Equal(t, 1, mgr.ChangeCount("alice"), "commit verify applied the change")
	requireCommitted(t, mgr, "", "10.0.0.4")
}

// TestWebTerminalCommitConfirmedNeedsDaemonWindow proves a web editor with no
// daemon window refuses `commit confirmed` with the SSH editor's answer and
// applies nothing.
//
// VALIDATES: AC-29 (web): the refusal names what is missing.
// PREVENTS: a web commit confirmed that applies with no window to revert it.
func TestWebTerminalCommitConfirmedNeedsDaemonWindow(t *testing.T) {
	mgr, schema := newPromotingEditorManager(t, "# ze config\n")
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.5"))
	assert.Contains(t, webCommit(schema, mgr, "alice", "confirmed", "60"), "needs a daemon")
	requireCommitted(t, mgr, "", "10.0.0.5")
}

// TestWebTerminalCommitVerifyReportsInvalid proves `commit verify` runs the
// validator the commit runs: an invalid view is reported and nothing applies.
//
// VALIDATES: AC-26 (web): verify names the validation error.
// PREVENTS: a verify that answers "valid" without validating.
func TestWebTerminalCommitVerifyReportsInvalid(t *testing.T) {
	mgr, schema := newPromotingEditorManager(t, "# ze config\n")
	installPeerValidator(t, func(*config.Tree) error { return errors.New("peer check refused") })
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.6"))
	output := webCommit(schema, mgr, "alice", "verify")
	assert.Contains(t, output, "peer check refused")
	assert.NotContains(t, output, "valid;")
	assert.Equal(t, 1, mgr.ChangeCount("alice"))
	requireCommitted(t, mgr, "", "10.0.0.6")
}
