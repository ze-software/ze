// Design: docs/architecture/cli/plugin-modes.md -- provision interface auto-config stub

//go:build !linux

package provision

import "errors"

func ensureAddress(_, _ string) error {
	return errors.New("automatic interface configuration requires linux")
}

func removeAddress(_, _ string) error {
	return errors.New("automatic interface configuration requires linux")
}
