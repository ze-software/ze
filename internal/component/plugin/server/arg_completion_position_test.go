// Design: docs/architecture/cli/command-completion.md -- position-aware builtin arguments
package server

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// positionCompletionServer uses the actual system completion RPC and shared
// tree overlay. No target operation can run during either completion route.
func positionCompletionServer(t *testing.T, producer ArgCompleter) (*Server, *CommandContext, *command.Node) {
	t.Helper()
	dispatcher := NewDispatcher()
	require.NoError(t, dispatcher.RegisterWithOptions("show completion position", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		t.Fatal("completion executed the target handler")
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{}), nil
	}, "Complete the next value", RegisterOptions{ReadOnly: true, Complete: producer}))
	require.NoError(t, dispatcher.RegisterWithOptions("system command complete", handleSystemCommandComplete, "Complete command arguments", RegisterOptions{ReadOnly: true}))
	server := &Server{dispatcher: dispatcher}
	ctx := &CommandContext{Server: server, RequestContext: t.Context(), Username: "operator", RemoteAddr: "203.0.113.9:4567", Sender: plugin.OperatorSender()}
	base := &command.Node{Children: map[string]*command.Node{"show": {Children: map[string]*command.Node{"completion": {Children: map[string]*command.Node{"position": {WireMethod: "position-test"}}}}}}}
	return server, ctx, base
}

// TestBuiltinArgumentCompletionReceivesCompletedValues proves the real RPC
// preserves completed values and quoting while filtering the separate partial.
func TestBuiltinArgumentCompletionReceivesCompletedValues(t *testing.T) {
	calls := 0
	server, ctx, _ := positionCompletionServer(t, func(_ *Server, completed []string) []Completion {
		calls++
		require.Equal(t, []string{"42", "scope", "west side", "listener"}, completed)
		return []Completion{{Value: "edge-west"}, {Value: "other"}}
	})
	response, err := server.Dispatcher().Dispatch(ctx, `system command complete args "show completion position" 42 scope "west side" listener "edge-"`)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	data, ok := response.Data.(plugin.Map)
	require.True(t, ok)
	require.Equal(t, []Completion{{Value: "edge-west"}}, data[fieldCompletions])
}

// TestBuiltinArgumentCompletionTreeRetainsPosition proves the shared overlay
// and query writer preserve a later argument position without editing the base.
func TestBuiltinArgumentCompletionTreeRetainsPosition(t *testing.T) {
	calls := 0
	server, ctx, base := positionCompletionServer(t, func(_ *Server, completed []string) []Completion {
		calls++
		require.Equal(t, []string{"42", "scope", "west side", "listener"}, completed)
		return []Completion{{Value: "edge-west", ShortHelp: "Accepted runtime endpoint"}}
	})
	overlay := server.WithArgCompletion(base, func(line string) (string, error) {
		response, err := server.Dispatcher().Dispatch(ctx, line)
		return plugin.ResponseJSON(response, err)
	})
	values, err := command.NewTreeCompleter(overlay).CompleteChecked(`show completion position 42 scope "west side" listener edge-`)
	require.NoError(t, err)
	require.Equal(t, []command.Suggestion{{Text: "edge-west", Description: "Accepted runtime endpoint", Type: command.SuggestionValue}}, values)
	require.Equal(t, 1, calls)
	require.Nil(t, command.FindNode(base, []string{"show", "completion", "position"}).ArgumentHints)
}

type positionCompletionPolicy struct {
	t       *testing.T
	allow   bool
	checked bool
}

func (policy *positionCompletionPolicy) Authorize(username, remote, input string, readOnly bool) bool {
	if !strings.HasPrefix(input, "show completion position") {
		return true
	}
	policy.checked = true
	require.Equal(policy.t, "operator", username)
	require.Equal(policy.t, "203.0.113.9:4567", remote)
	require.True(policy.t, readOnly)
	return policy.allow
}

// TestBuiltinPositionCompletionKeepsCallerAuthorization checks the target's
// policy through the actual RPC, with a positive authorized control.
func TestBuiltinPositionCompletionKeepsCallerAuthorization(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		t.Run(map[bool]string{false: "denied", true: "allowed"}[allowed], func(t *testing.T) {
			calls := 0
			server, ctx, _ := positionCompletionServer(t, func(_ *Server, completed []string) []Completion {
				calls++
				require.Equal(t, []string{"42", "listener"}, completed)
				return []Completion{{Value: "private-value"}}
			})
			policy := &positionCompletionPolicy{t: t, allow: allowed}
			ctx.Authorizer = policy
			response, err := server.Dispatcher().Dispatch(ctx, `system command complete args "show completion position" 42 listener ""`)
			require.True(t, policy.checked)
			if !allowed {
				require.ErrorIs(t, err, ErrUnauthorized)
				require.Zero(t, calls)
				require.Equal(t, plugin.StatusError, response.Status)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			data, ok := response.Data.(plugin.Map)
			require.True(t, ok)
			require.Equal(t, []Completion{{Value: "private-value"}}, data[fieldCompletions])
		})
	}
}

// TestBuiltinPositionCompletionKeepsCallerCancellation proves a canceled
// caller cannot read a position-specific builtin's runtime values.
func TestBuiltinPositionCompletionKeepsCallerCancellation(t *testing.T) {
	calls := 0
	server, ctx, _ := positionCompletionServer(t, func(*Server, []string) []Completion { calls++; return []Completion{{Value: "private-value"}} })
	request, cancel := context.WithCancel(t.Context())
	cancel()
	ctx.RequestContext = request
	_, err := server.Dispatcher().Dispatch(ctx, `system command complete args "show completion position" 42 listener ""`)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, calls)
}

// TestPluginRestartCompletionStopsAfterName proves a one-value owner does not
// offer a name in a later position introduced by the new generic seam.
func TestPluginRestartCompletionStopsAfterName(t *testing.T) {
	server := &Server{}
	server.procManager.Store(process.NewProcessManager([]plugin.PluginConfig{{Name: "configured"}}))
	require.Equal(t, []Completion{{Value: "configured", ShortHelp: completionNotRunning}}, completePluginRestart(server, nil))
	require.Equal(t, []Completion{{Value: "configured", ShortHelp: completionNotRunning}}, completePluginRestart(server, []string{pluginRestartLeaf}))
	require.Empty(t, completePluginRestart(server, []string{"configured"}))
}
