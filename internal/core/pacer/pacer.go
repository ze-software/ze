// Design: docs/architecture/core-design.md -- core leaf packages

// Package pacer paces the retries of a goroutine whose read keeps failing.
// A receiver goroutine on a socket answers a transient error by retrying at
// once, and a persistent one by retrying less and less often up to a fixed
// ceiling, so a socket that never recovers costs a bounded, small slice of a
// core rather than all of it.
package pacer

import "time"

// step is the delay a Pacer applies after the second consecutive failure,
// doubling after every failure beyond that until it reaches ceiling.
// ceiling is the longest a Pacer ever makes a caller wait between two
// retries.
//
// Both are fixed rather than configurable: an operator has no information
// with which to choose them, and a wrong value reintroduces the spin this
// package exists to remove. 250ms was chosen as the worst-case pause a
// subscriber-facing receiver goroutine adds between retries of a socket
// that fails on every read: at the ceiling a loop retries about four times
// a second rather than the hundreds of thousands a bare `continue`
// produces, and a transient failure's recovery is never delayed by more
// than that, which is unnoticeable next to session set-up itself.
const (
	step    = 10 * time.Millisecond
	ceiling = 250 * time.Millisecond
)

// Pacer paces the retries of one receiver goroutine reading from a socket
// that keeps failing. The zero value is ready to use and behaves as though
// the goroutine's last read succeeded, so the first failure it ever meets
// retries at once.
//
// Not safe for concurrent use. A Pacer belongs to exactly one goroutine, the
// same goroutine that owns the read loop calling Wait.
type Pacer struct {
	delay time.Duration // the delay Wait applies for the NEXT failure; zero means retry at once
	timer *time.Timer   // reused across waits so Wait allocates nothing after the first one
}

// Wait paces one retry after a failed read. Call it once per failure, where
// the loop used to `continue` at once. It returns true when stop fired
// before the delay elapsed; the caller MUST return rather than retry, the
// same way it already returns when its own exit signal fires directly.
// stop is whatever exit signal the caller already owns: a context's Done(),
// or a plain stop channel like UDPListener's -- both are <-chan struct{},
// so Wait forces neither shape on a caller that already has the other.
func (p *Pacer) Wait(stop <-chan struct{}) bool {
	delay := p.next()
	if delay == 0 {
		return false
	}

	if p.timer == nil {
		p.timer = time.NewTimer(delay)
	} else {
		p.timer.Reset(delay)
	}

	select {
	case <-p.timer.C:
		return false
	case <-stop:
		if !p.timer.Stop() {
			<-p.timer.C
		}
		return true
	}
}

// Succeed resets the pacer after a successful read, so the very next
// failure retries at once again rather than at whatever delay the last run
// of failures had grown to.
func (p *Pacer) Succeed() {
	p.delay = 0
}

// next returns the delay for the failure Wait is pacing and advances the
// pacer's state for the one after it. Split out from Wait so a test can walk
// the growth curve without waiting out real time.
func (p *Pacer) next() time.Duration {
	delay := p.delay
	if delay == 0 {
		p.delay = step
	} else {
		p.delay = min(delay*2, ceiling)
	}
	return delay
}
