package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/capability"
)

// TestParsePeerFromTree verifies basic peer parsing from a map[string]any tree.
//
// VALIDATES: parsePeerFromTree correctly extracts all scalar fields from a config tree.
// PREVENTS: Wrong field mapping between config keys and PeerSettings fields.
func TestParsePeerFromTree(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-as":      "65000",
		"router-id":     "10.0.0.1",
		"hold-time":     "180",
		"connection":    "passive",
		"group-updates": "false",
		"local-address": "192.168.1.1",
		"link-local":    "fe80::1",
	}

	ps, err := parsePeerFromTree("192.0.2.1", tree, 64999, 0x0a000001)
	require.NoError(t, err)

	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), ps.Address)
	assert.Equal(t, uint32(65001), ps.PeerAS)
	assert.Equal(t, uint32(65000), ps.LocalAS) // Peer-level overrides global.
	assert.Equal(t, ipToUint32(netip.MustParseAddr("10.0.0.1")), ps.RouterID)
	assert.Equal(t, 180*time.Second, ps.HoldTime)
	assert.Equal(t, ConnectionPassive, ps.Connection)
	assert.False(t, ps.GroupUpdates)
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), ps.LocalAddress)
	assert.Equal(t, netip.MustParseAddr("fe80::1"), ps.LinkLocal)
	assert.Equal(t, uint16(179), ps.Port) // Default from NewPeerSettings.
}

// TestParsePeerFromTreeDefaults verifies default values when optional fields are absent.
//
// VALIDATES: Minimal tree (only peer-as) produces valid PeerSettings with correct defaults.
// PREVENTS: Nil pointer or missing defaults on minimal config.
func TestParsePeerFromTreeDefaults(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65002",
		"local-address": "auto",
	}

	ps, err := parsePeerFromTree("10.0.0.2", tree, 65000, 0x01020304)
	require.NoError(t, err)

	assert.Equal(t, uint32(65002), ps.PeerAS)
	assert.Equal(t, uint32(65000), ps.LocalAS)       // Global default.
	assert.Equal(t, uint32(0x01020304), ps.RouterID) // Global default.
	assert.Equal(t, 90*time.Second, ps.HoldTime)     // Default.
	assert.Equal(t, ConnectionBoth, ps.Connection)   // Default.
	assert.True(t, ps.GroupUpdates)                  // Default.
	assert.Equal(t, netip.Addr{}, ps.LocalAddress)   // Unset ("auto").
	assert.Equal(t, netip.Addr{}, ps.LinkLocal)      // Unset.
}

// TestParsePeerFromTreeInvalid verifies error handling for invalid trees.
//
// VALIDATES: parsePeerFromTree returns clear errors for bad input.
// PREVENTS: Silent acceptance of invalid config data.
func TestParsePeerFromTreeInvalid(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		tree    map[string]any
		wantErr string
	}{
		{
			name:    "invalid_peer_address",
			addr:    "not-an-ip",
			tree:    map[string]any{"peer-as": "65001"},
			wantErr: "invalid peer address",
		},
		{
			name:    "missing_peer_as",
			addr:    "10.0.0.1",
			tree:    map[string]any{},
			wantErr: "missing required peer-as",
		},
		{
			name:    "invalid_router_id",
			addr:    "10.0.0.1",
			tree:    map[string]any{"peer-as": "65001", "local-address": "auto", "router-id": "not-an-ip"},
			wantErr: "invalid router-id",
		},
		{
			name:    "invalid_local_address",
			addr:    "10.0.0.1",
			tree:    map[string]any{"peer-as": "65001", "local-address": "bad"},
			wantErr: "invalid local-address",
		},
		{
			name:    "invalid_link_local",
			addr:    "10.0.0.1",
			tree:    map[string]any{"peer-as": "65001", "local-address": "auto", "link-local": "bad"},
			wantErr: "invalid link-local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePeerFromTree(tt.addr, tt.tree, 65000, 0)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestParsePeerFromTreeHoldTimeBoundary verifies RFC 4271 hold-time constraints.
//
// VALIDATES: Hold time 0 and >= 3 accepted; 1-2 rejected per RFC 4271 Section 4.2.
// PREVENTS: Accepting invalid hold times that violate the RFC.
// BOUNDARY: 0 (valid), 1 (invalid), 2 (invalid), 3 (valid).
func TestParsePeerFromTreeHoldTimeBoundary(t *testing.T) {
	tests := []struct {
		name    string
		ht      string
		wantErr bool
		wantDur time.Duration
	}{
		{"hold_time_0", "0", false, 0},
		{"hold_time_1_invalid", "1", true, 0},
		{"hold_time_2_invalid", "2", true, 0},
		{"hold_time_3", "3", false, 3 * time.Second},
		{"hold_time_180", "180", false, 180 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := map[string]any{
				"peer-as":       "65001",
				"local-address": "auto",
				"hold-time":     tt.ht,
			}
			ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid hold-time")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantDur, ps.HoldTime)
			}
		})
	}
}

