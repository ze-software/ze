// Design: docs/architecture/behavior/fsm.md -- administrative start events.
// RFC naming: untagged -- supplementary duplicate-start lifecycle regression, not coverage of every start event.
package fsm

import "testing"

// TestRFC4271AdvancedStartEventsIgnored drives the public Event consumer and
// checks that duplicate starts neither tear down a session nor alter retry history.
// RFC 4271 Section 8.2.2: "The start events (Events 1, 3-7) are ignored in the
// OpenSent state." "Any start event (Events 1, 3-7) is ignored in the
// OpenConfirm state." "Any Start event (Events 1, 3-7) is ignored in the
// Established state."
// Each state retains its session and retry history after these events.
func TestRFC4271AdvancedStartEventsIgnored(t *testing.T) {
	for _, state := range []State{StateOpenSent, StateOpenConfirm, StateEstablished} {
		for _, event := range []Event{EventManualStart, EventAutomaticStartWithDampPeerOscillations} {
			t.Run(state.String()+"/"+event.String(), func(t *testing.T) {
				f, counter := crcFSM(t, state, 7)
				changes := 0
				f.SetCallback(func(_, _ State) { changes++ })
				if err := f.Event(event); err != nil {
					t.Errorf("start returned %v, want ignored", err)
				}
				if got := f.State(); got != state {
					t.Errorf("state = %v, want %v", got, state)
				}
				if got := counter.Load(); got != 7 {
					t.Errorf("retry counter = %d, want 7", got)
				}
				if changes != 0 {
					t.Errorf("state callbacks = %d, want none", changes)
				}
			})
		}
	}
}
