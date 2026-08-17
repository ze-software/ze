package ssh

import (
	"context"
	"net"
	"sync"
	"time"
)

type serverConn struct {
	net.Conn

	idleTimeout   time.Duration
	maxDeadline   time.Time
	closeCanceler context.CancelFunc

	// handshakeDeadline is cleared once the handshake completes, which happens
	// after gossh.NewServerConn has started goroutines that read it via
	// updateDeadline. Access it only through the accessors below.
	mu                sync.Mutex
	handshakeDeadline time.Time
}

// setHandshakeDeadline bounds how long the handshake may take.
func (c *serverConn) setHandshakeDeadline(t time.Time) {
	c.mu.Lock()
	c.handshakeDeadline = t
	c.mu.Unlock()
	c.updateDeadline()
}

// clearHandshakeDeadline drops the handshake deadline once the handshake has
// completed, leaving the idle and max deadlines to govern the connection.
func (c *serverConn) clearHandshakeDeadline() {
	c.mu.Lock()
	c.handshakeDeadline = time.Time{}
	c.mu.Unlock()
	c.updateDeadline()
}

func (c *serverConn) Write(p []byte) (n int, err error) {
	if c.idleTimeout > 0 {
		c.updateDeadline()
	}
	n, err = c.Conn.Write(p)
	if _, isNetErr := err.(net.Error); isNetErr && c.closeCanceler != nil {
		c.closeCanceler()
	}
	return
}

func (c *serverConn) Read(b []byte) (n int, err error) {
	if c.idleTimeout > 0 {
		c.updateDeadline()
	}
	n, err = c.Conn.Read(b)
	if _, isNetErr := err.(net.Error); isNetErr && c.closeCanceler != nil {
		c.closeCanceler()
	}
	return
}

func (c *serverConn) Close() (err error) {
	err = c.Conn.Close()
	if c.closeCanceler != nil {
		c.closeCanceler()
	}
	return
}

func (c *serverConn) updateDeadline() {
	c.mu.Lock()
	handshakeDeadline := c.handshakeDeadline
	c.mu.Unlock()

	// idleTimeout and maxDeadline are set before the handshake starts and never
	// mutated afterwards, so they need no locking.
	deadline := c.maxDeadline

	if !handshakeDeadline.IsZero() && (deadline.IsZero() || handshakeDeadline.Before(deadline)) {
		deadline = handshakeDeadline
	}

	if c.idleTimeout > 0 {
		idleDeadline := time.Now().Add(c.idleTimeout)
		if deadline.IsZero() || idleDeadline.Before(deadline) {
			deadline = idleDeadline
		}
	}

	_ = c.SetDeadline(deadline)
}
