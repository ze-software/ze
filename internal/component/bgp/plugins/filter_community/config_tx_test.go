package filter_community

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// tagSection is a bgp delivery defining community "t" as value and tagging
// peer p's egress with it.
func tagSection(value string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{
		"community":{"standard":{"t":{"value":["` + value + `"]}}},
		"peer":{"p":{"connection":{"remote":{"ip":"192.0.2.1"}},
			"filter":{"egress":{"community":{"tag":["t"]}}}}}}}`}}
}

// installedTagWire is the wire value of community "t" a filter call reads,
// and whether peer p carries a filter config.
func installedTagWire(t *testing.T) (wire []byte, peerTagged bool) {
	t.Helper()
	mu.RLock()
	defer mu.RUnlock()
	_, peerTagged = peerConfigs["p"]
	def, ok := definitions["t"]
	if !ok || len(def.wireValues) != 1 {
		return nil, peerTagged
	}
	return def.wireValues[0], peerTagged
}

// VALIDATES: spec-plugin-reload-applies-or-refuses AC-1 to AC-4 for
// bgp-filter-community: a reload's verify installs nothing, apply installs the
// edited definition with the peer configs and agreements of the same config,
// rollback restores the boot config, and a candidate naming an undefined
// community is refused at verify and leaves the running one.
// PREVENTS: the silent reload: only the blackhole agreements took part in the
// transaction, so a tag edit answered OK and kept the startup definitions.
// Not parallel: the installed config is package state.
func TestConfigTxAppliesReloadedCommunities(t *testing.T) {
	withCleanConfig(t)
	boot, err := parseStandardWire("65001:1")
	require.NoError(t, err)
	edited, err := parseStandardWire("65001:2")
	require.NoError(t, err)

	tx := newCommunityConfigTx()
	require.NoError(t, tx.Configure(tagSection("65001:1")))

	require.NoError(t, tx.Verify(tagSection("65001:2")))
	wire, _ := installedTagWire(t)
	assert.Equal(t, boot, wire, "verify installed the candidate")

	require.NoError(t, tx.Apply())
	wire, tagged := installedTagWire(t)
	assert.Equal(t, edited, wire, "apply did not install the edited definition")
	assert.True(t, tagged, "apply dropped peer p's filter config")

	require.NoError(t, tx.Rollback())
	wire, _ = installedTagWire(t)
	assert.Equal(t, boot, wire, "rollback did not restore the boot definition")

	undefined := []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{"peer":{"p":{
		"connection":{"remote":{"ip":"192.0.2.1"}},"filter":{"egress":{"community":{"tag":["missing"]}}}}}}}`}}
	require.Error(t, tx.Verify(undefined), "a tag naming an undefined community is refused at verify")
	wire, _ = installedTagWire(t)
	assert.Equal(t, boot, wire, "a refused verify changed the running config")
}

// VALIDATES: AC-2 for bgp-filter-community: a reload that removes every
// definition and filter returns the plugin to its unconfigured behavior.
// Not parallel: the installed config is package state.
func TestConfigTxRemovedCommunitiesUnconfigure(t *testing.T) {
	withCleanConfig(t)
	tx := newCommunityConfigTx()
	require.NoError(t, tx.Configure(tagSection("65001:1")))

	require.NoError(t, tx.Verify([]sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{}}`}}))
	require.NoError(t, tx.Apply())
	wire, tagged := installedTagWire(t)
	assert.Nil(t, wire, "the removed definition is still installed")
	assert.False(t, tagged, "the removed peer filter is still installed")
}
