// Design: docs/architecture/l2tp/bng-5-pppoe.md -- typed PPP setup unavailable off Linux

//go:build !linux

package kernel

import (
	"errors"
	"os"
)

var errPppDevPppNotAvailableOn = errors.New("ppp: /dev/ppp not available on this platform")

// DevPPPSetup mirrors the Linux typed-owner signature and allocates nothing.
func DevPPPSetup(_ *UnitCreator, _ *os.File, _ string) (*CreatedChannel, *CreatedUnit, error) {
	return nil, nil, errPppDevPppNotAvailableOn
}

// SetIPv4Mode is unavailable on non-Linux platforms. The caller MUST retain
// descriptor ownership for the call, as required by the Linux implementation.
func SetIPv4Mode(_ int, mode NPMode) error {
	if _, err := mode.kernelValue(); err != nil {
		return err
	}
	return errPppDevPppNotAvailableOn
}
