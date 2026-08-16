// Design: docs/architecture/appliance/installer-initrd.md -- RunInitrd wiring test

//go:build linux && ze_installer

package disk

import "testing"

func TestRunInitrdExists(t *testing.T) {
	// RunInitrd is the PID-1 entry point. It cannot be unit-tested (requires
	// PID 1 context + kernel reboot). Validated by QEMU wiring test:
	// make ze-qemu-install-test boots the Go initrd and completes an HTTP
	// install end-to-end.
	//
	// This test verifies the function exists and is callable (compile check).
	var fn func()
	fn = RunInitrd
	_ = fn
}
