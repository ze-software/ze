package transport

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

// VALIDATES: a UDP transport bound to an ephemeral localhost port
// receives a BFD-shaped packet through the real kernel UDP stack.
// PREVENTS: regression where the receive goroutine drops the packet,
// the pool slot leaks, or the address round-trip loses the peer IP.
func TestUDPLoopback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping UDP loopback test in short mode")
	}

	rx := &UDP{
		Bind: netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0),
		Mode: api.SingleHop,
	}
	if err := rx.Start(); err != nil {
		t.Fatalf("rx.Start: %v", err)
	}
	defer func() {
		if err := rx.Stop(); err != nil {
			t.Errorf("rx.Stop: %v", err)
		}
	}()

	// Read back the actually-assigned ephemeral port.
	rxLocal, ok := rx.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("rx.conn.LocalAddr is %T, want *net.UDPAddr", rx.conn.LocalAddr())
	}

	// Independent sender: dial from a separate socket so we test the
	// receive path end-to-end through the real kernel stack.
	sender, err := net.DialUDP("udp", nil, rxLocal)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer func() { _ = sender.Close() }()

	msg := []byte{
		0x20, 0xC0, 0x03, 0x18, // ver=1 diag=0 state=Up poll=1 mult=3 len=24
		0x00, 0x00, 0x00, 0x01, // MyDisc
		0x00, 0x00, 0x00, 0x00, // YourDisc
		0x00, 0x0F, 0x42, 0x40, // DesiredMinTx 1s
		0x00, 0x0F, 0x42, 0x40, // RequiredMinRx 1s
		0x00, 0x00, 0x00, 0x00, // RequiredMinEchoRx
	}
	if _, err := sender.Write(msg); err != nil {
		t.Fatalf("sender.Write: %v", err)
	}

	select {
	case in, ok := <-rx.RX():
		if !ok {
			t.Fatalf("rx.RX closed")
		}
		defer in.Release()
		if len(in.Bytes) != len(msg) {
			t.Fatalf("bytes length: got %d want %d", len(in.Bytes), len(msg))
		}
		for i := range msg {
			if in.Bytes[i] != msg[i] {
				t.Fatalf("byte %d: got %#x want %#x", i, in.Bytes[i], msg[i])
			}
		}
		if !in.From.IsLoopback() {
			t.Fatalf("From is not loopback: %v", in.From)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive packet within 2s")
	}
}

// VALIDATES: Send's off-Linux source warning fires only for a packet whose
// session names a local address the platform could not set. The decision is a
// pure function, so it runs on every build: a nil control message is what
// pktinfoPin returns off Linux.
// PREVENTS: a single-hop packet pinned only by interface, with no local
// address, logging that its source address was lost, with local "invalid IP".
func TestSourceUnpinned(t *testing.T) {
	local := netip.MustParseAddr("192.0.2.1")
	built := []byte{0}
	cases := []struct {
		name string
		pin  sendPin
		oob  []byte
		want bool
	}{
		{"interface only, no control message", sendPin{ifindex: 3}, nil, false},
		{"source, no control message", sendPin{source: local}, nil, true},
		{"interface and source, no control message", sendPin{ifindex: 3, source: local}, nil, true},
		{"source, control message built", sendPin{source: local}, built, false},
		{"interface only, control message built", sendPin{ifindex: 3}, built, false},
	}
	for _, tc := range cases {
		if got := sourceUnpinned(tc.pin, tc.oob); got != tc.want {
			t.Errorf("%s: sourceUnpinned = %v, want %v", tc.name, got, tc.want)
		}
	}
}
