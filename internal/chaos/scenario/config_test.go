// Design: docs/architecture/chaos-web-dashboard.md -- scenario configuration.

package scenario

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rs/yang" // the rs-client session leaf
	_ "github.com/ze-software/ze/internal/component/bgp/yang"            // registers ze-bgp-conf.yang
	_ "github.com/ze-software/ze/internal/component/hub/yang"            // ze-bgp-conf.yang imports ze-hub-conf
	_ "github.com/ze-software/ze/internal/component/mcp/yang"            // the daemon MCP listener
	_ "github.com/ze-software/ze/internal/component/plugin/yang"         // the config's own bus block
)

// VALIDATES: The BGP block contains one peer with a remote endpoint per profile.
// PREVENTS: A generated scenario dropping peers before Ze can start them.
func TestConfigGenStructure(t *testing.T) {
	profiles := []PeerProfile{
		{Index: 0, ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"), Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1890},
		{Index: 1, ASN: 65000, RouterID: netip.MustParseAddr("10.255.0.2"), IsIBGP: true, Mode: ModePassive, RouteCount: 100, HoldTime: 90, Port: 1891},
	}

	params := ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1790,
		Profiles:  profiles,
	}

	config := GenerateConfig(params)

	assert.Contains(t, config, "bgp {")
	assert.Contains(t, config, "peer chaos-peer-")
	// Should have 2 peer blocks
	assert.Equal(t, 2, strings.Count(config, "remote {"), "expected 2 peer blocks with remote containers")
}

// TestConfigGenPeerBlock verifies each peer block has correct ASN, families,
// and passive flag.
//
// VALIDATES: Peer blocks contain correct per-peer config.
// PREVENTS: Wrong ASN or missing capabilities in generated config.
func TestConfigGenPeerBlock(t *testing.T) {
	profiles := []PeerProfile{
		{Index: 0, ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"), Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1890},
		{Index: 1, ASN: 65000, RouterID: netip.MustParseAddr("10.255.0.2"), IsIBGP: true, Mode: ModePassive, RouteCount: 100, HoldTime: 90, Port: 1891},
	}

	params := ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1790,
		Profiles:  profiles,
	}

	config := GenerateConfig(params)

	// eBGP peer (index 0): remote ASN should be 65001
	assert.Contains(t, config, "remote 65001;")
	// iBGP peer (index 1): remote ASN should be 65000
	assert.Contains(t, config, "remote 65000;")
	// connection container with local sub-container always present
	assert.Contains(t, config, "connection {")
	// Family block
	assert.Contains(t, config, "ipv4/unicast { prefix { maximum 10000; } }")
	// Passive peer should have connect false inside remote block
	assert.Contains(t, config, "connect false;")
}

// TestConfigGenDeterministic verifies that the same profiles always produce
// the same config output.
//
// VALIDATES: Config generation is deterministic.
// PREVENTS: Non-deterministic output breaking reproducibility.
func TestConfigGenDeterministic(t *testing.T) {
	profiles := []PeerProfile{
		{Index: 0, ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"), Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1890},
		{Index: 1, ASN: 65000, RouterID: netip.MustParseAddr("10.255.0.2"), IsIBGP: true, Mode: ModePassive, RouteCount: 100, HoldTime: 90, Port: 1891},
	}

	params := ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1790,
		Profiles:  profiles,
	}

	config1 := GenerateConfig(params)
	config2 := GenerateConfig(params)

	assert.Equal(t, config1, config2)
}

// TestConfigGenRouterID verifies the config includes the router-id.
//
// VALIDATES: Router ID is set in config.
// PREVENTS: Missing router-id causing Ze to fail.
func TestConfigGenRouterID(t *testing.T) {
	profiles := []PeerProfile{
		{Index: 0, ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"), Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1890},
	}

	params := ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1790,
		Profiles:  profiles,
	}

	config := GenerateConfig(params)
	assert.Contains(t, config, "router-id 10.0.0.1;")
}

