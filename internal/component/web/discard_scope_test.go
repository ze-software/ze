// Design: docs/architecture/web-interface.md -- browser discard preserves SSH intent.
// Related: discard_evicted_audit_test.go -- resumed browser discard and audit.

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/audit"
)

// TestWebDiscardPreservesSameAccountSSHIntent drives both browser discard
// entry points after eviction. The account's saved SSH leaf, unsaved leaf and
// structural copy retain their owners; the resumed browser change disappears
// and its audit row names only the change actually discarded.
func TestWebDiscardPreservesSameAccountSSHIntent(t *testing.T) {
	for _, surface := range []string{"button", "terminal"} {
		t.Run(surface, func(t *testing.T) {
			mgr, schema := newPromotingEditorManagerOver(t,
				"bgp { router-id 10.0.0.250; session { asn { local 64999; } } "+
					"group original { description committed-original; } }\n")
			mgr.editorFactory = testEditorFactory()
			ssh, err := cli.NewEditorWithStorage(mgr.store, mgr.configPath)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, ssh.Close()) })
			started := time.Date(2000, 1, 1, 0, 0, 1, 0, time.UTC)
			ssh.SetSession(&cli.EditSession{User: "testuser", Origin: "ssh", StartTime: started,
				ID: "testuser@ssh%" + started.Format(time.RFC3339)})
			require.NoError(t, ssh.SetValue([]string{"bgp"}, "router-id", "10.0.0.88"))
			require.NoError(t, ssh.SaveDraft())
			draftBefore, err := mgr.store.ReadFile(cli.DraftPath(mgr.configPath))
			require.NoError(t, err)
			require.NoError(t, ssh.SetValue([]string{"bgp", "group", "original"}, "description", "keep-ssh"))
			require.NoError(t, ssh.CopyListEntry([]string{"bgp"}, "group", "original", []string{"ssh-copy"}))
			changePath := cli.ChangePath(mgr.configPath, "testuser")
			sshBefore, err := mgr.store.ReadFile(changePath)
			require.NoError(t, err)
			require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "10.0.0.99"))
			committedBefore, err := mgr.committedConfig()
			require.NoError(t, err)
			mgr.mu.Lock()
			delete(mgr.sessions, "testuser")
			mgr.mu.Unlock()
			recorder, err := audit.NewMemory(100)
			require.NoError(t, err)
			response := httptest.NewRecorder()
			if surface == "button" {
				HandleConfigDiscardWithAuthorizerAndAudit(mgr, fakeAuthorizer{allowEdit: true}, recorder).
					ServeHTTP(response, authedRequest(t, http.MethodPost, "/config/discard/", url.Values{}))
				require.Equal(t, http.StatusSeeOther, response.Code, response.Body.String())
			} else {
				HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, nil, nil, fakeAuthorizer{allowEdit: true}, recorder).
					ServeHTTP(response, postConfigRequest(t, "/cli/terminal", url.Values{"command": {"discard all"}, "mode": {terminalModeConfig}}, "testuser"))
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			}
			sshAfter, err := mgr.store.ReadFile(changePath)
			require.NoError(t, err, "the SSH change file remains")
			assert.Equal(t, sshBefore, sshAfter, "SSH leaves and structural operations retain exact ownership")
			draftAfter, err := mgr.store.ReadFile(cli.DraftPath(mgr.configPath))
			require.NoError(t, err)
			_, beforeMeta, err := config.NewSetParser(schema).ParseWithMeta(string(draftBefore))
			require.NoError(t, err)
			_, afterMeta, err := config.NewSetParser(schema).ParseWithMeta(string(draftAfter))
			require.NoError(t, err)
			assert.ElementsMatch(t, beforeMeta.SessionEntries(schema, ssh.SessionID()), afterMeta.SessionEntries(schema, ssh.SessionID()))
			_, err = mgr.GetOrCreate("testuser")
			require.NoError(t, err)
			assert.Zero(t, mgr.ChangeCount("testuser"), "reopening does not resurrect discarded browser intent")
			committedAfter, err := mgr.committedConfig()
			require.NoError(t, err)
			assert.Equal(t, committedBefore, committedAfter)
			rows := recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard})
			require.Len(t, rows, 1)
			assert.Contains(t, rows[0].Detail, "10.0.0.99")
			assert.NotContains(t, rows[0].Detail, "10.0.0.88")
			assert.NotContains(t, rows[0].Detail, "keep-ssh")
			assert.NotContains(t, rows[0].Detail, "ssh-copy")
		})
	}
}

// TestWebDiscardWithoutBrowserIntentPreservesSSH proves an account's SSH
// changes do not become browser work merely because no browser change exists.
// The HTTP discard succeeds without consuming or claiming those SSH edits.
func TestWebDiscardWithoutBrowserIntentPreservesSSH(t *testing.T) {
	mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
	mgr.editorFactory = testEditorFactory()
	ssh, err := cli.NewEditorWithStorage(mgr.store, mgr.configPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ssh.Close()) })
	ssh.SetSession(cli.NewEditSession("testuser", "ssh"))
	require.NoError(t, ssh.SetValue([]string{"bgp"}, "router-id", "10.0.0.88"))
	require.NoError(t, ssh.CreateEntry([]string{"bgp", "group", "ssh-empty"}))
	path := cli.ChangePath(mgr.configPath, "testuser")
	before, err := mgr.store.ReadFile(path)
	require.NoError(t, err)
	detail, err := mgr.discardDiff("testuser")
	require.NoError(t, err)
	assert.Empty(t, detail, "no browser intent means no discarded change detail")
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	response := httptest.NewRecorder()
	HandleConfigDiscardWithAuthorizerAndAudit(mgr, fakeAuthorizer{allowEdit: true}, recorder).
		ServeHTTP(response, authedRequest(t, http.MethodPost, "/config/discard/", url.Values{}))
	require.Equal(t, http.StatusSeeOther, response.Code, response.Body.String())
	after, err := mgr.store.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	rows := recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard})
	require.Len(t, rows, 1)
	assert.Empty(t, rows[0].Detail, "audit must not claim the SSH changes were discarded")
}
