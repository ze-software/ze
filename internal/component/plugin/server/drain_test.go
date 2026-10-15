// Tests for the stop drain fan-out (drain.go) and the Stage 1 drain
// declaration check (startup.go validateDrainDecl).
// VALIDATES: spec-daemon-drain-before-stop AC-2, AC-3, AC-4, AC-6 (abort),
// AC-8 (live generations only), AC-13 (b), AC-35 (e), Data Flow step 4.2.
// PREVENTS: a sequential fan-out, a late plugin blocking its caller or the
// stop, a call dropped at the deadline, and a budget the daemon cannot honour.

package server

import (
	"context"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// startTestCaller builds and starts one drain caller over call, and returns it
// with the function that ends its generation (cancel, then wait for run).
func startTestCaller(t *testing.T, plugin string, budget time.Duration, call drainCallFunc) (*drainCaller, func()) {
	t.Helper()
	caller := newDrainCaller(plugin, budget, call)
	ctx, cancel := context.WithCancel(context.Background())
	go caller.run(ctx)
	stop := func() {
		cancel()
		<-caller.done
	}
	t.Cleanup(stop)
	return caller, stop
}

func holdCall(hold time.Duration) drainCallFunc {
	return func(ctx context.Context, _ *rpc.DrainInput) error {
		select {
		case <-time.After(hold):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func stopInput(deadline time.Time) rpc.DrainInput {
	return rpc.DrainInput{Kind: rpc.DrainKindStop, Deadline: deadline}
}

// TestDrainCallersOverlap proves AC-2: two plugins each holding their drain
// 2 s are drained in parallel, through their two callers, so the round ends
// in under 3 s rather than 4 s.
func TestDrainCallersOverlap(t *testing.T) {
	first, _ := startTestCaller(t, "first", 5*time.Second, holdCall(2*time.Second))
	second, _ := startTestCaller(t, "second", 5*time.Second, holdCall(2*time.Second))
	round := &DrainRound{callers: []*drainCaller{first, second}}

	deadline := time.Now().Add(5 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	start := time.Now()
	result := round.Run(ctx, stopInput(deadline))
	elapsed := time.Since(start)

	if elapsed >= 3*time.Second {
		t.Fatalf("two 2 s drains took %v: the callers did not overlap", elapsed)
	}
	if !slices.Equal(result.Answered, []string{"first", "second"}) {
		t.Fatalf("answered %v, want both plugins", result.Answered)
	}
}

// TestDrainCollectDeadlineNamesLatePlugin proves AC-3: a plugin that never
// answers is named late at the deadline; one that answers after the deadline
// posts into its abandoned reply channel without blocking, and its caller
// then serves a next request.
func TestDrainCollectDeadlineNamesLatePlugin(t *testing.T) {
	never := make(chan struct{})
	silent, _ := startTestCaller(t, "silent", time.Second, func(ctx context.Context, _ *rpc.DrainInput) error {
		select {
		case <-never:
		case <-ctx.Done():
		}
		return ctx.Err()
	})
	answeredLate := make(chan struct{})
	calls := 0
	tardy, _ := startTestCaller(t, "tardy", time.Second, func(_ context.Context, _ *rpc.DrainInput) error {
		calls++
		if calls == 1 {
			time.Sleep(600 * time.Millisecond)
			close(answeredLate)
		}
		return nil
	})
	round := &DrainRound{callers: []*drainCaller{silent, tardy}}

	deadline := time.Now().Add(300 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	result := round.Run(ctx, stopInput(deadline))
	if result.Aborted {
		t.Fatal("a deadline was reported as an abort")
	}
	if !slices.Equal(result.Late, []string{"silent", "tardy"}) {
		t.Fatalf("late %v, want silent and tardy", result.Late)
	}

	select {
	case <-answeredLate:
	case <-time.After(5 * time.Second):
		t.Fatal("the late answer never completed")
	}

	// The caller is free again: its late post did not block it.
	next := &DrainRound{callers: []*drainCaller{tardy}}
	nextDeadline := time.Now().Add(2 * time.Second)
	nextCtx, nextCancel := context.WithDeadline(context.Background(), nextDeadline)
	defer nextCancel()
	if got := next.Run(nextCtx, stopInput(nextDeadline)); !slices.Equal(got.Answered, []string{"tardy"}) {
		t.Fatalf("after a late answer the caller served %+v, want an answer", got)
	}
}

// TestDrainCallerCallOutlivesDeadline proves Data Flow step 4.2: the call
// runs under the caller's context, so it is still running after the drain
// deadline passed and is not cancelled by it.
func TestDrainCallerCallOutlivesDeadline(t *testing.T) {
	callCtxErr := make(chan error, 1)
	caller, _ := startTestCaller(t, "slow", time.Second, func(ctx context.Context, _ *rpc.DrainInput) error {
		time.Sleep(300 * time.Millisecond)
		callCtxErr <- ctx.Err()
		return nil
	})
	round := &DrainRound{callers: []*drainCaller{caller}}

	deadline := time.Now().Add(50 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	round.Run(ctx, stopInput(deadline))

	select {
	case err := <-callCtxErr:
		if err != nil {
			t.Fatalf("the call's context ended with the drain deadline: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the call never finished")
	}
}

// TestDrainCollectCancelAborts proves the abort half of AC-6: cancelling the
// drain context (a signal) stops the reading at once and reports an abort.
func TestDrainCollectCancelAborts(t *testing.T) {
	caller, _ := startTestCaller(t, "held", 30*time.Second, holdCall(30*time.Second))
	round := &DrainRound{callers: []*drainCaller{caller}}

	deadline := time.Now().Add(30 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	time.AfterFunc(100*time.Millisecond, cancel)

	start := time.Now()
	result := round.Run(ctx, stopInput(deadline))
	if !result.Aborted {
		t.Fatalf("cancel was not reported as an abort: %+v", result)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("the abort took %v", elapsed)
	}
}

// TestDrainAllZeroWaitsUnderBound proves AC-13 (b): plugins whose drain
// answers at once leave the stop as fast as before the drain existed.
func TestDrainAllZeroWaitsUnderBound(t *testing.T) {
	first, _ := startTestCaller(t, "first", time.Second, holdCall(0))
	second, _ := startTestCaller(t, "second", time.Second, holdCall(0))
	round := &DrainRound{callers: []*drainCaller{first, second}}

	deadline := time.Now().Add(time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	start := time.Now()
	round.Run(ctx, stopInput(deadline))
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("zero-wait drain took %v", elapsed)
	}
}

// TestDrainCallerSetLiveGenerationsOnly proves the AC-8 failure case: a
// plugin whose generation ended is not in the snapshot; a caller that ended
// after the snapshot is reported failed by name, never late, and its send
// does not block.
func TestDrainCallerSetLiveGenerationsOnly(t *testing.T) {
	s := &Server{}
	live, _ := startTestCaller(t, "live", time.Second, holdCall(0))
	gone, stopGone := startTestCaller(t, "gone", time.Second, holdCall(0))
	s.addDrainCaller(nil, live)

	if got := s.BeginDrain().Plugins(); got != 1 {
		t.Fatalf("snapshot holds %d plugins, want 1", got)
	}

	// The control's caller ends between the snapshot and the send.
	stopGone()
	round := &DrainRound{callers: []*drainCaller{live, gone}}
	deadline := time.Now().Add(2 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	result := round.Run(ctx, stopInput(deadline))
	if !slices.Equal(result.Failed, []string{"gone"}) {
		t.Fatalf("failed %v, want gone", result.Failed)
	}
	if len(result.Late) != 0 {
		t.Fatalf("a returned caller was named late: %v", result.Late)
	}
	if !slices.Equal(result.Answered, []string{"live"}) {
		t.Fatalf("answered %v, want live", result.Answered)
	}
}

// TestDrainStartsNoGoroutine proves AC-35 (e): the round starts no goroutine,
// because the callers are already running; and none runs after its
// generation ends.
func TestDrainStartsNoGoroutine(t *testing.T) {
	gate := make(chan struct{})
	call := func(context.Context, *rpc.DrainInput) error {
		<-gate
		return nil
	}
	first, stopFirst := startTestCaller(t, "first", time.Second, call)
	second, stopSecond := startTestCaller(t, "second", time.Second, call)
	round := &DrainRound{callers: []*drainCaller{first, second}}

	before := runtime.NumGoroutine()
	deadline := time.Now().Add(5 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	during := make(chan int, 1)
	time.AfterFunc(100*time.Millisecond, func() {
		during <- runtime.NumGoroutine()
		close(gate)
	})
	round.Run(ctx, stopInput(deadline))

	// The AfterFunc timer runs on its own goroutine for the sample.
	if got := <-during; got > before+1 {
		t.Fatalf("goroutines during the drain %d, before %d", got, before)
	}

	stopFirst()
	stopSecond()
	for _, caller := range []*drainCaller{first, second} {
		select {
		case <-caller.done:
		default:
			t.Fatalf("caller %s still running after its generation ended", caller.plugin)
		}
	}
}

// TestDrainRoundBudgetMax checks the hub's deadline input: the largest budget.
func TestDrainRoundBudgetMax(t *testing.T) {
	round := &DrainRound{callers: []*drainCaller{
		newDrainCaller("a", 2*time.Second, holdCall(0)),
		newDrainCaller("b", 7*time.Second, holdCall(0)),
	}}
	if got := round.BudgetMax(); got != 7*time.Second {
		t.Fatalf("budget max %v, want 7s", got)
	}
	if got := (&DrainRound{}).BudgetMax(); got != 0 {
		t.Fatalf("empty round budget max %v, want 0", got)
	}
}

// TestDrainBudgetAboveCeilingRefused proves AC-4: a Stage 1 budget above the
// ceiling is refused, naming the plugin, the budget and the ceiling; the
// ceiling itself is accepted.
func TestDrainBudgetAboveCeilingRefused(t *testing.T) {
	err := validateDrainDecl("p1", rpc.DrainCeilingSeconds+1, true)
	if err == nil {
		t.Fatal("a budget above the ceiling was accepted")
	}
	want := `plugin "p1" declares drain-budget 61 s, above the daemon drain ceiling of 60 s`
	if err.Error() != want {
		t.Fatalf("refusal %q, want %q", err, want)
	}
	if err := validateDrainDecl("p1", rpc.DrainCeilingSeconds, true); err != nil {
		t.Fatalf("the ceiling itself was refused: %v", err)
	}
	if err := validateDrainDecl("p1", -1, true); err == nil {
		t.Fatal("a negative budget was accepted")
	}
}

// TestDrainBudgetWithoutHandlerRefused proves AC-4's second half: a budget
// with no drain handler is refused; no budget needs no handler.
func TestDrainBudgetWithoutHandlerRefused(t *testing.T) {
	err := validateDrainDecl("p2", 5, false)
	if err == nil {
		t.Fatal("a budget with no handler was accepted")
	}
	want := `plugin "p2" declares drain-budget 5 s (daemon drain ceiling 60 s) and no drain handler`
	if err.Error() != want {
		t.Fatalf("refusal %q, want %q", err, want)
	}
	if err := validateDrainDecl("p2", 0, false); err != nil {
		t.Fatalf("no budget and no handler was refused: %v", err)
	}
}