// TestParsePeerFamilies verifies family parsing from a config tree.
//
// VALIDATES: Address families are parsed into Multiprotocol capabilities with correct modes.
// PREVENTS: Wrong AFI/SAFI mapping or missed family modes.
func TestParsePeerFamilies(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"family": map[string]any{
			"ipv4/unicast":   "enable",
			"ipv6/unicast":   "require",
			"ipv4/multicast": "ignore",
			"ipv4/flow":      "disable",
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)

	// 3 families enabled (disable skipped).
	var mpCaps []*capability.Multiprotocol
	for _, c := range ps.Capabilities {
		if mp, ok := c.(*capability.Multiprotocol); ok {
			mpCaps = append(mpCaps, mp)
		}
	}
	assert.Len(t, mpCaps, 3)

	// Check required families.
	assert.Len(t, ps.RequiredFamilies, 1)
	assert.Equal(t, capability.AFIIPv6, ps.RequiredFamilies[0].AFI)
	assert.Equal(t, capability.SAFIUnicast, ps.RequiredFamilies[0].SAFI)

	// Check ignored families.
	assert.Len(t, ps.IgnoreFamilies, 1)
	assert.Equal(t, capability.AFIIPv4, ps.IgnoreFamilies[0].AFI)
	assert.Equal(t, capability.SAFI(2), ps.IgnoreFamilies[0].SAFI) // multicast = 2
}

// TestParsePeerFamilyIgnoreMismatch verifies the ignore-mismatch flag.
//
// VALIDATES: ignore-mismatch in family block sets IgnoreFamilyMismatch on PeerSettings.
// PREVENTS: Missing the special-case key in the family map.
func TestParsePeerFamilyIgnoreMismatch(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"family": map[string]any{
			"ipv4/unicast":    "enable",
			"ignore-mismatch": "true",
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)
	assert.True(t, ps.IgnoreFamilyMismatch)
}

// TestParsePeerFamilyInvalid verifies error on unknown family string.
//
// VALIDATES: Unknown AFI/SAFI produces clear error.
// PREVENTS: Silently ignoring typos in family names.
func TestParsePeerFamilyInvalid(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"family": map[string]any{
			"bogus/family": "enable",
		},
	}

	_, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown address family")
}

// TestParsePeerCapabilities verifies capability parsing from config tree.
//
// VALIDATES: Capabilities (ASN4, extended-message, route-refresh, software-version) are
// correctly parsed into capability objects on PeerSettings.
// PREVENTS: Missing or misconfigured capabilities after config parsing.
func TestParsePeerCapabilities(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"family": map[string]any{
			"ipv4/unicast": "enable",
		},
		"capability": map[string]any{
			"asn4":             "true",
			"extended-message": "true",
			"route-refresh":    "enable",
			"software-version": "enable",
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)

	assert.False(t, ps.DisableASN4)

	// Count capability types.
	var hasExtMsg, hasRR, hasERR, hasSV bool
	for _, c := range ps.Capabilities {
		switch c.(type) {
		case *capability.ExtendedMessage:
			hasExtMsg = true
		case *capability.RouteRefresh:
			hasRR = true
		case *capability.EnhancedRouteRefresh:
			hasERR = true
		case *capability.SoftwareVersion:
			hasSV = true
		}
	}
	assert.True(t, hasExtMsg, "ExtendedMessage capability should be present")
	assert.True(t, hasRR, "RouteRefresh capability should be present")
	assert.True(t, hasERR, "EnhancedRouteRefresh capability should be present")
	assert.True(t, hasSV, "SoftwareVersion capability should be present")
}

