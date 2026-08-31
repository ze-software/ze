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
// RFC requirement: RFC3948-2.3-2 positive -- "The sender MUST use a one-octet-long payload
// with the value 0xFF" (rfc/full/rfc3948.txt, Section 2.3). Keepalive.Run writes the single
// constant octet keepaliveByte = 0xFF (keepalive.go:14,48), and the peer socket reads exactly
// one byte whose value is 0xFF.
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

// VALIDATES: a NAT-keepalive that arrives on the NAT-T socket is dropped by the transport
// and never becomes a packet a session can read.
// PREVENTS: keepalive reception standing in for liveness. A keepalive leaves a peer on a
// timer and says nothing about whether that peer still holds the IKE SA, so a liveness
// check that counted one would hold a dead SA up until user traffic failed.
//
// RFC requirement: RFC3948-4-3 negative -- reception of NAT-keepalive packets is not used
// to detect whether a connection is live: Run refuses to deliver the datagram at all
// (udp.go, the 28-byte IKE header floor), so no liveness path in the engine can reach one.
func TestNATKeepaliveIsNeverDeliveredToASession(t *testing.T) {
	log := slogutil.DiscardLogger()
	tr, err := NewNATTTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("NewNATTTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })

	go tr.Run()

	local, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}
	sender, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })

	if _, err := sender.Write([]byte{0xFF}); err != nil {
		t.Fatalf("write keepalive: %v", err)
	}

	select {
	case pkt := <-tr.Recv():
		t.Fatalf("a NAT-keepalive of %d byte(s) was delivered as a session packet", len(pkt.Data))
	case <-time.After(100 * time.Millisecond):
	}
}
