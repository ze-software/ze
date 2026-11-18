//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- positive PPP declaration.
// Related: original_attachment_linux.go -- exact query42 and file custody.

package kernel

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/doctor"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// Query42 is the single frozen 16-byte declaration. No peer query may stand
// in for it, and no failed syscall is a positive capability reply.
func TestOriginalSupportProbeExactReply(t *testing.T) {
	if pppSupportQuery != 0xc010742a {
		t.Fatal("query42 encoding changed")
	}
	if unsafe.Sizeof(pppSupportReply{}) != 16 {
		t.Fatal("support layout changed")
	}
	if unsafe.Alignof(pppSupportReply{}) != 4 {
		t.Fatal("support alignment changed")
	}
	for _, test := range []struct {
		name     string
		reply    pppSupportReply
		queryErr error
		required uint32
		want     originalAttachmentStatus
	}{
		{"supported", pppSupportReply{abi: 1, features: pppSupportKnown}, nil, pppSupportSession, originalAttachmentSupported},
		{"role-minimal", pppSupportReply{abi: 1, features: pppSupportSession}, nil, pppSupportSession, originalAttachmentSupported},
		{"bridge", pppSupportReply{abi: 1, features: pppSupportSession | pppSupportBridge}, nil, pppSupportSession | pppSupportBridge, originalAttachmentSupported},
		{"missing-connect", pppSupportReply{abi: 1, features: pppSupportCapture}, nil, pppSupportSession, originalAttachmentUnavailable},
		{"missing-capture", pppSupportReply{abi: 1, features: pppSupportConnect}, nil, pppSupportSession, originalAttachmentUnavailable},
		{"missing-bridge", pppSupportReply{abi: 1, features: pppSupportSession}, nil, pppSupportSession | pppSupportBridge, originalAttachmentUnavailable},
		{"unavailable", pppSupportReply{}, unix.ENOTTY, pppSupportSession, originalAttachmentUnavailable},
		{"permission", pppSupportReply{}, unix.EPERM, pppSupportSession, originalAttachmentUnknown},
		{"dispatch-only", pppSupportReply{}, unix.EBADF, pppSupportSession, originalAttachmentUnknown},
		{"wrapped-dispatch", pppSupportReply{}, errors.Join(unix.EBADF), pppSupportSession, originalAttachmentUnknown},
		{"mixed-dispatch", pppSupportReply{}, errors.Join(unix.EBADF, unix.EPERM), pppSupportSession, originalAttachmentUnknown},
		{"refused", pppSupportReply{}, unix.EINVAL, pppSupportSession, originalAttachmentUnknown},
		{"busy", pppSupportReply{}, unix.EBUSY, pppSupportSession, originalAttachmentUnknown},
		{"version", pppSupportReply{abi: 2, features: pppSupportKnown}, nil, pppSupportSession, originalAttachmentUnknown},
		{"empty-success", pppSupportReply{}, nil, pppSupportSession, originalAttachmentUnknown},
		{"reserved0", pppSupportReply{abi: 1, features: pppSupportKnown, reserved0: 1}, nil, pppSupportSession, originalAttachmentUnknown},
		{"reserved1", pppSupportReply{abi: 1, features: pppSupportKnown, reserved1: 1}, nil, pppSupportSession, originalAttachmentUnknown},
		{"known-ipv6-control", pppSupportReply{abi: 1, features: pppSupportKnown}, nil, pppSupportSession, originalAttachmentSupported},
		{"undeclared-11", pppSupportReply{abi: 1, features: pppSupportKnown | 1<<11}, nil, pppSupportSession, originalAttachmentUnknown},
		{"mixed-error", pppSupportReply{abi: 1, features: pppSupportKnown}, errors.Join(unix.ENOTTY, unix.EPERM), pppSupportSession, originalAttachmentUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "support-")
			if err != nil {
				t.Fatal(err)
			}
			var calls, closes int
			result := inspectOriginalAttachmentWith(test.required, originalAttachmentProbeOps{
				open: func() (*os.File, error) { return file, nil },
				inspect: func(actual *os.File, request *pppSupportReply) error {
					calls++
					if actual != file {
						t.Fatal("probe replaced original file")
					}
					if *request != (pppSupportReply{abi: 1}) {
						t.Fatalf("invalid query input %+v", request)
					}
					*request = test.reply
					return test.queryErr
				},
				close: func(actual *os.File) error { closes++; return actual.Close() },
			})
			if result.status != test.want {
				t.Fatalf("status %v cause %v", result.status, result.cause)
			}
			if calls != 1 {
				t.Fatalf("queries %d", calls)
			}
			if closes != 1 {
				t.Fatalf("closes %d", closes)
			}
			if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("probe file leaked: %v", err)
			}
			if test.queryErr != nil {
				if !errors.Is(result.cause, test.queryErr) {
					t.Fatal("query cause lost")
				}
			}
		})
	}
}

// A complete declaration does not hide cleanup failure; open failure cannot
// be reclassified as a statement that the operation is absent.
func TestOriginalSupportProbeCleanupUnknown(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "support-")
	if err != nil {
		t.Fatal(err)
	}
	result := inspectOriginalAttachmentWith(pppSupportSession, originalAttachmentProbeOps{
		open:    func() (*os.File, error) { return file, nil },
		inspect: func(_ *os.File, reply *pppSupportReply) error { reply.features = pppSupportSession; return unix.ENOTTY },
		close:   func(actual *os.File) error { return errors.Join(actual.Close(), unix.EIO) },
	})
	if result.status != originalAttachmentUnknown {
		t.Fatal("failed close became absence/support")
	}
	for _, cause := range []error{unix.ENOTTY, unix.EIO} {
		if !errors.Is(result.cause, cause) {
			t.Fatalf("lost %v: %v", cause, result.cause)
		}
	}
	result = inspectOriginalAttachmentWith(pppSupportSession, originalAttachmentProbeOps{
		open:    func() (*os.File, error) { return nil, unix.EACCES },
		inspect: func(*os.File, *pppSupportReply) error { t.Fatal("query after failed open"); return nil },
		close:   func(*os.File) error { t.Fatal("close absent file"); return nil },
	})
	if result.status != originalAttachmentUnknown {
		t.Fatal("inaccessible device became absent")
	}
	if !errors.Is(result.cause, unix.EACCES) {
		t.Fatal(result.cause)
	}
}

