package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/audit"
)

// bracketMemberConfig is a config whose eth0 holds a bracket address list;
// addrs is its members.
func bracketMemberConfig(addrs string) string {
	return "bgp {\n    router-id 1.2.3.4;\n    session {\n        asn {\n            local 65001;\n        }\n    }\n}\n" +
		"interface {\n    backend netlink;\n    ethernet eth0 {\n        unit 0 {\n            description lab;\n" +
		"            ipv4 {\n                address [ " + addrs + " ];\n            }\n        }\n    }\n}\n"
}

// eth0AddressPath is the bracket address list of bracketMemberConfig.
const eth0AddressPath = "interface ethernet eth0 unit 0 ipv4 address"

// memberConflictManager answers a manager whose user testuser deactivated
// the member 192.0.2.2/30 that a concurrent commit then removed, so the
// user's next commit meets a member conflict with a fix line.
func memberConflictManager(t *testing.T) (*EditorManager, *config.Schema) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(bracketMemberConfig("192.0.2.1/24 192.0.2.2/30")), 0o600))
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	store := testConfigStore(t, configPath)
	mgr := NewEditorManager(store, configPath, schema, testEditorFactory(), testEditSessionFactory())
	_, err = mgr.ApplyActivation("testuser", strings.Fields(eth0AddressPath+" 192.0.2.2/30"), false)
	require.NoError(t, err)
	// The concurrent commit: the store, which the editor reads, drops the member.
	require.NoError(t, store.WriteFile(filepath.Base(configPath), []byte(bracketMemberConfig("192.0.2.1/24")), 0o600))
	return mgr, schema
}

// memberConflictFix is the fix line memberConflictManager's conflict prints.
const memberConflictFix = "    fix: discard " + eth0AddressPath + " 192.0.2.2/30 drops the toggle"

// TestWebTerminalDiscardPath proves that the web terminal's `discard <path>`
// drops the changes at that path and keeps the others, refuses a path that
// holds none, and that `discard` alone still drops them all.
// VALIDATES: round-6 BLOCKER 3, the terminal surface.
// PREVENTS: the fix line `discard <path> <member>` discarding every pending
// change on the web.
func TestWebTerminalDiscardPath(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	_, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, nil, nil, nil)
	runTerminalCommand(t, handler, "set bgp router-id 5.6.7.8")
	runTerminalCommand(t, handler, "set interface ethernet eth0 description changed")
	require.Equal(t, 2, mgr.ChangeCount("testuser"))

	resp := runTerminalCommand(t, handler, "discard interface ethernet eth0 description")
	assert.Equal(t, "discarded: interface ethernet eth0 description", resp.Output)
	assert.Equal(t, resp.Output, resp.Feedback, "a path discard's feedback names the path, never every change")
	require.Equal(t, 1, mgr.ChangeCount("testuser"), "discard <path> must drop that path only")

	resp = runTerminalCommand(t, handler, "discard interface ethernet eth0 description")
	assert.Contains(t, resp.Output, "you have no pending change at this path")
	assert.Contains(t, resp.Output, "`compare` lists your pending changes")
	assert.Equal(t, resp.Output, resp.Feedback, "a refused discard's feedback is the refusal")
	require.Equal(t, 1, mgr.ChangeCount("testuser"), "a refused discard drops nothing")

	resp = runTerminalCommand(t, handler, "discard")
	assert.Equal(t, terminalOutputChangesDiscarded, resp.Feedback)
	require.Equal(t, 0, mgr.ChangeCount("testuser"))
}

// TestWebTerminalDiscardPathAudited proves that every successful web terminal
// discard, `discard <path>` and `discard` alone, writes one config-discard
// audit row, and that a refused discard writes none. It drives the terminal
// handler with a memory recorder and counts the rows after each command.
// VALIDATES: round-7 BLOCKER 1.
// PREVENTS: a path discard bypassing the audit trail that SSH's
// recordConfigDiscard writes for the same command, and the row naming a
// change the path discard kept (round-8 ISSUE 1).
func TestWebTerminalDiscardPathAudited(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "10.0.0.2"))
	require.NoError(t, mgr.SetValue("testuser", []string{"interface", "ethernet", "eth0"}, "description", "changed"))
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, nil, nil, recorder)
	rows := func() []audit.Entry { return recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard}) }

	runTerminalCommand(t, handler, "discard nosuch path")
	require.Empty(t, rows(), "a refused discard is not audited")

	runTerminalCommand(t, handler, "discard interface ethernet eth0 description")
	require.Equal(t, 1, mgr.ChangeCount("testuser"))
	entries := rows()
	require.Len(t, entries, 1, "a successful discard <path> is audited")
	assert.Equal(t, "testuser", entries[0].Actor)
	assert.Equal(t, audit.Web, entries[0].Surface)
	assert.Contains(t, entries[0].Detail, "changed", "the row carries the diff the discard dropped")
	assert.NotContains(t, entries[0].Detail, "10.0.0.2", "the row never names the router-id change the path discard kept")

	runTerminalCommand(t, handler, "discard")
	require.Len(t, rows(), 2, "a successful discard is audited")
}