// TestConfigGenHoldTime verifies the config includes hold-time per peer.
//
// VALIDATES: Hold time is set in peer block.
// PREVENTS: Default hold time being used when specific value intended.
func TestConfigGenHoldTime(t *testing.T) {
	profiles := []PeerProfile{
		{Index: 0, ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"), Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1890},
	}

	params := ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1790,
		Profiles:  profiles,
	}

	config := GenerateConfig(params)
	assert.Contains(t, config, "timer { receive-hold-time 90; }")
}

// TestConfigGenMultiplePeers verifies that config with many peers has
// the correct number of neighbor blocks.
//
// VALIDATES: Each profile produces a neighbor block.
// PREVENTS: Off-by-one in peer iteration.
func TestConfigGenMultiplePeers(t *testing.T) {
	profiles := make([]PeerProfile, 10)
	for i := range profiles {
		profiles[i] = PeerProfile{
			Index:      i,
			ASN:        uint32(65001 + i),
			RouterID:   netip.MustParseAddr("10.255.0.1"),
			Mode:       ModeActive,
			RouteCount: 100,
			HoldTime:   90,
			Port:       1890 + i,
		}
	}

	params := ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1790,
		Profiles:  profiles,
	}

	config := GenerateConfig(params)

	// Count peer blocks (each has a remote container)
	count := strings.Count(config, "peer chaos-peer-")
	require.Equal(t, 10, count, "expected 10 peer blocks")
}

// TestConfigGenAllPeersPassive verifies all chaos peers are passive from Ze's
// perspective. Per-port mode eliminates the need for Ze to dial out.
//
// VALIDATES: All peers have passive true regardless of Mode field.
// PREVENTS: Ze trying to dial fake loopback addresses that don't exist.
func TestConfigGenAllPeersPassive(t *testing.T) {
	profiles := []PeerProfile{
		{Index: 0, ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"), Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1890},
		{Index: 1, ASN: 65002, RouterID: netip.MustParseAddr("10.255.0.2"), Mode: ModePassive, RouteCount: 100, HoldTime: 90, Port: 1891},
	}

	params := ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1790,
		Profiles:  profiles,
	}

	config := GenerateConfig(params)

	// All peers have connect false inside remote block (Ze never dials out in per-port mode).
	assert.Equal(t, 2, strings.Count(config, "connect false;"))
}

// TestConfigGenPerPeerPort verifies each peer gets a unique Ze listen port.
//
// VALIDATES: Per-peer port directive emitted when ZePort is set.
// PREVENTS: All peers sharing a single port (which breaks per-port mode).
func TestConfigGenPerPeerPort(t *testing.T) {
	profiles := []PeerProfile{
		{Index: 0, ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"), Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1890, ZePort: 1790},
		{Index: 1, ASN: 65002, RouterID: netip.MustParseAddr("10.255.0.2"), Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1891, ZePort: 1791},
	}

	params := ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1790,
		Profiles:  profiles,
	}

	config := GenerateConfig(params)

	tree, err := zeconfig.ParseTreeWithYANG(config, nil)
	require.NoError(t, err)
	peers := tree.GetContainer("bgp").GetList("peer")
	require.Len(t, peers, 2)
	require.Contains(t, peers, "chaos-peer-0")
	require.Contains(t, peers, "chaos-peer-1")
	first, _ := peers["chaos-peer-0"].GetContainerPath("connection/local").Get("port")
	second, _ := peers["chaos-peer-1"].GetContainerPath("connection/local").Get("port")
	assert.Equal(t, "1790", first)
	assert.Equal(t, "1791", second)
}