// TestParsePeerCapabilityASN4Disabled verifies ASN4 can be disabled.
//
// VALIDATES: asn4 = false sets DisableASN4 = true.
// PREVENTS: Ignoring explicit ASN4 disable in config.
func TestParsePeerCapabilityASN4Disabled(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"capability": map[string]any{
			"asn4": "false",
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)
	assert.True(t, ps.DisableASN4)
}

// TestParsePeerCapabilityGracefulRestart verifies GR config is stored in RawCapabilityConfig.
//
// VALIDATES: graceful-restart block is stored for plugin delivery.
// PREVENTS: Lost GR config when converting from tree to PeerSettings.
func TestParsePeerCapabilityGracefulRestart(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"capability": map[string]any{
			"graceful-restart": map[string]any{
				"restart-time": "120",
			},
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)

	require.NotNil(t, ps.RawCapabilityConfig)
	require.Contains(t, ps.RawCapabilityConfig, "graceful-restart")
	assert.Equal(t, "120", ps.RawCapabilityConfig["graceful-restart"]["restart-time"])
}

// TestParsePeerCapabilityAddPathGlobal verifies global ADD-PATH mode.
//
// VALIDATES: Global add-path "send/receive" creates AddPath capability for all families.
// PREVENTS: ADD-PATH not applied to configured families.
func TestParsePeerCapabilityAddPathGlobal(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"family": map[string]any{
			"ipv4/unicast": "enable",
			"ipv6/unicast": "enable",
		},
		"capability": map[string]any{
			"add-path": "send/receive",
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)

	// Find AddPath capability.
	var addPath *capability.AddPath
	for _, c := range ps.Capabilities {
		if ap, ok := c.(*capability.AddPath); ok {
			addPath = ap
			break
		}
	}
	require.NotNil(t, addPath, "AddPath capability should be present")
	assert.Len(t, addPath.Families, 2)

	// Both families should have AddPathBoth.
	for _, f := range addPath.Families {
		assert.Equal(t, capability.AddPathBoth, f.Mode,
			"family %v should have AddPathBoth", f)
	}
}

// TestParsePeerCapabilityAddPathBlock verifies block-style ADD-PATH config.
//
// VALIDATES: add-path { send true; receive true; } is equivalent to "send/receive".
// PREVENTS: Block-style add-path config not being parsed.
func TestParsePeerCapabilityAddPathBlock(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"family": map[string]any{
			"ipv4/unicast": "enable",
		},
		"capability": map[string]any{
			"add-path": map[string]any{
				"send":    "true",
				"receive": "true",
			},
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)

	var addPath *capability.AddPath
	for _, c := range ps.Capabilities {
		if ap, ok := c.(*capability.AddPath); ok {
			addPath = ap
			break
		}
	}
	require.NotNil(t, addPath)
	require.Len(t, addPath.Families, 1)
	assert.Equal(t, capability.AddPathBoth, addPath.Families[0].Mode)
}

// TestParsePeerCapabilityAddPathSendOnly verifies send-only ADD-PATH.
//
// VALIDATES: add-path "send" creates AddPathSend for all families.
// PREVENTS: Wrong mode when only send is requested.
func TestParsePeerCapabilityAddPathSendOnly(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"family": map[string]any{
			"ipv4/unicast": "enable",
		},
		"capability": map[string]any{
			"add-path": "send",
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)

	var addPath *capability.AddPath
	for _, c := range ps.Capabilities {
		if ap, ok := c.(*capability.AddPath); ok {
			addPath = ap
			break
		}
	}
	require.NotNil(t, addPath)
	require.Len(t, addPath.Families, 1)
	assert.Equal(t, capability.AddPathSend, addPath.Families[0].Mode)
}

