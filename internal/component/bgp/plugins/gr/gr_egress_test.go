package gr

import (
	"bytes"
	"context"
	"encoding/binary"
	"log/slog"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// newTestEgressState creates an egressFilterState for testing with the given
// LLGR peer capabilities. iBGP detection no longer reads a stored local AS: the
// reactor supplies it per destination via dest.LocalAS, so tests set that field
// on the destination PeerFilterInfo instead. The mutex stands in for grPlugin.mu,
// which the production state shares with the map's writers.
func newTestEgressState(llgrPeers map[string]*llgrPeerCap) *egressFilterState {
	return &egressFilterState{
		mu:           new(sync.Mutex),
		peerLLGRCaps: llgrPeers,
	}
}

// TestLLGREgressFilter_NonStale verifies that non-stale routes pass through immediately.
//
// VALIDATES: AC-7: non-stale route passes through without modification.
// PREVENTS: Filter incorrectly acting on normal routes.
func TestLLGREgressFilter_NonStale(t *testing.T) {
	state := newTestEgressState(map[string]*llgrPeerCap{
		"10.0.0.1": {}, // One peer has LLGR caps
	})
	setEgressState(state)
	defer setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001, LocalAS: 65000}
	var mods filterapi.ModAccumulator

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
// TestLLGREgressFilter_StaleDepreferencesRegardlessOfActiveCount is the
// regression guard for a real RFC 9494 Section 4.5.3 violation that this test
// used to ENCODE as correct. That old test (TestLLGREgressFilter_NoLLGRActive)
// asserted no modifications whenever the peer-restart counter was zero. Its dest
// was eBGP (PeerAS 65001, LocalAS 65000), so its assertions were doubly weak: on
// an eBGP dest the correct outcome is a withdraw, which also leaves mods.Len()==0,
// so the test passed under both the bug and the fix and gated neither. This
// replacement pins the iBGP depreference, which the bug genuinely suppressed.
//
// That counter is incremented only when an LLGR RESTART transition completes
// (gr.go onLLGREntryDone), never by `request bgp rib mark-stale`. A route
// readvertised stale via the mark-stale path therefore reached the filter with
// staleLevel==2 but activeCount==0, took the fast-path early return, and went out
// FRESH toward a non-LLGR iBGP peer -- exactly the depreference RFC 9494 requires.
// The route's stale level, not a peer-state counter, is the authoritative signal.
//
// This strengthens the RFC9494-4.6-2 / 4.6-3 positives already proven by
// TestLLGREgressFilter_IBGPPartial (which sets activeCount=1). That test passed
// while the bug shipped precisely because it never exercised activeCount==0, the
// state the mark-stale readvertise path produces. No new requirement id: same
// obligation, exercised in the state that was missing.
func TestLLGREgressFilter_StaleDepreferencesRegardlessOfActiveCount(t *testing.T) {
	state := newTestEgressState(nil)
	setEgressState(state)
	defer setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	// iBGP: dest.PeerAS == dest.LocalAS, and NOT in peerLLGRCaps (non-LLGR).
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65000, LocalAS: 65000}
	var mods filterapi.ModAccumulator

	meta := map[string]any{"stale": uint8(2)}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	assert.True(t, accept, "route is delivered, depreferenced, not withdrawn")
	assert.True(t, mods.HasModifications(),
		"a stale route to a non-LLGR iBGP peer MUST be depreferenced even when activeCount==0")
}

// TestLLGREgressFilter_FreshRouteIsTheOnlyFastPath verifies the surviving fast
// exit is the stale-level check: a route carrying no stale metadata passes
// through untouched, which is the common (non-readvertise) case.
func TestLLGREgressFilter_FreshRouteIsTheOnlyFastPath(t *testing.T) {
	state := newTestEgressState(nil)
	setEgressState(state)
	defer setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65000, LocalAS: 65000}
	var mods filterapi.ModAccumulator

	// No "stale" key: a normal, non-readvertised route.
	accept := LLGREgressFilter(src, dest, nil, map[string]any{}, &mods)

	assert.True(t, accept, "a fresh route passes through")
	assert.Equal(t, 0, mods.Len(), "a fresh route gets no modifications")
}

