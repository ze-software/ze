package server

import (
	"sync"
	"testing"
	"time"
)

// TestReloadStatusBeforeFirstReload verifies the zero state is distinguishable
// from a processed reload.
//
// VALIDATES: AC-1 -- the counter is a fence, so "no reload yet" must be
// readable as generation 0 and never confused with a completed reload.
// PREVENTS: an observer that reads generation 0 and mistakes the daemon's
// initial state for a reload it triggered, asserting on pre-reload state.
func TestReloadStatusBeforeFirstReload(t *testing.T) {
	s := &Server{}

	gen, outcome, at := s.ReloadStatus()
	if gen != 0 {
		t.Errorf("generation = %d, want 0 before any reload", gen)
	}
	if outcome != ReloadOutcomeNone {
		t.Errorf("outcome = %q, want %q before any reload", outcome, ReloadOutcomeNone)
	}
	if !at.IsZero() {
		t.Errorf("at = %v, want zero time before any reload", at)
	}
}

// TestReloadStatusAdvancesOnRejectedReload is the core AC-1 assertion.
//
// VALIDATES: AC-1 -- "a reload (applied OR rejected) advances a
// plugin-queryable generation counter". A reload whose outcome is a rejection
// changes no other observable state, so the counter advancing is the ONLY
// evidence it ran.
// PREVENTS: the counter being incremented only on the success path, which
// would leave exactly the reject/no-op case -- the one this whole surface
// exists for -- with nothing to poll, and would silently hang the observer.
func TestReloadStatusAdvancesOnRejectedReload(t *testing.T) {
	tests := []struct {
		name        string
		applied     bool
		wantOutcome string
	}{
		{name: "applied reload", applied: true, wantOutcome: ReloadOutcomeApplied},
		{name: "rejected reload", applied: false, wantOutcome: ReloadOutcomeFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{}
			before, _, _ := s.ReloadStatus()

			s.MarkReloadProcessed(tt.applied)

			after, outcome, at := s.ReloadStatus()
			if after != before+1 {
				t.Errorf("generation = %d, want %d (must advance regardless of outcome)", after, before+1)
			}
			if outcome != tt.wantOutcome {
				t.Errorf("outcome = %q, want %q", outcome, tt.wantOutcome)
			}
			if at.IsZero() {
				t.Error("at = zero time, want the completion timestamp")
			}
		})
	}
}

// TestReloadStatusIsMonotonic verifies repeated reloads of mixed outcome each
// advance the counter by exactly one.
//
// VALIDATES: AC-2 -- an observer fences by reading the generation, triggering a
// reload, and polling until it advances. That only works if every processed
// reload advances it by one, whatever the outcome sequence.
// PREVENTS: a reject resetting or failing to advance the counter, which would
// make an observer that fenced on "generation > N" wait forever.
func TestReloadStatusIsMonotonic(t *testing.T) {
	s := &Server{}

	outcomes := []bool{true, false, false, true, false}
	for i, applied := range outcomes {
		s.MarkReloadProcessed(applied)
		gen, _, _ := s.ReloadStatus()
		if want := uint64(i + 1); gen != want {
			t.Fatalf("after %d reloads: generation = %d, want %d", i+1, gen, want)
		}
	}

	_, outcome, _ := s.ReloadStatus()
	if outcome != ReloadOutcomeFailed {
		t.Errorf("outcome = %q, want %q (the last reload failed)", outcome, ReloadOutcomeFailed)
	}
}

// TestReloadStatusConcurrent verifies the counter is race-free.
//
// VALIDATES: AC-2 -- the observer polls ReloadStatus from a command handler
// goroutine while the SIGHUP worker calls MarkReloadProcessed. Those genuinely
// race in the daemon.
// PREVENTS: a lost update or a torn (generation, outcome) pair under -race,
// which would make the fence unreliable exactly under the concurrent load it
// is used in.
func TestReloadStatusConcurrent(t *testing.T) {
	s := &Server{}

	const writers = 8
	const perWriter = 50

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Readers race the writers, mirroring a show handler polling mid-reload.
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					s.ReloadStatus()
				}
			}
		})
	}

	for range writers {
		wg.Go(func() {
			for range perWriter {
				s.MarkReloadProcessed(true)
			}
		})
	}

	// Wait for writers by checking the final count, then release readers.
	for {
		gen, _, _ := s.ReloadStatus()
		if gen == writers*perWriter {
			break
		}
		time.Sleep(time.Millisecond)
	}
	close(stop)
	wg.Wait()

	gen, _, _ := s.ReloadStatus()
	if want := uint64(writers * perWriter); gen != want {
		t.Errorf("generation = %d, want %d (lost update)", gen, want)
	}
}
