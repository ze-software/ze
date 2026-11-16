//go:build ze_web

// Design: docs/architecture/cli/command-completion.md -- live browser arguments.

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestWebCompletionUsesLiveArgumentProducer asks a real registered builtin
// completer through the browser overlay, preserving the shared tree and help.
// MUTATION: omit the argument-tree call in completer; live names disappear.
func TestWebCompletionUsesLiveArgumentProducer(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	live := "alpha"
	require.NoError(t, server.Dispatcher().RegisterWithOptions("request browser-completion probe", func(*pluginserver.CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		t.Fatal("completion executed the command")
		return nil, nil
	}, "Inspect live completion values", pluginserver.RegisterOptions{
		Complete: func(*pluginserver.Server, []string) []pluginserver.Completion {
			return []pluginserver.Completion{{Value: live, ShortHelp: "Configured plugin"}}
		},
	}))
	const explanation = "Inspect the selected live completion value."
	tree := &command.Node{Children: map[string]*command.Node{
		"request": {Children: map[string]*command.Node{
			"browser-completion": {Children: map[string]*command.Node{
				"probe": {Description: explanation},
			}},
		}},
	}}
	completer := webCompletionSource(tree, func() []command.CommandEntry { return nil }, server.WithArgCompletion, serverDispatcher(server, "web"))(t.Context(), plugin.CallerIdentity{Username: "operator"})
	assert.True(t, hasCompletion(completer.Complete("request browser-completion probe "), "alpha"))
	live = "beta"
	assert.False(t, hasCompletion(completer.Complete("request browser-completion probe "), "alpha"))
	assert.True(t, hasCompletion(completer.Complete("request browser-completion probe b"), "beta"))
	text, found := completer.Explain("request browser-completion probe")
	assert.True(t, found)
	assert.Equal(t, explanation, text)
	assert.Nil(t, command.FindNode(tree, []string{"request", "browser-completion", "probe"}).ValueHints)
}

// TestWebExplanationTracksPluginRegistration uses the same registry overlay as
// completion, so a departed plugin cannot retain stale command documentation.
func TestWebExplanationTracksPluginRegistration(t *testing.T) {
	live := []command.CommandEntry{{Name: "show example", Description: "Read the example plugin's state."}}
	completer := newPluginAwareCommandCompleter(nil, func() []command.CommandEntry { return live }, nil, command.SurfaceWebCompletion)
	text, found := completer.Explain("show example")
	assert.True(t, found)
	assert.Equal(t, live[0].Description, text)
	live = nil
	text, found = completer.Explain("show example")
	assert.False(t, found)
	assert.Empty(t, text)
}
