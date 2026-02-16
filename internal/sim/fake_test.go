package sim

import (
	"context"
	"net"
	"testing"
	"time"
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

	// Call again — should be identical (time doesn't advance automatically)
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
		t.Errorf("Now() after Advance(5s) = %v, want %v", got, want)
	}

	// Advance again — should be cumulative
	c.Add(10 * time.Second)
	got2 := c.Now()
	want2 := start.Add(15 * time.Second)
	if !got2.Equal(want2) {
		t.Errorf("Now() after Advance(5s+10s) = %v, want %v", got2, want2)
	}
}

// TestFakeClockImplementsClock verifies FakeClock satisfies Clock interface.
//
// VALIDATES: Compile-time interface conformance.
// PREVENTS: Missing methods on FakeClock.
func TestFakeClockImplementsClock(t *testing.T) {
	var _ Clock = &FakeClock{}
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
	}
	// Stop should not panic
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

	// Jump forward
	future := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	c.Set(future)
	if got := c.Now(); !got.Equal(future) {
		t.Errorf("Now() after Set(future) = %v, want %v", got, future)
	}

	// Jump backward (DST fall-back scenario)
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

	// 2025-11-02 is DST fall-back in US Eastern: 1:59 AM EDT → 1:00 AM EST
	// Construct times from UTC to avoid ambiguity (Go's time.Date picks first
	// offset for ambiguous wall-clock times, which gives EDT not EST).
	//
	// 1:30 AM EDT = 05:30 UTC (before fall-back)
	// 1:00 AM EST = 06:00 UTC (after fall-back — same wall-clock, later absolute time)
	beforeFallBack := time.Date(2025, 11, 2, 5, 30, 0, 0, time.UTC).In(loc)
	afterFallBack := time.Date(2025, 11, 2, 6, 0, 0, 0, time.UTC).In(loc)

	c := NewFakeClock(beforeFallBack)

	// Wall-clock: 1:30 AM EDT → 1:00 AM EST (looks backward)
	// Absolute time: 05:30 UTC → 06:00 UTC (actually forward)
	c.Set(afterFallBack)

	// Key insight: time.After() compares absolute instants, not wall-clock.
	// Even though wall-clock "went backward" (1:30→1:00), absolute time advanced.
	now := c.Now()
	if !now.After(beforeFallBack) {
		t.Errorf("expected 1:00 AM EST (06:00 UTC) to be after 1:30 AM EDT (05:30 UTC)")
	}

	// Verify the wall-clock display shows the "backward" jump
	if beforeFallBack.Hour() != 1 || afterFallBack.Hour() != 1 {
		t.Errorf("expected both times to show hour=1, got before=%d after=%d",
			beforeFallBack.Hour(), afterFallBack.Hour())
	}

	// But the UTC times confirm forward progress
	if afterFallBack.UTC().Before(beforeFallBack.UTC()) {
		t.Error("expected afterFallBack to be later in UTC")
	}
}

// TestFakeDialerImplementsDialer verifies FakeDialer satisfies Dialer interface.
//
// VALIDATES: Compile-time interface conformance.
// PREVENTS: Missing methods on FakeDialer.
func TestFakeDialerImplementsDialer(t *testing.T) {
	var _ Dialer = &FakeDialer{}
}

// TestFakeDialerDelegates verifies FakeDialer calls DialFunc.
//
// VALIDATES: DialContext delegates to the configured function.
// PREVENTS: DialFunc being ignored.
func TestFakeDialerDelegates(t *testing.T) {
	called := false
	d := &FakeDialer{
		DialFunc: func(_ context.Context, network, address string) (net.Conn, error) {
			called = true
			if network != "tcp" {
				t.Errorf("network = %q, want %q", network, "tcp")
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
	var _ ListenerFactory = &FakeListenerFactory{}
}

// TestFakeListenerFactoryDelegates verifies FakeListenerFactory calls ListenFunc.
//
// VALIDATES: Listen delegates to the configured function.
// PREVENTS: ListenFunc being ignored.
func TestFakeListenerFactoryDelegates(t *testing.T) {
	called := false
	f := &FakeListenerFactory{
		ListenFunc: func(_ context.Context, network, address string) (net.Listener, error) {
			called = true
			if network != "tcp" {
				t.Errorf("network = %q, want %q", network, "tcp")
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
