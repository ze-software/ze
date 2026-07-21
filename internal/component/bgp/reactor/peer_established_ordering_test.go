// Overview: peer_run.go — the FSM callback closure that sets/clears peer established state

package reactor

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
)

// TestPeerEstablishedTeardownOrdering pins AC-1 at the reactor's establish/teardown boundary:
// an OpenConfirm->Established transition racing an Established->Idle (or OpenConfirm->Idle)
// transition must never leave the peer's tracked "established" state true while the FSM has
// moved to Idle. The peer_run.go callback closure sets PeerStateEstablished on the
// to-Established arm and clears it on the from-Established arm; before the FSM transition
// queue those two callbacks could overlap and finish out of order, so a teardown callback
// could complete first and the later establish callback would mark a dead session
// Established.
//
// This mirrors that closure with a boolean the callback flips, driven directly on a real
// FSM through the same events the session read loop and hold timer fire, and asserts the
// tracked state agrees with the final FSM state every round.
//
// VALIDATES: AC-1 — a peer is never left established after a later Established->Idle.
// PREVENTS: an out-of-order establish callback marking a torn-down session established.
func TestPeerEstablishedTeardownOrdering(t *testing.T) {
	for round := range 300 {
		f := fsm.New()

		var mu sync.Mutex
		established := false
		f.SetCallback(func(from, to fsm.State) {
			mu.Lock()
			defer mu.Unlock()
			// Same arms as the peer_run.go closure's setState calls.
			if to == fsm.StateEstablished {
				established = true
			} else if from == fsm.StateEstablished {
				established = false
			}
		})

		// Drive to OpenConfirm so EventKeepaliveMsg -> Established is a real transition.
		require.NoError(t, f.Event(fsm.EventManualStart))            // Idle -> Connect
		require.NoError(t, f.Event(fsm.EventTCPConnectionConfirmed)) // Connect -> OpenSent
		require.NoError(t, f.Event(fsm.EventBGPOpen))                // OpenSent -> OpenConfirm

		var wg sync.WaitGroup
		wg.Go(func() { _ = f.Event(fsm.EventKeepaliveMsg) })     // read loop: -> Established
		wg.Go(func() { _ = f.Event(fsm.EventHoldTimerExpires) }) // hold timer: -> Idle
		wg.Wait()

		mu.Lock()
		agree := established == (f.State() == fsm.StateEstablished)
		mu.Unlock()
		require.True(t, agree,
			"round %d: tracked established=%v disagrees with FSM state %s (dead session left established)",
			round, established, f.State())
	}
}
