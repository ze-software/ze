// Design: docs/guide/graceful-restart.md -- consecutive restarts during the LLGR period
// RFC: rfc/short/rfc9494.md -- Section 4.2, Session Resets
// Overview: gr_consecutive.go -- the earlier connections a retained cycle still addresses
// Related: real_rib_test.go -- the registered RIB engine these tests read routes from
// Related: rfc9494_llgr_consecutive_test.go -- the same restarts against the state manager alone

package gr

import (
	"encoding/json"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// The three routes of the consecutive-restart sequence, named for the
// connection that last delivered each one when connection 42 goes down.
const (
	llgrRouteOfFirst  = "198.51.100.0/24" // connection 41 only: never refreshed
	llgrRouteOfSecond = "198.51.101.0/24" // refreshed by connection 42
	llgrRouteOfEvery  = "198.51.102.0/24" // refreshed by 42, and by 43 where one comes up
)

var llgrConsecutiveRoutes = []string{llgrRouteOfFirst, llgrRouteOfSecond, llgrRouteOfEvery}

// requireLLGRStale reads one route from the real RIB and requires the two marks
// of a route retained in the LLGR period: stale level 2 and LLGR_STALE.
func requireLLGRStale(t testing.TB, rib *realGRRIB, prefix string) {
	t.Helper()
	rib.requireStale(prefix, 2)
	require.Contains(t, rib.communities(prefix), uint32(0xffff0006), prefix)
}

// requireFresh reads one route from the real RIB and requires that it is not
// stale: the RIB states no stale level for a fresh route.
func requireFresh(t testing.TB, rib *realGRRIB, prefix string) {
	t.Helper()
	routes := rib.routes(prefix)
	require.Len(t, routes, 1, prefix)
	require.NotContains(t, routes[0], "stale-level", prefix)
	require.NotContains(t, rib.communities(prefix), uint32(0xffff0006), prefix)
}

// rfc9494Reconnect brings successor up on both sides: the GR plugin learns its
// capabilities (Restart Time 0, LLST 10 s for IPv4 unicast, both F bits set)
// and its UP, then the real RIB records the new physical connection.
func rfc9494Reconnect(t *testing.T, gp *grPlugin, rib *realGRRIB, successor grSource, session uint64) {
	t.Helper()
	cap := testCap(0, famIPv4)
	cap.connectionID = successor.connectionID
	gp.peerCaps[successor.owner] = cap
	gp.peerLLGRCaps[successor.owner] = &llgrPeerCap{connectionID: successor.connectionID, Families: []llgrCapFamily{
		{Family: family.IPv4Unicast, ForwardState: true, LLST: 10},
	}}
	declareLocalLLGR(gp, successor, family.IPv4Unicast)
	gp.handleStateEvent(successor, map[string]any{"state": "up"})
	rib.reconnect(successor, session)
}

// rfc9494SecondRestartDuringLLGR drives the real sequence through the GR
// plugin's production handlers and the registered RIB engine, and returns at
// the moment connection 42 is down. The caller MUST run it inside a synctest
// bubble.
//
//	t=0   connection 41 delivered three routes and goes down: Restart Time 0,
//	      so IPv4 unicast enters LLGR at once with a 10 s LLST
//	t=4s  connection 42 comes up and refreshes two of the three routes
//	t=4s  connection 42 goes down before End-of-RIB
//
// The returned deadline is the original LLST deadline, t=10s.
func rfc9494SecondRestartDuringLLGR(t *testing.T) (*grPlugin, *realGRRIB, time.Time) {
	t.Helper()
	gp, rib := newGRWithRealRIB(t)
	first := rib.source
	gp.peerCaps[first.owner] = testCap(0, famIPv4)
	gp.peerLLGRCaps[first.owner] = &llgrPeerCap{connectionID: first.connectionID, Families: []llgrCapFamily{
		{Family: family.IPv4Unicast, ForwardState: true, LLST: 10},
	}}
	declareLocalLLGR(gp, first, family.IPv4Unicast)
	rib.received("18c6336418c6336518c63366", grReceivedAttrs)

	deadline := time.Now().Add(10 * time.Second)
	rfc9494Down(t, gp, true)
	synctest.Wait()
	rib.down()
	for _, prefix := range llgrConsecutiveRoutes {
		requireLLGRStale(t, rib, prefix)
	}

	<-time.After(4 * time.Second)
	synctest.Wait()
	second := grFixtureSource(t, testPeer, first.owner, 42)
	rfc9494Reconnect(t, gp, rib, second, 142)
	rib.received("18c6336518c63366", grReceivedAttrs)
	// The refresh is partial: the first route still belongs to connection 41.
	requireLLGRStale(t, rib, llgrRouteOfFirst)
	requireFresh(t, rib, llgrRouteOfSecond)
	requireFresh(t, rib, llgrRouteOfEvery)

	gp.handleStateEvent(second, map[string]any{"state": "down", "reason": "tcp-failure"})
	synctest.Wait()
	rib.down()
	return gp, rib, deadline
}

// TestRFC9494ConsecutiveRestartKeepsTheEarlierConnectionsStaleRoute asks one
// question of the real RIB: after connection 42's DOWN, is the route that only
// connection 41 delivered still there?
//
// RFC 9494 Section 4.2: "However, in the case of consecutive restarts, the
// previously marked stale routes MUST NOT be deleted before the timer for the
// Long-Lived Stale Time expires."
//
// PREVENTS: the RIB refusing the second retention because the first connection
// still owns routes, which left the peer unretained at its DOWN and released
// every route it owned.
//
// RFC requirement: RFC9494-4.2-7 positive -- the route delivered by connection 41 and not refreshed by connection 42 is still in the real RIB, at stale level 2 with LLGR_STALE, after connection 42 went down before End-of-RIB and before the LLST deadline.
func TestRFC9494ConsecutiveRestartKeepsTheEarlierConnectionsStaleRoute(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gp, rib, _ := rfc9494SecondRestartDuringLLGR(t)
		requireLLGRStale(t, rib, llgrRouteOfFirst)
		assert.True(t, gp.state.familyRetained(rib.source, family.IPv4Unicast),
			"IPv4 unicast stays retained under the connection that ended last")
	})
}

