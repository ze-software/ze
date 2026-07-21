package fsm

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFSMCallbackOrderingRace reproduces the FSM state-change callback overlap/reorder bug.
// Before the fix, change() released the FSM lock around the callback, so two concurrent
// Event callers could run their transition callbacks at the same time and finish out of
// order (e.g. a to-Established callback completing AFTER a teardown callback, marking a dead
// session Established). -race does not flag this — each shared field is individually atomic
// or locked — so this is an ordering-assertion reproduction, not a -race one.
//
// The instrumented callback records an in-flight counter (to detect overlap) and the
// (from,to) chain (to detect reorder). After the fix, the per-FSM FIFO transition queue
// guarantees callbacks never overlap (maxInFlight == 1) and apply in transition order (each
// callback's from equals the previous callback's to).
//
// VALIDATES: AC-1 — callbacks never overlap and complete in transition order under
// concurrent Events.
// PREVENTS: two transition callbacks running concurrently / finishing out of order.
func TestFSMCallbackOrderingRace(t *testing.T) {
	f := New()

	var mu sync.Mutex
	inFlight := 0
	maxInFlight := 0
	var chain []transition

	f.SetCallback(func(from, to State) {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		chain = append(chain, transition{from: from, to: to})
		mu.Unlock()

		// Hold the "callback in progress" window so any overlapping callback is observable
		// by the in-flight counter.
		time.Sleep(150 * time.Microsecond)

		mu.Lock()
		inFlight--
		mu.Unlock()
	})

	const goroutines = 8
	const iters = 40
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range iters {
				_ = f.Event(EventManualStart) // Idle->Connect (state change) or ignored
				_ = f.Event(EventManualStop)  // Connect->Idle (state change) or ignored
			}
		})
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	require.Equal(t, 1, maxInFlight, "state-change callbacks must never overlap")

	// The transition chain must be consistent: each callback's from state is the previous
	// callback's to state. A reordered callback would break this link.
	for i := 1; i < len(chain); i++ {
		require.Equal(t, chain[i-1].to, chain[i].from,
			"callback %d ran out of transition order: prev %s->%s then %s->%s",
			i, chain[i-1].from, chain[i-1].to, chain[i].from, chain[i].to)
	}
	require.NotEmpty(t, chain, "the concurrent Events must have produced state changes")
}