// TestParsePeerCapabilityExtendedNextHop verifies RFC 8950 extended next-hop parsing.
//
// VALIDATES: nexthop { ipv4/unicast ipv6; } creates ExtendedNextHop capability.
// PREVENTS: Lost extended next-hop config.
func TestParsePeerCapabilityExtendedNextHop(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"capability": map[string]any{
			"nexthop": map[string]any{
				"ipv4/unicast ipv6": "true",
			},
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)

	var extNH *capability.ExtendedNextHop
	for _, c := range ps.Capabilities {
		if enh, ok := c.(*capability.ExtendedNextHop); ok {
			extNH = enh
			break
		}
	}
	require.NotNil(t, extNH, "ExtendedNextHop capability should be present")
	require.Len(t, extNH.Families, 1)
	assert.Equal(t, capability.AFI(1), extNH.Families[0].NLRIAFI)    // IPv4
	assert.Equal(t, capability.SAFI(1), extNH.Families[0].NLRISAFI)  // unicast
	assert.Equal(t, capability.AFI(2), extNH.Families[0].NextHopAFI) // IPv6
}

// TestParsePeerCapabilityHostname verifies hostname config in RawCapabilityConfig.
//
// VALIDATES: Hostname block is stored correctly for plugin delivery.
// PREVENTS: Lost hostname config when parsing capabilities.
func TestParsePeerCapabilityHostname(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"capability": map[string]any{
			"hostname": map[string]any{
				"host":   "router1",
				"domain": "example.com",
			},
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)

	require.NotNil(t, ps.RawCapabilityConfig)
	require.Contains(t, ps.RawCapabilityConfig, "hostname")
	assert.Equal(t, "router1", ps.RawCapabilityConfig["hostname"]["host"])
	assert.Equal(t, "example.com", ps.RawCapabilityConfig["hostname"]["domain"])
}

// TestParsePeerCapabilityHostnameTopLevel verifies YANG-augmented hostname fields.
//
// VALIDATES: Top-level host-name/domain-name fields (from plugin YANG augmentation) are
// mapped to RawCapabilityConfig["hostname"].
// PREVENTS: Plugin-augmented fields being ignored.
func TestParsePeerCapabilityHostnameTopLevel(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"host-name":     "myhost",
		"domain-name":   "mydomain.net",
		"capability":    map[string]any{},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)

	require.NotNil(t, ps.RawCapabilityConfig)
	assert.Equal(t, "myhost", ps.RawCapabilityConfig["hostname"]["host"])
	assert.Equal(t, "mydomain.net", ps.RawCapabilityConfig["hostname"]["domain"])
}

// TestParsePeerProcessBindings verifies process binding parsing.
//
// VALIDATES: Process bindings with content/receive/send settings are parsed correctly.
// PREVENTS: Lost process config or wrong flag mapping.
func TestParsePeerProcessBindings(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"process": map[string]any{
			"my-rib": map[string]any{
				"content": map[string]any{
					"encoding": "json",
					"format":   "parsed",
				},
				"receive": map[string]any{
					"update":       "true",
					"open":         "true",
					"notification": "true",
					"state":        "true",
				},
				"send": map[string]any{
					"update": "true",
				},
			},
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)

	require.Len(t, ps.ProcessBindings, 1)
	b := ps.ProcessBindings[0]
	assert.Equal(t, "my-rib", b.PluginName)
	assert.Equal(t, "json", b.Encoding)
	assert.Equal(t, "parsed", b.Format)
	assert.True(t, b.ReceiveUpdate)
	assert.True(t, b.ReceiveOpen)
	assert.True(t, b.ReceiveNotification)
	assert.True(t, b.ReceiveState)
	assert.False(t, b.ReceiveKeepalive)
	assert.False(t, b.ReceiveRefresh)
	assert.True(t, b.SendUpdate)
	assert.False(t, b.SendRefresh)
}

