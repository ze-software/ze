package filter_community

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// rfc7999Ingress parses a peer config whose ingress community block holds the
// given blackhole-propagation leaf (nil: no leaf), then runs one received
// UPDATE through applyIngressFilter, the ingress entry point for every route.
func rfc7999Ingress(t *testing.T, leaf any, payload []byte) []byte {
	t.Helper()
	block := map[string]any{}
	if leaf != nil {
		block["blackhole-propagation"] = leaf
	}
	fc := parseFilterConfig(ingressCommunityCfg(block))
	return applyIngressFilter(payload, nil, fc, blackholeLocalAS, 64511)
}

// RFC requirement: RFC7999-3.2-1 positive -- a received announcement tagged
// with BLACKHOLE gains a propagation community on ingress: with the peer's
// guard set, [BLACKHOLE 64496:1] leaves applyIngressFilter as
// [BLACKHOLE 64496:1 NO_EXPORT], and as [BLACKHOLE 64496:1 NO_ADVERTISE]
// under the other token.
//
// VALIDATES: RFC 7999 Section 3.2 community added to a BLACKHOLE-tagged route.
// PREVENTS: a BLACKHOLE route stored and re-advertised with nothing limiting its scope.
func TestRFC7999ReceivedBlackholeRouteGainsThePropagationCommunity(t *testing.T) {
	other := uint32(64496)<<16 | 1
	payload := blackholePayload(uint32(attribute.CommunityBlackhole), other)

	got := rfc7999Ingress(t, "no-export", payload)
	require.NotNil(t, got, "no-export guard: the BLACKHOLE route must be rewritten")
	assert.Equal(t, []uint32{uint32(attribute.CommunityBlackhole), other, uint32(attribute.CommunityNoExport)}, extractCommunities(got))

	got = rfc7999Ingress(t, "no-advertise", payload)
	require.NotNil(t, got, "no-advertise guard: the BLACKHOLE route must be rewritten")
	assert.Equal(t, []uint32{uint32(attribute.CommunityBlackhole), other, uint32(attribute.CommunityNoAdvertise)}, extractCommunities(got))
}

// RFC requirement: RFC7999-3.2-1 negative -- the community is added only to an
// announcement tagged with BLACKHOLE: with the guard set, a route with no
// COMMUNITY attribute, a route carrying 65535:665 and a route carrying
// 64496:1 each leave applyIngressFilter unchanged (nil), with no NO_EXPORT.
//
// VALIDATES: the trigger is the BLACKHOLE community and nothing else.
// PREVENTS: the guard limiting the scope of every route once it is enabled.
func TestRFC7999RouteWithoutBlackholeGainsNoPropagationCommunity(t *testing.T) {
	payloads := map[string][]byte{
		"no COMMUNITY attribute": blackholePayload(),
		"65535:665":              blackholePayload(uint32(attribute.CommunityBlackhole) - 1),
		"64496:1":                blackholePayload(uint32(64496)<<16 | 1),
	}
	for name, payload := range payloads {
		assert.Nil(t, rfc7999Ingress(t, "no-export", payload), "%s: route must pass unchanged", name)
	}
}

// RFC requirement: RFC7999-3.2-2 positive -- the propagation community is the
// one the operator's routing policy names: the blackhole-propagation leaf,
// parsed from the peer config in its string form, puts exactly NO_EXPORT or
// exactly NO_ADVERTISE on a received BLACKHOLE route.
//
// VALIDATES: RFC 7999 Section 3.2 community chosen by operator configuration.
// PREVENTS: the leaf being parsed but not reaching the ingress rewrite.
func TestRFC7999PropagationCommunityFollowsTheOperatorLeaf(t *testing.T) {
	payload := blackholePayload(uint32(attribute.CommunityBlackhole))
	cases := map[string]attribute.Community{
		"no-export":    attribute.CommunityNoExport,
		"no-advertise": attribute.CommunityNoAdvertise,
	}
	for leaf, want := range cases {
		got := rfc7999Ingress(t, leaf, payload)
		require.NotNil(t, got, "leaf %q", leaf)
		assert.Equal(t, []uint32{uint32(attribute.CommunityBlackhole), uint32(want)}, extractCommunities(got), "leaf %q", leaf)
	}
}

// RFC requirement: RFC7999-3.2-2 negative -- no propagation community is forced
// against the operator's policy: under no-advertise NO_EXPORT is absent, under
// no-export NO_ADVERTISE is absent, and with the leaf set to none or left
// unset a BLACKHOLE route passes unchanged, so no fixed default is applied.
//
// VALIDATES: the operator's choice, including the choice of none, is honored.
// PREVENTS: a hard-coded community overriding or adding to the configured one.
func TestRFC7999PropagationCommunityIsNeverForced(t *testing.T) {
	payload := blackholePayload(uint32(attribute.CommunityBlackhole))

	got := rfc7999Ingress(t, "no-advertise", payload)
	require.NotNil(t, got)
	assert.NotContains(t, extractCommunities(got), uint32(attribute.CommunityNoExport))

	got = rfc7999Ingress(t, "no-export", payload)
	require.NotNil(t, got)
	assert.NotContains(t, extractCommunities(got), uint32(attribute.CommunityNoAdvertise))

	assert.Nil(t, rfc7999Ingress(t, "none", payload), "leaf none: route must pass unchanged")
	assert.Nil(t, rfc7999Ingress(t, nil, payload), "no leaf: route must pass unchanged")
}
