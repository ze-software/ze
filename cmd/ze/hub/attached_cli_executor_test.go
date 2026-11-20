// Design: docs/architecture/api/commands.md -- a rendering surface sends the typed line and renders the residual chain.
// Related: session_editor.go -- attachedCLIExecutor, the `ze start --cli` console executor.

package hub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestAttachedCLIExecutorRendersResidualChain asks whether the `ze start
// --cli` console executor sends the line as typed, as a caller that renders
// the residual chain. The attached model applies that chain to the answer
// itself (cli.CommandExecutor), so a dispatch without the statement would
// apply it twice.
//
// Method: attachedCLIExecutor over a dispatcher that records the caller and
// the line, for a line carrying a pipe chain.
//
// MUTATION: drop RendersResidualChain from attachedCLIExecutor, or send the
// command part alone; either assertion fails.
func TestAttachedCLIExecutorRendersResidualChain(t *testing.T) {
	const line = "show fixture attached rows | match a | count"
	var seen []plugin.CallerIdentity
	var sent []string
	dispatch := plugin.CommandDispatcher(func(_ context.Context, caller plugin.CallerIdentity, input string) (*plugin.Response, error) {
		seen = append(seen, caller)
		sent = append(sent, input)
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"ok": true}), nil
	})

	output, err := attachedCLIExecutor(dispatch)(line)
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, output.Text, "the dispatcher's JSON, for the model to render")

	require.Len(t, seen, 1)
	assert.True(t, seen[0].RendersResidualChain, "the attached model renders the residual chain, so the dispatch must say so")
	assert.Equal(t, "root", seen[0].Username)
	assert.Equal(t, "local", seen[0].RemoteAddr)
	assert.Equal(t, []string{line}, sent, "the typed line, pipe chain included")
}
