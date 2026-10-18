// Design: docs/architecture/core-design.md -- LLGR egress filter
// RFC: rfc/short/rfc9494.md -- Section 4.3, a received LLGR_STALE route on every rail
// Related: gr_egress.go -- LLGREgressFilter and advertisedStaleLevel, the decision under test
// Related: gr_ingress.go -- llgrStaleFromSource, the predicate the rebuilt-route recovery reuses

package gr

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// railsEBGPDest is a neighbor without LLGR across an EBGP session:
// dest.PeerAS (65001) differs from dest.LocalAS (65000).
var railsEBGPDest = filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001, LocalAS: 65000}

// railsLLGRDest is a neighbor the LLGR Capability was received from.
var railsLLGRDest = filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.3"), PeerAS: 65001, LocalAS: 65000}

// railsLoadedState loads the egress state the way runGRPlugin does once peers
// are known: one neighbor (railsLLGRDest) sent LLGR, railsEBGPDest did not.
func railsLoadedState(t *testing.T) {
	t.Helper()
	setEgressState(newTestEgressState(map[string]*llgrPeerCap{
		railsLLGRDest.Address.String(): {Families: []llgrCapFamily{{LLST: 3600}}},
	}))
	t.Cleanup(func() { setEgressState(nil) })
}

// TestLLGREgressFilter_StateLoadedWithdrawsEBGPNonLLGR verifies the withhold
// decision with the plugin state LOADED, which is the only state a running
// daemon has. Method: a stale level 2 route to an EBGP neighbor absent from a
// non-empty capability map, beside a neighbor present in it.
//
// The nil-state case (TestLLGREgressFilter_NilStateWithdrawsEBGP) never consults
// the map, so a filter whose map lookup answered "received" for every neighbor
// passed every tagged test. This case goes red under that break.
//
// VALIDATES: the withhold decision reads the loaded capability map per destination.
// PREVENTS: a stale route reaching an EBGP neighbor that never sent LLGR.
//
// RFC requirement: RFC9494-4.3-3 positive -- with the egress state loaded and the LLGR Capability
// received from a different neighbor only, a stale level 2 route to an EBGP neighbor that did not send
// it is converted to a withdrawal (mods.SetWithdraw), while the neighbor that sent it gets the route
// with no modification.
func TestLLGREgressFilter_StateLoadedWithdrawsEBGPNonLLGR(t *testing.T) {
	railsLoadedState(t)
	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}

	var withheld filterapi.ModAccumulator
	assert.True(t, LLGREgressFilter(src, railsEBGPDest, nil, map[string]any{"stale": uint8(2)}, &withheld))
	assert.True(t, withheld.IsWithdraw(), "an EBGP neighbor the capability was not received from is sent a withdrawal")

	var kept filterapi.ModAccumulator
	assert.True(t, LLGREgressFilter(src, railsLLGRDest, nil, map[string]any{"stale": uint8(2)}, &kept))
	assert.False(t, kept.IsWithdraw(), "the neighbor that sent LLGR keeps the route")
	assert.Equal(t, 0, kept.Len(), "the neighbor that sent LLGR gets it unmodified")
}

// TestLLGREgressFilter_GRStaleLevelOnePassesThrough verifies AC-19 (Q-2, owner
// 2026-10-14): a route at stale level 1, the conventional GR period, carries no
// LLGR_STALE and is not one Section 4.3 governs, so the RIB rails that replay
// or refresh it to an EBGP neighbor without LLGR deliver it unchanged. Method:
// level 1 and level 2 to the same destination under the loaded state.
//
// VALIDATES: AC-19, the D6 threshold at the LLGR stale level.
// PREVENTS: a GR-period route being withdrawn from a re-established or refreshing neighbor.
//
// RFC requirement: RFC9494-4.3-3 negative -- a route at stale level 1 (conventional GR retention,
// no LLGR_STALE) to an EBGP neighbor the LLGR Capability was not received from is accepted with no
// withdrawal and no modification, while a level 2 route to that neighbor is still withdrawn.
func TestLLGREgressFilter_GRStaleLevelOnePassesThrough(t *testing.T) {
	railsLoadedState(t)
	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}

	var gr filterapi.ModAccumulator
	assert.True(t, LLGREgressFilter(src, railsEBGPDest, nil, map[string]any{"stale": uint8(1)}, &gr))
	assert.False(t, gr.IsWithdraw(), "a GR-stale route is advertised like any other")
	assert.Equal(t, 0, gr.Len(), "a GR-stale route is not modified")

	var llgr filterapi.ModAccumulator
	assert.True(t, LLGREgressFilter(src, railsEBGPDest, nil, map[string]any{"stale": uint8(2)}, &llgr))
	assert.True(t, llgr.IsWithdraw(), "a level 2 route to the same neighbor is still withheld")
}