// TestLLGREgressFilter_StaleEBGPWithdrawsRegardlessOfRestartState covers the
// other half of the same bug: with the removed peer-restart guard, a stale route
// to a non-LLGR EBGP peer would also have gone out FRESH when no peer was
// restarting, instead of being withdrawn (RFC 9494 Section 4.3: stale routes
// SHOULD NOT be advertised to peers lacking the LLGR capability). No peer-restart
// state is set up here on purpose.
func TestLLGREgressFilter_StaleEBGPWithdrawsRegardlessOfRestartState(t *testing.T) {
	state := newTestEgressState(nil) // 10.0.0.2 not in peerLLGRCaps => non-LLGR
	setEgressState(state)
	defer setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	// EBGP: dest.PeerAS (65001) != dest.LocalAS (65000).
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001, LocalAS: 65000}
	var mods filterapi.ModAccumulator

	meta := map[string]any{"stale": uint8(2)}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	assert.True(t, accept, "the route is handled (converted to a withdraw), not passed as-is")
	assert.True(t, mods.IsWithdraw(),
		"a stale route to a non-LLGR eBGP peer MUST be withdrawn, not advertised fresh")
}

// TestLLGREgressFilter_NilStateWithdrawsEBGP pins the answer for an unloaded
// plugin state, which used to be a silent ACCEPT.
//
// The window is real and is not closed by ordering. filterapi.Register puts
// LLGREgressFilter into the egress pipeline from init() (register.go), so it is
// callable the moment the ze_bgp build group is linked, while the only store of
// egressState is setEgressState from RunGRPlugin's OnConfigure callback (gr.go).
// If the GR plugin engine never runs in this process, the state is nil for its
// whole lifetime and the filter is still registered and still called.
//
// This test previously existed as TestLLGREgressFilter_NilState and asserted the
// opposite: a stale route to an EBGP destination passing through with
// mods.Len()==0. That is the Section 4.3 violation with a green bar on top
// (ai/rules/rfc-compliance.md: a test that pins non-conformant behavior is the
// wrong artifact). Its stated purpose, "PREVENTS: nil pointer panic", is still
// served -- a nil state must not panic -- but not-panicking never required
// advertising the route.
//
// RFC requirement: RFC9494-4.3-3 positive -- with no LLGR capability recorded for
// any neighbor, "has not been received" is true of this destination, so the stale
// route is withdrawn from it rather than advertised. LLGREgressFilter resolves an
// unloaded egressState to hasLLGR=false and falls to the EBGP SetWithdraw branch.
func TestLLGREgressFilter_NilStateWithdrawsEBGP(t *testing.T) {
	setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	// EBGP: dest.PeerAS (65001) != dest.LocalAS (65000).
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001, LocalAS: 65000}
	var mods filterapi.ModAccumulator

	meta := map[string]any{"stale": uint8(2)}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	assert.True(t, accept, "the route is handled (converted to a withdraw), not passed as-is")
	assert.True(t, mods.IsWithdraw(),
		"an unloaded egress state must not advertise a stale route to an EBGP peer whose LLGR capability was never received")
}

