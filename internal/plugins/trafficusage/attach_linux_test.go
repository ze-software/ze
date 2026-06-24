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
	// ebpfSupported is the side-effect-free check the doctor uses; on Linux the
	// feature is built in.
	if err := ebpfSupported(); err != nil {
		t.Errorf("ebpfSupported() = %v on Linux, want nil", err)
	}
}
