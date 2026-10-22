// Design: docs/architecture/appliance/docker-host.md -- the restart refusal after the boot sequence ran
// Related: guest_linux.go -- RunGuest, which asks it before booting

package dockerhost

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// errRestartRefused is the refusal of a program restarted after the boot
// sequence ran. It names both ways to reboot the guest: the guest is either
// the `ze` Colima profile or the QEMU docker-lab VM, and each restarts its
// own way.
var errRestartRefused = errors.New("this program exited after the boot sequence ran, " +
	"so the services it started may still run and a new supervisor would start a second copy of each; not restarting. " +
	"Fix: reboot the guest: for the Colima profile run `colima restart ze`, " +
	"for the QEMU guest stop it and rerun `./le test qemu docker-lab`")

// refuseRestart answers whether a run of the guest program may boot, from
// the marker the boot sequence writes once it has run (bootedMarker, on the
// /run tmpfs, so a reboot clears it). An absent marker answers nil: this is
// the first run of this boot. A present marker answers errRestartRefused.
//
// The services the previous run started (sshd, containerd, dockerd) outlive
// it, and a fresh supervisor knows none of them, so it would start a second
// sshd on the bound port and a second dockerd on the same data root. Killing
// recorded pids would need a pid file trusted across a crash; a reboot is the
// one recovery that restores a known state. A marker that cannot be read is
// refused too, because the program cannot tell that boot from a first one.
func refuseRestart(marker string) error {
	_, err := os.Stat(marker)
	if err == nil {
		return errRestartRefused
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("cannot tell whether the boot sequence already ran, so not booting: %w. "+
		"Fix: reboot the guest (`colima restart ze`, or rerun `./le test qemu docker-lab`)", err)
}
