package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
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

// TestValidatePluginReferences_GroupPeerUndefinedPlugin verifies that undefined
// plugin references in grouped peer process bindings are detected.
//
// VALIDATES: ValidatePluginReferences checks peers inside groups.
// PREVENTS: Grouped peer process bindings bypassing plugin validation.
func TestValidatePluginReferences_GroupPeerUndefinedPlugin(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	groupTree := config.NewTree()
	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")

	// Add process binding referencing undefined plugin.
	processTree := config.NewTree()
	processTree.Set("send", "update")
	peerTree.AddListEntry("process", "nonexistent-plugin", processTree)

	groupTree.AddListEntry("peer", "10.0.0.1", peerTree)
	bgp.AddListEntry("group", "test-group", groupTree)
	tree.SetContainer("bgp", bgp)

	// No plugins declared.
	err := ValidatePluginReferences(tree, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-plugin")
	assert.Contains(t, err.Error(), "undefined plugin")
	assert.Contains(t, err.Error(), "test-group")
}

// TestValidatePluginReferences_GroupPeerValidPlugin verifies that valid
// plugin references in grouped peers pass validation.
//
// VALIDATES: ValidatePluginReferences accepts declared plugins in group peers.
// PREVENTS: False positive validation errors for grouped peers with declared plugins.
func TestValidatePluginReferences_GroupPeerValidPlugin(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	groupTree := config.NewTree()
	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")

	processTree := config.NewTree()
	processTree.Set("send", "update")
	peerTree.AddListEntry("process", "my-plugin", processTree)

	groupTree.AddListEntry("peer", "10.0.0.1", peerTree)
	bgp.AddListEntry("group", "test-group", groupTree)
	tree.SetContainer("bgp", bgp)

	// Declare the plugin.
	plugins := []reactor.PluginConfig{{Name: "my-plugin"}}
	err := ValidatePluginReferences(tree, plugins)
	assert.NoError(t, err)
}

// TestValidatePluginReferences_GroupPeerInlinePlugin verifies that inline plugins
// (with run defined) in grouped peers are accepted without declaration.
//
// VALIDATES: Inline plugins in grouped peers skip the declaration check.
// PREVENTS: Inline plugins in groups being rejected as undefined.
func TestValidatePluginReferences_GroupPeerInlinePlugin(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	groupTree := config.NewTree()
	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")

	processTree := config.NewTree()
	processTree.Set("run", "/usr/local/bin/my-process")
	processTree.Set("send", "update")
	peerTree.AddListEntry("process", "inline-proc", processTree)

	groupTree.AddListEntry("peer", "10.0.0.1", peerTree)
	bgp.AddListEntry("group", "test-group", groupTree)
	tree.SetContainer("bgp", bgp)

	// No plugins declared -- inline should still pass.
	err := ValidatePluginReferences(tree, nil)
	assert.NoError(t, err)
}
