package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// fakeCommands returns a CommandSource with fixed test data.
func fakeCommands() CommandSource {
	return func() []CommandMeta {
		return []CommandMeta{
			{Name: "bgp summary", Description: "Show BGP summary", ReadOnly: true},
			{Name: "show bgp rib status", Description: "Show RIB status", ReadOnly: true},
			{Name: "show bgp rib", Description: "Show RIB routes", ReadOnly: true, Params: []ParamMeta{
				{Name: "family", Type: "string", Description: "Address family", Required: false},
			}},
			{Name: "peer list", Description: "List peers", ReadOnly: true},
			{Name: "daemon reload", Description: "Reload config", ReadOnly: false},
		}
	}
}

// fakeExecutor returns an Executor (the unified dispatcher) with typed
// responses. Structured commands carry JSON via RawJSON; plain-text commands
// carry raw text via RawJSON (which marshals to a JSON string).
func fakeExecutor() Executor {
	return func(_ context.Context, _ CallerIdentity, command string) (*plugin.Response, error) {
		switch command {
		case "bgp summary":
			return plugin.NewResponse(StatusDone, plugin.RawJSON(`{"peer-count":3,"established":2}`)), nil
		case "daemon reload":
			return plugin.NewResponse(StatusDone, plugin.RawJSON("reload initiated")), nil
		default:
			return plugin.NewResponse(StatusDone, plugin.RawJSON("ok")), nil
		}
	}
}

// denyAllAuth returns an AuthChecker that denies everything.
func denyAllAuth() AuthChecker {
	return func(_, _ string) bool { return false }
}

// allowAllAuth returns an AuthChecker that allows everything.
func allowAllAuth() AuthChecker {
	return func(_, _ string) bool { return true }
}

// VALIDATES: AC-1 -- Engine.ListCommands() returns all commands from dispatcher + plugin registry.
// PREVENTS: missing commands in API output.
func TestEngineListCommands(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	cmds := eng.ListCommands(&ListCommandsRequest{})
	assert.Len(t, cmds, 5)

	// Verify all expected commands are present.
	names := make(map[string]bool, len(cmds))
	for _, cmd := range cmds {
		names[cmd.Name] = true
	}
	assert.True(t, names["bgp summary"])
	assert.True(t, names["show bgp rib status"])
	assert.True(t, names["show bgp rib"])
	assert.True(t, names["peer list"])
	assert.True(t, names["daemon reload"])
}

// VALIDATES: AC-1 -- ListCommands with prefix filter.
// PREVENTS: filter returning wrong commands.
func TestEngineListCommandsWithPrefix(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	cmds := eng.ListCommands(&ListCommandsRequest{Prefix: "show bgp rib"})
	assert.Len(t, cmds, 2)
	for _, cmd := range cmds {
		assert.Contains(t, cmd.Name, "show bgp rib")
	}
}

// VALIDATES: DescribeCommand returns full metadata for one command.
// PREVENTS: missing params in describe output.
func TestEngineDescribeCommand(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	cmd, err := eng.DescribeCommand(&DescribeCommandRequest{Path: "show bgp rib"})
	require.NoError(t, err)
	assert.Equal(t, "show bgp rib", cmd.Name)
	assert.True(t, cmd.ReadOnly)
	require.Len(t, cmd.Params, 1)
	assert.Equal(t, "family", cmd.Params[0].Name)
}

// VALIDATES: DescribeCommand returns ErrNotFound for unknown commands.
// PREVENTS: nil results for unknown commands.
func TestEngineDescribeCommandNotFound(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	_, err := eng.DescribeCommand(&DescribeCommandRequest{Path: "nonexistent command"})
	assert.ErrorIs(t, err, ErrNotFound)
}

// VALIDATES: AC-2 -- Engine.Execute returns same output as dispatcher.
// PREVENTS: data loss in command execution path.
func TestEngineExecuteDispatch(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	result, err := eng.Execute(t.Context(), &ExecuteRequest{Caller: CallerIdentity{Username: "admin"}, Command: "bgp summary"})
	require.NoError(t, err)
	assert.Equal(t, StatusDone, result.Status)

	// test-relax: the engine no longer re-parses executor output into a
	// map[string]any (finding 3 removed that round trip). The map-field
	// assertions are replaced by an equivalent whole-payload JSON check on the
	// typed Data that now flows through unchanged.
	data, mErr := json.Marshal(result.Data)
	require.NoError(t, mErr)
	assert.JSONEq(t, `{"peer-count":3,"established":2}`, string(data))
}

// VALIDATES: AC-2 -- Execute with non-JSON output returns string.
// PREVENTS: string results lost or mangled.
func TestEngineExecuteStringOutput(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	result, err := eng.Execute(t.Context(), &ExecuteRequest{Caller: CallerIdentity{Username: "admin"}, Command: "daemon reload"})
	require.NoError(t, err)
	assert.Equal(t, StatusDone, result.Status)
	// test-relax: plain-text output now rides typed RawJSON Data (which marshals
	// to a JSON string) instead of a bare Go string on the envelope; assert on
	// the marshaled form.
	data, mErr := json.Marshal(result.Data)
	require.NoError(t, mErr)
	assert.Equal(t, `"reload initiated"`, string(data))
}

