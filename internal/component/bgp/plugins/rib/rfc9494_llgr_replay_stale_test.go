// Design: docs/architecture/plugin/rib-storage-design.md -- Adj-RIB-Out replay and refresh
// RFC: rfc/short/rfc9494.md -- Section 4.3, a received LLGR_STALE route on the RIB rails
// Related: rib_replay.go -- collectGroupedRibOutRoutesFiltered, the peer-up and resend rails
// Related: ribout_entry.go -- collectRibOutRoutes, the ROUTE-REFRESH rail
// Related: sent_owner.go -- sentSourceState, the one source lookup the rails share

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var (
	replayStaleSource      = netip.MustParseAddr("192.0.2.1")
	replayStaleDestination = netip.MustParseAddr("192.0.2.20")
	replayStaleKey         = ribOutKey{Prefix: netip.MustParsePrefix("10.0.0.0/8")}
)

// replayStaleSourceMsgID is the MsgID of the source's received UPDATE, and the
// SourceMessageID the destination's sent copy records.
const replayStaleSourceMsgID = 77

// feedReplayStaleSource stores the source's 10.0.0.0/8 the way the received
// handler does, with the ingress steps' metadata and a received MsgID.
func feedReplayStaleSource(r *RIBManager, body []byte, meta map[string]any, msgID uint64) {
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, _ := wu.Attrs()
	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: replayStaleSource.String(),
		PeerAS:      rpc.RemoteASOf(65001),
		LocalAS:     65000,
		Meta:        meta,
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   body,
			WireUpdate: wu,
			AttrsWire:  attrs,
			Meta:       meta,
			MessageID:  msgID,
		},
	})
}

// replayStaleFixture builds the two-peer state every RIB rail reads: the source
// received 10.0.0.0/8 (LLGR_STALE from an LLGR session when stale is true,
// fresh otherwise), and the destination's Adj-RIB-Out holds the copy Ze sent
// it, current against that received revision. The copy's own StaleLevel is 0,
// which is what the sent handler stores.
func replayStaleFixture(t *testing.T, stale bool) *RIBManager {
	t.Helper()
	r := newTestRIBManager(t)
	if stale {
		feedReplayStaleSource(r, receivedStaleUpdate(communityLLGRStaleWire), ingressStaleMeta(), replayStaleSourceMsgID)
	} else {
		feedReplayStaleSource(r, receivedStaleUpdate(communityNoExportWire), nil, replayStaleSourceMsgID)
	}
	event := nativeSentEvent(t, replayStaleDestination, family.IPv4Unicast, []byte{8, 10}, nativeSentAttrs(), false, false)
	event.MsgID = 101
	event.RouteMeta = map[string]any{
		"source-peer":            replayStaleSource.String(),
		"source-message-id":      float64(replayStaleSourceMsgID),
		bgptypes.SourceOwnerMeta: "5",
	}
	r.handleSent(event)
	t.Cleanup(func() {
		for _, routes := range r.ribOut[replayStaleDestination] {
			for _, entry := range routes {
				entry.release()
			}
		}
	})
	copyEntry, ok := r.ribOut[replayStaleDestination][family.IPv4Unicast][replayStaleKey]
	require.True(t, ok, "the destination holds the sent copy")
	require.Equal(t, uint8(0), copyEntry.StaleLevel, "the sent handler stores the copy at level 0")
	return r
}

// setCopyStaleLevel writes the copy's retention mark, as markStaleCommand and
// attachSentSourceCommunityLocked do, so a test can show the rails do not take
// the received fact from that field.
func setCopyStaleLevel(r *RIBManager, level uint8) {
	routes := r.ribOut[replayStaleDestination][family.IPv4Unicast]
	entry := routes[replayStaleKey]
	entry.StaleLevel = level
	routes[replayStaleKey] = entry
}

// captureStaleMeta records the meta["stale"] of every route command a rail
// dispatches, and how many route commands it dispatched.
func captureStaleMeta(r *RIBManager) *[]any {
	var levels []any
	r.updateHook = func(_ string, meta map[string]any) {
		if meta == nil {
			return // A cursor reset or terminator, not a route.
		}
		levels = append(levels, meta["stale"])
	}
	return &levels
}

