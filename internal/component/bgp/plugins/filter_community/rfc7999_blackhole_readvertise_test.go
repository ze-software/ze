// VALIDATES: RFC 7999 Section 3.1 on re-advertisement: a BLACKHOLE-tagged
// route is withheld from a bilateral eBGP destination whose blackhole
// container does not agree to the community, and reaches every other
// destination; the agreement is read from a cache built at config time and
// swapped by the reload transaction.
// PREVENTS: a received BLACKHOLE route re-advertised to a network that never
// agreed to act on it, which the origination gate alone did not stop.

package filter_community

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// readvertiseConfig is the bgp subtree every test here starts from. B agrees
// to BLACKHOLE, C agreed only to 65001:666, F states nothing but its blackhole
// container (so it has no filterConfig), and the dynamic group named like an
// address agrees to BLACKHOLE for every member built from it.
const readvertiseConfig = `{"bgp":{
	"peer":{
		"b":{"connection":{"remote":{"ip":"192.0.2.2"}},"blackhole":{"communities":["blackhole"]}},
		"c":{"connection":{"remote":{"ip":"192.0.2.3"}},"blackhole":{"communities":["65001:666"]}},
		"f":{"connection":{"remote":{"ip":"192.0.2.6"}},"blackhole":{"communities":["65535:666"]}}
	},
	"group":{"10.0.0.1":{
		"connection":{"remote":{"ip":"dynamic","range":["198.51.100.0/24"]}},
		"blackhole":{"communities":["blackhole"]}}}}}`

// configureReadvertise installs readvertiseConfig through the startup path.
func configureReadvertise(t *testing.T) {
	t.Helper()
	withCleanConfig(t)
	bgpCfg, ok := configjson.ParseBGPSubtree(readvertiseConfig)
	require.True(t, ok)
	require.NoError(t, configureCommunityFilter(bgpCfg))
}

// ebgpDest is a bilateral eBGP destination at addr.
func ebgpDest(addr string) filterapi.PeerFilterInfo {
	return filterapi.PeerFilterInfo{Address: netip.MustParseAddr(addr)}
}

// readvertised runs one route through the registered egress filter.
func readvertised(dest filterapi.PeerFilterInfo, payload []byte) bool {
	return egressFilter(filterapi.PeerFilterInfo{}, dest, payload, nil, &filterapi.ModAccumulator{})
}

// RFC requirement: RFC7999-3.1-2 positive -- a BLACKHOLE-tagged route is not re-advertised to an eBGP destination that did not agree to the BLACKHOLE community: egressFilter refuses it for a peer whose container lists another value, for a peer with no container, and before any config is installed.
func TestRFC7999ReadvertiseWithheldWithoutAgreement(t *testing.T) {
	configureReadvertise(t)
	tagged := blackholePayload(uint32(attribute.CommunityBlackhole))

	assert.False(t, readvertised(ebgpDest("192.0.2.3"), tagged),
		"C agreed to 65001:666 only: the well-known BLACKHOLE is not agreed")
	assert.False(t, readvertised(ebgpDest("192.0.2.9"), tagged),
		"a peer with no blackhole container agreed to nothing")
	assert.False(t, readvertised(ebgpDest("10.0.0.1"), tagged),
		"a group named 10.0.0.1 does not answer for the peer AT 10.0.0.1")

	mu.Lock()
	agreements = nil
	mu.Unlock()
	assert.False(t, readvertised(ebgpDest("192.0.2.2"), tagged),
		"no config installed: a miss means not agreed")
}

// RFC requirement: RFC7999-3.1-2 negative -- the gate binds only an unagreed bilateral eBGP destination and only a tagged route: an agreed peer, a peer whose only statement is a blackhole container, a dynamic-group member, an iBGP destination, a route-server client, and an untagged route all pass.
func TestRFC7999ReadvertiseReachesAgreedAndExemptDestinations(t *testing.T) {
	configureReadvertise(t)
	tagged := blackholePayload(uint32(attribute.CommunityBlackhole))

	assert.True(t, readvertised(ebgpDest("192.0.2.2"), tagged), "B agreed")
	assert.True(t, readvertised(ebgpDest("192.0.2.6"), tagged),
		"F's only statement is a blackhole container, so it has no filterConfig and still agreed")
	member := ebgpDest("198.51.100.7")
	member.GroupName = "10.0.0.1"
	assert.True(t, readvertised(member, tagged), "a dynamic member inherits its group's agreement")

	// An RFC 7705 migration session reaches the filter as IsIBGP true through
	// PeerSettings.IsIBGP, the same as any internal session.
	internal := ebgpDest("192.0.2.9")
	internal.IsIBGP = true
	assert.True(t, readvertised(internal, tagged), "iBGP: one network, not a bilateral relationship")

	rsClient := ebgpDest("192.0.2.9")
	rsClient.RSClient = true
	assert.True(t, readvertised(rsClient, tagged), "route-server client: the multilateral case")

	assert.True(t, readvertised(ebgpDest("192.0.2.3"), blackholePayload(uint32(64496)<<16|1)),
		"an untagged route is not gated")
}