// TestParsePeerProcessBindingsReceiveAll verifies the "all" shorthand.
//
// VALIDATES: receive { all; } sets all receive flags to true.
// PREVENTS: Missing flags when using shorthand notation.
func TestParsePeerProcessBindingsReceiveAll(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"process": map[string]any{
			"my-plugin": map[string]any{
				"receive": map[string]any{
					"all": "true",
				},
				"send": map[string]any{
					"all": "true",
				},
			},
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)

	require.Len(t, ps.ProcessBindings, 1)
	b := ps.ProcessBindings[0]
	assert.True(t, b.ReceiveUpdate)
	assert.True(t, b.ReceiveOpen)
	assert.True(t, b.ReceiveNotification)
	assert.True(t, b.ReceiveKeepalive)
	assert.True(t, b.ReceiveRefresh)
	assert.True(t, b.ReceiveState)
	assert.True(t, b.ReceiveSent)
	assert.True(t, b.ReceiveNegotiated)
	assert.True(t, b.SendUpdate)
	assert.True(t, b.SendRefresh)
}

// TestParsePeerCapabilityConfigJSON verifies CapabilityConfigJSON is populated.
//
// VALIDATES: The entire capability block is serialized to JSON for plugin delivery.
// PREVENTS: Plugins not receiving capability config they need.
func TestParsePeerCapabilityConfigJSON(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
		"capability": map[string]any{
			"asn4":          "true",
			"route-refresh": "enable",
		},
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)

	assert.NotEmpty(t, ps.CapabilityConfigJSON)
	assert.Contains(t, ps.CapabilityConfigJSON, `"asn4"`)
	assert.Contains(t, ps.CapabilityConfigJSON, `"route-refresh"`)
}

// TestParsePeerMissingLocalAddress verifies that a peer without local-address is rejected.
//
// VALIDATES: parsePeerFromTree requires local-address to be present in config.
// PREVENTS: Silent OS-dependent source IP selection causing inconsistent behavior.
func TestParsePeerMissingLocalAddress(t *testing.T) {
	tree := map[string]any{
		"peer-as": "65001",
	}

	_, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local-address is required")
}

// TestParsePeerLocalAddressAuto verifies "auto" local-address is treated as unset.
//
// VALIDATES: local-address "auto" does not set LocalAddress.
// PREVENTS: Trying to parse "auto" as an IP address.
func TestParsePeerLocalAddressAuto(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
	}

	ps, err := parsePeerFromTree("10.0.0.1", tree, 65000, 0)
	require.NoError(t, err)
	assert.Equal(t, netip.Addr{}, ps.LocalAddress)
}

// TestParsePeerIPv6Address verifies IPv6 peer addresses work.
//
// VALIDATES: parsePeerFromTree accepts IPv6 peer addresses.
// PREVENTS: IPv4-only assumption in address parsing.
func TestParsePeerIPv6Address(t *testing.T) {
	tree := map[string]any{
		"peer-as":       "65001",
		"local-address": "auto",
	}

	ps, err := parsePeerFromTree("2001:db8::1", tree, 65000, 0)
	require.NoError(t, err)
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), ps.Address)
}

// --- PeersFromTree tests (Step 2) ---

// TestPeersFromTree verifies full peer parsing from a bgp subtree.
//
// VALIDATES: PeersFromTree extracts global defaults and iterates peer map correctly.
// PREVENTS: Wrong global default inheritance or missing peers.
func TestPeersFromTree(t *testing.T) {
	bgpTree := map[string]any{
		"router-id": "10.0.0.1",
		"local-as":  "65000",
		"peer": map[string]any{
			"192.0.2.1": map[string]any{
				"peer-as":       "65001",
				"local-address": "192.0.2.100",
				"hold-time":     "180",
				"family": map[string]any{
					"ipv4/unicast": "enable",
				},
			},
			"192.0.2.2": map[string]any{
				"peer-as":       "65002",
				"local-address": "auto",
				"connection":    "passive",
			},
		},
	}

	peers, err := PeersFromTree(bgpTree)
	require.NoError(t, err)
	require.Len(t, peers, 2)

	// Build a lookup by address for deterministic assertions (map iteration is unordered).
	byAddr := make(map[string]*PeerSettings)
	for _, p := range peers {
		byAddr[p.Address.String()] = p
	}

	// Peer 1: inherits global local-as and router-id.
	p1 := byAddr["192.0.2.1"]
	require.NotNil(t, p1)
	assert.Equal(t, uint32(65001), p1.PeerAS)
	assert.Equal(t, uint32(65000), p1.LocalAS)
	assert.Equal(t, ipToUint32(netip.MustParseAddr("10.0.0.1")), p1.RouterID)
	assert.Equal(t, 180*time.Second, p1.HoldTime)
	assert.Equal(t, ConnectionBoth, p1.Connection)

	// Peer 2: also inherits globals.
	p2 := byAddr["192.0.2.2"]
	require.NotNil(t, p2)
	assert.Equal(t, uint32(65002), p2.PeerAS)
	assert.Equal(t, uint32(65000), p2.LocalAS)
	assert.Equal(t, ConnectionPassive, p2.Connection)
}

