// Design: docs/architecture/diagnostics/active-probes.md -- the fallback guard, proven without privilege
//
// VALIDATES: the Security Review row "Authorization failing open": the
// unprivileged datagram socket is opened only when the raw socket was
// refused for privilege, and every other raw refusal stays a refusal.
// PREVENTS: a datagram socket quietly standing in for a raw one on a host
// whose real defect is not privilege.

package probe

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"syscall"
	"testing"
)

// stubOpeners replaces both socket constructions for one test. rawErr is
// what the raw open answers; the datagram open records whether it was
// reached and answers dgramErr, or a loopback UDP conn with identifier
// stubDatagramID when dgramErr is nil.
func stubOpeners(t *testing.T, rawErr, dgramErr error) *bool {
	t.Helper()
	fallbackTried := false
	openRawICMP = func(context.Context, string, string, func(string, string, syscall.RawConn) error) (net.PacketConn, error) {
		return nil, rawErr
	}
	openDatagramICMP = func(Family, netip.Addr, DFMode) (net.PacketConn, uint16, error) {
		fallbackTried = true
		if dgramErr != nil {
			return nil, 0, dgramErr
		}
		var lc net.ListenConfig
		conn, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("stand-in conn: %v", err)
		}
		return conn, stubDatagramID, nil
	}
	t.Cleanup(func() {
		openRawICMP = listenRawICMP
		openDatagramICMP = listenDatagramICMP
	})
	return &fallbackTried
}

const stubDatagramID = 0x4d2e

// TestOpenICMPFallsBackOnlyOnPrivilegeRefusal is the guard test: a raw
// refusal that is not EPERM or EACCES never reaches the datagram opener,
// and the error the caller sees is the raw one.
func TestOpenICMPFallsBackOnlyOnPrivilegeRefusal(t *testing.T) {
	fallbackTried := stubOpeners(t, syscall.EAFNOSUPPORT, nil)
	sock, err := OpenICMP(context.Background(), FamilyIPv4, netip.Addr{}, DFOff)
	if err == nil {
		sock.Close() //nolint:errcheck // the test failed already
		t.Fatal("a raw refusal for EAFNOSUPPORT opened a socket")
	}
	if !errors.Is(err, syscall.EAFNOSUPPORT) {
		t.Errorf("error %v does not carry the raw refusal", err)
	}
	if *fallbackTried {
		t.Error("the datagram opener was reached for a refusal that is not privilege")
	}
}

// TestOpenICMPFallsBackOnPrivilegeRefusal proves the fallback: EPERM and
// EACCES on the raw open reach the datagram opener, and the Socket that
// comes back carries the datagram kind and the identifier the opener named.
func TestOpenICMPFallsBackOnPrivilegeRefusal(t *testing.T) {
	for _, rawErr := range []error{syscall.EPERM, syscall.EACCES} {
		fallbackTried := stubOpeners(t, rawErr, nil)
		sock, err := OpenICMP(context.Background(), FamilyIPv4, netip.Addr{}, DFOff)
		if err != nil {
			t.Fatalf("%v: OpenICMP: %v", rawErr, err)
		}
		if !*fallbackTried {
			t.Errorf("%v: the datagram opener was not reached", rawErr)
		}
		if sock.Kind() != SocketDatagram {
			t.Errorf("%v: kind %v, want datagram", rawErr, sock.Kind())
		}
		if sock.Identifier() != stubDatagramID {
			t.Errorf("%v: identifier %#x, want %#x from the opener", rawErr, sock.Identifier(), stubDatagramID)
		}
		if err := sock.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}
}

// TestOpenICMPNamesBothRefusals: when neither socket opens, the error
// carries the datagram refusal and names both fixes, so an operator learns
// whether to grant CAP_NET_RAW or widen ping_group_range.
func TestOpenICMPNamesBothRefusals(t *testing.T) {
	stubOpeners(t, syscall.EPERM, syscall.EACCES)
	_, err := OpenICMP(context.Background(), FamilyIPv4, netip.Addr{}, DFOff)
	if err == nil {
		t.Fatal("both refused, yet a socket opened")
	}
	if !errors.Is(err, syscall.EACCES) {
		t.Errorf("error %v does not carry the datagram refusal", err)
	}
	for _, want := range []string{"CAP_NET_RAW", "ping_group_range"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}
}

// TestSocketTranslatesTheDatagramAddress proves the address contract at
// its refusing edge: a datagram-kind Socket takes an *net.IPAddr only,
// while the conn underneath speaks *net.UDPAddr. The accepting edge, an
// IPAddr reaching the wire and a reply naming an IPAddr, is
// TestProbeUnprivilegedSocketReportsNextHopMTU against the kernel.
func TestSocketTranslatesTheDatagramAddress(t *testing.T) {
	stubOpeners(t, syscall.EPERM, nil)
	sock, err := OpenICMP(context.Background(), FamilyIPv4, netip.Addr{}, DFOff)
	if err != nil {
		t.Fatalf("OpenICMP: %v", err)
	}
	defer sock.Close() //nolint:errcheck // test cleanup
	local, ok := sock.PacketConn().LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("stand-in conn speaks %T, want *net.UDPAddr", sock.PacketConn().LocalAddr())
	}
	if _, err := sock.WriteTo([]byte("x"), &net.UDPAddr{IP: local.IP, Port: local.Port}); err == nil {
		t.Error("a *net.UDPAddr was accepted: the prober's IPAddr contract is not enforced")
	}
}
