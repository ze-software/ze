// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the IKE bypass on Ze's own IKE sockets
// Related: udp.go -- exemptFromIPsec and BypassError, the record under test

package transport

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"strings"
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

// VALIDATES: the Warn line of a refused bypass, and BypassFix, which the engine's doctor
// finding also names, give the restart for this process: `sudo systemctl restart ze`
// under ze.service, and ze signal stop then ze start, with no systemctl, for a ze
// started another way. Neither names `ze signal restart`, which exits without starting
// a new process.
// METHOD: fix the membership through privilege.SetServiceMembershipForTest, exempt a
// socket through a refusing bypass and read the log line and BypassFix.
// PREVENTS: an operator whose ze runs outside a supervisor told to run a command that
// leaves ze stopped, or told to run systemctl for a ze systemd did not start.
func TestRefusedIKESocketBypassNamesTheRestartForThisProcess(t *testing.T) {
	refused := func(t *testing.T) (logLine, fix string) {
		t.Helper()
		var logged bytes.Buffer
		tr, err := NewUDPTransport("127.0.0.1:0", slog.New(slog.NewTextHandler(&logged, nil)))
		if err != nil {
			t.Fatalf("open the socket: %v", err)
		}
		t.Cleanup(func() { tr.Close() }) //nolint:errcheck // the socket is discarded with the test
		logged.Reset()
		tr.exemptFromIPsec("127.0.0.1:0", func(*net.UDPConn) error {
			return errors.New("transport: setsockopt IP_XFRM_POLICY, direction 1: operation not permitted")
		})
		return logged.String(), BypassFix()
	}
	t.Run("ze.service", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
		logLine, fix := refused(t)
		want := "Run ze with CAP_NET_ADMIN on a kernel with CONFIG_XFRM, then restart it: sudo systemctl restart ze"
		if fix != want {
			t.Fatalf("BypassFix under ze.service is %q, want %q", fix, want)
		}
		if !strings.Contains(logLine, want) {
			t.Errorf("the Warn line does not name %q:\n%s", want, logLine)
		}
		if strings.Contains(logLine, "ze signal restart") {
			t.Errorf("the Warn line names ze signal restart, which leaves ze stopped:\n%s", logLine)
		}
	})
	t.Run("other", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
		logLine, fix := refused(t)
		for name, text := range map[string]string{"BypassFix": fix, "the Warn line": logLine} {
			if strings.Contains(text, "systemctl") {
				t.Errorf("outside ze.service %s names systemctl: %q", name, text)
			}
			if strings.Contains(text, "ze signal restart") {
				t.Errorf("%s names ze signal restart, which leaves ze stopped: %q", name, text)
			}
			for _, want := range []string{"CAP_NET_ADMIN", "CONFIG_XFRM", "ze signal stop", "ze start"} {
				if !strings.Contains(text, want) {
					t.Errorf("outside ze.service %s does not name %q: %q", name, want, text)
				}
			}
		}
	})
}
