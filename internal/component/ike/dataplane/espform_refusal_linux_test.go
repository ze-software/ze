//go:build linux

package dataplane

import (
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
)

// TestESPFormOpenRefusalsCarryTheirFix proves every failure of the complete path's open
// that the host can cause is an operator error that wraps the kernel's answer, names the
// cause in its problem, and names the fix for that answer: the capability install for a
// refused privilege on the raw socket, its policy exemption and a step of the device's
// configuration; the journal's security-module denials for EACCES, which no capability
// changes; the unit install that raises LimitNOFILE for the process's file limit, the
// fs.file-max write for the host's, the free-memory listing for a kernel short of memory;
// and the receiver's warnings for an answer Ze cannot classify, which it retries by itself.
//
// MUTATION: in espFormSocketRefusal, answer EACCES with the EPERM refusal: the EACCES row
// answers the capability install and names no security module.
// MUTATION: in espFormBypassRefusal, drop the EPERM test: EPERM answers the warnings.
// MUTATION: in espFormOpenFailure, drop the EMFILE test: EMFILE answers the warnings.
// MUTATION: in espFormOpenFailure, drop the ENOBUFS test: ENOBUFS answers the warnings.
// MUTATION: in espFormTUNStepRefusal, answer ESPFormFixLog for EPERM too: the refused
// step answers the warnings.
func TestESPFormOpenRefusalsCarryTheirFix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		refusal error
		errno   unix.Errno
		problem string
		fix     string
	}{
		{"raw socket, EPERM", espFormSocketRefusal(unix.EPERM), unix.EPERM, "no permission", privilege.DaemonCapabilitiesFix},
		{"raw socket, EACCES", espFormSocketRefusal(unix.EACCES), unix.EACCES, "security module", espFormFixSecurityModule},
		{"raw socket, EMFILE", espFormSocketRefusal(unix.EMFILE), unix.EMFILE, "Ze has reached its limit of open files", espFormFixFileLimit},
		{"raw socket, ENFILE", espFormSocketRefusal(unix.ENFILE), unix.ENFILE, "the host has reached its limit of open files", espFormFixFileTable},
		{"raw socket, ENOBUFS", espFormSocketRefusal(unix.ENOBUFS), unix.ENOBUFS, "short of memory", espFormFixMemory},
		{"raw socket, ENOMEM", espFormSocketRefusal(unix.ENOMEM), unix.ENOMEM, "short of memory", espFormFixMemory},
		{"raw socket, EAFNOSUPPORT", espFormSocketRefusal(unix.EAFNOSUPPORT), unix.EAFNOSUPPORT, "Ze cannot open the raw ESP socket", ESPFormFixLog},
		{"policy exemption, EPERM", espFormBypassRefusal(unix.EPERM), unix.EPERM, "no permission", privilege.DaemonCapabilitiesFix},
		{"policy exemption, EACCES", espFormBypassRefusal(unix.EACCES), unix.EACCES, "security module", espFormFixSecurityModule},
		{"policy exemption, ENOMEM", espFormBypassRefusal(unix.ENOMEM), unix.ENOMEM, "short of memory", espFormFixMemory},
		{"policy exemption, EINVAL", espFormBypassRefusal(unix.EINVAL), unix.EINVAL, "Ze cannot exempt its raw ESP socket", ESPFormFixLog},
		{"device step, EPERM", espFormTUNStepRefusal("LinkSetUp", "bring it up", unix.EPERM), unix.EPERM, "no permission to bring it up", privilege.DaemonCapabilitiesFix},
		{"device step, EIO", espFormTUNStepRefusal("LinkSetUp", "bring it up", unix.EIO), unix.EIO, "could not bring it up", ESPFormFixLog},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oe, ok := errors.AsType[*opererr.Error](tc.refusal)
			if !ok {
				t.Fatalf("the refusal %v is not an operator error", tc.refusal)
			}
			if oe.Fix() != tc.fix {
				t.Errorf("the refusal's fix is %q, want %q", oe.Fix(), tc.fix)
			}
			if !strings.Contains(oe.Problem(), tc.problem) {
				t.Errorf("the refusal's problem %q does not name %q", oe.Problem(), tc.problem)
			}
			if !errors.Is(tc.refusal, tc.errno) {
				t.Errorf("the refusal %v does not wrap the kernel's %v", tc.refusal, tc.errno)
			}
		})
	}
}

