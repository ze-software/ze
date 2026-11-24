// Design: docs/architecture/web-interface.md -- confirmed daemon lifecycle controls.

package web

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebCLIControlLifecycleConfirmsBeforeDispatch proves the browser controls
// only translate into canonical daemon commands after an explicit confirmation.
// The response-completion action is synthetic and never stops a real daemon.
func TestWebCLIControlLifecycleConfirmsBeforeDispatch(t *testing.T) {
	for _, tc := range []struct{ mode, line, command string }{
		{terminalModeOperational, "stop", "request shutdown"},
		{terminalModeOperational, "restart", "request restart"},
		{terminalModeConfig, "top stop", "request shutdown"},
		{terminalModeConfig, "top restart", "request restart"},
	} {
		t.Run(tc.mode+"/"+tc.line, func(t *testing.T) {
			mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
			terminalAsUser(t, mgr, "testuser", nil, "set bgp router-id 192.0.2.244")
			before := config.Serialize(mgr.snapshotTree("testuser"), schema)
			var seen []string
			completed := false
			dispatch := func(_ context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
				assert.Equal(t, "testuser", caller.Username)
				assert.Equal(t, "192.0.2.1:1234", caller.RemoteAddr)
				seen = append(seen, line)
				answer := plugin.NewResponse(plugin.StatusDone, plugin.Map{"message": "accepted lifecycle"})
				answer.OnTransportComplete(func() { completed = true })
				return answer, nil
			}
			handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, dispatch, nil, nil)
			form := url.Values{"command": {tc.line}, "mode": {tc.mode}, "path": {"bgp"}}
			first := cliControlJSON(t, handler, form)
			assert.NotEmpty(t, first["confirmation"])
			assert.Empty(t, seen, "an unconfirmed command must not dispatch")
			assert.False(t, completed)
			form.Set("confirm", "true")
			confirmed := cliControlJSON(t, handler, form)
			require.Equal(t, []string{tc.command}, seen)
			assert.Contains(t, confirmed["output"], "accepted lifecycle")
			assert.Empty(t, confirmed["confirmation"])
			assert.True(t, completed, "the transport must release the shared daemon action")
			assert.Equal(t, "bgp", confirmed["path"])
			assert.Equal(t, tc.mode, confirmed["mode"])
			assert.Equal(t, before, config.Serialize(mgr.snapshotTree("testuser"), schema))
		})
	}
}

// TestWebCLIControlLifecycleRetainsDispatcherDenial proves confirmation grants
// no authority: the canonical command still reaches the normal denial producer.
func TestWebCLIControlLifecycleRetainsDispatcherDenial(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	seen := ""
	dispatch := func(_ context.Context, _ plugin.CallerIdentity, line string) (*plugin.Response, error) {
		seen = line
		return &plugin.Response{Status: plugin.StatusError, Error: plugin.UnauthorizedMessage}, nil
	}
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, dispatch, nil, nil)
	answer := cliControlJSON(t, handler, url.Values{
		"command": {"restart"}, "mode": {terminalModeOperational}, "confirm": {"true"},
	})
	assert.Equal(t, "request restart", seen)
	assert.Contains(t, answer["output"], plugin.UnauthorizedMessage)
	assert.Empty(t, answer["confirmation"])
}
