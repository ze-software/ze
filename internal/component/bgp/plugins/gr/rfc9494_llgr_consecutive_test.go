// Design: docs/guide/graceful-restart.md -- LLGR consecutive restarts and LLST timer persistence
// RFC: rfc/short/rfc9494.md -- Section 4.2, consecutive restarts, LLST timer immutability, and sync-expiry reset
// Related: gr_state.go -- onSessionDownPlan, onSessionReestablished, onEORReceived, handleLLSTExpired

package gr

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC9494ConsecutiveRestartPreservesLLGRStaleRoutes verifies that a
// consecutive session drop while in LLGR does not delete previously marked
// LLGR-stale routes before the LLST timer expires, while a conventional GR
// consecutive restart (without LLGR) still purges prior stale routes per
// RFC 4724 Section 4.2.
//
// RFC 9494 Section 4.2: "However, in the case of consecutive restarts, the
// previously marked stale routes MUST NOT be deleted before the timer for the
// Long-Lived Stale Time expires."
//
// RFC requirement: RFC9494-4.2-7 positive -- on a consecutive session drop while
// ipv4/unicast is in its LLGR period, grPlugin dispatches no purge-stale and no
// level-1 mark-stale downgrade for ipv4/unicast; the family remains retained
// until its original LLST timer expires.
// RFC requirement: RFC9494-4.2-7 negative -- once the LLST timer expires (and
// on a conventional GR consecutive restart with no LLGR), purge-stale IS
// dispatched for the family.
func TestRFC9494ConsecutiveRestartPreservesLLGRStaleRoutes(t *testing.T) {
	t.Run("llgr-preserves-until-expiry", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			gp, _ := newRecordedGRPlugin()
			src := grFixtureSource(t, testPeer, 701, 41)
			t.Cleanup(func() { gp.state.removePeer(src.grPeer) })

			var mu sync.Mutex
			var cmds []string
			gp.dispatchHook = func(command string, args ...string) {
				mu.Lock()
				cmds = append(cmds, command+" "+strings.Join(args, " "))
				mu.Unlock()
			}

			gp.peerCaps[701] = testCap(0, famIPv4)
			gp.peerLLGRCaps[701] = &llgrPeerCap{
				connectionID: 41,
				Families:     []llgrCapFamily{{Family: family.IPv4Unicast, ForwardState: true, LLST: 10}},
			}
			declareLocalLLGR(gp, src, family.IPv4Unicast)

			// First drop enters LLGR immediately (RestartTime=0, LLST=10s).
			rfc9494Down(t, gp, true)
			synctest.Wait()

			mu.Lock()
			cmds = nil
			mu.Unlock()

			// Advance 4s. The peer then reconnects on a new connection, 42, and that
			// connection drops before End-of-RIB and before LLST expires: a second
			// DOWN of connection 41 itself cannot happen. cmds was cleared above,
			// before connection 42 comes up, so the reconnect is checked too.
			<-time.After(4 * time.Second)
			synctest.Wait()
			src42 := grFixtureSource(t, testPeer, 701, 42)
			gp.peerCaps[701] = &grPeerCap{connectionID: 42, Families: []grCapFamily{famIPv4}}
			gp.peerLLGRCaps[701] = &llgrPeerCap{
				connectionID: 42,
				Families:     []llgrCapFamily{{Family: family.IPv4Unicast, ForwardState: true, LLST: 10}},
			}
			declareLocalLLGR(gp, src42, family.IPv4Unicast)
			require.NoError(t, gp.handleStructuredEvent(&rpc.StructuredEvent{
				PeerAddress: testPeer, PeerOwner: 701, ConnectionID: 42,
				EventType: rpc.EventKindState, State: rpc.SessionStateUp,
			}))
			synctest.Wait()
			require.NoError(t, gp.handleStructuredEvent(&rpc.StructuredEvent{
				PeerAddress: testPeer, PeerOwner: 701, ConnectionID: 42,
				EventType: rpc.EventKindState, State: rpc.SessionStateDown, Reason: rpc.ReasonTCPFailure,
			}))
			synctest.Wait()

			mu.Lock()
			duringDrop := slices.Clone(cmds)
			mu.Unlock()
			for _, c := range duringDrop {
				assert.NotContains(t, c, "purge-stale", "consecutive drop during LLGR must not purge LLGR-stale routes")
				// RFC 4724 Section 4.2 requires the routes connection 42 delivered
				// to be marked stale; the mark must be the LLGR level, never level 1.
				if strings.Contains(c, "mark-stale") {
					assert.Contains(t, c, "stale-level 2", "consecutive drop must not downgrade already-LLGR family to level 1")
				}
			}
			assert.True(t, gp.state.peerActive(src))
			// The retained cycle is addressed by the connection that ended last.
			assert.True(t, gp.state.familyRetained(src42, family.IPv4Unicast))

			// Negative: once the remaining 6s of the original 10s LLST elapse, purge-stale fires.
			<-time.After(6 * time.Second)
			synctest.Wait()
			mu.Lock()
			afterExpiry := slices.Clone(cmds)
			mu.Unlock()
			hasPurge := false
			for _, c := range afterExpiry {
				if strings.Contains(c, "purge-stale") && strings.Contains(c, "ipv4/unicast") {
					hasPurge = true
				}
			}
			assert.True(t, hasPurge, "LLST expiry must purge ipv4/unicast stale routes")
			assert.Contains(t, afterExpiry, "request bgp rib source purge-stale owner 701 connection 41 family ipv4/unicast",
				"LLST expiry must reach the stale routes connection 41 delivered")
			assert.False(t, gp.state.peerActive(src))
		})
	})

	t.Run("conventional-gr-consecutive-still-purges", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			mgr := grFixtureState(t, grFixtureSource(t, testPeer, 701, 41), nil)
			plan1 := mgr.onSessionDownPlan(grFixtureSource(t, testPeer, 701, 41), testCap(120, famIPv4), nil, false)
			require.True(t, plan1.activated)
			require.False(t, plan1.purgeAllPrevious)

			// The peer reconnects on connection 42, which drops before End-of-RIB.
			// That second drop, without LLGR, must purge the prior conventional-GR
			// stale routes.
			_, purged, _ := mgr.onSessionReestablished(grFixtureSource(t, testPeer, 701, 42), testCap(120, famIPv4), nil)
			require.Empty(t, purged)
			plan2 := mgr.onSessionDownPlan(grFixtureSource(t, testPeer, 701, 42), testCap(120, famIPv4), nil, false)
			require.True(t, plan2.activated)
			assert.True(t, plan2.purgeAllPrevious, "conventional GR consecutive restart must purge prior stale routes")
			mgr.removePeer(grFixtureSource(t, testPeer, 701, 41).grPeer)
		})
	})
}

