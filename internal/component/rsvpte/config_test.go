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

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
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
