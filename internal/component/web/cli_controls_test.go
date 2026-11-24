// Design: docs/architecture/web-interface.md -- browser equivalents of CLI controls.

package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebCLIControlClearBypassesStreamDispatch proves the browser stream entry
// also keeps display clear local, while a real clear command still dispatches.
func TestWebCLIControlClearBypassesStreamDispatch(t *testing.T) {
	for _, tc := range []struct {
		mode, line string
		dispatches int
	}{
		{terminalModeOperational, "clear", 0},
		{terminalModeConfig, "run clear", 0},
		{terminalModeConfig, "top run clear", 0},
		{terminalModeOperational, "clear bgp peer 192.0.2.1", 1},
	} {
		t.Run(tc.mode+"/"+tc.line, func(t *testing.T) {
			calls := 0
			finite := false
			handler := HandleCLITerminalStream(nil, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				finite = true
				w.WriteHeader(http.StatusNoContent)
			}), func(context.Context, plugin.CallerIdentity, string, io.Writer) (bool, error) {
				calls++
				return false, nil
			})
			w := httptest.NewRecorder()
			handler(w, authedRequest(t, http.MethodPost, "/cli/stream", url.Values{"command": {tc.line}, "mode": {tc.mode}}))
			assert.Equal(t, tc.dispatches, calls)
			assert.True(t, finite)
			assert.Equal(t, http.StatusNoContent, w.Code)
		})
	}
}

// TestWebCLIControlClearKeepsCandidateAndPosition exercises Enter through HTTP;
// clearing a display must neither dispatch a daemon action nor discard a draft.
func TestWebCLIControlClearKeepsCandidateAndPosition(t *testing.T) {
	for _, tc := range []struct{ mode, line string }{
		{terminalModeConfig, "clear"},
		{terminalModeConfig, "top clear"},
		{terminalModeConfig, "run clear"},
		{terminalModeConfig, "top run clear"},
		{terminalModeOperational, "clear"},
	} {
		t.Run(tc.mode+"/"+tc.line, func(t *testing.T) {
			mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
			terminalAsUser(t, mgr, "testuser", nil, "set bgp router-id 192.0.2.241")
			before := config.Serialize(mgr.snapshotTree("testuser"), schema)
			calls := 0
			dispatch := func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
				calls++
				return plugin.NewResponse(plugin.StatusDone, plugin.Map{"unexpected": "daemon clear"}), nil
			}
			handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, dispatch, nil, nil)
			answer := runTerminalCommandForm(t, handler, url.Values{
				"command": {tc.line}, "mode": {tc.mode}, "path": {"bgp"},
			})
			assert.Zero(t, calls, "clear is a display control, not a daemon command")
			assert.Equal(t, tc.mode, answer.Mode)
			assert.Equal(t, "bgp", answer.Path)
			assert.NotContains(t, answer.Output, "error")
			assert.Equal(t, before, config.Serialize(mgr.snapshotTree("testuser"), schema))
			if tc.mode == terminalModeConfig {
				assert.Contains(t, answer.Output, "192.0.2.241", "SSH clear restores the current candidate view")
			} else {
				assert.Empty(t, answer.Output)
			}
		})
	}
}

// TestWebCLIControlOperationalHelpUsesSharedCatalog compares the HTTP answer to
// the CLI's actual command catalog, including future command-table additions.
func TestWebCLIControlOperationalHelpUsesSharedCatalog(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, nil, nil, nil)
	answer := runTerminalCommandForm(t, handler, url.Values{"command": {"help"}, "mode": {terminalModeOperational}})
	for _, entry := range cli.EditorHelpEntries() {
		assert.Contains(t, answer.Output, entry.Name)
		assert.Contains(t, answer.Output, entry.Desc)
	}
	assert.Equal(t, terminalModeOperational, answer.Mode)
}

// TestWebCLIControlOperationalExitNavigates follows the real JSON adapter;
// ending a browser CLI session returns to the primary graphical editor.
func TestWebCLIControlOperationalExitNavigates(t *testing.T) {
	for _, line := range []string{"exit", "quit"} {
		t.Run(line, func(t *testing.T) {
			mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
			handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, nil, nil, nil)
			answer := cliControlJSON(t, handler, url.Values{"command": {line}, "mode": {terminalModeOperational}})
			assert.Equal(t, "/show/", answer["navigate"])
			assert.NotContains(t, answer["output"], "not available")
		})
	}
}

// TestWebCLIControlPendingExitRequiresConfirmation keeps the candidate after
// both refusal and explicit confirmation; leaving a mode never discards work.
func TestWebCLIControlPendingExitRequiresConfirmation(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	terminalAsUser(t, mgr, "testuser", nil, "set bgp router-id 192.0.2.242")
	before := config.Serialize(mgr.snapshotTree("testuser"), schema)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, nil, nil, nil)
	form := url.Values{"command": {"exit"}, "mode": {terminalModeConfig}, "path": {"bgp"}}
	first := cliControlJSON(t, handler, form)
	assert.NotEmpty(t, first["confirmation"], "the operator must be offered the same force-exit choice as SSH")
	assert.Equal(t, terminalModeConfig, first["mode"])
	assert.Empty(t, first["navigate"])
	assert.Equal(t, before, config.Serialize(mgr.snapshotTree("testuser"), schema))
	form.Set("confirm", "true")
	confirmed := cliControlJSON(t, handler, form)
	assert.Equal(t, terminalModeOperational, confirmed["mode"])
	assert.Empty(t, confirmed["confirmation"])
	assert.Equal(t, "bgp", confirmed["path"])
	assert.Equal(t, before, config.Serialize(mgr.snapshotTree("testuser"), schema))
}

// cliControlJSON deliberately decodes existing HTTP JSON rather than requiring
// a new Go response field before the behavioral RED run.
func cliControlJSON(t *testing.T, handler http.HandlerFunc, form url.Values) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	handler(w, authedRequest(t, http.MethodPost, "/cli/terminal", form))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var answer map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
	return answer
}
