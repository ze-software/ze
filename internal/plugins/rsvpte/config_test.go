// VALIDATES: parseConfig handles the real config-delivery shape produced by
// config.Tree.ToMap + BuildPluginConfigSections: the subtree is wrapped under
// its root key ({"rsvp-te": {...}}), leaves are JSON strings (numeric leaves
// arrive as "30"), and YANG lists are keyed maps (interface keyed by name,
// tunnel keyed by name, explicit-route keyed by index) rather than arrays. A
// parser reading the unwrapped / numeric / array shape silently produced an
// empty config, leaving the engine idle and admission unconfigured -- the defect
// this test pins down. Hop order is recovered by sorting on the numeric index.
package rsvpte

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestParseRSVPTEConfigRealShape(t *testing.T) {
	// Exactly the JSON the engine receives at boot for a one-interface,
	// one-tunnel config with a two-hop explicit route.
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{` +
			`"router-id":"10.0.0.1",` +
			`"refresh-period":"30",` +
			`"interface":{"lo":{"address":"127.0.0.1/8","max-bandwidth":"10e9","max-reservable-bandwidth":"8e9"}},` +
			`"tunnel":{"to-egress":{"bandwidth":"1e9","destination":"10.0.0.9","tunnel-id":"1",` +
			`"explicit-route":{"2":{"address":"10.0.0.9/32"},"1":{"address":"10.0.0.5/32"}}}}` +
			`}}`,
	}}

	cfg, err := parseConfig(sections)
	require.NoError(t, err)

	assert.True(t, cfg.RouterID.IsValid(), "router-id must parse from the wrapped tree")
	assert.Equal(t, "10.0.0.1", cfg.RouterID.String())
	assert.Equal(t, 30*time.Second, cfg.RefreshPeriod, "string-typed numeric leaf must parse")

	require.Len(t, cfg.Interfaces, 1)
	lo := cfg.Interfaces[0]
	assert.Equal(t, "lo", lo.Name, "interface list key becomes the name")
	assert.Equal(t, float32(10e9), lo.MaxBW)
	assert.Equal(t, float32(8e9), lo.MaxReservableBW)
	assert.Equal(t, "127.0.0.1/8", lo.Prefix.String())

	require.Len(t, cfg.Tunnels, 1)
	tun := cfg.Tunnels[0]
	assert.Equal(t, "to-egress", tun.Name, "tunnel list key becomes the name")
	assert.Equal(t, "10.0.0.9", tun.Destination.String())
	assert.Equal(t, uint16(1), tun.TunnelID)
	assert.Equal(t, float32(1e9), tun.Bandwidth)

	// explicit-route is keyed by index in the tree (unordered); the parser must
	// restore hop order by numeric index, so hop 1 precedes hop 2.
	require.Len(t, tun.ERO, 2)
	assert.Equal(t, "10.0.0.5/32", tun.ERO[0].Address.String())
	assert.Equal(t, "10.0.0.9/32", tun.ERO[1].Address.String())
}

func TestParseRSVPTEConfigLooseHop(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{"router-id":"10.0.0.1","tunnel":{"t1":{"destination":"10.0.0.9","tunnel-id":"2",` +
			`"explicit-route":{"1":{"address":"10.0.0.9/32","type":"loose"}}}}}}`,
	}}
	cfg, err := parseConfig(sections)
	require.NoError(t, err)
	require.Len(t, cfg.Tunnels, 1)
	require.Len(t, cfg.Tunnels[0].ERO, 1)
	assert.True(t, cfg.Tunnels[0].ERO[0].Loose, "type loose must set the loose bit")
}

func TestParseRSVPTEConfigDefaultsAndEmpty(t *testing.T) {
	cfg, err := parseConfig(nil)
	require.NoError(t, err)
	assert.False(t, cfg.RouterID.IsValid())
	assert.Equal(t, DefaultRefreshPeriod, cfg.RefreshPeriod)
	assert.Equal(t, DefaultRefreshMultiplier, cfg.RefreshMultiplier)
	assert.Empty(t, cfg.Interfaces)
	assert.Empty(t, cfg.Tunnels)
}

func TestParseRSVPTEConfigInvalidRouterID(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{"router-id":"nope"}}`,
	}}
	_, err := parseConfig(sections)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid router-id")
}

// TestParseRSVPTEConfigFastReroute parses the fast-reroute container on a tunnel
// (booleans arrive as JSON strings) and confirms the head-end PATH then carries
// FAST_REROUTE + SESSION_ATTRIBUTE -- the config-to-wire wiring (AC-1).
func TestParseRSVPTEConfigFastReroute(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{"router-id":"10.0.0.1","tunnel":{"t1":{"destination":"10.0.0.9","tunnel-id":"1","bandwidth":"1e9",` +
			`"fast-reroute":{"backup":"facility","node-protection":"true","hop-limit":"8"}}}}}`,
	}}
	cfg, err := parseConfig(sections)
	require.NoError(t, err)
	require.Len(t, cfg.Tunnels, 1)
	fr := cfg.Tunnels[0].FastReroute
	require.NotNil(t, fr, "fast-reroute container parsed")
	assert.False(t, fr.OneToOne, "backup facility -> not one-to-one")
	assert.True(t, fr.NodeProtection)
	assert.Equal(t, uint8(8), fr.HopLimit)

	// config -> PSB.Protection -> PATH carries the objects.
	pr := fr.protection(cfg.Tunnels[0])
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: cfg.Tunnels[0].Destination, TunnelID: 1},
		SenderTemplate: senderTemplateIPv4{SenderAddr: cfg.RouterID, LSPID: 1},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		Protection:     pr,
	}
	msg, err := DecodeMessage(buildPath(psb, cfg.RouterID, 64))
	require.NoError(t, err)
	assert.True(t, msg.HasFastReroute)
	assert.NotZero(t, msg.SessionAttr.Flags&SessAttrNodeProtection)
}

