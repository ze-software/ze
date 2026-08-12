package filter_community

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

const (
	relLocalAS  uint32 = 65000
	relPeerAS   uint32 = 64511
	relPeerName        = "peer1"
)

// relationOn builds a filterConfig with the relation tag enabled.
func relationOn(function *uint32) filterConfig {
	return filterConfig{relationTag: new(true), relationFunction: function}
}

// withPeerConfig installs one peer's config in the plugin globals for the
// length of the test, then restores what was there. The registered filter
// reads the globals, so a wiring test has to go through them.
func withPeerConfig(t *testing.T, fc filterConfig) {
	t.Helper()
	mu.Lock()
	prev := peerConfigs
	peerConfigs = map[string]filterConfig{relPeerName: fc}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		peerConfigs = prev
		mu.Unlock()
	})
}

func relPeerInfo(peerAS uint32) filterapi.PeerFilterInfo {
	return filterapi.PeerFilterInfo{
		Name:    relPeerName,
		Address: netip.MustParseAddr("192.0.2.1"),
		PeerAS:  peerAS,
		LocalAS: relLocalAS,
	}
}

// TestRelationTagWiring is the Wiring Test row "peer config enables
// relation tagging -> ingress relation filter". It drives the REGISTERED
// filter func, not the helper underneath it. So it fails if the config leaf
// never reaches the filter or the filter never reads the meta key.
//
// VALIDATES: AC-1 (spec-bcp194-1-communities)
// PREVENTS: the feature existing as a function nothing calls, which is the
// failure mode this spec was written to correct twice over.
func TestRelationTagWiring(t *testing.T) {
	withPeerConfig(t, relationOn(nil))
	payload := buildPayload(buildOriginAttr())

	accept, modified := relationIngressFilter(
		relPeerInfo(relPeerAS), payload, map[string]any{"src-peer-role": roleProvider})

	assert.True(t, accept, "the relation tag never rejects a route")
	require.NotNil(t, modified, "a provider's route must gain the relation community")
	assert.Equal(t, [][3]uint32{{relLocalAS, 3, 4}}, extractLargeCommunities(modified))
}

// TestRelationTagPerRole covers AC-1 through AC-4 end to end through the
// filter.
//
// VALIDATES: AC-1, AC-2, AC-3, AC-4
// PREVENTS: a route-server session gaining an attribute, which RFC 7947 forbids,
// and an unresolved peer role producing a guessed relation.
func TestRelationTagPerRole(t *testing.T) {
	tests := []struct {
		name string
		role string
		want [][3]uint32
	}{
		{"AC-1 provider", roleProvider, [][3]uint32{{relLocalAS, 3, 4}}},
		{"AC-2 customer", roleCustomer, [][3]uint32{{relLocalAS, 3, 2}}},
		{"AC-3 peer", rolePeer, [][3]uint32{{relLocalAS, 3, 3}}},
		{"AC-4 rs writes nothing", roleRS, nil},
		{"AC-4 rs-client writes nothing", roleRSClient, nil},
		{"unresolved role writes nothing", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withPeerConfig(t, relationOn(nil))
			payload := buildPayload(buildOriginAttr())

			_, modified := relationIngressFilter(
				relPeerInfo(relPeerAS), payload, map[string]any{"src-peer-role": tt.role})

			if tt.want == nil {
				assert.Nil(t, modified, "the forwarded bytes stay identical")
				return
			}
			require.NotNil(t, modified)
			assert.Equal(t, tt.want, extractLargeCommunities(modified))
		})
	}
}

// TestRelationTagForgedValueRemoved covers AC-5: a peer that sends the
// local AS's own relation community has it removed. The stored route
// carries exactly one Function 3 value -- the one Ze derived.
//
// The de-forge is not gated on Section 11 scrub. Making it a second opt-in
// would let an operator enable the tag and still store a peer's claim about
// the relationship, which is the forgery this AC names.
//
// VALIDATES: AC-5
// PREVENTS: a neighbor choosing what this AS believes about its own relationship
// to a route, and a replayed route accumulating two relation values.
func TestRelationTagForgedValueRemoved(t *testing.T) {
	withPeerConfig(t, relationOn(nil))
	payload := buildPayload(append(buildOriginAttr(), buildLargeCommunityAttr(
		[3]uint32{relLocalAS, 3, 2},  // forged: "you learned me from a customer"
		[3]uint32{relLocalAS, 64, 1}, // an unrelated own-GA value, not this filter's business
		[3]uint32{64512, 3, 2},       // another AS's relation tag, never ours to touch
	)...))

	_, modified := relationIngressFilter(
		relPeerInfo(relPeerAS), payload, map[string]any{"src-peer-role": roleProvider})
	require.NotNil(t, modified)

	assert.Equal(t, [][3]uint32{
		{relLocalAS, 64, 1},
		{64512, 3, 2},
		{relLocalAS, 3, 4},
	}, extractLargeCommunities(modified))
}

