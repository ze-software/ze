// VALIDATES: the BGP FSM never panics and never lands in an invalid state, no
// matter what sequence of events (including unknown/out-of-range event numbers)
// is fed to it from Idle, in both active and passive mode.
// PREVENTS: a handler transitioning to a bogus state, corrupting state on an
// unhandled event, or panicking (e.g. nil timer deref) on some event ordering.

package fsm

import (
	"errors"
	"testing"
)

// validState reports whether s is one of the six RFC 4271 §8.2.2 states.
func validState(s State) bool {
	switch s {
	case StateIdle, StateActive, StateConnect, StateOpenSent, StateOpenConfirm, StateEstablished:
		return true
	default:
		return false
	}
}

func FuzzFSMEventSequence(f *testing.F) {
	// Seed corpus: representative event sequences, each byte = one Event.
	seeds := [][]byte{
		{}, // no events: stays Idle
		{byte(EventManualStart), byte(EventTCPConnectionConfirmed), byte(EventBGPOpen), byte(EventKeepaliveMsg), byte(EventUpdateMsg)}, // Idle→…→Established
		{byte(EventManualStart), byte(EventManualStop)},                                       // start then stop
		{byte(EventManualStart), byte(EventTCPConnectionFails)},                               // connect failure
		{byte(EventManualStart), byte(EventTCPConnectionConfirmed), byte(EventBGPOpenMsgErr)}, // OPEN error
		{byte(EventManualStart), byte(EventTCPConnectionConfirmed), byte(EventBGPOpen), byte(EventHoldTimerExpires)},
		{byte(EventManualStart), byte(EventTCPConnectionConfirmed), byte(EventBGPOpen), byte(EventKeepaliveMsg), byte(EventNotifMsg)},
		{0xff, 0x00, 0x07, 0x2a}, // out-of-range + valid event numbers interleaved
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Exercise both active and passive mode with the same event stream.
		for _, passive := range []bool{false, true} {
			m := New()
			m.SetPassive(passive)
			// Callback observes every transition; both endpoints must be valid.
			m.SetCallback(func(from, to State) {
				if !validState(from) || !validState(to) {
					t.Fatalf("callback saw invalid transition %v -> %v (passive=%v)", from, to, passive)
				}
			})

			if got := m.State(); got != StateIdle {
				t.Fatalf("New() state = %v, want IDLE", got)
			}

			for _, b := range data {
				// Raw byte covers valid events (0..14) and unknown numbers. An
				// event that lands in an error default arm returns ErrFSMError
				// (RFC 4271 Finite State Machine Error); that is expected. Any
				// OTHER error would signal a bug.
				if err := m.Event(Event(int(b))); err != nil && !errors.Is(err, ErrFSMError) {
					t.Fatalf("Event(%d) returned unexpected error: %v", int(b), err)
				}
				if st := m.State(); !validState(st) {
					t.Fatalf("invalid state %d after Event(%d) (passive=%v)", int(st), int(b), passive)
				}
			}
		}
	})
}
