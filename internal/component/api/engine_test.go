package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCommands returns a CommandSource with fixed test data.
func fakeCommands() CommandSource {
	return func() []CommandMeta {
		return []CommandMeta{
			{Name: "bgp summary", Description: "Show BGP summary", ReadOnly: true},
			{Name: "bgp rib status", Description: "Show RIB status", ReadOnly: true},
			{Name: "bgp rib routes", Description: "Show RIB routes", ReadOnly: true, Params: []ParamMeta{
				{Name: "family", Type: "string", Description: "Address family", Required: false},
			}},
			{Name: "peer list", Description: "List peers", ReadOnly: true},
			{Name: "daemon reload", Description: "Reload config", ReadOnly: false},
		}
	}
}

// fakeExecutor returns an Executor that echoes commands.
func fakeExecutor() Executor {
	return func(_ context.Context, _ CallerIdentity, command string) (string, error) {
		switch command {
		case "bgp summary":
			return `{"peer-count":3,"established":2}`, nil
		case "daemon reload":
			return "reload initiated", nil
		default:
			return "ok", nil
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

	cmds := eng.ListCommands("")
	assert.Len(t, cmds, 5)

	// Verify all expected commands are present.
	names := make(map[string]bool, len(cmds))
	for _, cmd := range cmds {
		names[cmd.Name] = true
	}
	assert.True(t, names["bgp summary"])
	assert.True(t, names["bgp rib status"])
	assert.True(t, names["bgp rib routes"])
	assert.True(t, names["peer list"])
	assert.True(t, names["daemon reload"])
}

// VALIDATES: AC-1 -- ListCommands with prefix filter.
// PREVENTS: filter returning wrong commands.
func TestEngineListCommandsWithPrefix(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	cmds := eng.ListCommands("bgp rib")
	assert.Len(t, cmds, 2)
	for _, cmd := range cmds {
		assert.Contains(t, cmd.Name, "bgp rib")
	}
}

// VALIDATES: DescribeCommand returns full metadata for one command.
// PREVENTS: missing params in describe output.
func TestEngineDescribeCommand(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	cmd, err := eng.DescribeCommand("bgp rib routes")
	require.NoError(t, err)
	assert.Equal(t, "bgp rib routes", cmd.Name)
	assert.True(t, cmd.ReadOnly)
	require.Len(t, cmd.Params, 1)
	assert.Equal(t, "family", cmd.Params[0].Name)
}

// VALIDATES: DescribeCommand returns ErrNotFound for unknown commands.
// PREVENTS: nil results for unknown commands.
func TestEngineDescribeCommandNotFound(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	_, err := eng.DescribeCommand("nonexistent command")
	assert.ErrorIs(t, err, ErrNotFound)
}

// VALIDATES: AC-2 -- Engine.Execute returns same output as dispatcher.
// PREVENTS: data loss in command execution path.
func TestEngineExecuteDispatch(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	result, err := eng.Execute(t.Context(), CallerIdentity{Username: "admin"}, "bgp summary")
	require.NoError(t, err)
	assert.Equal(t, StatusDone, result.Status)

	// JSON output should be parsed into structured data.
	data, ok := result.Data.(map[string]any)
	require.True(t, ok, "expected structured JSON data, got %T", result.Data)
	assert.Equal(t, float64(3), data["peer-count"])
	assert.Equal(t, float64(2), data["established"])
}

// VALIDATES: AC-2 -- Execute with non-JSON output returns string.
// PREVENTS: string results lost or mangled.
func TestEngineExecuteStringOutput(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	result, err := eng.Execute(t.Context(), CallerIdentity{Username: "admin"}, "daemon reload")
	require.NoError(t, err)
	assert.Equal(t, StatusDone, result.Status)
	assert.Equal(t, "reload initiated", result.Data)
}

// VALIDATES: AC-3 -- Execute with unauthorized user returns auth error.
// PREVENTS: unauthorized command execution.
func TestEngineExecuteUnauthorized(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), denyAllAuth(), nil)

	result, err := eng.Execute(t.Context(), CallerIdentity{Username: "readonly"}, "daemon reload")
	assert.ErrorIs(t, err, ErrUnauthorized)
	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error, "authorization denied")
}

// VALIDATES: Execute propagates executor errors.
// PREVENTS: swallowed errors in dispatch path.
func TestEngineExecuteError(t *testing.T) {
	errExec := func(_ context.Context, _ CallerIdentity, _ string) (string, error) {
		return "", errors.New("connection refused")
	}
	eng := NewAPIEngine(errExec, fakeCommands(), allowAllAuth(), nil)

	result, err := eng.Execute(t.Context(), CallerIdentity{Username: "admin"}, "bgp summary")
	require.Error(t, err)
	assert.Equal(t, StatusError, result.Status)
	assert.Equal(t, "connection refused", result.Error)
}

// VALIDATES: nil auth checker means allow all.
// PREVENTS: nil pointer on auth check.
func TestEngineExecuteNilAuth(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), nil, nil)

	result, err := eng.Execute(t.Context(), CallerIdentity{Username: ""}, "bgp summary")
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

	ch, cancel, err := eng.Stream(t.Context(), CallerIdentity{Username: "admin"}, "bgp monitor")
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

	_, _, err := eng.Stream(t.Context(), CallerIdentity{Username: "nobody"}, "bgp monitor")
	assert.ErrorIs(t, err, ErrUnauthorized)
}

// VALIDATES: Stream with nil StreamSource returns error.
// PREVENTS: nil pointer when streaming not configured.
func TestEngineStreamNotSupported(t *testing.T) {
	eng := NewAPIEngine(fakeExecutor(), fakeCommands(), allowAllAuth(), nil)

	_, _, err := eng.Stream(t.Context(), CallerIdentity{Username: "admin"}, "bgp monitor")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "streaming not supported")
}
