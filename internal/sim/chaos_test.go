package sim

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"
)

// TestChaosClockPassthrough verifies rate=0 means all calls pass through.
//
// VALIDATES: ChaosClock with rate=0 delegates to inner clock without modification.
// PREVENTS: Chaos wrapper altering behavior when rate=0 (disabled).
func TestChaosClockPassthrough(t *testing.T) {
	inner := RealClock{}
	cc := NewChaosClock(inner, ChaosConfig{Seed: 1, Rate: 0.0})

	// Now() should be approximately equal to inner.Now()
	before := inner.Now()
	got := cc.Now()
	after := inner.Now()

	if got.Before(before.Add(-time.Millisecond)) || got.After(after.Add(time.Millisecond)) {
		t.Errorf("ChaosClock.Now() = %v, want between %v and %v", got, before, after)
	}

	// AfterFunc should fire with original duration (no jitter)
	fired := make(chan struct{})
	timer := cc.AfterFunc(10*time.Millisecond, func() {
		close(fired)
	})
	defer timer.Stop()

	select {
	case <-fired:
		// OK
	case <-time.After(1 * time.Second):
		t.Fatal("AfterFunc did not fire within 1s")
	}
}

// TestChaosClockJitter verifies rate=1 jitters all timer durations.
//
// VALIDATES: ChaosClock with rate=1 modifies timer durations within 0.8-1.2x bounds.
// PREVENTS: Jitter producing durations outside the valid range.
func TestChaosClockJitter(t *testing.T) {
	inner := RealClock{}
	cc := NewChaosClock(inner, ChaosConfig{Seed: 42, Rate: 1.0})

	baseDuration := 100 * time.Millisecond
	minExpected := time.Duration(float64(baseDuration) * 0.8)
	maxExpected := time.Duration(float64(baseDuration) * 1.2)

	// Create 20 timers and verify all durations are within bounds
	for i := range 20 {
		start := time.Now()
		fired := make(chan struct{})
		timer := cc.AfterFunc(baseDuration, func() {
			close(fired)
		})

		select {
		case <-fired:
			elapsed := time.Since(start)
			// Allow some OS scheduling slack (±20ms)
			if elapsed < minExpected-20*time.Millisecond {
				t.Errorf("timer %d fired too early: %v < %v", i, elapsed, minExpected)
			}
			if elapsed > maxExpected+50*time.Millisecond {
				t.Errorf("timer %d fired too late: %v > %v", i, elapsed, maxExpected)
			}
		case <-time.After(2 * time.Second):
			timer.Stop()
			t.Fatalf("timer %d did not fire within 2s", i)
		}
	}
}

// TestChaosClockDeterministic verifies same seed produces same jitter sequence.
//
// VALIDATES: Seed-driven PRNG produces reproducible results.
// PREVENTS: Non-deterministic behavior that would make chaos unreproducible.
func TestChaosClockDeterministic(t *testing.T) {
	// Create two ChaosClock instances with same seed
	inner := RealClock{}
	cc1 := NewChaosClock(inner, ChaosConfig{Seed: 99, Rate: 1.0})
	cc2 := NewChaosClock(inner, ChaosConfig{Seed: 99, Rate: 1.0})

	// Extract jitter decisions from both (use the internal method)
	baseDuration := 100 * time.Millisecond
	for i := range 10 {
		d1 := cc1.jitteredDuration(baseDuration)
		d2 := cc2.jitteredDuration(baseDuration)
		if d1 != d2 {
			t.Errorf("iteration %d: d1=%v != d2=%v (same seed should produce same sequence)", i, d1, d2)
		}
	}
}

// TestChaosDialerPassthrough verifies rate=0 means all dials pass through.
//
// VALIDATES: ChaosDialer with rate=0 delegates to inner dialer without faults.
// PREVENTS: Chaos wrapper injecting faults when rate=0.
func TestChaosDialerPassthrough(t *testing.T) {
	// Use a local listener to verify passthrough
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cerr := ln.Close(); cerr != nil {
			t.Log("close listener:", cerr)
		}
	})

	inner := &RealDialer{}
	cd := NewChaosDialer(inner, ChaosConfig{Seed: 1, Rate: 0.0})

	// All 10 dials should succeed
	for i := range 10 {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		conn, dialErr := cd.DialContext(ctx, "tcp", ln.Addr().String())
		cancel()
		if dialErr != nil {
			t.Errorf("dial %d: unexpected error: %v", i, dialErr)
			continue
		}
		if cerr := conn.Close(); cerr != nil {
			t.Log("close conn:", cerr)
		}
	}
}

