//go:build !linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the application override guard
// Overview: override_guard.go -- the backend seam and the state it reports

package dataplane

import (
	"fmt"
	"runtime"
)

// AttachOverrideGuard refuses: no XFRM exists off Linux, so no guard can be attached
// and `application-override deny` cannot be enforced.
func (b *xfrmBackend) AttachOverrideGuard() error {
	return fmt.Errorf("%w: xfrm not available on %s", ErrNotSupported, runtime.GOOS)
}

// DetachOverrideGuard has nothing to remove: no guard was ever attached here.
func (b *xfrmBackend) DetachOverrideGuard() error { return nil }

// OverrideGuardState refuses, for the reason AttachOverrideGuard gives.
func (b *xfrmBackend) OverrideGuardState() (OverrideGuardState, error) {
	return OverrideGuardState{}, fmt.Errorf("%w: xfrm not available on %s", ErrNotSupported, runtime.GOOS)
}
