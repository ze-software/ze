package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// TestParsePeerFromTree verifies basic peer parsing from a map[string]any tree.
//
// VALIDATES: parsePeerFromTree correctly extracts all scalar fields from a config tree.
// PREVENTS: Wrong field mapping between config keys and PeerSettings fields.
func TestParsePeerFromTree(t *testing.T) {
	tree := map[string]any{
		"remote":        map[string]any{"ip": "192.0.2.1", "as": "65001"},
		"local":         map[string]any{"as": "65000", "ip": "192.168.1.1"},
		"router-id":     "10.0.0.1",
		"hold-time":     "180",
		"connection":    "passive",
		"group-updates": "false",
		"link-local":    "fe80::1",
	}

	ps, err := parsePeerFromTree("peer1", tree, 64999, 0x0a000001)
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
// VALIDATES: Minimal tree (only remote as/ip and local ip) produces valid PeerSettings with correct defaults.
// PREVENTS: Nil pointer or missing defaults on minimal config.
func TestParsePeerFromTreeDefaults(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.2", "as": "65002"},
		"local":  map[string]any{"ip": "auto"},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0x01020304)
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
		name     string
		peerName string
		tree     map[string]any
		wantErr  string
	}{
		{
			name:     "invalid_remote_ip",
			peerName: "peer1",
			tree:     map[string]any{"remote": map[string]any{"ip": "not-an-ip", "as": "65001"}, "local": map[string]any{"ip": "auto"}},
			wantErr:  "invalid remote ip",
		},
		{
			name:     "missing_remote_container",
			peerName: "peer1",
			tree:     map[string]any{},
			wantErr:  "missing required remote container",
		},
		{
			name:     "missing_remote_as",
			peerName: "peer1",
			tree:     map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}, "local": map[string]any{"ip": "auto"}},
			wantErr:  "missing required remote as",
		},
		{
			name:     "missing_remote_ip",
			peerName: "peer1",
			tree:     map[string]any{"remote": map[string]any{"as": "65001"}, "local": map[string]any{"ip": "auto"}},
			wantErr:  "missing required remote ip",
		},
		{
			name:     "invalid_router_id",
			peerName: "peer1",
			tree:     map[string]any{"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"}, "local": map[string]any{"ip": "auto"}, "router-id": "not-an-ip"},
			wantErr:  "invalid router-id",
		},
		{
			name:     "invalid_local_ip",
			peerName: "peer1",
			tree:     map[string]any{"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"}, "local": map[string]any{"ip": "bad"}},
			wantErr:  "invalid local ip",
		},
		{
			name:     "invalid_link_local",
			peerName: "peer1",
			tree:     map[string]any{"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"}, "local": map[string]any{"ip": "auto"}, "link-local": "bad"},
			wantErr:  "invalid link-local",
		},
		{
			name:     "invalid_connection_mode",
			peerName: "peer1",
			tree:     map[string]any{"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"}, "local": map[string]any{"ip": "auto"}, "connection": "maybe"},
			wantErr:  "invalid connection mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePeerFromTree(tt.peerName, tt.tree, 65000, 0)
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
				"remote":    map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":     map[string]any{"ip": "auto"},
				"hold-time": tt.ht,
			}
			ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
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
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"family": map[string]any{
			"ipv4/unicast":   map[string]any{"mode": "enable"},
			"ipv6/unicast":   map[string]any{"mode": "require"},
			"ipv4/multicast": map[string]any{"mode": "ignore"},
			"ipv4/flow":      map[string]any{"mode": "disable"},
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
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
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"family": map[string]any{
			"ipv4/unicast":    map[string]any{"mode": "enable"},
			"ignore-mismatch": map[string]any{"mode": "true"},
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)
	assert.True(t, ps.IgnoreFamilyMismatch)
}

// TestParsePeerFamilyInvalid verifies error on unknown family string.
//
// VALIDATES: Unknown AFI/SAFI produces clear error.
// PREVENTS: Silently ignoring typos in family names.
func TestParsePeerFamilyInvalid(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"family": map[string]any{
			"bogus/family": map[string]any{"mode": "enable"},
		},
	}

	_, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown address family")
}

