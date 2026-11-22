// Design: docs/guide/graceful-restart.md -- a reconnect that overtakes an LLGR entry
// RFC: rfc/short/rfc9494.md -- Section 4.2, Session Resets
// Overview: gr_state.go -- llgrFamilyActive, the check each entry command passes
// Related: gr.go -- wireStateCallbacks, the LLGR entry command sequence
// Related: real_rib_test.go -- the registered RIB engine these tests read routes from
// Related: rfc9494_llgr_consecutive_rib_test.go -- requireLLGRStale, requireFresh, rfc9494Reconnect

package gr

import (
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The three routes connection 41 delivers before the overtaken LLGR entry.
const (
	llgrEntryRouteStale     = "198.51.100.0/24" // never refreshed: stays stale
	llgrEntryRouteRefreshed = "198.51.101.0/24" // sent again by connection 42 during the entry
	llgrEntryRouteNoLLGR    = "198.51.102.0/24" // carries NO_LLGR, never refreshed
)

// rfc9494ReconnectDuringLLGREntry drives the real sequence through the GR
// plugin's production handlers and the registered RIB engine, and returns once
// the overtaken entry has finished. The caller MUST run it inside a synctest
// bubble.
//
//	t=0    connection 41 delivered three routes and goes down: Restart Time 1 s,
//	       LLST 10 s
//	t=1s   IPv4 unicast enters LLGR; the entry is held before its first command,
//	       the NO_LLGR deletion, reaches the RIB
//	t=1s   connection 42 comes up and sends one of the three routes again
//	t=1s   the held entry is released and runs to its end
//
// The returned deadline is the LLST deadline, t=11s.
func rfc9494ReconnectDuringLLGREntry(t *testing.T) (*grPlugin, *realGRRIB, time.Time) {
	t.Helper()
	gp, rib := newGRWithRealRIB(t)
	first := rib.source
	gp.peerCaps[first.owner] = testCap(1, famIPv4)
	gp.peerLLGRCaps[first.owner] = &llgrPeerCap{connectionID: first.connectionID, Families: []llgrCapFamily{
		{Family: family.IPv4Unicast, ForwardState: true, LLST: 10},
	}}
	declareLocalLLGR(gp, first, family.IPv4Unicast)
	rib.received("18c6336418c63365", grReceivedAttrs)
	rib.received("18c63366", grReceivedAttrs+"c00804ffff0007")

	// Hold the entry's first command before it reaches the RIB. Every other
	// command, and every later deletion, passes.
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
	// The Restart Time period: stale, and not yet in the LLGR period.
	for _, prefix := range []string{llgrEntryRouteStale, llgrEntryRouteRefreshed, llgrEntryRouteNoLLGR} {
		rib.requireStale(prefix, 1)
	}

	<-entered
	second := grFixtureSource(t, testPeer, first.owner, 42)
	rfc9494Reconnect(t, gp, rib, second, 142)
	rib.received("18c63365", grReceivedAttrs)
	requireFresh(t, rib, llgrEntryRouteRefreshed)
	// The entry is still held: nothing has been applied to the stale routes.
	rib.requireStale(llgrEntryRouteStale, 1)
	require.Len(t, rib.routes(llgrEntryRouteNoLLGR), 1)

	releaseOnce.Do(func() { close(release) })
	synctest.Wait()
	return gp, rib, deadline
}

// TestRFC9494ReconnectDuringLLGREntryStillAttachesLLGRStale asks one question
// of the real RIB: when the peer's new session comes up while the LLGR entry
// is still being applied, does the retained stale route end at stale level 2
// with LLGR_STALE?
//
// RFC 9494 Section 4.2: "Once the Restart Time period ends (including the case
// in which the Restart Time is zero), the LLGR period is said to have begun and
// the following procedures MUST be performed:" and "The helper router MUST
// attach the LLGR_STALE community to the stale routes being retained."
//
// PREVENTS: a reconnect stopping the entry after its first command, which left
// the route at stale level 1 with no community until End-of-RIB, LLST expiry
// or the next session-down.
//
// RFC requirement: RFC9494-4.2-4 positive -- a route connection 41 delivered, which connection 42 did not send again, is in the real RIB at stale level 2 with LLGR_STALE after connection 42 came up while the LLGR entry was held before its first command.
func TestRFC9494ReconnectDuringLLGREntryStillAttachesLLGRStale(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gp, rib, _ := rfc9494ReconnectDuringLLGREntry(t)
		requireLLGRStale(t, rib, llgrEntryRouteStale)
		first := grFixtureSource(t, testPeer, rib.source.owner, 41)
		assert.True(t, gp.state.familyRetained(first, family.IPv4Unicast),
			"IPv4 unicast stays retained under connection 41 until End-of-RIB")
	})
}

