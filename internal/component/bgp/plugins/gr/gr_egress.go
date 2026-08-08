// Design: docs/architecture/core-design.md -- LLGR egress filter
// RFC: rfc/short/rfc9494.md -- Long-Lived Graceful Restart readvertisement
// Overview: gr.go -- GR plugin entry point, peerLLGRCaps storage
// Related: gr_state.go -- LLGR state transitions

package gr

import (
	"encoding/binary"
	"log/slog"
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
//
// A nil state is the ABSENCE of an answer about every peer's LLGR capability,
// never the answer "no peer is stale". LLGREgressFilter therefore reads nil as
// "the LLGR Capability has not been received from this neighbor", which is the
// literal truth while nothing has been recorded, and applies the same RFC 9494
// treatment it applies to a peer known not to have advertised it.
var egressState atomic.Pointer[egressFilterState]

// egressStateMissingWarned latches the one WARN emitted when the egress filter
// runs on a stale route with no plugin state loaded (ai/rules/evidence.md, "or
// say something"). The filter is registered from init() in register.go, so it is
// live in the filterapi pipeline from process start, while the only store of
// egressState is RunGRPlugin's OnConfigure callback (gr.go). Nothing closes that
// gap by construction, and it never closes at all if the GR plugin engine does
// not run in this process.
//
// Latched rather than logged per route because this sits on the forward path
// (ai/rules/performance.md): at most one line per process, and the steady state
// is a single atomic load.
//
// The latch made the message UNHEARABLE on the one path it exists for, until
// egressWarnLogger below. Every caller of SetLogger is on the engine path
// (ConfigureEngineLogger and the CLI ConfigLogger, both in register.go), so in
// the case this warning is about -- the engine never runs, and the state is nil
// for the whole process -- logger() is still the discard logger from init().
// The latch was spent on a dropped line and no later occurrence could speak,
// which fails the "or say something" half of ai/rules/evidence.md while the
// fail-closed half still held.
var egressStateMissingWarned atomic.Bool

// grSubsystem is the canonical slog subsystem name for this plugin: the
// registration name "bgp-gr" (register.go) as CanonicalSubsystemName renders it,
// which is what ConfigureEngineLogger is handed on the engine path.
const grSubsystem = "bgp.gr"

// egressWarnLogger returns a logger that can actually carry the egress warning.
//
// When the engine path ran, it installed a logger and chose its level, so that
// choice is respected. When it did not run, loggerPtr still holds init()'s
// discard logger, and slogutil.Logger builds a live one from the same env
// hierarchy the engine would have used (defaulting to WARN). An operator who
// silenced the subsystem still gets silence: slogutil.Logger reads their
// setting. Called at most once per process, from behind the latch.
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
// Fast path: a route with no stale metadata (the common case, staleLevel == 0)
// returns true immediately with no modifications. That check comes first because
// it needs no plugin state, so the state is consulted only for a route the RFC
// actually governs.
//
// The capability is resolved from egressState, and an unloaded state answers
// "not received" rather than accepting -- see egressState's comment.
func LLGREgressFilter(src, dest filterapi.PeerFilterInfo, payload []byte, meta map[string]any, mods *filterapi.ModAccumulator) bool {
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

	// Route is stale. Resolve the destination peer's LLGR capability.
	//
	// A nil state does NOT mean "advertise anyway". RFC 9494 Section 4.3 keys the
	// decision on whether the Long-Lived Graceful Restart Capability "has been
	// received" from the neighbor, and with no state loaded nothing has been
	// received from anyone. So nil resolves to hasLLGR=false and the destination
	// takes the same path as a peer known not to have advertised the capability:
	// withdraw for EBGP, Section 4.6 depreference for IBGP.
	//
	// This is the safe direction of the two. Being wrong here costs a transient
	// withdraw or depreference toward a peer that turns out to be LLGR-capable,
	// repaired as soon as OnConfigure stores the state. Being wrong the other way
	// puts a long-lived stale route into a neighbor that never agreed to hold one,
	// at normal preference, which is the risk Section 5.2 describes.
	hasLLGR := false
	if s := egressState.Load(); s != nil {
		_, hasLLGR = s.peerLLGRCaps[dest.Address.String()]
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
