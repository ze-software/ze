// Design: docs/guide/route-policy.md -- the community members an operator names in a peer's filter chain
// Related: community_filter.go -- the filters, the insertion and the evaluator under test
// Related: chain_member.go -- the verify refusal under test

package filter_community

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestCommunityIsNoEngineStep proves D-6 (spec-filter-wire-6-stage-filters):
// no community step runs in the engine's stage list, so nothing of this
// plugin runs before the per-peer chain. That ordering was the regression an
// LLGR peer's route met: a community strip of LLGR_STALE ran at the Policy
// stage, before bgp-gr's llgr-ingress member, so the route was never marked
// stale (RFC 9494 Section 4.3: "Treat the route as the least preferred in
// route selection"), and a tag of LLGR_STALE marked a route that stayed
// fresh before. The members now run inside the chain, where the config
// loader places every inserted member, llgr-ingress included, ahead of the
// operator's. Method: read the stage lists this package's init fills.
func TestCommunityIsNoEngineStep(t *testing.T) {
	for _, f := range filterapi.IngressOrdered() {
		assert.NotContains(t, f.Name, communityPluginName, "an ingress engine step of this plugin runs before the chain")
	}
	for _, f := range filterapi.EgressOrdered() {
		assert.NotContains(t, f.Name, communityPluginName, "an egress engine step of this plugin runs before the chain")
	}
}

// TestCommunityMembersAreDeclared proves the declaration half of AC-3, AC-9
// and D-3: the operator's three members are declared and inserted nowhere,
// and the one inserted member is the RFC 7999 Section 3.1 agreement check,
// on export, on every eBGP session that is not a route-server client,
// required with its reason. Method: read the functions both registrations
// use.
func TestCommunityMembersAreDeclared(t *testing.T) {
	directions := map[string]sdk.FilterDirection{}
	for _, decl := range communityFilterDecls() {
		directions[decl.Name] = decl.Direction
		assert.Equal(t, sdk.OnErrorReject, decl.OnError, decl.Name)
	}
	assert.Equal(t, map[string]sdk.FilterDirection{
		ingressFilterName: sdk.FilterImport, relationFilterName: sdk.FilterImport,
		egressFilterName: sdk.FilterExport, blackholeFilterName: sdk.FilterExport,
	}, directions)

	insertions := communityInsertions()
	require.Len(t, insertions, 1)
	ins := insertions[0]
	assert.Equal(t, "bgp-filter-community:blackhole-agreement", ins.Filter)
	assert.Equal(t, rpc.FilterExport, ins.Direction)
	assert.Equal(t, []rpc.FilterSessionFact{
		{Fact: "ibgp", Values: []string{"false"}}, {Fact: "rs-client", Values: []string{"false"}},
	}, ins.Session)
	assert.True(t, ins.Required)
	assert.Contains(t, ins.Reason, "MUST be agreed upon by the two networks before advertising it.")
}

// TestCommunitySettingsNeedChainMember proves AC-11: a peer whose community
// settings no chain member runs is refused, naming the peer, the setting
// and the member to add; the member named at the bgp level discharges it,
// and a deactivated one does not, since the plugin never sees it. Method: parse the config verify and apply both parse.
func TestCommunitySettingsNeedChainMember(t *testing.T) {
	cfg := func(peerFilter, bgpFilter map[string]any) map[string]any {
		peer := map[string]any{"connection": map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}}, "filter": peerFilter}
		bgp := map[string]any{
			"community": map[string]any{"standard": map[string]any{"mark": map[string]any{"value": []any{"65001:1"}}}},
			"peer":      map[string]any{"edge": peer},
		}
		if bgpFilter != nil {
			bgp["filter"] = bgpFilter
		}
		return bgp
	}
	ingress := map[string]any{"community": map[string]any{"tag": []any{"mark"}}}
	egress := map[string]any{"community": map[string]any{"strip": []any{"mark"}}}
	chain := func(direction, member string) map[string]any {
		return map[string]any{direction: map[string]any{"filters": []any{member}}}
	}

	_, err := parseCommunityConfig(cfg(map[string]any{"ingress": ingress}, nil))
	require.ErrorIs(t, err, errCommunityMemberMissing)
	assert.Contains(t, err.Error(), "peer edge sets filter ingress community")
	assert.Contains(t, err.Error(), "filter { import { filters [ bgp-filter-community:ingress ]; } }")

	_, err = parseCommunityConfig(cfg(map[string]any{"egress": egress}, nil))
	require.ErrorIs(t, err, errCommunityMemberMissing)
	assert.Contains(t, err.Error(), "filter { export { filters [ bgp-filter-community:egress ]; } }")

	_, err = parseCommunityConfig(cfg(map[string]any{"ingress": ingress}, chain("import", "bgp-filter-community:ingress")))
	require.NoError(t, err, "a bgp-level member runs for every peer")
	_, err = parseCommunityConfig(cfg(map[string]any{"egress": egress,
		"export": map[string]any{"filters": []any{}}}, nil))
	require.ErrorIs(t, err, errCommunityMemberMissing,
		"a deactivated member is absent from the plugin's map (config Tree.toMap keeps active members only)")
	scrubTag := map[string]any{"community": map[string]any{"scrub-own-ga": "true", "relation-tag": "true"}}
	_, err = parseCommunityConfig(cfg(map[string]any{"ingress": scrubTag, "import": map[string]any{"filters": []any{
		"bgp-filter-community:relation", "bgp-filter-community:ingress"}}}, nil))
	require.ErrorIs(t, err, errCommunityMemberOrder, "a scrub after the relation tag deletes the tag")
	assert.Contains(t, err.Error(), "filter { import { filters [ bgp-filter-community:ingress bgp-filter-community:relation ]; } }")
	_, err = parseCommunityConfig(cfg(map[string]any{"ingress": scrubTag, "import": map[string]any{"filters": []any{
		"bgp-filter-community:ingress", "bgp-filter-community:relation"}}}, nil))
	require.NoError(t, err)
	_, err = parseCommunityConfig(cfg(map[string]any{"ingress": ingress}, chain("import", "bgp-filter-community:egress")))
	require.ErrorIs(t, err, errCommunityMemberMissing, "the egress member does not run ingress settings")
}

