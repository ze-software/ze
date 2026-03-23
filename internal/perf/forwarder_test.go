package perf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// forwarderSessionConfig is the SessionConfig used by the forwarder's OPEN message.
var forwarderSessionConfig = SessionConfig{
	ASN:      65000,
	RouterID: netip.MustParseAddr("10.0.0.1"),
	HoldTime: 90,
	Family:   "ipv4/unicast",
}

// doBGPHandshake performs the client side of a BGP OPEN/KEEPALIVE handshake
// in tests. Sets a 5-second deadline, calls DoHandshake, then clears the deadline.
func doBGPHandshake(t *testing.T, conn net.Conn, cfg SessionConfig) {
	t.Helper()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}

	if _, err := DoHandshake(conn, cfg); err != nil {
		t.Fatalf("handshake: %v", err)
	}

	// Clear deadline for data transfer phase.
	if err := conn.SetDeadline(time.Time{}); err != nil {
		t.Fatalf("clear deadline: %v", err)
	}
}

// testForwarder is a minimal BGP DUT for integration testing.
// It accepts two TCP connections, completes BGP OPEN/KEEPALIVE handshake
// on each, and forwards UPDATE messages bidirectionally between them.
//
// Caller MUST call Run in a goroutine after construction. Cancel the context
// to stop the forwarder.
type testForwarder struct {
	t        testing.TB
	listener net.Listener
}

// newTestForwarder creates a test forwarder listening on a random port.
// Caller MUST call Run to accept connections and start forwarding.
func newTestForwarder(t testing.TB) *testForwarder {
	t.Helper()

	var lc net.ListenConfig

	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	return &testForwarder{
		t:        t,
		listener: ln,
	}
}

// Addr returns the listener address (host:port) for dialing.
func (f *testForwarder) Addr() string {
	return f.listener.Addr().String()
}

// acceptOne accepts a single connection from the listener with deadline from ctx,
// sets TCP_NODELAY, and performs BGP handshake. Returns the connection on success.
func (f *testForwarder) acceptOne(ctx context.Context, idx int) (net.Conn, error) {
	if dl, ok := ctx.Deadline(); ok {
		if tcpLn, ok := f.listener.(*net.TCPListener); ok {
			_ = tcpLn.SetDeadline(dl)
		}
	}

	conn, err := f.listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("accept %d: %w", idx, err)
	}

	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}

	if err := f.doHandshake(conn); err != nil {
		defer func() { _ = conn.Close() }()

		return nil, fmt.Errorf("handshake %d: %w", idx, err)
	}

	return conn, nil
}

// Run accepts pairs of connections in a loop, performing BGP handshake on each
// immediately after acceptance (so RunBenchmark's connect-handshake-connect-handshake
// sequence works), then forwards UPDATEs bidirectionally. Each pair is handled
// until both connections close (one benchmark iteration), then the next pair is
// accepted. Blocks until ctx is canceled. Closes the listener on return.
func (f *testForwarder) Run(ctx context.Context) {
	defer func() { _ = f.listener.Close() }()

	for ctx.Err() == nil {
		f.runOnePair(ctx)
	}
}