// TestLLGREgressFilter_NilStateDepreferencesIBGP is the Section 4.6 half of the
// same window: an unloaded state must not skip the partial-deployment
// depreference either. The deferral shard recorded exactly this, that the nil
// state "withholds the destination's LLGR capability" and so Section 4.6's iBGP
// depreference "is skipped too".
//
// RFC requirement: RFC9494-4.6-2 positive -- NO_EXPORT is attached to the stale
// route on the partial-deployment branch even when the plugin state is unloaded,
// because an unloaded state resolves to "capability not received" rather than to
// an early accept that reaches no branch at all.
// RFC requirement: RFC9494-4.6-3 positive -- LOCAL_PREF is likewise set to zero on
// that branch with no plugin state loaded.
func TestLLGREgressFilter_NilStateDepreferencesIBGP(t *testing.T) {
	setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	// IBGP: dest.PeerAS (65000) == dest.LocalAS (65000).
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65000, LocalAS: 65000}
	var mods filterapi.ModAccumulator

	meta := map[string]any{"stale": uint8(2)}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	assert.True(t, accept, "IBGP partial deployment: the route is delivered, depreferenced, not withdrawn")
	assert.False(t, mods.IsWithdraw(), "an internal neighbor is the Section 4.6 exception, not a withdrawal target")

	hasCommunityAdd := false
	hasLocalPrefSet := false
	for _, op := range mods.Ops() {
		if op.Code == attrCodeCommunity && op.Action == filterapi.AttrModAdd {
			hasCommunityAdd = true
			assert.Equal(t, 4, len(op.Buf), "community value should be 4 bytes")
			assert.Equal(t, uint32(0xFFFFFF01), binary.BigEndian.Uint32(op.Buf), "should be NO_EXPORT community")
		}
		if op.Code == attrCodeLocalPref && op.Action == filterapi.AttrModSet {
			hasLocalPrefSet = true
			assert.Equal(t, 4, len(op.Buf), "local-pref value should be 4 bytes")
			assert.Equal(t, uint32(0), binary.BigEndian.Uint32(op.Buf), "should be LOCAL_PREF=0")
		}
	}
	assert.True(t, hasCommunityAdd, "should add NO_EXPORT community with no plugin state loaded")
	assert.True(t, hasLocalPrefSet, "should set LOCAL_PREF=0 with no plugin state loaded")
}

// TestLLGREgressFilter_NilStateDoesNotSuppressFreshRoute bounds the fix. Failing
// closed applies only to routes Section 4.3 governs, which are the stale ones. A
// route carrying no stale metadata must still pass untouched with no state
// loaded, or an unstarted GR plugin would suppress ordinary traffic.
func TestLLGREgressFilter_NilStateDoesNotSuppressFreshRoute(t *testing.T) {
	setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001, LocalAS: 65000}
	var mods filterapi.ModAccumulator

	accept := LLGREgressFilter(src, dest, nil, map[string]any{}, &mods)

	assert.True(t, accept, "a fresh route passes through with no plugin state")
	assert.False(t, mods.IsWithdraw(), "a fresh route is never withdrawn by the LLGR filter")
	assert.Equal(t, 0, mods.Len(), "a fresh route gets no modifications")
}

// rfc-test-change-approved: 2026-08-07 Thomas approved replacing TestLLGREgressFilter_NilState, which asserted the RFC 9494 Section 4.3 violation itself: accept with no mods for a stale route to an EBGP peer whose LLGR capability was never received. Coverage was checked and did not shrink. accept==true and the no-panic assertion moved to TestLLGREgressFilter_NilStateWithdrawsEBGP; the mods.Len()==0 assertion moved to TestLLGREgressFilter_NilStateDoesNotSuppressFreshRoute, where it is correct. Thomas separately approved consolidating gr_egress_warn_test.go into this file on 2026-08-07; the three logger tests below arrive from there verbatim, with no assertion changed.

// withGRLoggerRestored saves and restores the package logger globals, so a test
// that simulates "the engine never started" cannot leak that state into whatever
// runs next in this binary.
func withGRLoggerRestored(t *testing.T) {
	t.Helper()
	prevLogger := loggerPtr.Load()
	prevConfigured := loggerConfigured.Load()
	prevWarned := egressStateMissingWarned.Load()
	t.Cleanup(func() {
		loggerPtr.Store(prevLogger)
		loggerConfigured.Store(prevConfigured)
		egressStateMissingWarned.Store(prevWarned)
	})
}

