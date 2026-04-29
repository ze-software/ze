// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS client transport
// Related: packet.go -- packet encode/decode
// Related: dict.go -- packet codes

package radius

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Server holds configuration for a single RADIUS server.
type Server struct {
	Address   string // "host:port"
	SharedKey []byte // RADIUS shared secret (RFC 2865)
}

// ClientConfig holds configuration for the RADIUS client.
type ClientConfig struct {
	Servers       []Server
	Timeout       time.Duration // per-request timeout (default 3s)
	Retries       int           // retransmit count (default 3)
	SourceAddress net.IP        // bind outbound socket to this IP; nil = any
	Logger        *slog.Logger
}

// Client is a RADIUS UDP client with retransmit and server failover.
type Client struct {
	config ClientConfig
	logger *slog.Logger
	nextID atomic.Uint32
	mu     sync.Mutex
	conn   *net.UDPConn
	closed bool
	done   chan struct{}
	wait   map[responseKey][]*responseWaiter
}

type responseKey struct {
	server string
	id     uint8
}

type responseWaiter struct {
	auth   [AuthenticatorLen]byte
	secret []byte
	ch     chan []byte
}

// NewClient creates a RADIUS client.
func NewClient(cfg ClientConfig) (*Client, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 3 * time.Second
	}
	if cfg.Retries == 0 {
		cfg.Retries = 3
	}

	var laddr *net.UDPAddr
	if len(cfg.SourceAddress) > 0 {
		laddr = &net.UDPAddr{IP: cfg.SourceAddress}
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, fmt.Errorf("radius: listen: %w", err)
	}

	c := &Client{
		config: cfg,
		logger: logger,
		conn:   conn,
		done:   make(chan struct{}),
		wait:   make(map[responseKey][]*responseWaiter),
	}
	go c.readLoop()
	return c, nil
}

// Close releases the UDP socket.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	err := c.conn.Close()
	done := c.done
	c.mu.Unlock()
	<-done
	return err
}

// NextID returns the next RADIUS packet identifier (0-255 cycling).
func (c *Client) NextID() uint8 {
	return uint8(c.nextID.Add(1))
}

// Exchange sends a RADIUS request to a single server with retransmit.
// Returns the decoded response or error.
//
// RFC 2865 Section 5.2: User-Password attributes are XOR-encoded with
// the server's shared secret before sending. RFC 2866 Section 3:
// Accounting-Request authenticators are computed (not random).
func (c *Client) Exchange(ctx context.Context, pkt *Packet, secret []byte, serverAddr string) (*Packet, error) {
	buf := Bufs.Get()
	defer Bufs.Put(buf)

	wirePkt := prepareWirePacket(pkt, secret)
	n, err := wirePkt.EncodeTo(buf, 0)
	if err != nil {
		return nil, fmt.Errorf("radius: encode: %w", err)
	}

	requestAuth := pkt.Authenticator
	// RFC 2866 Section 3: Accounting-Request authenticator is computed.
	if pkt.Code == CodeAccountingReq {
		auth := AccountingRequestAuth(buf, n, secret)
		copy(buf[4:4+AuthenticatorLen], auth[:])
		requestAuth = auth
	}

	addr, err := net.ResolveUDPAddr("udp4", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("radius: resolve %s: %w", serverAddr, err)
	}
	key := responseKey{server: addr.String(), id: pkt.Identifier}
	waiter, err := c.registerWaiter(key, requestAuth, secret)
	if err != nil {
		return nil, err
	}
	defer c.unregisterWaiter(key, waiter)

	timeout := c.config.Timeout

retryLoop:
	for range c.config.Retries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// RFC 2865 Section 2.5: retransmit uses same ID and authenticator.
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return nil, errors.New("radius: client closed")
		}
		_, writeErr := c.conn.WriteToUDP(buf[:n], addr)
		c.mu.Unlock()

		if writeErr != nil {
			return nil, fmt.Errorf("radius: write to %s: %w", serverAddr, writeErr)
		}

		timer := time.NewTimer(timeout)
		for {
			select {
			case data := <-waiter.ch:
				resp, decErr := Decode(data)
				if decErr != nil {
					c.logger.Warn("radius: decode response failed",
						"server", serverAddr, "error", decErr)
					continue
				}
				stopTimer(timer)
				return resp, nil
			case <-timer.C:
				// Retry with exponential backoff.
				timeout *= 2
				continue retryLoop
			case <-ctx.Done():
				stopTimer(timer)
				return nil, ctx.Err()
			case <-c.done:
				stopTimer(timer)
				return nil, errors.New("radius: client closed")
			}
		}
	}

	return nil, fmt.Errorf("radius: all %d retries exhausted for %s", c.config.Retries, serverAddr)
}

