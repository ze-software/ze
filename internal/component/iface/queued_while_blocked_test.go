package iface

import (
	"log/slog"
	"testing"
)

// VALIDATES: an event pushed while the worker is waiting for the config-commit
// lock is counted, and one pushed while the worker is free is not.
//
// PREVENTS: the test/plugin/iface-link-flap-during-commit ambiguity returning.
// That test needs to know its burst met a held lock, and it read
// ze_iface_link_worker_blocked_total for it. That counter cannot answer it: the
// worker takes the lock once per drained entry, so at most one block exists per
// contiguous hold, and the 1 Hz carrier resync usually takes it while a burst
// coalesces behind it and finds the lock free. Reading it over a wide window
// made a guard true in every round that took the lock at all; over a narrow one
// it read zero through a genuine hold. This counter has no window: an event
// counted here arrived during the wait.
func TestEventsQueuedWhileTheWorkerIsBlockedAreCounted(t *testing.T) {
	reg := bindCapturingMetrics(t)
	queue := newLinkEventQueue(slog.New(slog.DiscardHandler))

	// Free worker: nothing is counted.
	queue.pushCarrier("eth0", true)
	counter, ok := reg.counterVecs["ze_iface_link_events_queued_while_blocked_total"]
	if !ok {
		t.Fatal("ze_iface_link_events_queued_while_blocked_total is not registered")
	}
	// value() answers -1 for a label it has never seen, which is not the same
	// as a series sitting at zero: nothing was counted, so no series exists.
	// Asserting 0 here would be asserting the wrong absence.
	if got := counter.value("eth0"); got != -1 {
		t.Errorf("an event queued while the worker was free created a series reading %v, want no series (-1)", got)
	}

	// Blocked worker: the same push is counted, against the interface.
	queue.setWorkerBlocked(true)
	queue.pushCarrier("eth0", false)
	if got := counter.value("eth0"); got != 1 {
		t.Errorf("an event queued while the worker was blocked counted %v, want 1", got)
	}

	// A resync arriving in the same window counts too, under its own label:
	// the question is what ARRIVED during the hold, not what survived it.
	queue.pushResync(map[string]bool{"eth0": true})
	if got := counter.value(resyncBlockedLabel); got != 1 {
		t.Errorf("a resync queued while blocked counted %v under %q, want 1", got, resyncBlockedLabel)
	}

	// Window closed: back to not counting.
	queue.setWorkerBlocked(false)
	queue.pushCarrier("eth0", true)
	if got := counter.value("eth0"); got != 1 {
		t.Errorf("an event queued after the window closed counted %v, want the earlier 1", got)
	}
}

// VALIDATES: a coalesced event still counts. A burst folds into one pending
// entry, and the count must report what ARRIVED, not what the worker later
// drained, or a burst of 101 transitions during a hold would count once.
func TestACoalescedEventStillCountsAsQueuedWhileBlocked(t *testing.T) {
	reg := bindCapturingMetrics(t)
	queue := newLinkEventQueue(slog.New(slog.DiscardHandler))
	counter := reg.counterVecs["ze_iface_link_events_queued_while_blocked_total"]

	queue.setWorkerBlocked(true)
	for range 5 {
		// Same key every time, so every push after the first coalesces.
		queue.pushCarrier("eth0", true)
	}
	if got := counter.value("eth0"); got != 5 {
		t.Errorf("five coalescing pushes during a hold counted %v, want 5: the count must report arrivals, not survivors", got)
	}
}