// runOnePair accepts one pair of connections, forwards UPDATEs between them,
// and returns when both connections close or context is canceled.
func (f *testForwarder) runOnePair(ctx context.Context) {
	conn0, err := f.acceptOne(ctx, 0)
	if err != nil {
		if ctx.Err() == nil {
			f.t.Logf("forwarder: %v", err)
		}

		return
	}
	defer func() { _ = conn0.Close() }()

	conn1, err := f.acceptOne(ctx, 1)
	if err != nil {
		if ctx.Err() == nil {
			f.t.Logf("forwarder: %v", err)
		}

		return
	}
	defer func() { _ = conn1.Close() }()

	// Use a pair-scoped context so keepalive goroutines stop
	// promptly when forwarding ends (connection closed by peer).
	pairCtx, pairCancel := context.WithCancel(ctx)
	defer pairCancel()

	conns := [2]net.Conn{conn0, conn1}

	// Start keepalive goroutines.
	var wg sync.WaitGroup

	for _, conn := range conns {
		wg.Add(1)

		go func(c net.Conn) {
			defer wg.Done()
			f.keepaliveLoop(pairCtx, c)
		}(conn)
	}

	// Forward UPDATEs bidirectionally. When both forwarders return
	// (connection closed), cancel pairCtx to stop keepalives.
	var fwdWg sync.WaitGroup

	fwdWg.Add(2)

	go func() {
		defer fwdWg.Done()
		f.forwardUpdates(pairCtx, conns[0], conns[1])
	}()

	go func() {
		defer fwdWg.Done()
		f.forwardUpdates(pairCtx, conns[1], conns[0])
	}()

	fwdWg.Wait()
	pairCancel()
	wg.Wait()
}

// doHandshake performs the forwarder side of a BGP OPEN/KEEPALIVE exchange.
// Reads peer's OPEN, sends our OPEN, reads peer's KEEPALIVE, sends KEEPALIVE.
func (f *testForwarder) doHandshake(conn net.Conn) error {
	// Set deadline for handshake.
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	// Read peer's OPEN.
	msgType, _, err := ReadMessage(conn)
	if err != nil {
		return fmt.Errorf("reading peer OPEN: %w", err)
	}

	if msgType != message.TypeOPEN {
		return fmt.Errorf("expected OPEN, got type %d", msgType)
	}

	// Send our OPEN.
	if err := WriteMessage(conn, BuildOpen(forwarderSessionConfig)); err != nil {
		return fmt.Errorf("sending OPEN: %w", err)
	}

	// Read peer's KEEPALIVE.
	msgType, _, err = ReadMessage(conn)
	if err != nil {
		return fmt.Errorf("reading peer KEEPALIVE: %w", err)
	}

	if msgType != message.TypeKEEPALIVE {
		return fmt.Errorf("expected KEEPALIVE, got type %d", msgType)
	}

	// Send our KEEPALIVE.
	if err := WriteMessage(conn, BuildKeepalive()); err != nil {
		return fmt.Errorf("sending KEEPALIVE: %w", err)
	}

	// Clear deadline.
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return fmt.Errorf("clear deadline: %w", err)
	}

	return nil
}

// keepaliveLoop sends KEEPALIVE messages every 30 seconds until ctx is canceled.
func (f *testForwarder) keepaliveLoop(ctx context.Context, conn net.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ka := BuildKeepalive()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := WriteMessage(conn, ka); err != nil {
				return
			}
		}
	}
}

// forwardUpdates reads BGP messages from src and copies UPDATE messages to dst.
// KEEPALIVE messages are silently consumed. Stops on context cancellation or error.
func (f *testForwarder) forwardUpdates(ctx context.Context, src, dst net.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Short read deadline to allow context check.
		_ = src.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		msgType, msg, err := ReadMessage(src)
		if err != nil {
			if isTimeout(err) {
				continue
			}

			return
		}

		if msgType == message.TypeUPDATE {
			if err := WriteMessage(dst, msg); err != nil {
				return
			}
		}
		// KEEPALIVE and other types are consumed silently.
	}
}

// testSinkForwarder is a minimal BGP DUT that accepts connections and does
// handshakes, but silently drops all UPDATE messages instead of forwarding them.
// This is used to test timeout/partial-convergence scenarios.
type testSinkForwarder struct {
	t        testing.TB
	listener net.Listener
}

// newTestSinkForwarder creates a sink forwarder listening on a random port.
// Caller MUST call Run to accept connections and start the sink loop.
func newTestSinkForwarder(t testing.TB) *testSinkForwarder {
	t.Helper()

	var lc net.ListenConfig

	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	return &testSinkForwarder{
		t:        t,
		listener: ln,
	}
}

// Addr returns the listener address (host:port) for dialing.
func (f *testSinkForwarder) Addr() string {
	return f.listener.Addr().String()
}

