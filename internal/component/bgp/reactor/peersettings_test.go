package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: ConnectionMode boolean operations and PeerSettings defaults.
// PREVENTS: Config parsing regressions where connection mode or peer key changes.

// TestConnectionMode_Presets verifies preset connection modes.
func TestConnectionMode_Presets(t *testing.T) {
	assert.True(t, ConnectionBoth.Connect)
	assert.True(t, ConnectionBoth.Accept)
	assert.True(t, ConnectionActive.Connect)
	assert.False(t, ConnectionActive.Accept)
	assert.False(t, ConnectionPassive.Connect)
	assert.True(t, ConnectionPassive.Accept)
}

// TestConnectionMode_IsActive verifies IsActive for all modes.
func TestConnectionMode_IsActive(t *testing.T) {
	tests := []struct {
		mode   ConnectionMode
		active bool
	}{
		{ConnectionActive, true},
		{ConnectionPassive, false},
		{ConnectionBoth, true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.active, tt.mode.IsActive(), "mode=%v", tt.mode)
	}
}

// TestConnectionMode_IsPassive verifies IsPassive for all modes.
func TestConnectionMode_IsPassive(t *testing.T) {
	tests := []struct {
		mode    ConnectionMode
		passive bool
	}{
		{ConnectionActive, false},
		{ConnectionPassive, true},
		{ConnectionBoth, true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.passive, tt.mode.IsPassive(), "mode=%v", tt.mode)
	}
}

// TestNewPeerSettings_Defaults verifies factory defaults.
// VALIDATES: Default values match RFC 4271 and project conventions.
func TestNewPeerSettings_Defaults(t *testing.T) {
	addr := netip.MustParseAddr("192.168.1.1")
	ps := NewPeerSettings(addr, 65001, 65002, 0x01020304)

	assert.Equal(t, addr, ps.Address)
	assert.Equal(t, uint16(DefaultBGPPort), ps.Port, "default port=179")
	assert.Equal(t, uint32(65001), ps.LocalAS)
	assert.Equal(t, uint32(65002), ps.PeerAS)
	assert.Equal(t, uint32(0x01020304), ps.RouterID)
	assert.Equal(t, DefaultReceiveHoldTime, ps.ReceiveHoldTime, "default receive hold time=90s")
	assert.Equal(t, time.Duration(0), ps.SendHoldTime, "default send hold time=0 (auto)")
	assert.Equal(t, ConnectionBoth, ps.Connection)
	assert.True(t, ps.GroupUpdates, "group updates enabled by default")
}

// TestPeerSettings_PeerKey verifies key returns correct AddrPort.
func TestPeerSettings_PeerKey(t *testing.T) {
	ps := NewPeerSettings(netip.MustParseAddr("10.0.0.1"), 65001, 65002, 1)
	assert.Equal(t, netip.MustParseAddrPort("10.0.0.1:179"), ps.PeerKey())
}

// TestPeerSettings_PeerKey_DefaultPort verifies port=0 uses DefaultBGPPort.
func TestPeerSettings_PeerKey_DefaultPort(t *testing.T) {
	ps := NewPeerSettings(netip.MustParseAddr("10.0.0.1"), 65001, 65002, 1)
	ps.Port = 0
	assert.Equal(t, netip.MustParseAddrPort("10.0.0.1:179"), ps.PeerKey())
}

// TestPeerSettings_PeerKey_CustomPort verifies custom port in key.
func TestPeerSettings_PeerKey_CustomPort(t *testing.T) {
	ps := NewPeerSettings(netip.MustParseAddr("10.0.0.1"), 65001, 65002, 1)
	ps.Port = 1179
	assert.Equal(t, netip.MustParseAddrPort("10.0.0.1:1179"), ps.PeerKey())
}

