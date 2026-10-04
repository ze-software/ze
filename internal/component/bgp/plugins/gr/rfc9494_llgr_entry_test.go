// Design: docs/guide/graceful-restart.md -- Long-Lived Graceful Restart, the helper's LLGR period
// RFC: rfc/short/rfc9494.md -- Section 4.2, Session Resets
// Overview: gr.go -- wireStateCallbacks and the session-down dispatch of both event paths
// Related: gr_state.go -- onSessionDown, enterLLGRLocked
// Related: real_rib_test.go -- registered RIB engine command consumer

package gr

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rfc9494Down delivers a TCP-failure session-down for testPeer on one of the two
// production event paths: the JSON handler or the structured (DirectBridge) one.
func rfc9494Down(gp *grPlugin, structured bool) {
	if structured {
		gp.handleStructuredEvent(&rpc.StructuredEvent{PeerAddress: testPeer,
			EventType: rpc.EventKindState, State: rpc.SessionStateDown, Reason: "tcp-failure"})
		return
	}
	gp.handleStateEvent(testPeer, map[string]any{"state": "down", "reason": "tcp-failure"})
}

// declareLocalLLGR records that Ze's own OPEN to testPeer carried the LLGR Capability
// for families, which is what the operator's long-lived-stale-time puts there. Without
// it a received LLGR family is ignored (RFC 9494 Section 5, exchangedLLGRLocked).
func declareLocalLLGR(gp *grPlugin, families ...family.Family) {
	declared := make(map[family.Family]bool, len(families))
	for _, f := range families {
		declared[f] = true
	}
	gp.recordSentLLGR(testPeer, declared)
}

// rfc9494Paths names the two session-down paths every test below runs on.
var rfc9494Paths = []struct {
	name       string
	structured bool
}{{"json", false}, {"structured", true}}

// TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod drops a session whose peer
// advertised Restart Time 0 and a Long-Lived Stale Time of 3600 seconds for IPv4,
// and reads the resulting received-route inventory and communities.
//
// VALIDATES: RFC 9494 Section 4.2: "After the session goes down, and before the session
// is re-established, the stale routes for an AFI/SAFI MUST be retained", and "if the
// Restart Time is zero and the Long-Lived Stale Time is nonzero, only the procedures
// particular to LLGR would apply". The session-down sequence (purge the previous cycle's
// stale routes, retain, mark stale) runs first; the LLGR entry then removes the NO_LLGR
// routes, attaches LLGR_STALE (wire value ffff0006) and raises the stale level to 2. No
// command after that purges or releases the retained routes.
// PREVENTS: the LLGR entry firing inside onSessionDown, before the session-down sequence,
// so the consecutive-restart "purge-stale <peer>" deleted every route the entry had just
// marked LLGR-stale and the "mark-stale <peer> 0" lowered the survivors back to level 1.
//
// RFC requirement: RFC9494-4.2-1 positive -- with Restart Time 0 and a nonzero Long-Lived Stale Time, either event path retains eligible received routes at LLGR stale level through DOWN.
// RFC requirement: RFC9494-4.2-4 positive -- the retained received route carries LLGR_STALE.
// RFC requirement: RFC9494-4.2-5 positive -- a received NO_LLGR route is deleted while an otherwise eligible sibling survives.
func TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod(t *testing.T) {
	for _, path := range rfc9494Paths {
		t.Run(path.name, func(t *testing.T) {
			gp, rib := newGRWithRealRIB(t)
			gp.peerCaps[testPeer] = testCap(0, famIPv4)
			gp.peerLLGRCaps[testPeer] = &llgrPeerCap{Families: []llgrCapFamily{
				{Family: family.IPv4Unicast, ForwardState: true, LLST: 3600},
			}}
			declareLocalLLGR(gp, family.IPv4Unicast)
			rib.received("18c63364", grReceivedAttrs)
			rib.received("18c63365", grReceivedAttrs+"c00804ffff0007")
			require.Len(t, rib.routes("198.51.101.0/24"), 1)

			rfc9494Down(gp, path.structured)

			rib.down()
			rib.requireStale("198.51.100.0/24", 2)
			require.Contains(t, rib.communities("198.51.100.0/24"), uint32(0xffff0006))
			require.Empty(t, rib.routes("198.51.101.0/24"))
			assert.True(t, gp.state.peerActive(testPeer), "the received route remains retained for LLST")
		})
	}
}

