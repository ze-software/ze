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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/bufpool"
)

// TACACS+ pool sizing. A TACACS+ packet is at most 12 (header) + 65535
// (body, uint16 ceiling in RFC 8907 §4.1) = 65547 bytes. Every wire read
// and write path uses buffers of this size. sync.Pool is the right
// structure because the client is consumed by three concurrent
// goroutines (SSH auth callback, dispatcher authorization, accounting
// worker) and sync.Pool's per-P local cache removes Get/Put contention.
//
// Seeding with poolBufs = 16 covers realistic peak (SSH max-sessions +
// accountant worker) without over-committing memory (16 * 65 547 B =
// ~1 MB). Under that load the pool's New func is practically never
// invoked; it is the last-resort fallback if bursts exceed the seed AND
// the GC has flushed the pool's victim cache in the same window.
const (
	poolBufSize = hdrLen + maxBodyLen // 12 + 65535 = 65547
	poolBufs    = 16
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
//
// When a server supports single-connect (RFC 8907 §4.4, flag 0x04 echoed
// on the first reply), its TCP connection is retained in `pool` and reused
// for subsequent sessions. Request serialization per server is enforced by
// `serverMu[address]`: exactly one send/receive cycle may be in flight on
// any given TACACS+ server at a time, so the shared TCP stream cannot be
// interleaved by concurrent auth / authz / accounting goroutines.
//
// True concurrent multiplexing (multiple in-flight sessions on one TCP,
// demultiplexed by session ID on read) would allow more throughput but is
// not implemented; sequential reuse still saves one handshake per auth
// and matches how Cisco and IOS XR TACACS+ clients behave.
type TacacsClient struct {
	config TacacsClientConfig
	logger *slog.Logger

	poolMu   sync.Mutex
	pool     map[string]net.Conn    // server address -> reusable connection
	serverMu map[string]*sync.Mutex // server address -> I/O serialization mutex
	closed   bool                   // set by Close(); new pool inserts close the conn instead of storing
	bufs     *bufpool.Pool          // pre-allocated wire buffers; used for every read path
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
	return &TacacsClient{
		config:   cfg,
		logger:   logger,
		pool:     make(map[string]net.Conn),
		serverMu: make(map[string]*sync.Mutex),
		bufs:     bufpool.New(poolBufs, poolBufSize, "tacacs"),
	}
}

// lockServer returns the per-server I/O mutex, creating it on first use.
// Callers MUST hold this mutex across the entire send/receive cycle for a
// given server address so concurrent goroutines (auth, authz, accounting)
// never interleave bytes on a pooled TCP connection.
func (c *TacacsClient) lockServer(address string) *sync.Mutex {
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	mu, ok := c.serverMu[address]
	if !ok {
		mu = &sync.Mutex{}
		c.serverMu[address] = mu
	}
	return mu
}

// Close releases every pooled single-connect TCP connection and marks the
// client as shut down so any in-flight request that dials a fresh conn
// after this point will close it instead of re-populating the pool.
// Safe to call multiple times. Callers should invoke this when the AAA
// bundle is replaced so the previous client's connections drain cleanly.
func (c *TacacsClient) Close() {
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	c.closed = true
	for addr, conn := range c.pool {
		if err := conn.Close(); err != nil {
			c.logger.Debug("TACACS+ close pooled connection", "server", addr, "error", err)
		}
		delete(c.pool, addr)
	}
}

// Authenticate performs PAP authentication against TACACS+ servers.
// Tries servers in order. Returns on first response (pass or fail).
// Returns error only on infrastructure failure (all servers unreachable).
//
// Memory: one pool buffer is acquired up front. The request body is
// marshaled directly into buf[hdrLen:] so no per-call allocation
// occurs on the send path. The reply parser (UnmarshalAuthenReply)
// copies the data it cares about into its own struct, so it is safe to
// Put the buffer as soon as parsing returns.
func (c *TacacsClient) Authenticate(username, password, port, remAddr string) (*AuthenReply, error) {
	start := NewPAPAuthenStart(username, password, port, remAddr)

	buf := c.bufs.Get()
	defer c.bufs.Put(buf)

	replyData, err := c.sendToServers(buf, start.MarshalBinaryInto, typeAuthentication, start.Version(), "authentication")
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
	buf := c.bufs.Get()
	defer c.bufs.Put(buf)

	replyData, err := c.sendToServers(buf, req.MarshalBinaryInto, typeAuthorization, 0xC0, "authorization")
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
	buf := c.bufs.Get()
	defer c.bufs.Put(buf)

	replyData, err := c.sendToServers(buf, req.MarshalBinaryInto, typeAccounting, 0xC0, "accounting")
	if err != nil {
		return nil, err
	}

	reply, err := UnmarshalAcctReply(replyData)
	if err != nil {
		return nil, fmt.Errorf("unmarshal acct reply: %w", err)
	}
	return reply, nil
}

// sendToServers marshals a request body into the caller-owned pool buffer
// at buf[hdrLen:] and sends it to TACACS+ servers in order, returning
// the first successful response body. Shared by Authenticate,
// SendAuthorization, and SendAccounting. Re-marshals on every server
// attempt so a previous attempt's in-place Encrypt does not corrupt the
// body on retry.
func (c *TacacsClient) sendToServers(buf []byte, marshalBody func([]byte) (int, error), pktType, version uint8, purpose string) ([]byte, error) {
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
		}

		replyData, sendErr := c.sendReceive(buf, marshalBody, srv, pkt)
		if sendErr != nil {
			c.logger.Warn("TACACS+ server unreachable",
				"purpose", purpose, "server", srv.Address, "error", sendErr)
			continue
		}

		return replyData, nil
	}

	return nil, fmt.Errorf("all TACACS+ servers unreachable for %s", purpose)
}