// TestParseRSVPTEConfigOneToOneDefault: backup one-to-one sets the flag; an
// absent fast-reroute container leaves FastReroute nil.
func TestParseRSVPTEConfigBackupModes(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{"router-id":"10.0.0.1","tunnel":{` +
			`"a":{"destination":"10.0.0.9","tunnel-id":"1","fast-reroute":{"backup":"one-to-one"}},` +
			`"b":{"destination":"10.0.0.8","tunnel-id":"2"}}}}`,
	}}
	cfg, err := parseConfig(sections)
	require.NoError(t, err)
	require.Len(t, cfg.Tunnels, 2)
	byName := map[string]tunnelConfig{cfg.Tunnels[0].Name: cfg.Tunnels[0], cfg.Tunnels[1].Name: cfg.Tunnels[1]}
	require.NotNil(t, byName["a"].FastReroute)
	assert.True(t, byName["a"].FastReroute.OneToOne, "backup one-to-one sets the flag")
	assert.Equal(t, uint8(16), byName["a"].FastReroute.HopLimit, "hop-limit defaults to 16")
	assert.Nil(t, byName["b"].FastReroute, "no fast-reroute container -> nil")
}

// TestParseRSVPTEConfigBypass parses a configured facility-backup bypass LSP.
func TestParseRSVPTEConfigBypass(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{"router-id":"10.0.0.2","bypass":{"bp1":{"merge-point":"10.0.0.3","node-protection":"true",` +
			`"explicit-route":{"1":{"address":"10.0.1.3/32"}}}}}}`,
	}}
	cfg, err := parseConfig(sections)
	require.NoError(t, err)
	require.Len(t, cfg.Bypasses, 1)
	bp := cfg.Bypasses[0]
	assert.Equal(t, "bp1", bp.Name)
	assert.Equal(t, "10.0.0.3", bp.MergePoint.String())
	assert.True(t, bp.NodeProtection)
	require.Len(t, bp.ERO, 1)
	assert.Equal(t, "10.0.1.3/32", bp.ERO[0].Address.String())
}

// TestParseRSVPTEConfigBypassInvalidMergePoint rejects a malformed merge-point.
func TestParseRSVPTEConfigBypassInvalidMergePoint(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{"bypass":{"bp1":{"merge-point":"not-an-ip"}}}}`,
	}}
	_, err := parseConfig(sections)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid merge-point")
}

// TestParseRSVPTEConfigInvalidDestination rejects a malformed tunnel destination
// (fail closed, consistent with merge-point).
func TestParseRSVPTEConfigInvalidDestination(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{"tunnel":{"t1":{"destination":"not-an-ip","tunnel-id":"1"}}}}`,
	}}
	_, err := parseConfig(sections)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid destination")
}

// TestParseRSVPTEConfigInvalidERO rejects a malformed explicit-route address.
func TestParseRSVPTEConfigInvalidERO(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{"tunnel":{"t1":{"destination":"10.0.0.9","tunnel-id":"1",` +
			`"explicit-route":{"1":{"address":"bogus"}}}}}}`,
	}}
	_, err := parseConfig(sections)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "explicit-route")
}

// TestParseRSVPTEConfigReservedTunnelID rejects a tunnel-id in the reserved
// fast-reroute bypass range (>= 0xF000).
func TestParseRSVPTEConfigReservedTunnelID(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{"tunnel":{"t1":{"destination":"10.0.0.9","tunnel-id":"61440"}}}}`, // 0xF000
	}}
	_, err := parseConfig(sections)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")
}

// TestParseRSVPTEConfigBypassNoRouterID: a bypass with no router-id must not
// panic in validateBypasses (bypassKey derives a tunnel-id from the router-id,
// whose As4() panics on the zero Addr). The engine stays idle without a router-id.
func TestParseRSVPTEConfigBypassNoRouterID(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{"bypass":{"b1":{"merge-point":"10.0.0.1"}}}}`,
	}}
	cfg, err := parseConfig(sections) // must not panic
	require.NoError(t, err)
	assert.False(t, cfg.RouterID.IsValid())
	require.Len(t, cfg.Bypasses, 1)
}

// TestParseRSVPTEConfigIPv6RouterID rejects a non-IPv4 router-id (rsvp-te here is
// IPv4-only; addrToUint32's As4() would otherwise panic).
func TestParseRSVPTEConfigIPv6RouterID(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{"router-id":"2001:db8::1"}}`,
	}}
	_, err := parseConfig(sections)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "IPv4")
}

// TestParseRSVPTEConfigTunnelIDOutOfRange rejects a tunnel-id outside 0-65535
// before the lossy uint16 cast can wrap it.
func TestParseRSVPTEConfigTunnelIDOutOfRange(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "rsvp-te",
		Data: `{"rsvp-te":{"tunnel":{"t1":{"destination":"10.0.0.9","tunnel-id":"65536"}}}}`,
	}}
	_, err := parseConfig(sections)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}
