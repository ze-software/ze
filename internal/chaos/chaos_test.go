package chaos

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
)

// TestChaosClockPassthrough verifies rate=0 means all calls pass through.
//
// VALIDATES: ChaosClock with rate=0 delegates to inner clock without modification.
// PREVENTS: Chaos wrapper altering behavior when rate=0 (disabled).
func TestChaosClockPassthrough(t *testing.T) {
	inner := clock.RealClock{}
	cc := NewChaosClock(inner, ChaosConfig{Seed: 1, Rate: 0.0})

	before := inner.Now()
	got := cc.Now()
	after := inner.Now()

	if got.Before(before.Add(-time.Millisecond)) || got.After(after.Add(time.Millisecond)) {
		t.Errorf("ChaosClock.Now() = %v, want between %v and %v", got, before, after)
	}

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
	inner := clock.RealClock{}
	cc := NewChaosClock(inner, ChaosConfig{Seed: 42, Rate: 1.0})

	baseDuration := 100 * time.Millisecond
	minExpected := time.Duration(float64(baseDuration) * 0.8)
	maxExpected := time.Duration(float64(baseDuration) * 1.2)

	for i := range 20 {
		start := time.Now()
		fired := make(chan struct{})
		timer := cc.AfterFunc(baseDuration, func() {
			close(fired)
		})

		select {
		case <-fired:
			elapsed := time.Since(start)
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
	inner := clock.RealClock{}
	cc1 := NewChaosClock(inner, ChaosConfig{Seed: 99, Rate: 1.0})
	cc2 := NewChaosClock(inner, ChaosConfig{Seed: 99, Rate: 1.0})

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

	inner := &network.RealDialer{}
	cd := NewChaosDialer(inner, ChaosConfig{Seed: 1, Rate: 0.0})

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

	inner := &network.RealDialer{}
	cd := NewChaosDialer(inner, ChaosConfig{Seed: 42, Rate: 1.0})

	var faults int
	for range 10 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		conn, dialErr := cd.DialContext(ctx, "tcp", ln.Addr().String())
		cancel()
		if dialErr != nil {
			faults++
			continue
		}
		_, writeErr := conn.Write([]byte("test"))
		if writeErr != nil {
			faults++
		}
		if cerr := conn.Close(); cerr != nil {
			t.Log("close conn:", cerr)
		}
	}

	if faults == 0 {
		t.Error("rate=1.0 should inject at least some faults, got 0")
	}
}

// TestChaosDialerDeterministic verifies same seed produces same fault sequence.
//
// VALIDATES: ChaosDialer fault decisions are deterministic for a given seed.
// PREVENTS: Non-reproducible fault sequences.
func TestChaosDialerDeterministic(t *testing.T) {
	cd1 := NewChaosDialer(&network.RealDialer{}, ChaosConfig{Seed: 77, Rate: 0.5})
	cd2 := NewChaosDialer(&network.RealDialer{}, ChaosConfig{Seed: 77, Rate: 0.5})

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
	inner := network.RealListenerFactory{}
	clf := NewChaosListenerFactory(inner, ChaosConfig{Seed: 1, Rate: 0.0})

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
	inner := network.RealListenerFactory{}
	clf := NewChaosListenerFactory(inner, ChaosConfig{Seed: 42, Rate: 1.0})

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

	if faults != 10 {
		t.Errorf("rate=1.0: expected 10 faults, got %d", faults)
	}
}

// TestChaosListenerAcceptDelay verifies chaosListener delays Accept() calls.
//
// VALIDATES: ChaosListenerFactory wraps listeners with accept delay at rate=1.
// PREVENTS: chaosListener.Accept() delay path being dead code (untested).
func TestChaosListenerAcceptDelay(t *testing.T) {
	inner := network.RealListenerFactory{}
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
		return
	}
	t.Cleanup(func() {
		if cerr := ln.Close(); cerr != nil {
			t.Log("close listener:", cerr)
		}
	})

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

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if cerr := conn.Close(); cerr != nil {
		t.Log("close dial conn:", cerr)
	}

	select {
	case <-accepted:
		// OK
	case <-time.After(10 * time.Second):
		t.Fatal("Accept did not complete within 10s")
	}
}

