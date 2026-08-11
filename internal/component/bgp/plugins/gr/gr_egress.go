// Design: docs/architecture/core-design.md -- LLGR egress filter
// RFC: rfc/short/rfc9494.md -- Long-Lived Graceful Restart readvertisement
// Overview: gr.go -- GR plugin entry point, peerLLGRCaps storage
// Related: gr_state.go -- LLGR state transitions

package gr

import (
	"encoding/binary"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/slogutil"
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
//
// peerLLGRCaps is grPlugin's live map, shared by reference so the filter sees
// peers as they are learned and released. mu is grPlugin.mu, the lock every
// writer of that map holds (gr.go, gr_removal.go). A reader must take it too:
// a peer flap concurrent with a forward is otherwise a fatal concurrent map
// read and map write, which recover() does not catch. Read it through hasLLGR.
//
// The local AS for iBGP detection is NOT stored here: the reactor supplies the
// effective per-peer local AS per destination via filterapi.PeerFilterInfo.LocalAS
// (dest.LocalAS), which correctly honors a per-peer local-as override where a
// single captured global value would not.
type egressFilterState struct {
	mu           *sync.Mutex             // grPlugin.mu: guards peerLLGRCaps
	peerLLGRCaps map[string]*llgrPeerCap // peerAddr -> LLGR capability from the peer's OPEN
}

// hasLLGR reports whether the LLGR Capability was received from peerAddr.
func (s *egressFilterState) hasLLGR(peerAddr string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.peerLLGRCaps[peerAddr]
	return ok
}

// egressState is the package-level pointer to the filter's shared state.
// Stored atomically: nil before RunGRPlugin, non-nil after.
//
// A nil state is the ABSENCE of an answer about every peer's LLGR capability,
// never the answer "no peer is stale". LLGREgressFilter therefore reads nil as
// "the LLGR Capability has not been received from this neighbor", which is the
// literal truth while nothing has been recorded, and applies the same RFC 9494
// treatment it applies to a peer known not to have advertised it.
var egressState atomic.Pointer[egressFilterState]

// egressStateMissingWarned latches the one WARN emitted when the egress filter
// runs on a stale route with no plugin state loaded. Latched rather than logged
// per route because this sits on the forward path (ai/rules/performance.md).
var egressStateMissingWarned atomic.Bool

// grSubsystem is the canonical slog subsystem name for this plugin: the
// registration name "bgp-gr" (register.go) as CanonicalSubsystemName renders it,
// which is what ConfigureEngineLogger is handed on the engine path.
const grSubsystem = "bgp.gr"

// egressWarnLogger returns a logger that can carry the egress warning even when
// the engine never started. Every caller of SetLogger is on the engine path
// (register.go), and the warning exists for the case where that path never runs,
// leaving loggerPtr at init()'s discard logger. slogutil.Logger then builds a
// live one from the env hierarchy the engine would have used, so an operator who
// silenced the subsystem still gets silence.
func egressWarnLogger() *slog.Logger {
	if loggerConfigured.Load() {
		return logger()
	}
	return slogutil.Logger(grSubsystem)
}

// setEgressState sets the package-level egress filter state.
// Called by RunGRPlugin on startup and by tests.
func setEgressState(s *egressFilterState) {
	egressState.Store(s)
}

// LLGREgressFilter is the LLGR egress filter registered with the BGP filter pipeline (filterapi).
// Called by the reactor for each destination peer during ForwardUpdate.
//
// RFC 9494 Section 4.3: "The route SHOULD NOT be advertised to any neighbor
// from which the Long-Lived Graceful Restart Capability has not been received.
// The exception is described in Section 4.6. Note that this requirement implies
// that such routes should be withdrawn from any such neighbor."
//
// A route with no stale metadata returns true immediately. That check comes
// first because it needs no plugin state, so the state is read only for a route
// the RFC governs. An unloaded state answers "not received" rather than
// accepting -- see egressState.
func LLGREgressFilter(src, dest filterapi.PeerFilterInfo, payload []byte, meta map[string]any, mods *filterapi.ModAccumulator) bool {
	// The stale level on the ROUTE is the authoritative signal, not a peer-state
	// counter: a route carries meta["stale"] both for an LLGR restart transition
	// (onLLGREntryDone) and for an operator or GR-timer `request bgp rib
	// mark-stale`, which marks routes stale with no LLGR restart at all.
	staleLevel := staleFromMeta(meta)
	if staleLevel == 0 {
		return true // Non-stale route, pass through. This is the real fast path.
	}

	// Route is stale. Resolve the destination peer's LLGR capability. Being wrong
	// on an unloaded state costs a withdraw or depreference toward a peer that
	// turns out to be LLGR-capable; being wrong the other way puts a long-lived
	// stale route into a neighbor that never agreed to hold one, which is the risk
	// RFC 9494 Section 5.2 names.
	hasLLGR := false
	if s := egressState.Load(); s != nil {
		hasLLGR = s.hasLLGR(dest.Address.String())
	} else if !egressStateMissingWarned.Load() && egressStateMissingWarned.CompareAndSwap(false, true) {
		egressWarnLogger().Warn("LLGR egress state not loaded, treating every destination as LLGR-incapable",
			"dest", dest.Address,
			"rfc", "RFC 9494 Section 4.3",
			"note", "first occurrence for this process; the GR plugin engine has not stored its egress state, so stale routes are withdrawn (EBGP) or depreferenced (IBGP) until it does")
	}
	if hasLLGR {
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
		// The route is delivered but deprioritized.
		mods.Op(attrCodeCommunity, filterapi.AttrModAdd, communityNoExport[:])
		mods.Op(attrCodeLocalPref, filterapi.AttrModSet, localPrefZero[:])
		return true
	}

	// EBGP non-LLGR peer.
	// RFC 9494 Section 4.3: "The route SHOULD NOT be advertised to any neighbor
	// from which the Long-Lived Graceful Restart Capability has not been
	// received. The exception is described in Section 4.6. Note that this
	// requirement implies that such routes should be withdrawn from any such
	// neighbor."
	// The forward path converts the announce UPDATE to a withdraw, so the peer
	// drops the stale route.
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
