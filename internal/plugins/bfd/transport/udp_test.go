package transport

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
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
