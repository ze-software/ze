// Design: docs/architecture/core-design.md -- LLGR egress filter
// RFC: rfc/short/rfc9494.md -- Long-Lived Graceful Restart readvertisement
// Overview: gr.go -- GR plugin entry point, peerLLGRCaps storage
// Related: gr_state.go -- LLGR state transitions, llgrActiveCount maintenance

package gr

import (
	"encoding/binary"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// Attribute type codes used in modification operations.
const (
	attrCodeLocalPref uint8 = 5 // LOCAL_PREF (RFC 4271)
	attrCodeCommunity uint8 = 8 // COMMUNITIES (RFC 1997)
)

// communityNoExport is NO_EXPORT (0xFFFFFF01) in big-endian wire format.
var communityNoExport = func() [4]byte {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], 0xFFFFFF01)
	return b
}()

// localPrefZero is LOCAL_PREF=0 in big-endian wire format.
var localPrefZero [4]byte // zero-value is correct: 0x00000000

// egressFilterState holds the shared state read by LLGREgressFilter.
// Set atomically by RunGRPlugin; read by the egress filter on the hot path.
type egressFilterState struct {
	localAS         uint32
	peerLLGRCaps    map[string]*llgrPeerCap // peerAddr -> LLGR capability (read under mu in grPlugin)
	llgrActiveCount atomic.Int32            // number of peers currently in LLGR state
}

// egressState is the package-level pointer to the filter's shared state.
// Stored atomically: nil before RunGRPlugin, non-nil after.
var egressState atomic.Pointer[egressFilterState]

// setEgressState sets the package-level egress filter state.
// Called by RunGRPlugin on startup and by tests.
func setEgressState(s *egressFilterState) {
	egressState.Store(s)
}

// LLGREgressFilter is the LLGR egress filter registered in the plugin registry.
// Called by the reactor for each destination peer during ForwardUpdate.
//
// RFC 9494 Section 4.5: LLGR_STALE routes SHOULD NOT be advertised to peers
// that have not advertised the LLGR capability.
//
// Fast path: when no peers are in LLGR state (common case), returns true
// after one atomic load -- zero overhead on normal traffic.
func LLGREgressFilter(src, dest registry.PeerFilterInfo, payload []byte, meta map[string]any, mods *registry.ModAccumulator) bool {
	s := egressState.Load()
	if s == nil {
		return true // Plugin not yet started.
	}

	// Fast path: no peers in LLGR state.
	if s.llgrActiveCount.Load() == 0 {
		return true
	}

	// Check route metadata for stale level.
	staleLevel := staleFromMeta(meta)
	if staleLevel == 0 {
		return true // Non-stale route, pass through.
	}

	// Route is stale. Check destination peer's LLGR capability.
	destAddr := dest.Address.String()
	if _, hasLLGR := s.peerLLGRCaps[destAddr]; hasLLGR {
		// RFC 9494: LLGR-capable peer receives the route as-is.
		// LLGR_STALE community is already in wire bytes (attached by rib attach-community).
		return true
	}

	// Destination peer does NOT have LLGR capability.
	isIBGP := dest.PeerAS == s.localAS

	if isIBGP {
		// RFC 9494 Section 4.5.3: Partial deployment (IBGP).
		// Attach NO_EXPORT community and set LOCAL_PREF=0.
		// Route is delivered but deprioritized.
		mods.Op(attrCodeCommunity, registry.AttrModAdd, communityNoExport[:])
		mods.Op(attrCodeLocalPref, registry.AttrModSet, localPrefZero[:])
		return true
	}

	// EBGP non-LLGR peer: convert announce to withdrawal.
	// RFC 9494: "routes with LLGR_STALE SHOULD NOT be advertised to
	// peers that have not advertised the LLGR capability."
	// The forward path converts the announce UPDATE to a withdrawal
	// so the peer removes the now-stale route from its RIB.
	mods.SetWithdraw()
	return true
}

// staleFromMeta extracts the stale level from route metadata.
// Returns 0 if meta is nil, has no "stale" key, or has an unexpected type.
func staleFromMeta(meta map[string]any) uint8 {
	if meta == nil {
		return 0
	}
	v, ok := meta["stale"]
	if !ok {
		return 0
	}
	switch level := v.(type) {
	case uint8:
		return level
	case int:
		return uint8(level)
	}
	logger().Warn("unexpected stale metadata type", "type", v)
	return 0
}
