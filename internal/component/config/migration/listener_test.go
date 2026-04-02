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

// TestLogBooleansToSubsystems verifies boolean log topics are converted to subsystem levels.
// VALIDATES: AC-1/AC-2 -- `log { packets true; }` -> `log { bgp.wire debug; }`, false -> disabled
// PREVENTS: ExaBGP boolean log leaves surviving migration without conversion to subsystem levels.
func TestLogBooleansToSubsystems(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		value   string
		wantKey string
		wantVal string
	}{
		{"packets true", "packets", "true", "bgp.wire", "debug"},
		{"packets false", "packets", "false", "bgp.wire", "disabled"},
		{"rib true", "rib", "true", "plugin.rib", "debug"},
		{"rib false", "rib", "false", "plugin.rib", "disabled"},
		{"configuration true", "configuration", "true", "config", "debug"},
		{"reactor true", "reactor", "true", "bgp.reactor", "debug"},
		{"daemon true", "daemon", "true", "daemon", "debug"},
		{"processes true", "processes", "true", "plugin", "debug"},
		{"network true", "network", "true", "bgp.wire", "debug"},
		{"statistics true", "statistics", "true", "bgp.metrics", "debug"},
		{"message true", "message", "true", "bgp.wire", "debug"},
		{"timers true", "timers", "true", "bgp.reactor", "debug"},
		{"routes true", "routes", "true", "plugin.rib", "debug"},
		{"parser true", "parser", "true", "config", "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := config.NewTree()
			env := config.NewTree()
			logTree := config.NewTree()
			logTree.Set(tt.topic, tt.value)
			env.SetContainer("log", logTree)
			tree.SetContainer("environment", env)

			assert.True(t, hasLogBooleans(tree), "should detect boolean log topic %s", tt.topic)

			result, err := migrateLogBooleans(tree)
			require.NoError(t, err)

			logResult := result.GetContainer("environment").GetContainer("log")
			require.NotNil(t, logResult)
			// When topic and subsystem have different names, old topic should be gone.
			if tt.topic != tt.wantKey {
				_, hasTopic := logResult.Get(tt.topic)
				assert.False(t, hasTopic, "old topic %s should be removed", tt.topic)
			}
			v, hasNew := logResult.Get(tt.wantKey)
			assert.True(t, hasNew, "subsystem %s should exist", tt.wantKey)
			assert.Equal(t, tt.wantVal, v, "subsystem level")
		})
	}
}

// TestLogBooleansToSubsystems_PreservesOtherLeaves verifies non-boolean log leaves are preserved.
// VALIDATES: Non-topic leaves (level, destination) survive boolean migration.
// PREVENTS: Log migration clobbering unrelated leaves in the log container.
func TestLogBooleansToSubsystems_PreservesOtherLeaves(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	logTree := config.NewTree()
	logTree.Set("packets", "true")
	logTree.Set("level", "warn")
	env.SetContainer("log", logTree)
	tree.SetContainer("environment", env)

	assert.True(t, hasLogBooleans(tree))

	result, err := migrateLogBooleans(tree)
	require.NoError(t, err)

	logResult := result.GetContainer("environment").GetContainer("log")
	require.NotNil(t, logResult)
	v, ok := logResult.Get("level")
	assert.True(t, ok, "level should be preserved")
	assert.Equal(t, "warn", v)
}

// TestLogBooleansToSubsystems_Absent verifies no-op when no boolean log topics exist.
// VALIDATES: No transformation when no boolean topics are present.
// PREVENTS: False positive detection triggering unnecessary migration.
func TestLogBooleansToSubsystems_Absent(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	logTree := config.NewTree()
	logTree.Set("level", "info")
	env.SetContainer("log", logTree)
	tree.SetContainer("environment", env)

	assert.False(t, hasLogBooleans(tree))
}

// TestLogBooleansToSubsystems_NoEnv verifies no-op when environment is absent.
// VALIDATES: No panic or error when environment container is missing.
// PREVENTS: Nil pointer dereference on trees without environment block.
func TestLogBooleansToSubsystems_NoEnv(t *testing.T) {
	tree := config.NewTree()
	assert.False(t, hasLogBooleans(tree))
}

// TestLogBooleansToSubsystems_MergesDuplicates verifies multiple topics mapping to same subsystem.
// packets, network, and message all map to bgp.wire. Only one entry should remain.
// VALIDATES: Duplicate subsystem mappings merge with debug-wins semantics.
// PREVENTS: Multiple entries for the same subsystem in migrated output.
func TestLogBooleansToSubsystems_MergesDuplicates(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	logTree := config.NewTree()
	logTree.Set("packets", "true")
	logTree.Set("network", "false")
	logTree.Set("message", "true")
	env.SetContainer("log", logTree)
	tree.SetContainer("environment", env)

	result, err := migrateLogBooleans(tree)
	require.NoError(t, err)

	logResult := result.GetContainer("environment").GetContainer("log")
	require.NotNil(t, logResult)
	// When multiple topics map to the same subsystem, "true" (debug) wins over "false" (disabled).
	v, ok := logResult.Get("bgp.wire")
	assert.True(t, ok)
	assert.Equal(t, "debug", v)
}

