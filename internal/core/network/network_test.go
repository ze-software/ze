package network

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestRealDialer verifies RealDialer connects to a real listener.
//
// VALIDATES: RealDialer.DialContext establishes a TCP connection.
// PREVENTS: Broken delegation that fails to connect.
func TestRealDialer(t *testing.T) {
	// Start a local listener using ListenConfig (linter-compliant)
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer closeOrLog(t, ln)

	// Accept in background
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return // Listener closed during test teardown
		}
		accepted <- conn
	}()

	// Dial using RealDialer
	d := &RealDialer{}
	conn, err := d.DialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("DialContext failed: %v", err)
	}
	defer closeOrLog(t, conn)

	// Verify connection was accepted
	serverConn := <-accepted
	defer closeOrLog(t, serverConn)

	if conn.RemoteAddr().String() != ln.Addr().String() {
		t.Errorf("remote addr = %s, want %s", conn.RemoteAddr(), ln.Addr())
	}
}

// TestRealDialerWithLocalAddr verifies RealDialer binds to local address.
//
// VALIDATES: LocalAddr field is used for source address binding.
// PREVENTS: LocalAddr being ignored.
func TestRealDialerWithLocalAddr(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer closeOrLog(t, ln)

	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return // Listener closed during test teardown
		}
		if err := conn.Close(); err != nil {
			// Best effort in goroutine
			return
		}
	}()

	d := &RealDialer{
		LocalAddr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)},
	}
	conn, err := d.DialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("DialContext with LocalAddr failed: %v", err)
	}
	defer closeOrLog(t, conn)

	localAddr, ok := conn.LocalAddr().(*net.TCPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.TCPAddr")
	}
	if !localAddr.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Errorf("local IP = %s, want 127.0.0.1", localAddr.IP)
	}
}

// TestRealListenerFactory verifies RealListenerFactory creates a real listener.
//
// VALIDATES: RealListenerFactory.Listen creates a working TCP listener.
// PREVENTS: Broken delegation to net.ListenConfig.
func TestRealListenerFactory(t *testing.T) {
	f := RealListenerFactory{}
	ln, err := f.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer closeOrLog(t, ln)

	addr := ln.Addr().String()
	if addr == "" {
		t.Error("listener address is empty")
	}

	// Verify we can connect to it
	var nd net.Dialer
	conn, err := nd.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("failed to connect to listener: %v", err)
	}
	closeOrLog(t, conn)
}

// TestDialerInterfaceSatisfied verifies RealDialer implements Dialer.
//
// VALIDATES: Compile-time interface conformance.
// PREVENTS: Missing methods on RealDialer.
func TestDialerInterfaceSatisfied(t *testing.T) {
	var _ Dialer = &RealDialer{}
}

// TestListenerFactoryInterfaceSatisfied verifies RealListenerFactory implements ListenerFactory.
//
// VALIDATES: Compile-time interface conformance.
// PREVENTS: Missing methods on RealListenerFactory.
func TestListenerFactoryInterfaceSatisfied(t *testing.T) {
	var _ ListenerFactory = RealListenerFactory{}
	var _ ListenerFactory = &RealListenerFactory{}
}

// TestTCPMD5SupportedReturnsValue verifies TCPMD5Supported returns a boolean.
//
// VALIDATES: TCPMD5Supported reports platform capability.
// PREVENTS: Missing platform-specific implementation.
func TestTCPMD5SupportedReturnsValue(t *testing.T) {
	// On macOS: false, on Linux/FreeBSD: true, on other: false
	got := TCPMD5Supported()
	t.Logf("TCPMD5Supported() = %v", got)
}

// TestSetTCPMD5SigPlatform verifies setTCPMD5Sig behavior on the current platform.
//
// VALIDATES: Platform-specific TCP MD5 function exists and returns expected error.
// PREVENTS: Missing build tag or function signature.
func TestSetTCPMD5SigPlatform(t *testing.T) {
	err := setTCPMD5Sig(0, net.IPv4(192, 0, 2, 1), "secret")
	if TCPMD5Supported() {
		// On Linux/FreeBSD: expect syscall error (bad fd), not "unsupported"
		if err == nil {
			t.Fatal("expected error for fd=0, got nil")
			return
		}
		t.Logf("supported platform error (expected): %v", err)
	} else {
		// On macOS/other: expect "not supported" error
		if err == nil {
			t.Fatal("expected unsupported error, got nil")
			return
		}
		t.Logf("unsupported platform error (expected): %v", err)
	}
}

// TestRealDialerMD5FieldsZeroValue verifies that MD5 fields default to zero values.
//
// VALIDATES: RealDialer with no MD5 config works identically to before.
// PREVENTS: MD5 fields breaking existing dialer behavior.
func TestRealDialerMD5FieldsZeroValue(t *testing.T) {
	d := &RealDialer{}
	if d.MD5Key != "" {
		t.Error("MD5Key should default to empty")
	}
	if d.PeerAddr != nil {
		t.Error("PeerAddr should default to nil")
	}
	if d.OutTTL != 0 {
		t.Error("OutTTL should default to zero")
	}
}

