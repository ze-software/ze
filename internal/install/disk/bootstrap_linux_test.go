// Design: docs/architecture/appliance/installer-initrd.md -- bootstrap tests

//go:build linux && ze_installer

package disk

import (
	"testing"
)

func TestBootstrapMountsProc(t *testing.T) {
	// bootstrap() mounts /proc, /sys, /dev and sets up console fan-out.
	// Validated by `./le qemu install-test`: the Go initrd boots,
	// parseCmdline reads /proc/cmdline (proves /proc
	// is mounted), findTargetDisk reads /sys/block (proves /sys), and
	// device nodes under /dev are accessible (proves devtmpfs).
	//
	// Direct unit testing requires root + unshare(CLONE_NEWNS) which is
	// not available in the CI test runner; the QEMU evidence test is the
	// authoritative gate.
	t.Skip("bootstrap requires PID 1 context; validated by QEMU evidence test")
}