// TestRelationTagIsIdempotent verifies a second pass over an already-tagged
// route leaves exactly one relation value, so a replay cannot accumulate
// them.
//
// VALIDATES: AC-5, the "exactly one Function 3 value" half.
func TestRelationTagIsIdempotent(t *testing.T) {
	withPeerConfig(t, relationOn(nil))
	meta := map[string]any{"src-peer-role": roleProvider}

	_, first := relationIngressFilter(relPeerInfo(relPeerAS), buildPayload(buildOriginAttr()), meta)
	require.NotNil(t, first)
	_, second := relationIngressFilter(relPeerInfo(relPeerAS), first, meta)

	if second != nil {
		first = second
	}
	assert.Equal(t, [][3]uint32{{relLocalAS, 3, 4}}, extractLargeCommunities(first))
}

// TestRelationTagIBGPWritesNothing covers AC-8's tag half: an iBGP peer
// gets no relation tag. The source is inside the local AS, so there is no
// customer/peer/provider relation for RFC 8195 Section 3.2 to state.
//
// It is driven with a role present in meta, so the assertion is about the
// iBGP gate and not about an unresolved role.
//
// VALIDATES: AC-8
// PREVENTS: an internal route being stamped with a relation it does not have,
// which downstream policy would then act on.
func TestRelationTagIBGPWritesNothing(t *testing.T) {
	withPeerConfig(t, relationOn(nil))

	_, modified := relationIngressFilter(
		relPeerInfo(relLocalAS), // PeerAS == LocalAS: iBGP
		buildPayload(buildOriginAttr()),
		map[string]any{"src-peer-role": roleProvider})

	assert.Nil(t, modified)
}

// TestRelationTagDisabledWritesNothing pins the default. The leaf ships
// false, so an existing config gains no attribute on upgrade.
//
// PREVENTS: a silent wire-visible change for every operator who did not ask for
// this feature.
func TestRelationTagDisabledWritesNothing(t *testing.T) {
	withPeerConfig(t, filterConfig{ingressTag: []string{"unused"}})

	_, modified := relationIngressFilter(
		relPeerInfo(relPeerAS), buildPayload(buildOriginAttr()),
		map[string]any{"src-peer-role": roleProvider})

	assert.Nil(t, modified)
}

// TestRelationFunctionNumberIsConfigurable covers AC-13: a function number
// other than 3 is written, and the shipped default stays 3.
//
// RFC 8195 Section 3.2 says an AS "could assign" the number, so it is a
// local convention. Hard-coding 3 would state that convention on the
// operator's behalf.
//
// VALIDATES: AC-13
// PREVENTS: an operator who already uses function 3 for something else having
// this feature collide with it and no way to move.
func TestRelationFunctionNumberIsConfigurable(t *testing.T) {
	assert.Equal(t, uint32(3), filterConfig{}.relationFunctionNumber(), "the shipped default")

	withPeerConfig(t, relationOn(new(uint32(64))))

	_, modified := relationIngressFilter(
		relPeerInfo(relPeerAS), buildPayload(buildOriginAttr()),
		map[string]any{"src-peer-role": roleCustomer})
	require.NotNil(t, modified)

	assert.Equal(t, [][3]uint32{{relLocalAS, 64, 2}}, extractLargeCommunities(modified))
}

// TestRelationDeForgeFollowsTheConfiguredFunction verifies the de-forge
// tracks the configured function number rather than the constant 3. So
// moving the convention does not leave the forgery door open at the new
// number.
//
// VALIDATES: AC-5 together with AC-13
// PREVENTS: a de-forge hard-coded to 3 while the tag is written at 64.
func TestRelationDeForgeFollowsTheConfiguredFunction(t *testing.T) {
	withPeerConfig(t, relationOn(new(uint32(64))))
	payload := buildPayload(append(buildOriginAttr(), buildLargeCommunityAttr(
		[3]uint32{relLocalAS, 64, 2}, // forged at the CONFIGURED function
	)...))

	_, modified := relationIngressFilter(
		relPeerInfo(relPeerAS), payload, map[string]any{"src-peer-role": roleProvider})
	require.NotNil(t, modified)

	assert.Equal(t, [][3]uint32{{relLocalAS, 64, 4}}, extractLargeCommunities(modified))
}