// TestRFC9494ReconnectDuringLLGREntryStillRemovesNoLLGRRoutes asks whether the
// stale route that carries NO_LLGR is removed when the peer's new session
// comes up while the LLGR entry is still being applied.
//
// RFC 9494 Section 4.2: "If any of the routes from the peer have been marked
// with the NO_LLGR community, either as sent by the peer or as the result of a
// configured policy, they MUST NOT be retained and MUST be removed as per the
// normal operation of [RFC4271]."
//
// PREVENTS: a reconnect stopping the entry while its NO_LLGR deletion is in
// flight, so that a NO_LLGR route stays retained through the LLGR period.
//
// RFC requirement: RFC9494-4.2-5 positive -- a NO_LLGR route connection 41 delivered, which connection 42 did not send again, is gone from the real RIB after connection 42 came up while the LLGR entry's NO_LLGR deletion was held before it reached the RIB.
func TestRFC9494ReconnectDuringLLGREntryStillRemovesNoLLGRRoutes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, rib, _ := rfc9494ReconnectDuringLLGREntry(t)
		require.Empty(t, rib.routes(llgrEntryRouteNoLLGR), "the NO_LLGR route of connection 41")
	})
}

// TestRFC9494ReconnectDuringLLGREntryLeavesTheNewConnectionsRouteFresh asks
// whether the entry, finishing after the reconnect, touches a route the new
// connection has already sent.
//
// RFC 9494 Section 4.2: "The helper router MUST attach the LLGR_STALE community
// to the stale routes being retained." A route connection 42 sent is neither
// stale nor retained.
//
// PREVENTS: the entry's commands reaching the peer's routes whatever
// connection delivered them, which would depreference a route the peer has
// just refreshed and advertise it with LLGR_STALE.
//
// RFC requirement: RFC9494-4.2-4 negative -- the route connection 42 sent again while the LLGR entry of connection 41 was held is in the real RIB with no stale level and without LLGR_STALE after that entry ran to its end.
func TestRFC9494ReconnectDuringLLGREntryLeavesTheNewConnectionsRouteFresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, rib, _ := rfc9494ReconnectDuringLLGREntry(t)
		requireFresh(t, rib, llgrEntryRouteRefreshed)
	})
}

// TestRFC9494ReconnectDuringLLGREntryKeepsTheLLSTDeadline lets the LLST timer
// run out during the synchronization that follows the overtaken entry, and
// reads the real RIB one nanosecond before the original deadline and at it.
//
// VALIDATES: RFC 9494 Section 4.2: "The timer is neither stopped nor updated
// until the EoR marker is received for the relevant AFI/SAFI from the peer."
// and "If the timer expires during synchronization with the peer, any stale
// routes that the peer has not refreshed are removed." Finishing the entry
// after the reconnect neither restarts the timer nor moves its deadline.
// PREVENTS: the completed entry re-arming the Long-Lived Stale Time, or the
// RIB's own safety timer from the session-down marking deleting the route
// early.
func TestRFC9494ReconnectDuringLLGREntryKeepsTheLLSTDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, rib, deadline := rfc9494ReconnectDuringLLGREntry(t)

		<-time.After(time.Until(deadline) - time.Nanosecond)
		synctest.Wait()
		requireLLGRStale(t, rib, llgrEntryRouteStale)

		<-time.After(time.Nanosecond)
		synctest.Wait()
		require.Empty(t, rib.routes(llgrEntryRouteStale), "the stale route the peer did not refresh")
		requireFresh(t, rib, llgrEntryRouteRefreshed)
	})
}