// TestCommunitySetsCarryWhatTheBodyChanged proves the evaluator's answer is
// the decision's rewrite: a value the body lost is one remove, a value it
// gained one add, in the text form the engine's set reads back to the same
// octets. Method: diff two bodies, then parse each set value back.
func TestCommunitySetsCarryWhatTheBodyChanged(t *testing.T) {
	llgrStale := uint32(0xFFFF0006)
	original := buildPayload(append(buildOriginAttr(), buildCommunityAttr(llgrStale, 0xFDE800C8)...))
	modified := buildPayload(append(buildOriginAttr(), buildCommunityAttr(0xFDE800C8, 0xFDE90001)...))
	out, err := rewriteVerdict(original, modified)
	require.NoError(t, err)
	assert.Equal(t, sdk.FilterModify, out.Action)
	assert.Equal(t, []rpc.FilterSet{
		{Field: policy.FieldCommunity.String(), Action: rpc.FilterSetActionRemove, Value: []byte(`"65535:6"`)},
		{Field: policy.FieldCommunity.String(), Action: rpc.FilterSetActionAdd, Value: []byte(`"65001:1"`)},
	}, out.Sets)

	unchanged, err := rewriteVerdict(original, nil)
	require.NoError(t, err)
	assert.Equal(t, sdk.FilterAccept, unchanged.Action)

	large, err := communitySet(attribute.AttrLargeCommunity, rpc.FilterSetActionAdd,
		[]byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 3, 0, 0, 0, 2})
	require.NoError(t, err)
	assert.Equal(t, `"65001:3:2"`, string(large.Value))

	for _, wire := range [][]byte{
		{0x00, 0x02, 0xfd, 0xe9, 0, 0, 0, 7}, // RFC 4360 two-octet AS route target
		{0x43, 0x0b, 1, 2, 3, 4, 5, 6},       // a type the named vocabulary does not name
	} {
		set, err := communitySet(attribute.AttrExtCommunity, rpc.FilterSetActionRemove, wire)
		require.NoError(t, err)
		text := string(set.Value[1 : len(set.Value)-1])
		back, err := attribute.ParseSingleExtCommunity(text)
		require.NoError(t, err, "the engine cannot read back %q", text)
		assert.Equal(t, wire, back[:], text)
	}
}

// TestCommunityFiltersPassConformance proves AC-18 for the four members:
// the fast path and the forked call answer alike. Method: the SDK's
// conformance check over one route per member.
func TestCommunityFiltersPassConformance(t *testing.T) {
	withCleanConfig(t)
	bgpCfg := map[string]any{
		"community": map[string]any{"standard": map[string]any{"mark": map[string]any{"value": []any{"65001:1"}}}},
		"filter": map[string]any{
			"import": map[string]any{"filters": []any{"bgp-filter-community:ingress", "bgp-filter-community:relation"}},
			"export": map[string]any{"filters": []any{"bgp-filter-community:egress"}},
		},
		"peer": map[string]any{"edge": map[string]any{
			"connection": map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}},
			"filter": map[string]any{
				"ingress": map[string]any{"community": map[string]any{"tag": []any{"mark"}}},
				"egress":  map[string]any{"community": map[string]any{"tag": []any{"mark"}}},
			},
		}},
	}
	require.NoError(t, configureCommunityFilter(bgpCfg))
	body := buildPayload(append(buildOriginAttr(), buildCommunityAttr(uint32(attribute.CommunityBlackhole))...))
	route := func(filter, fields string) sdk.ConformanceRoute {
		return sdk.ConformanceRoute{Input: rpc.FilterUpdateInput{Filter: filter, Raw: body, Fields: []byte(fields)}}
	}
	routes := []sdk.ConformanceRoute{
		route(ingressFilterName, `{"peer-name":"edge","local-as":65000,"peer-as":65001}`),
		route(relationFilterName, `{"peer-name":"edge","local-as":65000,"src-peer-role":"customer"}`),
		route(egressFilterName, `{"peer-name":"edge","peer-address":"10.0.0.1","ibgp":true}`),
		route(blackholeFilterName, `{"peer-name":"edge","peer-address":"10.0.0.1"}`),
	}
	if !sdk.CheckFilterConformance(t, communityPluginName, evaluateCommunityFilter, routes) {
		t.Fatal("the community evaluator breaks the fast-path contract")
	}
	unknown := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: "nope"})
	_, err := evaluateCommunityFilter(&unknown)
	assert.ErrorIs(t, err, errNoSuchCommunityFilter)
}
