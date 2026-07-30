package transport

import (
	"log/slog"
	"net"
	"testing"
	"time"
)

// prtArrive bounds a datagram that really crosses loopback. Delivery is immediate,
// so a generous deadline never lengthens the wait on a fast host.
const prtArrive = 5 * time.Second

// prtOddPorts are source ports that are neither 500 nor 4500. The test binds the
// first one that is free, so a busy host does not turn into a failure.
var prtOddPorts = []int{33333, 33334, 33335, 33336, 33337, 34401, 34402, 34403}

// prtSenderOnOddPort binds a sender socket to a port outside 500 and 4500.
func prtSenderOnOddPort(t *testing.T, dst *net.UDPAddr) (*net.UDPConn, int) {
	t.Helper()
	for _, port := range prtOddPorts {
		src := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}
		conn, err := net.DialUDP("udp4", src, dst)
		if err != nil {
			continue
		}
		t.Cleanup(func() { _ = conn.Close() })
		return conn, port
	}
	t.Fatalf("no port in %v was free for the sender", prtOddPorts)
	return nil, 0
}

// prtIKEDatagram returns a datagram long enough to clear the reader's size floor.
func prtIKEDatagram(marker byte) []byte {
	msg := make([]byte, 28)
	msg[0] = marker
	msg[17] = 0x20 // major version 2 in the upper nibble
	return msg
}

// VALIDATES: the reader delivers a request whatever source port it came from.
// RFC requirement: RFC7296-2.11-1 positive -- UDPTransport.Run (udp.go:89) reads every
// datagram and never consults the source port. A request from port 33333 arrives
// with that port preserved on the packet.
// RFC requirement: RFC7296-2.11-1 negative -- the reader is selective. It drops a datagram
// under 28 bytes at udp.go:103, and it drops that datagram from the very same
// source port. Delivery is therefore a decision, and the port is not part of it.
func TestPrtAcceptsDatagramFromAnySourcePort(t *testing.T) {
	tr, err := NewUDPTransport("127.0.0.1:0", slog.Default())
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	go tr.Run()

	dst, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}
	sender, port := prtSenderOnOddPort(t, dst)
	if port == IKEPort || port == 4500 {
		t.Fatalf("the sender bound port %d, which is a well-known IKE port", port)
	}

	// Negative first. A short datagram from this port is dropped, so the reader
	// does apply a rule. One socket keeps send order across loopback, so the full
	// datagram below arrives first only when the short one was discarded.
	if _, err := sender.Write(make([]byte, 10)); err != nil {
		t.Fatalf("write the short datagram: %v", err)
	}

	// Positive. A full datagram from the same port is delivered.
	want := prtIKEDatagram(0xa7)
	if _, err := sender.Write(want); err != nil {
		t.Fatalf("write the full datagram: %v", err)
	}

	select {
	case pkt := <-tr.Recv():
		if len(pkt.Data) != len(want) || pkt.Data[0] != want[0] {
			t.Fatalf("the reader delivered the short datagram, %d bytes", len(pkt.Data))
		}
		if pkt.RemoteAddr == nil {
			t.Fatal("the delivered packet carries no source address")
		}
		if pkt.RemoteAddr.Port != port {
			t.Errorf("delivered source port = %d, want %d", pkt.RemoteAddr.Port, port)
		}
	case <-time.After(prtArrive):
		t.Fatalf("no datagram from source port %d reached the reader", port)
	}
}
