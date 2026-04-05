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
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	groupTree := config.NewTree()
	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)

	// Add process binding referencing undefined plugin.
	processTree := config.NewTree()
	processTree.Set("send", "update")
	peerTree.AddListEntry("process", "nonexistent-plugin", processTree)

	groupTree.AddListEntry("peer", "peer1", peerTree)
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
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	groupTree := config.NewTree()
	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)

	processTree := config.NewTree()
	processTree.Set("send", "update")
	peerTree.AddListEntry("process", "my-plugin", processTree)

	groupTree.AddListEntry("peer", "peer1", peerTree)
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
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	groupTree := config.NewTree()
	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)

	processTree := config.NewTree()
	processTree.Set("run", "/usr/local/bin/my-process")
	processTree.Set("send", "update")
	peerTree.AddListEntry("process", "inline-proc", processTree)

	groupTree.AddListEntry("peer", "peer1", peerTree)
	bgp.AddListEntry("group", "test-group", groupTree)
	tree.SetContainer("bgp", bgp)

	// No plugins declared -- inline should still pass.
	err := ValidatePluginReferences(tree, nil)
	assert.NoError(t, err)
}

// TestExtractHubServers verifies named server blocks are parsed with host/port/secret.
//
// VALIDATES: Named server blocks extracted from config tree (AC-15).
// PREVENTS: TLS listener config being silently ignored.
func TestExtractHubServers(t *testing.T) {
	tree := config.NewTree()
	pluginContainer := config.NewTree()
	tree.SetContainer("plugin", pluginContainer)

	hubContainer := config.NewTree()
	pluginContainer.SetContainer("hub", hubContainer)

	serverTree := config.NewTree()
	serverTree.Set("ip", "127.0.0.1")
	serverTree.Set("port", "1790")
	serverTree.Set("secret", "test-token-42-abcdefghijklmnopqrst")
	hubContainer.AddListEntry("server", "local", serverTree)

	hub, err := config.ExtractHubConfig(tree)
	require.NoError(t, err)
	require.Len(t, hub.Servers, 1)
	assert.Equal(t, "local", hub.Servers[0].Name)
	assert.Equal(t, "127.0.0.1", hub.Servers[0].Host)
	assert.Equal(t, uint16(1790), hub.Servers[0].Port)
	assert.Equal(t, "test-token-42-abcdefghijklmnopqrst", hub.Servers[0].Secret)
	assert.Equal(t, "127.0.0.1:1790", hub.Servers[0].Address())
}

// TestExtractHubConfig_NoHub verifies that missing hub config returns empty.
//
// VALIDATES: No hub block returns zero-value HubConfig.
// PREVENTS: Panic or error when hub config is absent.
func TestExtractHubConfig_NoHub(t *testing.T) {
	tree := config.NewTree()
	hub, err := config.ExtractHubConfig(tree)
	require.NoError(t, err)
	assert.Empty(t, hub.Servers, "no hub block should return empty servers")
	assert.Empty(t, hub.Clients, "no hub block should return empty clients")
}

// TestExtractHubConfig_NoServers verifies hub block with no server entries returns empty.
//
// VALIDATES: Hub block without servers returns zero-value.
// PREVENTS: TLS listener starting without config.
func TestExtractHubConfig_NoServers(t *testing.T) {
	tree := config.NewTree()
	pluginContainer := config.NewTree()
	tree.SetContainer("plugin", pluginContainer)
	hubContainer := config.NewTree()
	pluginContainer.SetContainer("hub", hubContainer)

	hub, err := config.ExtractHubConfig(tree)
	require.NoError(t, err)
	assert.Empty(t, hub.Servers)
}