// TestLLGREgressWarnLoggerIsLiveWhenEngineNeverStarted pins the fix for a WARN
// that could not be heard on the one path it exists for.
//
// VALIDATES: the egress fail-closed guard says something, not just fails closed.
// PREVENTS: the latched warning being spent on a discard logger, leaving an
// unloaded LLGR egress state silent for the whole life of the process.
//
// Every caller of SetLogger is on the engine path (ConfigureEngineLogger and the
// CLI ConfigLogger, both in register.go). The case the warning is about is the
// case where that path never runs: no engine, so egressState is nil for the whole
// process. loggerPtr then still holds init()'s discard logger, so the latched
// WARN was spent on a line that went nowhere and no later occurrence could ever
// speak. Failing closed still held; "or say something" (ai/rules/evidence.md)
// did not.
//
// The assertion is on Enabled rather than on captured text because that is the
// property that was false: the logger reached in this state cannot carry a WARN
// at all. Reverting egressWarnLogger to plain logger() makes the second
// assertion fail.
func TestLLGREgressWarnLoggerIsLiveWhenEngineNeverStarted(t *testing.T) {
	withGRLoggerRestored(t)

	// Simulate a process where RunGRPlugin never ran: init()'s discard logger,
	// and SetLogger never called.
	loggerPtr.Store(slogutil.DiscardLogger())
	loggerConfigured.Store(false)

	ctx := context.Background()
	assert.False(t, logger().Enabled(ctx, slog.LevelWarn),
		"precondition: with no engine start the package logger is still init()'s discard logger")
	assert.True(t, egressWarnLogger().Enabled(ctx, slog.LevelWarn),
		"the egress warning must reach a live logger when the engine never started, or the latch is spent on a dropped line")
}

// TestLLGREgressWarnLoggerRespectsEngineChoice is the contrast: when the engine
// path DID run, its logger and its level are what the warning goes through. The
// fallback must not override an operator's configured logger.
func TestLLGREgressWarnLoggerRespectsEngineChoice(t *testing.T) {
	withGRLoggerRestored(t)

	var buf bytes.Buffer
	engineLogger := slogutil.LoggerWithOutput(grSubsystem, "warn", &buf)
	SetLogger(engineLogger)

	assert.Same(t, engineLogger, egressWarnLogger(),
		"once the engine installed a logger, the warning goes through that one, not a rebuilt fallback")
}

// TestLLGREgressFilterWarnsWhenStateMissing captures the logger and asserts the
// filter actually emits the diagnosis when it falls closed on an unloaded state.
// Deleting the warn branch leaves the buffer empty and fails this test.
//
// It also pins the latch: the message is once per process, so a second stale
// route must not re-emit it. That is what keeps the warning off the per-route
// forward path (ai/rules/performance.md).
func TestLLGREgressFilterWarnsWhenStateMissing(t *testing.T) {
	withGRLoggerRestored(t)

	var buf bytes.Buffer
	SetLogger(slogutil.LoggerWithOutput(grSubsystem, "warn", &buf))
	egressStateMissingWarned.Store(false)
	setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001, LocalAS: 65000}
	meta := map[string]any{"stale": uint8(2)}

	var mods filterapi.ModAccumulator
	LLGREgressFilter(src, dest, nil, meta, &mods)

	first := buf.String()
	assert.Contains(t, first, "LLGR egress state not loaded",
		"an unloaded state must say so, not fail closed in silence")
	assert.Contains(t, first, "10.0.0.2", "the warning names the destination it applied to")

	// Second stale route: same condition, no second line.
	var mods2 filterapi.ModAccumulator
	LLGREgressFilter(src, dest, nil, meta, &mods2)
	assert.Equal(t, first, buf.String(), "the warning is latched to one line per process")
}

// TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer is the contrast that
// keeps the fix from being a blanket suppression: once the state IS present and
// records the destination's LLGR capability, the same stale route goes out
// unmodified. Without this, "withdraw every stale route" would satisfy the
// positive above and be equally wrong.
//
// RFC requirement: RFC9494-4.3-3 negative -- the SHOULD NOT binds only a neighbor
// from which the capability "has not been received". For a destination present in
// peerLLGRCaps it has been received, so LLGREgressFilter returns before the
// withdraw and depreference branches and leaves the accumulator empty.
func TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer(t *testing.T) {
	state := newTestEgressState(map[string]*llgrPeerCap{
		"10.0.0.2": {Families: []llgrCapFamily{{LLST: 3600}}},
	})
	setEgressState(state)
	defer setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001, LocalAS: 65000}
	var mods filterapi.ModAccumulator

	meta := map[string]any{"stale": uint8(2)}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	assert.True(t, accept, "stale route accepted for an LLGR-capable peer")
	assert.False(t, mods.IsWithdraw(), "a peer that advertised LLGR must not have the route withdrawn")
	assert.Equal(t, 0, mods.Len(), "no mods for an LLGR-capable peer (community already in wire)")
}

