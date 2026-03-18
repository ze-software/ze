package sim

import (
	"context"
	"net"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
)

// TestFakeClockNow verifies FakeClock returns the configured time.
//
// VALIDATES: Now() returns the start time without advancing.
// PREVENTS: FakeClock using real time instead of fake time.
func TestFakeClockNow(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewFakeClock(start)

	got := c.Now()
	if !got.Equal(start) {
		t.Errorf("Now() = %v, want %v", got, start)
	}

	got2 := c.Now()
	if !got2.Equal(start) {
		t.Errorf("second Now() = %v, want %v", got2, start)
	}
}

// TestFakeClockAdd verifies Add shifts time by the given duration.
//
// VALIDATES: Add() changes what Now() returns, cumulative across calls.
// PREVENTS: Add being a no-op or using wrong arithmetic.
func TestFakeClockAdd(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewFakeClock(start)

	c.Add(5 * time.Second)
	got := c.Now()
	want := start.Add(5 * time.Second)
	if !got.Equal(want) {
		t.Errorf("Now() after Add(5s) = %v, want %v", got, want)
	}

	c.Add(10 * time.Second)
	got2 := c.Now()
	want2 := start.Add(15 * time.Second)
	if !got2.Equal(want2) {
		t.Errorf("Now() after Add(5s+10s) = %v, want %v", got2, want2)
	}
}

// TestFakeClockImplementsClock verifies FakeClock satisfies clock.Clock interface.
//
// VALIDATES: Compile-time interface conformance.
// PREVENTS: Missing methods on FakeClock.
func TestFakeClockImplementsClock(t *testing.T) {
	var _ clock.Clock = &FakeClock{}
}

// TestFakeClockSleepIsNoOp verifies Sleep doesn't block.
//
// VALIDATES: Sleep returns immediately in FakeClock.
// PREVENTS: Tests hanging on Sleep calls in production code.
func TestFakeClockSleepIsNoOp(t *testing.T) {
	c := NewFakeClock(time.Now())
	before := time.Now()
	c.Sleep(time.Hour)
	elapsed := time.Since(before)
	if elapsed > time.Second {
		t.Errorf("Sleep blocked for %v, expected immediate return", elapsed)
	}
}

// TestFakeClockAfterFuncReturnsTimer verifies AfterFunc returns a valid Timer.
//
// VALIDATES: AfterFunc returns non-nil Timer satisfying the interface.
// PREVENTS: Nil pointer dereference when production code calls AfterFunc.
func TestFakeClockAfterFuncReturnsTimer(t *testing.T) {
	c := NewFakeClock(time.Now())
	timer := c.AfterFunc(time.Second, func() {})
	if timer == nil {
		t.Fatal("AfterFunc returned nil")
		return
	}
	timer.Stop()
}

// TestFakeClockNewTimerReturnsTimer verifies NewTimer returns a valid Timer.
//
// VALIDATES: NewTimer returns non-nil Timer with non-nil C() channel.
// PREVENTS: Nil channel panics when production code selects on timer.C().
func TestFakeClockNewTimerReturnsTimer(t *testing.T) {
	c := NewFakeClock(time.Now())
	timer := c.NewTimer(time.Second)
	if timer == nil {
		t.Fatal("NewTimer returned nil")
		return
	}
	if timer.C() == nil {
		t.Error("NewTimer.C() returned nil, expected non-nil channel")
	}
	timer.Stop()
}

// TestFakeClockSet verifies Set jumps clock to arbitrary time.
//
// VALIDATES: Set() changes Now() to the given time, including backward jumps.
// PREVENTS: Inability to simulate DST fall-back (clock goes backward 1 hour).
func TestFakeClockSet(t *testing.T) {
	start := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	c := NewFakeClock(start)

	future := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	c.Set(future)
	if got := c.Now(); !got.Equal(future) {
		t.Errorf("Now() after Set(future) = %v, want %v", got, future)
	}

	past := time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)
	c.Set(past)
	if got := c.Now(); !got.Equal(past) {
		t.Errorf("Now() after Set(past) = %v, want %v", got, past)
	}
}

// TestFakeClockDSTFallBack simulates DST fall-back where wall-clock goes backward.
//
// VALIDATES: Set() enables DST simulation; Go's time.After() uses absolute instants.
// PREVENTS: Broken TTL when wall-clock appears to go backward during DST fall-back.
func TestFakeClockDSTFallBack(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone America/New_York not available: %v", err)
	}

	beforeFallBack := time.Date(2025, 11, 2, 5, 30, 0, 0, time.UTC).In(loc)
	afterFallBack := time.Date(2025, 11, 2, 6, 0, 0, 0, time.UTC).In(loc)

	c := NewFakeClock(beforeFallBack)
	c.Set(afterFallBack)

	now := c.Now()
	if !now.After(beforeFallBack) {
		t.Errorf("expected 1:00 AM EST (06:00 UTC) to be after 1:30 AM EDT (05:30 UTC)")
	}

	if beforeFallBack.Hour() != 1 || afterFallBack.Hour() != 1 {
		t.Errorf("expected both times to show hour=1, got before=%d after=%d",
			beforeFallBack.Hour(), afterFallBack.Hour())
	}

	if afterFallBack.UTC().Before(beforeFallBack.UTC()) {
		t.Error("expected afterFallBack to be later in UTC")
	}
}

