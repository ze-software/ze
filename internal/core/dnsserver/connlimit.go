// Design: docs/architecture/dns/server-harness.md -- the TCP connection cap
// Related: manager.go -- bind wraps each plain TCP listener when Options.TCPConnMax is set

package dnsserver

import (
	"log/slog"
	"net"
	"sync"
	"time"
)

// The per-connection deadlines every plain TCP listener runs with. They are
// the miekg/dns defaults, passed at the call site so a changed library default
// cannot change Ze in silence. Together with the connection cap they bound
// what one client can hold: a connection that sends nothing is closed after
// tcpReadTimeout, and one that goes quiet between queries after tcpIdleTimeout.
const (
	tcpReadTimeout  = 2 * time.Second
	tcpWriteTimeout = 2 * time.Second
	tcpIdleTimeout  = 8 * time.Second
	// tcpQueriesMax is how many queries one connection may carry before the
	// server closes it, the miekg/dns default.
	tcpQueriesMax = 128
)

// connLimitListener caps how many connections its listener holds open at
// once. A connection accepted past the cap is closed at once and counted, and
// Accept goes on waiting for the next one: the caller never sees it, so a
// flood of connections from anywhere, inside the consumer's ACL or not,
// cannot exhaust the process's file descriptors or goroutines.
//
// Refusing rather than leaving the connection in the kernel backlog
// (golang.org/x/net/netutil.LimitListener) tells the client at once, so it
// retries elsewhere instead of timing out against a full server.
//
// Safe for concurrent use: Accept runs on miekg's accept goroutine, and each
// connection's Close on its own serve goroutine.
type connLimitListener struct {
	net.Listener
	slots chan struct{}
	log   *slog.Logger
}

// newConnLimitListener wraps ln so it holds at most connMax connections.
// connMax MUST be positive.
func newConnLimitListener(ln net.Listener, connMax int, log *slog.Logger) *connLimitListener {
	if connMax <= 0 {
		panic("BUG: dnsserver: connection cap must be positive")
	}
	return &connLimitListener{Listener: ln, slots: make(chan struct{}, connMax), log: log}
}

// Accept returns the next connection that fits under the cap. Every
// connection past it is closed here and never returned.
//
// The loop ends only when the underlying Accept fails, which is how a Stop
// reaches it: the bound on its iterations is the listener's lifetime.
func (l *connLimitListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		select {
		case l.slots <- struct{}{}:
			return &limitedConn{Conn: c, slots: l.slots}, nil
		default:
		}
		hmetrics().tcpConnRefused.Inc()
		if cerr := c.Close(); cerr != nil {
			l.log.Debug("dnsserver: closing a connection past the cap", "remote", c.RemoteAddr().String(), logKeyError, cerr)
		}
	}
}

// limitedConn gives its slot back to the listener on the first Close. miekg
// closes a connection both when it finishes serving it and on shutdown, so the
// release runs once whatever the order.
type limitedConn struct {
	net.Conn
	slots   chan struct{}
	release sync.Once
}

// Close closes the connection and frees its slot.
func (c *limitedConn) Close() error {
	err := c.Conn.Close()
	c.release.Do(func() { <-c.slots })
	return err
}