// TestPeerKeyFromAddrPort verifies standalone key builder.
func TestPeerKeyFromAddrPort(t *testing.T) {
	addr := netip.MustParseAddr("192.168.1.1")
	assert.Equal(t, netip.MustParseAddrPort("192.168.1.1:179"), PeerKeyFromAddrPort(addr, 179))
	assert.Equal(t, netip.MustParseAddrPort("192.168.1.1:1179"), PeerKeyFromAddrPort(addr, 1179))
}

// TestPeerKeyFromAddrPort_IPv6 verifies IPv6 address in key.
func TestPeerKeyFromAddrPort_IPv6(t *testing.T) {
	addr := netip.MustParseAddr("2001:db8::1")
	assert.Equal(t, netip.MustParseAddrPort("[2001:db8::1]:179"), PeerKeyFromAddrPort(addr, 179))
}

// TestPeerSettings_IsIBGP verifies iBGP detection (same AS).
func TestPeerSettings_IsIBGP(t *testing.T) {
	ps := NewPeerSettings(netip.MustParseAddr("10.0.0.1"), 65001, 65001, 1)
	assert.True(t, ps.IsIBGP())
	assert.False(t, ps.IsEBGP())
}

// TestPeerSettings_IsEBGP verifies eBGP detection (different AS).
func TestPeerSettings_IsEBGP(t *testing.T) {
	ps := NewPeerSettings(netip.MustParseAddr("10.0.0.1"), 65001, 65002, 1)
	assert.False(t, ps.IsIBGP())
	assert.True(t, ps.IsEBGP())
}

// TestStaticRoute_IsVPN verifies VPN detection by RD presence.
func TestStaticRoute_IsVPN(t *testing.T) {
	assert.True(t, (&StaticRoute{RD: "100:100"}).IsVPN())
	assert.False(t, (&StaticRoute{}).IsVPN())
}

// TestStaticRoute_IsLabeledUnicast verifies labeled unicast detection.
// RFC 8277: Labeled routes have labels but no RD.
func TestStaticRoute_IsLabeledUnicast(t *testing.T) {
	assert.True(t, (&StaticRoute{Labels: []uint32{100}}).IsLabeledUnicast())
	assert.False(t, (&StaticRoute{Labels: []uint32{100}, RD: "100:100"}).IsLabeledUnicast(), "VPN has labels but also RD")
	assert.False(t, (&StaticRoute{}).IsLabeledUnicast())
}

// TestStaticRoute_SingleLabel_Basic verifies first label extraction.
func TestStaticRoute_SingleLabel_Basic(t *testing.T) {
	assert.Equal(t, uint32(100), (&StaticRoute{Labels: []uint32{100, 200}}).SingleLabel())
	assert.Equal(t, uint32(0), (&StaticRoute{}).SingleLabel())
}

// TestStaticRoute_RouteKey verifies route key format.
func TestStaticRoute_RouteKey(t *testing.T) {
	tests := []struct {
		name     string
		route    StaticRoute
		expected string
	}{
		{"plain", StaticRoute{Prefix: netip.MustParsePrefix("10.0.0.0/24")}, "10.0.0.0/24#0"},
		{"with RD", StaticRoute{Prefix: netip.MustParsePrefix("10.0.0.0/24"), RD: "100:100"}, "100:100:10.0.0.0/24#0"},
		{"with PathID", StaticRoute{Prefix: netip.MustParsePrefix("10.0.0.0/24"), PathID: 42}, "10.0.0.0/24#42"},
		{"with RD+PathID", StaticRoute{Prefix: netip.MustParsePrefix("10.0.0.0/24"), RD: "1:1", PathID: 7}, "1:1:10.0.0.0/24#7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.route.RouteKey())
		})
	}
}

// TestDefaultReceiveHoldTime verifies the constant is 90 seconds per RFC 4271.
func TestDefaultReceiveHoldTime(t *testing.T) {
	assert.Equal(t, 90*time.Second, DefaultReceiveHoldTime)
}

// TestDefaultBGPPort verifies the constant is 179 per RFC 4271.
func TestDefaultBGPPort(t *testing.T) {
	assert.Equal(t, 179, DefaultBGPPort)
}
