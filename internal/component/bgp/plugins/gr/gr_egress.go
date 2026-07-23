// Design: docs/architecture/core-design.md -- LLGR egress filter
// RFC: rfc/short/rfc9494.md -- Long-Lived Graceful Restart readvertisement
// Overview: gr.go -- GR plugin entry point, peerLLGRCaps storage
// Related: gr_state.go -- LLGR state transitions

package gr

import (
	"encoding/binary"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/filterapi"
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
//
// The local AS for iBGP detection is NOT stored here: the reactor supplies the
// effective per-peer local AS per destination via filterapi.PeerFilterInfo.LocalAS
// (dest.LocalAS), which correctly honors a per-peer local-as override where a
// single captured global value would not.
type egressFilterState struct {
	peerLLGRCaps map[string]*llgrPeerCap // peerAddr -> LLGR capability (read under mu in grPlugin)
}

// egressState is the package-level pointer to the filter's shared state.
// Stored atomically: nil before RunGRPlugin, non-nil after.
var egressState atomic.Pointer[egressFilterState]

// setEgressState sets the package-level egress filter state.
// Called by RunGRPlugin on startup and by tests.
func setEgressState(s *egressFilterState) {
	egressState.Store(s)
}

// LLGREgressFilter is the LLGR egress filter registered with the BGP filter pipeline (filterapi).
// Called by the reactor for each destination peer during ForwardUpdate.
//
// RFC 9494 Section 4.3: LLGR_STALE routes SHOULD NOT be advertised to peers
// that have not advertised the LLGR capability (the internal-neighbor exception
// is Section 4.6).
//
// Fast path: a route with no stale metadata (the common case, staleLevel == 0)
// returns true immediately with no modifications.
func LLGREgressFilter(src, dest filterapi.PeerFilterInfo, payload []byte, meta map[string]any, mods *filterapi.ModAccumulator) bool {
	s := egressState.Load()
	if s == nil {
		return true // Plugin not yet started.
	}

	// The stale level on the ROUTE is the authoritative signal, not a peer-state
	// counter. A route carries meta["stale"] whenever it is being readvertised
	// stale -- whether that was driven by an LLGR restart transition
	// (onLLGREntryDone) OR by an operator/GR-timer `request bgp rib mark-stale`,
	// which marks routes stale without any LLGR restart. An earlier version guarded
	// this filter on a peer-restart counter and dropped the depreference for the
	// mark-stale path entirely (the route went out fresh toward a non-LLGR iBGP
	// peer, an RFC 9494 Section 4.6 violation), so the check below on staleLevel is
	// the correct and sufficient fast exit.
	staleLevel := staleFromMeta(meta)
	if staleLevel == 0 {
		return true // Non-stale route, pass through. This is the real fast path.
	}

	// Route is stale. Check destination peer's LLGR capability.
	destAddr := dest.Address.String()
	if _, hasLLGR := s.peerLLGRCaps[destAddr]; hasLLGR {
		// RFC 9494: LLGR-capable peer receives the route as-is.
		// LLGR_STALE community is already in wire bytes (attached by rib attach-community).
		return true
	}

	// Destination peer does NOT have LLGR capability.
	// RFC 9494 Section 4.6: iBGP is dest.PeerAS == our local AS for this session.
	// The reactor supplies the effective per-peer local AS in dest.LocalAS
	// (peer_forward_facts.go / reactor_api_batch.go readvertise rail).
	isIBGP := dest.PeerAS == dest.LocalAS

	if isIBGP {
		// RFC 9494 Section 4.6: Optional Partial Deployment Procedure (IBGP).
		// Attach NO_EXPORT community and set LOCAL_PREF=0.
		// Route is delivered but deprioritized.
		mods.Op(attrCodeCommunity, filterapi.AttrModAdd, communityNoExport[:])
		mods.Op(attrCodeLocalPref, filterapi.AttrModSet, localPrefZero[:])
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
