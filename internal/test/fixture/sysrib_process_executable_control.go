// Design: docs/functional-tests.md -- privileged executable inspection child ownership.

package fixture

import (
	"errors"
	"fmt"
)

// requireSysribExecutableChildControl admits only an owner that can stop a
// privileged child. An unreadable capability set MUST remain an error.
func requireSysribExecutableChildControl(euid int, probe func() (bool, error)) error {
	if euid == 0 {
		return nil
	}
	canKill, err := probe()
	if err != nil {
		return fmt.Errorf("cannot verify authority to stop a privileged executable inspector: %w; fix: run the carrier through ./le test qemu ci test <ci-path>", err)
	}
	if !canKill {
		return errors.New("privileged executable inspection requires effective CAP_KILL so its owner can stop the root child; fix: run the carrier through ./le test qemu ci test <ci-path> with its fixture capability grant")
	}
	return nil
}
