// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS client transport
// Related: packet.go -- packet encode/decode
// Related: dict.go -- packet codes

package radius

import (
	"context"
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

	return &Client{
		config: cfg,
		logger: logger,
		conn:   conn,
	}, nil
}

// Close releases the UDP socket.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
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

	// RFC 2866 Section 3: Accounting-Request authenticator is computed.
	if pkt.Code == CodeAccountingReq {
		auth := AccountingRequestAuth(buf, n, secret)
		copy(buf[4:4+AuthenticatorLen], auth[:])
	}

	addr, err := net.ResolveUDPAddr("udp4", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("radius: resolve %s: %w", serverAddr, err)
	}

	timeout := c.config.Timeout
	respBuf := make([]byte, MaxPacketLen)

	for attempt := range c.config.Retries {
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

		deadline := time.Now().Add(timeout)
		_ = c.conn.SetReadDeadline(deadline)

		for {
			rn, from, readErr := c.conn.ReadFromUDP(respBuf)
			if readErr != nil {
				var netErr net.Error
				if errors.As(readErr, &netErr) && netErr.Timeout() {
					break // retry
				}
				return nil, fmt.Errorf("radius: read: %w", readErr)
			}

			// RFC 2865: only accept responses from the server we sent to.
			if !from.IP.Equal(addr.IP) || from.Port != addr.Port {
				continue
			}

			if rn < MinPacketLen {
				continue
			}

			if respBuf[1] != pkt.Identifier {
				continue
			}

			if !VerifyResponseAuth(respBuf[:rn], pkt.Authenticator, secret) {
				c.logger.Warn("radius: bad response authenticator, discarding",
					"server", serverAddr, "attempt", attempt+1)
				continue
			}

			resp, decErr := Decode(respBuf[:rn])
			if decErr != nil {
				c.logger.Warn("radius: decode response failed",
					"server", serverAddr, "error", decErr)
				continue
			}

			return resp, nil
		}

		// Exponential backoff for next retry.
		timeout *= 2
	}

	return nil, fmt.Errorf("radius: all %d retries exhausted for %s", c.config.Retries, serverAddr)
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
