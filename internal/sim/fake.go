package sim

import (
	"context"
	"net"
	"sync"
	"time"
)

// FakeClock is a Clock implementation with controllable time for testing
// and simulation. Time only advances when Advance() is called explicitly.
//
// Minimal implementation: supports Now() and Advance(). Timer methods
// (AfterFunc, NewTimer, After) return inert fakes sufficient for code
// paths that only use Now(). ze-bgp-chaos will extend this with
// time-advancement-triggered timer firing.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewFakeClock creates a FakeClock starting at the given time.
func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start}
}

// Now returns the current fake time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Add shifts the fake clock by d (positive = forward, negative = backward).
// Mirrors time.Time.Add() semantics.
func (c *FakeClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// Set jumps the fake clock to an arbitrary time, forward or backward.
// Use this for DST fall-back simulation (clock goes backward 1 hour)
// or any scenario where Advance is insufficient.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// Sleep is a no-op in FakeClock. Callers should use Advance() to
// control time progression.
func (c *FakeClock) Sleep(time.Duration) {}

// After returns a channel that never fires in this minimal implementation.
// Use Advance() with more advanced fake clocks for timer-driven code.
func (c *FakeClock) After(time.Duration) <-chan time.Time {
	return make(chan time.Time) // blocks forever — sufficient for Now()-only paths
}

// AfterFunc returns an inert fakeTimer. The callback is NOT automatically
// invoked. Sufficient for code paths that only use Now().
func (c *FakeClock) AfterFunc(time.Duration, func()) Timer {
	return &fakeTimer{}
}

// NewTimer returns a fakeTimer with a blocking channel.
// Sufficient for code paths that only use Now().
func (c *FakeClock) NewTimer(time.Duration) Timer {
	return &fakeTimer{ch: make(chan time.Time)}
}

// fakeTimer is a minimal Timer implementation for FakeClock.
// All operations are no-ops; C() returns a blocking channel (or nil for AfterFunc).
type fakeTimer struct {
	ch chan time.Time
}

// Stop is a no-op. Returns true (pretends timer was active).
func (t *fakeTimer) Stop() bool { return true }

// Reset is a no-op. Returns true (pretends timer was active).
func (t *fakeTimer) Reset(time.Duration) bool { return true }

// C returns the timer's channel. Nil for AfterFunc-created timers,
// blocking channel for NewTimer-created timers.
func (t *fakeTimer) C() <-chan time.Time { return t.ch }

// FakeDialer is a Dialer implementation that delegates to a configurable function.
// For testing injection of custom dialers into reactor components.
type FakeDialer struct {
	// DialFunc is called by DialContext. Must be set before use.
	DialFunc func(ctx context.Context, network, address string) (net.Conn, error)
}

// DialContext delegates to DialFunc.
func (d *FakeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.DialFunc(ctx, network, address)
}

// FakeListenerFactory is a ListenerFactory implementation that delegates
// to a configurable function. For testing injection of custom listeners.
type FakeListenerFactory struct {
	// ListenFunc is called by Listen. Must be set before use.
	ListenFunc func(ctx context.Context, network, address string) (net.Listener, error)
}

// Listen delegates to ListenFunc.
func (f *FakeListenerFactory) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	return f.ListenFunc(ctx, network, address)
}