// TestLLGREgressFilter_LLGRPeer verifies stale routes are accepted for LLGR-capable peers.
//
// VALIDATES: AC-1: LLGR_STALE route advertised to LLGR-capable peer.
// VALIDATES: AC-3: LLGR_STALE community NOT removed (no mods, already in wire bytes).
// PREVENTS: Stale routes being suppressed or modified for LLGR-capable peers.
//
// RFC requirement: RFC9494-4.6-2 negative -- NO_EXPORT is a partial-deployment measure, not a
// blanket rewrite: when the destination is present in peerLLGRCaps the filter returns before the
// iBGP branch and emits no community modification at all (LLGREgressFilter's `if hasLLGR` early
// return, internal/component/bgp/plugins/gr/gr_egress.go).
// RFC requirement: RFC9494-4.6-3 negative -- likewise no LOCAL_PREF is forced to zero for an
// LLGR-capable neighbor: that same early return leaves the accumulator empty, so the depreference
// applies only to the partial-deployment path.
func TestLLGREgressFilter_LLGRPeer(t *testing.T) {
	state := newTestEgressState(map[string]*llgrPeerCap{
		"10.0.0.2": {Families: []llgrCapFamily{{LLST: 3600}}},
	})
	setEgressState(state)
	defer setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001, LocalAS: 65000}
	var mods filterapi.ModAccumulator

	meta := map[string]any{"stale": uint8(2)}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	assert.True(t, accept, "stale route accepted for LLGR-capable peer")
	assert.Equal(t, 0, mods.Len(), "no mods for LLGR-capable peer (community already in wire)")
}

// TestLLGREgressFilter_EBGPNonLLGR verifies stale routes trigger withdrawal for EBGP non-LLGR peers.
//
// VALIDATES: AC-2: LLGR_STALE route to EBGP peer without LLGR -> withdrawal via SetWithdraw.
// PREVENTS: Stale routes being advertised to peers that cannot handle LLGR_STALE.
//
// RFC requirement: RFC9494-4.6-1 negative -- an EXTERNAL neighbor without LLGR never receives the
// stale route under the partial-deployment rules: with dest.PeerAS != dest.LocalAS LLGREgressFilter's
// isIBGP test fails and it converts the announce to a withdrawal with mods.SetWithdraw
// (internal/component/bgp/plugins/gr/gr_egress.go).
func TestLLGREgressFilter_EBGPNonLLGR(t *testing.T) {
	state := newTestEgressState(map[string]*llgrPeerCap{
		// 10.0.0.2 NOT in peerLLGRCaps => non-LLGR
	})
	setEgressState(state)
	defer setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	// EBGP: dest.PeerAS (65001) != dest.LocalAS (65000).
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001, LocalAS: 65000}
	var mods filterapi.ModAccumulator

	meta := map[string]any{"stale": uint8(2)}
	accept := LLGREgressFilter(src, dest, nil, meta, &mods)

	// RFC 9494: LLGR_STALE routes SHOULD NOT be advertised to non-LLGR peers.
	// Filter returns true (route processed) but marks for withdrawal conversion.
	assert.True(t, accept, "filter accepts, forward path converts to withdrawal")
	assert.True(t, mods.IsWithdraw(), "should set withdraw for EBGP non-LLGR peer")
}