// TestScrubThenTagOrder proves the two ingress passes run in the order the
// spec requires, by running them exactly as the pipeline does: the
// policy-stage filter first, then the annotation-stage relation filter over
// its output.
//
// The order matters in both directions. Tag before scrub would delete the
// value Ze just derived, because the scrub's keep-list never keeps the
// relation function. Scrub before tag leaves a route carrying exactly the
// derived value.
//
// VALIDATES: AC-5, AC-6, AC-7 in one pass
// PREVENTS: the two features silently canceling each other on a peer with both on.
func TestScrubThenTagOrder(t *testing.T) {
	fc := filterConfig{
		relationTag:    new(true),
		scrubOwnGA:     new(true),
		scrubKeepFuncs: []uint32{64},
	}
	withPeerConfig(t, fc)

	payload := buildPayload(append(buildOriginAttr(), buildLargeCommunityAttr(
		[3]uint32{relLocalAS, 3, 2},  // forged relation tag
		[3]uint32{relLocalAS, 64, 7}, // kept: the operator listed function 64
		[3]uint32{relLocalAS, 99, 7}, // scrubbed: not in the keep-list
		[3]uint32{64512, 99, 7},      // foreign Global Administrator, untouched
	)...))

	scrubbed := applyIngressFilter(payload, nil, fc, relLocalAS, relPeerAS)
	require.NotNil(t, scrubbed)

	_, tagged := relationIngressFilter(
		relPeerInfo(relPeerAS), scrubbed, map[string]any{"src-peer-role": rolePeer})
	require.NotNil(t, tagged)

	assert.Equal(t, [][3]uint32{
		{relLocalAS, 64, 7},
		{64512, 99, 7},
		{relLocalAS, 3, 3},
	}, extractLargeCommunities(tagged))
}

// TestIBGPScrubsNothing covers AC-8's scrub half.
//
// VALIDATES: AC-8
// PREVENTS: Section 11 scrub deleting this network's own internal signaling,
// which travels on iBGP sessions carrying exactly the own-AS values the
// scrub selects.
func TestIBGPScrubsNothing(t *testing.T) {
	fc := filterConfig{scrubOwnGA: new(true)}
	payload := buildPayload(append(buildOriginAttr(), buildLargeCommunityAttr(
		[3]uint32{relLocalAS, 99, 7},
	)...))

	assert.Nil(t, applyIngressFilter(payload, nil, fc, relLocalAS, relLocalAS),
		"peer AS equals local AS, so the session is iBGP and nothing is scrubbed")
	require.NotNil(t, applyIngressFilter(payload, nil, fc, relLocalAS, relPeerAS),
		"the same route on an eBGP session IS scrubbed, so the gate is the AS comparison")
}

// TestRelationFilterRegisteredAfterRole pins the ordering DECLARATION
// rather than its effect. filterapi sorts by stage, then priority, then
// name. The relation filter must sort after bgp-role so the meta key it
// reads is written before it runs.
//
// The names are spelled here rather than imported because the role plugin
// may not be linked into a given binary. The assertion is over what
// filter_community itself declares, plus the relative order when both are
// present.
//
// PREVENTS: a stage or priority edit that leaves the relation filter reading a
// key nothing has written yet, which would silently stop every tag being
// written.
func TestRelationFilterRegisteredAfterRole(t *testing.T) {
	var relation, policy *filterapi.Filter
	for _, f := range filterapi.IngressOrdered() {
		switch f.Name {
		case "bgp-filter-community-relation":
			relation = &f
		case "bgp-filter-community":
			policy = &f
		}
	}
	require.NotNil(t, relation, "the relation filter must be registered")
	require.NotNil(t, policy, "the policy-stage filter must still be registered")

	assert.Equal(t, filterapi.FilterStageAnnotation, relation.Stage)
	assert.Equal(t, filterapi.FilterStagePolicy, policy.Stage)
	assert.True(t, filterapi.LessOrder(policy.Name, policy.Stage, policy.Priority,
		relation.Name, relation.Stage, relation.Priority),
		"the scrub must run before the tag")

	// bgp-role registers at the same stage with priority 0. Only assert the
	// relative order when that plugin is linked.
	for _, f := range filterapi.IngressOrdered() {
		if f.Name != "bgp-role" {
			continue
		}
		assert.True(t, filterapi.LessOrder(f.Name, f.Stage, f.Priority,
			relation.Name, relation.Stage, relation.Priority),
			"bgp-role writes src-peer-role, so it must sort before the relation filter")
	}
}
