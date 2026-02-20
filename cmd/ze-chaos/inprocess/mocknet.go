// Package inprocess provides in-process execution of ze-chaos with mock
// network connections and virtual clock, enabling fast deterministic simulation.
package inprocess

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// errListenerClosed is returned by Accept when the listener has been closed.
var errListenerClosed = errors.New("listener closed")

// errNoConnection is returned by MockDialer when no connection is registered for an address.
var errNoConnection = errors.New("no mock connection registered for address")

// ConnPairManager creates and manages net.Pipe() connection pairs.
// Each pair consists of a peerEnd (held by chaos peer) and a reactorEnd
// (given to the reactor via MockListener or MockDialer).
type ConnPairManager struct{}

// NewConnPairManager creates a ConnPairManager.
func NewConnPairManager() *ConnPairManager {
	return &ConnPairManager{}
}

// NewPair creates a TCP loopback connection pair.
// Returns (peerEnd, reactorEnd) — buffered, bidirectional byte streams.
//
// Uses real TCP connections instead of net.Pipe() because the BGP OPEN exchange
// requires both sides to write simultaneously. net.Pipe() is synchronous
// (unbuffered), so simultaneous writes deadlock. TCP has kernel write buffers
// that allow writes to complete without the other side reading first.
func (m *ConnPairManager) NewPair() (peerEnd, reactorEnd net.Conn, err error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("listen: %w", err)
	}
	defer func() {
		if closeErr := ln.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close listener: %w", closeErr)
		}
	}()

	type acceptResult struct {
		conn net.Conn
		err  error
	}
	ch := make(chan acceptResult, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		ch <- acceptResult{conn: conn, err: acceptErr}
	}()

	var d net.Dialer
	dialed, err := d.DialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		return nil, nil, fmt.Errorf("dial: %w", err)
	}

	result := <-ch
	if result.err != nil {
		if closeErr := dialed.Close(); closeErr != nil {
			return nil, nil, fmt.Errorf("accept: %w (close: %w)", result.err, closeErr)
		}
		return nil, nil, fmt.Errorf("accept: %w", result.err)
	}

	return dialed, result.conn, nil
}

// MockDialer implements sim.Dialer by returning pre-registered connections.
// Used when the reactor dials out to peers (active peers from reactor's perspective).
type MockDialer struct {
	mu    sync.Mutex
	conns map[string][]net.Conn // key: "network:address"
}

// NewMockDialer creates a MockDialer with no registered connections.
func NewMockDialer() *MockDialer {
	return &MockDialer{
		conns: make(map[string][]net.Conn),
	}
}

// Register adds a connection that will be returned by DialContext for the given address.
// Multiple registrations for the same address are returned in FIFO order.
func (d *MockDialer) Register(network, address string, conn net.Conn) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := network + ":" + address
	d.conns[key] = append(d.conns[key], conn)
}

// DialContext returns a pre-registered connection for the given address.
// Returns an error if no connection is registered or the context is canceled.
func (d *MockDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	key := network + ":" + address
	conns := d.conns[key]
	if len(conns) == 0 {
		return nil, errNoConnection
	}

	conn := conns[0]
	d.conns[key] = conns[1:]

	return conn, nil
}

// MockListenerFactory implements sim.ListenerFactory by creating MockListeners.
type MockListenerFactory struct {
	mu        sync.Mutex
	listeners map[string]*MockListener // key: "network:address"
}

// NewMockListenerFactory creates a MockListenerFactory.
func NewMockListenerFactory() *MockListenerFactory {
	return &MockListenerFactory{
		listeners: make(map[string]*MockListener),
	}
}

// Listen creates a MockListener for the given address.
// If a listener already exists for the address, returns the existing one.
func (f *MockListenerFactory) Listen(_ context.Context, network, address string) (net.Listener, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := network + ":" + address
	if ml, ok := f.listeners[key]; ok {
		return ml, nil
	}

	ml := &MockListener{
		addr:  mockAddr{network: network, address: address},
		conns: make(chan net.Conn, 64),
		done:  make(chan struct{}),
	}
	f.listeners[key] = ml

	return ml, nil
}

// GetListener returns an existing MockListener for the given address, or nil.
func (f *MockListenerFactory) GetListener(network, address string) *MockListener {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := network + ":" + address
	return f.listeners[key]
}

// MockListener implements net.Listener by returning queued connections from Accept().
type MockListener struct {
	addr   mockAddr
	conns  chan net.Conn
	done   chan struct{}
	closed bool
	mu     sync.Mutex
}

// QueueConn adds a connection to be returned by the next Accept() call.
func (l *MockListener) QueueConn(conn net.Conn) {
	l.conns <- conn
}

// Accept returns the next queued connection, or blocks until one is available.
// Returns an error if the listener has been closed.
func (l *MockListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.done:
		return nil, errListenerClosed
	}
}

// SetDeadline satisfies the deadlineSetter interface checked by reactor's
// acceptLoop (listener.go). Without this method, the accept loop exits
// immediately. The method is a no-op because MockListener.Accept() already
// handles shutdown via the done channel — no deadline-based polling needed.
func (l *MockListener) SetDeadline(_ time.Time) error { return nil }

// Close stops the listener. Any pending Accept() calls will return an error.
func (l *MockListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.closed {
		l.closed = true
		close(l.done)
	}

	return nil
}

// Addr returns the listener's network address.
func (l *MockListener) Addr() net.Addr {
	return l.addr
}

// ConnWithAddr wraps a net.Conn with custom local and remote addresses.
// This allows net.Pipe() connections to report *net.TCPAddr, which the
// reactor's handleConnection requires for peer lookup by IP address.
//
// Deadline methods are no-ops because the reactor sets deadlines using the
// virtual clock (e.g., 2025-01-01T00:00:13Z), which is in the past relative
// to real system time. Real TCP connections interpret past deadlines as
// "already expired", causing every Read to return timeout immediately.
// By making deadlines no-ops, reads block normally until data arrives.
type ConnWithAddr struct {
	net.Conn
	local  net.Addr
	remote net.Addr
}

// NewConnWithAddr wraps conn so that LocalAddr/RemoteAddr return the given TCP addresses.
// Deadline methods are disabled for virtual-clock compatibility.
func NewConnWithAddr(conn net.Conn, localAddr, remoteAddr *net.TCPAddr) *ConnWithAddr {
	return &ConnWithAddr{Conn: conn, local: localAddr, remote: remoteAddr}
}

// LocalAddr returns the custom local address.
func (c *ConnWithAddr) LocalAddr() net.Addr { return c.local }

// RemoteAddr returns the custom remote address.
func (c *ConnWithAddr) RemoteAddr() net.Addr { return c.remote }

// SetDeadline is a no-op — virtual clock deadlines would expire immediately on real TCP.
func (c *ConnWithAddr) SetDeadline(_ time.Time) error { return nil }

// SetReadDeadline is a no-op — virtual clock deadlines would expire immediately on real TCP.
func (c *ConnWithAddr) SetReadDeadline(_ time.Time) error { return nil }

// SetWriteDeadline is a no-op — virtual clock deadlines would expire immediately on real TCP.
func (c *ConnWithAddr) SetWriteDeadline(_ time.Time) error { return nil }

// mockAddr implements net.Addr for MockListener.
type mockAddr struct {
	network string
	address string
}

func (a mockAddr) Network() string { return a.network }
func (a mockAddr) String() string  { return a.address }
