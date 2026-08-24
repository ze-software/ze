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
	// This test verifies the function exists and takes no argument. The type on
	// the left is what does the checking. Infer it from RunInitrd instead, and a
	// signature change would compile here and break only at the call site.
	var _ func() = RunInitrd //nolint:staticcheck // QF1011: the written type IS the assertion; inferring it would check nothing
}
