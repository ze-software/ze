// Design: docs/research/l2tpv2-ze-integration.md -- bounded native cleanup ownership
// Related: session_release.go -- exact native unit release gate
// Related: manager.go -- strong owner retention after protocol retirement

package ppp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
)

var (
	ErrNativeReleaseBudgetMissing   = errors.New("ppp: native release budget is not configured")
	ErrNativeReleaseBudgetExhausted = errors.New("ppp: native release budget is exhausted")
	ErrNativeReleaseUnavailable     = errors.New("ppp: session owner has no native release capability")
	ErrNativeReleaseUnbound         = errors.New("ppp: native release gate is not bound to the open owner")
	ErrNativeReleasePending         = errors.New("ppp: native release remains unconfirmed")
)

// NativeReleaseBudget bounds actual or uncertain remote native obligations.
// It has no owner/token registry. Composition MUST create it from the actual
// named native-owner quota and pass the SAME pointer to EVERY PPP Driver,
// including all L2TP listeners and PPPoE. Its zero value is invalid.
type NativeReleaseBudget struct {
	mu    sync.Mutex
	limit int
	used  int
}

func NewNativeReleaseBudget(limit int) (*NativeReleaseBudget, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("ppp: native release budget must be positive, got %d", limit)
	}
	return &NativeReleaseBudget{limit: limit}, nil
}

