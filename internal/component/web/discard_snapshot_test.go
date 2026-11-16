// Design: docs/architecture/web-interface.md -- discard snapshots own durable intent.
package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/core/audit"
)

// TestDiscardAuditSnapshotReadsAnEvictedUser proves that a missing cache entry
// is not an empty durable draft. Reading the snapshot changes neither owner's
// persisted intent and masks the current user's secret.
func TestDiscardAuditSnapshotReadsAnEvictedUser(t *testing.T) {
	mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
	us, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	adapter, ok := us.editor.(*testEditorAdapter)
	require.True(t, ok)
	oldTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	adapter.ed.SetSession(&cli.EditSession{User: "testuser", Origin: "web", ID: "testuser@web%2000-01-01T00:00:00Z", StartTime: oldTime})
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "mine"}, "description", "pending-owner"))
	const secret = "private-discard-snapshot-secret"
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "mine", "connection", "md5"}, "password", secret))
	require.NoError(t, mgr.SetValue("other", []string{"bgp", "group", "theirs"}, "description", "pending-other"))
	ownPath := cli.ChangePath(mgr.configPath, "testuser")
	otherPath := cli.ChangePath(mgr.configPath, "other")
	ownBefore, err := mgr.store.ReadFile(ownPath)
	require.NoError(t, err)
	otherBefore, err := mgr.store.ReadFile(otherPath)
	require.NoError(t, err)
	mgr.mu.Lock()
	us.lastActivity = oldTime
	mgr.evictInactive()
	_, cached := mgr.sessions["testuser"]
	mgr.mu.Unlock()
	require.False(t, cached)

	detail, err := mgr.discardDiff("testuser")
	require.NoError(t, err)
	assert.Contains(t, detail, "pending-owner")
	assert.Contains(t, detail, "password")
	assert.NotContains(t, detail, secret)
	assert.NotContains(t, detail, "pending-other")
	ownAfter, err := mgr.store.ReadFile(ownPath)
	require.NoError(t, err)
	assert.Equal(t, ownBefore, ownAfter)
	otherAfter, err := mgr.store.ReadFile(otherPath)
	require.NoError(t, err)
	assert.Equal(t, otherBefore, otherAfter)
}

// TestDiscardAuditSnapshotOpeningFailureStopsDiscard distinguishes a failed
// read from an empty successful snapshot, through the context helper and every
// discard transport, without dropping intent or recording a completed action.
func TestDiscardAuditSnapshotOpeningFailureStopsDiscard(t *testing.T) {
	mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "mine"}, "description", "pending-owner"))
	require.NoError(t, mgr.SetValue("other", []string{"bgp", "group", "theirs"}, "description", "pending-other"))
	ownPath := cli.ChangePath(mgr.configPath, "testuser")
	otherPath := cli.ChangePath(mgr.configPath, "other")
	ownBefore, err := mgr.store.ReadFile(ownPath)
	require.NoError(t, err)
	otherBefore, err := mgr.store.ReadFile(otherPath)
	require.NoError(t, err)
	committedBefore, err := mgr.committedConfig()
	require.NoError(t, err)
	mgr.mu.Lock()
	delete(mgr.sessions, "testuser")
	mgr.mu.Unlock()
	openFailure := errors.New("saved changes unavailable")
	factory := mgr.editorFactory
	mgr.editorFactory = func(any, string) (contract.Editor, error) {
		return nil, openFailure
	}
	detail, err := mgr.discardDiff("testuser")
	require.ErrorIs(t, err, openFailure)
	assert.Empty(t, detail)
	for _, line := range []string{"discard", "discard all", "top discard all"} {
		action, detail, err := terminalAuditContext(mgr, "testuser", parseCLICommand(line), line)
		require.ErrorIs(t, err, openFailure, line)
		assert.Equal(t, audit.ActionConfigDiscard, action)
		assert.Empty(t, detail)
	}
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	auth := fakeAuthorizer{allowEdit: true}
	for _, surface := range []struct {
		name, path string
		form       url.Values
		handler    http.HandlerFunc
	}{
		{"button", "/config/discard/", url.Values{}, HandleConfigDiscardWithAuthorizerAndAudit(mgr, auth, recorder)},
		{"terminal", "/cli/terminal", url.Values{"command": {"discard all"}}, HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, nil, nil, auth, recorder)},
		{"CLI bar", "/cli", url.Values{"command": {"discard all"}}, HandleCLICommandWithAuthorizerAndAudit(mgr, schema, nil, auth, recorder)},
		{"graphical action", "/config/action", url.Values{"action": {"discard"}}, HandleGraphicalAction(mgr, schema, nil, auth, recorder, nil)},
	} {
		t.Run(surface.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			surface.handler(response, authedRequest(t, http.MethodPost, surface.path, surface.form))
			assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
			assert.Empty(t, recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard}))
			ownAfter, err := mgr.store.ReadFile(ownPath)
			require.NoError(t, err)
			assert.Equal(t, ownBefore, ownAfter)
			otherAfter, err := mgr.store.ReadFile(otherPath)
			require.NoError(t, err)
			assert.Equal(t, otherBefore, otherAfter)
			committedAfter, err := mgr.committedConfig()
			require.NoError(t, err)
			assert.Equal(t, committedBefore, committedAfter)
		})
	}
	mgr.editorFactory = factory
	detail, err = mgr.discardDiff("clean-user")
	require.NoError(t, err)
	assert.Empty(t, detail, "a successful read may find no intent for this owner")
}

// TestUnauditedCommandsHaveNoDiscardSnapshot keeps the no-audit answer distinct
// from a failed discard read and proves the consumer guard ignores result flags
// on commands that do not own an audit action.
func TestUnauditedCommandsHaveNoDiscardSnapshot(t *testing.T) {
	result := terminalCommandResult{auditSuccess: true, discarded: true}
	for _, line := range []string{"show", "help", "option none", "set bgp router-id 192.0.2.1", "top show"} {
		cmd := parseCLICommand(line)
		action, detail, err := terminalAuditContext(nil, "testuser", cmd, line)
		require.NoError(t, err, line)
		assert.Empty(t, action, line)
		assert.Empty(t, detail, line)
		assert.False(t, terminalAuditSucceeded(cmd, result), line)
	}
	for _, line := range []string{"commit now", "discard all", "rollback 1", "top discard all"} {
		assert.True(t, terminalAuditSucceeded(parseCLICommand(line), result), line)
	}
}
