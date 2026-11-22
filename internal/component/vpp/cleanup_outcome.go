// Design: docs/research/vpp-deployment-reference.md -- native cleanup outcomes
// Related: vpp.go -- original managed child cleanup and lifecycle decisions
package vpp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
)

// nativeCleanupError identifies an unresolved failure of an original cleanup
// operation. Producers retain its cause; an ordinary process crash is distinct.
type nativeCleanupError struct {
	cause   error
	receipt *nativeCleanupReceipt
}

func (e nativeCleanupError) Error() string { return e.cause.Error() }
func (e nativeCleanupError) Unwrap() error { return e.cause }

// nativeCleanupReceipt retains the original manager and its captured driver
// restoration operation after Run has joined. The lifecycle MUST retain this
// receipt until retry confirms cleanup. An unresolved child result cannot be
// cleared by successful NIC restoration. Safe only after Run has joined and
// while the lifecycle serializes its retries.
type nativeCleanupReceipt struct {
	manager      *VPPManager
	restore      func() error
	childFailure error
	cause        error
}

// retry makes one attempt against the SAME captured restoration boundary.
// The caller MUST keep the receipt when this returns a cleanup failure.
func (receipt *nativeCleanupReceipt) retry() error {
	err := receipt.restore()
	if err == nil && receipt.childFailure == nil {
		return nil
	}
	return nativeCleanupError{cause: errors.Join(receipt.cause, err), receipt: receipt}
}

// retainNativeCleanup captures this manager's terminal cleanup boundary. Its
// caller MUST retain the returned receipt until retry confirms completion.
func (m *VPPManager) retainNativeCleanup(cause, childFailure error, restore func() error) error {
	return nativeCleanupError{cause: cause, receipt: &nativeCleanupReceipt{
		manager: m, restore: restore, childFailure: childFailure, cause: cause,
	}}
}

// managedChildStop records only successful kills by this command's original
// CommandContext cancellation callback. Install it before Start; Wait joins
// that callback before the receipt is consulted. It owns no worker or handle.
type managedChildStop struct{ killed atomic.Bool }

func trackManagedChildStop(cmd *exec.Cmd) *managedChildStop {
	stop := &managedChildStop{}
	cancel := cmd.Cancel
	cmd.Cancel = func() error {
		err := cancel()
		if err == nil {
			stop.killed.Store(true)
		}
		return err
	}
	return stop
}

func (s *managedChildStop) confirmed() bool { return s != nil && s.killed.Load() }

// expectedVPPKill classifies only the actual result of this original command's
// Wait. A wrapped/joined error or another command's ProcessState is not proof.
func expectedVPPKill(cmd *exec.Cmd, err error) bool {
	exit, ok := err.(*exec.ExitError) //nolint:errorlint // Only the direct original Wait exit error proves this child's kill; wrapped or joined errors keep their other causes.
	if !ok || cmd.ProcessState == nil || exit.ProcessState != cmd.ProcessState {
		return false
	}
	status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGKILL
}

// managedChildCleanup receives only the original Kill and completed Wait
// outcomes. Expected owned kills are clean; ordinary reaped crash status keeps
// its cause without being confused with an unresolved native cleanup failure.
func managedChildCleanup(ctx context.Context, cmd *exec.Cmd, killErr, waitErr error, automaticKill bool) error {
	ownedKill := killErr == nil
	if errors.Is(killErr, os.ErrProcessDone) {
		killErr = nil
	}
	if (ownedKill || automaticKill) && expectedVPPKill(cmd, waitErr) {
		waitErr = nil
	}
	if waitErr != nil && waitErr == ctx.Err() && cmd.ProcessState != nil && cmd.ProcessState.Success() { //nolint:errorlint // Only exact ctx.Err identity proves clean cancellation; a joined cleanup error must survive.
		waitErr = nil // exact cancellation plus an actual successful reap is clean
	}
	result := managedVPPWaitResult(ctx, cmd, waitErr)
	if killErr == nil {
		return result
	}
	return errors.Join(nativeCleanupError{cause: killErr}, result)
}

// managedVPPWaitResult classifies one already-completed original Wait. It
// performs no process operation and never substitutes another command's state.
func managedVPPWaitResult(ctx context.Context, cmd *exec.Cmd, waitErr error) error {
	if waitErr == nil {
		return nil
	}
	exit, reapedExit := waitErr.(*exec.ExitError) //nolint:errorlint // Only the direct original Wait exit error is a normal reaped crash; a joined error keeps its cleanup classification.
	if reapedExit && exit.ProcessState == cmd.ProcessState && cmd.ProcessState != nil {
		return waitErr
	}
	if waitErr == ctx.Err() && cmd.ProcessState != nil && cmd.ProcessState.Success() { //nolint:errorlint // Only exact ctx.Err identity proves clean cancellation; a joined cleanup error must survive.
		return waitErr
	}
	return nativeCleanupError{cause: waitErr}
}

func stopVPPChildWithStop(ctx context.Context, cmd *exec.Cmd, stop *managedChildStop) error {
	killErr := cmd.Process.Kill()
	waitErr := cmd.Wait()
	return managedChildCleanup(ctx, cmd, killErr, waitErr, stop.confirmed())
}