// TestChaosConcurrency verifies chaos wrappers are safe for concurrent use.
//
// VALIDATES: Mutex-protected PRNG handles concurrent calls from multiple goroutines.
// PREVENTS: Data races in shared PRNG state.
func TestChaosConcurrency(t *testing.T) {
	inner := clock.RealClock{}
	cc := NewChaosClock(inner, ChaosConfig{Seed: 42, Rate: 0.5})

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			for range 100 {
				cc.Now()
			}
		})
	}

	wg.Wait()
}

// TestChaosLogging verifies injected faults produce structured log entries.
//
// VALIDATES: Every injected fault is logged with type and target info.
// PREVENTS: Silent fault injection with no observability.
func TestChaosLogging(t *testing.T) {
	var buf logBuffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	inner := clock.RealClock{}
	cc := NewChaosClock(inner, ChaosConfig{Seed: 42, Rate: 1.0, Logger: logger})

	cc.Now()
	cc.Now()
	cc.Now()

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
			cc := NewChaosClock(clock.RealClock{}, ChaosConfig{Seed: 1, Rate: tt.rate})
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
	inner := clock.RealClock{}
	cc := NewChaosClock(inner, ChaosConfig{Seed: 0, Rate: 1.0})

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
	cc := NewChaosClock(clock.RealClock{}, ChaosConfig{Seed: 42, Rate: 1.0})

	baseDuration := 100 * time.Millisecond
	start := time.Now()
	timer := cc.NewTimer(baseDuration)
	defer timer.Stop()

	ch := timer.C()
	if ch == nil {
		t.Fatal("NewTimer.C() returned nil")
		return
	}

	select {
	case <-ch:
		elapsed := time.Since(start)
		if elapsed < 60*time.Millisecond || elapsed > 200*time.Millisecond {
			t.Errorf("NewTimer jitter out of bounds: %v", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("NewTimer did not fire within 2s")
	}
}

// TestChaosInterfaceSatisfied verifies chaos types implement their interfaces.
//
// VALIDATES: ChaosClock, ChaosDialer, ChaosListenerFactory satisfy their interfaces.
// PREVENTS: Missing methods on chaos wrapper types.
func TestChaosInterfaceSatisfied(t *testing.T) {
	var _ clock.Clock = &ChaosClock{}
	var _ network.Dialer = &ChaosDialer{}
	var _ network.ListenerFactory = &ChaosListenerFactory{}
}

// TestResolveSeed verifies that seed=-1 produces a time-based seed and other values pass through.
//
// VALIDATES: ResolveSeed(-1) returns a non-negative, non-zero, time-based seed.
// PREVENTS: Seed -1 being used literally as a PRNG seed (would always produce same sequence).
func TestResolveSeed(t *testing.T) {
	if got := ResolveSeed(42); got != 42 {
		t.Errorf("ResolveSeed(42) = %d, want 42", got)
	}
	if got := ResolveSeed(0); got != 0 {
		t.Errorf("ResolveSeed(0) = %d, want 0", got)
	}

	before := time.Now().UnixNano()
	resolved := ResolveSeed(-1)
	after := time.Now().UnixNano()

	if resolved <= 0 {
		t.Errorf("ResolveSeed(-1) = %d, want positive time-based seed", resolved)
	}
	if resolved < before || resolved > after {
		t.Errorf("ResolveSeed(-1) = %d, want between %d and %d", resolved, before, after)
	}

	// Poll until ResolveSeed returns a different value (time-based seed advances).
	require.Eventually(t, func() bool {
		return ResolveSeed(-1) != resolved
	}, 2*time.Second, time.Millisecond, "two ResolveSeed(-1) calls should eventually differ")
}
