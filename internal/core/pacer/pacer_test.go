// VALIDATES: AC-1 (bounded growth to a fixed ceiling), AC-2 (the first
// retry after a success is immediate), AC-3 (a success resets the pacer),
// AC-4 and AC-5 (the wait returns at once when the caller's exit signal
// fires, even while waiting at the ceiling).
// PREVENTS: a Pacer that never paces (a stub returning zero forever, which
// leaves the busy loop this package exists to remove), a Pacer that grows
// without bound, and a Pacer whose wait ignores the caller's stop signal.

package pacer

import (
	"testing"
	"time"
)

// TestPacerFirstRetryIsImmediate proves AC-2: the very first failure a
// Pacer ever meets retries at once, with no wait and without touching the
// stop channel at all.
func TestPacerFirstRetryIsImmediate(t *testing.T) {
	var p Pacer
	stop := make(chan struct{}) // never closed; Wait must not read it for a zero delay

	start := time.Now()
	stopped := p.Wait(stop)
	elapsed := time.Since(start)

	if stopped {
		t.Fatal("Wait reported stopped on the first-ever failure, want an immediate retry")
	}
	if elapsed > 5*time.Millisecond {
		t.Fatalf("first retry took %s, want near-immediate", elapsed)
	}
}

// TestPacerGrowsToCeiling proves AC-1's bound: a run of consecutive
// failures grows the delay and never exceeds ceiling. It drives next()
// directly so the growth curve is asserted without waiting out real time.
func TestPacerGrowsToCeiling(t *testing.T) {
	var p Pacer

	want := time.Duration(0)
	for i := range 12 {
		got := p.next()
		if got != want {
			t.Fatalf("call %d: next() = %s, want %s", i, got, want)
		}
		if want == 0 {
			want = step
		} else {
			want = min(want*2, ceiling)
		}
	}

	// By now the curve has long since saturated; every further call must
	// return exactly ceiling, never more.
	for i := range 5 {
		got := p.next()
		if got != ceiling {
			t.Fatalf("call past saturation %d: next() = %s, want ceiling %s", i, got, ceiling)
		}
	}
}

// TestPacerResetsAfterSuccess proves AC-3: after a run of failures has
// grown the delay, a success resets it, so the next failure retries at
// once again.
func TestPacerResetsAfterSuccess(t *testing.T) {
	var p Pacer

	for range 6 {
		p.next()
	}
	if got := p.next(); got != ceiling {
		t.Fatalf("delay before Succeed = %s, want it to have reached ceiling %s", got, ceiling)
	}

	p.Succeed()

	if got := p.next(); got != 0 {
		t.Fatalf("delay after Succeed = %s, want 0 (immediate retry)", got)
	}
}

// TestPacerWaitReturnsOnStop proves AC-4 and AC-5: Wait returns at once
// when stop fires, even while it is waiting at the ceiling rather than at
// the very first, short delay.
func TestPacerWaitReturnsOnStop(t *testing.T) {
	var p Pacer

	// Drive the pacer to the ceiling without waiting out any real delay:
	// next() advances the growth state without touching the timer.
	for range 10 {
		p.next()
	}

	stop := make(chan struct{})
	close(stop)

	start := time.Now()
	stopped := p.Wait(stop)
	elapsed := time.Since(start)

	if !stopped {
		t.Fatal("Wait did not report stopped though stop was already closed")
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("Wait took %s to notice a closed stop channel while waiting at the ceiling (%s), want near-immediate", elapsed, ceiling)
	}
}

// TestPacerWaitReusesTheTimer proves the allocation discipline AC-8
// depends on: Wait creates its *time.Timer once and reuses it, rather than
// allocating a new one on every call.
func TestPacerWaitReusesTheTimer(t *testing.T) {
	var p Pacer

	stop := make(chan struct{})
	if stopped := p.Wait(stop); stopped {
		t.Fatal("first Wait reported stopped, want the immediate retry")
	}
	if stopped := p.Wait(stop); stopped {
		t.Fatal("second Wait reported stopped")
	}
	first := p.timer
	if first == nil {
		t.Fatal("Wait with a positive delay must create a timer")
	}
	if stopped := p.Wait(stop); stopped {
		t.Fatal("third Wait reported stopped")
	}
	if p.timer != first {
		t.Fatal("Wait allocated a new timer instead of reusing the one from the previous call")
	}
}
