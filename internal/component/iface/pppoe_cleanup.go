// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- retained CPE cleanup
// Related: pppoe_client.go -- original worker and registry own at most one session

package iface

import (
	"context"
	"errors"
	"fmt"
)

// finishCleanup uses the existing client's one cleanup slot. No status/registry
// lock spans the callback. The production callback creates a fresh bounded
// context per attempt, including Stop retries after the run worker has exited.
func (c *PPPoEClient) finishCleanup() error {
	if !c.cleanupBusy.CompareAndSwap(false, true) {
		return errors.Join(ErrNativePPPPending, errors.New("pppoe-client: original cleanup is already running"))
	}
	defer c.cleanupBusy.Store(false)
	c.mu.Lock()
	cleanup := c.pendingCleanup
	c.mu.Unlock()
	if cleanup == nil {
		return c.nativeAdmissionResult()
	}
	c.beginNativeCleanup()
	err := invokePPPoECleanup(cleanup)
	c.publishNativeCleanup(err)
	c.reportCleanupFailure(context.Background(), err)
	// Without the pending marker the original was released, so the cleanup did
	// not fail: its errors are causes to inspect, from earlier native writes or
	// from closing the unit, and no custody remains to release.
	if errors.Is(err, ErrNativePPPPending) {
		c.logger.Warn("pppoe-client: original session cleanup failed", "error", err,
			"code", pppoeCleanupDoctorCode, "fix", pppoeCleanupRecovery(err))
	} else if err != nil {
		c.logger.Warn("pppoe-client: original session released; its cleanup reported errors", "error", err,
			"code", pppoeCleanupDoctorCode, "fix", pppoeCleanupRecovery(err))
	}
	return err
}

func invokePPPoECleanup(cleanup func() error) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			cause, ok := failure.(error)
			if !ok {
				cause = fmt.Errorf("pppoe-client: original cleanup panicked: %v", failure)
			}
			err = errors.Join(cause, ErrNativePPPPending)
		}
	}()
	return cleanup()
}

// retainMissingCleanup refuses malformed negotiated sessions permanently while
// keeping the exact returned writer reachable. It cannot manufacture authority
// to close a unit the dialer failed to return. Production supplies Cleanup on
// every nonempty partial result; this lane exposes a violated dialer contract.
func (c *PPPoEClient) retainMissingCleanup(session PPPoESession) error { //nolint:gocritic // hugeParam: one owned partial session, off the data plane.
	cleanup := func() error {
		if session.NativeWriter == nil {
			return errors.Join(ErrNativePPPPending, ErrNativePPPUnavailable,
				errors.New("pppoe-client: original session omitted its cleanup owner and writer"))
		}
		return errors.Join(ErrNativePPPPending,
			errors.New("pppoe-client: original session omitted its cleanup owner"))
	}
	c.mu.Lock()
	c.pendingCleanup = cleanup
	c.mu.Unlock()
	return c.finishCleanup()
}