// TestRFC9494LLSTTimerNotUpdatedUntilSynchronized verifies that a running LLST
// timer keeps its original deadline across consecutive session drops (never
// extended by a new LLGR capability's LLST), and only accepts a new LLST after
// the peer has re-established and synchronized via EOR.
//
// RFC 9494 Section 4.2: "If a Long-Lived Stale Time timer is running for routes
// with a given AFI/SAFI received from a peer, it MUST NOT be updated (other
// than by manual operator intervention) until the peer has established and
// synchronized a new session."
//
// RFC requirement: RFC9494-4.2-9 positive -- a consecutive session drop with a
// larger LLST (60s) does not extend a running 10s LLST timer: expiry still
// fires at the original 10s deadline.
// RFC requirement: RFC9494-4.2-9 negative -- after the peer re-establishes and
// sends EOR (synchronizes), a subsequent session drop DOES start a fresh timer
// using the new 60s LLST.
func TestRFC9494LLSTTimerNotUpdatedUntilSynchronized(t *testing.T) {
	t.Run("consecutive-drop-does-not-extend-running-llst", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var mu sync.Mutex
			var expired []family.Family
			getExpired := func() []family.Family {
				mu.Lock()
				defer mu.Unlock()
				return slices.Clone(expired)
			}
			mgr := grFixtureState(t, grFixtureSource(t, testPeer, 701, 41), nil)
			mgr.onLLGREnter = func(grSource, family.Family, uint32) {}
			mgr.onLLGRFamilyExpired = func(_ grSource, fam family.Family) {
				mu.Lock()
				expired = append(expired, fam)
				mu.Unlock()
			}

			llgr10 := &llgrPeerCap{Families: []llgrCapFamily{{Family: family.IPv4Unicast, ForwardState: true, LLST: 10}}}
			llgr60 := &llgrPeerCap{Families: []llgrCapFamily{{Family: family.IPv4Unicast, ForwardState: true, LLST: 60}}}

			// Enter LLGR with LLST=10s.
			require.True(t, mgr.onSessionDown(grFixtureSource(t, testPeer, 701, 41), testCap(0, famIPv4), llgr10, false))

			// Wait 6s, reconnect without EOR, then drop again advertising LLST=60s.
			<-time.After(6 * time.Second)
			synctest.Wait()
			_, purged, _ := mgr.onSessionReestablished(grFixtureSource(t, testPeer, 701, 42), testCap(0, famIPv4), llgr60)
			require.Empty(t, purged)
			require.True(t, mgr.onSessionDown(grFixtureSource(t, testPeer, 701, 42), testCap(0, famIPv4), llgr60, false))

			// At t=9.999s, original timer has not fired yet.
			<-time.After(4*time.Second - time.Millisecond)
			synctest.Wait()
			assert.Empty(t, getExpired(), "original 10s LLST has not elapsed yet")

			// At t=10s, original timer MUST fire (not extended to 66s).
			<-time.After(time.Millisecond)
			synctest.Wait()
			assert.Equal(t, []family.Family{family.IPv4Unicast}, getExpired(), "running LLST timer must not be updated before synchronization")
		})
	})

	t.Run("after-eor-sync-new-llst-applies", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var mu sync.Mutex
			var expired []family.Family
			getExpired := func() []family.Family {
				mu.Lock()
				defer mu.Unlock()
				return slices.Clone(expired)
			}
			mgr := grFixtureState(t, grFixtureSource(t, testPeer, 701, 41), nil)
			mgr.onLLGREnter = func(grSource, family.Family, uint32) {}
			mgr.onLLGRFamilyExpired = func(_ grSource, fam family.Family) {
				mu.Lock()
				expired = append(expired, fam)
				mu.Unlock()
			}

			llgr10 := &llgrPeerCap{Families: []llgrCapFamily{{Family: family.IPv4Unicast, ForwardState: true, LLST: 10}}}
			llgr60 := &llgrPeerCap{Families: []llgrCapFamily{{Family: family.IPv4Unicast, ForwardState: true, LLST: 60}}}

			require.True(t, mgr.onSessionDown(grFixtureSource(t, testPeer, 701, 41), testCap(0, famIPv4), llgr10, false))
			<-time.After(4 * time.Second)
			synctest.Wait()

			// Re-establish and synchronize via EOR.
			_, purged, _ := mgr.onSessionReestablished(grFixtureSource(t, testPeer, 701, 42), testCap(0, famIPv4), llgr60)
			require.Empty(t, purged)
			_, shouldPurge := mgr.onEORReceived(grFixtureSource(t, testPeer, 701, 42), family.IPv4Unicast)
			require.True(t, shouldPurge)
			require.False(t, mgr.peerActive(grFixtureSource(t, testPeer, 701, 42)))

			// New session drop after synchronization uses the new 60s LLST.
			require.True(t, mgr.onSessionDown(grFixtureSource(t, testPeer, 701, 42), testCap(0, famIPv4), llgr60, false))
			<-time.After(20 * time.Second)
			synctest.Wait()
			assert.Empty(t, getExpired(), "synchronized session must use its new 60s LLST")

			<-time.After(40 * time.Second)
			synctest.Wait()
			assert.Equal(t, []family.Family{family.IPv4Unicast}, getExpired(), "new 60s LLST expires at 60s")
		})
	})
}

