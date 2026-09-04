// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS client transport
// Related: packet.go -- packet encode/decode
// Related: dict.go -- packet codes
// RFC: rfc/short/rfc2865.md -- Request Authenticator (Section 4.1), retransmission (Section 2.5)
// RFC: rfc/short/rfc2866.md -- Accounting-Request authenticator (Section 3)
// RFC: rfc/short/rfc2869.md -- Message-Authenticator on a response (Section 5.14)
// RFC: rfc/short/rfc3579.md -- Message-Authenticator signing (Section 3.2), attributes forbidden in accounting (Section 3.3)

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
	// RFC 2865 Section 3: "The secret MUST NOT be empty (length 0) since this
	// would allow packets to be trivially forged." Every RADIUS packet ze sends
	// leaves through here, so this is the one place that has to hold the rule.
	// ExtractConfig (config.go) refuses the empty secret when the operator's
	// configuration is read; this is the paired check at the socket.
	if len(secret) == 0 {
		return nil, fmt.Errorf("radius: empty shared secret for %s", serverAddr)
	}

	buf := Bufs.Get()
	defer Bufs.Put(buf)

	// The instant the client began trying to send this record. RFC 2866
	// Section 5.2 defines Acct-Delay-Time as "how many seconds the client has
	// been trying to send this record", so it is measured from here and not
	// from the event the record describes.
	firstAttempt := time.Now()

	// A record whose author held Acct-Delay-Time back carries no value for this
	// client to update, so it is replayed byte for byte the way an
	// Access-Request is. RFC 2866 Section 4.1 asks for the new Identifier only
	// "if Acct-Delay-Time is included in the attributes of an
	// Accounting-Request".
	stampsAcctDelayTime := pkt.Code == CodeAccountingReq && !pkt.OmitAcctDelayTime
	if stampsAcctDelayTime {
		setAcctDelayTime(pkt, 0)
	}

	n, requestAuth, err := encodeRequest(pkt, secret, buf)
	if err != nil {
		return nil, err
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
	// The key and the waiter MOVE on an accounting retransmission, so the
	// cleanup reads whichever pair is current rather than the first one.
	defer func() { c.unregisterWaiter(key, waiter) }()

	timeout := c.config.Timeout

retryLoop:
	for attempt := range c.config.Retries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// The two request codes want OPPOSITE things from this loop.
		//
		// RFC 2865 Section 2.5 governs an Access-Request: nothing about it
		// changes between attempts, so it is replayed byte for byte and the
		// server can still match the reply. TestAccessRequestRetransmitIsByte-
		// Identical holds that.
		//
		// RFC 2866 Section 4.1 governs an Accounting-Request: "if
		// Acct-Delay-Time is included in the attributes of an Accounting-Request
		// then the Acct-Delay-Time value will be updated when the packet is
		// retransmitted, changing the content of the Attributes field and
		// requiring a new Identifier and Request Authenticator." Section 3 makes
		// that authenticator an MD5 over the attributes, so it moves with them
		// by construction. A constant delay is not a shortcut around this: it
		// reports a number that is never true after the first attempt.
		if attempt > 0 && stampsAcctDelayTime {
			c.unregisterWaiter(key, waiter)
			setAcctDelayTime(pkt, time.Since(firstAttempt))
			pkt.Identifier = c.NextID()
			n, requestAuth, err = encodeRequest(pkt, secret, buf)
			if err != nil {
				return nil, err
			}
			key = responseKey{server: addr.String(), id: pkt.Identifier}
			waiter, err = c.registerWaiter(key, requestAuth, secret)
			if err != nil {
				return nil, err
			}
		}

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

// encodeRequest writes the packet into buf and returns its length and the
// Request Authenticator the reply will be verified against.
//
// An Accounting-Request's authenticator is DERIVED from the bytes rather than
// carried in the packet: RFC 2866 Section 3 computes it over "the Code +
// Identifier + Length + 16 zero octets + request attributes + shared secret",
// so it has to be recomputed whenever any attribute changes.
//
// The Message-Authenticator is signed here, after the encode and before that
// derivation, because RFC 3579 Section 3.2 hashes "the entire Access-Request
// packet, including Type, ID, Length and Authenticator": there is no packet to
// hash until the attributes are laid out. The same section adds that the value
// "is calculated and inserted in the packet before the Response Authenticator
// is calculated", which is the order the two steps run in below.
func encodeRequest(pkt *Packet, secret, buf []byte) (int, [AuthenticatorLen]byte, error) {
	// RFC 3579 Section 3.3: "The EAP-Message and Message-Authenticator
	// attributes specified in this document MUST NOT be present in an
	// Accounting-Request." Every RADIUS packet ze sends leaves through
	// Exchange, so this is the one place that has to hold the rule, and it
	// refuses rather than stripping: a caller that built one of these into an
	// accounting record wants the record it built or an error, never a quietly
	// different record.
	if pkt.Code == CodeAccountingReq {
		for _, a := range pkt.Attrs {
			if a.Type == AttrEAPMessage || a.Type == AttrMessageAuthenticator {
				return 0, [AuthenticatorLen]byte{}, fmt.Errorf(
					"radius: attribute %d is forbidden in an Accounting-Request (RFC 3579 Section 3.3)", a.Type)
			}
		}
	}

	// RFC 3579 Section 3.3 Note 1: "An Access-Request that contains either a
	// User-Password or CHAP-Password or ARAP-Password or one or more
	// EAP-Message attributes MUST NOT contain more than one type of those four
	// attributes." RFC 2865 Section 4.1 states the User-Password/CHAP-Password
	// half on its own, and the authenticator's `credential` selects rather than
	// appends, so this is the paired check at the socket: a builder that ever
	// appended two would be caught here rather than by a permissive server.
	//
	// ARAP-Password is not named. Ze implements no ARAP attribute, which RFC
	// 2869 Section 1.1 requires of a NAS that cannot offer the service, and
	// rfc2869_unoffered_service_attributes_test.go holds it.
	if pkt.Code == CodeAccessRequest {
		if err := oneCredentialType(pkt); err != nil {
			return 0, [AuthenticatorLen]byte{}, err
		}
	}

	wirePkt := prepareWirePacket(pkt, secret)
	n, err := wirePkt.EncodeTo(buf, 0)
	if err != nil {
		return 0, [AuthenticatorLen]byte{}, fmt.Errorf("radius: encode: %w", err)
	}

	signed, err := SignMessageAuthenticator(buf, n, secret)
	if err != nil {
		return 0, [AuthenticatorLen]byte{}, err
	}
	// RFC 3579 Section 3.1: "the Message-Authenticator attribute MUST be used
	// to protect all Access-Request, Access-Challenge, Access-Accept, and
	// Access-Reject packets containing an EAP-Message attribute." The builder
	// appends the placeholder (eapCredential, eap.go); this is the paired check
	// at the socket, so no path can put an unprotected EAP request on the wire.
	if !signed && carriesEAPMessage(pkt) {
		return 0, [AuthenticatorLen]byte{}, errors.New(
			"radius: an EAP-Message packet needs a Message-Authenticator (RFC 3579 Section 3.1)")
	}

	if pkt.Code != CodeAccountingReq {
		return n, pkt.Authenticator, nil
	}
	auth := AccountingRequestAuth(buf, n, secret)
	copy(buf[4:4+AuthenticatorLen], auth[:])
	return n, auth, nil
}

// carriesEAPMessage reports whether the packet encapsulates an EAP packet.
func carriesEAPMessage(pkt *Packet) bool {
	for _, a := range pkt.Attrs {
		if a.Type == AttrEAPMessage {
			return true
		}
	}
	return false
}

// oneCredentialType refuses an Access-Request carrying more than one of the
// credential types RFC 3579 Section 3.3 Note 1 makes exclusive.
//
// The three ze can build are named, and the error names both offenders so the
// builder that appended rather than selected is identifiable from the log.
func oneCredentialType(pkt *Packet) error {
	names := map[uint8]string{
		AttrUserPassword: "User-Password",
		AttrCHAPPassword: "CHAP-Password",
		AttrEAPMessage:   "EAP-Message",
	}
	found := ""
	for _, a := range pkt.Attrs {
		name, credential := names[a.Type]
		if !credential {
			continue
		}
		if found == "" {
			found = name
			continue
		}
		if found != name {
			return fmt.Errorf(
				"radius: an Access-Request carries %s beside %s; RFC 3579 Section 3.3 Note 1 permits one credential type",
				name, found)
		}
	}
	return nil
}

// setAcctDelayTime records how long the client has been trying to send this
// record, replacing any value already present rather than appending a second
// one: RFC 2866 Section 5.13 gives Acct-Delay-Time a count of 0-1.
//
// A sub-second elapsed time is zero seconds, which is the truth on the first
// attempt and is why the attribute is written before the first send as well as
// before each retransmission. What would be false is leaving that zero in place
// on a later attempt.
func setAcctDelayTime(pkt *Packet, elapsed time.Duration) {
	seconds := uint32(0)
	if elapsed > 0 {
		seconds = uint32(elapsed / time.Second)
	}
	value := AttrUint32(seconds)
	for index := range pkt.Attrs {
		if pkt.Attrs[index].Type == AttrAcctDelayTime {
			pkt.Attrs[index].Value = value
			return
		}
	}
	pkt.Attrs = append(pkt.Attrs, Attr{Type: AttrAcctDelayTime, Value: value})
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
		// RFC 2869 Section 5.14: "A RADIUS Client receiving an Access-Accept,
		// Access-Reject or Access-Challenge with a Message-Authenticator
		// Attribute present MUST calculate the correct value of the
		// Message-Authenticator and silently discard the packet if it does not
		// match the value sent."
		if !verifyResponseMessageAuthenticator(data, w.auth, w.secret) {
			c.logger.Warn("radius: bad Message-Authenticator, discarding", "server", key.server)
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

		// RFC 2865 Section 4.1: "The Request Authenticator value MUST be changed
		// each time a new Identifier is used."
		// RFC 2865 Section 2.5: "If any attributes have changed, you MUST use a
		// new Request Authenticator and ID."
		//
		// Failover is not a retransmission: the next server gets a new Identifier
		// on the line above, and the User-Password is re-encoded under that
		// server's own secret, so both sentences apply. prepareWirePacket reads
		// this field inside Exchange, so writing it here also re-derives the
		// User-Password keystream, which is MD5(secret + Request Authenticator).
		auth, err := RandomAuthenticator()
		if err != nil {
			return nil, err
		}
		pkt.Authenticator = auth

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