// TestExtractHubConfig_ShortSecret verifies that short secrets are rejected.
//
// VALIDATES: Minimum token length enforced.
// PREVENTS: Weak auth tokens accepted in config.
func TestExtractHubConfig_ShortSecret(t *testing.T) {
	tree := config.NewTree()
	pluginContainer := config.NewTree()
	tree.SetContainer("plugin", pluginContainer)

	hubContainer := config.NewTree()
	pluginContainer.SetContainer("hub", hubContainer)

	serverTree := config.NewTree()
	serverTree.Set("ip", "127.0.0.1")
	serverTree.Set("port", "1790")
	serverTree.Set("secret", "too-short")
	hubContainer.AddListEntry("server", "local", serverTree)

	_, err := config.ExtractHubConfig(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

// TestExtractMultipleServers verifies multiple named server blocks are parsed.
//
// VALIDATES: Multiple server blocks with different names parsed.
// PREVENTS: Only first server block being extracted.
func TestExtractMultipleServers(t *testing.T) {
	tree := config.NewTree()
	pluginContainer := config.NewTree()
	tree.SetContainer("plugin", pluginContainer)

	hubContainer := config.NewTree()
	pluginContainer.SetContainer("hub", hubContainer)

	local := config.NewTree()
	local.Set("ip", "127.0.0.1")
	local.Set("port", "1790")
	local.Set("secret", "local-secret-that-is-at-least-32c")
	hubContainer.AddListEntry("server", "local", local)

	central := config.NewTree()
	central.Set("ip", "0.0.0.0")
	central.Set("port", "1791")
	central.Set("secret", "central-secret-that-is-at-least32")
	hubContainer.AddListEntry("server", "central", central)

	hub, err := config.ExtractHubConfig(tree)
	require.NoError(t, err)
	require.Len(t, hub.Servers, 2)

	// Verify both servers present (order preserved by GetListOrdered)
	assert.Equal(t, "local", hub.Servers[0].Name)
	assert.Equal(t, uint16(1790), hub.Servers[0].Port)
	assert.Equal(t, "central", hub.Servers[1].Name)
	assert.Equal(t, uint16(1791), hub.Servers[1].Port)
}

// TestExtractHubClients verifies hub-level client blocks are parsed.
//
// VALIDATES: Hub-level client blocks extracted with host/port/secret (AC-14).
// PREVENTS: Managed client unable to find hub connection info.
func TestExtractHubClients(t *testing.T) {
	tree := config.NewTree()
	pluginContainer := config.NewTree()
	tree.SetContainer("plugin", pluginContainer)

	hubContainer := config.NewTree()
	pluginContainer.SetContainer("hub", hubContainer)

	clientTree := config.NewTree()
	clientTree.Set("host", "10.0.0.1")
	clientTree.Set("port", "1791")
	clientTree.Set("secret", "client-token-that-is-at-least-32c")
	hubContainer.AddListEntry("client", "edge-01", clientTree)

	hub, err := config.ExtractHubConfig(tree)
	require.NoError(t, err)
	require.Len(t, hub.Clients, 1)
	assert.Equal(t, "edge-01", hub.Clients[0].Name)
	assert.Equal(t, "10.0.0.1", hub.Clients[0].Host)
	assert.Equal(t, uint16(1791), hub.Clients[0].Port)
	assert.Equal(t, "client-token-that-is-at-least-32c", hub.Clients[0].Secret)
	assert.Equal(t, "10.0.0.1:1791", hub.Clients[0].Address())
}

// TestExtractHubClientMissing verifies no hub-level client block returns empty.
//
// VALIDATES: No client block returns empty clients list.
// PREVENTS: False positive for managed mode.
func TestExtractHubClientMissing(t *testing.T) {
	tree := config.NewTree()
	pluginContainer := config.NewTree()
	tree.SetContainer("plugin", pluginContainer)

	hubContainer := config.NewTree()
	pluginContainer.SetContainer("hub", hubContainer)

	serverTree := config.NewTree()
	serverTree.Set("ip", "127.0.0.1")
	serverTree.Set("port", "1790")
	serverTree.Set("secret", "local-secret-that-is-at-least-32c")
	hubContainer.AddListEntry("server", "local", serverTree)

	hub, err := config.ExtractHubConfig(tree)
	require.NoError(t, err)
	assert.Empty(t, hub.Clients)
	require.Len(t, hub.Servers, 1)
}

// TestExtractHubServerClients verifies nested client entries under server block.
//
// VALIDATES: Per-client secrets extracted from server block.
// PREVENTS: Hub unable to authenticate managed clients.
func TestExtractHubServerClients(t *testing.T) {
	tree := config.NewTree()
	pluginContainer := config.NewTree()
	tree.SetContainer("plugin", pluginContainer)

	hubContainer := config.NewTree()
	pluginContainer.SetContainer("hub", hubContainer)

	serverTree := config.NewTree()
	serverTree.Set("ip", "0.0.0.0")
	serverTree.Set("port", "1791")
	serverTree.Set("secret", "central-secret-that-is-at-least32")
	hubContainer.AddListEntry("server", "central", serverTree)

	client1 := config.NewTree()
	client1.Set("secret", "edge01-secret-that-is-at-least-32")
	serverTree.AddListEntry("client", "edge-01", client1)

	client2 := config.NewTree()
	client2.Set("secret", "edge02-secret-that-is-at-least-32")
	serverTree.AddListEntry("client", "edge-02", client2)

	hub, err := config.ExtractHubConfig(tree)
	require.NoError(t, err)
	require.Len(t, hub.Servers, 1)
	require.Len(t, hub.Servers[0].Clients, 2)
	assert.Equal(t, "edge01-secret-that-is-at-least-32", hub.Servers[0].Clients["edge-01"])
	assert.Equal(t, "edge02-secret-that-is-at-least-32", hub.Servers[0].Clients["edge-02"])
}

// TestExtractHubServerClientSecretTooShort verifies client secret validation.
//
// VALIDATES: Client secret < 32 chars returns error.
// PREVENTS: Weak per-client tokens accepted.
func TestExtractHubServerClientSecretTooShort(t *testing.T) {
	tree := config.NewTree()
	pluginContainer := config.NewTree()
	tree.SetContainer("plugin", pluginContainer)

	hubContainer := config.NewTree()
	pluginContainer.SetContainer("hub", hubContainer)

	serverTree := config.NewTree()
	serverTree.Set("ip", "0.0.0.0")
	serverTree.Set("port", "1791")
	serverTree.Set("secret", "central-secret-that-is-at-least32")
	hubContainer.AddListEntry("server", "central", serverTree)

	client := config.NewTree()
	client.Set("secret", "short")
	serverTree.AddListEntry("client", "edge-01", client)

	_, err := config.ExtractHubConfig(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
	assert.Contains(t, err.Error(), "edge-01")
}