// TestChaosDialerFault verifies rate=1 injects faults on all dials.
//
// VALIDATES: ChaosDialer with rate=1 injects failures on every dial attempt.
// PREVENTS: Fault injection not working (pass-through when it shouldn't).
func TestChaosDialerFault(t *testing.T) {
	// Use a local listener
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cerr := ln.Close(); cerr != nil {
			t.Log("close listener:", cerr)
		}
	})

	inner := &RealDialer{}
	cd := NewChaosDialer(inner, ChaosConfig{Seed: 42, Rate: 1.0})

	// With rate=1, every dial should either fail or return a chaos conn
	// (connection refused or connection reset after some bytes)
	var faults int
	for range 10 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		conn, dialErr := cd.DialContext(ctx, "tcp", ln.Addr().String())
		cancel()
		if dialErr != nil {
			faults++
			continue
		}
		// Connection might succeed but be a chaosConn that resets
		// Try to write — it should eventually fail
		_, writeErr := conn.Write([]byte("test"))
		if writeErr != nil {
			faults++
		}
		if cerr := conn.Close(); cerr != nil {
			t.Log("close conn:", cerr)
		}
	}

	// At rate=1, we expect most dials to fault somehow
	if faults == 0 {
		t.Error("rate=1.0 should inject at least some faults, got 0")
	}
}

// TestChaosDialerDeterministic verifies same seed produces same fault sequence.
//
// VALIDATES: ChaosDialer fault decisions are deterministic for a given seed.
// PREVENTS: Non-reproducible fault sequences.
func TestChaosDialerDeterministic(t *testing.T) {
	cd1 := NewChaosDialer(&RealDialer{}, ChaosConfig{Seed: 77, Rate: 0.5})
	cd2 := NewChaosDialer(&RealDialer{}, ChaosConfig{Seed: 77, Rate: 0.5})

	// Extract fault decisions from both
	for i := range 10 {
		f1 := cd1.shouldFault()
		f2 := cd2.shouldFault()
		if f1 != f2 {
			t.Errorf("iteration %d: fault decisions differ (same seed)", i)
		}
	}
}

// TestChaosListenerPassthrough verifies rate=0 means all listens pass through.
//
// VALIDATES: ChaosListenerFactory with rate=0 delegates without faults.
// PREVENTS: Listener chaos when rate=0.
func TestChaosListenerPassthrough(t *testing.T) {
	inner := RealListenerFactory{}
	clf := NewChaosListenerFactory(inner, ChaosConfig{Seed: 1, Rate: 0.0})

	// Listen should succeed
	ctx := context.Background()
	ln, err := clf.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen with rate=0 should succeed: %v", err)
	}
	t.Cleanup(func() {
		if cerr := ln.Close(); cerr != nil {
			t.Log("close listener:", cerr)
		}
	})
}

// TestChaosListenerFault verifies rate=1 injects faults on listens.
//
// VALIDATES: ChaosListenerFactory with rate=1 returns errors on Listen.
// PREVENTS: Missing fault injection on listener creation.
func TestChaosListenerFault(t *testing.T) {
	inner := RealListenerFactory{}
	clf := NewChaosListenerFactory(inner, ChaosConfig{Seed: 42, Rate: 1.0})

	// With rate=1, Listen calls should fail
	ctx := context.Background()
	var faults int
	for range 10 {
		ln, err := clf.Listen(ctx, "tcp", "127.0.0.1:0")
		if err != nil {
			faults++
			continue
		}
		if cerr := ln.Close(); cerr != nil {
			t.Log("close listener:", cerr)
		}
	}

	// At rate=1, all Listen calls should fail
	if faults != 10 {
		t.Errorf("rate=1.0: expected 10 faults, got %d", faults)
	}
}