// TestLLGREgressFilter_IBGPPartial verifies stale routes are modified for IBGP non-LLGR peers.
//
// VALIDATES: AC-4: partial deployment: IBGP peer without LLGR gets NO_EXPORT + LOCAL_PREF=0.
// PREVENTS: Stale routes being silently dropped for IBGP peers in partial deployment.
//
// RFC requirement: RFC9494-4.6-1 positive -- the partial-deployment branch that still delivers a
// stale route is reached only for an internal neighbor: LLGREgressFilter computes
// isIBGP = dest.PeerAS == dest.LocalAS and only that branch keeps the announce
// (internal/component/bgp/plugins/gr/gr_egress.go).
// RFC requirement: RFC9494-4.6-2 positive -- that branch attaches NO_EXPORT to the stale route via
// mods.Op(attrCodeCommunity, AttrModAdd, communityNoExport), the 0xFFFFFF01 wire value built by the
// communityNoExport package var (internal/component/bgp/plugins/gr/gr_egress.go).
// RFC requirement: RFC9494-4.6-3 positive -- the same branch sets LOCAL_PREF to zero via
// mods.Op(attrCodeLocalPref, AttrModSet, localPrefZero), the localPrefZero package var
// (internal/component/bgp/plugins/gr/gr_egress.go).
func TestLLGREgressFilter_IBGPPartial(t *testing.T) {
	state := newTestEgressState(map[string]*llgrPeerCap{
		// 10.0.0.2 NOT in peerLLGRCaps => non-LLGR
	})
	setEgressState(state)
	defer setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	// IBGP: dest.PeerAS (65000) == dest.LocalAS (65000).
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65000, LocalAS: 65000}
	var mods filterapi.ModAccumulator

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
		if op.Code == 8 && op.Action == filterapi.AttrModAdd {
			hasCommunityAdd = true
			// Verify NO_EXPORT community value (0xFFFFFF01) in big-endian
			assert.Equal(t, 4, len(op.Buf), "community value should be 4 bytes")
			val := binary.BigEndian.Uint32(op.Buf)
			assert.Equal(t, uint32(0xFFFFFF01), val, "should be NO_EXPORT community")
		}
		// LOCAL_PREF type code 5
		if op.Code == 5 && op.Action == filterapi.AttrModSet {
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

// TestLLGREgressIBGPClassification is the wiring test for the local-AS fix
// (plan/spec-fixit-local-asn-config-key.md AC-3). It proves the LLGR egress
// filter classifies iBGP vs eBGP from the reactor-supplied dest.LocalAS, NOT
// from a config-parsed value that was always 0. With the old bug (localAS==0),
// no real peer classified as iBGP and every stale iBGP route was wrongly
// withdrawn; here the same AS on both sides must take the NO_EXPORT branch.
func TestLLGREgressIBGPClassification(t *testing.T) {
	// 10.0.0.2 not LLGR-capable so classification is exercised.
	state := newTestEgressState(map[string]*llgrPeerCap{})
	setEgressState(state)
	defer setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	meta := map[string]any{"stale": uint8(2)}

	// Case 1: iBGP (dest.PeerAS == dest.LocalAS): NO_EXPORT + LOCAL_PREF=0, no withdraw.
	ibgp := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65000, LocalAS: 65000}
	var ibgpMods filterapi.ModAccumulator
	assert.True(t, LLGREgressFilter(src, ibgp, nil, meta, &ibgpMods), "iBGP stale route accepted")
	assert.False(t, ibgpMods.IsWithdraw(), "iBGP stale route must NOT be withdrawn (RFC 9494 4.5.3)")
	assert.GreaterOrEqual(t, ibgpMods.Len(), 2, "iBGP stale route gets NO_EXPORT + LOCAL_PREF=0")

	// Case 2: eBGP (dest.PeerAS != dest.LocalAS): withdrawal.
	ebgp := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001, LocalAS: 65000}
	var ebgpMods filterapi.ModAccumulator
	assert.True(t, LLGREgressFilter(src, ebgp, nil, meta, &ebgpMods), "eBGP stale route accepted (converted to withdrawal)")
	assert.True(t, ebgpMods.IsWithdraw(), "eBGP stale route withdrawn")

	// Regression guard: the old bug used a captured local AS of 0, so an iBGP
	// peer whose AS is 65000 would compare 65000 == 0 and fall to the eBGP
	// withdrawal branch. A dest.LocalAS of 0 must therefore classify a
	// nonzero-AS peer as eBGP (withdraw), never iBGP.
	zeroLocal := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65000, LocalAS: 0}
	var zeroMods filterapi.ModAccumulator
	assert.True(t, LLGREgressFilter(src, zeroLocal, nil, meta, &zeroMods), "route accepted")
	assert.True(t, zeroMods.IsWithdraw(), "peer with LocalAS==0 must not be misread as iBGP")
}

