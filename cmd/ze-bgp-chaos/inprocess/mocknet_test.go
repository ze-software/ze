package inprocess

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/sim"
)

// TestConnPairReadWrite verifies net.Pipe pair: write on one end, read on other.
//
// VALIDATES: Data written to peerEnd is readable from reactorEnd and vice versa.
// PREVENTS: Mock connections being one-directional or lossy.
func TestConnPairReadWrite(t *testing.T) {
	mgr := NewConnPairManager()
	peerEnd, reactorEnd, err := mgr.NewPair()
	if err != nil {
		t.Fatalf("NewPair: %v", err)
	}

	msg := []byte("hello from peer")
	go func() {
		if _, err := peerEnd.Write(msg); err != nil {
			t.Errorf("Write error: %v", err)
		}
	}()

	buf := make([]byte, len(msg))
	n, err := reactorEnd.Read(buf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if string(buf[:n]) != "hello from peer" {
		t.Errorf("Read = %q, want %q", buf[:n], msg)
	}

	// Reverse direction.
	reply := []byte("hello from reactor")
	go func() {
		if _, err := reactorEnd.Write(reply); err != nil {
			t.Errorf("Write error: %v", err)
		}
	}()

	buf2 := make([]byte, len(reply))
	n2, err := peerEnd.Read(buf2)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if string(buf2[:n2]) != "hello from reactor" {
		t.Errorf("Read = %q, want %q", buf2[:n2], reply)
	}

	if err := peerEnd.Close(); err != nil {
		t.Errorf("peerEnd.Close error: %v", err)
	}
	if err := reactorEnd.Close(); err != nil {
		t.Errorf("reactorEnd.Close error: %v", err)
	}
}

// TestConnPairClose verifies Close one end → Read on other returns io.EOF.
//
// VALIDATES: Closing one end of a pipe makes the other end's Read return EOF.
// PREVENTS: Connections hanging forever after close.
func TestConnPairClose(t *testing.T) {
	mgr := NewConnPairManager()
	peerEnd, reactorEnd, err := mgr.NewPair()
	if err != nil {
		t.Fatalf("NewPair: %v", err)
	}

	// Close peer end.
	if err := peerEnd.Close(); err != nil {
		t.Fatalf("peerEnd.Close error: %v", err)
	}

	// Read on reactor end should return EOF.
	buf := make([]byte, 16)
	_, err = reactorEnd.Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Errorf("Read after close = %v, want io.EOF", err)
	}

	if err := reactorEnd.Close(); err != nil {
		t.Errorf("reactorEnd.Close error: %v", err)
	}
}

// TestMockDialerReturnsConn verifies MockDialer.DialContext returns registered connection.
//
// VALIDATES: After registering a conn for an address, DialContext returns it.
// PREVENTS: Dialer failing to find pre-registered connections.
func TestMockDialerReturnsConn(t *testing.T) {
	mgr := NewConnPairManager()
	peerEnd, reactorEnd, pairErr := mgr.NewPair()
	if pairErr != nil {
		t.Fatalf("NewPair: %v", pairErr)
	}
	defer func() {
		if err := peerEnd.Close(); err != nil {
			t.Errorf("peerEnd.Close: %v", err)
		}
	}()

	md := NewMockDialer()
	md.Register("tcp", "127.0.0.1:1790", reactorEnd)

	ctx := context.Background()
	conn, err := md.DialContext(ctx, "tcp", "127.0.0.1:1790")
	if err != nil {
		t.Fatalf("DialContext error: %v", err)
	}
	if conn != reactorEnd {
		t.Error("DialContext returned wrong connection")
	}

	if err := conn.Close(); err != nil {
		t.Errorf("conn.Close: %v", err)
	}
}

// TestMockDialerNoConn verifies MockDialer.DialContext returns error when nothing registered.
//
// VALIDATES: DialContext to an unregistered address returns an error.
// PREVENTS: Silent nil conn on unregistered addresses.
func TestMockDialerNoConn(t *testing.T) {
	md := NewMockDialer()

	ctx := context.Background()
	conn, err := md.DialContext(ctx, "tcp", "127.0.0.1:1790")
	if err == nil {
		t.Fatal("DialContext should return error for unregistered address")
	}
	if conn != nil {
		t.Error("DialContext should return nil conn on error")
	}
}

