// Design: docs/research/vpp-deployment-reference.md -- GoVPP connection management
// Overview: config.go -- VPPSettings with APISocket path

package vpp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.fd.io/govpp/api"
)

var (
	errVppConnectAlreadyInProgress  = errors.New("vpp: connect already in progress")
	errVppNotConnected              = errors.New("vpp: not connected")
	errVppConnectTimeoutNotPositive = errors.New("vpp: connect timeout must be greater than 0")
	errVppGenerationChanged         = errors.New("vpp: connection generation changed")
)

// Connector manages the GoVPP connection to VPP's binary API socket.
// It provides API channels for dependent plugins (fibvpp, ifacevpp) via NewChannel.
// MUST call Close on shutdown.
type Connector struct {
	mu         sync.Mutex
	conn       connectorConnection
	socket     string
	connected  bool
	connecting bool   // prevents concurrent Connect calls
	generation uint64 // changes on successful connection publication and every Close
	identity   uint64 // immutable process-local owner identity, never an OS PID
	health     *connectorHealth
}

// NewConnector creates a Connector for the given VPP API socket path.
func NewConnector(apiSocket string) *Connector {
	return &Connector{socket: apiSocket, identity: nextConnectorIdentity()}
}

// Connect establishes the GoVPP connection to VPP's binary API socket.
// It uses synchronous, non-reconnecting GoVPP connections. Only this owner can
// publish a replacement, after invalidating the preceding generation.
// maxAttempts bounds retries, with retryInterval between each failed attempt.
// Blocks until connected, context canceled, or all attempts exhausted.
// Does NOT hold the mutex during the blocking wait.
func (c *Connector) Connect(ctx context.Context, maxAttempts int, retryInterval time.Duration) error {
	return c.connect(ctx, maxAttempts, retryInterval, func(ctx context.Context) (connectorConnection, error) {
		connection, err := connectVPPOnce(ctx, c.socket)
		if err != nil {
			return nil, err
		}
		if connection == nil {
			return nil, errors.New("vpp: native connector returned no connection")
		}
		return connection, nil
	})
}

func (c *Connector) connect(ctx context.Context, maxAttempts int, retryInterval time.Duration,
	open func(context.Context) (connectorConnection, error)) error {
	if ctx == nil {
		return errors.New("vpp: connect requires a context")
	}
	if maxAttempts <= 0 {
		return errors.New("vpp: connect attempts must be positive")
	}
	if maxAttempts > 1 && retryInterval <= 0 {
		return errors.New("vpp: connect retry interval must be positive")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return nil
	}
	if c.connecting {
		c.mu.Unlock()
		return errVppConnectAlreadyInProgress
	}
	c.connecting = true
	started := c.generation
	previous := c.conn
	previousHealth := c.health
	c.conn = nil
	c.health = nil
	c.mu.Unlock()
	stopConnectorHealth(previous, previousHealth)

	defer func() {
		c.mu.Lock()
		c.connecting = false
		c.mu.Unlock()
	}()

	var failure error
	for attempt := range maxAttempts {
		if err := ctx.Err(); err != nil {
			return err
		}
		conn, err := open(ctx)
		if err == nil && conn == nil {
			err = errors.New("vpp: connect returned no connection")
		}
		if err == nil {
			c.mu.Lock()
			if c.generation != started {
				err = errVppGenerationChanged
			}
			if err == nil {
				err = ctx.Err()
			}
			if err == nil {
				if physical, ok := conn.(witnessedConnection); ok {
					peer := physical.processPeer()
					if peer == nil {
						err = errors.New("vpp: API connection has no physical process witness")
					} else {
						err = peer.bind(c.identity, c.generation+1, processObservers.peerSlots)
					}
				}
			}
			if err == nil {
				c.conn = conn
				c.generation++
				c.connected = true
				c.health = c.startHealthLocked(conn, c.generation)
			}
			c.mu.Unlock()
			if err != nil {
				conn.Disconnect()
			}
			return err
		}
		if conn != nil {
			conn.Disconnect()
		}
		failure = err
		if attempt+1 == maxAttempts {
			break
		}
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("vpp: connect to %s: %w", c.socket, failure)
}

// NewChannel creates a new GoVPP API channel for making VPP binary API calls.
// Each caller should create its own channel. Channels are safe for concurrent use.
// Caller MUST call channel.Close() when done.
func (c *Connector) NewChannel() (api.Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return nil, errVppNotConnected
	}

	ch, err := c.conn.NewAPIChannel()
	if err != nil {
		return nil, fmt.Errorf("vpp: new channel: %w", err)
	}
	return ch, nil
}

// NewStream opens a context-bound binary API stream for APIs that terminate
// with an explicit reply rather than a control-ping dump marker. The caller
// MUST Close every successful stream. Safe for concurrent use.
func (c *Connector) NewStream(ctx context.Context, options ...api.StreamOption) (api.Stream, error) {
	if ctx == nil {
		return nil, errors.New("vpp: stream requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.newStreamLocked(ctx, options...)
}

// ConnectionGeneration snapshots the connected transport's ownership epoch.
// A caller retaining VPP resources MUST use NewStreamForGeneration with this
// result for every subsequent operation. Safe for concurrent use.
func (c *Connector) ConnectionGeneration() (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.connected {
		return 0, errVppNotConnected
	}
	if c.conn == nil {
		return 0, errVppNotConnected
	}
	return c.generation, nil
}

// NewStreamForGeneration atomically checks a retained resource owner's epoch
// while selecting the transport. It never sends an old owner's operation to a
// replacement VPP connection. The caller MUST Close each returned stream.
// Safe for concurrent use.
func (c *Connector) NewStreamForGeneration(ctx context.Context, generation uint64,
	options ...api.StreamOption) (api.Stream, error) {
	if ctx == nil {
		return nil, errors.New("vpp: stream requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation != generation {
		return nil, errVppGenerationChanged
	}
	return c.newStreamLocked(ctx, options...)
}

// The generation check and c.conn selection share mu; splitting them across
// public methods would permit Close/Connect between the two observations.
func (c *Connector) newStreamLocked(ctx context.Context, options ...api.StreamOption) (api.Stream, error) {
	if !c.connected {
		return nil, errVppNotConnected
	}
	if c.conn == nil {
		return nil, errVppNotConnected
	}
	stream, err := c.conn.NewStream(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("vpp: new stream: %w", err)
	}
	return stream, nil
}

// Socket returns the VPP API socket path this connector dials: the configured
// `vpp { api-socket }`, or its YANG default. Set once by NewConnector and never
// written again, so it is safe for concurrent use without the mutex.
func (c *Connector) Socket() string {
	return c.socket
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
		return errVppConnectTimeoutNotPositive
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
			return fmt.Errorf("vpp not connected after %s", timeout)
		case <-tick.C:
			if c.IsConnected() {
				return nil
			}
		}
	}
}

// Close disconnects from VPP. Safe to call multiple times.
func (c *Connector) Close() {
	c.closeConnection()
}

// closeConnection joins the health worker outside mu. Disconnection does not
// prove process death, including when a supervised child separately exited.
func (c *Connector) closeConnection() {
	c.mu.Lock()
	c.generation++
	conn, health := c.conn, c.health
	c.conn, c.health = nil, nil
	c.connected = false
	c.mu.Unlock()
	stopConnectorHealth(conn, health)
}