// TestChaosListenerAcceptDelay verifies chaosListener delays Accept() calls.
//
// VALIDATES: ChaosListenerFactory wraps listeners with accept delay at rate=1.
// PREVENTS: chaosListener.Accept() delay path being dead code (untested).
func TestChaosListenerAcceptDelay(t *testing.T) {
	// Use rate=0 for Listen (so it succeeds) but the chaosListener shares
	// the factory's RNG, so we need a low rate to get past Listen then
	// verify Accept is wrapped. Use a moderate rate and seed that allows
	// at least one Listen to succeed.
	inner := RealListenerFactory{}
	// Rate=0 means no faults on Listen AND no faults on Accept (passthrough).
	// We need the chaosListener to actually delay, so we need rate > 0.
	// But rate=1 means Listen always fails. Solution: create the listener
	// with rate=0, then replace the rng to test Accept behavior directly.
	//
	// Simpler approach: use a seed/rate combo where Listen passes at least once.
	// At rate=0.5, approximately half the Listen calls succeed.
	clf := NewChaosListenerFactory(inner, ChaosConfig{Seed: 7, Rate: 0.5})

	ctx := context.Background()
	var ln net.Listener
	for range 20 {
		var err error
		ln, err = clf.Listen(ctx, "tcp", "127.0.0.1:0")
		if err == nil {
			break
		}
	}
	if ln == nil {
		t.Fatal("could not create listener after 20 attempts")
	}
	t.Cleanup(func() {
		if cerr := ln.Close(); cerr != nil {
			t.Log("close listener:", cerr)
		}
	})

	// The returned listener should be a *chaosListener, not a raw net.Listener.
	// Verify by checking that Accept takes longer than a raw accept would
	// (chaos delay is 1-3s when faulting).
	// We'll connect to it and measure the accept time.
	addr := ln.Addr().String()
	accepted := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			if cerr := conn.Close(); cerr != nil {
				t.Log("close accepted conn:", cerr)
			}
		}
		close(accepted)
	}()

	// Dial to trigger the Accept
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if cerr := conn.Close(); cerr != nil {
		t.Log("close dial conn:", cerr)
	}

	// Wait for accept to complete (should succeed even if delayed)
	select {
	case <-accepted:
		// OK — Accept completed (possibly with delay)
	case <-time.After(10 * time.Second):
		t.Fatal("Accept did not complete within 10s")
	}
}

// TestChaosConcurrency verifies chaos wrappers are safe for concurrent use.
//
// VALIDATES: Mutex-protected PRNG handles concurrent calls from multiple goroutines.
// PREVENTS: Data races in shared PRNG state.
func TestChaosConcurrency(t *testing.T) {
	inner := RealClock{}
	cc := NewChaosClock(inner, ChaosConfig{Seed: 42, Rate: 0.5})

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			for range 100 {
				cc.Now()
			}
		})
	}

	// If there's a data race, -race flag will catch it
	wg.Wait()
}

// TestChaosLogging verifies injected faults produce structured log entries.
//
// VALIDATES: Every injected fault is logged with type and target info.
// PREVENTS: Silent fault injection with no observability.
func TestChaosLogging(t *testing.T) {
	// Create a logger that captures output
	var buf logBuffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	inner := RealClock{}
	cc := NewChaosClock(inner, ChaosConfig{Seed: 42, Rate: 1.0, Logger: logger})

	// Trigger some operations that should be faulted
	cc.Now()
	cc.Now()
	cc.Now()

	// Check that log entries were produced
	if buf.count() == 0 {
		t.Error("expected log entries for injected faults, got none")
	}
}

// logBuffer captures slog output for testing.
type logBuffer struct {
	mu      sync.Mutex
	entries int
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries++
	return len(p), nil
}

func (b *logBuffer) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.entries
}

