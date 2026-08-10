// Package sim provides fake implementations of clock and network interfaces
// for use in unit tests. Time only advances when the test explicitly calls
// Add/Set/FireTickers.
//
// Design: docs/architecture/chaos-web-dashboard.md — simulation infrastructure
package sim

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/network"
)

// FakeClock is a Clock implementation with controllable time for testing.
// Time only advances when Add() or Set() is called explicitly.
//
// AfterFunc timers are scheduled: their callbacks fire synchronously, in
// deadline order, when Add()/Set() advances past their deadline (matching
// chaos.VirtualClock). NewTimer/After channels remain inert and tickers fire
// only via FireTickers().
type FakeClock struct {
	mu      sync.Mutex
	now     time.Time
	tickers []*fakeTicker
	timers  []*fakeTimer // scheduled AfterFunc timers, fired on Add/Set
	seq     uint64       // FIFO tie-break for same-deadline timers
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

// Add shifts the fake clock by d (positive = forward, negative = backward),
// firing scheduled AfterFunc timers whose deadline is reached when moving
// forward. Mirrors time.Time.Add() semantics.
func (c *FakeClock) Add(d time.Duration) {
	c.mu.Lock()
	target := c.now.Add(d)
	c.mu.Unlock()
	c.advanceTo(target)
}

// Set jumps the fake clock to an arbitrary time, forward or backward, firing
// scheduled AfterFunc timers whose deadline is reached when moving forward.
// Use this for DST fall-back simulation (clock goes backward 1 hour) or any
// scenario where Add is insufficient.
func (c *FakeClock) Set(t time.Time) {
	c.advanceTo(t)
}

// advanceTo moves now to target, firing AfterFunc callbacks whose deadline is
// <= target in deadline order (FIFO for ties). The lock is released before each
// callback, which may take other locks or schedule new timers. Moving backward
// (target <= now) fires nothing.
func (c *FakeClock) advanceTo(target time.Time) {
	for {
		c.mu.Lock()
		var earliest *fakeTimer
		if target.After(c.now) {
			for _, t := range c.timers {
				if t.stopped || t.fired || t.deadline.After(target) {
					continue
				}
				if earliest == nil || t.deadline.Before(earliest.deadline) ||
					(t.deadline.Equal(earliest.deadline) && t.seq < earliest.seq) {
					earliest = t
				}
			}
		}
		if earliest == nil {
			c.now = target
			c.mu.Unlock()
			return
		}
		earliest.fired = true
		c.now = earliest.deadline
		cb := earliest.callback
		c.mu.Unlock()
		if cb != nil {
			cb()
		}
	}
}

// Sleep is a no-op in FakeClock. Callers should use Add() to
// control time progression.
func (c *FakeClock) Sleep(time.Duration) {}

// After returns a channel that never fires in this minimal implementation.
func (c *FakeClock) After(time.Duration) <-chan time.Time {
	return make(chan time.Time) // blocks forever — sufficient for Now()-only paths
}

// AfterFunc schedules f to be called when now+d is reached via Add()/Set().
// The callback fires synchronously during Add()/Set(), in deadline order.
func (c *FakeClock) AfterFunc(d time.Duration, f func()) clock.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{clock: c, deadline: c.now.Add(d), callback: f, seq: c.seq}
	c.seq++
	c.timers = append(c.timers, t)
	return t
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

// fakeTimer implements clock.Timer for FakeClock. AfterFunc timers carry a
// clock pointer, deadline, and callback, and are fired by advanceTo. NewTimer
// timers (clock == nil) are inert channels that never fire.
type fakeTimer struct {
	clock    *FakeClock
	ch       chan time.Time
	deadline time.Time
	callback func()
	seq      uint64
	stopped  bool
	fired    bool
}

// Stop prevents an AfterFunc timer from firing. Returns true if it was still
// active. Inert (NewTimer) timers report true without state.
func (t *fakeTimer) Stop() bool {
	if t.clock == nil {
		return true
	}
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	if t.stopped || t.fired {
		return false
	}
	t.stopped = true
	return true
}

// Reset re-arms an AfterFunc timer to fire d after the current fake time.
func (t *fakeTimer) Reset(d time.Duration) bool {
	if t.clock == nil {
		return true
	}
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	wasActive := !t.stopped && !t.fired
	t.deadline = t.clock.now.Add(d)
	t.stopped = false
	t.fired = false
	t.seq = t.clock.seq
	t.clock.seq++
	return wasActive
}

func (t *fakeTimer) C() <-chan time.Time { return t.ch }

// fakeTicker is a minimal Ticker implementation for FakeClock.
type fakeTicker struct {
	ch      chan time.Time
	stopped bool
}

func (t *fakeTicker) Stop()               { t.stopped = true }
func (t *fakeTicker) C() <-chan time.Time { return t.ch }

// fakeDialer is a Dialer implementation that delegates to a configurable function.
type fakeDialer struct {
	DialFunc func(ctx context.Context, network, address string) (net.Conn, error)
}

// DialContext delegates to DialFunc.
func (d *fakeDialer) DialContext(ctx context.Context, nw, address string) (net.Conn, error) {
	return d.DialFunc(ctx, nw, address)
}

// fakeListenerFactory is a ListenerFactory implementation that delegates
// to a configurable function.
type fakeListenerFactory struct {
	ListenFunc func(ctx context.Context, network, address string) (net.Listener, error)
}

// Listen delegates to ListenFunc.
func (f *fakeListenerFactory) Listen(ctx context.Context, nw, address string) (net.Listener, error) {
	return f.ListenFunc(ctx, nw, address)
}

// Compile-time interface checks.
var (
	_ clock.Clock             = (*FakeClock)(nil)
	_ network.Dialer          = (*fakeDialer)(nil)
	_ network.ListenerFactory = (*fakeListenerFactory)(nil)
)
