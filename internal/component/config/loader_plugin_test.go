// Design: docs/architecture/config/syntax.md -- the boot and SIGHUP load path
// Related: loader.go -- ExtractPluginsFromTree, the producer these tests drive

package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// pluginConfigFromFile runs the whole operator path -- config text, LoadConfig,
// ExtractPluginsFromTree -- and returns the block named "watched".
func pluginConfigFromFile(t *testing.T, body string) plugin.PluginConfig {
	t.Helper()
	input := "plugin {\n\texternal watched {\n\t\trun \"/bin/true\";\n" + body + "\t}\n}\n"

	result, err := LoadConfig(input, filepath.Join(t.TempDir(), "plugin.conf"), nil)
	require.NoError(t, err)

	plugins, err := ExtractPluginsFromTree(result.Tree)
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	require.Equal(t, "watched", plugins[0].Name)
	return plugins[0]
}

// TestExtractPluginsReadsTheRespawnLeaf: what an operator writes reaches the
// engine, and the three states stay three.
//
// VALIDATES: AC-9 -- the respawn leaf is read into PluginConfig.Respawn, and an
// absent leaf is not the same answer as a written false.
//
// PREVENTS: the defect this spec was written about. ExtractPluginsFromTree read
// run, use, encoder and timeout and never respawn, so PluginConfig.Respawn was
// false for every configured plugin and the leaf reached no code at all. It also
// prevents the narrower version of that defect, in which the leaf is read as a
// plain boolean: silence would then be indistinguishable from a written false,
// and every plugin ze ships would look as though its operator had declined the
// restart its author asked for.
func TestExtractPluginsReadsTheRespawnLeaf(t *testing.T) {
	cases := []struct {
		name string
		body string
		want plugin.RespawnRequest
	}{
		{"no leaf leaves the decision to the plugin", "", plugin.RespawnUnstated},
		{"true asks for a restart", "\t\trespawn true;\n", plugin.RespawnAsked},
		{"false declines one", "\t\trespawn false;\n", plugin.RespawnDeclined},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, pluginConfigFromFile(t, tc.body).Respawn)
		})
	}
}