// espFormUnservedMigration is the move the refusal tests below describe.
var espFormUnservedMigration = TunnelMigration{
	OldLocal: net.ParseIP("198.51.100.1"), OldRemote: net.ParseIP("198.51.100.2"),
	NewLocal: net.ParseIP("192.0.2.1"), NewRemote: net.ParseIP("192.0.2.2"),
	InboundSPI: 0x71910008, OutboundSPI: 0x71910009, IfID: 7, ReqID: 740,
	LocalPort: espFormUDPPort, RemotePort: 62001,
}

// TestXFRMMigrationUnservedRendersTheFixOnce proves the refusal of a move whose paths
// cannot open renders one fix line, the cause's, with the cause's problem and the
// kernel's answer as the reason's evidence, and still wraps the cause.
//
// MUTATION: in xfrmMigrationUnserved, wrap err itself rather than espFormRefusalCause:
// the cause renders its own fix line, so the refusal carries two.
func TestXFRMMigrationUnservedRendersTheFixOnce(t *testing.T) {
	cause := espFormSocketRefusal(unix.EPERM)
	causeOE, ok := errors.AsType[*opererr.Error](cause)
	if !ok {
		t.Fatalf("the cause %v is not an operator error", cause)
	}
	err := xfrmMigrationUnserved(cause, espFormUnservedMigration, netip.MustParseAddr("192.0.2.1"))
	text := err.Error()
	if got := strings.Count(text, "fix: "); got != 1 {
		t.Errorf("the refusal renders %d fix lines, want one:\n%s", got, text)
	}
	if !strings.HasSuffix(text, "fix: "+privilege.DaemonCapabilitiesFix) {
		t.Errorf("the refusal does not end with the cause's fix:\n%s", text)
	}
	for _, want := range []string{causeOE.Problem(), unix.EPERM.Error()} {
		if !strings.Contains(text, want) {
			t.Errorf("the refusal does not carry %q:\n%s", want, text)
		}
	}
	if !errors.Is(err, cause) {
		t.Errorf("the refusal %v does not wrap the cause", err)
	}
	if !errors.Is(err, unix.EPERM) {
		t.Errorf("the refusal %v does not wrap the kernel's EPERM", err)
	}
}

// TestXFRMMigrationUnservedPassesADefectThrough proves a cause that is no operator error,
// which only a Ze defect produces (the nil receiver's errNoESPFormReceiver), is no
// operator error with a made-up fix: it passes through with the move's context.
//
// MUTATION: in xfrmMigrationUnserved, answer a cause that is no operator error with
// opererr.Wrap and the fix ze show doctor: errors.As finds an operator error.
func TestXFRMMigrationUnservedPassesADefectThrough(t *testing.T) {
	err := xfrmMigrationUnserved(errNoESPFormReceiver, espFormUnservedMigration, netip.MustParseAddr("192.0.2.1"))
	if oe, ok := errors.AsType[*opererr.Error](err); ok {
		t.Errorf("a defect is answered as an operator error with the fix %q", oe.Fix())
	}
	if !errors.Is(err, errNoESPFormReceiver) {
		t.Errorf("the refusal %v does not wrap the defect", err)
	}
	if !strings.Contains(err.Error(), "inbound SPI 1905328136") {
		t.Errorf("the refusal %v does not name the move's SPI", err)
	}
}

// TestXFRMMigrationPanicDuringTheMoveEndsTheHold proves a panic between the acquire and
// the end of the hold, here in the state move, still ends the hold, so the receiver's
// paths do not stay open with no SA watched for the rest of the daemon's life.
//
// METHOD: the fake SAD seam panics during the state move; the test recovers it.
// MUTATION: in MigrateTunnel, drop the deferred release: the panic leaves holds at 1 and
// the paths open.
func TestXFRMMigrationPanicDuringTheMoveEndsTheHold(t *testing.T) {
	m := espFormUnservedMigration
	r := espFormTestReceiver(t, func() (espFormPaths, error) { return newIdleESPFormPaths(), nil })
	b := &xfrmMobikeBackend{xfrmBackend: &xfrmBackend{espForms: r}}
	migrationFakeKernel(t, m, 0, func() { panic("test: the state move panics") })

	func() {
		defer func() {
			if recover() == nil {
				t.Error("the state move's panic did not reach MigrateTunnel's caller")
			}
		}()
		if err := b.MigrateTunnel(m); err != nil {
			t.Errorf("the move answered %v instead of panicking", err)
		}
	}()
	if r.holds != 0 {
		t.Errorf("after the panic the receiver holds its paths %d times, want none", r.holds)
	}
	if r.running() {
		t.Error("after the panic the receiver's paths are open with no SA watched")
	}
}
