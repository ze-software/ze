package fsm

import (
	"sync"
	"testing"
)

// TestFSMCallbackCarriesTheCausingEvent drives one FSM from Idle to Established and
// drops the transport.
//
// VALIDATES: each state-change callback receives the event whose handling made that
// change, so the Established to Idle callback receives Event 18, TcpConnectionFails.
// PREVENTS: a consumer of the teardown (the reactor's close reason, BMP's Peer Down
// reason 2) guessing the cause from the target state, which is Idle for every close.
func TestFSMCallbackCarriesTheCausingEvent(t *testing.T) {
	f := New()
	type seen struct {
		from, to State
		event    Event
	}
	var got []seen
	f.SetCallback(func(from, to State, event Event) {
		got = append(got, seen{from, to, event})
	})

	for _, event := range []Event{
		EventManualStart, EventTCPConnectionConfirmed, EventBGPOpen, EventKeepaliveMsg,
		EventTCPConnectionFails,
	} {
		_ = f.Event(event) //nolint:errcheck // the transitions are asserted below
	}

	want := []seen{
		{StateIdle, StateConnect, EventManualStart},
		{StateConnect, StateOpenSent, EventTCPConnectionConfirmed},
		{StateOpenSent, StateOpenConfirm, EventBGPOpen},
		{StateOpenConfirm, StateEstablished, EventKeepaliveMsg},
		{StateEstablished, StateIdle, EventTCPConnectionFails},
	}
	if len(got) != len(want) {
		t.Fatalf("transitions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("transition %d = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestFSMCallbackEventSurvivesAsyncDrain races two goroutines through Idle and Connect,
// so some callbacks run on the other goroutine after a later event was dispatched.
//
// VALIDATES: every Idle to Connect callback carries ManualStart and every Connect to Idle
// callback carries ManualStop, whichever goroutine drains it.
// PREVENTS: the event being read when the callback runs instead of when the change was
// queued, which would hand a queued teardown the cause of the event after it.
func TestFSMCallbackEventSurvivesAsyncDrain(t *testing.T) {
	f := New()
	var mu sync.Mutex
	var wrong []string
	f.SetCallback(func(from, to State, event Event) {
		mu.Lock()
		defer mu.Unlock()
		if from == StateIdle && to == StateConnect && event != EventManualStart {
			wrong = append(wrong, "Idle->Connect carried "+event.String())
		}
		if from == StateConnect && to == StateIdle && event != EventManualStop {
			wrong = append(wrong, "Connect->Idle carried "+event.String())
		}
	})

	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 200 {
				_ = f.Event(EventManualStart) //nolint:errcheck // ignored when not in Idle
				_ = f.Event(EventManualStop)  //nolint:errcheck // ignored when in Idle
			}
		})
	}
	wg.Wait()

	if len(wrong) != 0 {
		t.Fatalf("%d callbacks carried another transition's event, first: %s", len(wrong), wrong[0])
	}
}

// TestEventNumberIsTheRegisteredCode pins the numbers BMP writes into a reason 2 Peer
// Down to RFC 4271 Section 8.1 (and the BFD strict-mode draft for Events 30 to 35).
//
// VALIDATES: the events ze closes a session on answer their registered number, and an
// event outside the table answers not-registered.
// PREVENTS: ze's iota ordinal leaking out as an FSM event code.
func TestEventNumberIsTheRegisteredCode(t *testing.T) {
	for event, want := range map[Event]uint16{
		EventManualStop: 2, EventAutomaticStop: 8, EventHoldTimerExpires: 10,
		EventTCPConnectionFails: 18, EventBGPHeaderErr: 21, EventOpenCollisionDump: 23,
		EventNotifMsg: 25, EventUpdateMsgErr: 28, EventBfdDown: 31,
	} {
		got, ok := event.Number()
		if !ok || got != want {
			t.Errorf("%s.Number() = %d, %v; want %d, true", event, got, ok, want)
		}
	}
	if _, ok := Event(-1).Number(); ok {
		t.Error("an event outside the table answered a registered number")
	}
}