// sendReceive connects to a server (or reuses a pooled single-connect TCP),
// sends a packet, and reads the response. Returns a slice of `buf` that
// aliases the decrypted response body -- the caller MUST parse it before
// releasing `buf` back to the pool.
//
// Concurrency:
//   - A per-server mutex is held for the entire send/receive so concurrent
//     goroutines (auth callback, dispatcher authorization, accounting worker)
//     cannot interleave bytes on a pooled TCP.
//
// Single-connect handshake (RFC 8907 §4.4):
//   - On a fresh TCP, the client sets FlagSingleConnect on the first packet.
//   - If the server echoes FlagSingleConnect on its reply, the connection is
//     retained in the pool for future sessions; subsequent packets do NOT
//     set the flag.
//   - If the server does not echo the flag, the connection is closed after
//     the exchange and the next session opens a fresh TCP.
//   - A pooled connection that has become dead (read/write failure) is
//     evicted and a fresh dial retried once; persistent failure surfaces
//     to the caller which will try the next server.
func (c *TacacsClient) sendReceive(buf []byte, marshalBody func([]byte) (int, error), srv TacacsServer, pkt *Packet) ([]byte, error) {
	mu := c.lockServer(srv.Address)
	mu.Lock()
	defer mu.Unlock()

	if reply, err := c.trySend(buf, marshalBody, srv, pkt, true); err == nil {
		return reply, nil
	} else if !isPooledConnError(err) {
		return nil, err
	}
	// Pooled connection was dead: trySend already closed and removed it via
	// closeAndEvict before returning pooledConnErr, so retry directly with a
	// fresh dial. The retry must re-run marshalBody because the first
	// trySend's in-place Encrypt XOR'd buf[hdrLen:], leaving it in a state
	// that a second MarshalInto+Encrypt would XOR back to plaintext
	// (double-encrypt == identity for stream ciphers) and put cleartext
	// bytes on the wire.
	return c.trySend(buf, marshalBody, srv, pkt, false)
}