// TestRFC9494BothTimesZeroRetainsNothing drops a session whose peer advertised Restart
// Time 0 and listed IPv4 unicast in its LLGR Capability with a Long-Lived Stale Time of 0.
//
// VALIDATES: RFC 9494 Section 4.2: "If both are zero, none of these procedures would
// apply, only those of the base BGP specification [RFC4271]".
// PREVENTS: retaining a received route forever when neither expiry timer applies.
//
// RFC requirement: RFC9494-4.2-1 negative -- with both times zero, either event path removes the received route instead of retaining it.
// RFC requirement: RFC9494-4.2-2 negative -- a Long-Lived Stale Time of zero arms no timer: the family is purged at once and the peer leaves the LLGR state.
func TestRFC9494BothTimesZeroRetainsNothing(t *testing.T) {
	for _, path := range rfc9494Paths {
		t.Run(path.name, func(t *testing.T) {
			gp, rib := newGRWithRealRIB(t)
			gp.peerCaps[testPeer] = testCap(0, famIPv4)
			gp.peerLLGRCaps[testPeer] = &llgrPeerCap{Families: []llgrCapFamily{
				{Family: family.IPv4Unicast, ForwardState: true, LLST: 0},
			}}
			declareLocalLLGR(gp, family.IPv4Unicast)
			rib.received("18c63364", grReceivedAttrs)
			require.Len(t, rib.routes("198.51.100.0/24"), 1)

			rfc9494Down(gp, path.structured)

			rib.down()
			require.Empty(t, rib.routes("198.51.100.0/24"))
			assert.False(t, gp.state.peerActive(testPeer), "no LLST timer is armed for a zero Long-Lived Stale Time")
		})
	}
}

// TestRFC9494RestartTimeThenLongLivedStaleTime drops a session whose peer advertised a
// Restart Time of 1 second and a Long-Lived Stale Time of 1 second for IPv4 unicast, and
// watches the two periods run one after the other.
//
// VALIDATES: RFC 9494 Section 4.2: "if both are nonzero, then the procedures would be
// applied serially: first those of GR and then those of LLGR", and "For each AFI/SAFI for
// which it has received a nonzero Long-Lived Stale Time, the helper router MUST start a
// timer for that Long-Lived Stale Time." Before the Restart Time elapses, nothing is
// purged and LLGR_STALE is not attached; once it elapses the LLGR entry runs and the
// family's LLST timer is armed; the family is purged only when that timer fires.
// PREVENTS: an LLGR entry that never arms the timer (routes kept forever), and one that
// purges at entry instead of retaining.
//
// RFC requirement: RFC9494-4.2-1 positive -- the received route survives GR at level 1 and LLGR at level 2 before expiry removes it.
// RFC requirement: RFC9494-4.2-2 positive -- a nonzero received LLST bounds the second period; the route is present at LLGR entry and absent after its timer fires.
func TestRFC9494RestartTimeThenLongLivedStaleTime(t *testing.T) {
	gp, rib := newGRWithRealRIB(t)
	gp.peerCaps[testPeer] = testCap(1, famIPv4)
	gp.peerLLGRCaps[testPeer] = &llgrPeerCap{Families: []llgrCapFamily{
		{Family: family.IPv4Unicast, ForwardState: true, LLST: 1},
	}}
	declareLocalLLGR(gp, family.IPv4Unicast)
	rib.received("18c63364", grReceivedAttrs)
	rfc9494Down(gp, false)
	rib.down()
	rib.requireStale("198.51.100.0/24", 1)
	require.NotContains(t, rib.communities("198.51.100.0/24"), uint32(0xffff0006))

	require.Eventually(t, func() bool {
		routes := rib.routes("198.51.100.0/24")
		return len(routes) == 1 && routes[0]["stale-level"] == float64(2)
	}, 5*time.Second, 10*time.Millisecond, "the Restart Time elapsed and the retained route never entered LLGR")

	gp.state.mu.Lock()
	state := gp.state.peers[testPeer]
	require.NotNil(t, state, "the peer must still be retained at LLGR entry")
	_, armed := state.llgrFamilies[family.IPv4Unicast]
	gp.state.mu.Unlock()
	assert.True(t, armed, "a nonzero Long-Lived Stale Time arms the family's timer")
	require.Contains(t, rib.communities("198.51.100.0/24"), uint32(0xffff0006))

	require.Eventually(t, func() bool { return len(rib.routes("198.51.100.0/24")) == 0 },
		5*time.Second, 10*time.Millisecond, "the Long-Lived Stale Time elapsed and the received route was never purged")
}
