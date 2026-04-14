// Design: (none -- new utility for system reboot)
//
// Package reboot provides platform-specific system reboot.
// On Linux (including gokrazy), it calls the reboot(2) syscall.
// The caller MUST complete all graceful shutdown before calling Reboot.

//go:build linux

package reboot

import (
	"fmt"
	"os"
	"syscall"
)

// Reboot performs a system reboot via the Linux reboot(2) syscall.
// Requires root privileges (UID 0). Returns an error if not root
// or if the syscall fails. The caller MUST complete all graceful
// shutdown (drain connections, stop subsystems) before calling this.
func Reboot() error {
	if os.Getuid() != 0 {
		return fmt.Errorf("not running as root (uid %d), cannot reboot system", os.Getuid())
	}
	syscall.Sync()
	return syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
}
