// Design: docs/architecture/cli/command-completion.md -- mode-specific browser completion.

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWebConfigCompletionKeepsSessionFormatOutOfContext proves that completing
// a config command does not offer an operational preference. A sole `set`
// candidate lets the browser's question-mark handler advance to the schema
// children at the current edit position.
// PREVENTS: `set cli format` turning that sole match into an unrelated menu.
func TestWebConfigCompletionKeepsSessionFormatOutOfContext(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	terminal := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, nil, nil, nil)
	created := runTerminalCommandForm(t, terminal, url.Values{
		"command": {"set environment daemon user browser-nav"}, "mode": {terminalModeConfig},
	})
	require.Equal(t, "set environment daemon user browser-nav", created.Output)
	edited := runTerminalCommandForm(t, terminal, url.Values{
		"command": {"edit environment"}, "mode": {terminalModeConfig},
	})
	require.Equal(t, "environment", edited.Path)
	require.Contains(t, edited.Prompt, "ze[environment]#")

	handler := HandleCLICompleteWithCommandCompleter(newCompleter(t), &fakeCommandCompleter{}, mgr, schema)
	complete := func(input, mode string) []string {
		t.Helper()
		query := url.Values{"input": {input}, "path": {edited.Path}, "mode": {mode}}
		w := httptest.NewRecorder()
		handler(w, authedRequest(t, http.MethodGet, "/cli/complete?"+query.Encode(), nil))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var items []struct {
			Text string `json:"text"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
		texts := make([]string, len(items))
		for i, item := range items {
			texts[i] = item.Text
		}
		return texts
	}

	assert.Equal(t, []string{"set"}, complete("set", terminalModeConfig))
	children := complete("set ", terminalModeConfig)
	assert.Contains(t, children, "daemon")
	assert.Contains(t, children, "log")
	assert.NotContains(t, children, "set cli format")
	assert.Contains(t, complete("set", terminalModeOperational), "set cli format")
}
