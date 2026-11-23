// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the IKE bypass on Ze's own IKE sockets
// Related: udp.go -- exemptFromIPsec and BypassError, the record under test

package transport

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/core/privilege"
)

// VALIDATES: a bypass the kernel refuses is kept on the transport, so BypassError
// answers it to the engine's doctor check, and it is logged at Warn with its doctor
// code; a bypass that is set leaves BypassError nil.
// METHOD: open a socket, exempt it again through a refusing bypass and read the record
// and the log; then through an accepting one.
// PREVENTS: the refusal living only in a log line, with nothing doctor can read.
func TestRefusedIKESocketBypassIsRecorded(t *testing.T) {
	var logged bytes.Buffer
	tr, err := NewUDPTransport("127.0.0.1:0", slog.New(slog.NewTextHandler(&logged, nil)))
	if err != nil {
		t.Fatalf("open the socket: %v", err)
	}
	t.Cleanup(func() { tr.Close() }) //nolint:errcheck // the socket is discarded with the test

	refusal := errors.New("transport: setsockopt IP_XFRM_POLICY, direction 1: operation not permitted")
	logged.Reset()
	tr.exemptFromIPsec("127.0.0.1:0", func(*net.UDPConn) error { return refusal })
	if !errors.Is(tr.BypassError(), refusal) {
		t.Fatalf("BypassError answered %v, want the kernel's refusal", tr.BypassError())
	}
	out := logged.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "code="+CodeDoctorIPsecIKESocketBypass) {
		t.Fatalf("the refusal was not logged at Warn with its doctor code:\n%s", out)
	}

	tr.exemptFromIPsec("127.0.0.1:0", func(*net.UDPConn) error { return nil })
	if tr.BypassError() != nil {
		t.Fatalf("BypassError answered %v for a socket that carries its bypass", tr.BypassError())
	}
}