// trySend performs one send/receive cycle using the provided pool buffer.
// Layout within `buf`:
//
//	[0 : hdrLen]               request/response header (written twice)
//	[hdrLen : hdrLen+bodyLen]  body (request written first, then overwritten
//	                            by the response read)
//
// One buffer serves the request marshal, the wire write, the response
// header read, and the response body read. No `make([]byte, N)` anywhere.
// When `allowPool` is true the pooled connection (if any) is used;
// otherwise a fresh TCP is always dialed. The caller retries once after
// eviction; the retry re-runs `marshalBody` so that a previous attempt's
// in-place Encrypt never feeds ciphertext back through MarshalInto's
// second encrypt (XOR is its own inverse).
func (c *TacacsClient) trySend(buf []byte, marshalBody func([]byte) (int, error), srv TacacsServer, pkt *Packet, allowPool bool) ([]byte, error) {
	conn, reused, err := c.acquireConn(srv.Address, allowPool)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", srv.Address, err)
	}

	// On a fresh TCP, set FlagSingleConnect on the first packet so the
	// server can advertise support. On a reused pooled TCP, the flag is
	// omitted -- the server already agreed on the first exchange.
	if !reused {
		pkt.Header.Flags |= FlagSingleConnect
	}

	if deadlineErr := conn.SetDeadline(time.Now().Add(c.config.Timeout)); deadlineErr != nil {
		c.closeAndEvict(srv.Address, conn)
		return nil, fmt.Errorf("set deadline: %w", deadlineErr)
	}

	// Write fresh plaintext body into buf[hdrLen:] on every call. This
	// is the invariant that keeps the retry path correct: the previous
	// attempt's Encrypt has mutated buf[hdrLen:] and must not be fed
	// through MarshalInto again.
	bodyLen, bodyErr := marshalBody(buf[hdrLen:])
	if bodyErr != nil {
		c.closeAndEvict(srv.Address, conn)
		return nil, fmt.Errorf("marshal body: %w", bodyErr)
	}
	pkt.Body = buf[hdrLen : hdrLen+bodyLen]

	// Marshal the full packet into the pool buffer -- header written into
	// buf[:hdrLen] and body encrypted in place via the self-aliased copy.
	wireLen, marshalErr := pkt.MarshalInto(buf, srv.Key)
	if marshalErr != nil {
		c.closeAndEvict(srv.Address, conn)
		return nil, fmt.Errorf("marshal packet: %w", marshalErr)
	}
	if _, writeErr := conn.Write(buf[:wireLen]); writeErr != nil {
		c.closeAndEvict(srv.Address, conn)
		if reused {
			return nil, pooledConnErr{err: writeErr}
		}
		return nil, fmt.Errorf("write: %w", writeErr)
	}

	// Read the response header into the same buffer (overwrites the
	// request bytes that have already been flushed to the socket).
	if _, readErr := io.ReadFull(conn, buf[:hdrLen]); readErr != nil {
		c.closeAndEvict(srv.Address, conn)
		if reused {
			return nil, pooledConnErr{err: readErr}
		}
		return nil, fmt.Errorf("read header: %w", readErr)
	}
	respHdr, hdrErr := UnmarshalPacketHeader(buf[:hdrLen])
	if hdrErr != nil {
		c.closeAndEvict(srv.Address, conn)
		return nil, fmt.Errorf("parse header: %w", hdrErr)
	}

	if respHdr.SessionID != pkt.Header.SessionID {
		c.closeAndEvict(srv.Address, conn)
		return nil, fmt.Errorf("session ID mismatch: sent %x, got %x",
			pkt.Header.SessionID, respHdr.SessionID)
	}
	if respHdr.Length > maxBodyLen {
		c.closeAndEvict(srv.Address, conn)
		return nil, ErrBodyTooBig
	}

	// Read the response body directly into the buffer slot immediately
	// after the header.
	bodyEnd := hdrLen + int(respHdr.Length)
	if _, bodyErr := io.ReadFull(conn, buf[hdrLen:bodyEnd]); bodyErr != nil {
		c.closeAndEvict(srv.Address, conn)
		if reused {
			return nil, pooledConnErr{err: bodyErr}
		}
		return nil, fmt.Errorf("read body: %w", bodyErr)
	}

	if len(srv.Key) > 0 && respHdr.Flags&FlagUnencrypted == 0 {
		Encrypt(buf[hdrLen:bodyEnd], respHdr.SessionID, srv.Key, respHdr.Version, respHdr.SeqNo)
	}

	// Post-exchange connection disposition: if this was a fresh TCP AND
	// the server echoed FlagSingleConnect, promote the conn to the pool;
	// otherwise close it.
	if !reused {
		if respHdr.Flags&FlagSingleConnect != 0 {
			c.storeConn(srv.Address, conn)
		} else {
			if closeErr := conn.Close(); closeErr != nil {
				c.logger.Debug("TACACS+ close non-reusable connection",
					"server", srv.Address, "error", closeErr)
			}
		}
	}
	// When reused == true the connection is already in the pool and stays
	// there; no action needed on the happy path.

	// Return a slice of the pool buffer aliasing the decrypted body. The
	// caller (Authenticate/SendAuthorization/SendAccounting) MUST parse
	// it before its deferred Put releases the buffer.
	return buf[hdrLen:bodyEnd], nil
}

