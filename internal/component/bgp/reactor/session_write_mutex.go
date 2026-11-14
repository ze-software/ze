// Design: docs/architecture/bgp/structural-forwarding.md -- request-scoped route writes
// Related: session_write.go -- the writers that take the write path through the acquisition below
package reactor

import (
	"context"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
)

// sessionWriteMutex has zero-value mutex semantics and cancellable acquisition.
// Its token is allocated once per session, never once per message. A canceled
// waiter, or one whose wait ran out, never acquires ownership later or
// interrupts the current writer.
type sessionWriteMutex struct {
	once  sync.Once
	token chan struct{}
}

func (m *sessionWriteMutex) init() {
	m.once.Do(func() { m.token = make(chan struct{}, 1) })
}

func (m *sessionWriteMutex) Lock() {
	m.init()
	m.token <- struct{}{}
}

func (m *sessionWriteMutex) LockContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.init()
	select {
	case m.token <- struct{}{}:
		if err := ctx.Err(); err != nil {
			m.Unlock()
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// LockWithin takes the mutex if it comes free within wait, and reports whether
// it did. A free mutex costs no timer: one is created, on clk, only when
// another holder has to be waited for.
func (m *sessionWriteMutex) LockWithin(clk clock.Clock, wait time.Duration) bool {
	if m.TryLock() {
		return true
	}
	if wait <= 0 {
		return false
	}

	timer := clk.NewTimer(wait)
	defer timer.Stop()

	select {
	case m.token <- struct{}{}:
		return true
	case <-timer.C():
		return false
	}
}

func (m *sessionWriteMutex) TryLock() bool {
	m.init()
	select {
	case m.token <- struct{}{}:
		return true
	default:
		return false
	}
}

func (m *sessionWriteMutex) Unlock() {
	select {
	case <-m.token:
	default:
		panic("BUG: unlock of unlocked session write mutex")
	}
}

// The write path acquisition.
//
// Every writer of a session socket takes writeMu through one of the forms
// below, or through holdWrites (session_write.go), and never through writeMu
// itself. Each form arms the socket's write deadline once the path is held, so
// the whole budget is spent on the write and none of it on the wait, and
// releaseWrites clears it. A write that passes its budget fails, and the writer
// retires the session (retireWrite). The bound is a property of the path: a
// writer cannot forget it.
//
// A form answers nil with the path held and the budget armed. On any error the
// path is NOT held and the caller MUST NOT call releaseWrites. A socket that
// refuses its deadline is a closed one. An UPDATE writer's form retires the
// session for it, as the write to that socket would have (armWriteOrRetire). A
// control message's form only hands the error back (armWriteOrRelease).
//
// The takers of writeMu that write nothing (negotiateWith, setSendCtxID,
// connectionEstablished, the policy reload barriers, the paths-limit readers)
// lock it directly: they have no write to bound.

// acquireWrite takes the write path, waiting for it as long as it takes, and
// arms the write budget. The caller MUST call releaseWrites when it answers
// nil.
func (s *Session) acquireWrite(budget time.Duration) error {
	s.writeMu.Lock()
	return s.armWriteOrRetire(s.clock.Now(), budget)
}

// acquireWriteContext is acquireWrite for a request-scoped write: the wait for
// the path ends with ctx, and the budget is the shorter of budget and the time
// left to ctx's deadline, so a context with no deadline is still bounded. The
// caller MUST call releaseWrites when it answers nil.
func (s *Session) acquireWriteContext(ctx context.Context, budget time.Duration) error {
	if err := s.writeMu.LockContext(ctx); err != nil {
		return err
	}
	now := s.clock.Now()
	if requested, ok := ctx.Deadline(); ok {
		if left := requested.Sub(now); left < budget {
			budget = left
		}
	}
	return s.armWriteOrRetire(now, budget)
}

// acquireWriteWithin is acquireWrite for a writer that MUST NOT queue behind
// another one for longer than wait: it answers errWritePathBusy when the path
// stayed held that long. The wait runs on the session's clock. The caller MUST
// call releaseWrites when it answers nil.
func (s *Session) acquireWriteWithin(wait, budget time.Duration) error {
	if !s.writeMu.LockWithin(s.clock, wait) {
		return errWritePathBusy
	}
	return s.armWriteOrRelease(s.clock.Now(), budget)
}

// tryAcquireWrite is acquireWrite for a writer that MUST NOT wait at all: it
// answers errWritePathBusy when another writer holds the path. The caller MUST
// call releaseWrites when it answers nil.
func (s *Session) tryAcquireWrite(budget time.Duration) error {
	if !s.writeMu.TryLock() {
		return errWritePathBusy
	}
	return s.armWriteOrRetire(s.clock.Now(), budget)
}

// armWriteBudget arms the socket's write deadline to now plus budget. The
// caller MUST hold writeMu. A held run of messages calls it before each one
// (sendUpdateHeld).
func (s *Session) armWriteBudget(budget time.Duration) error {
	return s.armWriteDeadline(s.clock.Now(), budget)
}

// armWriteDeadline arms the socket's write deadline to now plus budget, and
// records both for the report of an expiry (writeBudgetExpiry). The caller MUST
// hold writeMu, the lock s.conn and s.writeFailed are written under. A session
// with no connection, or one already retired, has nothing to arm: its writer
// answers ErrNotConnected or the retiring error itself.
func (s *Session) armWriteDeadline(now time.Time, budget time.Duration) error {
	if s.conn != nil {
		if s.writeFailed == nil {
			end := now.Add(budget)
			if err := s.conn.SetWriteDeadline(end); err != nil {
				return err
			}
			s.writeBudgetArmed = true
			s.writeBudget = budget
			s.writeBudgetEnd = end
		}
	}
	return nil
}

// armWriteOrRelease completes a control message's acquisition, whose path is
// already held: on a refused deadline it gives the path back, so an error
// always means not held. The session is not retired: a control message that
// could not be sent answers its error, and the read loop ends a dead socket.
func (s *Session) armWriteOrRelease(now time.Time, budget time.Duration) error {
	if err := s.armWriteDeadline(now, budget); err != nil {
		s.writeMu.Unlock()
		return err
	}
	return nil
}

// armWriteOrRetire completes an UPDATE writer's acquisition, whose path is
// already held. A socket that refuses its deadline is closed, and a write to a
// closed socket retires the session: so does this, with the same error the
// write would have met, before it gives the path back.
func (s *Session) armWriteOrRetire(now time.Time, budget time.Duration) error {
	if err := s.armWriteDeadline(now, budget); err != nil {
		s.retireWrite(err)
		s.writeMu.Unlock()
		return err
	}
	return nil
}