// rebuiltMeta is the map buildRelayUpdate hands the egress filters for a route
// rebuilt from bgp-adj-rib-in storage.
func rebuiltMeta() map[string]any {
	return map[string]any{filterapi.MetaKeyRebuilt: true}
}

// TestLLGREgressFilter_RebuiltRecoversReceivedStale verifies the relay rail
// (AC-16 rail 3). A route rebuilt from storage carries no stale level, because
// no ingress step ran on it; the filter recovers the fact with the ingress
// predicate over the source and the stored bytes. Method: the rebuilt key, a
// source session whose own OPEN carried LLGR, and LLGR_STALE in COMMUNITIES,
// with each of the three removed in turn.
//
// VALIDATES: AC-16 rail 3, the rebuilt-route recovery by the ingress predicate.
// PREVENTS: a bgp-adj-rib-in replay sending a received LLGR_STALE route to a neighbor without LLGR.
//
// RFC requirement: RFC9494-4.3-3 positive -- a route rebuilt from storage (meta key "rebuilt")
// that carries LLGR_STALE and came from a session whose own OPEN carried the LLGR Capability is
// converted to a withdrawal toward an EBGP neighbor the capability was not received from, and reaches
// a neighbor that sent the capability unmodified, its LLGR_STALE left in the bytes.
// RFC requirement: RFC9494-4.3-3 negative -- a rebuilt route without LLGR_STALE, a rebuilt route from
// a session whose OPEN carried no LLGR Capability, and a route without the rebuilt key are each
// accepted toward that EBGP neighbor with no withdrawal and no modification.
func TestLLGREgressFilter_RebuiltRecoversReceivedStale(t *testing.T) {
	railsLoadedState(t)
	llgrSource := filterapi.PeerFilterInfo{Address: ingressTestPeer, LLGRAdvertised: true}
	plainSource := filterapi.PeerFilterInfo{Address: ingressTestPeer}
	stale := ingressTestUpdate(ingressLLGRStale)

	var withheld filterapi.ModAccumulator
	assert.True(t, LLGREgressFilter(llgrSource, railsEBGPDest, stale, rebuiltMeta(), &withheld))
	assert.True(t, withheld.IsWithdraw(), "the recovered LLGR_STALE route is withheld from the EBGP neighbor without LLGR")

	var kept filterapi.ModAccumulator
	assert.True(t, LLGREgressFilter(llgrSource, railsLLGRDest, stale, rebuiltMeta(), &kept))
	assert.False(t, kept.IsWithdraw(), "the neighbor that sent LLGR receives the route")
	assert.Equal(t, 0, kept.Len(), "LLGR_STALE stays in the bytes, so no modification is recorded")

	cases := []struct {
		name    string
		src     filterapi.PeerFilterInfo
		payload []byte
		meta    map[string]any
	}{
		{"rebuilt without LLGR_STALE", llgrSource, ingressTestUpdate(ingressNoExport), rebuiltMeta()},
		{"rebuilt from a session that sent no LLGR", plainSource, stale, rebuiltMeta()},
		{"not rebuilt: the ingress step already ran", llgrSource, stale, map[string]any{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mods filterapi.ModAccumulator
			assert.True(t, LLGREgressFilter(tc.src, railsEBGPDest, tc.payload, tc.meta, &mods))
			assert.False(t, mods.IsWithdraw(), "the route is advertised")
			assert.Equal(t, 0, mods.Len(), "the route is not modified")
		})
	}
}
