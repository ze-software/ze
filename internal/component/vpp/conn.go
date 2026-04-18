// Design: docs/research/vpp-deployment-reference.md -- GoVPP connection management
// Overview: config.go -- VPPSettings with APISocket path

package vpp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.fd.io/govpp/adapter/socketclient"
	"go.fd.io/govpp/api"
	"go.fd.io/govpp/core"
)

// Connector manages the GoVPP connection to VPP's binary API socket.
// It provides API channels for dependent plugins (fibvpp, ifacevpp) via NewChannel.
// MUST call Close on shutdown.
type Connector struct {
	mu         sync.Mutex
	conn       *core.Connection
	socket     string
	connected  bool
	connecting bool // prevents concurrent Connect calls
}

// NewConnector creates a Connector for the given VPP API socket path.
func NewConnector(apiSocket string) *Connector {
	return &Connector{socket: apiSocket}
}

// Connect establishes the GoVPP connection to VPP's binary API socket.
// It uses AsyncConnect with retry logic (maxAttempts attempts, retryInterval between each).
// Blocks until connected, context canceled, or all attempts exhausted.
// Does NOT hold the mutex during the blocking wait.
func (c *Connector) Connect(ctx context.Context, maxAttempts int, retryInterval time.Duration) error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return nil
	}
	if c.connecting {
		c.mu.Unlock()
		return fmt.Errorf("govpp: connect already in progress")
	}
	c.connecting = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.connecting = false
		c.mu.Unlock()
	}()

	conn, connEv, err := core.AsyncConnect(
		socketclient.NewVppClient(c.socket),
		maxAttempts,
		retryInterval,
	)
	if err != nil {
		return fmt.Errorf("govpp async connect to %s: %w", c.socket, err)
	}

	// Wait for connection event without holding the mutex.
	select {
	case e := <-connEv:
		if e.State == core.Connected {
			c.mu.Lock()
			c.conn = conn
			c.connected = true
			c.mu.Unlock()
			return nil
		}
		conn.Disconnect()
		return fmt.Errorf("govpp connection failed: state=%v: %w", e.State, e.Error)
	case <-ctx.Done():
		conn.Disconnect()
		return ctx.Err()
	}
}

// NewChannel creates a new GoVPP API channel for making VPP binary API calls.
// Each caller should create its own channel. Channels are safe for concurrent use.
// Caller MUST call channel.Close() when done.
func (c *Connector) NewChannel() (api.Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return nil, fmt.Errorf("govpp: not connected")
	}

	ch, err := c.conn.NewAPIChannel()
	if err != nil {
		return nil, fmt.Errorf("govpp new channel: %w", err)
	}
	return ch, nil
}

// IsConnected returns whether the GoVPP connection is established.
func (c *Connector) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// WaitConnected blocks until the Connector reports connected, or the timeout
// elapses, or ctx is canceled. Returns nil on success, ctx.Err() on cancel,
// and a timeout error on deadline. Callers that need a synchronous guarantee
// before calling NewChannel use this to smooth over cold-boot races where ze
// and VPP start together but VPP has not yet accepted API clients.
//
// Implementation polls IsConnected at a 50ms interval; this is coarse enough
// to avoid burning CPU on a warm cache and fine enough that a 5-second wait
// loses at most ~50ms of latency. No condition variable because Connect can
// happen from any goroutine and we do not want to re-architect the mutex.
func (c *Connector) WaitConnected(ctx context.Context, timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("govpp WaitConnected: timeout must be > 0")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.IsConnected() {
		return nil
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("govpp WaitConnected: not connected after %s", timeout)
		case <-tick.C:
			if c.IsConnected() {
				return nil
			}
		}
	}
}

// Close disconnects from VPP. Safe to call multiple times.
func (c *Connector) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Disconnect()
		c.conn = nil
		c.connected = false
	}
}
