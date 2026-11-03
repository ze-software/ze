//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- synchronous AC expectations.
// Related: expect_linux.go -- keepalive service without another socket reader.

package pppoeac

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestExpectedPacketEcho interleaves Echo with phase control and IPv4 data,
// checking the exact reply and the eventual packet across distinct LCP epochs.
func TestExpectedPacketEcho(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	ac, client := expectationWire(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	for _, magic := range []uint32{0x10203040, 0x50607080, 0} {
		expectationNegotiate(t, ctx, ac, client, magic, 0xaabbccdd)
		for _, proto := range []uint16{ppp.ProtoIPCP, ppp.ProtoPAP, ppp.ProtoIPv4} {
			// RFC 1661 Section 5.8. Include binary trailing data and a nontrivial ID.
			request := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0, 0xff, 0x80, 7}
			if err := client.SendControl(ppp.ProtoLCP, ppp.LCPEchoRequest, 213, request); err != nil {
				t.Fatal(err)
			}
			want := []byte{ppp.LCPConfigureRequest, 19, 0, 7, 3, 3, 9}
			if err := client.SendPacket(proto, want); err != nil {
				t.Fatal(err)
			}
			var got []byte
			var received uint16
			var err error
			if proto == ppp.ProtoIPv4 {
				received, got, err = ac.ReadExpectedPacket(ctx)
			} else {
				var packet ppp.LCPPacket
				received, packet, err = ac.ReadExpectedControl(ctx)
				if packet.Code != want[0] {
					t.Fatalf("expected control code %d, received %d", want[0], packet.Code)
				}
				if packet.Identifier != want[1] {
					t.Fatalf("expected control ID %d, received %d", want[1], packet.Identifier)
				}
				got, want = packet.Data, want[4:]
			}
			if err != nil {
				t.Fatal(err)
			}
			if received != proto {
				t.Fatalf("protocol = %x, want %x", received, proto)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("packet = %x, want %x", got, want)
			}
			// Raw reads must expose Echo; checking the full payload pins ID and tail.
			received, reply, err := client.ReadPacket(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if received != ppp.ProtoLCP {
				t.Fatalf("reply protocol = %x", received)
			}
			wantReply := []byte{10, 213, 0, 12, byte(magic >> 24), byte(magic >> 16), byte(magic >> 8), byte(magic), 0, 0xff, 0x80, 7}
			if !bytes.Equal(reply, wantReply) {
				t.Fatalf("reply = %x, want %x", reply, wantReply)
			}
		}
	}
}

// TestExpectedPacketKeepsFaults proves that control faults, reentry and termination
// remain visible. Only a complete LCP Echo-Request is serviced internally.
func TestExpectedPacketKeepsFaults(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	ac, client := expectationWire(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	for _, payload := range [][]byte{
		{ppp.LCPTerminateRequest, 1, 0, 4},
		{ppp.LCPConfigureRequest, 2, 0, 4},
		{ppp.LCPEchoReply, 3, 0, 8, 1, 2, 3, 4},
		{ppp.LCPEchoRequest, 4, 0, 7, 1, 2, 3},
		{ppp.LCPEchoRequest, 5, 0, 9, 1, 2, 3, 4},
	} {
		if err := client.SendPacket(ppp.ProtoLCP, payload); err != nil {
			t.Fatal(err)
		}
		proto, got, err := ac.ReadExpectedPacket(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if proto != ppp.ProtoLCP {
			t.Fatalf("fault protocol = %x", proto)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("fault changed: %x, want %x", got, payload)
		}
	}
	// A raw control read must still return an Echo-Request unchanged.
	if err := client.SendControl(ppp.ProtoLCP, ppp.LCPEchoRequest, 9, []byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	_, packet, err := ac.ReadControl(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Code != ppp.LCPEchoRequest {
		t.Fatalf("raw read swallowed Echo: %+v", packet)
	}
}

// TestExpectedPacketDeadline checks that queued Echo traffic cannot extend the
// supplied deadline, including when a packet is already available on the socket.
func TestExpectedPacketDeadline(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	ac, client := expectationWire(t)
	for id := range 16 {
		if err := client.SendControl(ppp.ProtoLCP, ppp.LCPEchoRequest, uint8(id), []byte{1, 2, 3, 4}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	if _, _, err := ac.ReadExpectedPacket(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired expectation = %v", err)
	}
	live, stop := context.WithTimeout(t.Context(), 3*time.Second)
	defer stop()
	_, packet, err := ac.ReadControl(live)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Identifier != 0 {
		t.Fatalf("expired expectation consumed packet: %+v", packet)
	}
}

// TestExpectedPacketClosedLCP discards a queued old-epoch Echo during LCP, then
// exposes the next Configure-Request without emitting a forbidden Echo-Reply.
func TestExpectedPacketClosedLCP(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	ac, client := expectationWire(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if err := client.SendControl(ppp.ProtoLCP, ppp.LCPEchoRequest, 11, []byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	if err := client.SendControl(ppp.ProtoLCP, ppp.LCPConfigureRequest, 12, nil); err != nil {
		t.Fatal(err)
	}
	proto, packet, err := ac.ReadExpectedControl(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if proto != ppp.ProtoLCP {
		t.Fatalf("protocol = %x", proto)
	}
	if packet.Code != ppp.LCPConfigureRequest {
		t.Fatalf("code = %d", packet.Code)
	}
	if packet.Identifier != 12 {
		t.Fatalf("identifier = %d", packet.Identifier)
	}
	// This positive barrier must be the first returned packet: an Echo reply
	// emitted before it cannot hide behind a timeout or an inactive socket.
	if err := ac.SendControl(ppp.ProtoLCP, ppp.LCPConfigureAck, 12, nil); err != nil {
		t.Fatal(err)
	}
	_, packet, err = client.ReadControl(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Code != ppp.LCPConfigureAck {
		t.Fatalf("closed LCP emitted an Echo reply: %+v", packet)
	}
}

// expectationNegotiate drives both real wire exchanges, including exact Acks.
func expectationNegotiate(t *testing.T, ctx context.Context, ac, client *Session, local, peer uint32) {
	t.Helper()
	ours, theirs := expectationOptions(local), expectationOptions(peer)
	expectationTransfer(t, ctx, ac, client, ppp.LCPConfigureRequest, 21, ours)
	expectationTransfer(t, ctx, client, ac, ppp.LCPConfigureRequest, 22, theirs)
	expectationTransfer(t, ctx, client, ac, ppp.LCPConfigureAck, 21, ours)
	expectationTransfer(t, ctx, ac, client, ppp.LCPConfigureAck, 22, theirs)
}

// expectationOptions omits Magic when zero is requested by a fixture.
func expectationOptions(magic uint32) []byte {
	if magic == 0 {
		return nil
	}
	options := []byte{5, 6, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(options[2:], magic)
	return options
}

// expectationTransfer verifies each packet rather than setting observer state.
func expectationTransfer(t *testing.T, ctx context.Context, from, to *Session, code, id uint8, data []byte) {
	t.Helper()
	if err := from.SendControl(ppp.ProtoLCP, code, id, data); err != nil {
		t.Fatal(err)
	}
	proto, packet, err := to.ReadControl(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if proto != ppp.ProtoLCP {
		t.Fatalf("negotiation protocol = %x", proto)
	}
	if packet.Code != code {
		t.Fatalf("negotiation code = %d, want %d", packet.Code, code)
	}
	if packet.Identifier != id {
		t.Fatalf("negotiation ID = %d, want %d", packet.Identifier, id)
	}
	if !bytes.Equal(packet.Data, data) {
		t.Fatalf("negotiation data = %x, want %x", packet.Data, data)
	}
}

// expectationWire owns both endpoints synchronously, using the shared encoder.
func expectationWire(t *testing.T) (*Session, *Session) {
	t.Helper()
	peer := OpenVeth(t, "zec0", "zea0", 44)
	link, err := netlink.LinkByName("zec0")
	if err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, int(NetworkOrder(unix.ETH_P_ALL)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	})
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: link.Attrs().Index, Protocol: NetworkOrder(unix.ETH_P_ALL)}); err != nil {
		t.Fatal(err)
	}
	if err := pppoe.SetRecvTimeout(fd, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	var mac [6]byte
	copy(mac[:], link.Attrs().HardwareAddr)
	client, err := New(fd, link.Attrs().Index, mac, 44)
	if err != nil {
		t.Fatal(err)
	}
	return &Session{peer: peer, remote: mac}, &Session{peer: client, remote: peer.mac}
}
