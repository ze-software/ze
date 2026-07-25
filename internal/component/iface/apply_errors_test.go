package iface

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
)

// permErr rebuilds the error shape the production apply path actually produces
// for a privilege refusal, so these tests exercise the real chain rather than a
// convenient stand-in:
//
//	internal/plugins/iface/netlink/manage_linux.go:57-59  CreateDummy wraps
//	    netlink.LinkAdd's syscall.Errno with %w
//	internal/component/iface/config_apply.go:347-350      record() wraps that
//	    with %w under a "<what> create" prefix
//
// The kernel returns EPERM for RTM_NEWLINK without CAP_NET_ADMIN; "operation
// not permitted" in the QEMU reproduction is exactly syscall.EPERM.Error().
func permErr(errno syscall.Errno) error {
	inner := fmt.Errorf("iface: create dummy %q: %w", "zdiag0", errno)
	return fmt.Errorf("%s: %w", "dummy zdiag0 create", inner)
}

// VALIDATES: an apply failure the kernel refused for want of privilege returns
// an error carrying the corrective action (CAP_NET_ADMIN), the offending object,
// and the underlying reason.
// PREVENTS: the operator-facing regression -- an unprivileged ze aborted startup
// reporting only "operation not permitted" with no statement of which privilege
// was missing or how to grant it (ai/rules/error-messages.md leg 3).
func TestJoinApplyErrorsAddsPermissionRemediation(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.EPERM, syscall.EACCES} {
		err := joinApplyErrors("interface config", []error{permErr(errno)})
		if err == nil {
			t.Fatalf("errno %v: joinApplyErrors = nil, want an error", errno)
		}
		got := err.Error()
		for _, want := range []string{
			"interface config", // what failed
			"zdiag0",           // the offending object
			"CAP_NET_ADMIN",    // what to do next
			"setcap",           // ... and how
		} {
			if !strings.Contains(got, want) {
				t.Errorf("errno %v: error missing %q\ngot: %s", errno, want, got)
			}
		}
		// The cause must stay reachable for errors.Is, not just be pasted in.
		if !errors.Is(err, errno) {
			t.Errorf("errno %v: errors.Is lost the cause; got %v", errno, err)
		}
	}
}

// VALIDATES: a failure that is NOT a privilege refusal gets no CAP_NET_ADMIN
// advice, while still naming its cause.
// PREVENTS: a misleading remediation -- telling an operator to grant
// CAP_NET_ADMIN when the real problem is an unsupported device or a bad name
// sends them down the wrong path. A remediation must be TRUE, not merely
// present.
func TestJoinApplyErrorsOmitsRemediationForNonPermissionError(t *testing.T) {
	cause := fmt.Errorf("dummy zdiag0 create: %w",
		fmt.Errorf("iface: create dummy %q: %w", "zdiag0", syscall.EEXIST))

	err := joinApplyErrors("interface config", []error{cause})
	got := err.Error()

	if strings.Contains(got, "CAP_NET_ADMIN") {
		t.Errorf("non-permission error advertised CAP_NET_ADMIN\ngot: %s", got)
	}
	if !errors.Is(err, syscall.EEXIST) {
		t.Errorf("errors.Is lost the cause; got %v", err)
	}
}

// VALIDATES: several apply failures report the count AND a concrete cause,
// wrapped so errors.Is still works.
// PREVENTS: the previous "N errors (see log for details)" summary, which named
// no cause at all. This error crosses the plugin RPC boundary as text, so the
// engine's copy cannot consult the plugin's log -- the evidence has to travel
// inside the error (ai/rules/error-messages.md leg 2).
func TestJoinApplyErrorsMultipleNamesCountAndFirstCause(t *testing.T) {
	first := permErr(syscall.EPERM)
	second := fmt.Errorf("veth zdiag1 create: %w",
		fmt.Errorf("iface: create veth %q: %w", "zdiag1", syscall.EPERM))

	err := joinApplyErrors("interface config", []error{first, second})
	got := err.Error()

	if !strings.Contains(got, "2 errors") {
		t.Errorf("error does not report the count\ngot: %s", got)
	}
	if !strings.Contains(got, "zdiag0") {
		t.Errorf("error does not carry the first cause\ngot: %s", got)
	}
	if !strings.Contains(got, "CAP_NET_ADMIN") {
		t.Errorf("error does not carry the remediation\ngot: %s", got)
	}
	if !errors.Is(err, syscall.EPERM) {
		t.Errorf("errors.Is lost the cause; got %v", err)
	}
}

// VALIDATES: lacksPrivilege detects a permission refusal anywhere in the error
// set, not only in the first entry.
// PREVENTS: the remediation being dropped when the privilege failure is
// preceded by an unrelated error -- applyConfig appends rollback errors to the
// same slice, so the EPERM is frequently not first.
func TestLacksPrivilegeScansEveryError(t *testing.T) {
	unrelated := errors.New("rollback partial apply: link busy")
	if !lacksPrivilege([]error{unrelated, permErr(syscall.EPERM)}) {
		t.Error("lacksPrivilege = false, want true when a later error is EPERM")
	}
	if lacksPrivilege([]error{unrelated}) {
		t.Error("lacksPrivilege = true, want false with no permission error")
	}
	if lacksPrivilege(nil) {
		t.Error("lacksPrivilege(nil) = true, want false")
	}
}
