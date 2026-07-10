// VALIDATES: Reboot() refuses to act when the process is not root (uid != 0),
// returning an error instead of attempting the reboot(2) syscall.
// PREVENTS: a non-privileged caller reaching the syscall, and a regression that
// drops the root guard (which would fail with EPERM or, worse, succeed).

//go:build linux

package reboot

import (
	"os"
	"testing"
)

func TestRebootRequiresRoot(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: cannot exercise the non-root guard without rebooting")
	}
	if err := Reboot(); err == nil {
		t.Fatal("Reboot() returned nil when not root; expected a privilege error")
	}
}
