// Design: docs/guide/audit.md -- browser commit recording on both command transports.
package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/audit"
)

// TestBrowserCommandCommitAudit follows the successful HTTP request through
// publication and, for confirmed commits, the actual daemon window. Its paired
// refusal must retain intent and committed bytes and add no audit row.
func TestBrowserCommandCommitAudit(t *testing.T) {
	for _, surface := range []string{"terminal confirmed", "CLI bar"} {
		t.Run(surface, func(t *testing.T) {
			mgr, schema, window := newWindowEditorManager(t)
			publications := 0
			mgr.SetCommitHook(func() error {
				if err := storage.PromoteCandidate(mgr.store, mgr.configPath); err != nil {
					return err
				}
				publications++
				return nil
			})
			hash, err := bcrypt.GenerateFromPassword([]byte("private-audit-password"), bcrypt.MinCost)
			require.NoError(t, err)
			require.NoError(t, mgr.SetValue("alice", []string{"system", "authentication", "user", "audit-user"}, "password", string(hash)))
			require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "192.0.2.10"))
			recorder, err := audit.NewMemory(100)
			require.NoError(t, err)
			authorizer := adminWebAuthorizer()
			var handler http.Handler
			command, route := "commit confirmed 60", "/cli/terminal"
			if surface == "terminal confirmed" {
				handler = HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, nil, nil, authorizer, recorder)
			} else {
				renderer, err := NewRenderer()
				require.NoError(t, err)
				handler = HandleCLICommandWithAuthorizerAndAudit(mgr, schema, renderer, authorizer, recorder)
				command, route = "commit now", "/cli"
			}
			detail, err := mgr.Diff("alice")
			require.NoError(t, err)
			require.Contains(t, detail, "+ bgp router-id 192.0.2.10")
			require.Contains(t, detail, "password "+config.SecretDataPlaceholder)
			require.NotContains(t, detail, string(hash))
			response := httptest.NewRecorder()
			request := postConfigRequest(t, route, url.Values{"command": {command}, "mode": {terminalModeConfig}}, "alice")
			handler.ServeHTTP(response, request)
			if surface == "terminal confirmed" {
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				var answer terminalResponse
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
				assert.Contains(t, answer.Output, "commit accept")
				status, open := window.Status()
				require.True(t, open)
				assert.Equal(t, "alice", status.User)
			} else {
				require.Equal(t, http.StatusSeeOther, response.Code, response.Body.String())
				_, open := window.Status()
				assert.False(t, open)
			}
			assert.Equal(t, 1, publications)
			requireCommitted(t, mgr, "192.0.2.10", "")
			committed, err := mgr.committedConfig()
			require.NoError(t, err)
			assert.Contains(t, string(committed), string(hash), "the real secret must publish while its audit is masked")
			rows := recorder.Query(audit.Filter{Action: audit.ActionConfigCommit})
			require.Len(t, rows, 1)
			assert.Equal(t, "alice", rows[0].Actor)
			assert.Equal(t, audit.Web, rows[0].Surface)
			assert.Equal(t, audit.ActionConfigCommit, rows[0].Action)
			assert.Equal(t, audit.OutcomeSuccess, rows[0].Outcome)
			assert.Equal(t, request.RemoteAddr, rows[0].RemoteAddr)
			assert.Equal(t, detail, rows[0].Detail)

			// An invalid commit grammar is refused by both transports even
			// with pending work, independently of window ownership.
			require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "192.0.2.20"))
			pending := pendingProgramBytes(t, mgr, "alice")
			refused := httptest.NewRecorder()
			handler.ServeHTTP(refused, postConfigRequest(t, route, url.Values{"command": {"commit unexpected"}, "mode": {terminalModeConfig}}, "alice"))
			assert.Equal(t, http.StatusOK, refused.Code, refused.Body.String())
			assert.Equal(t, pending, pendingProgramBytes(t, mgr, "alice"))
			assert.Equal(t, 1, publications)
			assert.Equal(t, rows, recorder.Query(audit.Filter{Action: audit.ActionConfigCommit}))
		})
	}
}

// TestWebRollbackStagesBeforeTheCommitHook observes the accepted rollback
// alternative through the real terminal HTTP route: restore candidate, review,
// then publish through the same commit hook as any other edit.
func TestWebRollbackStagesBeforeTheCommitHook(t *testing.T) {
	mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
	backup := strings.ReplaceAll(windowCommittedConfig, "10.0.0.250", "192.0.2.30")
	require.NoError(t, mgr.store.WriteVersion(mgr.configPath, []byte(backup), time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)))
	publications := 0
	mgr.SetCommitHook(func() error {
		if err := storage.PromoteCandidate(mgr.store, mgr.configPath); err != nil {
			return err
		}
		publications++
		return nil
	})
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, nil, nil, adminWebAuthorizer(), recorder)
	run := func(line string) terminalResponse {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, postConfigRequest(t, "/cli/terminal", url.Values{"command": {line}, "mode": {terminalModeConfig}}, "alice"))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var answer terminalResponse
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
		return answer
	}
	before, err := mgr.committedConfig()
	require.NoError(t, err)
	answer := run("rollback 1")
	assert.NotContains(t, answer.Output, "error:")
	assert.Zero(t, publications, "restoring a session candidate must not publish before review")
	unchanged, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, before, unchanged)
	assert.Empty(t, recorder.Query(audit.Filter{Action: audit.ActionConfigCommit}))
	assert.Contains(t, mgr.ContentAtPath("alice", []string{"bgp"}), "192.0.2.30")
	answer = run("show | compare")
	assert.Contains(t, answer.Output, "- bgp router-id 10.0.0.250")
	assert.Contains(t, answer.Output, "+ bgp router-id 192.0.2.30")
	answer = run("commit now")
	assert.Equal(t, terminalOutputCommitSuccessful, answer.Output)
	assert.Equal(t, 1, publications)
	requireCommitted(t, mgr, "192.0.2.30", "10.0.0.250")
	rows := recorder.Query(audit.Filter{Action: audit.ActionConfigCommit})
	require.Len(t, rows, 1)
	assert.Contains(t, rows[0].Detail, "+ bgp router-id 192.0.2.30")
}
