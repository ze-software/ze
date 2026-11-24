// Design: docs/architecture/web-interface.md -- operational command precedence.

package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebRegisteredOperationalDeleteReachesDispatcher proves command registry
// precedence preserves caller identity and leaves the candidate untouched.
func TestWebRegisteredOperationalDeleteReachesDispatcher(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	_, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	before := config.Serialize(mgr.snapshotTree("testuser"), schema)
	const line = "delete agent skill agent all"
	var caller plugin.CallerIdentity
	var seen string
	dispatch := func(_ context.Context, identity plugin.CallerIdentity, input string) (*plugin.Response, error) {
		caller, seen = identity, input
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"result": "daemon answer"}), nil
	}
	registered := func(input string) bool { return input == line }
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, dispatch, fakeAuthorizer{}, nil, registered)
	answer := runTerminalCommandForm(t, handler, url.Values{"command": {line}, "mode": {"operational"}})
	assert.Equal(t, line, seen)
	assert.Equal(t, "testuser", caller.Username)
	assert.Equal(t, "192.0.2.1:1234", caller.RemoteAddr)
	assert.Contains(t, answer.Output, "daemon answer")
	assert.Equal(t, terminalModeOperational, answer.Mode)
	assert.Equal(t, before, config.Serialize(mgr.snapshotTree("testuser"), schema))
}

// TestWebStreamSelectionPreservesRegisteredOperationalInput proves stream
// discovery uses the same domain decision before finite-command fallback.
func TestWebStreamSelectionPreservesRegisteredOperationalInput(t *testing.T) {
	const line = "delete agent skill agent all"
	seen := ""
	finite := false
	handler := HandleCLITerminalStream(nil, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		finite = true
		w.WriteHeader(http.StatusNoContent)
	}), func(_ context.Context, _ plugin.CallerIdentity, input string, _ io.Writer) (bool, error) {
		seen = input
		return false, nil
	}, func(input string) bool { return input == line })
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, authedRequest(t, http.MethodPost, "/cli/stream", url.Values{"command": {line}, "mode": {"operational"}}))
	assert.Equal(t, line, seen)
	assert.True(t, finite)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

// TestWebRegisteredOperationalDeletePreservesDaemonDenial proves a command
// reaching operational dispatch preserves its denial response without claiming
// a successful config deletion. This test exercises the transport projection;
// the actual daemon authorization decision is outside its dispatcher seam.
func TestWebRegisteredOperationalDeletePreservesDaemonDenial(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	const line = "delete agent skill agent all"
	called := false
	dispatch := func(_ context.Context, _ plugin.CallerIdentity, input string) (*plugin.Response, error) {
		called = true
		assert.Equal(t, line, input)
		return &plugin.Response{Status: plugin.StatusError, Error: plugin.UnauthorizedMessage}, nil
	}
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, dispatch, fakeAuthorizer{}, nil, func(string) bool { return true })
	answer := runTerminalCommandForm(t, handler, url.Values{"command": {line}, "mode": {"operational"}})
	assert.True(t, called)
	assert.Contains(t, answer.Output, plugin.UnauthorizedMessage)
	assert.NotContains(t, answer.Feedback, "deleted")
	assert.Zero(t, mgr.ChangeCount("testuser"))
}

// TestWebOperationalRecognitionIsOneRequestDecision simulates registry removal
// between authorization and execution; it must not turn an admitted daemon
// command into a configuration mutation without configuration permission.
func TestWebOperationalRecognitionIsOneRequestDecision(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	_, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	before := config.Serialize(mgr.snapshotTree("testuser"), schema)
	const line = "delete bgp"
	lookups := 0
	registered := func(string) bool {
		lookups++
		return lookups == 1
	}
	seen := ""
	dispatch := func(_ context.Context, _ plugin.CallerIdentity, input string) (*plugin.Response, error) {
		seen = input
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"result": "operational answer"}), nil
	}
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, dispatch, fakeAuthorizer{}, nil, registered)
	answer := runTerminalCommandForm(t, handler, url.Values{"command": {line}, "mode": {"operational"}})
	assert.Equal(t, 1, lookups)
	assert.Equal(t, line, seen)
	assert.Equal(t, terminalModeOperational, answer.Mode)
	assert.Contains(t, answer.Output, "operational answer")
	assert.Equal(t, before, config.Serialize(mgr.snapshotTree("testuser"), schema))
}

// TestWebConfigDeleteRetainsMutationAuthorization checks registry precedence
// applies only in operational mode, and config convenience still enters it.
func TestWebConfigDeleteRetainsMutationAuthorization(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	called := false
	dispatch := func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		called = true
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{}), nil
	}
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, dispatch, fakeAuthorizer{}, nil, func(string) bool { return true })
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, authedRequest(t, http.MethodPost, "/cli/terminal", url.Values{"command": {"delete bgp"}, "mode": {"config"}}))
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.False(t, called)
	allowed := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, dispatch, fakeAuthorizer{allowEdit: true}, nil, func(string) bool { return false })
	answer := runTerminalCommandForm(t, allowed, url.Values{"command": {"set bgp router-id 192.0.2.44"}, "mode": {"operational"}})
	assert.Equal(t, terminalModeConfig, answer.Mode)
	assert.Contains(t, runTerminalCommandForm(t, allowed, url.Values{"command": {"show"}, "mode": {"config"}}).Output, "192.0.2.44")
	assert.False(t, called)
}