// TestConfigGenMultiFamily verifies that per-peer family blocks use the
// peer's Families list instead of hardcoded ipv4 unicast.
//
// VALIDATES: Config family block matches peer's assigned families.
// PREVENTS: Hardcoded ipv4 unicast ignoring multi-family assignment.
func TestConfigGenMultiFamily(t *testing.T) {
	profiles := []PeerProfile{
		{
			Index: 0, ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"),
			Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1890,
			Families: []string{"ipv4/unicast", "ipv6/unicast", "l2vpn/evpn"},
		},
		{
			Index: 1, ASN: 65002, RouterID: netip.MustParseAddr("10.255.0.2"),
			Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1891,
			Families: []string{"ipv4/unicast"},
		},
	}

	params := ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1790,
		Profiles:  profiles,
	}

	config := GenerateConfig(params)

	// Peer 0 should have all three families.
	assert.Contains(t, config, "ipv6/unicast { prefix { maximum 10000; } }")
	assert.Contains(t, config, "l2vpn/evpn { prefix { maximum 10000; } }")

	// Both peers have ipv4 unicast (2 occurrences).
	assert.Equal(t, 2, strings.Count(config, "ipv4/unicast { prefix { maximum 10000; } }"))

	// Only 1 peer has ipv6 unicast.
	assert.Equal(t, 1, strings.Count(config, "ipv6/unicast { prefix { maximum 10000; } }"))

	// Only 1 peer has l2vpn evpn.
	assert.Equal(t, 1, strings.Count(config, "l2vpn/evpn { prefix { maximum 10000; } }"))
}

// TestConfigGenFallbackToIPv4 verifies that peers with no Families field
// get ipv4 unicast by default (backward compatibility with Phase 1).
//
// VALIDATES: Empty Families defaults to ipv4/unicast.
// PREVENTS: Missing family block when Families is nil.
func TestConfigGenFallbackToIPv4(t *testing.T) {
	profiles := []PeerProfile{
		{Index: 0, ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"),
			Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1890},
	}

	params := ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1790,
		Profiles:  profiles,
	}

	config := GenerateConfig(params)
	assert.Contains(t, config, "ipv4/unicast { prefix { maximum 10000; } }")
}

// VALIDATES: The generated Ze config enables MCP on the requested loopback port.
// PREVENTS: Emitting a port on the wrong node or leaving the service disabled.
func TestConfigGenerateMCPPort(t *testing.T) {
	profiles := []PeerProfile{
		{Index: 0, ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"),
			Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1890},
	}

	params := ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1790,
		Profiles:  profiles,
		MCPPort:   9718,
	}

	config := GenerateConfig(params)
	assert.Contains(t, config, "mcp {")
	assert.Contains(t, config, "port 9718;")
	tree, err := zeconfig.ParseTreeWithYANG(config, nil)
	require.NoError(t, err)
	mcp, enabled := zeconfig.ExtractMCPConfig(tree)
	require.True(t, enabled)
	assert.Equal(t, []zeconfig.ServerEndpoint{{Host: "127.0.0.1", Port: "9718"}}, mcp.Servers)

	// MCPPort 0 should not emit mcp block.
	params.MCPPort = 0
	config = GenerateConfig(params)
	assert.NotContains(t, config, "mcp {")
}

// VALIDATES: The pprof address is the environment setting Ze's schema accepts.
// PREVENTS: Emitting a retired environment debug block.
func TestConfigGeneratePprof(t *testing.T) {
	const address = "127.0.0.1:6061"
	tree, err := zeconfig.ParseTreeWithYANG(GenerateConfig(ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		PprofAddr: address,
	}), nil)
	require.NoError(t, err)
	environment := tree.GetContainer("environment")
	require.NotNil(t, environment)
	got, ok := environment.Get("pprof")
	assert.True(t, ok)
	assert.Equal(t, address, got)
}

