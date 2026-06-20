package flowexport

import (
	"context"
	"net"
	"testing"
)

func TestBufferPoolGetPut(t *testing.T) {
	b := GetBuf()
	if b == nil {
		t.Fatal("GetBuf returned nil")
	}
	if len(*b) != MaxDatagramSize {
		t.Fatalf("buffer size = %d, want %d", len(*b), MaxDatagramSize)
	}

	// Write some data, put back, get again: pool reuses.
	(*b)[0] = 0xFF
	PutBuf(b)

	b2 := GetBuf()
	if b2 == nil {
		t.Fatal("second GetBuf returned nil")
	}
	if len(*b2) != MaxDatagramSize {
		t.Fatalf("second buffer size = %d, want %d", len(*b2), MaxDatagramSize)
	}
	PutBuf(b2)
}

func TestPutBufNil(t *testing.T) {
	PutBuf(nil) // must not panic
}

func TestSenderUDP(t *testing.T) {
	// Listen on a random loopback port.
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pc.Close() }()

	addr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}

	s, err := NewSender("127.0.0.1", addr.Port)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	payload := []byte{0x01, 0x02, 0x03, 0x04}
	if err := s.Send(payload); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 64)
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(payload) {
		t.Fatalf("received %d bytes, want %d", n, len(payload))
	}
	for i := range payload {
		if buf[i] != payload[i] {
			t.Fatalf("byte %d: got %02x, want %02x", i, buf[i], payload[i])
		}
	}

	datagrams, bytes, errors := s.Stats()
	if datagrams != 1 {
		t.Fatalf("datagrams = %d, want 1", datagrams)
	}
	if bytes != 4 {
		t.Fatalf("bytes = %d, want 4", bytes)
	}
	if errors != 0 {
		t.Fatalf("errors = %d, want 0", errors)
	}
}
