// Design: docs/functional-tests.md -- Linux authority to stop a privileged child.

package fixture

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/runner"
)

func sysribExecutableChildControl() error {
	return requireSysribExecutableChildControl(os.Geteuid(), func() (bool, error) {
		return sysribExecutableKillCapability(unix.Capget)
	})
}

func sysribExecutableKillCapability(capget func(*unix.CapUserHeader, *unix.CapUserData) error) (bool, error) {
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := capget(&header, &data[0]); err != nil {
		return false, fmt.Errorf("read effective fixture capabilities: %w", err)
	}
	bit := runner.FixtureProcessControlCapability().Bit
	return data[bit/32].Effective&(uint32(1)<<(bit%32)) != 0, nil
}
