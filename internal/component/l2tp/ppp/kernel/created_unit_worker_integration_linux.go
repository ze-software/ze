//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- actual worker restore fault
// Related: devppp_namespace_linux.go -- joined locked worker before real capture

package kernel

import "errors"

// SetNativePPPWorkerStartForTest attaches an instance-local startup observer to
// this SAME genuinely created unit before binding. The callback runs on the
// locked joined worker before its real namespace capture and supplies no owner,
// receipt, namespace identity, I/O result or release grant. The worker MUST retire
// its locked thread on every outcome after this observer, even successful ones.
// This setter is not compiled into shipped binaries. The caller MUST not retain
// the worker and MUST install before BindNativeOperations or SessionConfig.
func SetNativePPPWorkerStartForTest(unit *CreatedUnit, start func() error) error {
	if unit == nil {
		return errors.New("ppp: test worker has no original unit")
	}
	if start == nil {
		return errors.New("ppp: test worker startup observer is absent")
	}
	unit.mu.Lock()
	defer unit.mu.Unlock()
	if unit.self != unit {
		return errors.New("ppp: test worker refuses copied original unit")
	}
	if unit.closed {
		return errors.New("ppp: test worker refuses closed original unit")
	}
	if !unit.ready {
		return errors.New("ppp: test worker requires genuine confirmed unit creation")
	}
	if unit.nativeOperations != nil {
		return errors.New("ppp: test worker must precede native binding")
	}
	if unit.sessionClaim != nil {
		return errors.New("ppp: test worker must precede session claim")
	}
	if unit.nativeNamespaceStart != nil {
		return errors.New("ppp: test worker startup already observed")
	}
	unit.nativeNamespaceStart = start
	return nil
}