// ConfigureNativeReleaseBudget binds the shared quota before Start. There is
// deliberately no per-Driver default or allocation. Repeating the SAME binding
// before Start is harmless; replacing it or configuring after Start is refused.
func (d *Driver) ConfigureNativeReleaseBudget(budget *NativeReleaseBudget) error {
	if budget == nil {
		return ErrNativeReleaseBudgetMissing
	}
	if budget.limit <= 0 {
		return ErrNativeReleaseBudgetMissing
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.started {
		return errors.New("ppp: native release budget must be configured before Start")
	}
	if d.stopped {
		return ErrDriverStopped
	}
	if d.nativeReleaseBudget != nil {
		if d.nativeReleaseBudget == budget {
			return nil
		}
		return errors.New("ppp: a different native release budget is already configured")
	}
	d.nativeReleaseBudget = budget
	return nil
}

func (budget *NativeReleaseBudget) reserve() bool {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if budget.used == budget.limit {
		return false
	}
	budget.used++
	return true
}

func (budget *NativeReleaseBudget) release() {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if budget.used == 0 {
		panic("BUG: PPP native release budget released without a reservation")
	}
	budget.used--
}

// ReserveNativeRelease reserves this exact active worker once, BEFORE its
// first actual remote native RPC. Absent-shaper/NoResource paths MUST NOT call
// it. The caller MUST hold the admitted policy's operation gate through RPC
// classification, so its seal cannot overtake the attempted native operation.
// No Driver/budget lock is held while the native owner is inspected.
func (control SessionControl) ReserveNativeRelease() error {
	if control.driver == nil {
		return ErrSessionNotFound
	}
	d := control.driver
	d.mu.Lock()
	session, ok := d.nativeActiveControlLocked(control)
	if !ok {
		d.mu.Unlock()
		return ErrSessionNotFound
	}
	budget := d.nativeReleaseBudget
	owner := session.dataplane
	birthTag := session.nativeBirthTag
	d.mu.Unlock()
	if budget == nil {
		return ErrNativeReleaseBudgetMissing
	}
	gate, capable := owner.(NativeReleaseHandle)
	if !capable {
		return ErrNativeReleaseUnavailable
	}
	if birthTag == "" || !gate.NativeReleaseBound() {
		return ErrNativeReleaseUnbound
	}
	d.mu.Lock()
	current, live := d.nativeActiveControlLocked(control)
	if !live {
		d.mu.Unlock()
		return ErrSessionNotFound
	}
	if current.nativeReserved {
		d.mu.Unlock()
		return nil
	}
	// The quota is pure count state: no callback or native I/O. Keep this
	// decision atomic with the worker flag so concurrent first reservations
	// of the SAME worker remain idempotent even at a full shared quota.
	if !budget.reserve() {
		d.mu.Unlock()
		return ErrNativeReleaseBudgetExhausted
	}
	current.nativeReserved = true
	d.mu.Unlock()
	return nil
}

// nativeActiveControlLocked also fences a stop signal before run has returned.
// Caller MUST hold d.mu. It performs no native I/O or policy call.
func (d *Driver) nativeActiveControlLocked(control SessionControl) (*pppSession, bool) {
	session, ok := d.sessionForControlLocked(control)
	if !ok {
		return nil, false
	}
	if session.nativeRetired {
		return nil, false
	}
	select {
	case <-session.sessStop:
		return nil, false
	default:
		return session, true
	}
}

// RetryNativeRelease reconciles only this exact OLD retired owner. An End
// callback arriving before retirement performs no close (nil does NOT prove
// release); callers MUST inspect CleanupStatus. Run's exit always
// retries once after marking retirement. Concurrent calls use the owner's
// existing serialized/idempotent Close, with NO Driver/budget locks held while
// Close or Seal runs. The callback MUST run outside the policy operation gate.
func (control SessionControl) RetryNativeRelease() error {
	return control.retryNativeRelease(context.Background())
}

func (control SessionControl) retryNativeRelease(ctx context.Context) error {
	if !control.capturedWorker() {
		return ErrSessionNotFound
	}
	return control.driver.retrySessionOwner(ctx, control.worker)
}

// Captured owner pointers include refused handoffs with NO admitted Control.
// An active protocol owner is never closed by a retirement retry.
func (d *Driver) retrySessionOwner(ctx context.Context, session *pppSession) error {
	d.mu.Lock()
	if session.nativeReleased {
		err := session.nativeReleaseErr
		d.mu.Unlock()
		return err
	}
	if d.owners[session.dataplane] != session {
		d.mu.Unlock()
		return ErrSessionNotFound
	}
	retired := session.nativeRetired
	d.mu.Unlock()
	if !retired {
		return nil
	}
	session.nativeReleaseMu.Lock()
	defer session.nativeReleaseMu.Unlock()
	if err := ctx.Err(); err != nil {
		return &nativeReleaseRetryCanceled{cause: err}
	}
	d.mu.Lock()
	if session.nativeReleased {
		err := session.nativeReleaseErr
		d.mu.Unlock()
		return err
	}
	if d.owners[session.dataplane] != session {
		d.mu.Unlock()
		return ErrSessionNotFound
	}
	if !session.nativeRetired {
		d.mu.Unlock()
		return nil
	}
	attempted, confirmed := session.ownerCloseAttempted, session.ownerReleased
	err := session.nativeReleaseErr
	d.mu.Unlock()
	// closeFiles and a canceled-owner caller share this gate. A completed
	// first success must not produce a second physical Close on retirement.
	if !attempted || !confirmed {
		err = d.closeSessionOwnerLocked(session)
	}
	d.mu.Lock()
	physicalReleased := session.ownerReleased
	d.mu.Unlock()
	childErr := session.retryRetiredIPv6Cleanup(ctx, physicalReleased)
	d.releaseRetiredOwnerLocked(session)
	return errors.Join(err, childErr)
}

// Caller holds the owner's nativeReleaseMu. Physical I/O uses NO Driver lock.
func (d *Driver) closeSessionOwnerLocked(session *pppSession) error {
	session.closeAuthAdmission()
	// Driver.Stop, StopSession and canceled-owner cleanup all reach this
	// boundary. The frozen measurement precedes the actual original Close.
	session.captureNetworkTerminal()
	err := session.dataplane.Close()
	confirmed := err == nil
	gate, native := session.dataplane.(NativeReleaseHandle)
	if native {
		confirmed = !gate.NativeReleasePending()
		if !confirmed {
			err = errors.Join(err, ErrNativeReleasePending)
		}
	}
	// Only exact physical release retires the copied IPv4 receipt. A failed
	// descriptor close can coexist with confirmed native-unit destruction;
	// its error remains independent below. Pending release MUST keep this
	// receipt, without trying native mutation through the closed handle.
	if confirmed {
		session.installedIPv4 = netip.Prefix{}
	}
	d.mu.Lock()
	session.ownerCloseAttempted = true
	session.ownerReleased = confirmed
	session.nativeReleaseErr = err
	d.mu.Unlock()
	if err != nil {
		d.publishNativeReleaseFailure(session, err)
	} else if native {
		session.clearNativeSessionCleanupFailure()
	}
	return err
}

// Release the index/quota ONLY after protocol retirement and physical release.
// A native owner can be physically released with an independent local Close
// error; that error remains observable and never becomes a nil close result.
func (d *Driver) releaseRetiredOwnerLocked(session *pppSession) {
	d.mu.Lock()
	if session.nativeReleased || !session.nativeRetired || !session.ownerReleased || session.ownerServicePending {
		d.mu.Unlock()
		return
	}
	if d.owners[session.dataplane] != session {
		d.mu.Unlock()
		return
	}
	session.nativeReleased = true
	delete(d.owners, session.dataplane)
	reserved := session.nativeReserved
	session.nativeReserved = false
	budget := d.nativeReleaseBudget
	d.mu.Unlock()
	if reserved {
		if budget == nil {
			panic("BUG: PPP native reservation has no shared budget")
		}
		budget.release()
	}
}

// This private result marks cancellation BEFORE any physical retry. An actual
// Close returning context cancellation still carries its real native failure
// and MUST be reported rather than mistaken for an operator-paused sweep.
type nativeReleaseRetryCanceled struct{ cause error }

func (err *nativeReleaseRetryCanceled) Error() string { return err.cause.Error() }
func (err *nativeReleaseRetryCanceled) Unwrap() error { return err.cause }

// RetryPendingNativeRelease services the existing operator reload seam. It
// snapshots exact retired owner pointers and checks the caller deadline BEFORE EACH
// bounded native retry. It neither starts a worker nor looks up current IDs.
func (d *Driver) RetryPendingNativeRelease(ctx context.Context) error {
	d.mu.Lock()
	owners := make([]*pppSession, 0, len(d.owners))
	for _, session := range d.owners {
		if session.nativeRetired && !session.nativeReleased {
			owners = append(owners, session)
		}
	}
	d.mu.Unlock()
	var result error
	for _, owner := range owners {
		if err := ctx.Err(); err != nil {
			return errors.Join(result, err)
		}
		if err := d.retrySessionOwner(ctx, owner); err != nil {
			if _, canceled := errors.AsType[*nativeReleaseRetryCanceled](err); !canceled {
				d.reportNativeReleaseFailure(owner, err)
			}
			result = errors.Join(result, err)
		}
	}
	return result
}

func (d *Driver) reportNativeReleaseFailure(session *pppSession, err error) {
	if err == nil {
		return
	}
	// A caller can be delayed after its failed Retry returns. Serialize this
	// report with later success/clear, so an old result cannot resurrect a
	// warning after confirmed release. Sticky first-Close errors were already
	// recorded by closeFiles while it owned this same gate.
	session.nativeReleaseMu.Lock()
	defer session.nativeReleaseMu.Unlock()
	d.mu.Lock()
	released := session.nativeReleased
	d.mu.Unlock()
	if released || session.nativeReleaseErr == nil {
		return
	}
	// Use the current serialized failure, not a delayed caller's old result.
	d.publishNativeReleaseFailure(session, session.nativeReleaseErr)
}

// Caller holds session.nativeReleaseMu; no Driver/budget lock is held.
func (d *Driver) publishNativeReleaseFailure(session *pppSession, err error) {
	session.reportSessionCleanupFailure(session.dataplane, err)
	d.logger.Warn("ppp: native session close reported an error",
		"code", CodeDoctorPPPCleanup, "subscriber-id", session.subscriberID,
		"error", err, "fix", CleanupFailureFix)
}

// HasPendingNativeRelease reports retained physical or IPv6 child ownership
// using Driver metadata only. Composition MUST keep the Driver reachable across stop and
// restart while true; native handles are never inspected under d.mu.
func (d *Driver) HasPendingNativeRelease() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, session := range d.owners {
		if session.nativeReserved || (session.nativeRetired && !session.nativeReleased) {
			return true
		}
	}
	return false
}

// Counters reads the exact admitted handle without holding a Driver lock and
// rejects a snapshot if that physical worker was replaced or stopped during
// the read. These are raw TRANSPORT counters; the caller MUST independently
// validate the actual network generation before publishing a logical baseline.
func (control SessionControl) Counters() (SessionCounters, error) {
	if control.driver == nil {
		return SessionCounters{}, ErrSessionNotFound
	}
	session, err := control.driver.liveSession(control)
	if err != nil {
		return SessionCounters{}, err
	}
	counters, err := session.dataplane.Counters()
	if err != nil {
		return SessionCounters{}, err
	}
	if !control.Active() {
		return SessionCounters{}, ErrSessionNotFound
	}
	return counters, nil
}
