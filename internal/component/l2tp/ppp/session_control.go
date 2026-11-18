// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact PPP admission capabilities
// Related: manager.go -- canonical admission and worker ownership
// Related: events.go -- admitted lifecycle events and refused attempts

package ppp

import (
	"errors"
	"reflect"
	"sync"
)

// SessionAttempt is one transport admission's immutable handoff identity. Each
// successful NewSessionAttempt call mints a distinct binding, even if an ID or
// lease was used before. Copies compare equal only for that same handoff.
// Its zero value is invalid and cannot identify a transport admission.
type SessionAttempt struct {
	binding *sessionAttemptBinding
}

type sessionAttemptBinding struct {
	subscriberID  string
	lease         SessionHandle
	mu            sync.Mutex
	claimed       bool
	canceled      bool
	finished      bool
	leaseAcquired bool
	owner         *pppSession
	control       SessionControl
	unusedCleanup *untransferredClose
}

// A nonnil untransferredClose records an actual producer Close. A nil cause is
// a confirmed physical release, distinct from an absent observation.
type untransferredClose struct{ cause error }

// NewSessionAttempt validates identity before the transport queues StartSession.
// It borrows the lease without transferring or closing it. The transport MUST
// retain this exact attempt beside its Opened owner and pass it unchanged on
// StartSession. On an error the transport still owns and MUST close its owner.
// Channel delivery transfers the PPP lease as StartSession documents.
func NewSessionAttempt(subscriberID string, lease SessionHandle) (SessionAttempt, error) {
	if subscriberID == "" {
		return SessionAttempt{}, errors.New("ppp: session attempt requires a canonical subscriber identity")
	}
	if sessionResourceMissing(lease) {
		return SessionAttempt{}, errors.New("ppp: session attempt requires a data-plane lease")
	}
	if !reflect.ValueOf(lease).Comparable() {
		return SessionAttempt{}, errors.New("ppp: session attempt requires a comparable data-plane lease")
	}
	return SessionAttempt{binding: &sessionAttemptBinding{subscriberID: subscriberID, lease: lease}}, nil
}

// SubscriberID returns the exact opaque ID supplied at construction. The empty
// result identifies only the invalid zero attempt; it never supplies a fallback.
func (attempt SessionAttempt) SubscriberID() string {
	if attempt.binding == nil {
		return ""
	}
	return attempt.binding.subscriberID
}

// Cancel revokes only this handoff. Before admission it prevents protocol I/O;
// after binding it signals only the original worker. The caller still MUST
// close a transport/unused lease it never transfers. No mutex is held while
// signaling the worker, and no successor can be selected by ID or lease.
func (attempt SessionAttempt) Cancel() bool {
	if attempt.binding == nil {
		return false
	}
	binding := attempt.binding
	binding.mu.Lock()
	binding.canceled = true
	control := binding.control
	binding.mu.Unlock()
	if control.worker != nil {
		control.Cancel()
	}
	return true
}

// RecordUntransferredClose records the actual Close of this canceled handoff's
// exact unused owner. The producer MUST call it only after closing that owner
// and MUST NOT call it after delivering StartSession to the Driver. It records
// no protocol admission, grants no close authority and never changes a claimed
// binding or a previously recorded outcome. Safe for concurrent observation.
func (attempt SessionAttempt) RecordUntransferredClose(owner SessionHandle, cause error) error {
	if attempt.binding == nil {
		return ErrSessionAttemptInvalid
	}
	binding := attempt.binding
	binding.mu.Lock()
	defer binding.mu.Unlock()
	if binding.lease != owner {
		return ErrSessionAttemptInvalid
	}
	if binding.claimed {
		return ErrSessionAttemptUsed
	}
	if !binding.canceled {
		return ErrSessionNotCanceled
	}
	binding.claimed = true
	binding.finished = true
	binding.unusedCleanup = &untransferredClose{cause: cause}
	return nil
}

// Control returns only the once-bound admitted worker, including after it
// retires. A refused attempt never fabricates an admitted Control.
func (attempt SessionAttempt) Control() (SessionControl, bool) {
	if attempt.binding == nil {
		return SessionControl{}, false
	}
	attempt.binding.mu.Lock()
	control := attempt.binding.control
	attempt.binding.mu.Unlock()
	return control, control.worker != nil
}

func (attempt SessionAttempt) admissionState() (claimed, canceled bool) {
	if attempt.binding == nil {
		return false, false
	}
	attempt.binding.mu.Lock()
	claimed, canceled = attempt.binding.claimed, attempt.binding.canceled
	attempt.binding.mu.Unlock()
	return
}

