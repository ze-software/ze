//go:build integration && linux

package network

import (
	"bufio"
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type acceptedTCPConn struct {
	conn *net.TCPConn
	err  error
}

// TestRealDialerOutTTL255PassesMinTTLListenerIPv4 proves the outgoing TTL is
// applied before connect: a listener requiring inbound TTL 255 accepts the SYN.
//
// VALIDATES: Peer-configured outgoing TTL reaches the kernel before TCP connect.
// PREVENTS: GTSM setting only post-connect TTL, leaving the SYN rejected by a peer using GTSM.
func TestRealDialerOutTTL255PassesMinTTLListenerIPv4(t *testing.T) {
	var lc net.ListenConfig
	lc.Control = func(_, _ string, c syscall.RawConn) error {
		var controlErr error
		if err := c.Control(func(fd uintptr) {
			controlErr = setIPMinTTL(int(fd), net.IPv4(127, 0, 0, 1), 255)
		}); err != nil {
			return err
		}
		return controlErr
	}

	ln, err := lc.Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer closeOrLog(t, ln)

	acceptCh := make(chan acceptedTCPConn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			acceptCh <- acceptedTCPConn{err: acceptErr}
			return
		}
		tcp, ok := conn.(*net.TCPConn)
		if !ok {
			_ = conn.Close()
			acceptCh <- acceptedTCPConn{err: errors.New("accepted connection is not TCP")}
			return
		}
		acceptCh <- acceptedTCPConn{conn: tcp}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, err := (&RealDialer{OutTTL: 255}).DialContext(ctx, "tcp4", ln.Addr().String())
	if err != nil {
		t.Fatalf("DialContext with OutTTL=255: %v", err)
	}
	defer closeOrLog(t, client)

	accepted := <-acceptCh
	if accepted.err != nil {
		t.Fatalf("Accept: %v", accepted.err)
	}
	defer closeOrLog(t, accepted.conn)

	clientTCP, ok := client.(*net.TCPConn)
	if !ok {
		t.Fatalf("dialed connection is %T, want *net.TCPConn", client)
	}
	if got := getsockoptInt(t, clientTCP, unix.IPPROTO_IP, unix.IP_TTL); got != 255 {
		t.Fatalf("IP_TTL = %d, want 255", got)
	}
}

// TestRealListenerFactoryListenTTLPassesPeerMinTTL proves the listen-socket TTL
// makes the kernel emit SYN-ACKs with TTL 255, so a GTSM peer that initiates the
// connection (its socket has IP_MINTTL=255) does not drop our SYN-ACK. Without
// ListenTTL the SYN-ACK carries the default TTL and connect times out -- the
// exact failure observed against FRR ttl-security on the passive path.
//
// VALIDATES: RealListenerFactory.ListenTTL sets the listen-socket outgoing TTL.
// PREVENTS: GTSM passive (accept) path failing because SYN-ACK carries default TTL.
func TestRealListenerFactoryListenTTLPassesPeerMinTTL(t *testing.T) {
	ln, err := RealListenerFactory{ListenTTL: 255}.Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer closeOrLog(t, ln)

	acceptCh := make(chan acceptedTCPConn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			acceptCh <- acceptedTCPConn{err: acceptErr}
			return
		}
		tcp, ok := conn.(*net.TCPConn)
		if !ok {
			closeOrLog(t, conn)
			acceptCh <- acceptedTCPConn{err: errors.New("accepted connection is not TCP")}
			return
		}
		acceptCh <- acceptedTCPConn{conn: tcp}
	}()

	// Client requires inbound TTL 255 (peer-side GTSM gate). The SYN-ACK is an
	// inbound segment on this socket, so a SYN-ACK below TTL 255 is dropped and
	// the connect times out -- which is what happens without the listener TTL.
	d := net.Dialer{Control: func(_, _ string, c syscall.RawConn) error {
		var ctrlErr error
		if err := c.Control(func(fd uintptr) {
			ctrlErr = setIPMinTTL(int(fd), net.IPv4(127, 0, 0, 1), 255)
		}); err != nil {
			return err
		}
		return ctrlErr
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, err := d.DialContext(ctx, "tcp4", ln.Addr().String())
	if err != nil {
		t.Fatalf("connect with peer IP_MINTTL=255 failed (listener SYN-ACK TTL too low?): %v", err)
	}
	defer closeOrLog(t, client)

	accepted := <-acceptCh
	if accepted.err != nil {
		t.Fatalf("Accept: %v", accepted.err)
	}
	defer closeOrLog(t, accepted.conn)

	// The accepted socket inherits the listen socket's outgoing TTL.
	if got := getsockoptInt(t, accepted.conn, unix.IPPROTO_IP, unix.IP_TTL); got != 255 {
		t.Fatalf("accepted socket IP_TTL = %d, want 255 (inherited from listener)", got)
	}
}

// TestSetIPMinTTLRejectsLowTTLIPv4 proves the Linux IP_MINTTL option drops a
// packet whose TTL is below the configured minimum.
//
// VALIDATES: IP_MINTTL=255 drops data sent with IP_TTL=254.
// PREVENTS: GTSM inbound enforcement being a no-op despite successful readback.
func TestSetIPMinTTLRejectsLowTTLIPv4(t *testing.T) {
	server, client := tcpPair(t, "tcp4", "127.0.0.1:0")
	defer closeOrLog(t, server)
	defer closeOrLog(t, client)

	controlSocket(t, server, func(fd int) error {
		return setIPMinTTL(fd, net.IPv4(127, 0, 0, 1), 255)
	})
	controlSocket(t, client, func(fd int) error {
		return setIPTTL(fd, net.IPv4(127, 0, 0, 1), 254)
	})

	if _, err := client.Write([]byte{0x42}); err != nil {
		t.Fatalf("client write with low TTL: %v", err)
	}
	if err := server.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	buf := []byte{0}
	_, err := server.Read(buf)
	if err == nil {
		t.Fatal("server read succeeded; low-TTL packet was not dropped")
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("server read error = %v, want timeout from dropped low-TTL packet", err)
	}
}

// TestSetIPv6TTLAndMinHopReadback verifies IPv6 hop-limit helpers use the IPv6
// socket options rather than IPv4 TTL options.
//
// VALIDATES: IPV6_UNICAST_HOPS and IPV6_MINHOPCOUNT are set for IPv6 peers.
// PREVENTS: IPv6 GTSM config silently applying only IPv4 socket options.
func TestSetIPv6TTLAndMinHopReadback(t *testing.T) {
	server, client := tcpPair(t, "tcp6", "[::1]:0")
	defer closeOrLog(t, server)
	defer closeOrLog(t, client)

	controlSocket(t, client, func(fd int) error {
		return setIPTTL(fd, net.ParseIP("::1"), 255)
	})
	controlSocket(t, server, func(fd int) error {
		return setIPMinTTL(fd, net.ParseIP("::1"), 255)
	})

	if got := getsockoptInt(t, client, unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS); got != 255 {
		t.Fatalf("IPV6_UNICAST_HOPS = %d, want 255", got)
	}
	if got := getsockoptInt(t, server, unix.IPPROTO_IPV6, unix.IPV6_MINHOPCOUNT); got != 255 {
		t.Fatalf("IPV6_MINHOPCOUNT = %d, want 255", got)
	}
}

// TestSetIPv6MinHopCountEnforcesTheFloor proves the kernel ACTS on the IPv6
// minimum hop count ze installs, rather than merely storing it. The readback
// test above shows getsockopt answers 255, and a socket option the kernel never
// consulted would answer exactly the same, so the readback alone cannot tell a
// working floor from a dead one.
//
// The method is one socket pair and two segments that differ only in their hop
// limit. The segment at 255 is delivered and moves no counter. The segment at
// 254 is discarded before delivery and counted in TCPMinTTLDrop, which
// tcp_v6_rcv increments on that one comparison. The delivered segment is the
// positive control: without it, a read timeout would read the same for a
// working floor and for a path that carries nothing at all.
//
// VALIDATES: IPV6_MINHOPCOUNT=255 discards a segment sent at hop limit 254.
// PREVENTS: IPv6 GTSM inbound enforcement being a no-op behind a successful readback.
func TestSetIPv6MinHopCountEnforcesTheFloor(t *testing.T) {
	server, client := tcpPair(t, "tcp6", "[::1]:0")
	defer closeOrLog(t, server)
	defer closeOrLog(t, client)

	controlSocket(t, server, func(fd int) error {
		return setIPMinTTL(fd, net.ParseIP("::1"), 255)
	})

	// The positive control: at the floor the segment arrives.
	controlSocket(t, client, func(fd int) error {
		return setIPTTL(fd, net.ParseIP("::1"), 255)
	})
	atFloor := tcpMinTTLDrop(t)
	if _, err := client.Write([]byte{0x41}); err != nil {
		t.Fatalf("client write at hop limit 255: %v", err)
	}
	if err := server.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	buf := []byte{0}
	if _, err := server.Read(buf); err != nil {
		t.Fatalf("server read at hop limit 255: %v", err)
	}
	if buf[0] != 0x41 {
		t.Fatalf("server read 0x%02x, want 0x41", buf[0])
	}
	if got := tcpMinTTLDrop(t); got != atFloor {
		t.Fatalf("TCPMinTTLDrop went from %d to %d, want no change: a segment at the floor is not below it", atFloor, got)
	}

	// One below the floor, on the same pair.
	controlSocket(t, client, func(fd int) error {
		return setIPTTL(fd, net.ParseIP("::1"), 254)
	})
	belowFloor := tcpMinTTLDrop(t)
	if _, err := client.Write([]byte{0x42}); err != nil {
		t.Fatalf("client write at hop limit 254: %v", err)
	}
	if err := server.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	_, err := server.Read(buf)
	if err == nil {
		t.Fatalf("server read 0x%02x; the segment below the floor was delivered", buf[0])
	}
	var netErr net.Error
	if !errors.As(err, &netErr) {
		t.Fatalf("server read error = %v, want a net.Error timeout from the discarded segment", err)
	}
	if !netErr.Timeout() {
		t.Fatalf("server read error = %v, want a timeout from the discarded segment", err)
	}

	// A floor, not an exact count: TCP retransmits the segment the kernel threw
	// away, and each retransmission is refused on the same comparison, so the
	// number the kernel reaches depends on how many retransmission timers fired
	// inside the read deadline.
	after := tcpMinTTLDrop(t)
	if after <= belowFloor {
		t.Fatalf("TCPMinTTLDrop stayed at %d, want an increase: the segment was lost for some reason other than the hop limit floor", belowFloor)
	}
}

// tcpMinTTLDrop reads the counter the kernel increments when it refuses a
// segment on the socket's minimum TTL or hop limit. It is the one observable
// that names the comparison, so it separates an enforced floor from a segment
// that went missing for any other reason.
func tcpMinTTLDrop(t *testing.T) uint64 {
	t.Helper()

	file, err := os.Open("/proc/net/netstat")
	if err != nil {
		t.Fatalf("open /proc/net/netstat: %v", err)
	}
	defer closeOrLog(t, file)

	scanner := bufio.NewScanner(file)
	var names []string
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 || fields[0] != "TcpExt:" {
			continue
		}
		if names == nil {
			names = fields
			continue
		}
		for i, name := range names {
			if name != "TCPMinTTLDrop" || i >= len(fields) {
				continue
			}
			value, parseErr := strconv.ParseUint(fields[i], 10, 64)
			if parseErr != nil {
				t.Fatalf("parse TcpExt TCPMinTTLDrop %q: %v", fields[i], parseErr)
			}
			return value
		}
		names = nil
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read /proc/net/netstat: %v", err)
	}
	t.Fatal("this kernel reports no TCPMinTTLDrop counter, so the hop limit floor cannot be observed")
	return 0
}

func tcpPair(t *testing.T, networkName, address string) (*net.TCPConn, *net.TCPConn) {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), networkName, address)
	if err != nil {
		t.Skipf("listen %s %s: %v", networkName, address, err)
	}
	defer closeOrLog(t, ln)

	acceptCh := make(chan acceptedTCPConn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			acceptCh <- acceptedTCPConn{err: acceptErr}
			return
		}
		tcp, ok := conn.(*net.TCPConn)
		if !ok {
			_ = conn.Close()
			acceptCh <- acceptedTCPConn{err: errors.New("accepted connection is not TCP")}
			return
		}
		acceptCh <- acceptedTCPConn{conn: tcp}
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

func controlSocket(t *testing.T, conn *net.TCPConn, fn func(fd int) error) {
	t.Helper()
	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	var controlErr error
	if err := raw.Control(func(fd uintptr) {
		controlErr = fn(int(fd))
	}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	if controlErr != nil {
		t.Fatalf("socket option: %v", controlErr)
	}
}

func getsockoptInt(t *testing.T, conn *net.TCPConn, level, opt int) int {
	t.Helper()
	var value int
	controlSocket(t, conn, func(fd int) error {
		got, err := unix.GetsockoptInt(fd, level, opt)
		value = got
		return err
	})
	return value
}