// Cleanup retains independently meaningful reply refusal, including malformed
// data and missing bits, instead of replacing those causes with the close error.
func TestOriginalSupportProbeReplyCloseCauses(t *testing.T) {
	for _, test := range []struct {
		name  string
		reply pppSupportReply
		cause string
	}{
		{"positive", pppSupportReply{abi: 1, features: pppSupportSession}, ""},
		{"malformed", pppSupportReply{abi: 2, features: pppSupportSession}, "ABI"},
		{"missing", pppSupportReply{abi: 1}, "required feature bits"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "support-")
			if err != nil {
				t.Fatal(err)
			}
			result := inspectOriginalAttachmentWith(pppSupportSession, originalAttachmentProbeOps{
				open:    func() (*os.File, error) { return file, nil },
				inspect: func(_ *os.File, reply *pppSupportReply) error { *reply = test.reply; return nil },
				close:   func(actual *os.File) error { return errors.Join(actual.Close(), unix.EIO) },
			})
			if result.status != originalAttachmentUnknown {
				t.Fatalf("close failure status %v", result.status)
			}
			if !errors.Is(result.cause, unix.EIO) {
				t.Fatal("close cause lost")
			}
			if test.cause != "" {
				if !strings.Contains(result.cause.Error(), test.cause) {
					t.Fatal("reply cause lost")
				}
			}
		})
	}
}

// The owning caller supplies one check's configured role. An unused dependency
// does not open /dev/ppp, and bridge requirements are not silently dropped.
func TestOriginalSupportDoctorScopePort(t *testing.T) {
	for _, test := range []struct {
		name string
		want uint32
	}{
		{"unused", 0}, {"session", pppSupportSession}, {"bridge", pppSupportSession | pppSupportBridge},
	} {
		t.Run(test.name, func(t *testing.T) {
			var called uint32
			check := originalAttachmentDoctorCheck(func(required uint32) originalAttachmentResult {
				called = required
				return originalAttachmentResult{status: originalAttachmentSupported}
			}, originalAttachmentKernel, func(diagnostic.DoctorCheckContext) (uint32, error) { return test.want, nil })
			if got := check.Check(diagnostic.DoctorCheckContext{}); len(got) != 0 {
				t.Fatal(got)
			}
			if called != test.want {
				t.Fatalf("required %#x want %#x", called, test.want)
			}
		})
	}
}

// The public daemon runner discovers per-check classification without a
// mutable product hook. Real production provenance is tested separately.
func TestOriginalSupportDoctorRegisteredRunner(t *testing.T) {
	if diagnostic.Lookup(originalAttachmentDoctorCode) == nil {
		t.Fatal("unexplained code")
	}
	tree := config.NewTree()
	tree.GetOrCreateContainer("l2tp")
	for _, test := range []struct {
		name   string
		status originalAttachmentStatus
		cause  error
		text   string
	}{
		{"supported", originalAttachmentSupported, nil, ""},
		{"absent", originalAttachmentUnavailable, unix.ENOTTY, "lacks"},
		{"unknown", originalAttachmentUnknown, unix.EPERM, "cannot verify"},
	} {
		t.Run(test.name, func(t *testing.T) {
			check := originalAttachmentDoctorCheck(func(uint32) originalAttachmentResult {
				return originalAttachmentResult{status: test.status, cause: test.cause}
			}, originalAttachmentKernel,
				func(diagnostic.DoctorCheckContext) (uint32, error) { return pppSupportSession, nil })
			check.Name = "ppp-support-test-" + test.name
			check.Phase = diagnostic.DoctorPhaseRuntime
			if err := diagnostic.RegisterDoctorCheck(check); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { diagnostic.UnregisterDoctorCheckForTest(check.Name) })
			var rows []diagnostic.Diagnostic
			for _, row := range doctor.RunRuntimeChecks(tree, nil) {
				if row.Code == originalAttachmentDoctorCode {
					rows = append(rows, row)
				}
			}
			if test.text == "" {
				if len(rows) != 0 {
					t.Fatal(rows)
				}
				return
			}
			if len(rows) != 1 {
				t.Fatal(rows)
			}
			if !strings.Contains(rows[0].Message, test.text) {
				t.Fatal(rows[0].Message)
			}
			provenance, err := originalAttachmentKernel()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(rows[0].Message, provenance) {
				t.Fatal("actual answering kernel provenance lost")
			}
			if !strings.Contains(rows[0].Message, test.cause.Error()) {
				t.Fatal("cause lost")
			}
			if strings.Contains(rows[0].Fix, "firmware apply") {
				t.Fatal("universal firmware repair")
			}
			if test.status == originalAttachmentUnavailable {
				wantFix := "./le build host-driver\n./ze-host appliance kernel --target runtime --arch " + runtime.GOARCH
				if rows[0].Fix != wantFix {
					t.Fatalf("unavailable kernel fix=%q, want %q", rows[0].Fix, wantFix)
				}
			}
			if test.status == originalAttachmentUnknown {
				if rows[0].Fix != "ze show doctor" {
					t.Fatal(rows[0].Fix)
				}
			}
		})
	}
}
