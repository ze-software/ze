// Tests for the hub stop drain step (drain.go).
// VALIDATES: spec-daemon-drain-before-stop R-5, I-5: the drain deadline is
// the earlier of the ceiling counted from the stop start and the largest
// budget counted from the drain start, so a slow config fence spends from
// the ceiling.
// PREVENTS: a drain that restarts the ceiling at drain start and so runs
// past the supervisor's stop timeout.

package hub

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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

// fakeDrainRound records the input runStopDrain sends and, when hold is set,
// waits in Run until the round's context ends, as a plugin still draining does.
type fakeDrainRound struct {
	hold   bool
	ran    chan rpc.DrainInput
	ctxErr chan error
}

func newFakeDrainRound(hold bool) *fakeDrainRound {
	return &fakeDrainRound{hold: hold, ran: make(chan rpc.DrainInput, 1), ctxErr: make(chan error, 1)}
}

func (f *fakeDrainRound) Plugins() int             { return 1 }
func (f *fakeDrainRound) BudgetMax() time.Duration { return 30 * time.Second }

func (f *fakeDrainRound) Run(ctx context.Context, input rpc.DrainInput) pluginserver.DrainResult {
	f.ran <- input
	if f.hold {
		<-ctx.Done()
		f.ctxErr <- ctx.Err()
	}
	return pluginserver.DrainResult{}
}

// TestHubSignalStateTable proves spec-daemon-drain-before-stop AC-6 cell by
// cell, over the hub's stopSignals and runStopDrain with a fake round: a
// signal during the fence skips the drain (no drain RPC) and the stop goes on;
// a signal during the drain cancels the round and the stop goes on; any signal
// after a skip, an abort or a finished drain forces exit. The force exit is
// a counter here, os.Exit in the daemon.
func TestHubSignalStateTable(t *testing.T) {
	stopStart := time.Now()

	t.Run("fence skips the drain, the next signal forces", func(t *testing.T) {
		var forced atomic.Int32
		signals := newStopSignals(func() { forced.Add(1) })
		signals.onSignal()
		if forced.Load() != 0 {
			t.Fatal("a signal during the fence forced exit instead of skipping the drain")
		}
		round := newFakeDrainRound(false)
		runStopDrain(stopStart, round, rpc.DrainInput{Kind: rpc.DrainKindStop}, signals)
		select {
		case <-round.ran:
			t.Fatal("a drain skipped during the fence still sent the drain RPC")
		default:
		}
		signals.onSignal()
		if forced.Load() != 1 {
			t.Fatalf("a signal after a skipped drain: %d forced exits, want 1", forced.Load())
		}
	})

	// The same cell covers `request shutdown` then one SIGTERM: the value
	// request shutdown injects goes to stopCh, never to stopSignals, so the
	// operator's first OS signal is this one.
	t.Run("drain aborts on the first signal, the next forces", func(t *testing.T) {
		var forced atomic.Int32
		signals := newStopSignals(func() { forced.Add(1) })
		round := newFakeDrainRound(true)
		done := make(chan struct{})
		go func() {
			defer close(done)
			runStopDrain(stopStart, round, rpc.DrainInput{Kind: rpc.DrainKindStop}, signals)
		}()
		<-round.ran
		signals.onSignal()
		if err := <-round.ctxErr; !errors.Is(err, context.Canceled) {
			t.Fatalf("drain context ended with %v, want context.Canceled (aborted by signal)", err)
		}
		<-done
		if forced.Load() != 0 {
			t.Fatal("a signal during the drain forced exit instead of aborting the drain")
		}
		signals.onSignal()
		if forced.Load() != 1 {
			t.Fatalf("a signal after an aborted drain: %d forced exits, want 1", forced.Load())
		}
	})

	t.Run("teardown after a finished drain forces", func(t *testing.T) {
		var forced atomic.Int32
		signals := newStopSignals(func() { forced.Add(1) })
		round := newFakeDrainRound(false)
		runStopDrain(stopStart, round, rpc.DrainInput{Kind: rpc.DrainKindStop}, signals)
		<-round.ran
		signals.enterTeardown()
		signals.onSignal()
		if forced.Load() != 1 {
			t.Fatalf("a signal during teardown: %d forced exits, want 1", forced.Load())
		}
	})
}

// TestHubStopKindRestartSentInDrain proves AC-7 at the hub: a stop the restart
// or reboot callbacks requested reaches the drain RPC with kind restart and
// its cause, and a stop nothing marked is kind stop. requestRestartStop is the
// call every restart callback makes before it signals the stop
// (TestSSHLifecycleEndsServerWait asserts the SSH callbacks make it).
func TestHubStopKindRestartSentInDrain(t *testing.T) {
	tests := []struct {
		name  string
		cause *rpc.DrainRestartCause
		want  rpc.DrainInput
	}{
		{name: "stop", want: rpc.DrainInput{Kind: rpc.DrainKindStop}},
		{name: "restart in place", cause: new(rpc.DrainRestartInPlace),
			want: rpc.DrainInput{Kind: rpc.DrainKindRestart, RestartCause: rpc.DrainRestartInPlace}},
		{name: "reboot", cause: new(rpc.DrainRestartReboot),
			want: rpc.DrainInput{Kind: rpc.DrainKindRestart, RestartCause: rpc.DrainRestartReboot}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopRestartCause.Store(nil)
			t.Cleanup(func() { stopRestartCause.Store(nil) })
			if tt.cause != nil {
				requestRestartStop(*tt.cause)
			}

			round := newFakeDrainRound(false)
			runStopDrain(time.Now(), round, stopDrainInput(), newStopSignals(func() { t.Error("forced exit") }))
			got := <-round.ran
			if got.Deadline.IsZero() {
				t.Fatal("the drain RPC carries no deadline")
			}
			got.Deadline = time.Time{}
			if got != tt.want {
				t.Fatalf("drain input %+v, want %+v", got, tt.want)
			}
		})
	}
}