// TestRFC9494ConsecutiveRestartMarksTheLaterConnectionsRoutes asks whether the
// routes connection 42 delivered, for a family whose LLGR period was already
// running, are marked when connection 42 goes down.
//
// RFC 4724 Section 4.2: "it MUST retain the routes received from the peer for
// all the address families that were previously received in the Graceful
// Restart Capability and MUST mark them as stale routing information."
// RFC 9494 Section 4.2: "The helper router MUST attach the LLGR_STALE community
// to the stale routes being retained."
//
// PREVENTS: retaining those routes fresh, with no community, so that the purge
// at the LLST deadline, which deletes stale routes only, left them in place.
//
// RFC requirement: RFC9494-4.2-4 positive -- the routes connection 42 delivered for a family already in its LLGR period carry LLGR_STALE at stale level 2 in the real RIB after connection 42's DOWN.
func TestRFC9494ConsecutiveRestartMarksTheLaterConnectionsRoutes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, rib, _ := rfc9494SecondRestartDuringLLGR(t)
		requireLLGRStale(t, rib, llgrRouteOfSecond)
		requireLLGRStale(t, rib, llgrRouteOfEvery)
	})
}

// TestRFC9494ConsecutiveRestartExpiryDeletesEveryConnectionsRoutes lets the
// original LLST timer run out after the second restart and reads the real RIB
// one nanosecond before the deadline and at it.
//
// RFC 9494 Section 4.2: "If the timer for the Long-Lived Stale Time for a given
// AFI/SAFI expires before the session is re-established, the helper MUST delete
// all stale routes of that AFI/SAFI from the neighbor that it is retaining."
//
// PREVENTS: the purge addressed to connection 42 alone, which matched no route
// of connection 41 and left that connection's stale route in the RIB for ever.
//
// RFC requirement: RFC9494-4.2-3 positive -- at the original LLST deadline, after a consecutive restart, the stale routes of both connections 41 and 42 are deleted from the real RIB and the peer leaves the LLGR state.
// RFC requirement: RFC9494-4.2-3 negative -- one nanosecond before that deadline all three routes are still in the real RIB at stale level 2 with LLGR_STALE.
func TestRFC9494ConsecutiveRestartExpiryDeletesEveryConnectionsRoutes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gp, rib, deadline := rfc9494SecondRestartDuringLLGR(t)

		<-time.After(time.Until(deadline) - time.Nanosecond)
		synctest.Wait()
		for _, prefix := range llgrConsecutiveRoutes {
			requireLLGRStale(t, rib, prefix)
		}

		<-time.After(time.Nanosecond)
		synctest.Wait()
		for _, prefix := range llgrConsecutiveRoutes {
			require.Empty(t, rib.routes(prefix), prefix)
		}
		assert.False(t, gp.state.peerActive(rib.source), "the last LLST deadline releases the peer")
	})
}

