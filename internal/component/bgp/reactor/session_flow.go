// Design: docs/architecture/behavior/peer-lifecycle.md — captured-session admission receipts
// Overview: session.go — BGP session struct and lifecycle

package reactor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
)

// Pause holds the public backpressure claim. It is idempotent and does not
// acknowledge an in-flight message. Its owner MUST call Resume to release it.
// KEEPALIVE writes and all timers remain independent of the read gate.
func (s *Session) Pause() {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	s.publicPause = true
	s.paused.Store(true)
}

// Resume MUST release only the public Pause claim, never an admission receipt.
func (s *Session) Resume() {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	s.publicPause = false
	s.paused.Store(s.admissionClaims != 0)
	s.signalAdmissionLocked()
}

// IsPaused reports either public backpressure or an owned admission stop.
func (s *Session) IsPaused() bool { return s.paused.Load() }

// AdmissionReceipt owns one acknowledged stop on exactly one captured session.
// The caller MUST check its deadline before mutation and MUST Release only after
// compensation or finalization has made admitting input safe. Expiry does not
// automatically resume input through a tentative policy.
// A receipt fences admission, not asynchronous downstream application or writes.
// Those already-accepted consumers remain live and need their own drain receipts.
// Receipts MUST NOT be copied.
type AdmissionReceipt struct {
	session            *Session
	deadline           time.Time
	completionDeadline time.Time
	reserve            time.Duration
	mu                 sync.Mutex
	released           bool
}

// Deadline is the work deadline, reserving the requested abort/resume interval.
func (r *AdmissionReceipt) Deadline() time.Time { return r.deadline }

// CompletionDeadline is the captured hard bound for compensation/finalization and
// resumption, before subtracting reserve. It never moves on a HoldTimer restart.
func (r *AdmissionReceipt) CompletionDeadline() time.Time { return r.completionDeadline }

// Check refuses an unusable receipt or exhausted work budget before apply.
func (r *AdmissionReceipt) Check() error {
	return r.checkDeadline(r.deadline, r.reserve)
}

// CheckCompletion allows compensation/finalization to consume the reserved
// interval, but refuses a released receipt, session loss, or the hard deadline.
// The caller MUST still Release when cleanup finishes or fails.
func (r *AdmissionReceipt) CheckCompletion() error {
	return r.checkDeadline(r.completionDeadline, 0)
}

func (r *AdmissionReceipt) checkDeadline(deadline time.Time, reserve time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.released {
		return fmt.Errorf("admission receipt already released")
	}
	if !r.session.clock.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	if err := r.session.admissionAlive(); err != nil {
		return err
	}
	budget := r.session.HoldBudget()
	if budget.State != fsm.HoldBudgetArmed && budget.State != fsm.HoldBudgetDisabled {
		return fmt.Errorf("session HoldTimer no longer has a live budget (state %d)", budget.State)
	}
	if budget.State == fsm.HoldBudgetArmed && budget.Remaining <= reserve {
		return fmt.Errorf("session budget no longer preserves %s for abort/resume", reserve)
	}
	return nil
}

// Release MUST follow compensation or finalization. It is idempotent and never
// looks up a Peer, so it cannot release a replacement session or another owner.
func (r *AdmissionReceipt) Release() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.released {
		return
	}
	r.released = true
	s := r.session
	s.pauseMu.Lock()
	s.admissionClaims--
	s.paused.Store(s.publicPause || s.admissionClaims != 0)
	s.signalAdmissionLocked()
	s.pauseMu.Unlock()
}

// HoldBudget exposes the actual armed timer, including disabled/unarmed/expired.
func (s *Session) HoldBudget() fsm.HoldBudget { return s.timers.HoldBudget() }

