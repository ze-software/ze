package scenario

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigGenStructure verifies the generated config has the required
// top-level structure: process block for RR plugin and N neighbor blocks.
//
// VALIDATES: Config has process + neighbor blocks.
// PREVENTS: Missing structural elements causing Ze parse failures.
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
	assert.Contains(t, config, "peer 127.0.0.1")
	// Should have 2 peer blocks
	assert.Equal(t, 2, strings.Count(config, "peer 127.0.0.1 {"), "expected 2 peer blocks")
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

	// eBGP peer (index 0): peer-as should be 65001
	assert.Contains(t, config, "peer-as 65001;")
	// iBGP peer (index 1): peer-as should be 65000
	assert.Contains(t, config, "peer-as 65000;")
	// local-as always present
	assert.Contains(t, config, "local-as 65000;")
	// Family block
	assert.Contains(t, config, "ipv4/unicast;")
	// Passive peer should have passive flag
	assert.Contains(t, config, "passive true;")
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
	assert.Contains(t, config, "hold-time 90;")
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

	// Count peer blocks
	count := strings.Count(config, "peer 127.0.0.1 {")
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

	// All peers have passive true (Ze never dials out in per-port mode).
	assert.Equal(t, 2, strings.Count(config, "passive true;"))
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

	assert.Contains(t, config, "port 1790;")
	assert.Contains(t, config, "port 1791;")
	assert.Equal(t, 2, strings.Count(config, "port "))
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
	assert.Contains(t, config, "ipv6/unicast;")
	assert.Contains(t, config, "l2vpn/evpn;")

	// Both peers have ipv4 unicast (2 occurrences).
	assert.Equal(t, 2, strings.Count(config, "ipv4/unicast;"))

	// Only 1 peer has ipv6 unicast.
	assert.Equal(t, 1, strings.Count(config, "ipv6/unicast;"))

	// Only 1 peer has l2vpn evpn.
	assert.Equal(t, 1, strings.Count(config, "l2vpn/evpn;"))
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
	assert.Contains(t, config, "ipv4/unicast;")
}
