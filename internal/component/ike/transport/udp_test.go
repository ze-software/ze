package transport

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/probe"
)

func TestUDPTransportSendReceive(t *testing.T) {
	tr, err := NewUDPTransport("127.0.0.1:0", slog.Default())
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })

	go tr.Run()

	localAddr, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}

	sender, err := net.DialUDP("udp4", nil, localAddr)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })

	msg := make([]byte, 28)
	msg[0] = 0xaa
	msg[17] = 0x20

	if _, err := sender.Write(msg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	select {
	case pkt := <-tr.Recv():
		if len(pkt.Data) != 28 {
			t.Fatalf("expected 28 bytes, got %d", len(pkt.Data))
		}
		if pkt.Data[0] != 0xaa {
			t.Fatalf("expected first byte 0xaa, got 0x%02x", pkt.Data[0])
		}
		if pkt.RemoteAddr == nil {
			t.Fatal("RemoteAddr should not be nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for packet")
	}
}

func TestUDPTransportSend(t *testing.T) {
	recvConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	t.Cleanup(func() { _ = recvConn.Close() })
	recvAddr, ok := recvConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}

	tr, err := NewUDPTransport("127.0.0.1:0", slog.Default())
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })

	msg := make([]byte, 28)
	msg[0] = 0xbb
	if err := tr.Send(msg, recvAddr); err != nil {
		t.Fatalf("Send: %v", err)
	}

	buf := make([]byte, MaxMsgSize)
	if err := recvConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	n, _, err := recvConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("ReadFromUDP: %v", err)
	}
	if n != 28 {
		t.Fatalf("expected 28 bytes, got %d", n)
	}
	if buf[0] != 0xbb {
		t.Fatalf("expected first byte 0xbb, got 0x%02x", buf[0])
	}
}

func TestUDPTransportDropsShortPackets(t *testing.T) {
	tr, err := NewUDPTransport("127.0.0.1:0", slog.Default())
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })

	go tr.Run()

	localAddr, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}
	sender, err := net.DialUDP("udp4", nil, localAddr)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })

	short := make([]byte, 10)
	if _, err := sender.Write(short); err != nil {
		t.Fatalf("Write: %v", err)
	}

	select {
	case <-tr.Recv():
		t.Fatal("should not receive short packets")
	case <-time.After(100 * time.Millisecond):
	}
}

// TestQueuedSizeRefusalReachesTheEventChannel proves what the read loop
// hands the engine: an EMSGSIZE entry becomes one SizeRefusal carrying the
// peer the datagram was sent to, the reported MTU under its outcome, the
// offender and the local flag, stamped with the socket's role; an entry
// that is not a size refusal is dropped; and a full channel drops the
// newest entry rather than blocking the loop.
func TestQueuedSizeRefusalReachesTheEventChannel(t *testing.T) {
	tr, err := NewNATTTransport("127.0.0.1:0", slog.Default())
	if err != nil {
		t.Fatalf("NewNATTTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })

	peer := netip.MustParseAddrPort("192.0.2.9:4500")
	router := netip.MustParseAddr("192.0.2.1")
	tr.deliverQueuedError(probe.QueuedError{
		Outcome: probe.ErrQueueMTUReported, MTU: 1400, Errno: syscall.EMSGSIZE,
		Offender: router, Dest: peer,
	})
	tr.deliverQueuedError(probe.QueuedError{
		Outcome: probe.ErrQueueMTUUnreported, Errno: syscall.ECONNREFUSED,
		Offender: router, Dest: peer,
	})
	tr.deliverQueuedError(probe.QueuedError{
		Outcome: probe.ErrQueueMTUReported, MTU: 1300, Errno: syscall.EMSGSIZE, Local: true, Dest: peer,
	})

	want := []SizeRefusal{
		{Peer: peer, Outcome: probe.ErrQueueMTUReported, MTU: 1400, Offender: router, NATT: true},
		{Peer: peer, Outcome: probe.ErrQueueMTUReported, MTU: 1300, Local: true, NATT: true},
	}
	for i, w := range want {
		select {
		case got := <-tr.Refusals():
			if got != w {
				t.Errorf("refusal %d = %+v, want %+v", i, got, w)
			}
		default:
			t.Fatalf("refusal %d never reached the channel", i)
		}
	}
	select {
	case got := <-tr.Refusals():
		t.Fatalf("an entry that is not a size refusal reached the channel: %+v", got)
	default:
	}

	for range refusalQueueDepth + 1 {
		tr.deliverQueuedError(probe.QueuedError{
			Outcome: probe.ErrQueueMTUReported, MTU: 1400, Errno: syscall.EMSGSIZE, Dest: peer,
		})
	}
	if got := len(tr.Refusals()); got != refusalQueueDepth {
		t.Errorf("channel holds %d refusals after an overflow, want the declared depth %d", got, refusalQueueDepth)
	}
}

// TestUDPTransportSendFromBoundSource proves a concrete-bound socket sends
// from the requested endpoint on every platform, rejects a different local
// address without sending, and refuses writes after Close.
func TestUDPTransportSendFromBoundSource(t *testing.T) {
	tr, err := NewUDPTransport("127.0.0.1:0", slog.Default())
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	local, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}
	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	remote, ok := peer.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer address is not *net.UDPAddr")
	}
	wrong := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: local.Port}
	if err := tr.SendFrom(prtIKEDatagram(0xad), wrong, remote); !errors.Is(err, ErrSendFailed) {
		t.Fatalf("SendFrom with a different bound address = %v, want ErrSendFailed", err)
	}
	want := prtIKEDatagram(0xa9)
	if err := tr.SendFrom(want, local, remote); err != nil {
		t.Fatalf("SendFrom bound source: %v", err)
	}
	if err := peer.SetReadDeadline(time.Now().Add(prtArrive)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var response [MaxMsgSize]byte
	n, source, err := peer.ReadFromUDP(response[:])
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !bytes.Equal(response[:n], want) {
		t.Fatalf("response = %x, want only valid send %x", response[:n], want)
	}
	if !source.IP.Equal(local.IP) || source.Port != local.Port || source.Zone != local.Zone {
		t.Fatalf("source = %v, want %v", source, local)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := tr.SendFrom(want, local, remote); !errors.Is(err, ErrClosed) {
		t.Fatalf("SendFrom after Close = %v, want ErrClosed", err)
	}
}