// TestRFC9494ConsecutiveRestartEORDeletesEveryConnectionsStaleRoutes brings a
// third connection up after the second restart. It refreshes one route and
// sends End-of-RIB before the LLST deadline.
//
// VALIDATES: RFC 4724 Section 4.2: "Once the End-of-RIB marker for an address
// family is received from the peer, it MUST immediately remove any routes from
// the peer that are still marked as stale for that address family." The stale
// route of connection 41 and the stale route of connection 42 are both "from
// the peer"; the route connection 43 refreshed is not stale and stays.
// PREVENTS: the End-of-RIB purge addressed to one connection only.
func TestRFC9494ConsecutiveRestartEORDeletesEveryConnectionsStaleRoutes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gp, rib, _ := rfc9494SecondRestartDuringLLGR(t)

		<-time.After(time.Second)
		synctest.Wait()
		third := grFixtureSource(t, testPeer, rib.source.owner, 43)
		rfc9494Reconnect(t, gp, rib, third, 143)
		rib.received("18c63366", grReceivedAttrs)
		requireLLGRStale(t, rib, llgrRouteOfFirst)
		requireLLGRStale(t, rib, llgrRouteOfSecond)

		gp.handleEOREvent(third, map[string]any{"eor": map[string]any{"family": family.IPv4Unicast.String()}})
		synctest.Wait()

		require.Empty(t, rib.routes(llgrRouteOfFirst), "the stale route of connection 41")
		require.Empty(t, rib.routes(llgrRouteOfSecond), "the stale route of connection 42")
		requireFresh(t, rib, llgrRouteOfEvery)
		assert.False(t, gp.state.peerActive(third), "End-of-RIB for the only retained family completes the cycle")
	})
}

// TestRFC9494ConsecutiveRestartCompletesARevokedLLGREntry reconnects while the
// first connection's LLGR entry is still being dispatched. The reconnect
// revokes the rest of that entry, so the route connection 41 delivered stays
// stale without LLGR_STALE although its family's LLST timer runs. Connection
// 42 then goes down before End-of-RIB.
//
//	t=0    connection 41 goes down: Restart Time 1 s, LLST 10 s
//	t=1s   IPv4 unicast enters LLGR; the entry is held at its first command
//	t=1s   connection 42 comes up, the held entry returns and sends no more
//	t=1s   connection 42 refreshes one route and goes down
//	t=11s  the LLST deadline
//
// VALIDATES: RFC 9494 Section 4.2: "The helper router MUST attach the
// LLGR_STALE community to the stale routes being retained." At the second DOWN
// the route of connection 41 gains LLGR_STALE and stale level 2, and it is
// deleted with the others at the LLST deadline.
// PREVENTS: decorating the latest connection's routes only, which left the
// earlier connection's route at stale level 1 with no community for the whole
// Long-Lived Stale Time.
func TestRFC9494ConsecutiveRestartCompletesARevokedLLGREntry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gp, rib := newGRWithRealRIB(t)
		first := rib.source
		gp.peerCaps[first.owner] = testCap(1, famIPv4)
		gp.peerLLGRCaps[first.owner] = &llgrPeerCap{connectionID: first.connectionID, Families: []llgrCapFamily{
			{Family: family.IPv4Unicast, ForwardState: true, LLST: 10},
		}}
		declareLocalLLGR(gp, first, family.IPv4Unicast)
		rib.received("18c6336418c63365", grReceivedAttrs)

		// Hold the entry's first command, the NO_LLGR deletion, before it reaches
		// the RIB. Every other command, and every later deletion, passes.
		entered, release := make(chan struct{}), make(chan struct{})
		var holdEntryOnce, releaseOnce sync.Once
		t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
		gp.answerHook = func(command string, args ...string) json.RawMessage {
			if command == "request bgp rib source delete-with-community" {
				holdEntryOnce.Do(func() {
					close(entered)
					<-release
				})
			}
			return rib.command(command, args...)
		}

		deadline := time.Now().Add(11 * time.Second)
		rfc9494Down(t, gp, true)
		synctest.Wait()
		rib.down()
		rib.requireStale(llgrRouteOfFirst, 1)

		<-entered
		second := grFixtureSource(t, testPeer, first.owner, 42)
		rfc9494Reconnect(t, gp, rib, second, 142)
		releaseOnce.Do(func() { close(release) })
		synctest.Wait()
		// The revoked entry left the earlier route stale and undecorated.
		rib.requireStale(llgrRouteOfFirst, 1)
		require.NotContains(t, rib.communities(llgrRouteOfFirst), uint32(0xffff0006))

		rib.received("18c63365", grReceivedAttrs)
		gp.handleStateEvent(second, map[string]any{"state": "down", "reason": "tcp-failure"})
		synctest.Wait()
		rib.down()
		requireLLGRStale(t, rib, llgrRouteOfFirst)
		requireLLGRStale(t, rib, llgrRouteOfSecond)

		<-time.After(time.Until(deadline))
		synctest.Wait()
		require.Empty(t, rib.routes(llgrRouteOfFirst), "the stale route of connection 41")
		require.Empty(t, rib.routes(llgrRouteOfSecond), "the stale route of connection 42")
		assert.False(t, gp.state.peerActive(rib.source), "the LLST deadline releases the peer")
	})
}