// TestMockDialerContextCancelled verifies MockDialer respects context cancellation.
//
// VALIDATES: DialContext returns error when context is cancelled.
// PREVENTS: Mock dialer ignoring context.
func TestMockDialerContextCancelled(t *testing.T) {
	md := NewMockDialer()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := md.DialContext(ctx, "tcp", "127.0.0.1:1790")
	if err == nil {
		t.Fatal("DialContext should return error for cancelled context")
	}
}

// TestMockListenerAccept verifies MockListener.Accept returns queued connections in order.
//
// VALIDATES: Connections registered for Accept() are returned in FIFO order.
// PREVENTS: Wrong connection going to wrong peer.
func TestMockListenerAccept(t *testing.T) {
	mgr := NewConnPairManager()
	peer1, reactor1, err := mgr.NewPair()
	if err != nil {
		t.Fatalf("NewPair 1: %v", err)
	}
	peer2, reactor2, err := mgr.NewPair()
	if err != nil {
		t.Fatalf("NewPair 2: %v", err)
	}
	defer func() {
		if err := peer1.Close(); err != nil {
			t.Errorf("peer1.Close: %v", err)
		}
	}()
	defer func() {
		if err := peer2.Close(); err != nil {
			t.Errorf("peer2.Close: %v", err)
		}
	}()

	mlf := NewMockListenerFactory()
	ctx := context.Background()
	ln, err := mlf.Listen(ctx, "tcp", "127.0.0.1:1790")
	if err != nil {
		t.Fatalf("Listen error: %v", err)
	}

	// Queue two connections.
	ml, ok := ln.(*MockListener)
	if !ok {
		t.Fatal("Listen did not return *MockListener")
	}
	ml.QueueConn(reactor1)
	ml.QueueConn(reactor2)

	// Accept should return them in order.
	conn1, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept error: %v", err)
	}
	if conn1 != reactor1 {
		t.Error("first Accept returned wrong connection")
	}

	conn2, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept error: %v", err)
	}
	if conn2 != reactor2 {
		t.Error("second Accept returned wrong connection")
	}

	if err := ln.Close(); err != nil {
		t.Errorf("ln.Close: %v", err)
	}
}

// TestMockListenerClose verifies MockListener.Close → Accept returns error.
//
// VALIDATES: After Close(), Accept() returns an error instead of blocking.
// PREVENTS: Accept blocking forever on closed listener.
func TestMockListenerClose(t *testing.T) {
	mlf := NewMockListenerFactory()
	ctx := context.Background()
	ln, err := mlf.Listen(ctx, "tcp", "127.0.0.1:1790")
	if err != nil {
		t.Fatalf("Listen error: %v", err)
	}

	if err := ln.Close(); err != nil {
		t.Fatalf("ln.Close: %v", err)
	}

	// Accept after close should return error.
	done := make(chan error, 1)
	go func() {
		_, acceptErr := ln.Accept()
		done <- acceptErr
	}()

	select {
	case acceptErr := <-done:
		if acceptErr == nil {
			t.Fatal("Accept should return error after Close")
		}
	case <-time.After(time.Second):
		t.Fatal("Accept blocked after Close")
	}
}

// TestMockListenerFactoryImplements verifies compile-time interface conformance.
//
// VALIDATES: MockListenerFactory satisfies sim.ListenerFactory.
// PREVENTS: Missing methods breaking injection into reactor.
func TestMockListenerFactoryImplements(t *testing.T) {
	var _ sim.ListenerFactory = &MockListenerFactory{}
}

// TestMockDialerImplements verifies compile-time interface conformance.
//
// VALIDATES: MockDialer satisfies sim.Dialer.
// PREVENTS: Missing methods breaking injection into reactor.
func TestMockDialerImplements(t *testing.T) {
	var _ sim.Dialer = &MockDialer{}
}
