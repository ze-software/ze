// Design: docs/research/vpp-deployment-reference.md -- immutable API connections
// Related: conn.go -- publication; process_exit.go -- confirmed process death
package vpp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"go.fd.io/govpp/adapter/socketclient"
	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/memclnt"
	"go.fd.io/govpp/core"
)

const connectionProbeInterval = time.Second
const connectionProbeTimeout = 5 * time.Second

// connectorConnection is one initialized, immutable native transport. Its
// private constructor admits pointer owners and never a reconnecting client.
type connectorConnection interface {
	NewAPIChannel() (api.Channel, error)
	NewStream(context.Context, ...api.StreamOption) (api.Stream, error)
	Disconnect()
}

// connectorHealth owns one worker for one immutable GoVPP connection. Its
// connector MUST cancel and join it before forgetting the connection. The
// worker never reconnects; failed probes retire the publication before sending
// the one bounded loss notification the manager consumes.
type connectorHealth struct {
	cancel context.CancelFunc
	done   chan struct{}
	lost   chan error
}

func connectVPPOnce(ctx context.Context, socket string) (connectorConnection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timeout := time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = min(timeout, time.Until(deadline))
	}
	if timeout <= 0 {
		return nil, context.DeadlineExceeded
	}
	client := socketclient.NewVppClient(socket)
	client.SetConnectTimeout(timeout)
	var peer *processWitness
	client.SetConnectionHook(func(connection *net.UnixConn) error {
		var err error
		peer, err = captureProcessWitness(connection)
		return err
	})
	// GoVPP AsyncConnect can replace its internal transport without this
	// connector's mutex or epoch. core.Connect starts no reconnect loop, so an
	// old stream can only ever reach its original Unix connection.
	connection, err := core.Connect(client)
	if err != nil {
		if peer != nil {
			err = errors.Join(err, peer.release())
		}
		return nil, err
	}
	if peer == nil {
		connection.Disconnect()
		return nil, errors.New("vpp: API handshake completed without its physical process witness")
	}
	return &processAPIConnection{Connection: connection, peer: peer}, nil
}

func (c *Connector) startHealthLocked(conn connectorConnection, generation uint64) *connectorHealth {
	ticker := time.NewTicker(connectionProbeInterval)
	return c.startHealth(conn, generation, ticker.C, ticker.Stop, probeVPPConnection)
}

func (c *Connector) startHealth(conn connectorConnection, generation uint64, ticks <-chan time.Time,
	stopTicks func(), probe func(context.Context, connectorConnection) error) *connectorHealth {
	ctx, cancel := context.WithCancel(context.Background())
	owner := &connectorHealth{cancel: cancel, done: make(chan struct{}), lost: make(chan error, 1)}
	go func() {
		defer close(owner.done)
		defer stopTicks()
		// One worker lives until its connection closes or its first failed
		// probe. No per-probe goroutine or unbounded retry exists.
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticks:
				if err := probe(ctx, conn); err != nil {
					if ctx.Err() != nil {
						return
					}
					c.connectionLost(conn, generation, owner, err)
					return
				}
			}
		}
	}()
	return owner
}

func (c *Connector) connectionLost(conn connectorConnection, generation uint64,
	owner *connectorHealth, failure error) {
	c.mu.Lock()
	current := c.conn == conn && c.health == owner && c.generation == generation
	if current {
		c.connected = false
		c.generation++
		owner.lost <- failure
	}
	c.mu.Unlock()
	// This connection cannot redial. Its owner closes the native socket only
	// after joining statistics; invalidation already refuses every new stream.
}

func stopConnectorHealth(conn connectorConnection, health *connectorHealth) {
	if health != nil {
		health.cancel()
	}
	if conn != nil {
		conn.Disconnect()
	}
	if health != nil {
		<-health.done
	}
}

// ConnectionLoss returns the current connection worker's single loss signal.
// The manager MUST select it until shutdown or loss, then Close the connector.
// It never identifies process death; an API failure can leave VPP alive.
func (c *Connector) ConnectionLoss() (<-chan error, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.health == nil {
		return nil, errVppNotConnected
	}
	return c.health.lost, nil
}

func probeVPPConnection(ctx context.Context, conn connectorConnection) (failure error) {
	ctx, cancel := context.WithTimeout(ctx, connectionProbeTimeout)
	defer cancel()
	stream, err := conn.NewStream(ctx)
	if err != nil {
		return err
	}
	defer func() { failure = errors.Join(failure, stream.Close()) }()
	if err := stream.SendMsg(&memclnt.ControlPing{}); err != nil {
		return err
	}
	message, err := stream.RecvMsg()
	if err != nil {
		return err
	}
	reply, ok := message.(*memclnt.ControlPingReply)
	if !ok {
		return fmt.Errorf("vpp: connection probe returned %T", message)
	}
	if reply == nil {
		return errors.New("vpp: connection probe returned no reply")
	}
	if reply.Retval != 0 {
		return fmt.Errorf("vpp: connection probe refused with result %d", reply.Retval)
	}
	return ctx.Err()
}
