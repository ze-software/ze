package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"
)

// TestExtractPluginsFromTree_InternalPlugin verifies that explicit plugins
// with run "ze.X" are marked Internal=true.
//
// VALIDATES: Config-file internal plugins detected via ResolvePlugin.
// PREVENTS: Internal plugins being treated as external (fork instead of goroutine).
func TestExtractPluginsFromTree_InternalPlugin(t *testing.T) {
	tree := config.NewTree()
	pluginContainer := config.NewTree()
	tree.SetContainer("plugin", pluginContainer)

	ext := config.NewTree()
	ext.Set("run", "ze.bgp-rs")
	pluginContainer.AddListEntry("external", "rr", ext)

	plugins, err := ExtractPluginsFromTree(tree)
	require.NoError(t, err)
	require.Len(t, plugins, 1)

	assert.Equal(t, "rr", plugins[0].Name)
	assert.Equal(t, "ze.bgp-rs", plugins[0].Run)
	assert.True(t, plugins[0].Internal, "plugin with run ze.bgp-rs should be Internal")
}

// TestExtractPluginsFromTree_ExternalPlugin verifies that external plugins
// are NOT marked internal.
//
// VALIDATES: External plugins have Internal=false.
// PREVENTS: External plugins running as goroutines.
func TestExtractPluginsFromTree_ExternalPlugin(t *testing.T) {
	tree := config.NewTree()
	pluginContainer := config.NewTree()
	tree.SetContainer("plugin", pluginContainer)

	ext := config.NewTree()
	ext.Set("run", "/usr/bin/custom-plugin")
	pluginContainer.AddListEntry("external", "custom", ext)

	plugins, err := ExtractPluginsFromTree(tree)
	require.NoError(t, err)
	require.Len(t, plugins, 1)

	assert.Equal(t, "custom", plugins[0].Name)
	assert.False(t, plugins[0].Internal, "external plugin should not be Internal")
}

// TestExtractPluginsFromTree_UnknownInternalPlugin verifies that an unknown
// ze.X plugin is NOT marked internal (validation via ResolvePlugin).
//
// VALIDATES: Unknown "ze.typo" is not blindly marked Internal.
// PREVENTS: Bug where strings.HasPrefix fast-path skipped validation.
func TestExtractPluginsFromTree_UnknownInternalPlugin(t *testing.T) {
	tree := config.NewTree()
	pluginContainer := config.NewTree()
	tree.SetContainer("plugin", pluginContainer)

	ext := config.NewTree()
	ext.Set("run", "ze.nonexistent")
	pluginContainer.AddListEntry("external", "bad", ext)

	plugins, err := ExtractPluginsFromTree(tree)
	require.NoError(t, err)
	require.Len(t, plugins, 1)

	assert.False(t, plugins[0].Internal, "unknown ze.X should not be Internal")
}
