//go:build linux

// VALIDATES: the Linux build provides a concrete attacher constructor.
// PREVENTS: a nil attacher on Linux (which would panic the monitor). Real
// load/attach behavior is covered by the QEMU integration tests
// (attach_integration_linux_test.go, program_test.go).

package trafficusage

import "testing"

func TestNewAttacherLinux(t *testing.T) {
	if a := newAttacher(); a == nil {
		t.Fatal("newAttacher() = nil on Linux")
	}
	// The doctor's probe asks the kernel, so on Linux it answers ready, absent or
	// unknown (an unprivileged run), and never returns no verdict.
	if got := probeTCX(); got.state == tcxReadinessUnspecified || got.reason == nil {
		t.Errorf("probeTCX() = %v (%v) on Linux, want a verdict with its evidence", got.state, got.reason)
	}
}