// TestLLGREgressFilter_NilMeta verifies nil meta is handled safely.
//
// VALIDATES: Defensive: nil meta does not panic.
// PREVENTS: Nil pointer dereference when meta is nil.
func TestLLGREgressFilter_NilMeta(t *testing.T) {
	state := newTestEgressState(nil)
	setEgressState(state)
	defer setEgressState(nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001, LocalAS: 65000}
	var mods filterapi.ModAccumulator

	accept := LLGREgressFilter(src, dest, nil, nil, &mods)

	assert.True(t, accept, "nil meta should pass through")
	assert.Equal(t, 0, mods.Len(), "no mods for nil meta")
}

// TestLLGREgressFilterReadsPeerCapsUnderTheWritersLock runs the forward path
// against the map's real WRITERS, which is the pairing that crashed the daemon.
//
// VALIDATES: LLGREgressFilter reads peerLLGRCaps under grPlugin.mu, the lock its
// writers hold.
// PREVENTS: `fatal error: concurrent map read and map write` on a peer flap. That
// fault is unrecoverable -- recover() does not catch it -- so a peer bouncing
// while any stale route is forwarded took the whole daemon down.
//
// egressFilterState carried gp.peerLLGRCaps BY REFERENCE (gr.go, OnConfigure)
// and the filter indexed that map with no lock, while extractGRCaps (gr.go)
// inserted and onPeerRemoved (gr_removal.go) deleted under gp.mu. This test
// drives those two producers, not a stand-in map write, so it fails for the
// reason the daemon failed.
//
// Reverting the fix (indexing s.peerLLGRCaps directly instead of s.hasLLGR)
// makes `go test -race` report the read/write pair here.
func TestLLGREgressFilterReadsPeerCapsUnderTheWritersLock(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		removedPeers: make(map[string]bool),
	}
	gp.state = newGRStateManager(func(string) {})
	setEgressState(&egressFilterState{mu: &gp.mu, peerLLGRCaps: gp.peerLLGRCaps})
	defer setEgressState(nil)

	// Capability 71 for ipv4/unicast, F-bit set, LLST 3600: one 7-byte tuple
	// (RFC 9494 Section 3) wrapped in its code/length header.
	llgrCapBytes := []byte{71, 7, 0x00, 0x01, 0x01, 0x80, 0x00, 0x0e, 0x10}
	const peerAddr = "10.0.0.2"

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr(peerAddr), PeerAS: 65001, LocalAS: 65000}
	meta := map[string]any{"stale": uint8(2)}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { // reader: the forward path
			for range 500 {
				var mods filterapi.ModAccumulator
				LLGREgressFilter(src, dest, nil, meta, &mods)
			}
		})
		wg.Go(func() { // writer: the peer flapping
			for range 500 {
				gp.extractGRCaps(peerAddr, llgrCapBytes, false)
				gp.onPeerRemoved(peerAddr)
			}
		})
	}
	wg.Wait()
}

// TestLLGREgressFilter_ConcurrentAccess verifies thread safety of egress state access.
//
// VALIDATES: Concurrent egress filter calls do not race on shared state.
// PREVENTS: Data race under concurrent ForwardUpdate to multiple peers.
// It reads only; the concurrent WRITER case is
// TestLLGREgressFilterReadsPeerCapsUnderTheWritersLock above.
func TestLLGREgressFilter_ConcurrentAccess(t *testing.T) {
	state := newTestEgressState(map[string]*llgrPeerCap{
		"10.0.0.2": {Families: []llgrCapFamily{{LLST: 3600}}},
	})
	setEgressState(state)
	defer setEgressState(nil)

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
			dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 64501, LocalAS: 64500}
			var mods filterapi.ModAccumulator
			meta := map[string]any{"stale": uint8(2)}
			LLGREgressFilter(src, dest, nil, meta, &mods)
		})
	}
	wg.Wait()
}