// Caller holds Driver.mu. Attempt observers release binding.mu BEFORE Driver.mu.
func (attempt SessionAttempt) claim(owner *pppSession, admit bool) error {
	if attempt.binding == nil {
		return ErrSessionAttemptInvalid
	}
	binding := attempt.binding
	binding.mu.Lock()
	defer binding.mu.Unlock()
	if binding.claimed {
		return ErrSessionAttemptUsed
	}
	if admit && binding.canceled {
		return ErrSessionAttemptCanceled
	}
	binding.claimed = true
	binding.leaseAcquired = true
	binding.owner = owner
	if admit {
		binding.control = owner.control
	}
	return nil
}

// An existing lease is never acquired or closed by this refusal. Do not alter
// the binding of a prior claimant (including a concurrent original admission).
func (attempt SessionAttempt) finishUnowned() {
	if attempt.binding == nil {
		return
	}
	binding := attempt.binding
	binding.mu.Lock()
	if !binding.claimed {
		binding.claimed = true
		binding.finished = true
	}
	binding.mu.Unlock()
}

func (attempt SessionAttempt) finishRefusal() {
	if attempt.binding == nil {
		return
	}
	attempt.binding.mu.Lock()
	attempt.binding.finished = true
	attempt.binding.mu.Unlock()
}

// hasLease is used only after raw ownership was checked comparable. A malformed
// raw ID cannot authorize closing a lease already claimed through this binding.
func (attempt SessionAttempt) hasLease(lease SessionHandle) bool {
	return attempt.binding != nil && attempt.binding.lease == lease
}

// matches is called only after admission validated the supplied lease as
// comparable. The private binding cannot be rewritten by a transport caller.
func (attempt SessionAttempt) matches(subscriberID string, lease SessionHandle) bool {
	if attempt.binding == nil {
		return false
	}
	if attempt.binding.subscriberID != subscriberID {
		return false
	}
	return attempt.binding.lease == lease
}

// SessionControl is the capability of one validated, admitted PPP worker.
// Only admission mints it. A copied control always refers to that same worker,
// even after its canonical ID and lease are reused by a successor. Its zero
// value is invalid. It is local to the PPP transport boundary, never serialized
// as a subscriber event or given to an external plugin process.
type SessionControl struct {
	driver *Driver
	worker *pppSession
}

// SubscriberID returns this worker's immutable opaque admission identity.
// An invalid zero control returns empty, never a reconstructed identity.
func (control SessionControl) SubscriberID() string {
	if control.worker == nil {
		return ""
	}
	return control.worker.subscriberID
}

// Attempt is the exact transport handoff that produced this worker. A transport
// MUST compare it with its retained admission before binding or acting on an
// initial control. This also fences a delayed rejection before PPP admission.
func (control SessionControl) Attempt() SessionAttempt {
	if control.worker == nil {
		return SessionAttempt{}
	}
	return control.worker.attempt
}

// Active reports whether this exact worker still owns its admitted key and
// lease and has not been canceled. A transport MUST recheck its own exact
// session and this control after an external operation, before acknowledging
// an assignment. False never authorizes acting on a key's current occupant.
func (control SessionControl) Active() bool {
	if control.driver == nil {
		return false
	}
	_, err := control.driver.liveSession(control)
	return err == nil
}

// Cancel signals only the captured worker, without joining it, acknowledging an
// assignment, looking up a current ID or lease, or releasing its admission key.
// The worker owns cleanup and removal when it exits. Repeated calls are safe;
// a stale control can never cancel a successor. False means the zero control.
// Safe for concurrent use.
func (control SessionControl) Cancel() bool {
	if control.worker == nil {
		return false
	}
	control.worker.sessStopOnce.Do(func() { close(control.worker.sessStop) })
	return true
}

// sessionForControlLocked verifies the captured worker against both indexes.
// Matching an ID alone or matching a recycled lease alone grants no authority.
// Caller MUST hold d.mu; the returned pointer never changes its incarnation.
func (d *Driver) sessionForControlLocked(control SessionControl) (*pppSession, bool) {
	if control.driver != d {
		return nil, false
	}
	if control.worker == nil {
		return nil, false
	}
	if d.stopped {
		return nil, false
	}
	session := control.worker
	if d.sessions[session.subscriberID] != session {
		return nil, false
	}
	if d.owners[session.dataplane] != session {
		return nil, false
	}
	return session, true
}
