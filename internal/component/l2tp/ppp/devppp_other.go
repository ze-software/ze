// Design: docs/architecture/l2tp/bng-5-pppoe.md -- non-Linux stub for /dev/ppp setup

//go:build !linux

package ppp

import (
	"errors"
)

var errPppDevPppNotAvailableOn = errors.New("ppp: /dev/ppp not available on this platform")

// DevPPPSetup is not available on non-Linux platforms.
func DevPPPSetup(_ int) (int, int, int, error) {
	return -1, -1, -1, errPppDevPppNotAvailableOn
}
