// Design: docs/architecture/l2tp/bng-5-pppoe.md -- non-Linux stub for /dev/ppp setup

//go:build !linux

package ppp

import (
	"errors"
)

var errPppDevPppNotAvailableOn = errors.New("ppp: /dev/ppp not available on this platform")

// DevPPPSetup is not available on non-Linux platforms. The name parameter
// mirrors the Linux signature so the two platforms stay in sync; it is
// otherwise unused here (child-1,
// plan/release/fixes/spec-home-cpe-1-logical-interface-name.md).
func DevPPPSetup(_ int, _ string) (int, int, int, error) {
	return -1, -1, -1, errPppDevPppNotAvailableOn
}

// SetIPv4Mode is unavailable on non-Linux platforms. The caller MUST retain
// descriptor ownership for the call, as required by the Linux implementation.
func SetIPv4Mode(_ int, mode NPMode) error {
	if _, err := mode.kernelValue(); err != nil {
		return err
	}
	return errPppDevPppNotAvailableOn
}