func (c *Client) readLoop() {
	defer close(c.done)
	buf := make([]byte, MaxPacketLen)
	for {
		n, from, err := c.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if n < MinPacketLen {
			continue
		}
		pktLen := int(binary.BigEndian.Uint16(buf[2:4]))
		if pktLen < MinPacketLen || pktLen > n || pktLen > MaxPacketLen {
			continue
		}
		c.dispatchResponse(responseKey{server: from.String(), id: buf[1]}, buf[:pktLen])
	}
}

func (c *Client) registerWaiter(key responseKey, auth [AuthenticatorLen]byte, secret []byte) (*responseWaiter, error) {
	w := &responseWaiter{auth: auth, secret: secret, ch: make(chan []byte, 4)}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("radius: client closed")
	}
	c.wait[key] = append(c.wait[key], w)
	return w, nil
}

func (c *Client) unregisterWaiter(key responseKey, waiter *responseWaiter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	waits := c.wait[key]
	for i, w := range waits {
		if w == waiter {
			waits = append(waits[:i], waits[i+1:]...)
			break
		}
	}
	if len(waits) == 0 {
		delete(c.wait, key)
		return
	}
	c.wait[key] = waits
}

func (c *Client) dispatchResponse(key responseKey, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	waits := c.wait[key]
	for _, w := range waits {
		if !VerifyResponseAuth(data, w.auth, w.secret) {
			continue
		}
		copyData := make([]byte, len(data))
		copy(copyData, data)
		select {
		case w.ch <- copyData:
		default:
		}
		return
	}
	if len(waits) > 0 {
		c.logger.Warn("radius: bad response authenticator, discarding", "server", key.server)
	}
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

// prepareWirePacket returns a copy of pkt with User-Password attributes
// XOR-encoded per RFC 2865 Section 5.2. The original packet is not modified
// so failover to a different server (different secret) re-encodes correctly.
func prepareWirePacket(pkt *Packet, secret []byte) *Packet {
	hasUserPassword := false
	for _, a := range pkt.Attrs {
		if a.Type == AttrUserPassword {
			hasUserPassword = true
			break
		}
	}
	if !hasUserPassword {
		return pkt
	}

	encoded := make([]Attr, len(pkt.Attrs))
	copy(encoded, pkt.Attrs)
	for i := range encoded {
		if encoded[i].Type == AttrUserPassword {
			encoded[i].Value = EncodeUserPassword(encoded[i].Value, secret, pkt.Authenticator)
		}
	}
	return &Packet{
		Code:          pkt.Code,
		Identifier:    pkt.Identifier,
		Authenticator: pkt.Authenticator,
		Attrs:         encoded,
	}
}

// SendToServers sends a request to RADIUS servers in failover order.
// Returns the first successful response.
func (c *Client) SendToServers(ctx context.Context, pkt *Packet) (*Packet, error) {
	for _, srv := range c.config.Servers {
		pkt.Identifier = c.NextID()
		resp, err := c.Exchange(ctx, pkt, srv.SharedKey, srv.Address)
		if err != nil {
			c.logger.Warn("radius: server unreachable",
				"server", srv.Address, "error", err)
			continue
		}
		return resp, nil
	}
	return nil, errors.New("radius: all servers unreachable")
}