// The egress check reads the cache and never parses config per UPDATE: an
// agreement installed straight into the cache, with no config document behind
// it, decides the route.
func TestRFC7999ReadvertiseReadsTheCache(t *testing.T) {
	configureReadvertise(t)
	tagged := blackholePayload(uint32(attribute.CommunityBlackhole))
	require.False(t, readvertised(ebgpDest("192.0.2.3"), tagged))

	set, err := parseBlackholeAgreements(mustBGP(t, `{"bgp":{"peer":{"c":{
		"connection":{"remote":{"ip":"192.0.2.3"}},"blackhole":{"communities":["blackhole"]}}}}}`))
	require.NoError(t, err)
	mu.Lock()
	agreements = set
	mu.Unlock()
	assert.True(t, readvertised(ebgpDest("192.0.2.3"), tagged), "the decision follows the cache")
}

// The gate runs once per route per destination, so it allocates nothing on
// any path: an agreed peer found by address, one found through its group, a
// miss, and an untagged route.
func TestRFC7999ReadvertiseGateAllocatesNothing(t *testing.T) {
	configureReadvertise(t)
	tagged := blackholePayload(uint32(attribute.CommunityBlackhole))
	untagged := blackholePayload(uint32(64496)<<16 | 1)
	member := ebgpDest("198.51.100.7")
	member.GroupName = "10.0.0.1"
	cases := []struct {
		name    string
		dest    filterapi.PeerFilterInfo
		payload []byte
	}{
		{"agreed-by-address", ebgpDest("192.0.2.2"), tagged},
		{"agreed-by-group", member, tagged},
		{"not-agreed-miss", ebgpDest("192.0.2.9"), tagged},
		{"untagged", ebgpDest("192.0.2.9"), untagged},
	}
	set := agreements
	for _, tc := range cases {
		allocs := testing.AllocsPerRun(100, func() {
			blackholeReadvertiseRefused(set, tc.dest, tc.payload)
		})
		assert.Zero(t, allocs, tc.name)
	}
}

// AC-12a: verify stages the candidate, apply swaps it in, rollback restores
// the previous set, and a verify that fails leaves the installed set alone.
func TestRFC7999AgreementReloadTransaction(t *testing.T) {
	withCleanConfig(t)
	tx := newCommunityConfigTx()
	require.NoError(t, tx.Configure([]sdk.ConfigSection{{Root: configRootBGP, Data: readvertiseConfig}}))
	tagged := blackholePayload(uint32(attribute.CommunityBlackhole))
	b, c := ebgpDest("192.0.2.2"), ebgpDest("192.0.2.3")

	// The candidate removes B's agreement and adds one for C.
	candidate := []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{"peer":{
		"b":{"connection":{"remote":{"ip":"192.0.2.2"}}},
		"c":{"connection":{"remote":{"ip":"192.0.2.3"}},"blackhole":{"communities":["blackhole"]}}}}}`}}

	require.NoError(t, tx.Verify(candidate))
	assert.True(t, readvertised(b, tagged), "verify only stages: B keeps its agreement until apply")

	require.NoError(t, tx.Apply())
	assert.False(t, readvertised(b, tagged), "after apply B no longer agreed")
	assert.True(t, readvertised(c, tagged), "after apply C agreed")

	require.NoError(t, tx.Rollback())
	assert.True(t, readvertised(b, tagged), "rollback restores B's agreement")
	assert.False(t, readvertised(c, tagged), "rollback removes C's")

	bad := []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{"peer":{"b":{
		"connection":{"remote":{"ip":"192.0.2.2"}},"blackhole":{"communities":["not-a-community"]}}}}}`}}
	require.Error(t, tx.Verify(bad), "a malformed community is refused at verify")
	require.ErrorIs(t, tx.Apply(), sdk.ErrConfigApplyUnverified, "a failed verify stages nothing to apply")
	assert.True(t, readvertised(b, tagged), "a failed verify leaves the installed set")

	require.NoError(t, tx.Verify([]sdk.ConfigSection{{Root: "interface", Data: `{}`}}))
	require.NoError(t, tx.Apply())
	require.NoError(t, tx.Rollback())
	assert.True(t, readvertised(b, tagged), "a transaction without a bgp root neither swaps nor restores")
}

func mustBGP(t *testing.T, jsonStr string) map[string]any {
	t.Helper()
	bgpCfg, ok := configjson.ParseBGPSubtree(jsonStr)
	require.True(t, ok)
	return bgpCfg
}
