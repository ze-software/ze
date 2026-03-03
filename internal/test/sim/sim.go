// Package sim provides fake implementations of clock and network interfaces
// for use in unit tests. These are minimal, inert fakes — time only advances
// when the test explicitly calls Add/Set/FireTickers.
//
// For simulation-grade clocks with timer heaps and Advance-driven firing,
// see internal/chaos.VirtualClock.
//
// Design: docs/architecture/chaos-web-dashboard.md — simulation infrastructure
package sim

import (
	"context"
	"net"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
)

// FakeClock is a Clock implementation with controllable time for testing.
// Time only advances when Add() or Set() is called explicitly.
//
// Minimal implementation: supports Now() and Add(). Timer methods
// (AfterFunc, NewTimer, After) return inert fakes sufficient for code
// paths that only use Now().
type FakeClock struct {
	mu      sync.Mutex
	now     time.Time
	tickers []*fakeTicker
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
// or any scenario where Add is insufficient.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// Sleep is a no-op in FakeClock. Callers should use Add() to
// control time progression.
func (c *FakeClock) Sleep(time.Duration) {}

// After returns a channel that never fires in this minimal implementation.
func (c *FakeClock) After(time.Duration) <-chan time.Time {
	return make(chan time.Time) // blocks forever — sufficient for Now()-only paths
}

// AfterFunc returns an inert fakeTimer. The callback is NOT automatically
// invoked. Sufficient for code paths that only use Now().
func (c *FakeClock) AfterFunc(time.Duration, func()) clock.Timer {
	return &fakeTimer{}
}

// NewTimer returns a fakeTimer with a blocking channel.
// Sufficient for code paths that only use Now().
func (c *FakeClock) NewTimer(time.Duration) clock.Timer {
	return &fakeTimer{ch: make(chan time.Time)}
}

// NewTicker returns a fakeTicker with a buffered channel.
// The ticker does not fire autonomously. Use FireTickers() to deliver
// ticks to all active tickers created by this clock.
func (c *FakeClock) NewTicker(time.Duration) clock.Ticker {
	c.mu.Lock()
	defer c.mu.Unlock()
	ft := &fakeTicker{ch: make(chan time.Time, 1)}
	c.tickers = append(c.tickers, ft)
	return ft
}

// FireTickers sends the current fake time to all non-stopped tickers.
// The send is non-blocking (buffered channel, size 1), so it is safe
// to call before the consumer goroutine enters its select loop.
func (c *FakeClock) FireTickers() {
	c.mu.Lock()
	now := c.now
	tickers := append([]*fakeTicker(nil), c.tickers...)
	c.mu.Unlock()
	for _, ft := range tickers {
		if !ft.stopped {
			select {
			case ft.ch <- now:
			default: // buffer full — tick already pending, skip
			}
		}
	}
}

// fakeTimer is a minimal Timer implementation for FakeClock.
type fakeTimer struct {
	ch chan time.Time
}

func (t *fakeTimer) Stop() bool               { return true }
func (t *fakeTimer) Reset(time.Duration) bool { return true }
func (t *fakeTimer) C() <-chan time.Time      { return t.ch }

// fakeTicker is a minimal Ticker implementation for FakeClock.
type fakeTicker struct {
	ch      chan time.Time
	stopped bool
}

func (t *fakeTicker) Stop()               { t.stopped = true }
func (t *fakeTicker) C() <-chan time.Time { return t.ch }

// FakeDialer is a Dialer implementation that delegates to a configurable function.
type FakeDialer struct {
	DialFunc func(ctx context.Context, network, address string) (net.Conn, error)
}

// DialContext delegates to DialFunc.
func (d *FakeDialer) DialContext(ctx context.Context, nw, address string) (net.Conn, error) {
	return d.DialFunc(ctx, nw, address)
}

// FakeListenerFactory is a ListenerFactory implementation that delegates
// to a configurable function.
type FakeListenerFactory struct {
	ListenFunc func(ctx context.Context, network, address string) (net.Listener, error)
}

// Listen delegates to ListenFunc.
func (f *FakeListenerFactory) Listen(ctx context.Context, nw, address string) (net.Listener, error) {
	return f.ListenFunc(ctx, nw, address)
}

// Compile-time interface checks.
var (
	_ clock.Clock             = (*FakeClock)(nil)
	_ network.Dialer          = (*FakeDialer)(nil)
	_ network.ListenerFactory = (*FakeListenerFactory)(nil)
)
