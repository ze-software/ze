//go:build integration && linux

package reactor

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type acceptedSessionTCPConn struct {
	conn *net.TCPConn
	err  error
}

// TestSessionMinTTLSetOnSocket verifies the established-session socket seam
// applies inbound GTSM filtering to the connected TCP socket.
//
// VALIDATES: connectionEstablished's socket tuning path sets IP_MINTTL from PeerSettings.MinTTL.
// PREVENTS: Parsed GTSM min TTL never being applied to the live BGP TCP socket.
func TestSessionMinTTLSetOnSocket(t *testing.T) {
	server, client := sessionTCPPair(t, "tcp4", "127.0.0.1:0")
	defer closeTCPForTest(t, server)
	defer closeTCPForTest(t, client)

	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65001, 65002, 0x01020301)
	settings.MinTTL = 255
	session := NewSession(settings)
	if err := session.tuneTCPConnection(server); err != nil {
		t.Fatalf("tuneTCPConnection: %v", err)
	}

	if got := sessionGetsockoptInt(t, server, unix.IPPROTO_IP, unix.IP_MINTTL); got != 255 {
		t.Fatalf("IP_MINTTL = %d, want 255", got)
	}
}

// TestSessionOutTTLSetOnAcceptedSocket verifies passive peers set outgoing TTL
// on accepted sockets before sending OPEN, KEEPALIVE, or UPDATE messages.
//
// VALIDATES: connectionEstablished's socket tuning path sets IP_TTL from PeerSettings.OutTTL.
// PREVENTS: Passive GTSM sessions sending BGP packets with the OS default TTL.
func TestSessionOutTTLSetOnAcceptedSocket(t *testing.T) {
	server, client := sessionTCPPair(t, "tcp4", "127.0.0.1:0")
	defer closeTCPForTest(t, server)
	defer closeTCPForTest(t, client)

	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65001, 65002, 0x01020301)
	settings.OutTTL = 255
	session := NewSession(settings)
	if err := session.tuneTCPConnection(server); err != nil {
		t.Fatalf("tuneTCPConnection: %v", err)
	}

	if got := sessionGetsockoptInt(t, server, unix.IPPROTO_IP, unix.IP_TTL); got != 255 {
		t.Fatalf("IP_TTL = %d, want 255", got)
	}
}

// TestSessionMinTTLSetOnIPv6Socket verifies the same session seam for IPv6.
//
// VALIDATES: connectionEstablished's socket tuning path sets IPV6_MINHOPCOUNT for IPv6 peers.
// PREVENTS: IPv6 GTSM configuration silently using IPv4-only socket options.
func TestSessionMinTTLSetOnIPv6Socket(t *testing.T) {
	server, client := sessionTCPPair(t, "tcp6", "[::1]:0")
	defer closeTCPForTest(t, server)
	defer closeTCPForTest(t, client)

	settings := NewPeerSettings(netip.MustParseAddr("::1"), 65001, 65002, 0x01020301)
	settings.MinTTL = 255
	session := NewSession(settings)
	if err := session.tuneTCPConnection(server); err != nil {
		t.Fatalf("tuneTCPConnection: %v", err)
	}

	if got := sessionGetsockoptInt(t, server, unix.IPPROTO_IPV6, unix.IPV6_MINHOPCOUNT); got != 255 {
		t.Fatalf("IPV6_MINHOPCOUNT = %d, want 255", got)
	}
}

// TestSessionOutTTLSetOnAcceptedIPv6Socket verifies passive IPv6 peers set the
// outgoing Hop Limit on accepted sockets.
//
// VALIDATES: connectionEstablished's socket tuning path sets IPV6_UNICAST_HOPS from PeerSettings.OutTTL.
// PREVENTS: IPv6 passive GTSM sessions sending BGP packets with the OS default Hop Limit.
func TestSessionOutTTLSetOnAcceptedIPv6Socket(t *testing.T) {
	server, client := sessionTCPPair(t, "tcp6", "[::1]:0")
	defer closeTCPForTest(t, server)
	defer closeTCPForTest(t, client)

	settings := NewPeerSettings(netip.MustParseAddr("::1"), 65001, 65002, 0x01020301)
	settings.OutTTL = 255
	session := NewSession(settings)
	if err := session.tuneTCPConnection(server); err != nil {
		t.Fatalf("tuneTCPConnection: %v", err)
	}

	if got := sessionGetsockoptInt(t, server, unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS); got != 255 {
		t.Fatalf("IPV6_UNICAST_HOPS = %d, want 255", got)
	}
}

func sessionTCPPair(t *testing.T, networkName, address string) (*net.TCPConn, *net.TCPConn) {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), networkName, address)
	if err != nil {
		t.Skipf("listen %s %s: %v", networkName, address, err)
	}
	defer closeTCPForTest(t, ln)

	acceptCh := make(chan acceptedSessionTCPConn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			acceptCh <- acceptedSessionTCPConn{err: acceptErr}
			return
		}
		tcp, ok := conn.(*net.TCPConn)
		if !ok {
			_ = conn.Close()
			acceptCh <- acceptedSessionTCPConn{err: errors.New("accepted connection is not TCP")}
			return
		}
		acceptCh <- acceptedSessionTCPConn{conn: tcp}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, err := (&net.Dialer{}).DialContext(ctx, networkName, ln.Addr().String())
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	clientTCP, ok := client.(*net.TCPConn)
	if !ok {
		_ = client.Close()
		t.Fatalf("dialed connection is %T, want *net.TCPConn", client)
	}

	accepted := <-acceptCh
	if accepted.err != nil {
		_ = clientTCP.Close()
		t.Fatalf("Accept: %v", accepted.err)
	}
	return accepted.conn, clientTCP
}

func sessionGetsockoptInt(t *testing.T, conn *net.TCPConn, level, opt int) int {
	t.Helper()
	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	var value int
	var controlErr error
	if err := raw.Control(func(fd uintptr) {
		value, controlErr = unix.GetsockoptInt(int(fd), level, opt)
	}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	if controlErr != nil {
		t.Fatalf("GetsockoptInt: %v", controlErr)
	}
	return value
}

func closeTCPForTest(t *testing.T, c interface{ Close() error }) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Logf("close: %v", err)
	}
}
