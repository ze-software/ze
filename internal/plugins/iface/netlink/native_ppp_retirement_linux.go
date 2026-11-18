// Design: docs/features/interfaces.md -- same-original-unit terminal transport join
// Related: native_ppp_transport_linux.go -- retained unknown-outcome history

//go:build linux

package ifacenetlink

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/iface"
)

// observeUnitRetirementLocked never starts Close or waits for a live unit. The
// actual SAME owner checks completed retirement BEFORE selecting ctx.Done, so
// an already-cancelled passive observation can return nil only after that join.
// All negative observations retain custody; their absence is never a receipt.
// The writer mutex is held, but no unit descriptor borrow is held here.
func (w *nativePPPWriter) observeUnitRetirementLocked() bool {
	owner, ok := w.owner.(iface.NativePPPUnitRetirementOwner)
	if !ok {
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := owner.WaitNativePPPUnitRetired(ctx)
	if err == nil {
		return true
	}
	// The cancelled observation is a non-result, not an operation failure.
	// Preserve an actual original/duplicate Close error once, without growing
	// history on repeated observation of the same immutable terminal result.
	if err != context.Canceled && w.pending.retirementErr == nil {
		w.pending.retirementErr = err
	}
	return false
}

// settleRetiredUnitLocked settles the transport after same-unit retirement.
// Socket custody ends only after its physical Close succeeds. It never upgrades an
// unknown operation to success, a matched NACK, no effects, rollback or retry
// authority. Primary retains all domain effect/cleanup records, joins its final
// accounting/disconnect, original Close and native-release seal, then calls
// final Settle/Close. No descriptor is reacquired and no request is resent.
func (w *nativePPPWriter) settleRetiredUnitLocked() error {
	transaction := w.pending
	cause := errors.Join(errors.New("PPP original unit retired with an unknown native write outcome; retain domain effect cleanup"),
		transaction.cause, transaction.nativeErr, transaction.poison,
		errors.Join(transaction.waitErrs...), transaction.retirementErr)
	if transaction.turns >= nativePPPSettlementLimit {
		cause = errors.Join(cause, errors.New("PPP original settlement turn limit reached"))
	}
	w.pending = nil
	w.closed = true
	w.socketCloseErr = w.file.Close()
	w.closeErr = errors.Join(w.closeErr, cause, w.socketCloseErr)
	return w.closeErr
}

// ClosePending reports retained native custody, independently of error history.
// A live socket still needs Close even before its first mutation. A failed
// physical Close retains custody despite os.File's logical retirement; its
// stored error is never replaced by another Close attempt's os.ErrClosed.
func (w *nativePPPWriter) ClosePending() bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending != nil {
		return true
	}
	if w.socketCloseErr != nil {
		return true
	}
	if w.closed {
		return false
	}
	return w.file != nil
}
