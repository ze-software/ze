package gr

import (
	"encoding/binary"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// newTestEgressState creates an egressFilterState for testing with the given localAS
// and LLGR peer capabilities.
func newTestEgressState(localAS uint32, llgrPeers map[string]*llgrPeerCap) *egressFilterState {
	s := &egressFilterState{
		localAS:      localAS,
		peerLLGRCaps: llgrPeers,
	}
	return s
}

// TestLLGREgressFilter_NonStale verifies that non-stale routes pass through immediately.
//
// VALIDATES: AC-7: non-stale route passes through without modification.
// PREVENTS: Filter incorrectly acting on normal routes.
func TestLLGREgressFilter_NonStale(t *testing.T) {
	state := newTestEgressState(65000, map[string]*llgrPeerCap{
		"10.0.0.1": {}, // One peer has LLGR caps
	})
	state.llgrActiveCount.Store(1)
	setEgressState(state)
	defer setEgressState(nil)

	src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001}
	var mods registry.ModAccumulator

	// No meta["stale"] => non-stale route, should pass through.
	meta := map[string]any{}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	assert.True(t, accept, "non-stale route should be accepted")
	assert.Equal(t, 0, mods.Len(), "no mods for non-stale route")
}

// TestLLGREgressFilter_NoLLGRActive verifies atomic fast path when no peers are in LLGR.
//
// VALIDATES: AC-7: egress filter returns immediately when no peers in LLGR (zero overhead).
// PREVENTS: Unnecessary map lookups and metadata checks on normal traffic.
func TestLLGREgressFilter_NoLLGRActive(t *testing.T) {
	state := newTestEgressState(65000, nil)
	state.llgrActiveCount.Store(0)
	setEgressState(state)
	defer setEgressState(nil)

	src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001}
	var mods registry.ModAccumulator

	// Even with stale metadata, fast path skips everything.
	meta := map[string]any{"stale": uint8(2)}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	assert.True(t, accept, "fast path should accept when no LLGR active")
	assert.Equal(t, 0, mods.Len(), "no mods on fast path")
}

// TestLLGREgressFilter_NilState verifies filter passes through when plugin not yet started.
//
// VALIDATES: Safe behavior before plugin initialization.
// PREVENTS: Nil pointer panic before RunGRPlugin.
func TestLLGREgressFilter_NilState(t *testing.T) {
	setEgressState(nil)

	src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001}
	var mods registry.ModAccumulator

	meta := map[string]any{"stale": uint8(2)}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	assert.True(t, accept, "nil state should pass through")
	assert.Equal(t, 0, mods.Len(), "no mods when state is nil")
}

// TestLLGREgressFilter_LLGRPeer verifies stale routes are accepted for LLGR-capable peers.
//
// VALIDATES: AC-1: LLGR_STALE route advertised to LLGR-capable peer.
// VALIDATES: AC-3: LLGR_STALE community NOT removed (no mods, already in wire bytes).
// PREVENTS: Stale routes being suppressed or modified for LLGR-capable peers.
func TestLLGREgressFilter_LLGRPeer(t *testing.T) {
	state := newTestEgressState(65000, map[string]*llgrPeerCap{
		"10.0.0.2": {Families: []llgrCapFamily{{LLST: 3600}}},
	})
	state.llgrActiveCount.Store(1)
	setEgressState(state)
	defer setEgressState(nil)

	src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001}
	var mods registry.ModAccumulator

	meta := map[string]any{"stale": uint8(2)}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	assert.True(t, accept, "stale route accepted for LLGR-capable peer")
	assert.Equal(t, 0, mods.Len(), "no mods for LLGR-capable peer (community already in wire)")
}

// TestLLGREgressFilter_EBGPNonLLGR verifies stale routes are suppressed for EBGP non-LLGR peers.
//
// VALIDATES: AC-2: LLGR_STALE route to EBGP peer without LLGR capability is suppressed.
// PREVENTS: Stale routes being advertised to peers that cannot handle LLGR_STALE.
func TestLLGREgressFilter_EBGPNonLLGR(t *testing.T) {
	state := newTestEgressState(65000, map[string]*llgrPeerCap{
		// 10.0.0.2 NOT in peerLLGRCaps => non-LLGR
	})
	state.llgrActiveCount.Store(1)
	setEgressState(state)
	defer setEgressState(nil)

	src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001} // EBGP (65001 != 65000)
	var mods registry.ModAccumulator

	meta := map[string]any{"stale": uint8(2)}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	// RFC 9494: LLGR_STALE routes SHOULD NOT be advertised to non-LLGR peers.
	assert.False(t, accept, "stale route suppressed for EBGP non-LLGR peer")
}

