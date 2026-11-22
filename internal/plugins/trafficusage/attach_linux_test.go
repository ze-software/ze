//go:build linux

// VALIDATES: the Linux build provides a concrete attacher constructor, and
// markAttachError marks each kind of kernel answer so classifyAttachError
// reaches a distinct outcome: no TCX support is absent, a verifier refusal is
// unknown even with its EACCES, EPERM and EACCES are a permission refusal, and
// any other errno is unknown.
// PREVENTS: a nil attacher on Linux (which would panic the monitor); a verifier
// refusal reported as missing capabilities, sending the operator to the
// service unit for a program defect. Real load/attach behavior is covered by
// the QEMU integration tests (attach_integration_linux_test.go,
// program_test.go, readiness_integration_linux_test.go).

package trafficusage

import (
	"errors"
	"fmt"
	"syscall"
	"testing"

	"github.com/cilium/ebpf"
)

func TestNewAttacherLinux(t *testing.T) {
	if a := newAttacher(); a == nil {
		t.Fatal("newAttacher() = nil on Linux")
	}
}

func TestMarkAttachErrorClassifies(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want attachState
	}{
		{"kernel without TCX", fmt.Errorf("attach tcx: %w", ebpf.ErrNotSupported), attachStateAbsent},
		{"verifier refused the program", &ebpf.VerifierError{Cause: syscall.EACCES}, attachStateUnknown},
		{"process may not load", fmt.Errorf("load program: %w", syscall.EPERM), attachStateRefused},
		{"process may not attach", syscall.EACCES, attachStateRefused},
		{"any other errno", syscall.EINVAL, attachStateUnknown},
	}
	for _, tc := range cases {
		marked := markAttachError("attach TCX ingress", tc.err)
		if got := classifyAttachError(marked); got != tc.want {
			t.Errorf("%s: classified %v, want %v (error %v)", tc.name, got, tc.want, marked)
		}
		if !errors.Is(marked, tc.err) {
			t.Errorf("%s: the marked error lost the kernel's answer: %v", tc.name, marked)
		}
	}
}