// TestPeersFromTreeNoPeers verifies empty peer map returns empty slice.
//
// VALIDATES: PeersFromTree handles bgp tree with no peers gracefully.
// PREVENTS: Nil/error on empty peer map.
func TestPeersFromTreeNoPeers(t *testing.T) {
	bgpTree := map[string]any{
		"router-id": "10.0.0.1",
		"local-as":  "65000",
	}

	peers, err := PeersFromTree(bgpTree)
	require.NoError(t, err)
	assert.Empty(t, peers)
}

// TestPeersFromTreeMissingLocalAS verifies error on missing local-as.
//
// VALIDATES: PeersFromTree requires local-as in bgp tree.
// PREVENTS: Creating peers with zero local-as silently.
func TestPeersFromTreeMissingLocalAS(t *testing.T) {
	bgpTree := map[string]any{
		"router-id": "10.0.0.1",
		"peer": map[string]any{
			"192.0.2.1": map[string]any{
				"peer-as": "65001",
			},
		},
	}

	_, err := PeersFromTree(bgpTree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local-as")
}

// TestPeersFromTreePeerLocalASOverride verifies per-peer local-as override.
//
// VALIDATES: A peer can override the global local-as.
// PREVENTS: Global override clobbering peer-level config.
func TestPeersFromTreePeerLocalASOverride(t *testing.T) {
	bgpTree := map[string]any{
		"router-id": "10.0.0.1",
		"local-as":  "65000",
		"peer": map[string]any{
			"192.0.2.1": map[string]any{
				"peer-as":       "65001",
				"local-address": "auto",
				"local-as":      "65100",
			},
		},
	}

	peers, err := PeersFromTree(bgpTree)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	assert.Equal(t, uint32(65100), peers[0].LocalAS)
}

// TestPeersFromTreeConfiguredFamilies verifies family collection.
//
// VALIDATES: PeersFromTree returns configured families aggregated from all peers.
// PREVENTS: Missing families in deferred auto-load list.
func TestPeersFromTreeConfiguredFamilies(t *testing.T) {
	bgpTree := map[string]any{
		"router-id": "10.0.0.1",
		"local-as":  "65000",
		"peer": map[string]any{
			"192.0.2.1": map[string]any{
				"peer-as":       "65001",
				"local-address": "auto",
				"family": map[string]any{
					"ipv4/unicast": "enable",
					"ipv6/unicast": "enable",
				},
			},
			"192.0.2.2": map[string]any{
				"peer-as":       "65002",
				"local-address": "auto",
				"family": map[string]any{
					"ipv4/unicast": "enable",
				},
			},
		},
	}

	peers, err := PeersFromTree(bgpTree)
	require.NoError(t, err)
	require.Len(t, peers, 2)

	// Verify families are present on the PeerSettings capabilities.
	var totalMPCaps int
	for _, p := range peers {
		for _, c := range p.Capabilities {
			if _, ok := c.(*capability.Multiprotocol); ok {
				totalMPCaps++
			}
		}
	}
	assert.Equal(t, 3, totalMPCaps, "should have 3 total MP capabilities across 2 peers")
}

// TestPeersFromTreePeerError verifies error propagation from bad peer config.
//
// VALIDATES: PeersFromTree propagates per-peer errors with peer address context.
// PREVENTS: Silent skip of invalid peers.
func TestPeersFromTreePeerError(t *testing.T) {
	bgpTree := map[string]any{
		"router-id": "10.0.0.1",
		"local-as":  "65000",
		"peer": map[string]any{
			"192.0.2.1": map[string]any{
				// Missing peer-as → should error.
			},
		},
	}

	_, err := PeersFromTree(bgpTree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "192.0.2.1")
	assert.Contains(t, err.Error(), "peer-as")
}

// TestFamilyModeParsing verifies all family mode string values.
//
// VALIDATES: parseFamilyMode correctly maps all mode strings.
// PREVENTS: Typos or missed mode strings causing wrong behavior.
func TestFamilyModeParsing(t *testing.T) {
	tests := []struct {
		input string
		want  familyMode
	}{
		{"", familyModeEnable},
		{"true", familyModeEnable},
		{"enable", familyModeEnable},
		{"TRUE", familyModeEnable},
		{"false", familyModeDisable},
		{"disable", familyModeDisable},
		{"require", familyModeRequire},
		{"ignore", familyModeIgnore},
		{"unknown", familyModeEnable}, // Lenient: unknown defaults to enable.
	}

	for _, tt := range tests {
		t.Run("mode_"+tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, parseFamilyMode(tt.input))
		})
	}
}