// TestRealListenerFactoryMD5PeersZeroValue verifies MD5Peers defaults to nil.
//
// VALIDATES: RealListenerFactory with no MD5 config works identically to before.
// PREVENTS: MD5Peers field breaking existing factory behavior.
func TestRealListenerFactoryMD5PeersZeroValue(t *testing.T) {
	f := RealListenerFactory{}
	if f.MD5Peers != nil {
		t.Error("MD5Peers should default to nil")
	}
	// Verify it still creates a working listener
	ln, err := f.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	closeOrLog(t, ln)
}

// TestRealDialerTimeout verifies the Timeout field is applied to the inner
// net.Dialer so a connect that would otherwise hang is bounded.
//
// VALIDATES: AC-1 -- RealDialer with Timeout set honors it (same as net.Dialer.Timeout).
// PREVENTS: Timeout field being dropped, leaving connects on the ~75s OS default.
func TestRealDialerTimeout(t *testing.T) {
	// 192.0.2.1 is RFC 5737 TEST-NET-1: not routed to a real host, so the SYN
	// is dropped and the connect can only end via the Timeout (or a fast
	// "unreachable" on an offline host -- both satisfy the bound below). A
	// context WITHOUT a deadline isolates the Timeout field as the sole bound.
	d := &RealDialer{Timeout: 200 * time.Millisecond}
	start := time.Now()
	conn, err := d.DialContext(context.Background(), "tcp", "192.0.2.1:80")
	elapsed := time.Since(start)
	if err == nil {
		closeOrLog(t, conn)
		t.Fatal("expected dial to fail, got a connection")
	}
	// Without an honored Timeout the connect would block far longer than this.
	if elapsed > 5*time.Second {
		t.Fatalf("dial took %v, Timeout was not applied", elapsed)
	}
}

// TestRealDialerTimeoutZero verifies a zero Timeout leaves connects controlled
// by the context only -- the pre-existing behavior for every non-BGP caller.
//
// VALIDATES: AC-2 -- RealDialer with Timeout zero dials successfully (unchanged).
// PREVENTS: A regression where the zero value would abort valid connections.
func TestRealDialerTimeoutZero(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer closeOrLog(t, ln)

	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		closeOrLog(t, conn)
	}()

	d := &RealDialer{} // Timeout zero-value
	conn, err := d.DialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("DialContext with zero Timeout failed: %v", err)
	}
	closeOrLog(t, conn)
}

// TestRealDialerSetSourceAddress verifies source-address binding is validated
// uniformly: empty is a no-op, valid IPs set LocalAddr, and an unparseable
// address is rejected instead of silently binding to the wildcard.
//
// VALIDATES: every outbound TCP/TLS service routes source-address through here,
// so an invalid value is rejected rather than ignored.
// PREVENTS: a nil-IP TCPAddr silently binding the wildcard (OS default).
func TestRealDialerSetSourceAddress(t *testing.T) {
	// Empty: no-op, LocalAddr stays nil.
	d := &RealDialer{}
	if err := d.SetSourceAddress(""); err != nil {
		t.Fatalf("empty source: unexpected error %v", err)
	}
	if d.LocalAddr != nil {
		t.Errorf("empty source set LocalAddr = %v, want nil", d.LocalAddr)
	}

	// Valid IPv4.
	d = &RealDialer{}
	if err := d.SetSourceAddress("127.0.0.1"); err != nil {
		t.Fatalf("valid IPv4: unexpected error %v", err)
	}
	if d.LocalAddr == nil || !d.LocalAddr.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Errorf("LocalAddr = %v, want 127.0.0.1", d.LocalAddr)
	}

	// Valid IPv6.
	d = &RealDialer{}
	if err := d.SetSourceAddress("::1"); err != nil {
		t.Fatalf("valid IPv6: unexpected error %v", err)
	}
	if d.LocalAddr == nil || !d.LocalAddr.IP.Equal(net.IPv6loopback) {
		t.Errorf("LocalAddr = %v, want ::1", d.LocalAddr)
	}

	// Invalid: rejected, LocalAddr left untouched (not a wildcard bind).
	d = &RealDialer{}
	if err := d.SetSourceAddress("not-an-ip"); err == nil {
		t.Error("invalid source: expected error, got nil")
	}
	if d.LocalAddr != nil {
		t.Errorf("invalid source set LocalAddr = %v, want nil", d.LocalAddr)
	}
}

type closer interface {
	Close() error
}

func closeOrLog(t *testing.T, c closer) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Logf("close: %v", err)
	}
}
