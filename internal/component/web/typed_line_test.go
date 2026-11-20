// Design: docs/architecture/api/commands.md -- a rendering surface sends the typed line and renders the residual chain.
package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
)

// typedLineDispatch records each line and caller it is handed and answers a
// compact JSON document, so a surface that renders must indent it for | json.
func typedLineDispatch(lines *[]string, callers *[]plugin.CallerIdentity) CommandDispatcher {
	return func(_ context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
		*lines = append(*lines, line)
		*callers = append(*callers, caller)
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.RawJSON(`{"ok":"yes"}`)}, nil
	}
}

// TestWebSurfacesSendTheTypedLine asks whether the web CLI terminal and a
// tool page hand the dispatcher the line as typed, pipe chain included, state
// that they render the residual chain (plugin.CallerIdentity
// .RendersResidualChain), and render it over the answer themselves (package
// 3c of spec-pipe-chain-honored-on-every-command-surface).
//
// Method: a command path that declares one operation (`scope`) and a
// dispatcher that records what it receives. The line `<path> | scope | json`
// must reach it whole with the rendering statement set, and the surface's
// output must be the document as `| json` renders it (indented).
//
// MUTATION: dispatch the command part the formatter returns, or drop the
// rendering statement; the recorded line or caller then differs.
func TestWebSurfacesSendTheTypedLine(t *testing.T) {
	const path = "show test web-typed-line"
	command.RegisterPipeFilters([]string{path},
		command.PipeFilter{Name: "scope", Description: "Pick a scope", Argument: command.ArgNone})
	const line = path + " | scope | json"

	t.Run("terminal", func(t *testing.T) {
		var lines []string
		var callers []plugin.CallerIdentity
		output, rendered, _, _, err := executeTerminalOperationalResult(t.Context(), typedLineDispatch(&lines, &callers), "alice", "127.0.0.1", line, "")
		require.NoError(t, err)
		defer rendered.TransportComplete()
		require.Equal(t, []string{line}, lines, "the terminal did not dispatch the typed line")
		assert.True(t, callers[0].RendersResidualChain, "the terminal did not state that it renders the chain")
		assert.Contains(t, output, `"ok": "yes"`, "the terminal did not render | json over the answer")
	})

	t.Run("tool page", func(t *testing.T) {
		var lines []string
		var callers []plugin.CallerIdentity
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/tools/ping/", nil)
		data := dispatchToolCommand(request, typedLineDispatch(&lines, &callers), line)
		require.Empty(t, data.Error)
		require.Equal(t, []string{line}, lines, "the tool page did not dispatch the typed line")
		assert.True(t, callers[0].RendersResidualChain, "the tool page did not state that it renders the chain")
		assert.Contains(t, data.Output, `"ok": "yes"`, "the tool page did not render | json over the answer")
	})
}
