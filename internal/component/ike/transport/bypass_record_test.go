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

	"github.com/ze-software/ze/internal/core/diagnostic"
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
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "code="+diagnostic.CodeDoctorIPsecIKESocketBypass) {
		t.Fatalf("the refusal was not logged at Warn with its doctor code:\n%s", out)
	}

	tr.exemptFromIPsec("127.0.0.1:0", func(*net.UDPConn) error { return nil })
	if tr.BypassError() != nil {
		t.Fatalf("BypassError answered %v for a socket that carries its bypass", tr.BypassError())
	}
}