// TestWebCLIBarDiscardAudited proves that the web CLI bar's discard, with a
// path and alone, writes one config-discard audit row each time it succeeds,
// and none when it is refused. It drives the CLI bar handler with a memory
// recorder.
// VALIDATES: round-7 NOTE 4.
// PREVENTS: the CLI bar discarding changes with no audit row, while the
// terminal, the Discard button and SSH record the same command.
// PREVENTS: the row naming a change the path discard kept (round-8 ISSUE 1).
func TestWebCLIBarDiscardAudited(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "10.0.0.2"))
	require.NoError(t, mgr.SetValue("testuser", []string{"interface", "ethernet", "eth0"}, "description", "changed"))
	handler := HandleCLICommandWithAuthorizerAndAudit(mgr, schema, renderer, nil, recorder)
	run := func(command string) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, authedRequest(t, http.MethodPost, "/cli", url.Values{"command": {command}}))
	}
	rows := func() []audit.Entry { return recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard}) }

	run("discard nosuch path")
	require.Empty(t, rows(), "a refused discard is not audited")

	run("discard interface ethernet eth0 description")
	require.Equal(t, 1, mgr.ChangeCount("testuser"))
	entries := rows()
	require.Len(t, entries, 1, "a successful discard <path> is audited")
	assert.Equal(t, "testuser", entries[0].Actor)
	assert.Equal(t, audit.Web, entries[0].Surface)
	assert.Contains(t, entries[0].Detail, "changed", "the row carries the diff the discard dropped")
	assert.NotContains(t, entries[0].Detail, "10.0.0.2", "the row never names the router-id change the path discard kept")

	run("discard all")
	require.Equal(t, 0, mgr.ChangeCount("testuser"))
	require.Len(t, rows(), 2, "a successful discard all is audited")
}

// TestWebCLIBarDiscardPath proves the CLI bar's `discard <path>` drops only
// that path's changes and refuses a path holding none.
// VALIDATES: round-6 BLOCKER 3, the CLI bar surface.
// PREVENTS: handleCLIDiscard ignoring its arguments.
func TestWebCLIBarDiscardPath(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "5.6.7.8"))
	require.NoError(t, mgr.SetValue("testuser", []string{"interface", "ethernet", "eth0"}, "description", "changed"))
	require.Equal(t, 2, mgr.ChangeCount("testuser"))

	dispatch := func(args ...string) string {
		w := httptest.NewRecorder()
		r := authedRequest(t, http.MethodPost, "/cli", nil)
		dispatchCLICommand(w, r, cliCommand{Verb: verbDiscard, Args: args}, nil, mgr, schema, renderer, "testuser", nil)
		return w.Body.String()
	}
	dispatch("interface", "ethernet", "eth0", "description")
	require.Equal(t, 1, mgr.ChangeCount("testuser"), "discard <path> must drop that path only")
	body := dispatch("interface", "ethernet", "eth0", "description")
	assert.Contains(t, body, "you have no pending change at this path")
	require.Equal(t, 1, mgr.ChangeCount("testuser"))
}

// TestWebTerminalMemberConflictFixLine proves that the web terminal's
// `commit now` prints a member conflict's fix line, and that following it
// there resolves the conflict.
// VALIDATES: round-6 ISSUE 2, the web terminal; BLOCKER 3 end to end.
// PREVENTS: a fix line the web terminal does not print, or cannot follow.
func TestWebTerminalMemberConflictFixLine(t *testing.T) {
	mgr, schema := memberConflictManager(t)
	_, output := executeTerminalNav(schema, nil, mgr, "testuser", nil, cliCommand{Verb: verbCommit, Args: []string{"now"}})
	require.Contains(t, output, memberConflictFix)

	result := executeTerminalConfigMode(t.Context(), schema, nil, mgr, "testuser", "", nil,
		cliCommand{Verb: verbDiscard, Args: strings.Fields(eth0AddressPath + " 192.0.2.2/30")}, nil)
	require.Equal(t, "discarded: "+eth0AddressPath+" 192.0.2.2/30", result.output)
	assert.Equal(t, 0, mgr.ChangeCount("testuser"))
}

// TestWebCommitButtonMemberConflictFixLine proves that the web editor's
// Commit button renders a member conflict with its fix line.
// VALIDATES: round-6 ISSUE 2, the Commit button.
// PREVENTS: handleCommitPost rendering `want "...", other () has "..."` with
// no command that resolves the conflict.
func TestWebCommitButtonMemberConflictFixLine(t *testing.T) {
	mgr, _ := memberConflictManager(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleConfigCommitWithAuthorizerAndAudit(mgr, renderer, nil, nil, nil)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/config/commit/", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), ctxKeyUsername, "testuser"))
	req.Header.Set("HX-Request", htmxRequestTrue)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	assert.Contains(t, body, "member removed", "the commit met no member conflict, so this case proves nothing")
	assert.Contains(t, body, strings.TrimSpace(memberConflictFix))
}

// TestWebTerminalDiscardRefusesPipe proves that `discard | match x` in the
// web terminal is refused by name, drops nothing and writes no audit row. It
// runs the piped discard over one pending change and checks the answer, the
// change count and the recorder.
// VALIDATES: round-8 NOTE 2.
// PREVENTS: the refusal naming the pipe as a config path the operator never
// typed ("you have no pending change at this path").
func TestWebTerminalDiscardRefusesPipe(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "10.0.0.2"))
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, nil, nil, recorder)

	resp := runTerminalCommand(t, handler, "discard | match x")
	assert.Contains(t, resp.Output, "discard takes no pipe")
	assert.NotContains(t, resp.Output, "no pending change at this path")
	assert.Equal(t, 1, mgr.ChangeCount("testuser"), "a refused discard drops nothing")
	assert.Empty(t, recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard}), "a refused discard is not audited")
}
