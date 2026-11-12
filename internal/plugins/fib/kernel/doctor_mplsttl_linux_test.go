//go:build linux

// Design: docs/architecture/mpls/mpls-kernel.md -- the pushed label carries the IP TTL
// Detail: doctor_mplsttl_linux.go -- checkMPLSTTLPropagate and its registration

package fibkernel

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withMPLSTTLPropagate stands in the kernel's answer for
// net.mpls.ip_ttl_propagate for the life of one test, and counts the reads so
// a test can prove the check never looked.
func withMPLSTTLPropagate(t *testing.T, value string, err error) *int {
	t.Helper()
	reads := 0
	previous := mplsTTLPropagateRead
	mplsTTLPropagateRead = func(key string) (string, error) {
		reads++
		if key != mplsTTLPropagateKey {
			t.Errorf("read key %q, want %q", key, mplsTTLPropagateKey)
		}
		return value, err
	}
	t.Cleanup(func() { mplsTTLPropagateRead = previous })
	return &reads
}

// labelWorkOnKernelFIB is a config that makes fib-kernel push labels: LDP on
// the kernel FIB.
func labelWorkOnKernelFIB() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("fib").GetOrCreateContainer("kernel")
	tree.GetOrCreateContainer("ldp")
	return tree
}

// TestCheckMPLSTTLPropagateReportsZero drives the check over a kernel whose
// net.mpls.ip_ttl_propagate another program set to 0.
//
// VALIDATES: an error under doctor-mpls-ttl-propagate-off whose message names
// the key, the TTL the labels carry instead and RFC 3032 Section 2.4.3, with
// the sysctl command that restores 1 as its fix.
// PREVENTS: fib-kernel pushing labels that carry net.mpls.default_ttl while
// `ze doctor` reports the box ready, because Ze refuses 0 only on its own
// writes and never reads the key back.
func TestCheckMPLSTTLPropagateReportsZero(t *testing.T) {
	withMPLSTTLPropagate(t, "0", nil)

	diags := checkMPLSTTLPropagate(diagnostic.DoctorCheckContext{Tree: labelWorkOnKernelFIB()})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	d := diags[0]
	if d.Code != diagnostic.CodeDoctorMPLSTTLPropagateOff || d.Severity != diagnostic.SeverityError {
		t.Fatalf("diagnostic = %s/%s, want %s at error", d.Code, d.Severity, diagnostic.CodeDoctorMPLSTTLPropagateOff)
	}
	for _, want := range []string{mplsTTLPropagateKey, "net.mpls.default_ttl", "RFC 3032 Section 2.4.3"} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("message %q does not name %q", d.Message, want)
		}
	}
	if d.Fix != "sysctl -w net.mpls.ip_ttl_propagate=1" {
		t.Errorf("fix = %q, want the sysctl command that restores 1", d.Fix)
	}
}

// TestCheckMPLSTTLPropagateIsSilentOnOne is the positive half.
//
// VALIDATES: no diagnostic when the kernel copies the IP TTL into the label.
// PREVENTS: an error on every correctly configured label router.
func TestCheckMPLSTTLPropagateIsSilentOnOne(t *testing.T) {
	reads := withMPLSTTLPropagate(t, "1", nil)

	if diags := checkMPLSTTLPropagate(diagnostic.DoctorCheckContext{Tree: labelWorkOnKernelFIB()}); len(diags) != 0 {
		t.Fatalf("diagnostics = %d, want 0: %+v", len(diags), diags)
	}
	if *reads != 1 {
		t.Fatalf("reads = %d, want 1: the check must look at the kernel", *reads)
	}
}