// TestListenerToList verifies flat host+port converted to server list.
// VALIDATES: AC-3/AC-4 -- `web { host 0.0.0.0; port 3443; }` -> `web { enabled true; server main { ip 0.0.0.0; port 3443; } }`
// PREVENTS: Old listener format surviving migration without conversion to named server list.
func TestListenerToList(t *testing.T) {
	containers := []struct {
		name string
		host string
		port string
	}{
		{"web", "0.0.0.0", "3443"},
		{"ssh", "127.0.0.1", "2222"},
		{"mcp", "127.0.0.1", "3000"},
		{"looking-glass", "0.0.0.0", "8080"},
		{"telemetry", "0.0.0.0", "9090"},
	}

	for _, tt := range containers {
		t.Run(tt.name, func(t *testing.T) {
			tree := config.NewTree()
			env := config.NewTree()
			svc := config.NewTree()
			svc.Set("host", tt.host)
			svc.Set("port", tt.port)
			env.SetContainer(tt.name, svc)
			tree.SetContainer("environment", env)

			assert.True(t, hasListenerFlatFormat(tree), "should detect flat format in %s", tt.name)

			result, err := migrateListenerToList(tree)
			require.NoError(t, err)

			svcResult := result.GetContainer("environment").GetContainer(tt.name)
			require.NotNil(t, svcResult, "container %s should exist", tt.name)

			// host should be removed
			_, hasHost := svcResult.Get("host")
			assert.False(t, hasHost, "host should be removed from %s", tt.name)

			// enabled should be true
			enabled, hasEnabled := svcResult.Get("enabled")
			assert.True(t, hasEnabled, "enabled should exist in %s", tt.name)
			assert.Equal(t, "true", enabled)

			// server main should exist with ip and port
			servers := svcResult.GetList("server")
			require.Contains(t, servers, "main", "server main should exist in %s", tt.name)
			srv := servers["main"]
			ip, hasIP := srv.Get("ip")
			assert.True(t, hasIP, "ip should exist in server main")
			assert.Equal(t, tt.host, ip)
			port, hasPort := srv.Get("port")
			assert.True(t, hasPort, "port should exist in server main")
			assert.Equal(t, tt.port, port)
		})
	}
}

// TestListenerToList_Absent verifies no-op when no flat listener format exists.
// VALIDATES: No transformation when services already use server list format.
// PREVENTS: False positive detection triggering unnecessary listener migration.
func TestListenerToList_Absent(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	web := config.NewTree()
	web.Set("enabled", "true")
	srv := config.NewTree()
	srv.Set("ip", "0.0.0.0")
	srv.Set("port", "3443")
	web.AddListEntry("server", "main", srv)
	env.SetContainer("web", web)
	tree.SetContainer("environment", env)

	assert.False(t, hasListenerFlatFormat(tree))
}

// TestListenerToList_HostWithoutPort verifies host-only migration creates server entry without port.
// VALIDATES: migrateListenerToList handles host without port leaf.
// PREVENTS: Missing port leaf causing panic or empty port in server entry.
func TestListenerToList_HostWithoutPort(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	web := config.NewTree()
	web.Set("host", "127.0.0.1")
	env.SetContainer("web", web)
	tree.SetContainer("environment", env)

	assert.True(t, hasListenerFlatFormat(tree), "should detect flat format")

	result, err := migrateListenerToList(tree)
	require.NoError(t, err)

	webResult := result.GetContainer("environment").GetContainer("web")
	require.NotNil(t, webResult)

	servers := webResult.GetList("server")
	require.Contains(t, servers, "main")
	srv := servers["main"]
	ip, hasIP := srv.Get("ip")
	assert.True(t, hasIP, "ip should exist")
	assert.Equal(t, "127.0.0.1", ip)
	_, hasPort := srv.Get("port")
	assert.False(t, hasPort, "port should not exist when not in source")
}

// TestListenerToList_PreservesOtherLeaves verifies non-host/port leaves are preserved.
// VALIDATES: migrateListenerToList preserves unrelated leaves during migration.
// PREVENTS: Non-listener leaves lost during host+port to server list conversion.
func TestListenerToList_PreservesOtherLeaves(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	web := config.NewTree()
	web.Set("host", "0.0.0.0")
	web.Set("port", "3443")
	web.Set("tls", "true")
	env.SetContainer("web", web)
	tree.SetContainer("environment", env)

	result, err := migrateListenerToList(tree)
	require.NoError(t, err)

	webResult := result.GetContainer("environment").GetContainer("web")
	require.NotNil(t, webResult)
	v, ok := webResult.Get("tls")
	assert.True(t, ok, "tls should be preserved")
	assert.Equal(t, "true", v)
}
