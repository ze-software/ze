//go:build !linux

// Design: docs/functional-tests.md -- only root can stop a root child without Linux capabilities.

package fixture

import "os"

func sysribExecutableChildControl() error {
	return requireSysribExecutableChildControl(os.Geteuid(), func() (bool, error) {
		return false, nil
	})
}