// VALIDATES: Propagation config names the output role of each session.
// PREVENTS: Route-server or reflector sessions establishing without their role.
func TestConfigGenPropagationPeerRoles(t *testing.T) {
	tree, err := zeconfig.ParseTreeWithYANG(GenerateConfig(ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		Profiles: []PeerProfile{
			{Index: 0, ASN: 65001, ZePort: 1850, HoldTime: 90, RouteCount: 100},
			{Index: 1, ASN: 65000, ZePort: 1851, HoldTime: 90, RouteCount: 100, IsIBGP: true},
		},
	}), nil)
	require.NoError(t, err)
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	peers := bgp.GetList("peer")
	require.Contains(t, peers, "chaos-peer-0")
	require.Contains(t, peers, "chaos-peer-1")
	external := peers["chaos-peer-0"].GetContainer("session")
	require.NotNil(t, external)
	internal := peers["chaos-peer-1"].GetContainer("session")
	require.NotNil(t, internal)

	externalRS, _ := external.Get("rs-client")
	externalRR, _ := external.Get("route-reflector-client")
	assert.Equal(t, "true", externalRS)
	assert.NotEqual(t, "true", externalRR)
	internalRS, _ := internal.Get("rs-client")
	internalRR, _ := internal.Get("route-reflector-client")
	clusterID, _ := internal.Get("cluster-id")
	assert.NotEqual(t, "true", internalRS)
	assert.Equal(t, "true", internalRR)
	assert.Equal(t, "10.0.0.1", clusterID)
}

// VALIDATES: The generated propagation config imports and exports test routes.
// PREVENTS: A healthy BGP session with all eBGP routes rejected by default.
func TestConfigGenPropagationPolicy(t *testing.T) {
	tree, err := zeconfig.ParseTreeWithYANG(GenerateConfig(ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		Profiles: []PeerProfile{
			{Index: 0, ASN: 65001, ZePort: 1850, HoldTime: 90, RouteCount: 100},
		},
	}), nil)
	require.NoError(t, err)
	filters := tree.GetContainerPath("bgp/filter")
	require.NotNil(t, filters)
	for _, direction := range []string{"import", "export"} {
		policy := filters.GetContainer(direction)
		require.NotNil(t, policy, direction)
		final, ok := policy.Get("final")
		assert.True(t, ok, direction)
		assert.Equal(t, "accept", final, direction)
	}
}

// VALIDATES: The current schema parses built-in endpoint identities on both
// transports, with the route server alone separate when requested.
// PREVENTS: Forking shipped plugins as foreign programs or during a simulation.
func TestConfigGenAttachesEveryPluginItRuns(t *testing.T) {
	profiles := []PeerProfile{
		{Index: 0, ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"), Mode: ModeActive, RouteCount: 100, HoldTime: 90, Port: 1890},
		{Index: 1, ASN: 65000, RouterID: netip.MustParseAddr("10.255.0.2"), IsIBGP: true, Mode: ModePassive, RouteCount: 100, HoldTime: 90, Port: 1891},
	}
	params := ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1790,
		Profiles:  profiles,
	}

	for _, placement := range []struct {
		name     string
		separate bool
	}{
		{name: "in-daemon"},
		{name: "separate-route-server", separate: true},
	} {
		t.Run(placement.name, func(t *testing.T) {
			params.SeparateRouteServer = placement.separate
			config := GenerateConfig(params)
			assert.Contains(t, config, "bus {\n")
			assert.NotContains(t, config, "attach")
			tree, err := zeconfig.ParseTreeWithYANG(config, nil)
			require.NoError(t, err)
			bus := tree.GetContainer("bus")
			require.NotNil(t, bus)
			endpoints := bus.GetList("endpoint")
			assert.Len(t, endpoints, 4)
			for _, name := range []string{"bgp-rib", "bgp-adj-rib-in", "bgp-rs", "bgp-rr"} {
				require.Contains(t, endpoints, name)
				endpoint := endpoints[name]
				identity, _ := endpoint.Get("internal")
				assert.Equal(t, name, identity)
				assert.Nil(t, endpoint.GetContainer("external"))
				mode, processSet := endpoint.Get("process")
				if name == "bgp-rs" && placement.separate {
					assert.True(t, processSet, name)
					assert.Equal(t, "separate", mode, name)
				} else {
					assert.False(t, processSet, name)
				}
			}
		})
	}
}