// TestCheckMPLSTTLPropagateReportsUnreadable covers a key the check could not
// read, and a value outside the 0 and 1 the kernel accepts.
//
// VALIDATES: a warning under doctor-mpls-ttl-propagate-unknown that carries
// the reason it could not tell, with a fix, for an absent key, a refused read
// and an unexpected value alike.
// PREVENTS: an unreadable key read as "TTL propagates", the silent default the
// principles forbid.
func TestCheckMPLSTTLPropagateReportsUnreadable(t *testing.T) {
	cases := []struct {
		name  string
		value string
		err   error
		cause string
	}{
		{"absent", "", fs.ErrNotExist, fs.ErrNotExist.Error()},
		{"refused", "", errors.New("sysctl read net.mpls.ip_ttl_propagate: permission denied"), "permission denied"},
		{"unexpected value", "2", nil, `"2"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withMPLSTTLPropagate(t, tc.value, tc.err)

			diags := checkMPLSTTLPropagate(diagnostic.DoctorCheckContext{Tree: labelWorkOnKernelFIB()})
			if len(diags) != 1 {
				t.Fatalf("diagnostics = %d, want 1: %+v", len(diags), diags)
			}
			d := diags[0]
			if d.Code != diagnostic.CodeDoctorMPLSTTLPropagateUnknown || d.Severity != diagnostic.SeverityWarning {
				t.Fatalf("diagnostic = %s/%s, want %s at warning", d.Code, d.Severity, diagnostic.CodeDoctorMPLSTTLPropagateUnknown)
			}
			if !strings.Contains(d.Message, tc.cause) {
				t.Errorf("message %q does not carry the cause %q", d.Message, tc.cause)
			}
			if !strings.Contains(d.Message, "RFC 3032 Section 2.4.3") {
				t.Errorf("message %q does not name RFC 3032 Section 2.4.3", d.Message)
			}
			if d.Fix == "" {
				t.Error("fix is empty")
			}
		})
	}
}

// TestCheckMPLSTTLPropagateQuietWithoutKernelLabelWork covers every config
// under which fib-kernel pushes no label.
//
// VALIDATES: no diagnostic and no read when no label work is configured, when
// the label work runs on another data plane (no `fib { kernel { } }`), and
// when no config was parsed.
// PREVENTS: a warning about an absent key on every box with no MPLS module,
// and a kernel question asked of a VPP router that never uses the kernel's
// MPLS table.
func TestCheckMPLSTTLPropagateQuietWithoutKernelLabelWork(t *testing.T) {
	kernelOnly := config.NewTree()
	kernelOnly.GetOrCreateContainer("fib").GetOrCreateContainer("kernel")
	otherPlane := config.NewTree()
	otherPlane.GetOrCreateContainer("ldp")

	cases := []struct {
		name string
		tree any
	}{
		{"no label work", kernelOnly},
		{"label work on another data plane", otherPlane},
		{"no config", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reads := withMPLSTTLPropagate(t, "", fs.ErrNotExist)

			if diags := checkMPLSTTLPropagate(diagnostic.DoctorCheckContext{Tree: tc.tree}); len(diags) != 0 {
				t.Fatalf("diagnostics = %d, want 0: %+v", len(diags), diags)
			}
			if *reads != 0 {
				t.Fatalf("reads = %d, want 0", *reads)
			}
		})
	}
}

// TestMPLSTTLPropagateDoctorCheckRegistered asks the registry the doctor
// runner reads whether it holds this check at its declared phase and order,
// and whether both codes resolve for `ze show diagnostic`.
//
// VALIDATES: the init() in register_linux.go installed the check.
// PREVENTS: a check that `ze doctor` never runs while its unit tests pass.
func TestMPLSTTLPropagateDoctorCheckRegistered(t *testing.T) {
	want := mplsTTLPropagateDoctorCheck
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("doctor check %q is not registered for phase %q", want.Name, want.Phase)
	}
	if found.Order != want.Order || found.Component != pluginName {
		t.Fatalf("registered %d/%q, want %d/%q", found.Order, found.Component, want.Order, pluginName)
	}

	diagnostic.RegisterBuiltinCodes()
	for _, code := range want.Codes {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is not registered", code)
		}
	}
}