// TestFakeClockNewTickerReturnsTicker verifies FakeClock.NewTicker returns a valid Ticker.
//
// VALIDATES: NewTicker returns non-nil Ticker with non-nil C() channel.
// PREVENTS: Nil channel panics when production code selects on ticker.C().
func TestFakeClockNewTickerReturnsTicker(t *testing.T) {
	c := NewFakeClock(time.Now())
	ticker := c.NewTicker(time.Second)
	if ticker == nil {
		t.Fatal("NewTicker returned nil")
		return
	}
	if ticker.C() == nil {
		t.Error("NewTicker.C() returned nil, expected non-nil channel")
	}
	ticker.Stop()
}

// TestFakeClockFireTickers verifies FireTickers sends the current fake time to active tickers.
//
// VALIDATES: FireTickers delivers a tick with the current fake time.
// PREVENTS: Background goroutines starving when using FakeClock.
func TestFakeClockFireTickers(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewFakeClock(start)
	ticker := c.NewTicker(time.Second)
	defer ticker.Stop()

	c.FireTickers()

	select {
	case tm := <-ticker.C():
		if !tm.Equal(start) {
			t.Errorf("tick time = %v, want %v", tm, start)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("FireTickers did not deliver tick")
	}
}

// TestFakeTickerStopPreventsFire verifies a stopped ticker does not receive from FireTickers.
//
// VALIDATES: Stop() prevents subsequent ticks from being delivered.
// PREVENTS: Stopped tickers continuing to receive events.
func TestFakeTickerStopPreventsFire(t *testing.T) {
	c := NewFakeClock(time.Now())
	ticker := c.NewTicker(time.Second)
	ticker.Stop()

	c.FireTickers()

	select {
	case <-ticker.C():
		t.Error("stopped ticker should not receive ticks")
	case <-time.After(50 * time.Millisecond):
		// OK — stopped ticker didn't fire
	}
}

// TestFakeDialerImplementsDialer verifies FakeDialer satisfies network.Dialer interface.
//
// VALIDATES: Compile-time interface conformance.
// PREVENTS: Missing methods on FakeDialer.
func TestFakeDialerImplementsDialer(t *testing.T) {
	var _ network.Dialer = &FakeDialer{}
}

// TestFakeDialerDelegates verifies FakeDialer calls DialFunc.
//
// VALIDATES: DialContext delegates to the configured function.
// PREVENTS: DialFunc being ignored.
func TestFakeDialerDelegates(t *testing.T) {
	called := false
	d := &FakeDialer{
		DialFunc: func(_ context.Context, nw, address string) (net.Conn, error) {
			called = true
			if nw != "tcp" {
				t.Errorf("network = %q, want %q", nw, "tcp")
			}
			if address != "10.0.0.1:179" {
				t.Errorf("address = %q, want %q", address, "10.0.0.1:179")
			}
			return nil, nil //nolint:nilnil // test fake returns no connection
		},
	}

	conn, err := d.DialContext(context.Background(), "tcp", "10.0.0.1:179")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn != nil {
		t.Error("expected nil conn from fake")
	}
	if !called {
		t.Error("DialFunc was not called")
	}
}

// TestFakeListenerFactoryImplementsListenerFactory verifies interface conformance.
//
// VALIDATES: Compile-time interface conformance.
// PREVENTS: Missing methods on FakeListenerFactory.
func TestFakeListenerFactoryImplementsListenerFactory(t *testing.T) {
	var _ network.ListenerFactory = &FakeListenerFactory{}
}

// TestFakeListenerFactoryDelegates verifies FakeListenerFactory calls ListenFunc.
//
// VALIDATES: Listen delegates to the configured function.
// PREVENTS: ListenFunc being ignored.
func TestFakeListenerFactoryDelegates(t *testing.T) {
	called := false
	f := &FakeListenerFactory{
		ListenFunc: func(_ context.Context, nw, address string) (net.Listener, error) {
			called = true
			if nw != "tcp" {
				t.Errorf("network = %q, want %q", nw, "tcp")
			}
			if address != ":179" {
				t.Errorf("address = %q, want %q", address, ":179")
			}
			return nil, nil //nolint:nilnil // test fake returns no listener
		},
	}

	ln, err := f.Listen(context.Background(), "tcp", ":179")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ln != nil {
		t.Error("expected nil listener from fake")
	}
	if !called {
		t.Error("ListenFunc was not called")
	}
}
