// Design: docs/architecture/doctor-and-health-checks.md -- the request-refused bound, read from the error ring
// Related: report.go -- RaiseErrorFixOnce, raiseErrorOnce

package report

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ageNewestError moves the newest error event's Raised back by age, under the
// store lock. The bus reads time.Now() with no injectable clock, so a test
// plays the window passing by aging the held event instead of sleeping.
func ageNewestError(t *testing.T, age time.Duration) {
	t.Helper()
	s := pkg.Load()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errorCount == 0 {
		t.Fatal("no error event to age")
	}
	held := s.errors[(s.errorHead-1+s.errorCap)%s.errorCap]
	held.Raised = held.Raised.Add(-age)
}

// TestRaiseErrorFixOnce raises error events through RaiseErrorFixOnce and
// reads the ring after each step. The window passing is played by aging the
// held event (ageNewestError), because the bus has no clock seam.
//
// VALIDATES: one event per source, code and subject inside the window; a new
// event once the held one is a window old; a distinct subject is a key of its
// own; concurrent raises of one key give one event; an over-long subject is
// refused as RaiseErrorFix refuses it.
// PREVENTS: one producer looping on a bad request overwriting every other
// owner's events in the error ring.
func TestRaiseErrorFixOnce(t *testing.T) {
	const window = time.Minute

	t.Run("one event inside the window", func(t *testing.T) {
		reset()
		t.Cleanup(reset)
		if !RaiseErrorFixOnce("src", "code", "subj", "first", "ze show errors", window) {
			t.Fatal("first raise answered false, want true")
		}
		// Guards the key match in raiseErrorOnce: a held event of the same
		// source, code and subject younger than window answers false.
		for range 5 {
			if RaiseErrorFixOnce("src", "code", "subj", "again", "ze show errors", window) {
				t.Fatal("repeat raise inside the window answered true, want false")
			}
		}
		got := Errors(0)
		if len(got) != 1 {
			t.Fatalf("ring holds %d events, want 1: %v", len(got), got)
		}
		// Guards the appended Issue: the first raise's message and fix are
		// what the ring keeps.
		if got[0].Message != "first" || got[0].Fix != "ze show errors" {
			t.Errorf("held event message %q fix %q, want the first raise's", got[0].Message, got[0].Fix)
		}
		if got[0].Severity != SeverityError {
			t.Errorf("severity %d, want %d", got[0].Severity, SeverityError)
		}
	})

	t.Run("a new event once the window has passed", func(t *testing.T) {
		reset()
		t.Cleanup(reset)
		RaiseErrorFixOnce("src", "code", "subj", "first", "", window)
		ageNewestError(t, window)
		// Guards `now.Sub(held.Raised) >= window` in raiseErrorOnce: a held
		// event a full window old no longer blocks the key.
		if !RaiseErrorFixOnce("src", "code", "subj", "second", "", window) {
			t.Fatal("raise after the window answered false, want true")
		}
		if got := Errors(0); len(got) != 2 {
			t.Fatalf("ring holds %d events, want 2: %v", len(got), got)
		}
		// The second event is young, so the key is bound again.
		if RaiseErrorFixOnce("src", "code", "subj", "third", "", window) {
			t.Fatal("raise inside the new window answered true, want false")
		}
	})

	t.Run("a distinct subject is its own key", func(t *testing.T) {
		reset()
		t.Cleanup(reset)
		// Guards the subject term of the key match: a held event of another
		// subject does not block this one.
		if !RaiseErrorFixOnce("src", "code", "plugin-a", "a", "", window) {
			t.Fatal("raise for plugin-a answered false, want true")
		}
		if !RaiseErrorFixOnce("src", "code", "plugin-b", "b", "", window) {
			t.Fatal("raise for plugin-b answered false, want true")
		}
		if got := Errors(0); len(got) != 2 {
			t.Fatalf("ring holds %d events, want 2: %v", len(got), got)
		}
	})

	t.Run("concurrent raises give one event", func(t *testing.T) {
		reset()
		t.Cleanup(reset)
		// Guards the single hold of s.mu across the look and the append in
		// raiseErrorOnce: split, two callers both miss and both append.
		var raised atomic.Int32
		var wg sync.WaitGroup
		for range 32 {
			wg.Go(func() {
				if RaiseErrorFixOnce("src", "code", "subj", "msg", "", window) {
					raised.Add(1)
				}
			})
		}
		wg.Wait()
		if n := raised.Load(); n != 1 {
			t.Errorf("%d raises answered true, want 1", n)
		}
		if got := Errors(0); len(got) != 1 {
			t.Errorf("ring holds %d events, want 1", len(got))
		}
	})

	t.Run("an over-long subject is refused as the plain raise is", func(t *testing.T) {
		reset()
		t.Cleanup(reset)
		long := strings.Repeat("a", SubjectMax+1)
		// Guards the validFields check at the top of raiseErrorOnce.
		if RaiseErrorFixOnce("src", "code", long, "msg", "", window) {
			t.Error("raise with an over-long subject answered true, want false")
		}
		if got := Errors(0); len(got) != 0 {
			t.Fatalf("ring holds %d events after the refused raise, want 0", len(got))
		}
		RaiseErrorFix("src", "code", long, "msg", "")
		if got := Errors(0); len(got) != 0 {
			t.Fatalf("RaiseErrorFix kept %d events for the same subject, want 0: the two raises disagree", len(got))
		}
	})
}