// TestRFC9494ReconnectDuringLLGREntrySendsTheRestToTheOriginalConnection holds
// the real timer callback at the entry's first RIB command, reconnects, and
// reads the commands the entry sends once released.
//
// VALIDATES: RFC 9494 Section 4.2: "The helper router MUST attach the
// LLGR_STALE community to the stale routes being retained. Note that this
// requirement implies that the routes would need to be readvertised in order
// to disseminate the modified community." The entry sends the attachment, the
// level-2 marking and the readvertisement after the reconnect, each source
// command to connection 2346, and leaves the family's LLST timer the one that
// was started.
// PREVENTS: an entry command addressed to the successor connection, whose
// routes are fresh, and a second LLST timer.
func TestRFC9494ReconnectDuringLLGREntrySendsTheRestToTheOriginalConnection(t *testing.T) {
	old := grFixtureSource(t, "192.0.2.248", 1346, 2346)
	fresh := grFixtureSource(t, old.peer(), old.owner, 2347)
	gp := &grPlugin{peerCaps: make(map[uint64]*grPeerCap), peerLLGRCaps: make(map[uint64]*llgrPeerCap)}
	gp.wireStateCallbacks()
	t.Cleanup(func() { gp.state.removePeer(old.grPeer) })
	gp.emitHook = func(string) {}
	entered, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce, holdEntryOnce sync.Once
	var mu sync.Mutex
	var commands [][]string
	gp.dispatchHook = func(command string, args ...string) {
		mu.Lock()
		commands = append(commands, append([]string{command}, args...))
		mu.Unlock()
		if command == "request bgp rib source delete-with-community" {
			holdEntryOnce.Do(func() {
				close(entered)
				<-release
			})
		}
	}
	grConnectionOpen(t, gp, old, true)
	grConnectionState(t, gp, old, true, rpc.SessionStateUp)
	grConnectionState(t, gp, old, true, rpc.SessionStateDown)
	gp.state.mu.Lock()
	captured := gp.state.peers[old.owner]
	gp.state.mu.Unlock()
	require.NotNil(t, captured)
	// Stop the long real timer before driving its exact captured callback, so
	// this deterministic test does not retain an unjoined timer after return.
	require.NotNil(t, captured.restartTimer)
	captured.restartTimer.Stop()
	// One captured timer lifecycle; cleanup always releases and joins it.
	go func() { defer close(joined); gp.state.handleTimerExpired(old, captured) }()

	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); <-joined })
	select {
	case <-entered:
	case <-t.Context().Done():
		t.Fatal("LLGR entry did not reach its first RPC")
	}
	gp.state.mu.Lock()
	started := captured.llgrFamilies[family.IPv4Unicast]
	gp.state.mu.Unlock()
	require.NotNil(t, started, "the LLGR period has begun: the family holds its LLST timer")

	grConnectionOpen(t, gp, fresh, true)
	grConnectionState(t, gp, fresh, true, rpc.SessionStateUp)
	mu.Lock()
	before := len(commands)
	mu.Unlock()
	releaseOnce.Do(func() { close(release) })
	select {
	case <-joined:
	case <-t.Context().Done():
		t.Fatal("LLGR entry did not run to its end")
	}

	owner := strconv.FormatUint(old.owner, 10)
	original := strconv.FormatUint(old.connectionID, 10)
	mu.Lock()
	rest := commands[before:]
	mu.Unlock()
	require.Equal(t, [][]string{
		{"request bgp rib source attach-community", "owner", owner, "connection", original, "family", "ipv4/unicast", "community", "ffff0006"},
		{"request bgp rib source mark-stale", "owner", owner, "connection", original, "restart-time", "0", "stale-level", "2", "family", "ipv4/unicast"},
		{"clear bgp rib out-excluding-owner", "owner", owner, "family", "ipv4/unicast"},
	}, rest, "the entry sends its remaining commands, in order, to the original connection")

	gp.state.mu.Lock()
	running := captured.llgrFamilies[family.IPv4Unicast]
	gp.state.mu.Unlock()
	require.Same(t, started, running, "the LLST timer is the one the entry started")
	require.Equal(t, []family.Family{family.IPv4Unicast}, gp.state.retainedFamilies(old))
}