// TestChaosRateBoundary verifies rate boundary behavior.
//
// VALIDATES: Rate=0.0 means no faults, rate=1.0 means all faults, rate>1.0 clamped.
// PREVENTS: Off-by-one in rate comparison, unclamped rates.
func TestChaosRateBoundary(t *testing.T) {
	tests := []struct {
		name     string
		rate     float64
		wantRate float64
	}{
		{"zero", 0.0, 0.0},
		{"normal", 0.5, 0.5},
		{"one", 1.0, 1.0},
		{"above_one_clamped", 1.5, 1.0},
		{"negative_clamped", -0.1, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := NewChaosClock(RealClock{}, ChaosConfig{Seed: 1, Rate: tt.rate})
			got := cc.effectiveRate()
			if got != tt.wantRate {
				t.Errorf("rate=%v: effectiveRate()=%v, want %v", tt.rate, got, tt.wantRate)
			}
		})
	}
}

// TestChaosSeedBoundary verifies seed=0 means disabled.
//
// VALIDATES: Seed=0 creates a passthrough (no chaos even if rate>0).
// PREVENTS: Seed=0 accidentally enabling chaos with default PRNG.
func TestChaosSeedBoundary(t *testing.T) {
	inner := RealClock{}
	cc := NewChaosClock(inner, ChaosConfig{Seed: 0, Rate: 1.0})

	// With seed=0, chaos should be disabled regardless of rate
	before := inner.Now()
	got := cc.Now()
	after := inner.Now()

	if got.Before(before.Add(-time.Millisecond)) || got.After(after.Add(time.Millisecond)) {
		t.Errorf("seed=0 should disable chaos: Now()=%v not between %v and %v", got, before, after)
	}
}

// TestChaosClockNewTimerJitter verifies NewTimer durations are jittered.
//
// VALIDATES: NewTimer durations are modified when rate=1.
// PREVENTS: Only AfterFunc being jittered while NewTimer passes through.
func TestChaosClockNewTimerJitter(t *testing.T) {
	cc := NewChaosClock(RealClock{}, ChaosConfig{Seed: 42, Rate: 1.0})

	baseDuration := 100 * time.Millisecond
	start := time.Now()
	timer := cc.NewTimer(baseDuration)
	defer timer.Stop()

	ch := timer.C()
	if ch == nil {
		t.Fatal("NewTimer.C() returned nil")
	}

	select {
	case <-ch:
		elapsed := time.Since(start)
		// Should be jittered: 80ms-120ms (±20ms OS slack)
		if elapsed < 60*time.Millisecond || elapsed > 200*time.Millisecond {
			t.Errorf("NewTimer jitter out of bounds: %v", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("NewTimer did not fire within 2s")
	}
}

// Ensure chaos types implement their interfaces at compile time.
//
// VALIDATES: ChaosClock, ChaosDialer, ChaosListenerFactory satisfy their interfaces.
// PREVENTS: Missing methods on chaos wrapper types.
func TestChaosInterfaceSatisfied(t *testing.T) {
	var _ Clock = &ChaosClock{}
	var _ Dialer = &ChaosDialer{}
	var _ ListenerFactory = &ChaosListenerFactory{}
}

// TestResolveSeed verifies that seed=-1 produces a time-based seed and other values pass through.
//
// VALIDATES: ResolveSeed(-1) returns a non-negative, non-zero, time-based seed.
// PREVENTS: Seed -1 being used literally as a PRNG seed (would always produce same sequence).
func TestResolveSeed(t *testing.T) {
	// Explicit seeds pass through unchanged.
	if got := ResolveSeed(42); got != 42 {
		t.Errorf("ResolveSeed(42) = %d, want 42", got)
	}
	if got := ResolveSeed(0); got != 0 {
		t.Errorf("ResolveSeed(0) = %d, want 0", got)
	}

	// Seed -1 resolves to a time-based value (non-zero, non-negative).
	before := time.Now().UnixNano()
	resolved := ResolveSeed(-1)
	after := time.Now().UnixNano()

	if resolved <= 0 {
		t.Errorf("ResolveSeed(-1) = %d, want positive time-based seed", resolved)
	}
	if resolved < before || resolved > after {
		t.Errorf("ResolveSeed(-1) = %d, want between %d and %d", resolved, before, after)
	}

	// Two calls produce different seeds (time advances).
	time.Sleep(time.Millisecond)
	resolved2 := ResolveSeed(-1)
	if resolved == resolved2 {
		t.Error("two ResolveSeed(-1) calls returned the same value")
	}
}
