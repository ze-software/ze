// Design: (none -- new TACACS+ component)
// Overview: packet.go -- packet header and encryption
// Detail: authenticator.go -- bridges client to authz.Authenticator
// Detail: authorizer.go -- bridges client to per-command authorization
// Detail: accounting.go -- bridges client to aaa.Accountant

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
	typeAuthorization  = 0x02
	typeAccounting     = 0x03
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

	replyData, err := c.sendToServers(body, typeAuthentication, start.Version(), "authentication")
	if err != nil {
		return nil, err
	}

	reply, err := UnmarshalAuthenReply(replyData)
	if err != nil {
		return nil, fmt.Errorf("unmarshal authen reply: %w", err)
	}
	return reply, nil
}

// SendAuthorization sends an authorization REQUEST to the first reachable TACACS+ server.
// Returns the AuthorResponse on success or error if all servers are unreachable.
// RFC 8907 Section 6.
func (c *TacacsClient) SendAuthorization(req *AuthorRequest) (*AuthorResponse, error) {
	body, err := req.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal author request: %w", err)
	}

	replyData, err := c.sendToServers(body, typeAuthorization, 0xC0, "authorization")
	if err != nil {
		return nil, err
	}

	resp, err := UnmarshalAuthorResponse(replyData)
	if err != nil {
		return nil, fmt.Errorf("unmarshal author response: %w", err)
	}
	return resp, nil
}

// SendAccounting sends an accounting REQUEST to the first reachable TACACS+ server.
// Returns the AcctReply on success or error if all servers are unreachable.
// Accounting errors are informational -- callers should log them, never block.
func (c *TacacsClient) SendAccounting(req *AcctRequest) (*AcctReply, error) {
	body, err := req.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal acct request: %w", err)
	}

	replyData, err := c.sendToServers(body, typeAccounting, 0xC0, "accounting")
	if err != nil {
		return nil, err
	}

	reply, err := UnmarshalAcctReply(replyData)
	if err != nil {
		return nil, fmt.Errorf("unmarshal acct reply: %w", err)
	}
	return reply, nil
}

// sendToServers sends a request body to TACACS+ servers in order, returning
// the first successful response body. Shared by Authenticate, SendAuthorization,
// and SendAccounting.
func (c *TacacsClient) sendToServers(body []byte, pktType, version uint8, purpose string) ([]byte, error) {
	sessionID, err := randomSessionID()
	if err != nil {
		return nil, fmt.Errorf("generate session ID: %w", err)
	}

	for _, srv := range c.config.Servers {
		pkt := &Packet{
			Header: PacketHeader{
				Version:   version,
				Type:      pktType,
				SeqNo:     1,
				SessionID: sessionID,
			},
			Body: body,
		}

		replyData, sendErr := c.sendReceive(srv, pkt)
		if sendErr != nil {
			c.logger.Warn("TACACS+ server unreachable",
				"purpose", purpose, "server", srv.Address, "error", sendErr)
			continue
		}

		return replyData, nil
	}

	return nil, fmt.Errorf("all TACACS+ servers unreachable for %s", purpose)
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