// TestPeerUpReplayReportsSourceIngressLevel verifies the peer-up RIB rail
// (AC-16 rail 2, design review r4 I-1). Method: a re-established destination
// (seenBefore true) whose sent copy is current against the source's received
// route; collectPeerUpReplay must hand on stale level 2 when the source route
// arrived LLGR_STALE, though the copy's own StaleLevel is 0, and level 0 when
// it arrived fresh. replayRoutesWithCursor then carries it as meta["stale"].
//
// VALIDATES: a received LLGR_STALE route replayed to a re-established neighbor reaches the
// readvertise egress filters at stale level 2.
// PREVENTS: the replay sending it as fresh, so LLGREgressFilter never withholds it.
//
// RFC requirement: RFC9494-4.3-3 positive -- the peer-up replay of a destination's copy of a route
// received with LLGR_STALE from a session whose OPEN carried LLGR is grouped at stale level 2 and
// dispatched with meta["stale"] 2, the level LLGREgressFilter withholds from a neighbor without
// LLGR, while the copy's own StaleLevel is 0.
// RFC requirement: RFC9494-4.3-3 negative -- the same copy of a route received without LLGR_STALE
// is grouped at stale level 0 and dispatched with no stale key.
func TestPeerUpReplayReportsSourceIngressLevel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		stale bool
		level uint8
		meta  any
	}{
		{"received LLGR_STALE", true, 2, uint8(2)},
		{"received fresh", false, 0, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := replayStaleFixture(t, tc.stale)
			r.peerMu.RLock()
			groups := r.collectPeerUpReplay(replayStaleDestination, true)
			r.peerMu.RUnlock()
			require.Len(t, groups, 1, "the current copy is replayed")
			assert.Equal(t, tc.level, groups[0].StaleLevel)
			assert.Equal(t, tc.level, groups[0].Route.StaleLevel)

			levels := captureStaleMeta(r)
			r.replayRoutesWithCursor(replayStaleDestination.String(), groups, 1)
			require.Len(t, *levels, 1, "one route command")
			assert.Equal(t, tc.meta, (*levels)[0])
		})
	}
}

// TestRIBReplayMetaReadsSourceIngressLevel verifies the resend and
// ROUTE-REFRESH RIB rails (AC-16 rail 2). Method: the copy's retention mark is
// written to 1 first, so the level 2 the rails hand on can only come from the
// source's received route; the refresh rail is also driven after the source
// re-announced the route still LLGR_STALE under a new MsgID the copy does not
// hold (design review r3 N-1), where the copy is no longer current yet the
// level must still be 2.
//
// VALIDATES: resendRoutesWithCursor and sendRoutes dispatch a received LLGR_STALE route with
// meta["stale"] 2, whatever the copy's own StaleLevel and MsgID say.
// PREVENTS: a refresh or `clear bgp rib out` sending the route as fresh, or at level 1, to a
// neighbor without LLGR.
//
// RFC requirement: RFC9494-4.3-3 positive -- the resend rail (collectGroupedRibOutRoutes into
// resendRoutesWithCursor) and the ROUTE-REFRESH rail (collectRibOutRoutes into sendRoutes) dispatch
// a destination's copy of a route received with LLGR_STALE with meta["stale"] 2 when the copy's own
// StaleLevel is 1, and the refresh rail still does after the source re-announced it with LLGR_STALE
// under a new MsgID.
// RFC requirement: RFC9494-4.3-3 negative -- the same two rails dispatch a copy of a route received
// without LLGR_STALE, whose own StaleLevel is 0, with no stale key.
func TestRIBReplayMetaReadsSourceIngressLevel(t *testing.T) {
	for _, tc := range []struct {
		name      string
		stale     bool
		copyLevel uint8
		meta      any
	}{
		{"received LLGR_STALE, copy marked level 1", true, 1, uint8(2)},
		{"received fresh, copy unmarked", false, 0, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := replayStaleFixture(t, tc.stale)
			setCopyStaleLevel(r, tc.copyLevel)

			r.peerMu.RLock()
			groups := r.collectGroupedRibOutRoutes(replayStaleDestination)
			refresh := r.collectRibOutRoutes(replayStaleDestination, family.IPv4Unicast)
			r.peerMu.RUnlock()

			levels := captureStaleMeta(r)
			resent, refused := r.resendRoutesWithCursor(replayStaleDestination.String(), groups)
			require.Equal(t, 1, resent)
			require.Equal(t, 0, refused)
			require.Len(t, *levels, 1, "resend: one route command")
			assert.Equal(t, tc.meta, (*levels)[0], "resend rail")

			levels = captureStaleMeta(r)
			r.sendRoutes(replayStaleDestination.String(), refresh)
			require.Len(t, *levels, 1, "refresh: one route command")
			assert.Equal(t, tc.meta, (*levels)[0], "refresh rail")
		})
	}

	t.Run("re-announced LLGR_STALE under a new MsgID", func(t *testing.T) {
		r := replayStaleFixture(t, true)
		feedReplayStaleSource(r, receivedStaleUpdate(communityLLGRStaleWire), ingressStaleMeta(), replayStaleSourceMsgID+1)

		r.peerMu.RLock()
		copyEntry := r.ribOut[replayStaleDestination][family.IPv4Unicast][replayStaleKey]
		current, _ := r.sentSourceState(family.IPv4Unicast, replayStaleKey, copyEntry)
		refresh := r.collectRibOutRoutes(replayStaleDestination, family.IPv4Unicast)
		r.peerMu.RUnlock()
		assert.False(t, current, "the copy no longer matches the source's current revision")

		levels := captureStaleMeta(r)
		r.sendRoutes(replayStaleDestination.String(), refresh)
		require.Len(t, *levels, 1)
		assert.Equal(t, uint8(2), (*levels)[0], "the level is read by key, never gated on MsgID")
	})
}
