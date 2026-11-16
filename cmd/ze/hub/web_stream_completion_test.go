//go:build ze_web && ze_ssh && ze_bgp

// Design: docs/architecture/api/commands.md -- browser terminal streaming discovery.

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestWebTerminalCompletionIncludesDaemonStreams derives the complete set of
// streaming-only commands from their registry, then compares actual terminal
// completion with actual admin-tree construction. A one-shot form stays absent.
func TestWebTerminalCompletionIncludesDaemonStreams(t *testing.T) {
	schema, err := yangloader.DefaultLoader()
	require.NoError(t, err)
	tree := yangloader.BuildCommandTree(schema)
	terminal := newPluginAwareCommandCompleter(tree, func() []command.CommandEntry { return nil }, nil, command.SurfaceWebCompletion)
	admin := adminCommandTrees(tree, nil)()
	prefixes := pluginserver.StreamingOnlyPrefixes()
	require.NotEmpty(t, prefixes)
	for _, prefix := range prefixes {
		assert.True(t, completionOffers(terminal.Complete, prefix), "terminal completion omitted %s", prefix)
		assert.False(t, treeHasPath(admin, prefix), "one-shot admin offers %s", prefix)
	}
}