// VALIDATES: the Warn line of a refused bypass, and BypassFix and BypassCause, which the
// engine's doctor finding also names, give the cause the kernel's errno shows and the
// fix for that cause, worded for this process: EPERM is a missing CAP_NET_ADMIN, whose
// fix is the capability remedy (the unit reinstall under ze.service); ENOPROTOOPT a
// kernel without CONFIG_XFRM; EACCES a security module; any other answer is named as
// unclassified with both fixes. Every restart is `sudo systemctl restart ze` under
// ze.service and ze signal stop then ze start, with no systemctl, for a ze started
// another way. None names `ze signal restart`, which exits without starting a new
// process.
// METHOD: for each errno and each membership (privilege.SetServiceMembershipForTest),
// exempt a socket through a bypass refusing with that errno, wrapped as
// dataplane.BypassIKESocket wraps it, and compare BypassFix and BypassCause with the
// exact text; read the Warn line for both.
// PREVENTS: an operator under ze.service whose unit lacks CAP_NET_ADMIN told to restart,
// which changes nothing; an operator on a kernel without XFRM told about a capability
// ze holds; an operator whose ze runs outside a supervisor told to run a command that
// leaves ze stopped, or told to run systemctl for a ze systemd did not start.
func TestRefusedIKESocketBypassNamesTheRestartForThisProcess(t *testing.T) {
	const (
		serviceRestart = "sudo systemctl restart ze"
		otherRestart   = "no single command restarts an unsupervised ze: run ze signal stop, then start ze " +
			"again the way it was started (ze start, or ze start <config-file>)"
		serviceCapability = "sudo systemctl stop ze && sudo ze install systemd --force --start"
		otherCapability   = "restart ze as root, or holding CAP_NET_ADMIN: grant it as an ambient capability " +
			"from whatever starts ze, or run sudo setcap cap_net_admin+ep on the ze binary"
		kernel   = "boot a kernel built with CONFIG_XFRM (Ze's own kernel carries it), then restart ze: "
		security = "find the denial with sudo journalctl -b --grep 'avc:|apparmor=\"DENIED\"', allow ze the " +
			"IP_XFRM_POLICY socket option in that policy, then restart ze: "
	)
	classes := []struct {
		name                 string
		errno                syscall.Errno
		cause                string
		serviceFix, otherFix string
	}{
		{
			name:       "EPERM",
			errno:      syscall.EPERM,
			cause:      "ze lacks CAP_NET_ADMIN, which the kernel asks of a process that sets a socket's IPsec policy",
			serviceFix: serviceCapability,
			otherFix:   otherCapability,
		},
		{
			name:       "ENOPROTOOPT",
			errno:      syscall.ENOPROTOOPT,
			cause:      "the kernel is built without CONFIG_XFRM, so it has no IPsec socket policy to set",
			serviceFix: kernel + serviceRestart,
			otherFix:   kernel + otherRestart,
		},
		{
			name:       "EACCES",
			errno:      syscall.EACCES,
			cause:      "a security module (SELinux or AppArmor) refused ze the IP_XFRM_POLICY socket option",
			serviceFix: security + serviceRestart,
			otherFix:   security + otherRestart,
		},
		{
			name:  "EINVAL",
			errno: syscall.EINVAL,
			cause: "Ze could not tell why from the kernel's answer: ze may lack CAP_NET_ADMIN, " +
				"or the kernel may be built without CONFIG_XFRM",
			serviceFix: "if ze lacks CAP_NET_ADMIN: " + serviceCapability + "; if the kernel lacks CONFIG_XFRM: " +
				kernel + serviceRestart,
			otherFix: "if ze lacks CAP_NET_ADMIN: " + otherCapability + "; if the kernel lacks CONFIG_XFRM: " +
				kernel + otherRestart,
		},
	}
	memberships := []struct {
		name       string
		membership privilege.ServiceMembership
	}{
		{"ze.service", privilege.MembershipZeService},
		{"other", privilege.MembershipOther},
	}
	for _, class := range classes {
		for _, member := range memberships {
			t.Run(class.name+"/"+member.name, func(t *testing.T) {
				t.Cleanup(privilege.SetServiceMembershipForTest(member.membership, nil))
				var logged bytes.Buffer
				tr, err := NewUDPTransport("127.0.0.1:0", slog.New(slog.NewTextHandler(&logged, nil)))
				if err != nil {
					t.Fatalf("open the socket: %v", err)
				}
				t.Cleanup(func() { tr.Close() }) //nolint:errcheck // the socket is discarded with the test
				logged.Reset()
				tr.exemptFromIPsec("127.0.0.1:0", func(*net.UDPConn) error {
					return fmt.Errorf("transport: setsockopt IP_XFRM_POLICY, direction 1: %w", class.errno)
				})
				want := class.serviceFix
				if member.membership == privilege.MembershipOther {
					want = class.otherFix
				}
				if fix := BypassFix(tr.BypassError()); fix != want {
					t.Errorf("BypassFix is %q, want %q", fix, want)
				}
				if cause := BypassCause(tr.BypassError()); cause != class.cause {
					t.Errorf("BypassCause is %q, want %q", cause, class.cause)
				}
				logLine := logged.String()
				for _, part := range []string{class.cause, want} {
					// slog's text handler quotes the message, so a quote in it reads \".
					if !strings.Contains(logLine, strings.ReplaceAll(part, `"`, `\"`)) {
						t.Errorf("the Warn line does not name %q:\n%s", part, logLine)
					}
				}
				if strings.Contains(logLine, "ze signal restart") {
					t.Errorf("the Warn line names ze signal restart, which leaves ze stopped:\n%s", logLine)
				}
				if member.membership == privilege.MembershipOther && strings.Contains(logLine, "systemctl") {
					t.Errorf("outside ze.service the Warn line names systemctl:\n%s", logLine)
				}
			})
		}
	}
}
