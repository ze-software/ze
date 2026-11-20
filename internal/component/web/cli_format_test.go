package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: AC-33 -- the browser uses the SSH session format command and
// carries its choice across requests and mode switches, independently per user.
// PREVENTS: routing operational `set cli format` into configuration mutations,
// changing the process default, or ignoring the choice during dispatch.
func TestTerminalSessionFormat(t *testing.T) {
	t.Setenv("ZE_CLI_FORMAT", "yaml")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	var callers []plugin.CallerIdentity
	// The terminal dispatches the line as typed, chain included, so the one
	// line that types `| yaml` sets the line the dispatcher must see.
	dispatched := "show format probe"
	dispatch := func(_ context.Context, caller plugin.CallerIdentity, input string) (*plugin.Response, error) {
		assert.Equal(t, dispatched, input)
		callers = append(callers, caller)
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"answer": "kept"}), nil
	}
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, dispatch, fakeAuthorizer{}, nil)
	run := func(user, mode, line string) terminalResponse {
		t.Helper()
		request := postConfigRequest(t, "/cli/terminal", url.Values{"command": {line}, "mode": {mode}}, user)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var answer terminalResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &answer))
		return answer
	}

	assert.Equal(t, "cli format: yaml", run("alice", "operational", "set cli format").Output)
	assert.Equal(t, "cli format set to json", run("alice", "operational", "set cli format json").Output)
	assert.JSONEq(t, `{"answer":"kept"}`, run("alice", "operational", "show format probe").Output)
	assert.Equal(t, "cli format: yaml", run("bob", "operational", "set cli format").Output)
	assert.Contains(t, run("bob", "operational", "show format probe").Output, "answer: kept")
	dispatched = "show format probe | yaml"
	assert.Contains(t, run("alice", "operational", "show format probe | yaml").Output, "answer: kept")
	dispatched = "show format probe"
	assert.Equal(t, "cli format: json", run("alice", "operational", "set cli format").Output)
	invalid := run("alice", "operational", "set cli format bogus")
	assert.Contains(t, invalid.Output, `cli format: "bogus" is not an accepted value`)
	assert.Contains(t, invalid.Output, "fix: set cli format <text|table|json|yaml|ndjson>")
	assert.Equal(t, "cli format: json", run("alice", "operational", "set cli format").Output)
	assert.Equal(t, terminalModeConfig, run("alice", "operational", "configure").Mode)
	assert.JSONEq(t, `{"answer":"kept"}`, run("alice", "config", "run show format probe").Output)
	assert.Equal(t, terminalModeOperational, run("alice", "config", "exit").Mode)
	assert.Equal(t, "cli format: json", run("alice", "operational", "set cli format").Output)
	assert.Equal(t, "yaml", env.Get("ze.cli.format"))
	assert.Zero(t, mgr.ChangeCount("alice"))
	assert.Zero(t, mgr.ChangeCount("bob"))
	require.Len(t, callers, 4)
	assert.Equal(t, []string{"alice", "bob", "alice", "alice"}, []string{callers[0].Username, callers[1].Username, callers[2].Username, callers[3].Username})
}

// VALIDATES: AC-33 -- config-mode set retains the shared schema command's
// meaning; a web session preference is only accepted in operational mode.
// PREVENTS: accepting a local preference where SSH interprets a config path.
func TestTerminalSessionFormatConfigModeUsesSchema(t *testing.T) {
	t.Setenv("ZE_CLI_FORMAT", "text")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	_, err := mgr.ConfigCommand("native", contract.ConfigCommandRequest{Line: "set cli format json"})
	require.Error(t, err)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, nil, nil, nil)
	answer := runTerminalCommandForm(t, handler, url.Values{"command": {"set cli format json"}, "mode": {"config"}})
	assert.Equal(t, "error: "+err.Error(), answer.Output)
	assert.NotContains(t, answer.Output, "cli format set to json")
	assert.Contains(t, answer.Output, "cli")
	assert.Zero(t, mgr.ChangeCount("testuser"))
	answer = runTerminalCommandForm(t, handler, url.Values{"command": {"set cli format"}, "mode": {"operational"}})
	assert.Equal(t, "cli format: text", answer.Output)
}

// VALIDATES: AC-33 -- the preference's authorization exception uses exactly
// the same input recognition as execution, rather than normalizing a config
// mutation into a local command before checking its privilege.
// PREVENTS: quoted command words or extra spacing bypassing configuration RBAC.
func TestTerminalSessionFormatAuthorizationUsesRawCommand(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, nil, fakeAuthorizer{}, nil)
	for _, line := range []string{`set "cli" format json`, "set   cli format json"} {
		recorder := httptest.NewRecorder()
		request := postConfigRequest(t, "/cli/terminal", url.Values{"command": {line}, "mode": {"operational"}}, "alice")
		handler.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusForbidden, recorder.Code, line)
	}
	assert.Zero(t, mgr.ChangeCount("alice"))
}

// VALIDATES: AC-33 -- browser completion uses the CLI producer and the schema
// enumeration.
// PREVENTS: a second web-only command grammar or hand-maintained format list.
func TestTerminalSessionFormatCompletion(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	handler := HandleCLICompleteWithCommandCompleter(newCompleter(t), &fakeCommandCompleter{}, mgr, schema)
	for _, input := range []string{"set cli", "set cli format ", "set cli format j"} {
		recorder := httptest.NewRecorder()
		request := authedRequest(t, http.MethodGet, "/cli/complete?mode=operational&input="+url.QueryEscape(input), nil)
		handler.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code)
		var got []struct {
			Text        string `json:"text"`
			Description string `json:"description"`
			Explanation string `json:"explanation"`
			Type        string `json:"type"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
		want := cli.SessionFormatCompletions(schema, input)
		require.NotEmpty(t, want)
		var completions []contract.Completion
		for _, item := range got {
			completions = append(completions, contract.Completion{Text: item.Text, ShortHelp: item.Description, Description: item.Explanation, Type: item.Type})
		}
		assert.Equal(t, want, completions)
	}
}
