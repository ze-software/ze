//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- script admission boundaries.

package pppoeac

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestScriptAdmission rejects invalid handles and lengths before any socket I/O.
// Positive wire behavior is exercised by TestScriptedACRenegotiatesDial.
func TestScriptAdmission(t *testing.T) {
	for _, input := range []struct {
		name      string
		fd, index int
		sid       uint16
	}{
		{"descriptor", -1, 1, 42},
		{"interface", 0, 0, 42},
		{"zero-session", 0, 1, 0},
		{"reserved-session", 0, 1, 65535},
	} {
		t.Run(input.name, func(t *testing.T) {
			if _, err := New(input.fd, input.index, [6]byte{2}, input.sid); err == nil {
				t.Fatal("invalid peer accepted")
			}
		})
	}
	for _, peer := range []*Peer{nil, {}} {
		if err := peer.SendFrame(nil); err == nil {
			t.Fatal("uninitialized peer sent frame")
		}
		if err := peer.SendControl(nil, 0, 0, 0, nil); err == nil {
			t.Fatal("uninitialized peer sent control")
		}
		if _, err := peer.Accept(t.Context()); err == nil {
			t.Fatal("uninitialized peer accepted discovery")
		}
		if err := peer.ServeNoAuth(t.Context(), nil); err == nil {
			t.Fatal("uninitialized peer served")
		}
	}
	peer, err := New(0, 1, [6]byte{2}, 42)
	if err != nil {
		t.Fatal(err)
	}
	if err := peer.SendFrame(make([]byte, 13)); err == nil {
		t.Fatal("short Ethernet header accepted")
	}
	if err := peer.SendControl(make([]byte, 5), 0, 0, 0, nil); err == nil {
		t.Fatal("short destination accepted")
	}
	scriptAdmissionError(t, peer.SendControl(make([]byte, 6), 0, 0, 0, make([]byte, pppoe.EthMaxLen-25)),
		"wire peer control packet exceeds the Ethernet buffer")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := peer.Accept(ctx); err == nil {
		t.Fatal("canceled discovery succeeded")
	}
	for _, session := range []*Session{nil, {}} {
		if err := session.SendControl(0, 0, 0, nil); err == nil {
			t.Fatal("uninitialized session sent control")
		}
		if _, _, err := session.ReadControl(t.Context()); err == nil {
			t.Fatal("uninitialized session read control")
		}
		scriptAdmissionError(t, session.CHAP(t.Context(), 1, nil, "name", "secret"),
			"CHAP challenge is empty")
		scriptAdmissionError(t, session.CHAP(t.Context(), 1, make([]byte, 256), "name", "secret"),
			"CHAP challenge exceeds one-octet Value-Size")
		if err := session.IPCP(t.Context(), 1, netip.Addr{}, netip.MustParseAddr("192.0.2.2")); err == nil {
			t.Fatal("invalid local IPv4 address accepted")
		}
		if err := session.IPCP(t.Context(), 1, netip.MustParseAddr("192.0.2.1"), netip.Addr{}); err == nil {
			t.Fatal("invalid remote IPv4 address accepted")
		}
	}
}

// TestScriptLengthAdmission isolates malformed lengths from invalid handles.
// A real bound socket and initialized session must return the length error,
// never a send failure or a timeout waiting for a response to invalid input.
func TestScriptLengthAdmission(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := OpenVeth(t, "zec0", "zea0", 42)
	client, err := net.InterfaceByName("zec0")
	if err != nil {
		t.Fatal(err)
	}
	session := &Session{peer: peer}
	if len(client.HardwareAddr) != len(session.remote) {
		t.Fatalf("client MAC length = %d, want %d", len(client.HardwareAddr), len(session.remote))
	}
	copy(session.remote[:], client.HardwareAddr)
	for _, input := range []struct {
		name string
		run  func(context.Context) error
		want string
	}{
		{"empty-challenge", func(ctx context.Context) error {
			return session.CHAP(ctx, 1, nil, "name", "secret")
		}, "CHAP challenge is empty"},
		{"oversized-challenge", func(ctx context.Context) error {
			return session.CHAP(ctx, 1, make([]byte, 256), "name", "secret")
		}, "CHAP challenge exceeds one-octet Value-Size"},
		{"oversized-control", func(context.Context) error {
			return peer.SendControl(session.remote[:], ppp.ProtoLCP, ppp.LCPConfigureRequest, 1,
				make([]byte, pppoe.EthMaxLen-25))
		}, "wire peer control packet exceeds the Ethernet buffer"},
	} {
		t.Run(input.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			scriptAdmissionError(t, input.run(ctx), input.want)
		})
	}
}

// scriptAdmissionError retains the rejection assertion and identifies its guard.
func scriptAdmissionError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("malformed input accepted, want %q", want)
	}
	if err.Error() != want {
		t.Fatalf("admission error = %q, want %q", err, want)
	}
}
