package network

import (
	"context"
	"net"
	"testing"
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

type closer interface {
	Close() error
}

func closeOrLog(t *testing.T, c closer) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Logf("close: %v", err)
	}
}
