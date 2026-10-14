// Tests for the hub stop drain step (drain.go).
// VALIDATES: spec-daemon-drain-before-stop R-5, I-5: the drain deadline is
// the earlier of the ceiling counted from the stop start and the largest
// budget counted from the drain start, so a slow config fence spends from
// the ceiling.
// PREVENTS: a drain that restarts the ceiling at drain start and so runs
// past the supervisor's stop timeout.

package hub

import (
	"testing"
	"time"
)

func TestDrainDeadlineCountedFromStopStart(t *testing.T) {
	stopStart := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	// A short fence: the plugin's own budget ends first.
	drainStart := stopStart.Add(2 * time.Second)
	if got, want := stopDrainDeadline(stopStart, drainStart, 10*time.Second), drainStart.Add(10*time.Second); !got.Equal(want) {
		t.Fatalf("short fence: deadline %v, want %v", got, want)
	}

	// A fence that spent 55 s: the ceiling, counted from the stop start, ends first.
	drainStart = stopStart.Add(55 * time.Second)
	if got, want := stopDrainDeadline(stopStart, drainStart, 10*time.Second), stopStart.Add(drainCeiling); !got.Equal(want) {
		t.Fatalf("long fence: deadline %v, want the ceiling %v", got, want)
	}

	// A fence that spent the whole ceiling leaves the drain no time.
	drainStart = stopStart.Add(drainCeiling + time.Second)
	if got := stopDrainDeadline(stopStart, drainStart, 10*time.Second); got.After(drainStart) {
		t.Fatalf("spent ceiling: deadline %v is after the drain start %v", got, drainStart)
	}
}