// PauseAdmission stops new semantic admission and waits for the current admission
// to leave, without holding peer/reactor locks or interrupting socket reads.
// ctx MUST have a deadline and reserve MUST leave positive abort/resume time.
// On success the caller MUST Release the receipt. Failure releases only this
// invocation's claim. Neither pause, cancellation nor release resets any timer.
func (s *Session) PauseAdmission(ctx context.Context, reserve time.Duration) (*AdmissionReceipt, error) {
	deadline, bounded := ctx.Deadline()
	if !bounded {
		return nil, fmt.Errorf("admission pause requires a transaction deadline")
	}
	if reserve <= 0 {
		return nil, fmt.Errorf("admission pause requires a transaction deadline and positive abort/resume reserve")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.admissionAlive(); err != nil {
		return nil, err
	}
	budget := s.HoldBudget()
	switch budget.State {
	case fsm.HoldBudgetArmed:
		if budget.Expires.Before(deadline) {
			deadline = budget.Expires
		}
	case fsm.HoldBudgetDisabled:
		// The transaction deadline still bounds a zero-HoldTime session.
	case fsm.HoldBudgetUnspecified, fsm.HoldBudgetUnarmed, fsm.HoldBudgetExpired:
		return nil, fmt.Errorf("cannot pause admission without a live HoldTimer budget (state %d)", budget.State)
	default:
		return nil, fmt.Errorf("unknown HoldTimer budget state %d", budget.State)
	}
	completionDeadline := deadline
	deadline = deadline.Add(-reserve)
	remaining := deadline.Sub(s.clock.Now())
	if remaining <= 0 {
		return nil, fmt.Errorf("insufficient session budget for admission pause and %s abort/resume reserve", reserve)
	}

	receipt := &AdmissionReceipt{
		session: s, deadline: deadline, completionDeadline: completionDeadline, reserve: reserve,
	}
	s.pauseMu.Lock()
	s.admissionClaims++
	s.paused.Store(true)
	s.pauseMu.Unlock()
	accepted := false
	defer func() {
		if !accepted {
			receipt.Release()
		}
	}()
	expired := make(chan struct{})
	timer := s.clock.AfterFunc(max(deadline.Sub(s.clock.Now()), time.Nanosecond), func() { close(expired) })
	defer timer.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := receipt.Check(); err != nil {
			return nil, err
		}
		s.pauseMu.Lock()
		if s.admissionActive == 0 {
			s.pauseMu.Unlock()
			accepted = true
			return receipt, nil
		}
		changed := s.admissionChangedLocked()
		s.pauseMu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-expired:
			return nil, context.DeadlineExceeded
		case <-s.done:
			return nil, ErrConnectionClosed
		}
	}
}

func (s *Session) admissionAlive() error {
	if reason := s.closeReason.Load(); reason != nil {
		return *reason
	}
	if s.tearingDown.Load() {
		return ErrConnectionClosed
	}
	if s.State() != fsm.StateEstablished {
		return fmt.Errorf("admission pause requires the captured established session")
	}
	s.pauseMu.Lock()
	closed := s.admissionClosed
	s.pauseMu.Unlock()
	if closed {
		return ErrConnectionClosed
	}
	return nil
}

// beginAdmission MUST be paired with endAdmission. It covers semantic processing
// and coalesced flushes, never the socket read itself. No mutex survives entry:
// callbacks may re-enter the reactor while the active admission drains.
func (s *Session) beginAdmission() error {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	// Deliberately waits for explicit owner release or real session loss;
	// receipt acquisition and the coordinator's work have separate deadlines.
	for !s.admissionClosed {
		if s.admissionClaims == 0 && s.closeReason.Load() == nil {
			s.admissionActive++
			return nil
		}
		// A published cause is not terminal completion: even a released
		// receipt must not race the producer's pending NOTIFICATION.
		changed := s.admissionChangedLocked()
		s.pauseMu.Unlock()
		<-changed
		s.pauseMu.Lock()
	}
	if reason := s.closeReason.Load(); reason != nil {
		return *reason
	}
	return ErrConnectionClosed
}

// endAdmission MUST run after every successful beginAdmission, even on errors.
func (s *Session) endAdmission() {
	s.pauseMu.Lock()
	s.admissionActive--
	if s.admissionActive == 0 {
		s.signalAdmissionLocked()
	}
	s.pauseMu.Unlock()
}

func (s *Session) admissionChangedLocked() <-chan struct{} {
	if s.resumeCh == nil {
		s.resumeCh = make(chan struct{})
	}
	return s.resumeCh
}

func (s *Session) signalAdmissionLocked() {
	if s.resumeCh != nil {
		close(s.resumeCh)
		s.resumeCh = nil
	}
}

// stopAdmission wakes waiters on real session loss without releasing any owner.
func (s *Session) stopAdmission() {
	s.pauseMu.Lock()
	s.admissionClosed = true
	s.signalAdmissionLocked()
	s.pauseMu.Unlock()
}

func (s *Session) waitForResume(ctx context.Context) error {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	for {
		if s.admissionClosed {
			if reason := s.closeReason.Load(); reason != nil {
				return *reason
			}
			return ErrConnectionClosed
		}
		if !s.paused.Load() && s.closeReason.Load() == nil {
			return nil
		}
		changed := s.admissionChangedLocked()
		s.pauseMu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			s.pauseMu.Lock()
			return ctx.Err()
		}
		s.pauseMu.Lock()
	}
}
