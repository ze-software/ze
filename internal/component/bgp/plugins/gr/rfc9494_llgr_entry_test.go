// Design: docs/guide/graceful-restart.md -- Long-Lived Graceful Restart, the helper's LLGR period
// RFC: rfc/short/rfc9494.md -- Section 4.2, Session Resets
// Overview: gr.go -- wireStateCallbacks and the session-down dispatch of both event paths
// Related: gr_state.go -- onSessionDown, enterLLGRLocked
// Related: rfc4724_retention_test.go -- newRecordedGRPlugin, the dispatch recorder

package gr

import (
	"slices"
	"strings"
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

// rfc9494Paths names the two session-down paths every test below runs on.
var rfc9494Paths = []struct {
	name       string
	structured bool
}{{"json", false}, {"structured", true}}

// TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod drops a session whose peer
// advertised Restart Time 0 and a Long-Lived Stale Time of 3600 seconds for IPv4 unicast,
// and reads every RIB command the plugin's production callbacks dispatch, in order.
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
// RFC requirement: RFC9494-4.2-1 positive -- with Restart Time 0 and a nonzero Long-Lived Stale Time, a TCP failure on either event path dispatches purge-stale, retain-routes and mark-stale <peer> 0 first, then the LLGR entry, and no purge-stale or release-routes for the peer after it.
// RFC requirement: RFC9494-4.2-4 positive -- the LLGR entry dispatches "request bgp rib attach-community <peer> ipv4/unicast ffff0006" for the retained family.
// RFC requirement: RFC9494-4.2-5 positive -- the LLGR entry dispatches "request bgp rib delete-with-community <peer> ipv4/unicast ffff0007", before LLGR_STALE is attached.
func TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod(t *testing.T) {
	ipv4 := family.IPv4Unicast.String()
	for _, path := range rfc9494Paths {
		t.Run(path.name, func(t *testing.T) {
			gp, rec := newRecordedGRPlugin()
			gp.peerCaps[testPeer] = testCap(0, famIPv4)
			gp.peerLLGRCaps[testPeer] = &llgrPeerCap{Families: []llgrCapFamily{
				{Family: family.IPv4Unicast, ForwardState: true, LLST: 3600},
			}}

			rfc9494Down(gp, path.structured)

			sent := rec.all()
			require.Len(t, sent, 7, "session-down sequence, LLGR entry, one readvertisement: %q", sent)
			assert.Equal(t, []string{
				"request bgp rib purge-stale " + testPeer,
				"request bgp rib retain-routes " + testPeer,
				"request bgp rib mark-stale " + testPeer + " 0",
				"request bgp rib delete-with-community " + testPeer + " " + ipv4 + " ffff0007",
				"request bgp rib attach-community " + testPeer + " " + ipv4 + " ffff0006",
				"request bgp rib mark-stale " + testPeer + " 0 2",
			}, sent[:6], "retain first, then enter the LLGR period")
			assert.True(t, strings.HasPrefix(sent[6], "clear bgp rib out "), "readvertise: %q", sent[6])
			assert.True(t, strings.HasSuffix(sent[6], " "+ipv4), "readvertise ipv4 only: %q", sent[6])
			assert.True(t, gp.state.peerActive(testPeer), "the routes are retained for the Long-Lived Stale Time")
		})
	}
}

// TestRFC9494BothTimesZeroRetainsNothing drops a session whose peer advertised Restart
// Time 0 and listed IPv4 unicast in its LLGR Capability with a Long-Lived Stale Time of 0.
//
// VALIDATES: RFC 9494 Section 4.2: "If both are zero, none of these procedures would
// apply, only those of the base BGP specification [RFC4271]". The retention interval is
// zero, so the last command for the peer releases its routes.
// PREVENTS: retain-routes and mark-stale <peer> 0 (no RIB expiry timer) landing after the
// release, which left the routes retained as stale with no timer to remove them.
//
// RFC requirement: RFC9494-4.2-1 negative -- with both times zero, on either event path, the routes are not retained: release-routes is the last command dispatched and no retain-routes follows it.
// RFC requirement: RFC9494-4.2-2 negative -- a Long-Lived Stale Time of zero arms no timer: the family is purged at once and the peer leaves the LLGR state.
func TestRFC9494BothTimesZeroRetainsNothing(t *testing.T) {
	for _, path := range rfc9494Paths {
		t.Run(path.name, func(t *testing.T) {
			gp, rec := newRecordedGRPlugin()
			gp.peerCaps[testPeer] = testCap(0, famIPv4)
			gp.peerLLGRCaps[testPeer] = &llgrPeerCap{Families: []llgrCapFamily{
				{Family: family.IPv4Unicast, ForwardState: true, LLST: 0},
			}}

			rfc9494Down(gp, path.structured)

			sent := rec.all()
			require.NotEmpty(t, sent)
			assert.Equal(t, "request bgp rib release-routes "+testPeer, sent[len(sent)-1],
				"both times zero: the peer's routes are released, not retained: %q", sent)
			assert.Contains(t, sent, "request bgp rib purge-stale "+testPeer+" "+family.IPv4Unicast.String())
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
// RFC requirement: RFC9494-4.2-1 positive -- across the Restart Time and into the LLGR period, no purge-stale for the family and no release-routes is dispatched until the Long-Lived Stale Time fires.
// RFC requirement: RFC9494-4.2-2 positive -- the LLGR entry arms an IPv4 unicast LLST timer, and the family is purged only after it fires, not at entry.
func TestRFC9494RestartTimeThenLongLivedStaleTime(t *testing.T) {
	gp, rec := newRecordedGRPlugin()
	gp.peerCaps[testPeer] = testCap(1, famIPv4)
	gp.peerLLGRCaps[testPeer] = &llgrPeerCap{Families: []llgrCapFamily{
		{Family: family.IPv4Unicast, ForwardState: true, LLST: 1},
	}}
	ipv4 := family.IPv4Unicast.String()
	attach := "request bgp rib attach-community " + testPeer + " " + ipv4 + " ffff0006"
	purge := "request bgp rib purge-stale " + testPeer + " " + ipv4
	release := "request bgp rib release-routes " + testPeer

	rfc9494Down(gp, false)
	assert.Equal(t, []string{
		"request bgp rib purge-stale " + testPeer,
		"request bgp rib retain-routes " + testPeer,
		"request bgp rib mark-stale " + testPeer + " 1",
	}, rec.all(), "the GR period starts: routes retained, LLGR not entered")

	require.Eventually(t, func() bool { return slices.Contains(rec.all(), attach) },
		5*time.Second, 10*time.Millisecond, "the Restart Time elapsed and the LLGR period never began")

	gp.state.mu.Lock()
	state := gp.state.peers[testPeer]
	require.NotNil(t, state, "the peer must still be retained at LLGR entry")
	_, armed := state.llgrFamilies[family.IPv4Unicast]
	gp.state.mu.Unlock()
	assert.True(t, armed, "a nonzero Long-Lived Stale Time arms the family's timer")
	assert.NotContains(t, rec.all(), purge, "the family is retained when the LLGR period begins")
	assert.NotContains(t, rec.all(), release, "the peer is retained when the LLGR period begins")

	require.Eventually(t, func() bool { return slices.Contains(rec.all(), purge) },
		5*time.Second, 10*time.Millisecond, "the Long-Lived Stale Time elapsed and the family was never purged")
}
