package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
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
	require.Equal(t, 1, mgr.ChangeCount("testuser"), "discard <path> must drop that path only")

	resp = runTerminalCommand(t, handler, "discard interface ethernet eth0 description")
	assert.Contains(t, resp.Output, "you have no pending change at this path")
	assert.Contains(t, resp.Output, "`compare` lists your pending changes")
	require.Equal(t, 1, mgr.ChangeCount("testuser"), "a refused discard drops nothing")

	runTerminalCommand(t, handler, "discard")
	require.Equal(t, 0, mgr.ChangeCount("testuser"))
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
		dispatchCLICommand(w, r, cliCommand{Verb: verbDiscard, Args: args}, nil, mgr, schema, renderer, "testuser")
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

	_, output = executeTerminalNav(schema, nil, mgr, "testuser", nil, cliCommand{Verb: verbDiscard, Args: strings.Fields(eth0AddressPath + " 192.0.2.2/30")})
	require.Equal(t, "discarded: "+eth0AddressPath+" 192.0.2.2/30", output)
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
