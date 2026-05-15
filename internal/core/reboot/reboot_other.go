// Design: (none -- new utility for system reboot)
//
// Package reboot provides platform-specific system reboot.
// On non-Linux platforms, Reboot returns an error.

//go:build !linux

package reboot

import (
	"errors"
)

var errSystemRebootIsOnlySupportedOn = errors.New("system reboot is only supported on Linux")

// Reboot is not supported on non-Linux platforms.
// Ze targets Linux (including gokrazy) for system reboot.
func Reboot() error {
	return errSystemRebootIsOnlySupportedOn
}
