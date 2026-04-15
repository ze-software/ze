// Design: (none -- new TACACS+ component)
// Overview: packet.go -- packet header and encryption

// TACACS+ TCP client with server failover.
// RFC 8907 Section 4 -- connection management.
package tacacs

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"
)

// Packet type constants used by the client.
const (
	typeAuthentication = 0x01
)

// TacacsServer holds configuration for a single TACACS+ server.
type TacacsServer struct {
	Address string // "host:port"
	Key     []byte // shared encryption key (RFC 8907 "shared secret")
}

// TacacsClientConfig holds configuration for the TACACS+ client.
type TacacsClientConfig struct {
	Servers       []TacacsServer
	Timeout       time.Duration // per-server connection timeout
	SourceAddress string        // optional source IP for outbound connections
	Logger        *slog.Logger
}

// TacacsClient is a TACACS+ client that connects to servers in order.
type TacacsClient struct {
	config TacacsClientConfig
	logger *slog.Logger
}

// NewTacacsClient creates a TACACS+ client.
func NewTacacsClient(cfg TacacsClientConfig) *TacacsClient {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &TacacsClient{config: cfg, logger: logger}
}

// Authenticate performs PAP authentication against TACACS+ servers.
// Tries servers in order. Returns on first response (pass or fail).
// Returns error only on infrastructure failure (all servers unreachable).
func (c *TacacsClient) Authenticate(username, password, port, remAddr string) (*AuthenReply, error) {
	start := NewPAPAuthenStart(username, password, port, remAddr)
	body, err := start.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal authen start: %w", err)
	}

	sessionID, err := randomSessionID()
	if err != nil {
		return nil, fmt.Errorf("generate session ID: %w", err)
	}

	for _, srv := range c.config.Servers {
		pkt := &Packet{
			Header: PacketHeader{
				Version:   start.Version(),
				Type:      typeAuthentication,
				SeqNo:     1,
				SessionID: sessionID,
			},
			Body: body,
		}

		replyData, err := c.sendReceive(srv, pkt)
		if err != nil {
			c.logger.Warn("TACACS+ server unreachable",
				"server", srv.Address, "error", err)
			continue // try next server
		}

		reply, err := UnmarshalAuthenReply(replyData)
		if err != nil {
			c.logger.Warn("TACACS+ malformed reply (possible wrong shared secret)",
				"server", srv.Address, "error", err)
			continue
		}

		return reply, nil
	}

	return nil, fmt.Errorf("all TACACS+ servers unreachable")
}

// sendReceive connects to a server, sends a packet, and reads the response.
// Returns the decrypted response body.
func (c *TacacsClient) sendReceive(srv TacacsServer, pkt *Packet) ([]byte, error) {
	conn, err := c.dial(srv.Address)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", srv.Address, err)
	}
	defer func() { _ = conn.Close() }()

	// Set read/write deadlines.
	deadline := time.Now().Add(c.config.Timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	// Marshal and send.
	wire, err := pkt.Marshal(srv.Key)
	if err != nil {
		return nil, fmt.Errorf("marshal packet: %w", err)
	}
	if _, err := conn.Write(wire); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// Read response header.
	hdrBuf := make([]byte, hdrLen)
	if _, err := io.ReadFull(conn, hdrBuf); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	respHdr, err := UnmarshalPacketHeader(hdrBuf)
	if err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}

	// Validate response header.
	if respHdr.SessionID != pkt.Header.SessionID {
		return nil, fmt.Errorf("session ID mismatch: sent %x, got %x",
			pkt.Header.SessionID, respHdr.SessionID)
	}
	if respHdr.Length > maxBodyLen {
		return nil, ErrBodyTooBig
	}

	// Read response body.
	body := make([]byte, respHdr.Length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Decrypt.
	if len(srv.Key) > 0 && respHdr.Flags&flagUnencrypted == 0 {
		Encrypt(body, respHdr.SessionID, srv.Key, respHdr.Version, respHdr.SeqNo)
	}

	return body, nil
}

// dial creates a TCP connection to the server with the configured timeout.
func (c *TacacsClient) dial(address string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: c.config.Timeout}
	if c.config.SourceAddress != "" {
		dialer.LocalAddr = &net.TCPAddr{IP: net.ParseIP(c.config.SourceAddress)}
	}
	return dialer.Dial("tcp", address)
}

// randomSessionID generates a cryptographically random 4-byte session ID.
func randomSessionID() (uint32, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf[:]), nil
}