// TestMapHelpers verifies the map navigation helper functions.
//
// VALIDATES: mapString, mapUint32, mapBool, mapMap, flexString handle present/absent/wrong types.
// PREVENTS: Panics on type assertions or incorrect fallback behavior.
func TestMapHelpers(t *testing.T) {
	m := map[string]any{
		"str":    "hello",
		"num":    "42",
		"bool":   "true",
		"nested": map[string]any{"inner": "value"},
		"flex":   "simple",
		"badnum": "notanumber",
	}

	// mapString.
	v, ok := mapString(m, "str")
	assert.True(t, ok)
	assert.Equal(t, "hello", v)
	_, ok = mapString(m, "missing")
	assert.False(t, ok)

	// mapUint32.
	n, ok := mapUint32(m, "num")
	assert.True(t, ok)
	assert.Equal(t, uint32(42), n)
	_, ok = mapUint32(m, "missing")
	assert.False(t, ok)
	_, ok = mapUint32(m, "badnum")
	assert.False(t, ok)

	// mapBool.
	b, ok := mapBool(m, "bool")
	assert.True(t, ok)
	assert.True(t, b)
	_, ok = mapBool(m, "missing")
	assert.False(t, ok)

	// mapMap.
	sub, ok := mapMap(m, "nested")
	assert.True(t, ok)
	assert.Equal(t, "value", sub["inner"])
	_, ok = mapMap(m, "missing")
	assert.False(t, ok)

	// flexString.
	assert.Equal(t, "simple", flexString(m, "flex"))
	assert.Equal(t, "", flexString(m, "nested")) // Map, not string.
	assert.Equal(t, "", flexString(m, "missing"))
}

// TestIpToUint32 verifies IP-to-uint32 conversion.
//
// VALIDATES: IPv4 addresses convert to correct uint32 values.
// PREVENTS: Byte order mistakes in router-id conversion.
func TestIpToUint32(t *testing.T) {
	tests := []struct {
		ip   string
		want uint32
	}{
		{"1.2.3.4", 0x01020304},
		{"10.0.0.1", 0x0a000001},
		{"255.255.255.255", 0xffffffff},
		{"0.0.0.0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.ip)
			assert.Equal(t, tt.want, ipToUint32(addr))
		})
	}

	// IPv6 returns 0.
	assert.Equal(t, uint32(0), ipToUint32(netip.MustParseAddr("::1")))
}

// TestMapToJSON verifies JSON serialization helper.
//
// VALIDATES: mapToJSON produces valid JSON from map[string]any.
// PREVENTS: Empty or nil maps producing invalid JSON.
func TestMapToJSON(t *testing.T) {
	assert.Equal(t, "", mapToJSON(nil))
	assert.Equal(t, "", mapToJSON(map[string]any{}))

	result := mapToJSON(map[string]any{"key": "val"})
	assert.Contains(t, result, `"key"`)
	assert.Contains(t, result, `"val"`)
}
