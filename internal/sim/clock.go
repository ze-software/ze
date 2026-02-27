// Package sim provides injectable abstractions for time and network operations.
//
// Production code uses RealClock and RealDialer/RealListenerFactory which delegate
// directly to the standard library with zero overhead beyond interface dispatch.
// Simulation and testing code can inject virtual clocks and mock networks for
// deterministic, fast execution.
//
// Design: DST analysis Section 4.4 — interface injection, idiomatic Go.
package sim

import "time"

// Clock abstracts time operations for injectable simulation.
//
// Production code uses RealClock{}. Simulation code provides a virtual clock
// that controls time advancement, enabling 100-1000x speedup.
//
// All methods match their time package counterparts in semantics.
type Clock interface {
	// Now returns the current time.
	Now() time.Time

	// Sleep pauses the current goroutine for at least duration d.
	Sleep(d time.Duration)

	// After waits for duration d to elapse and then sends the current time
	// on the returned channel.
	After(d time.Duration) <-chan time.Time

	// AfterFunc waits for duration d to elapse and then calls f in its own goroutine.
	// It returns a Timer that can be used to cancel the call.
	// The returned Timer's Stop method returns false if f has already been called.
	AfterFunc(d time.Duration, f func()) Timer

	// NewTimer creates a new Timer that will send the current time on its
	// channel after at least duration d.
	NewTimer(d time.Duration) Timer

	// NewTicker returns a new Ticker containing a channel that will send
	// the current time on the channel after each tick. The period of the
	// ticks is specified by the duration argument.
	NewTicker(d time.Duration) Ticker
}

// Ticker abstracts a repeating ticker for injectable simulation.
//
// In production, wraps *time.Ticker. In simulation, ticks are delivered by the
// clock implementation (FakeClock.FireTickers or VirtualClock.Advance).
type Ticker interface {
	// Stop turns off a ticker. After Stop, no more ticks will be sent.
	// Stop does not close the channel, to prevent a concurrent goroutine
	// reading from the channel from seeing an erroneous "tick".
	Stop()

	// C returns the Ticker's channel. A tick is sent on C after each interval.
	C() <-chan time.Time
}

// Timer abstracts a single event timer for injectable simulation.
//
// In production, wraps *time.Timer. In simulation, controlled by virtual clock.
type Timer interface {
	// Stop prevents the Timer from firing.
	// Returns true if the call stops the timer, false if the timer
	// has already expired or been stopped.
	Stop() bool

	// Reset changes the timer to expire after duration d.
	// Returns true if the timer had been active, false if it had expired or been stopped.
	Reset(d time.Duration) bool

	// C returns the Timer's channel. After the timer fires, the current time
	// is sent on C. For timers created with AfterFunc, C returns nil.
	C() <-chan time.Time
}

// RealClock implements Clock using the standard time package.
// Zero allocation, zero overhead beyond interface dispatch.
type RealClock struct{}

// Now returns time.Now().
func (RealClock) Now() time.Time { return time.Now() }

// Sleep calls time.Sleep(d).
func (RealClock) Sleep(d time.Duration) { time.Sleep(d) }

// After calls time.After(d).
func (RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// AfterFunc calls time.AfterFunc(d, f) and wraps the result.
func (RealClock) AfterFunc(d time.Duration, f func()) Timer {
	return &realTimer{timer: time.AfterFunc(d, f)}
}

// NewTimer calls time.NewTimer(d) and wraps the result.
func (RealClock) NewTimer(d time.Duration) Timer {
	return &realTimer{timer: time.NewTimer(d)}
}

// NewTicker calls time.NewTicker(d) and wraps the result.
func (RealClock) NewTicker(d time.Duration) Ticker {
	return &realTicker{ticker: time.NewTicker(d)}
}

// realTicker wraps *time.Ticker to implement the Ticker interface.
type realTicker struct {
	ticker *time.Ticker
}

// Stop delegates to (*time.Ticker).Stop().
func (t *realTicker) Stop() { t.ticker.Stop() }

// C returns the ticker's channel.
func (t *realTicker) C() <-chan time.Time { return t.ticker.C }

// realTimer wraps *time.Timer to implement the Timer interface.
type realTimer struct {
	timer *time.Timer
}

// Stop delegates to (*time.Timer).Stop().
func (t *realTimer) Stop() bool { return t.timer.Stop() }

// Reset delegates to (*time.Timer).Reset(d).
func (t *realTimer) Reset(d time.Duration) bool { return t.timer.Reset(d) }

// C returns the timer's channel. For AfterFunc timers, this returns nil
// (same as *time.Timer.C for AfterFunc-created timers).
func (t *realTimer) C() <-chan time.Time { return t.timer.C }
