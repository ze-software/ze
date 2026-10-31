package fsm

import (
	"testing"
)

// eventRecovering fires ev and returns what a panic out of Event carried, or
// nil when Event returned normally.
func eventRecovering(f *FSM, ev Event) (recovered any) {
	defer func() { recovered = recover() }()
	if err := f.Event(ev); err != nil {
		return err
	}
	return nil
}

// TestCallbackPanicLeavesFSMUsable proves a panicking state-change callback
// unwinds as an ordinary, recoverable panic and leaves the FSM working.
//
// Method: the callback panics on Idle->Connect. Before the fix the drain loop
// had released f.mu around the callback, so Event's deferred Unlock met an
// unlocked mutex: "fatal error: sync: unlock of unlocked mutex", which no
// recover catches and which kills the whole test binary. After it, the test
// recovers the panic, reads the state (f.mu must be unlocked), and fires the
// next event, whose callback must run (draining must have been cleared, or the
// transition would queue behind a drainer that no longer exists).
func TestCallbackPanicLeavesFSMUsable(t *testing.T) {
	f := New()
	var seen []State
	f.SetCallback(func(_, to State, _ Event) {
		seen = append(seen, to)
		if to == StateConnect {
			panic("callback failure")
		}
	})

	got := eventRecovering(f, EventManualStart)
	if got != "callback failure" {
		t.Fatalf("Event(ManualStart) recovered %v, want the callback's panic", got)
	}
	if state := f.State(); state != StateConnect {
		t.Fatalf("state after panicking callback = %v, want Connect", state)
	}

	if got := eventRecovering(f, EventManualStop); got != nil {
		t.Fatalf("Event(ManualStop) after a panicking callback = %v, want nil", got)
	}
	if state := f.State(); state != StateIdle {
		t.Fatalf("state after ManualStop = %v, want Idle", state)
	}
	if len(seen) != 2 {
		t.Fatalf("callbacks seen = %v, want [Connect Idle]: the drain after a panic did not run", seen)
	}
	if seen[1] != StateIdle {
		t.Fatalf("second callback saw %v, want Idle", seen[1])
	}
}