// pooledConnErr marks an error as coming from a reused pooled connection so
// sendReceive can choose to retry with a fresh dial. Never surfaces to
// callers outside this file.
type pooledConnErr struct{ err error }

func (e pooledConnErr) Error() string { return e.err.Error() }

func isPooledConnError(err error) bool {
	var p pooledConnErr
	return errors.As(err, &p)
}

// acquireConn returns a connection to `address`. When `allowPool` is true
// and the pool has a live connection, it is returned with reused=true; the
// caller should not dial or negotiate single-connect again. Otherwise a
// fresh TCP is dialed.
func (c *TacacsClient) acquireConn(address string, allowPool bool) (net.Conn, bool, error) {
	if allowPool {
		c.poolMu.Lock()
		if conn, ok := c.pool[address]; ok {
			c.poolMu.Unlock()
			return conn, true, nil
		}
		c.poolMu.Unlock()
	}
	conn, err := c.dial(address)
	if err != nil {
		return nil, false, err
	}
	return conn, false, nil
}

// storeConn promotes a fresh connection to the pool. If another connection
// is already pooled (rare, from concurrent negotiations) the new one is
// stored and the prior one closed. If the client has already been Close()d
// (for example, a retry after a config-reload-triggered bundle swap), the
// fresh conn is closed instead of being stashed into an abandoned pool
// that nothing will drain.
func (c *TacacsClient) storeConn(address string, conn net.Conn) {
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	if c.closed {
		if err := conn.Close(); err != nil {
			c.logger.Debug("TACACS+ discard post-close connection", "server", address, "error", err)
		}
		return
	}
	if prev, ok := c.pool[address]; ok && prev != conn {
		if err := prev.Close(); err != nil {
			c.logger.Debug("TACACS+ replace pooled connection", "server", address, "error", err)
		}
	}
	c.pool[address] = conn
}

// closeAndEvict removes a pooled connection and closes it. Used on local
// errors (set deadline, marshal) where the connection may still be alive
// but cannot be trusted for subsequent sessions.
func (c *TacacsClient) closeAndEvict(address string, conn net.Conn) {
	c.poolMu.Lock()
	if pooled, ok := c.pool[address]; ok && pooled == conn {
		delete(c.pool, address)
	}
	c.poolMu.Unlock()
	if err := conn.Close(); err != nil {
		c.logger.Debug("TACACS+ close on error", "server", address, "error", err)
	}
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
