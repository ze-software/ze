//go:build !linux && !darwin

// Design: docs/architecture/testing/runner-architecture.md -- owned signal causality.
package runner

import "errors"

// exitedBeforeStop MUST run under signalMu. Unsupported observation grants
// no intentional-exit exception; the owner still performs checked cleanup.
func (*ownedProcess) exitedBeforeStop() (bool, error) {
	return false, errors.New("cannot establish parent exit before an owned signal on this platform")
}