// acceptOne accepts a single connection, sets TCP_NODELAY, and performs BGP handshake.
func (f *testSinkForwarder) acceptOne(ctx context.Context, idx int) (net.Conn, error) {
	if dl, ok := ctx.Deadline(); ok {
		if tcpLn, ok := f.listener.(*net.TCPListener); ok {
			_ = tcpLn.SetDeadline(dl)
		}
	}

	conn, err := f.listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("accept %d: %w", idx, err)
	}

	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}

	if err := f.doHandshake(conn); err != nil {
		defer func() { _ = conn.Close() }()

		return nil, fmt.Errorf("handshake %d: %w", idx, err)
	}

	return conn, nil
}

// Run accepts pairs of connections in a loop, performing BGP handshake on each
// immediately after acceptance, then reads and discards all messages (never
// forwards). Blocks until ctx is canceled. Closes the listener on return.
func (f *testSinkForwarder) Run(ctx context.Context) {
	defer func() { _ = f.listener.Close() }()

	for ctx.Err() == nil {
		f.runOnePair(ctx)
	}
}

// runOnePair accepts one pair of connections, sinks all messages, and returns
// when both connections close or context is canceled.
func (f *testSinkForwarder) runOnePair(ctx context.Context) {
	conn0, err := f.acceptOne(ctx, 0)
	if err != nil {
		if ctx.Err() == nil {
			f.t.Logf("sink: %v", err)
		}

		return
	}
	defer func() { _ = conn0.Close() }()

	conn1, err := f.acceptOne(ctx, 1)
	if err != nil {
		if ctx.Err() == nil {
			f.t.Logf("sink: %v", err)
		}

		return
	}
	defer func() { _ = conn1.Close() }()

	// Start sink loops for both connections.
	var wg sync.WaitGroup

	for _, conn := range [2]net.Conn{conn0, conn1} {
		wg.Add(1)

		go func(c net.Conn) {
			defer wg.Done()
			f.sinkLoop(ctx, c)
		}(conn)
	}

	wg.Wait()
}

// doHandshake performs the sink forwarder side of a BGP OPEN/KEEPALIVE exchange.
func (f *testSinkForwarder) doHandshake(conn net.Conn) error {
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	msgType, _, err := ReadMessage(conn)
	if err != nil {
		return fmt.Errorf("reading peer OPEN: %w", err)
	}

	if msgType != message.TypeOPEN {
		return fmt.Errorf("expected OPEN, got type %d", msgType)
	}

	if err := WriteMessage(conn, BuildOpen(forwarderSessionConfig)); err != nil {
		return fmt.Errorf("sending OPEN: %w", err)
	}

	msgType, _, err = ReadMessage(conn)
	if err != nil {
		return fmt.Errorf("reading peer KEEPALIVE: %w", err)
	}

	if msgType != message.TypeKEEPALIVE {
		return fmt.Errorf("expected KEEPALIVE, got type %d", msgType)
	}

	if err := WriteMessage(conn, BuildKeepalive()); err != nil {
		return fmt.Errorf("sending KEEPALIVE: %w", err)
	}

	if err := conn.SetDeadline(time.Time{}); err != nil {
		return fmt.Errorf("clear deadline: %w", err)
	}

	return nil
}

// sinkLoop reads and discards all messages, sending keepalives periodically.
func (f *testSinkForwarder) sinkLoop(ctx context.Context, conn net.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ka := BuildKeepalive()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := WriteMessage(conn, ka); err != nil {
				return
			}
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		_, _, err := ReadMessage(conn)
		if err != nil {
			if isTimeout(err) {
				continue
			}

			return
		}

		// All messages silently consumed -- never forwarded.
	}
}

// isTimeout reports whether err is a network timeout error.
func isTimeout(err error) bool {
	var ne net.Error
	//nolint:errorlint // errors.As is used correctly here.
	if errors.As(err, &ne) {
		return ne.Timeout()
	}

	return false
}