// VALIDATES: AC-3 -- Execute with unauthorized user returns auth error.
// PREVENTS: unauthorized command execution.
func TestEngineExecuteUnauthorized(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), denyAllAuth(), nil)

	result, err := eng.Execute(t.Context(), &ExecuteRequest{Caller: CallerIdentity{Username: "readonly"}, Command: "daemon reload"})
	assert.ErrorIs(t, err, ErrUnauthorized)
	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error, plugin.UnauthorizedMessage)
}

// VALIDATES: AC-8 -- a no-auth/read-only API caller can run reads but not writes.
// PREVENTS: REST/gRPC no-auth mode granting admin write access to the default api identity.
func TestEngineReadOnlyCallerDeniedWrite(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	readResult, readErr := eng.Execute(t.Context(), &ExecuteRequest{Caller: CallerIdentity{Username: "api", ReadOnly: true}, Command: "bgp summary"})
	require.NoError(t, readErr)
	assert.Equal(t, StatusDone, readResult.Status)

	writeResult, writeErr := eng.Execute(t.Context(), &ExecuteRequest{Caller: CallerIdentity{Username: "api", ReadOnly: true}, Command: "daemon reload"})
	assert.ErrorIs(t, writeErr, ErrUnauthorized)
	assert.Equal(t, StatusError, writeResult.Status)
	assert.Contains(t, writeResult.Error, plugin.UnauthorizedMessage)
}

// VALIDATES: Execute propagates executor errors.
// PREVENTS: swallowed errors in dispatch path.
func TestEngineExecuteError(t *testing.T) {
	errExec := func(_ context.Context, _ CallerIdentity, _ string) (*plugin.Response, error) {
		return nil, errors.New("connection refused")
	}
	eng := NewAPIEngine(errExec, fakeCommands(), allowAllAuth(), nil)

	result, err := eng.Execute(t.Context(), &ExecuteRequest{Caller: CallerIdentity{Username: "admin"}, Command: "bgp summary"})
	require.Error(t, err)
	assert.Equal(t, StatusError, result.Status)
	assert.Equal(t, "connection refused", result.Error)
}

// VALIDATES: an operational error Response (Status=error, typed Data, no Error
// message, nil Go error -- e.g. as112 health {healthy:false}) reaches the API
// engine with its diagnostic Data intact, rather than being collapsed to a
// generic "unknown error" as the old marshal-to-string adapter did.
// PREVENTS: finding-3 regressing back to a lossy string flatten that drops the
// diagnostic payload on the REST/gRPC surface. Text surfaces still flatten such
// responses via plugin.ResponseJSON; only the API surface carries Data through.
func TestEngineExecuteErrorStatusPreservesData(t *testing.T) {
	exec := func(_ context.Context, _ CallerIdentity, _ string) (*plugin.Response, error) {
		return &plugin.Response{Status: StatusError, Data: plugin.Map{"healthy": false}}, nil
	}
	eng := NewAPIEngine(exec, fakeCommands(), allowAllAuth(), nil)

	result, err := eng.Execute(t.Context(), &ExecuteRequest{Caller: CallerIdentity{Username: "admin"}, Command: "bgp summary"})
	require.NoError(t, err)
	assert.Equal(t, StatusError, result.Status)
	assert.Equal(t, "", result.Error)
	data, mErr := json.Marshal(result.Data)
	require.NoError(t, mErr)
	assert.JSONEq(t, `{"healthy":false}`, string(data))
}

// VALIDATES: nil auth checker means allow all.
// PREVENTS: nil pointer on auth check.
func TestEngineExecuteNilAuth(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), nil, nil)

	result, err := eng.Execute(t.Context(), &ExecuteRequest{Caller: CallerIdentity{Username: ""}, Command: "bgp summary"})
	require.NoError(t, err)
	assert.Equal(t, StatusDone, result.Status)
}

// fakeStream returns a StreamSource that sends N events then closes.
func fakeStream(events []string) StreamSource {
	return func(_ context.Context, _ CallerIdentity, _ string) (<-chan string, func(), error) {
		ch := make(chan string, len(events))
		for _, e := range events {
			ch <- e
		}
		close(ch)
		return ch, func() {}, nil
	}
}

// VALIDATES: AC-4 -- Engine.Stream returns channel that delivers BGP events.
// PREVENTS: streaming events lost or not delivered.
func TestEngineStream(t *testing.T) {
	events := []string{`{"type":"update","peer":"10.0.0.1"}`, `{"type":"update","peer":"10.0.0.2"}`}
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), fakeStream(events))

	ch, cancel, err := eng.Stream(t.Context(), &StreamRequest{Caller: CallerIdentity{Username: "admin"}, Command: "bgp monitor"})
	require.NoError(t, err)
	defer cancel()

	var received []string
	for e := range ch {
		received = append(received, e)
	}
	assert.Equal(t, events, received)
}

// VALIDATES: Stream with unauthorized user returns error.
// PREVENTS: unauthorized streaming access.
func TestEngineStreamUnauthorized(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), denyAllAuth(), fakeStream(nil))

	_, _, err := eng.Stream(t.Context(), &StreamRequest{Caller: CallerIdentity{Username: "nobody"}, Command: "bgp monitor"})
	assert.ErrorIs(t, err, ErrUnauthorized)
}

// VALIDATES: Stream with nil StreamSource returns error.
// PREVENTS: nil pointer when streaming not configured.
func TestEngineStreamNotSupported(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	_, _, err := eng.Stream(t.Context(), &StreamRequest{Caller: CallerIdentity{Username: "admin"}, Command: "bgp monitor"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "streaming not supported")
}