// TestLLGREgressFilter_IBGPPartial verifies stale routes are modified for IBGP non-LLGR peers.
//
// VALIDATES: AC-4: partial deployment: IBGP peer without LLGR gets NO_EXPORT + LOCAL_PREF=0.
// PREVENTS: Stale routes being silently dropped for IBGP peers in partial deployment.
func TestLLGREgressFilter_IBGPPartial(t *testing.T) {
	state := newTestEgressState(65000, map[string]*llgrPeerCap{
		// 10.0.0.2 NOT in peerLLGRCaps => non-LLGR
	})
	state.llgrActiveCount.Store(1)
	setEgressState(state)
	defer setEgressState(nil)

	src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65000} // IBGP (65000 == localAS)
	var mods registry.ModAccumulator

	meta := map[string]any{"stale": uint8(2)}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	assert.True(t, accept, "IBGP partial deployment: route accepted with mods")
	ops := mods.Ops()
	assert.GreaterOrEqual(t, len(ops), 2, "should have community add + local-pref set mods")

	// Verify we have community add (NO_EXPORT) and local-pref set (0)
	hasCommunityAdd := false
	hasLocalPrefSet := false
	for _, op := range ops {
		// COMMUNITIES type code 8
		if op.Code == 8 && op.Action == registry.AttrModAdd {
			hasCommunityAdd = true
			// Verify NO_EXPORT community value (0xFFFFFF01) in big-endian
			assert.Equal(t, 4, len(op.Buf), "community value should be 4 bytes")
			val := binary.BigEndian.Uint32(op.Buf)
			assert.Equal(t, uint32(0xFFFFFF01), val, "should be NO_EXPORT community")
		}
		// LOCAL_PREF type code 5
		if op.Code == 5 && op.Action == registry.AttrModSet {
			hasLocalPrefSet = true
			// Verify LOCAL_PREF=0 in big-endian
			assert.Equal(t, 4, len(op.Buf), "local-pref value should be 4 bytes")
			val := binary.BigEndian.Uint32(op.Buf)
			assert.Equal(t, uint32(0), val, "should be LOCAL_PREF=0")
		}
	}
	assert.True(t, hasCommunityAdd, "should add NO_EXPORT community (attr code 8)")
	assert.True(t, hasLocalPrefSet, "should set LOCAL_PREF=0 (attr code 5)")
}

// TestLLGREgressFilter_NilMeta verifies nil meta is handled safely.
//
// VALIDATES: Defensive: nil meta does not panic.
// PREVENTS: Nil pointer dereference when meta is nil.
func TestLLGREgressFilter_NilMeta(t *testing.T) {
	state := newTestEgressState(65000, nil)
	state.llgrActiveCount.Store(1)
	setEgressState(state)
	defer setEgressState(nil)

	src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001}
	var mods registry.ModAccumulator

	accept := LLGREgressFilter(src, dest, nil, nil, &mods)

	assert.True(t, accept, "nil meta should pass through")
	assert.Equal(t, 0, mods.Len(), "no mods for nil meta")
}

// TestLLGREgressFilter_ConcurrentAccess verifies thread safety of egress state access.
//
// VALIDATES: Concurrent egress filter calls do not race on shared state.
// PREVENTS: Data race under concurrent ForwardUpdate to multiple peers.
func TestLLGREgressFilter_ConcurrentAccess(t *testing.T) {
	state := newTestEgressState(64500, map[string]*llgrPeerCap{
		"10.0.0.2": {Families: []llgrCapFamily{{LLST: 3600}}},
	})
	state.llgrActiveCount.Store(1)
	setEgressState(state)
	defer setEgressState(nil)

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
			dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 64501}
			var mods registry.ModAccumulator
			meta := map[string]any{"stale": uint8(2)}
			LLGREgressFilter(src, dest, nil, meta, &mods)
		})
	}
	wg.Wait()
}
