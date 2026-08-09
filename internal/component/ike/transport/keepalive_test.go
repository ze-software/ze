// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT keepalive tests

package transport

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// RFC requirement: RFC3948-4-1 positive -- Run emits a 1-byte 0xFF NAT-keepalive on the wire
// at the configured interval (keepalive.go:43-58), the mechanism that refreshes the NAT UDP
// binding before it can expire; the server side reads exactly one 0xFF byte.
func TestNATKeepalive(t *testing.T) {
	// Create a UDP pair for testing.
	serverAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serverConn, err := net.ListenUDP("udp4", serverAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = serverConn.Close() }()

	clientAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clientConn, err := net.ListenUDP("udp4", clientAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = clientConn.Close() }()

	remote, ok := serverConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}
	log := slogutil.DiscardLogger()

	ka := NewKeepalive(clientConn, remote, 50*time.Millisecond, log)
	go ka.Run()

	// Read a keepalive packet from the server side.
	if err := serverConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	n, _, readErr := serverConn.ReadFromUDP(buf)
	if readErr != nil {
		t.Fatalf("read keepalive: %v", readErr)
	}
	if n != 1 || buf[0] != 0xFF {
		t.Fatalf("keepalive: got %d bytes %x, want 1 byte 0xFF", n, buf[:n])
	}

	ka.Stop()
}

// RFC requirement: RFC3948-4-1 positive -- the default NAT-keepalive interval is a small,
// positive, conservative constant (<= 20s, keepalive.go:13), well under a typical NAT UDP
// binding lifetime, so keepalives refresh the mapping before it can expire.
func TestKeepaliveDefaultInterval(t *testing.T) {
	if DefaultKeepaliveInterval <= 0 {
		t.Fatalf("DefaultKeepaliveInterval = %v, want a positive interval", DefaultKeepaliveInterval)
	}
	if DefaultKeepaliveInterval > 20*time.Second {
		t.Errorf("DefaultKeepaliveInterval = %v, want <= 20s (shorter than a typical NAT UDP binding timeout)", DefaultKeepaliveInterval)
	}
}