// TestParsePeerCapabilities verifies capability parsing from config tree.
//
// VALIDATES: Capabilities (ASN4, extended-message, route-refresh) are
// correctly parsed into capability objects on PeerSettings.
// PREVENTS: Missing or misconfigured capabilities after config parsing.
func TestParsePeerCapabilities(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"family": map[string]any{
			"ipv4/unicast": map[string]any{},
		},
		"capability": map[string]any{
			"asn4":             "true",
			"extended-message": "true",
			"route-refresh":    "enable",
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)

	assert.False(t, ps.DisableASN4)

	// Count capability types.
	var hasExtMsg, hasRR, hasERR bool
	for _, c := range ps.Capabilities {
		switch c.(type) {
		case *capability.ExtendedMessage:
			hasExtMsg = true
		case *capability.RouteRefresh:
			hasRR = true
		case *capability.EnhancedRouteRefresh:
			hasERR = true
		}
	}
	assert.True(t, hasExtMsg, "ExtendedMessage capability should be present")
	assert.True(t, hasRR, "RouteRefresh capability should be present")
	assert.True(t, hasERR, "EnhancedRouteRefresh capability should be present")
}

// TestParsePeerCapabilityASN4Disabled verifies ASN4 can be disabled.
//
// VALIDATES: asn4 = false sets DisableASN4 = true.
// PREVENTS: Ignoring explicit ASN4 disable in config.
func TestParsePeerCapabilityASN4Disabled(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"capability": map[string]any{
			"asn4": "false",
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)
	assert.True(t, ps.DisableASN4)
}

