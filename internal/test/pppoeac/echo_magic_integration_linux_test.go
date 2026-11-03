//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- negotiated Echo identity.
// Related: echo_state_linux.go -- observations of actual accepted wire requests.

package pppoeac

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestExpectedEchoIPCP injects Echo before the client request and before its Ack.
// The shared exchange must finish and emit every exact control reply in order.
func TestExpectedEchoIPCP(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	ac, client := expectationWire(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	expectationNegotiate(t, ctx, ac, client, 11, 21)
	ours, theirs := []byte{3, 6, 192, 0, 2, 1}, []byte{3, 6, 192, 0, 2, 2}
	for _, step := range []struct {
		proto    uint16
		code, id uint8
		data     []byte
	}{
		{ppp.ProtoLCP, ppp.LCPEchoRequest, 70, []byte{0, 0, 0, 21, 0xff}},
		{ppp.ProtoIPCP, ppp.LCPConfigureRequest, 31, theirs},
		{ppp.ProtoLCP, ppp.LCPEchoRequest, 71, []byte{0, 0, 0, 21, 0x80}},
		{ppp.ProtoIPCP, ppp.LCPConfigureAck, 90, ours},
	} {
		if err := client.SendControl(step.proto, step.code, step.id, step.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := ac.IPCP(ctx, 90, netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		proto    uint16
		code, id uint8
		data     []byte
	}{
		{ppp.ProtoLCP, ppp.LCPEchoReply, 70, []byte{0, 0, 0, 11, 0xff}},
		{ppp.ProtoIPCP, ppp.LCPConfigureAck, 31, theirs},
		{ppp.ProtoIPCP, ppp.LCPConfigureRequest, 90, ours},
		{ppp.ProtoLCP, ppp.LCPEchoReply, 71, []byte{0, 0, 0, 11, 0x80}},
	} {
		proto, packet, err := client.ReadControl(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if proto != want.proto {
			t.Fatalf("protocol = %x, want %x", proto, want.proto)
		}
		if packet.Code != want.code {
			t.Fatalf("code = %d, want %d", packet.Code, want.code)
		}
		if packet.Identifier != want.id {
			t.Fatalf("ID = %d, want %d", packet.Identifier, want.id)
		}
		if !bytes.Equal(packet.Data, want.data) {
			t.Fatalf("data = %x, want %x", packet.Data, want.data)
		}
	}
}

// TestExpectedEchoErrors proves raw receive/parser errors still reach callers and
// a failed Ack write cannot establish negotiated magic.
func TestExpectedEchoErrors(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	ac, client := expectationWire(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	var absent *Session
	if _, _, err := absent.ReadExpectedControl(ctx); err == nil {
		t.Fatal("nil reader succeeded")
	}
	if err := client.SendPacket(ppp.ProtoIPCP, []byte{1, 2, 0}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ac.ReadExpectedControl(ctx); err == nil {
		t.Fatal("malformed control parsed")
	}
	expectationTransfer(t, ctx, ac, client, ppp.LCPConfigureRequest, 21, expectationOptions(11))
	expectationTransfer(t, ctx, client, ac, ppp.LCPConfigureRequest, 22, expectationOptions(21))
	expectationTransfer(t, ctx, client, ac, ppp.LCPConfigureAck, 21, expectationOptions(11))
	ac.peer.initialized = false
	err := ac.SendControl(ppp.ProtoLCP, ppp.LCPConfigureAck, 22, expectationOptions(21))
	ac.peer.initialized = true
	if err == nil {
		t.Fatal("uninitialized sender wrote Ack")
	}
	ac.echo.mu.Lock()
	opened := ac.echo.opened()
	ac.echo.mu.Unlock()
	if opened {
		t.Fatal("failed Ack established negotiation")
	}
	expectationTransfer(t, ctx, ac, client, ppp.LCPConfigureAck, 22, expectationOptions(21))
	expectationEchoBarrier(t, ctx, ac, client, 11, 21)
}

// TestExpectedEchoReplyWriteFailure reads Echo before bringing the AC link down.
// The same reply producer used by expectations must report the send's ENETDOWN.
func TestExpectedEchoReplyWriteFailure(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	ac, client := expectationWire(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	expectationNegotiate(t, ctx, ac, client, 11, 21)
	if err := client.SendControl(ppp.ProtoLCP, ppp.LCPEchoRequest, 45, []byte{0, 0, 0, 21}); err != nil {
		t.Fatal(err)
	}
	proto, packet, err := ac.ReadControl(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if proto != ppp.ProtoLCP {
		t.Fatalf("protocol = %x", proto)
	}
	if packet.Code != ppp.LCPEchoRequest {
		t.Fatalf("code = %d", packet.Code)
	}
	if len(packet.Data) != 4 {
		t.Fatalf("Echo length = %d", len(packet.Data))
	}
	link, err := netlink.LinkByName("zea0")
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetDown(link); err != nil {
		t.Fatal(err)
	}
	err = ac.replyEcho(packet)
	if !errors.Is(err, unix.ENETDOWN) {
		t.Fatalf("reply write failure = %v, want ENETDOWN", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	// Link-down also posts a receive-side socket error. Measure and clear that
	// separate notification before proving successful traffic after recovery.
	pending, err := unix.GetsockoptInt(ac.peer.fd, unix.SOL_SOCKET, unix.SO_ERROR)
	if err != nil {
		t.Fatal(err)
	}
	if pending != int(unix.ENETDOWN) {
		t.Fatalf("pending link notification = %d, want ENETDOWN", pending)
	}
	expectationEchoBarrier(t, ctx, ac, client, 11, 21)
}

// TestExpectedEchoMagic rejects wrong, zero and stale client Magic, while a
// positive packet barrier proves that each refusal emitted no Echo-Reply.
func TestExpectedEchoMagic(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	ac, client := expectationWire(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for _, epoch := range []struct{ local, peer uint32 }{{11, 21}, {12, 22}, {0, 0}} {
		expectationNegotiate(t, ctx, ac, client, epoch.local, epoch.peer)
		for _, wrong := range []uint32{99, 21, 0} {
			if wrong == epoch.peer {
				continue
			}
			var data [4]byte
			binary.BigEndian.PutUint32(data[:], wrong)
			if err := client.SendControl(ppp.ProtoLCP, ppp.LCPEchoRequest, 77, data[:]); err != nil {
				t.Fatal(err)
			}
			_, _, err := ac.ReadExpectedControl(ctx)
			if err == nil {
				t.Fatalf("accepted client Magic %d, negotiated %d", wrong, epoch.peer)
			}
			if !strings.Contains(err.Error(), "client Echo Magic-Number") {
				t.Fatalf("wrong failure: %v", err)
			}
			if err := ac.SendControl(ppp.ProtoIPCP, ppp.LCPConfigureAck, 78, nil); err != nil {
				t.Fatal(err)
			}
			proto, packet, err := client.ReadControl(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if proto != ppp.ProtoIPCP {
				t.Fatalf("invalid Magic drew a reply: %x %+v", proto, packet)
			}
			if packet.Identifier != 78 {
				t.Fatalf("barrier ID = %d", packet.Identifier)
			}
		}
		expectationEchoBarrier(t, ctx, ac, client, epoch.local, epoch.peer)
	}
}

// TestExpectedEchoRevisedOptions proves that Nak/Reject proposals are not the
// accepted value, and omitted Magic replaces the preceding epoch's value.
func TestExpectedEchoRevisedOptions(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	ac, client := expectationWire(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for _, refusal := range []uint8{ppp.LCPConfigureNak, ppp.LCPConfigureReject} {
		expectationNegotiate(t, ctx, ac, client, 11, 21)
		expectationTransfer(t, ctx, ac, client, ppp.LCPConfigureRequest, 31, expectationOptions(12))
		expectationTransfer(t, ctx, client, ac, ppp.LCPConfigureRequest, 32, expectationOptions(22))
		expectationTransfer(t, ctx, ac, client, refusal, 32, expectationOptions(22))
		peer := uint32(23)
		if refusal == ppp.LCPConfigureReject {
			peer = 0
		}
		expectationTransfer(t, ctx, client, ac, ppp.LCPConfigureRequest, 33, expectationOptions(peer))
		// A stale Ack and a changed-data Ack must not establish the peer value.
		expectationTransfer(t, ctx, ac, client, ppp.LCPConfigureAck, 32, expectationOptions(22))
		expectationTransfer(t, ctx, client, ac, ppp.LCPConfigureAck, 31, expectationOptions(12))
		ac.echo.mu.Lock()
		opened := ac.echo.opened()
		ac.echo.mu.Unlock()
		if opened {
			t.Fatal("stale Ack opened LCP")
		}
		expectationTransfer(t, ctx, ac, client, ppp.LCPConfigureAck, 33, expectationOptions(24))
		ac.echo.mu.Lock()
		opened = ac.echo.opened()
		ac.echo.mu.Unlock()
		if opened {
			t.Fatal("changed Ack opened LCP")
		}
		expectationTransfer(t, ctx, ac, client, ppp.LCPConfigureAck, 33, expectationOptions(peer))
		expectationEchoBarrier(t, ctx, ac, client, 12, peer)
	}
}

// expectationEchoBarrier checks exact Echo output before a known data packet.
func expectationEchoBarrier(t *testing.T, ctx context.Context, ac, client *Session, local, peer uint32) {
	t.Helper()
	data := []byte{0, 0, 0, 0, 0xff, 3}
	binary.BigEndian.PutUint32(data, peer)
	if err := client.SendControl(ppp.ProtoLCP, ppp.LCPEchoRequest, 98, data); err != nil {
		t.Fatal(err)
	}
	if err := client.SendPacket(ppp.ProtoIPv4, []byte{0x45, 1}); err != nil {
		t.Fatal(err)
	}
	proto, packet, err := ac.ReadExpectedPacket(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if proto != ppp.ProtoIPv4 {
		t.Fatalf("barrier protocol = %x", proto)
	}
	if !bytes.Equal(packet, []byte{0x45, 1}) {
		t.Fatalf("barrier payload = %x", packet)
	}
	proto, reply, err := client.ReadControl(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if proto != ppp.ProtoLCP {
		t.Fatalf("reply protocol = %x", proto)
	}
	if reply.Code != ppp.LCPEchoReply {
		t.Fatalf("reply code = %d", reply.Code)
	}
	if reply.Identifier != 98 {
		t.Fatalf("reply ID = %d", reply.Identifier)
	}
	binary.BigEndian.PutUint32(data, local)
	if !bytes.Equal(reply.Data, data) {
		t.Fatalf("reply data = %x, want %x", reply.Data, data)
	}
}

// TestExpectedEchoLiveDeadline drives continuous valid Echo until the original
// deadline. The peer worker owns the other socket and is canceled and joined.
func TestExpectedEchoLiveDeadline(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	ac, client := expectationWire(t)
	setup, stop := context.WithTimeout(t.Context(), 3*time.Second)
	defer stop()
	expectationNegotiate(t, setup, ac, client, 11, 21)
	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	done := make(chan int, 1)
	go expectationFlood(ctx, client, done)
	_, _, err := ac.ReadExpectedControl(ctx)
	cancel()
	count := <-done
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("live deadline = %v", err)
	}
	if count == 0 {
		t.Fatal("no Echo serviced before deadline")
	}
}

// expectationFlood is one bounded peer lifecycle. Its caller MUST cancel and join.
func expectationFlood(ctx context.Context, client *Session, done chan<- int) {
	count := 0
	defer func() { done <- count }()
	for ctx.Err() == nil {
		if err := client.SendControl(ppp.ProtoLCP, ppp.LCPEchoRequest, uint8(count), []byte{0, 0, 0, 21}); err != nil {
			return
		}
		proto, reply, err := client.ReadControl(ctx)
		if err != nil {
			return
		}
		if proto != ppp.ProtoLCP {
			return
		}
		if reply.Code != ppp.LCPEchoReply {
			return
		}
		if !bytes.Equal(reply.Data, []byte{0, 0, 0, 11}) {
			return
		}
		count++
	}
}
