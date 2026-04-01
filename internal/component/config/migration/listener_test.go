package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestRemoveBGPListenLeaf verifies detection and removal of bgp > listen.
// VALIDATES: migration remove-bgp-listen transformation.
// PREVENTS: ExaBGP legacy listen leaf surviving migration.
func TestRemoveBGPListenLeaf(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("listen", "0.0.0.0:179")
	bgp.Set("router-id", "1.2.3.4")
	tree.SetContainer("bgp", bgp)

	assert.True(t, hasBGPListenLeaf(tree), "should detect listen leaf")

	result, err := removeBGPListenLeaf(tree)
	require.NoError(t, err)

	bgpResult := result.GetContainer("bgp")
	require.NotNil(t, bgpResult)
	_, ok := bgpResult.Get("listen")
	assert.False(t, ok, "listen should be removed")
	v, ok := bgpResult.Get("router-id")
	assert.True(t, ok, "router-id should be preserved")
	assert.Equal(t, "1.2.3.4", v)
	assert.False(t, hasBGPListenLeaf(result), "detect should return false after removal")
}

// TestRemoveBGPListenLeaf_Absent verifies no-op when listen is absent.
func TestRemoveBGPListenLeaf_Absent(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	tree.SetContainer("bgp", bgp)
	assert.False(t, hasBGPListenLeaf(tree))
}

// TestRemoveBGPListenLeaf_NoBGP verifies no-op when bgp container is absent.
func TestRemoveBGPListenLeaf_NoBGP(t *testing.T) {
	tree := config.NewTree()
	assert.False(t, hasBGPListenLeaf(tree))
}

// TestRemoveTCPPortLeaf verifies detection and removal of environment > tcp > port.
// VALIDATES: migration remove-tcp-port transformation.
// PREVENTS: ExaBGP legacy tcp.port surviving migration.
func TestRemoveTCPPortLeaf(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	tcp := config.NewTree()
	tcp.Set("port", "179")
	tcp.Set("attempts", "3")
	env.SetContainer("tcp", tcp)
	tree.SetContainer("environment", env)

	assert.True(t, hasTCPPortLeaf(tree), "should detect tcp port")

	result, err := removeTCPPortLeaf(tree)
	require.NoError(t, err)

	tcpResult := result.GetContainer("environment").GetContainer("tcp")
	require.NotNil(t, tcpResult)
	_, ok := tcpResult.Get("port")
	assert.False(t, ok, "port should be removed")
	v, ok := tcpResult.Get("attempts")
	assert.True(t, ok, "attempts should be preserved")
	assert.Equal(t, "3", v)
}

// TestRemoveTCPPortLeaf_Absent verifies no-op when tcp.port is absent.
func TestRemoveTCPPortLeaf_Absent(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	tcp := config.NewTree()
	tcp.Set("attempts", "3")
	env.SetContainer("tcp", tcp)
	tree.SetContainer("environment", env)
	assert.False(t, hasTCPPortLeaf(tree))
}

// TestRemoveEnvBGPConnect verifies detection and removal of environment > bgp > connect.
// VALIDATES: migration remove-env-bgp-connect transformation.
// PREVENTS: ExaBGP legacy bgp.connect surviving migration.
func TestRemoveEnvBGPConnect(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("connect", "true")
	bgp.Set("openwait", "120")
	env.SetContainer("bgp", bgp)
	tree.SetContainer("environment", env)

	assert.True(t, hasEnvBGPConnect(tree), "should detect bgp connect")

	result, err := removeEnvBGPConnect(tree)
	require.NoError(t, err)

	bgpResult := result.GetContainer("environment").GetContainer("bgp")
	require.NotNil(t, bgpResult)
	_, ok := bgpResult.Get("connect")
	assert.False(t, ok, "connect should be removed")
	v, ok := bgpResult.Get("openwait")
	assert.True(t, ok, "openwait should be preserved")
	assert.Equal(t, "120", v)
}

// TestRemoveEnvBGPConnect_Absent verifies no-op when bgp.connect is absent.
func TestRemoveEnvBGPConnect_Absent(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("openwait", "120")
	env.SetContainer("bgp", bgp)
	tree.SetContainer("environment", env)
	assert.False(t, hasEnvBGPConnect(tree))
}

// TestRemoveEnvBGPAccept verifies detection and removal of environment > bgp > accept.
// VALIDATES: migration remove-env-bgp-accept transformation.
// PREVENTS: ExaBGP legacy bgp.accept surviving migration.
func TestRemoveEnvBGPAccept(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("accept", "true")
	bgp.Set("openwait", "120")
	env.SetContainer("bgp", bgp)
	tree.SetContainer("environment", env)

	assert.True(t, hasEnvBGPAccept(tree), "should detect bgp accept")

	result, err := removeEnvBGPAccept(tree)
	require.NoError(t, err)

	bgpResult := result.GetContainer("environment").GetContainer("bgp")
	require.NotNil(t, bgpResult)
	_, ok := bgpResult.Get("accept")
	assert.False(t, ok, "accept should be removed")
	v, ok := bgpResult.Get("openwait")
	assert.True(t, ok, "openwait should be preserved")
	assert.Equal(t, "120", v)
}

// TestRemoveEnvBGPAccept_Absent verifies no-op when bgp.accept is absent.
func TestRemoveEnvBGPAccept_Absent(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("openwait", "120")
	env.SetContainer("bgp", bgp)
	tree.SetContainer("environment", env)
	assert.False(t, hasEnvBGPAccept(tree))
}

// TestHubServerHostToIP verifies detection and rename of plugin > hub > server > host to ip.
// VALIDATES: migration hub-server-host-to-ip transformation.
// PREVENTS: Old hub configs with host leaf silently losing listen address.
func TestHubServerHostToIP(t *testing.T) {
	tree := config.NewTree()
	plug := config.NewTree()
	hub := config.NewTree()
	srv := config.NewTree()
	srv.Set("host", "127.0.0.1")
	srv.Set("port", "12700")
	hub.AddListEntry("server", "local", srv)
	plug.SetContainer("hub", hub)
	tree.SetContainer("plugin", plug)

	assert.True(t, hasHubServerHost(tree), "should detect host leaf")

	result, err := renameHubServerHost(tree)
	require.NoError(t, err)

	servers := result.GetContainer("plugin").GetContainer("hub").GetList("server")
	require.Contains(t, servers, "local")
	entry := servers["local"]
	_, hasHost := entry.Get("host")
	assert.False(t, hasHost, "host should be removed")
	ip, hasIP := entry.Get("ip")
	assert.True(t, hasIP, "ip should exist")
	assert.Equal(t, "127.0.0.1", ip)
	port, hasPort := entry.Get("port")
	assert.True(t, hasPort, "port should be preserved")
	assert.Equal(t, "12700", port)
}

// TestHubServerHostToIP_Absent verifies no-op when host is already ip.
func TestHubServerHostToIP_Absent(t *testing.T) {
	tree := config.NewTree()
	plug := config.NewTree()
	hub := config.NewTree()
	srv := config.NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", "12700")
	hub.AddListEntry("server", "local", srv)
	plug.SetContainer("hub", hub)
	tree.SetContainer("plugin", plug)

	assert.False(t, hasHubServerHost(tree))
}
