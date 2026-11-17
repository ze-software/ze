// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact cleanup observations
// Related: native_release_budget.go -- serialized owner Close and retired retention
package ppp

import (
	"context"
	"errors"
)

var (
	ErrIPv6CleanupPending     = errors.New("ppp: exact retired IPv6 child cleanup is unconfirmed")
	ErrSessionNotCanceled     = errors.New("ppp: exact session must be canceled before closing its owner")
	ErrCleanupInProgress      = errors.New("ppp: exact owner cleanup is already in progress")
	ErrSessionAttemptInvalid  = errors.New("ppp: session attempt is invalid")
	ErrSessionAttemptUsed     = errors.New("ppp: session attempt has already been claimed")
	ErrSessionAttemptCanceled = errors.New("ppp: session attempt was canceled before admission")
)

// SessionCleanupStatus observes THIS admitted worker. Retired means run
// returned; Released means its physical owner release is confirmed, which may
// precede retirement after cancellation. Err retains actual physical Close,
// owned IPv6 child and native-pending failures, never wait/contention errors. Native
// release can be confirmed despite a local descriptor error: callers MUST
// require Released AND Err==nil before claiming successful cleanup.
type SessionCleanupStatus struct {
	Retired  bool
	Released bool
	Err      error
}

// capturedWorker validates the immutable minted relationship, never an index,
// Active, a current ID/lease owner, or native state.
func (control SessionControl) capturedWorker() bool {
	if control.driver == nil || control.worker == nil {
		return false
	}
	return control.worker.driver == control.driver && control.worker.control == control
}

// CleanupStatus is a pure short-Driver-lock snapshot, including after Stop,
// replacement, or completed index release. It performs no native I/O and never
// waits on the close gate. It does not certify rate/pool/accounting operations
// owned by the transport.
func (control SessionControl) CleanupStatus() (SessionCleanupStatus, error) {
	if !control.capturedWorker() {
		return SessionCleanupStatus{}, ErrSessionNotFound
	}
	d, s := control.driver, control.worker
	d.mu.Lock()
	status := SessionCleanupStatus{Retired: s.nativeRetired, Released: s.ownerReleased, Err: sessionCleanupErrorLocked(s)}
	d.mu.Unlock()
	return status, nil
}

// WaitRetired waits ONLY the captured existing worker.done, which closes after
// its deferred first retirement reconciliation. Nil error means retirement was
// observed, never that Released or Err implies success. The caller MUST use a
// nonnil context, and MUST NOT wait from a callback holding a receipt that this
// worker awaits. Cancellation returns promptly even while physical Close runs.
func (control SessionControl) WaitRetired(ctx context.Context) (SessionCleanupStatus, error) {
	if !control.capturedWorker() {
		return SessionCleanupStatus{}, ErrSessionNotFound
	}
	if ctx == nil {
		return SessionCleanupStatus{}, errors.New("ppp: retirement wait requires a context")
	}
	if err := ctx.Err(); err != nil {
		status, _ := control.CleanupStatus()
		return status, err
	}
	select {
	case <-control.worker.done:
		return control.CleanupStatus()
	case <-ctx.Done():
		status, _ := control.CleanupStatus()
		return status, ctx.Err()
	}
}

// CloseCanceledOwner closes only this captured, explicitly canceled owner. It
// NEVER joins worker.done or waits for event ACK. Busy/expired calls change no
// stored result. Nil means the actual/cached exact Close succeeded AND native
// release is confirmed. Native descriptor errors remain errors even when its
// unit is released. The caller MUST retain its existing admission slot until
// CleanupStatus reports Released && Err==nil; no quota/MaxSessions is clamped.
// Once dispatched, contextless Close uses the backend's own bounded operation;
// the context cannot interrupt it. Callers MUST NOT hold a receipt/operation
// gate whose completion that Close needs. An in-flight/live IPv6 service
// returns ErrCleanupInProgress until its worker-owned stop confirms cleanup.
func (control SessionControl) CloseCanceledOwner(ctx context.Context) error {
	if !control.capturedWorker() {
		return ErrSessionNotFound
	}
	if ctx == nil {
		return errors.New("ppp: canceled-owner close requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s, d := control.worker, control.driver
	select {
	case <-s.sessStop:
	default:
		return ErrSessionNotCanceled
	}
	if !s.nativeReleaseMu.TryLock() {
		return ErrCleanupInProgress
	}
	defer s.nativeReleaseMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	if s.ownerServicePending {
		cause := s.ownerServiceErr
		d.mu.Unlock()
		return errors.Join(ErrCleanupInProgress, cause)
	}
	if s.nativeReleased || s.ownerReleased {
		err := s.nativeReleaseErr
		d.mu.Unlock()
		return err
	}
	if d.owners[s.dataplane] != s {
		d.mu.Unlock()
		return ErrSessionNotFound
	}
	d.mu.Unlock()
	// The emitting worker may hold its own mutex. Only exact physical Close
	// belongs here; worker-owned IPv6 cleanup must already be confirmed.
	err := d.closeSessionOwnerLocked(s)
	d.releaseRetiredOwnerLocked(s)
	return err
}

// AttemptCleanupStatus distinguishes ownership refusal from deletion. No lease
// acquired NEVER means another worker's resource was released. Finished means
// rejection processing resolved, or the admitted protocol run returned.
type AttemptCleanupStatus struct {
	Admitted      bool
	Finished      bool
	LeaseAcquired bool
	Released      bool
	Err           error
}

// CleanupStatus observes this once-claimed handoff without manufacturing a
// Control for a rejection. Rejection reason is carried by its event, not Err.
func (attempt SessionAttempt) CleanupStatus() (AttemptCleanupStatus, error) {
	if attempt.binding == nil {
		return AttemptCleanupStatus{}, ErrSessionNotFound
	}
	binding := attempt.binding
	binding.mu.Lock()
	owner, control := binding.owner, binding.control
	status := AttemptCleanupStatus{Admitted: control.worker != nil, Finished: binding.finished, LeaseAcquired: binding.leaseAcquired}
	binding.mu.Unlock()
	if owner == nil {
		return status, nil
	}
	d := owner.driver
	d.mu.Lock()
	status.Released, status.Err = owner.ownerReleased, sessionCleanupErrorLocked(owner)
	if status.Admitted {
		status.Finished = owner.nativeRetired
	}
	d.mu.Unlock()
	return status, nil
}

// Caller holds Driver.mu. Physical Released alone never certifies an owned
// child's cleanup, even if the first native Close itself returned nil.
func sessionCleanupErrorLocked(s *pppSession) error {
	result := errors.Join(s.nativeReleaseErr, s.ownerServiceErr)
	if s.ownerReleased && s.ownerServicePending {
		return errors.Join(result, ErrIPv6CleanupPending)
	}
	return result
}
