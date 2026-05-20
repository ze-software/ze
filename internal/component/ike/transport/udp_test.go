package transport

import (
	"log/slog"
	"net"
	"testing"
	"time"
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