// TestParsePeerCapabilityGracefulRestart verifies GR config is stored in RawCapabilityConfig.
//
// VALIDATES: graceful-restart block is stored for plugin delivery.
// PREVENTS: Lost GR config when converting from tree to PeerSettings.
func TestParsePeerCapabilityGracefulRestart(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"capability": map[string]any{
			"graceful-restart": map[string]any{
				"restart-time": "120",
			},
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
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
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"family": map[string]any{
			"ipv4/unicast": map[string]any{},
			"ipv6/unicast": map[string]any{},
		},
		"capability": map[string]any{
			"add-path": "send/receive",
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
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
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"family": map[string]any{
			"ipv4/unicast": map[string]any{},
		},
		"capability": map[string]any{
			"add-path": map[string]any{
				"send":    "true",
				"receive": "true",
			},
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
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
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"family": map[string]any{
			"ipv4/unicast": map[string]any{},
		},
		"capability": map[string]any{
			"add-path": "send",
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
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
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"capability": map[string]any{
			"nexthop": map[string]any{
				"ipv4/unicast": map[string]any{"nhafi": "ipv6"},
			},
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
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

// TestParsePeerCapabilityExtendedNextHopInlineMode verifies inline mode tokens
// on nexthop family lines (e.g., "ipv4/unicast ipv6 require;").
//
// VALIDATES: trailing mode token parsed from structured nexthop family value.
// PREVENTS: inline mode silently ignored, require/refuse not applied.
func TestParsePeerCapabilityExtendedNextHopInlineMode(t *testing.T) {
	tests := []struct {
		name         string
		nhMap        map[string]any
		wantFamilies int
		wantRequired []capability.Code
		wantRefused  []capability.Code
	}{
		{
			name:         "require mode",
			nhMap:        map[string]any{"ipv4/unicast": map[string]any{"nhafi": "ipv6", "mode": "require"}},
			wantFamilies: 1,
			wantRequired: []capability.Code{capability.CodeExtendedNextHop},
		},
		{
			name:         "refuse mode suppresses family",
			nhMap:        map[string]any{"ipv4/unicast": map[string]any{"nhafi": "ipv6", "mode": "refuse"}},
			wantFamilies: 0,
			wantRefused:  []capability.Code{capability.CodeExtendedNextHop},
		},
		{
			name:         "disable mode suppresses family",
			nhMap:        map[string]any{"ipv4/unicast": map[string]any{"nhafi": "ipv6", "mode": "disable"}},
			wantFamilies: 0,
		},
		{
			name:         "enable mode explicit",
			nhMap:        map[string]any{"ipv4/unicast": map[string]any{"nhafi": "ipv6", "mode": "enable"}},
			wantFamilies: 1,
		},
		{
			name: "mixed modes — require wins",
			nhMap: map[string]any{
				"ipv4/unicast":   map[string]any{"nhafi": "ipv6"},
				"ipv4/multicast": map[string]any{"nhafi": "ipv6", "mode": "require"},
			},
			wantFamilies: 2,
			wantRequired: []capability.Code{capability.CodeExtendedNextHop},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := map[string]any{
				"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":  map[string]any{"ip": "auto"},
				"capability": map[string]any{
					"nexthop": tt.nhMap,
				},
			}

			ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
			require.NoError(t, err)

			var extNH *capability.ExtendedNextHop
			for _, c := range ps.Capabilities {
				if enh, ok := c.(*capability.ExtendedNextHop); ok {
					extNH = enh
					break
				}
			}
			if tt.wantFamilies == 0 {
				assert.Nil(t, extNH, "ExtendedNextHop capability should be absent")
			} else {
				require.NotNil(t, extNH, "ExtendedNextHop capability should be present")
				assert.Len(t, extNH.Families, tt.wantFamilies)
			}
			assert.Equal(t, tt.wantRequired, ps.RequiredCapabilities, "RequiredCapabilities")
			assert.Equal(t, tt.wantRefused, ps.RefusedCapabilities, "RefusedCapabilities")
		})
	}
}

// TestParsePeerCapabilityHostname verifies hostname config in RawCapabilityConfig.
//
// VALIDATES: Hostname block is stored correctly for plugin delivery.
// PREVENTS: Lost hostname config when parsing capabilities.
func TestParsePeerCapabilityHostname(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"capability": map[string]any{
			"hostname": map[string]any{
				"host":   "router1",
				"domain": "example.com",
			},
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
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
		"remote":      map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":       map[string]any{"ip": "auto"},
		"host-name":   "myhost",
		"domain-name": "mydomain.net",
		"capability":  map[string]any{},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
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
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"process": map[string]any{
			"my-rib": map[string]any{
				"content": map[string]any{
					"encoding": "json",
					"format":   "parsed",
				},
				"receive": "update open notification state",
				"send":    "update",
			},
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
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

// TestParsePeerProcessBindingsReceiveAllRejected verifies "all" is not accepted.
// Users must list event types explicitly to avoid silently receiving new types
// when plugins register them.
//
// VALIDATES: receive [ all ] rejected with helpful error.
// PREVENTS: Silent inclusion of new plugin event types.
func TestParsePeerProcessBindingsReceiveAllRejected(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"process": map[string]any{
			"my-plugin": map[string]any{
				"receive": "all",
			},
		},
	}

	_, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all")
}

// TestParsePeerProcessBindingsSendAllRejected verifies "all" is not accepted for send.
//
// VALIDATES: send [ all ] rejected with helpful error.
// PREVENTS: Silent inclusion of future send types.
func TestParsePeerProcessBindingsSendAllRejected(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"process": map[string]any{
			"my-plugin": map[string]any{
				"send": "all",
			},
		},
	}

	_, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all")
}

// TestParsePeerProcessBindingsExplicitAll verifies explicit listing of all base types.
//
// VALIDATES: All base receive/send types accepted when listed explicitly.
// PREVENTS: Regression from removing "all" shorthand.
func TestParsePeerProcessBindingsExplicitAll(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"process": map[string]any{
			"my-plugin": map[string]any{
				"receive": "update open notification keepalive refresh state sent negotiated",
				"send":    "update refresh",
			},
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
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

// TestParseOneSendFlagRejectsUnknown verifies that parseOneSendFlag returns an error
// for an unrecognized send token.
//
// VALIDATES: parseOneSendFlag rejects typos with a clear error message listing valid values.
// PREVENTS: Misspelled send flags silently accepted in config.
func TestParseOneSendFlagRejectsUnknown(t *testing.T) {
	var b ProcessBinding
	err := parseOneSendFlag("updat", &b)
	require.Error(t, err, "typo should be rejected")
	assert.Contains(t, err.Error(), "updat")
	assert.Contains(t, err.Error(), "update")
	assert.Contains(t, err.Error(), "refresh")
	assert.False(t, b.SendUpdate, "SendUpdate should remain false")
	assert.False(t, b.SendRefresh, "SendRefresh should remain false")
}

// TestParseSendFlagsMixedValidInvalid verifies that parseSendFlags fails on the first
// invalid token even when valid tokens precede it.
//
// VALIDATES: parseSendFlags stops at first invalid token and returns error.
// PREVENTS: Invalid flags silently skipped when mixed with valid ones.
func TestParseSendFlagsMixedValidInvalid(t *testing.T) {
	var b ProcessBinding
	err := parseSendFlags("update bogus refresh", &b)
	require.Error(t, err, "bogus token should cause failure")
	assert.Contains(t, err.Error(), "bogus")
}

// TestParseReceiveFlagsRejectsUnknown verifies that parseReceiveFlags returns an error
// for event types not registered with plugin.RegisterEventType.
//
// VALIDATES: AC-9: unknown event types rejected at runtime (not YANG).
// PREVENTS: Typos in config silently accepted.
func TestParseReceiveFlagsRejectsUnknown(t *testing.T) {
	var b ProcessBinding
	err := parseReceiveFlags("bogus", &b)
	require.Error(t, err, "unknown event type should be rejected")
	assert.Contains(t, err.Error(), "bogus")
}

// TestParseReceiveFlagsAcceptsRegistered verifies that parseReceiveFlags accepts
// plugin-registered custom event types and stores them in ReceiveCustom.
//
// VALIDATES: AC-1: registered event types accepted in receive config.
// PREVENTS: Plugin-registered types incorrectly rejected.
func TestParseReceiveFlagsAcceptsRegistered(t *testing.T) {
	// Register a custom event type for this test.
	plugin.RegisterEventType(plugin.NamespaceBGP, "test-custom-event") //nolint:errcheck // test setup

	var b ProcessBinding
	err := parseReceiveFlags("update test-custom-event", &b)
	require.NoError(t, err)
	assert.True(t, b.ReceiveUpdate, "base type should be set")
	assert.True(t, b.ReceiveCustom["test-custom-event"], "custom type should be in ReceiveCustom")
}

// TestReceiveCustomMapInit verifies that parseReceiveFlags correctly initializes and
// reuses the ReceiveCustom map for plugin-registered custom event types.
//
// VALIDATES: First custom event type initializes ReceiveCustom from nil; second reuses existing map.
// PREVENTS: Nil map panic on first custom event or map re-creation losing earlier entries.
func TestReceiveCustomMapInit(t *testing.T) {
	plugin.RegisterEventType(plugin.NamespaceBGP, "test-map-init-1") //nolint:errcheck // test setup
	plugin.RegisterEventType(plugin.NamespaceBGP, "test-map-init-2") //nolint:errcheck // test setup

	var b ProcessBinding
	// Confirm ReceiveCustom starts nil.
	assert.Nil(t, b.ReceiveCustom, "ReceiveCustom should be nil before any custom event")

	// First custom event: should initialize the map.
	err := parseReceiveFlags("test-map-init-1", &b)
	require.NoError(t, err)
	require.NotNil(t, b.ReceiveCustom, "ReceiveCustom should be initialized after first custom event")
	assert.True(t, b.ReceiveCustom["test-map-init-1"])

	// Second custom event: should reuse the same map without losing the first entry.
	err = parseReceiveFlags("test-map-init-2", &b)
	require.NoError(t, err)
	assert.True(t, b.ReceiveCustom["test-map-init-1"], "first custom event should still be present")
	assert.True(t, b.ReceiveCustom["test-map-init-2"], "second custom event should be added")
	assert.Len(t, b.ReceiveCustom, 2, "exactly two custom events should be in the map")
}

// TestParsePeerCapabilityConfigJSON verifies CapabilityConfigJSON is populated.
//
// VALIDATES: The entire capability block is serialized to JSON for plugin delivery.
// PREVENTS: Plugins not receiving capability config they need.
func TestParsePeerCapabilityConfigJSON(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"capability": map[string]any{
			"asn4":          "true",
			"route-refresh": "enable",
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)

	assert.NotEmpty(t, ps.CapabilityConfigJSON)
	assert.Contains(t, ps.CapabilityConfigJSON, `"asn4"`)
	assert.Contains(t, ps.CapabilityConfigJSON, `"route-refresh"`)
}

// TestParsePeerMissingLocalIP verifies that a peer without local ip is rejected.
//
// VALIDATES: parsePeerFromTree requires local > ip to be present in config.
// PREVENTS: Silent OS-dependent source IP selection causing inconsistent behavior.
func TestParsePeerMissingLocalIP(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"as": "65000"},
	}

	_, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local ip is required")
}

// TestParsePeerLocalIPAuto verifies "auto" local ip is treated as unset.
//
// VALIDATES: local > ip "auto" does not set LocalAddress.
// PREVENTS: Trying to parse "auto" as an IP address.
func TestParsePeerLocalIPAuto(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)
	assert.Equal(t, netip.Addr{}, ps.LocalAddress)
}

// TestParsePeerIPv6Address verifies IPv6 peer addresses work.
//
// VALIDATES: parsePeerFromTree accepts IPv6 peer addresses.
// PREVENTS: IPv4-only assumption in address parsing.
func TestParsePeerIPv6Address(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "2001:db8::1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
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
		"local":     map[string]any{"as": "65000"},
		"peer": map[string]any{
			"peer1": map[string]any{
				"remote":    map[string]any{"ip": "192.0.2.1", "as": "65001"},
				"local":     map[string]any{"ip": "192.0.2.100"},
				"hold-time": "180",
				"family": map[string]any{
					"ipv4/unicast": map[string]any{},
				},
			},
			"peer2": map[string]any{
				"remote":     map[string]any{"ip": "192.0.2.2", "as": "65002"},
				"local":      map[string]any{"ip": "auto"},
				"connection": "passive",
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
		"local":     map[string]any{"as": "65000"},
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
			"peer1": map[string]any{
				"remote": map[string]any{"ip": "192.0.2.1", "as": "65001"},
				"local":  map[string]any{"ip": "auto"},
			},
		},
	}

	_, err := PeersFromTree(bgpTree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local as")
}

// TestPeersFromTreePeerLocalASOverride verifies per-peer local-as override.
//
// VALIDATES: A peer can override the global local-as.
// PREVENTS: Global override clobbering peer-level config.
func TestPeersFromTreePeerLocalASOverride(t *testing.T) {
	bgpTree := map[string]any{
		"router-id": "10.0.0.1",
		"local":     map[string]any{"as": "65000"},
		"peer": map[string]any{
			"peer1": map[string]any{
				"remote": map[string]any{"ip": "192.0.2.1", "as": "65001"},
				"local":  map[string]any{"ip": "auto", "as": "65100"},
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
		"local":     map[string]any{"as": "65000"},
		"peer": map[string]any{
			"peer1": map[string]any{
				"remote": map[string]any{"ip": "192.0.2.1", "as": "65001"},
				"local":  map[string]any{"ip": "auto"},
				"family": map[string]any{
					"ipv4/unicast": map[string]any{},
					"ipv6/unicast": map[string]any{},
				},
			},
			"peer2": map[string]any{
				"remote": map[string]any{"ip": "192.0.2.2", "as": "65002"},
				"local":  map[string]any{"ip": "auto"},
				"family": map[string]any{
					"ipv4/unicast": map[string]any{},
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
		"local":     map[string]any{"as": "65000"},
		"peer": map[string]any{
			"peer1": map[string]any{
				// Missing remote container -> should error.
			},
		},
	}

	_, err := PeersFromTree(bgpTree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "peer1")
	assert.Contains(t, err.Error(), "remote")
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

// TestParseConnectionMode verifies parse round-trip for all valid modes and error on invalid.
//
// VALIDATES: ParseConnectionMode accepts "both", "passive", "active", "" and rejects invalid strings.
// PREVENTS: Silently accepting typos or unknown connection mode strings.
func TestParseConnectionMode(t *testing.T) {
	tests := []struct {
		input   string
		want    ConnectionMode
		wantErr bool
	}{
		{"both", ConnectionBoth, false},
		{"passive", ConnectionPassive, false},
		{"active", ConnectionActive, false},
		{"", ConnectionBoth, false}, // empty defaults to both
		{"maybe", 0, true},
		{"BOTH", 0, true},    // case-sensitive
		{"Passive", 0, true}, // case-sensitive
	}
	for _, tt := range tests {
		t.Run("input_"+tt.input, func(t *testing.T) {
			got, err := ParseConnectionMode(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid connection mode")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestConnectionModeString verifies String() returns the config-level name for each mode.
//
// VALIDATES: String() produces round-trippable output for valid modes and a safe default for zero value.
// PREVENTS: Zero value (invalid) silently acting as a valid mode in log output or config generation.
func TestConnectionModeString(t *testing.T) {
	tests := []struct {
		mode ConnectionMode
		want string
	}{
		{ConnectionBoth, "both"},
		{ConnectionPassive, "passive"},
		{ConnectionActive, "active"},
		{ConnectionMode(0), "both"}, // zero value defaults to "both" in String()
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.mode.String())
		})
	}
}

// TestConnectionModeIsActiveIsPassive verifies bitmask helpers.
//
// VALIDATES: IsActive/IsPassive correctly reflect the bitmask bits.
// PREVENTS: Bitmask logic errors where Both doesn't report both capabilities.
func TestConnectionModeIsActiveIsPassive(t *testing.T) {
	tests := []struct {
		mode      ConnectionMode
		isActive  bool
		isPassive bool
	}{
		{ConnectionBoth, true, true},
		{ConnectionActive, true, false},
		{ConnectionPassive, false, true},
		{ConnectionMode(0), false, false}, // zero value: neither active nor passive
	}
	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			assert.Equal(t, tt.isActive, tt.mode.IsActive(), "IsActive")
			assert.Equal(t, tt.isPassive, tt.mode.IsPassive(), "IsPassive")
		})
	}
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

// TestParseCapabilityMode verifies that all capability types accept
// the four-mode vocabulary: enable, disable, require, refuse.
//
// VALIDATES: capMode type and parseCapMode parse all four modes for simple caps.
// PREVENTS: require/refuse silently ignored for non-family capabilities.
func TestParseCapabilityMode(t *testing.T) {
	tests := []struct {
		name            string
		capConfig       map[string]any
		wantDisableASN4 bool
		wantHasExtMsg   bool
		wantHasRR       bool
		wantRequired    []capability.Code
		wantRefused     []capability.Code
	}{
		{
			name:            "asn4 require",
			capConfig:       map[string]any{"asn4": "require"},
			wantDisableASN4: false, // require = advertise + enforce
			wantRequired:    []capability.Code{capability.CodeASN4},
		},
		{
			name:            "asn4 refuse",
			capConfig:       map[string]any{"asn4": "refuse"},
			wantDisableASN4: true, // refuse = don't advertise + reject if peer has it
			wantRefused:     []capability.Code{capability.CodeASN4},
		},
		{
			name:          "extended-message require",
			capConfig:     map[string]any{"extended-message": "require"},
			wantHasExtMsg: true,
			wantRequired:  []capability.Code{capability.CodeExtendedMessage},
		},
		{
			name:         "route-refresh require",
			capConfig:    map[string]any{"route-refresh": "require"},
			wantHasRR:    true,
			wantRequired: []capability.Code{capability.CodeRouteRefresh},
		},
		{
			name:        "route-refresh refuse",
			capConfig:   map[string]any{"route-refresh": "refuse"},
			wantRefused: []capability.Code{capability.CodeRouteRefresh},
		},
		{
			name:          "multiple require and refuse",
			capConfig:     map[string]any{"asn4": "require", "extended-message": "refuse"},
			wantHasExtMsg: false,
			wantRequired:  []capability.Code{capability.CodeASN4},
			wantRefused:   []capability.Code{capability.CodeExtendedMessage},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := map[string]any{
				"remote":     map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":      map[string]any{"ip": "auto"},
				"capability": tt.capConfig,
			}

			ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
			require.NoError(t, err)

			assert.Equal(t, tt.wantDisableASN4, ps.DisableASN4, "DisableASN4")

			var hasExtMsg, hasRR bool
			for _, c := range ps.Capabilities {
				switch c.(type) {
				case *capability.ExtendedMessage:
					hasExtMsg = true
				case *capability.RouteRefresh:
					hasRR = true
				}
			}
			assert.Equal(t, tt.wantHasExtMsg, hasExtMsg, "ExtendedMessage present")
			assert.Equal(t, tt.wantHasRR, hasRR, "RouteRefresh present")

			assert.Equal(t, tt.wantRequired, ps.RequiredCapabilities, "RequiredCapabilities")
			assert.Equal(t, tt.wantRefused, ps.RefusedCapabilities, "RefusedCapabilities")
		})
	}
}

// TestParseCapabilityModeBackwardsCompat verifies old syntax still works.
//
// VALIDATES: true/false/bare name map to enable/disable correctly.
// PREVENTS: Breaking existing config files.
func TestParseCapabilityModeBackwardsCompat(t *testing.T) {
	tests := []struct {
		name            string
		capConfig       map[string]any
		wantDisableASN4 bool
		wantHasExtMsg   bool
	}{
		{
			name:            "asn4 true means enable",
			capConfig:       map[string]any{"asn4": "true"},
			wantDisableASN4: false,
		},
		{
			name:            "asn4 false means disable",
			capConfig:       map[string]any{"asn4": "false"},
			wantDisableASN4: true,
		},
		{
			name:            "asn4 enable",
			capConfig:       map[string]any{"asn4": "enable"},
			wantDisableASN4: false,
		},
		{
			name:          "extended-message true means enable",
			capConfig:     map[string]any{"extended-message": "true"},
			wantHasExtMsg: true,
		},
		{
			name:          "extended-message enable means enable",
			capConfig:     map[string]any{"extended-message": "enable"},
			wantHasExtMsg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := map[string]any{
				"remote":     map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":      map[string]any{"ip": "auto"},
				"capability": tt.capConfig,
			}

			ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
			require.NoError(t, err)

			assert.Equal(t, tt.wantDisableASN4, ps.DisableASN4)

			var hasExtMsg bool
			for _, c := range ps.Capabilities {
				if _, ok := c.(*capability.ExtendedMessage); ok {
					hasExtMsg = true
				}
			}
			assert.Equal(t, tt.wantHasExtMsg, hasExtMsg)
		})
	}
}

// TestParseAddPathWithMode verifies add-path capability modes.
//
// VALIDATES: Global and per-family add-path modes parsed with trailing mode token.
// PREVENTS: Mode token consumed as direction, or direction parsing broken.
func TestParseAddPathWithMode(t *testing.T) {
	tests := []struct {
		name         string
		tree         map[string]any
		wantRequired []capability.Code
		wantRefused  []capability.Code
	}{
		{
			name: "global add-path require",
			tree: map[string]any{
				"remote":     map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":      map[string]any{"ip": "auto"},
				"family":     map[string]any{"ipv4/unicast": map[string]any{}},
				"capability": map[string]any{"add-path": "send/receive require"},
			},
			wantRequired: []capability.Code{capability.CodeAddPath},
		},
		{
			name: "per-family add-path require",
			tree: map[string]any{
				"remote":     map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":      map[string]any{"ip": "auto"},
				"family":     map[string]any{"ipv4/unicast": map[string]any{}},
				"capability": map[string]any{},
				"add-path":   map[string]any{"ipv4/unicast": map[string]any{"direction": "send", "mode": "require"}},
			},
			wantRequired: []capability.Code{capability.CodeAddPath},
		},
		{
			name: "global add-path refuse",
			tree: map[string]any{
				"remote":     map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":      map[string]any{"ip": "auto"},
				"family":     map[string]any{"ipv4/unicast": map[string]any{}},
				"capability": map[string]any{"add-path": "send/receive refuse"},
			},
			wantRefused: []capability.Code{capability.CodeAddPath},
		},
		{
			name: "global add-path no mode means enable",
			tree: map[string]any{
				"remote":     map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":      map[string]any{"ip": "auto"},
				"family":     map[string]any{"ipv4/unicast": map[string]any{}},
				"capability": map[string]any{"add-path": "send/receive"},
			},
			// No mode specified = enable (default) — no require/refuse entries.
		},
		{
			name: "global add-path disable suppresses capability",
			tree: map[string]any{
				"remote":     map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":      map[string]any{"ip": "auto"},
				"family":     map[string]any{"ipv4/unicast": map[string]any{}},
				"capability": map[string]any{"add-path": "send/receive disable"},
			},
			// disable = don't advertise, no enforcement.
		},
		{
			name: "global add-path refuse suppresses capability",
			tree: map[string]any{
				"remote":     map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":      map[string]any{"ip": "auto"},
				"family":     map[string]any{"ipv4/unicast": map[string]any{}},
				"capability": map[string]any{"add-path": "send/receive refuse"},
			},
			wantRefused: []capability.Code{capability.CodeAddPath},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps, err := parsePeerFromTree("peer1", tt.tree, 65000, 0)
			require.NoError(t, err)

			assert.Equal(t, tt.wantRequired, ps.RequiredCapabilities, "RequiredCapabilities")
			assert.Equal(t, tt.wantRefused, ps.RefusedCapabilities, "RefusedCapabilities")

			// Verify capability suppression for disable/refuse.
			var hasAddPath bool
			for _, c := range ps.Capabilities {
				if _, ok := c.(*capability.AddPath); ok {
					hasAddPath = true
				}
			}
			if tt.name == "global add-path disable suppresses capability" ||
				tt.name == "global add-path refuse suppresses capability" {
				assert.False(t, hasAddPath, "AddPath capability should be suppressed")
			}
		})
	}
}

// TestParseGracefulRestartWithMode verifies block-level mode key for GR.
//
// VALIDATES: graceful-restart block accepts mode key alongside restart-time.
// PREVENTS: mode key ignored in block capabilities.
func TestParseGracefulRestartWithMode(t *testing.T) {
	tests := []struct {
		name         string
		grConfig     map[string]any
		wantRequired []capability.Code
		wantRefused  []capability.Code
	}{
		{
			name:         "GR require",
			grConfig:     map[string]any{"mode": "require", "restart-time": "120"},
			wantRequired: []capability.Code{capability.CodeGracefulRestart},
		},
		{
			name:        "GR refuse",
			grConfig:    map[string]any{"mode": "refuse"},
			wantRefused: []capability.Code{capability.CodeGracefulRestart},
		},
		{
			name:     "GR no mode means enable",
			grConfig: map[string]any{"restart-time": "120"},
			// No mode = enable by default — no require/refuse.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := map[string]any{
				"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":  map[string]any{"ip": "auto"},
				"capability": map[string]any{
					"graceful-restart": tt.grConfig,
				},
			}

			ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
			require.NoError(t, err)

			assert.Equal(t, tt.wantRequired, ps.RequiredCapabilities, "RequiredCapabilities")
			assert.Equal(t, tt.wantRefused, ps.RefusedCapabilities, "RefusedCapabilities")
		})
	}
}

// TestParsePeerMD5FieldsParsed verifies md5-password and md5-ip are stored in PeerSettings.
//
// VALIDATES: MD5 fields are populated from config on all platforms.
// PREVENTS: MD5 config silently ignored during parsing.
func TestParsePeerMD5FieldsParsed(t *testing.T) {
	tree := map[string]any{
		"remote":       map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":        map[string]any{"ip": "auto"},
		"md5-password": "bgp-secret-key",
		"md5-ip":       "192.0.2.100",
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)
	assert.Equal(t, "bgp-secret-key", ps.MD5Key)
	assert.Equal(t, netip.MustParseAddr("192.0.2.100"), ps.MD5IP)
}

// TestParsePeerMD5InvalidIP verifies md5-ip validation.
//
// VALIDATES: Invalid md5-ip returns error.
// PREVENTS: Broken MD5 configuration silently accepted.
func TestParsePeerMD5InvalidIP(t *testing.T) {
	tree := map[string]any{
		"remote":       map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":        map[string]any{"ip": "auto"},
		"md5-password": "secret",
		"md5-ip":       "not-an-ip",
	}

	_, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid md5-ip")
}

// TestParsePeerNoMD5FieldsWhenAbsent verifies MD5 fields are empty when not configured.
//
// VALIDATES: MD5 fields default to zero values when md5-password is absent.
// PREVENTS: False positive MD5 activation.
func TestParsePeerNoMD5FieldsWhenAbsent(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)
	assert.Empty(t, ps.MD5Key)
	assert.False(t, ps.MD5IP.IsValid())
}

// TestParsePeerMD5IPWithoutPassword verifies md5-ip without md5-password is rejected.
//
// VALIDATES: md5-ip requires md5-password to be set.
// PREVENTS: Orphaned md5-ip silently ignored.
func TestParsePeerMD5IPWithoutPassword(t *testing.T) {
	tree := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
		"local":  map[string]any{"ip": "auto"},
		"md5-ip": "10.0.0.99",
	}

	_, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "md5-ip requires md5-password")
}

// TestParsePeerFromTree_Name verifies the Name field is parsed from the peer tree.
//
// VALIDATES: AC-7 -- peer name flows from config tree into PeerSettings.
// PREVENTS: Name field silently dropped during parsing.
func TestParsePeerFromTree_Name(t *testing.T) {
	peers, err := PeersFromTree(map[string]any{
		"local":     map[string]any{"as": "65000"},
		"router-id": "1.2.3.4",
		"peer": map[string]any{
			"router-east": map[string]any{
				"remote": map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":  map[string]any{"ip": "auto"},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, peers, 1)
	assert.Equal(t, "router-east", peers[0].Name)
}

// TestParsePeerFromTree_GroupName verifies the GroupName field is parsed from the peer tree.
//
// VALIDATES: AC-7 -- group name injected by ResolveBGPTree flows into PeerSettings.
// PREVENTS: GroupName field silently dropped during parsing.
func TestParsePeerFromTree_GroupName(t *testing.T) {
	peers, err := PeersFromTree(map[string]any{
		"local":     map[string]any{"as": "65000"},
		"router-id": "1.2.3.4",
		"peer": map[string]any{
			"peer1": map[string]any{
				"remote":     map[string]any{"ip": "10.0.0.1", "as": "65001"},
				"local":      map[string]any{"ip": "auto"},
				"group-name": "rr-clients",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, peers, 1)
	assert.Equal(t, "rr-clients", peers[0].GroupName)
}