// TestRFC9494ResetAfterSyncLLSTExpiryRemovesImmediately verifies that if an LLST
// timer expires while a re-established session is synchronizing (before EOR),
// a subsequent session reset before EOR immediately removes any remaining
// routes for that AFI/SAFI rather than retaining them again.
//
// RFC 9494 Section 4.2: "If the session subsequently resets prior to becoming
// synchronized, any remaining routes (for the AFI/SAFI whose LLST timer
// expired) MUST be removed immediately."
//
// RFC requirement: RFC9494-4.2-10 positive -- when ipv4/unicast's LLST timer
// expires during reconnect synchronization and the session resets before EOR,
// ipv4/unicast is not retained and is flagged for immediate removal.
// RFC requirement: RFC9494-4.2-10 negative -- a sibling family (ipv6/unicast)
// whose LLST timer has NOT expired during synchronization remains retained
// across the same reset.
func TestRFC9494ResetAfterSyncLLSTExpiryRemovesImmediately(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var syncMu sync.Mutex
		var syncExpired []family.Family
		mgr := grFixtureState(t, grFixtureSource(t, testPeer, 701, 41), nil)
		mgr.onLLGREnter = func(grSource, family.Family, uint32) {}
		mgr.onLLGRFamilyExpired = func(_ grSource, fam family.Family) {
			syncMu.Lock()
			syncExpired = append(syncExpired, fam)
			syncMu.Unlock()
		}

		// IPv4 has LLST=5s; IPv6 has LLST=60s.
		llgrBoth := &llgrPeerCap{Families: []llgrCapFamily{
			{Family: family.IPv4Unicast, ForwardState: true, LLST: 5},
			{Family: family.IPv6Unicast, ForwardState: true, LLST: 60},
		}}

		// Initial drop enters LLGR for both families.
		require.True(t, mgr.onSessionDown(grFixtureSource(t, testPeer, 701, 41), testCap(0, famIPv4, famIPv6), llgrBoth, false))

		// Re-establish at t=2s before either LLST expires.
		<-time.After(2 * time.Second)
		synctest.Wait()
		_, purged, _ := mgr.onSessionReestablished(grFixtureSource(t, testPeer, 701, 42), testCap(120, famIPv4, famIPv6), llgrBoth)
		require.Empty(t, purged)

		// Advance 3s to t=5s without sending EOR: IPv4 LLST expires during sync!
		<-time.After(3 * time.Second)
		synctest.Wait()
		syncMu.Lock()
		gotSyncExpired := slices.Clone(syncExpired)
		syncMu.Unlock()
		require.Equal(t, []family.Family{family.IPv4Unicast}, gotSyncExpired,
			"IPv4 LLST must continue running and expire during synchronization before EOR")

		// Session resets at t=6s BEFORE EOR arrives for IPv4 or IPv6.
		<-time.After(1 * time.Second)
		synctest.Wait()
		plan := mgr.onSessionDownPlan(grFixtureSource(t, testPeer, 701, 42), testCap(120, famIPv4, famIPv6), llgrBoth, false)
		require.True(t, plan.activated, "IPv6 still has a running LLST timer and stays retained")
		assert.Contains(t, plan.purgePreviousFams, family.IPv4Unicast,
			"IPv4 whose LLST expired during sync must be purged immediately on reset")
		assert.False(t, mgr.familyRetained(grFixtureSource(t, testPeer, 701, 42), family.IPv4Unicast),
			"IPv4 must NOT be retained for a new GR/LLGR cycle after sync LLST expiry")
		assert.True(t, mgr.familyRetained(grFixtureSource(t, testPeer, 701, 42), family.IPv6Unicast),
			"IPv6 whose LLST did NOT expire remains retained until its original 60s deadline")

		// Also verify through grPlugin dispatch for a single family whose LLST
		// expires during sync and then resets before EOR.
		gp, _ := newRecordedGRPlugin()
		src41 := grFixtureSource(t, testPeer, 701, 41)
		src42 := grFixtureSource(t, testPeer, 701, 42)
		t.Cleanup(func() { gp.state.removePeer(src41.grPeer) })

		var mu sync.Mutex
		var cmds []string
		gp.dispatchHook = func(command string, args ...string) {
			mu.Lock()
			cmds = append(cmds, command+" "+strings.Join(args, " "))
			mu.Unlock()
		}
		gp.peerCaps[701] = testCap(0, famIPv4)
		gp.peerLLGRCaps[701] = &llgrPeerCap{
			connectionID: 41,
			Families:     []llgrCapFamily{{Family: family.IPv4Unicast, ForwardState: true, LLST: 5}},
		}
		declareLocalLLGR(gp, src41, family.IPv4Unicast)
		rfc9494Down(t, gp, true)
		synctest.Wait()

		gp.peerCaps[701] = &grPeerCap{connectionID: 42, RestartTime: 120, Families: []grCapFamily{famIPv4}}
		gp.peerLLGRCaps[701] = &llgrPeerCap{connectionID: 42, Families: []llgrCapFamily{{Family: family.IPv4Unicast, ForwardState: true, LLST: 60}}}
		declareLocalLLGR(gp, src42, family.IPv4Unicast)
		require.NoError(t, gp.handleStructuredEvent(&rpc.StructuredEvent{
			PeerAddress: testPeer, PeerOwner: 701, ConnectionID: 42,
			EventType: rpc.EventKindState, State: rpc.SessionStateUp,
		}))

		// Advance past the 5s LLST while synchronizing (no EOR).
		<-time.After(5 * time.Second)
		synctest.Wait()
		mu.Lock()
		cmds = nil
		mu.Unlock()

		// Reset connection 42 before EOR: retention must NOT activate for IPv4,
		// and purge-stale must be dispatched immediately.
		require.NoError(t, gp.handleStructuredEvent(&rpc.StructuredEvent{
			PeerAddress: testPeer, PeerOwner: 701, ConnectionID: 42,
			EventType: rpc.EventKindState, State: rpc.SessionStateDown, Reason: rpc.ReasonTCPFailure,
		}))
		synctest.Wait()
		assert.False(t, gp.state.peerActive(src42), "reset after sync LLST expiry must not retain IPv4")
		mu.Lock()
		resetCmds := slices.Clone(cmds)
		mu.Unlock()
		hasPurge := false
		for _, c := range resetCmds {
			if strings.Contains(c, "purge-stale") && strings.Contains(c, "ipv4/unicast") {
				hasPurge = true
			}
		}
		assert.True(t, hasPurge, "reset after sync LLST expiry must immediately purge ipv4/unicast")

		mgr.removePeer(grFixtureSource(t, testPeer, 701, 42).grPeer)
	})
}
