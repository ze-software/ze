package reactor

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/wire"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/source"
	"codeberg.org/thomas-mangin/ze/internal/trace"
)

// sessionLogger is the session subsystem logger.
// Controlled by ze.log.bgp.session environment variable.
var sessionLogger = slogutil.Logger("session")

// readBufPool4K provides reusable 4K read buffers for standard messages.
// Used before Extended Message capability is negotiated.
var readBufPool4K = sync.Pool{
	New: func() any {
		return make([]byte, message.MaxMsgLen) // 4096
	},
}

// readBufPool64K provides reusable 64K read buffers for extended messages.
// Used after Extended Message capability is negotiated (RFC 8654).
var readBufPool64K = sync.Pool{
	New: func() any {
		return make([]byte, message.ExtMsgLen) // 65535
	},
}

// ReturnReadBuffer returns a buffer to the appropriate pool based on capacity.
// Used by cache to return buffers when entries are evicted.
func ReturnReadBuffer(buf []byte) {
	if buf == nil {
		return
	}
	if cap(buf) >= message.ExtMsgLen {
		readBufPool64K.Put(buf) //nolint:staticcheck // SA6002: slice in pool is idiomatic Go
	} else {
		readBufPool4K.Put(buf) //nolint:staticcheck // SA6002: slice in pool is idiomatic Go
	}
}

// Session errors.
var (
	ErrNotConnected        = errors.New("not connected")
	ErrAlreadyConnected    = errors.New("already connected")
	ErrInvalidState        = errors.New("invalid FSM state")
	ErrNotificationRecv    = errors.New("notification received")
	ErrConnectionClosed    = errors.New("connection closed")
	ErrHoldTimerExpired    = errors.New("hold timer expired")
	ErrInvalidMessage      = errors.New("invalid message")
	ErrUnsupportedVersion  = errors.New("unsupported BGP version")
	ErrFamilyNotNegotiated = errors.New("address family not negotiated")
	ErrSessionTearingDown  = errors.New("session is tearing down")
)

// Session manages a single BGP peer connection.
//
// It integrates the FSM, timers, and message I/O to drive the BGP
// state machine through the connection lifecycle.

// MessageCallback is called when a BGP message is sent or received.
// peerAddr is the peer's address, msgType is the message type, rawBytes is the body (without header).
// direction is "sent" or "received".
// wireUpdate is non-nil for UPDATE messages (zero-copy), nil for other types.
// ctxID is the encoding context for zero-copy decisions.
// buf is the pool buffer for received messages (nil for sent).
// Returns true if callback took ownership of buf (caller should not return to pool).
type MessageCallback func(peerAddr netip.Addr, msgType message.MessageType, rawBytes []byte, wireUpdate *plugin.WireUpdate, ctxID bgpctx.ContextID, direction string, buf []byte) (kept bool)

type Session struct {
	mu sync.RWMutex

	settings   *PeerSettings
	fsm        *fsm.FSM
	timers     *fsm.Timers
	conn       net.Conn
	negotiated *capability.Negotiated

	// localOpen stores our OPEN for reference during negotiation.
	localOpen *message.Open

	// peerOpen stores the peer's OPEN for reference.
	peerOpen *message.Open

	// extendedMessage tracks if Extended Message capability was negotiated.
	// Thread safety: only accessed from session's read goroutine:
	//   negotiate() ← handleOpen() ← processMessage() ← readAndProcessMessage()
	// No synchronization needed.
	extendedMessage bool

	// Write buffer for zero-allocation message building.
	// Allocated at 4096 bytes initially, resized to 65535 if Extended Message negotiated.
	writeBuf *wire.SessionBuffer

	// Error channel for timer callbacks to signal errors.
	errChan chan error

	// tearingDown is set when Teardown starts, preventing Accept race.
	tearingDown atomic.Bool

	// onMessageReceived is called when any BGP message is received.
	// Set by Peer to forward raw bytes to reactor.
	onMessageReceived MessageCallback

	// recvCtxID is the encoding context for received messages.
	// Set by Peer after capability negotiation for zero-copy WireUpdate creation.
	recvCtxID bgpctx.ContextID

	// sendCtxID is the encoding context for sent messages.
	// Set by Peer after capability negotiation for AttrsWire creation in callbacks.
	sendCtxID bgpctx.ContextID

	// sourceID identifies the peer in the source registry.
	// Set by Peer at creation time.
	sourceID source.SourceID

	// pluginCapGetter retrieves plugin-declared capabilities for OPEN messages.
	// Set by Peer to link to plugin.Server.GetPluginCapabilities().
	// Called in sendOpen() to inject plugin capabilities into OPEN.
	pluginCapGetter func() []capability.Capability
}

// NewSession creates a new BGP session for a peer.
func NewSession(settings *PeerSettings) *Session {
	s := &Session{
		settings: settings,
		fsm:      fsm.New(),
		timers:   fsm.NewTimers(),
		writeBuf: wire.NewSessionBuffer(false), // Start with 4096, resize if Extended Message
		errChan:  make(chan error, 2),          // Buffer 2: normal error + teardown
	}

	// Configure FSM passive mode.
	s.fsm.SetPassive(settings.Passive)

	// Configure timers.
	s.timers.SetHoldTime(settings.HoldTime)

	// Wire up timer callbacks.
	s.timers.OnHoldTimerExpires(func() {
		s.mu.Lock()
		_ = s.fsm.Event(fsm.EventHoldTimerExpires)
		s.mu.Unlock()
		select {
		case s.errChan <- ErrHoldTimerExpired:
		default:
		}
	})

	s.timers.OnKeepaliveTimerExpires(func() {
		s.mu.Lock()
		conn := s.conn
		s.mu.Unlock()

		if conn != nil {
			_ = s.sendKeepalive(conn)
		}
	})

	return s
}

// State returns the current FSM state.
func (s *Session) State() fsm.State {
	return s.fsm.State()
}

// Conn returns the current connection (nil if not connected).
func (s *Session) Conn() net.Conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conn
}

// Negotiated returns the negotiated capabilities (nil until OPENCONFIRM).
func (s *Session) Negotiated() *capability.Negotiated {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.negotiated
}

// SetRecvCtxID sets the encoding context ID for received messages.
// Called by Peer after capability negotiation for zero-copy WireUpdate creation.
func (s *Session) SetRecvCtxID(ctxID bgpctx.ContextID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recvCtxID = ctxID
}

// SetSendCtxID sets the encoding context ID for sent messages.
// Called by Peer after capability negotiation for AttrsWire creation in callbacks.
func (s *Session) SetSendCtxID(ctxID bgpctx.ContextID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendCtxID = ctxID
}

// SetSourceID sets the source ID identifying this peer.
// Called by Peer at creation time.
func (s *Session) SetSourceID(id source.SourceID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sourceID = id
}

// SetPluginCapabilityGetter sets the callback for retrieving plugin capabilities.
// Called by Peer at creation time to link to plugin.Server.GetPluginCapabilities().
func (s *Session) SetPluginCapabilityGetter(getter func() []capability.Capability) {
	s.pluginCapGetter = getter
}

// WriteBuf returns the session's write buffer for zero-allocation message building.
// The buffer is sized based on negotiated Extended Message capability.
func (s *Session) WriteBuf() *wire.SessionBuffer {
	return s.writeBuf
}

// getReadBuffer gets an appropriately-sized buffer from pool.
// Uses 4K pool before Extended Message negotiation, 64K after.
func (s *Session) getReadBuffer() []byte {
	if s.extendedMessage {
		if buf, ok := readBufPool64K.Get().([]byte); ok {
			return buf
		}
		return make([]byte, message.ExtMsgLen)
	}
	if buf, ok := readBufPool4K.Get().([]byte); ok {
		return buf
	}
	return make([]byte, message.MaxMsgLen)
}

// returnReadBuffer returns buffer to the appropriate pool based on size.
func (s *Session) returnReadBuffer(buf []byte) {
	if cap(buf) >= message.ExtMsgLen {
		readBufPool64K.Put(buf) //nolint:staticcheck // SA6002: slice in pool is idiomatic Go
	} else {
		readBufPool4K.Put(buf) //nolint:staticcheck // SA6002: slice in pool is idiomatic Go
	}
}

// DetectCollision checks if an incoming connection causes a collision.
// RFC 4271 §6.8 - BGP Connection Collision Detection.
//
// Returns (shouldAccept, shouldCloseExisting):
//   - shouldAccept: true if the new connection should be accepted
//   - shouldCloseExisting: true if the existing connection should be closed
//
// The collision resolution algorithm:
//   - ESTABLISHED: always reject new connection
//   - OPENCONFIRM: compare BGP IDs as uint32
//   - If local_id < remote_id: accept new, close existing
//   - If local_id >= remote_id: reject new, keep existing
//   - Other states: accept new (no collision detection possible)
func (s *Session) DetectCollision(remoteBGPID uint32) (shouldAccept, shouldCloseExisting bool) {
	state := s.fsm.State()

	switch state {
	case fsm.StateEstablished:
		// RFC 4271 §6.8: "collision with existing BGP connection that is in
		// the Established state causes closing of the newly created connection"
		return false, false

	case fsm.StateOpenConfirm:
		// RFC 4271 §6.8: "Upon receipt of an OPEN message, the local system
		// MUST examine all of its connections that are in the OpenConfirm state"
		localID := s.settings.RouterID

		// RFC 4271 §6.8: "Comparing BGP Identifiers is done by converting them
		// to host byte order and treating them as 4-octet unsigned integers"
		if localID < remoteBGPID {
			// RFC 4271 §6.8: "If the value of the local BGP Identifier is less
			// than the remote one, the local system closes the BGP connection
			// that already exists and accepts the BGP connection initiated by
			// the remote system"
			return true, true
		}
		// RFC 4271 §6.8: "Otherwise, the local system closes the newly created
		// BGP connection and continues to use the existing one"
		return false, false

	case fsm.StateIdle, fsm.StateConnect, fsm.StateActive, fsm.StateOpenSent:
		// RFC 4271 §6.8: "a connection collision cannot be detected with
		// connections that are in Idle, Connect, or Active states"
		// OpenSent MAY detect if BGP ID known by other means - we don't implement this
		return true, false
	}
	// Unreachable, but required for exhaustive switch
	return true, false
}

// Start triggers the ManualStart event to begin the connection process.
func (s *Session) Start() error {
	return s.fsm.Event(fsm.EventManualStart)
}

// Stop triggers the ManualStop event.
func (s *Session) Stop() error {
	s.timers.StopAll()
	return s.fsm.Event(fsm.EventManualStop)
}

// Connect initiates an outgoing TCP connection.
// If LocalAddress is configured, binds to it for outgoing connections.
// This ensures consistent source address for next-hop self resolution.
func (s *Session) Connect(ctx context.Context) error {
	s.mu.Lock()
	if s.conn != nil {
		s.mu.Unlock()
		return ErrAlreadyConnected
	}
	s.mu.Unlock()

	addr := net.JoinHostPort(s.settings.Address.String(), fmt.Sprintf("%d", s.settings.Port))

	d := net.Dialer{}

	// Bind to local address if configured
	if s.settings.LocalAddress.IsValid() {
		d.LocalAddr = &net.TCPAddr{IP: s.settings.LocalAddress.AsSlice()}
	}

	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		_ = s.fsm.Event(fsm.EventTCPConnectionFails)
		return fmt.Errorf("connect to %s: %w", addr, err)
	}

	return s.connectionEstablished(conn)
}

// Accept accepts an incoming TCP connection.
func (s *Session) Accept(conn net.Conn) error {
	// Check if session is being torn down - reject if so.
	// This prevents accepting a connection on a session that's about to exit.
	if s.tearingDown.Load() {
		return ErrSessionTearingDown
	}

	s.mu.Lock()
	if s.conn != nil {
		s.mu.Unlock()
		return ErrAlreadyConnected
	}
	s.mu.Unlock()

	// Drain any stale errors from previous teardown.
	// If a teardown was queued but a new connection arrives before Run() exits,
	// the old ErrTeardown would incorrectly terminate the new session.
	// Drain all buffered errors (channel has buffer size 2).
drainLoop:
	for {
		select {
		case <-s.errChan:
		default:
			break drainLoop
		}
	}

	// Reset tearing down flag for new connection.
	// This allows the session to be reused after a teardown.
	s.tearingDown.Store(false)

	err := s.connectionEstablished(conn)
	if err != nil {
		return err
	}

	// Drain errChan again after connection setup.
	// A concurrent Teardown() may have sent ErrTeardown between our first
	// drain and connectionEstablished(). This second drain catches it.
drainLoop2:
	for {
		select {
		case <-s.errChan:
		default:
			break drainLoop2
		}
	}

	return nil
}

// AcceptWithOpen accepts a connection and processes a pre-received OPEN.
// RFC 4271 §6.8: Used for collision resolution when we've already read the peer's OPEN.
func (s *Session) AcceptWithOpen(conn net.Conn, peerOpen *message.Open) error {
	s.mu.Lock()
	if s.conn != nil {
		s.mu.Unlock()
		return ErrAlreadyConnected
	}
	s.mu.Unlock()

	// Establish connection (sends our OPEN)
	if err := s.connectionEstablished(conn); err != nil {
		return err
	}

	// Process the pre-received OPEN
	return s.processOpen(peerOpen)
}

// processOpen handles a pre-parsed OPEN message.
// Used by AcceptWithOpen for collision resolution.
func (s *Session) processOpen(open *message.Open) error {
	// Validate version
	if open.Version != 4 {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		_ = s.sendNotification(conn,
			message.NotifyOpenMessage,
			message.NotifyOpenUnsupportedVersion,
			[]byte{4},
		)
		_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return ErrUnsupportedVersion
	}

	// Validate hold time
	if err := open.ValidateHoldTime(); err != nil {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		var notif *message.Notification
		if errors.As(err, &notif) {
			_ = s.sendNotification(conn, notif.ErrorCode, notif.ErrorSubcode, notif.Data)
		}
		_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return fmt.Errorf("invalid hold time %d: %w", open.HoldTime, err)
	}

	s.mu.Lock()
	s.peerOpen = open
	s.mu.Unlock()

	// Negotiate capabilities
	s.negotiate()

	// Validate required families
	s.mu.RLock()
	conn := s.conn
	neg := s.negotiated
	requiredFamilies := s.settings.RequiredFamilies
	s.mu.RUnlock()

	if len(requiredFamilies) > 0 && neg != nil {
		if missing := neg.CheckRequired(requiredFamilies); len(missing) > 0 {
			capData := buildUnsupportedCapabilityData(missing)
			_ = s.sendNotification(conn,
				message.NotifyOpenMessage,
				message.NotifyOpenUnsupportedCapability,
				capData,
			)
			_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
			s.closeConn()
			return fmt.Errorf("%w: required families not negotiated: %v", ErrInvalidState, missing)
		}
	}

	// Update FSM
	if err := s.fsm.Event(fsm.EventBGPOpen); err != nil {
		return err
	}

	// Send KEEPALIVE
	if err := s.sendKeepalive(conn); err != nil {
		return err
	}

	// Reset hold timer
	s.timers.ResetHoldTimer()

	return nil
}

// connectionEstablished handles a new TCP connection (incoming or outgoing).
func (s *Session) connectionEstablished(conn net.Conn) error {
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()

	// Signal FSM.
	if err := s.fsm.Event(fsm.EventTCPConnectionConfirmed); err != nil {
		return err
	}

	// Send OPEN message.
	if err := s.sendOpen(conn); err != nil {
		s.closeConn()
		return err
	}

	// Start hold timer (for waiting for peer's OPEN).
	s.timers.StartHoldTimer()

	return nil
}

// Close gracefully closes the session.
func (s *Session) Close() error {
	s.timers.StopAll()

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()

	if conn != nil {
		// Send NOTIFICATION (Cease/Administrative Shutdown).
		_ = s.sendNotification(conn,
			message.NotifyCease,
			message.NotifyCeaseAdminShutdown,
			nil,
		)
	}

	s.closeConn()
	_ = s.fsm.Event(fsm.EventManualStop)

	return nil
}

// CloseWithNotification closes the session with a specific NOTIFICATION.
// RFC 4271 §6.8: Used for collision detection to close with Cease/Connection Collision.
func (s *Session) CloseWithNotification(code message.NotifyErrorCode, subcode uint8) error {
	s.timers.StopAll()

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()

	if conn != nil {
		_ = s.sendNotification(conn, code, subcode, nil)
	}

	s.closeConn()
	_ = s.fsm.Event(fsm.EventManualStop)

	return nil
}

// ErrTeardown is returned when the session is torn down via API.
var ErrTeardown = errors.New("session teardown")

// Teardown sends a Cease NOTIFICATION with the given subcode and closes.
// RFC 4486 defines Cease subcodes: 1=MaxPrefixes, 2=AdminShutdown, 3=PeerDeconfigured,
// 4=AdminReset, 5=ConnectionRejected, 6=OtherConfigChange, 7=Collision, 8=OutOfResources.
// RFC 9003 specifies that subcodes 2/4 include a length-prefixed message.
func (s *Session) Teardown(subcode uint8) error {
	// Mark session as tearing down to prevent accepting new connections
	s.tearingDown.Store(true)

	s.timers.StopAll()

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()

	if conn != nil {
		// Build data per RFC 9003: length byte + message for subcodes 2/4
		var data []byte
		msg := message.CeaseSubcodeString(subcode)
		if subcode == message.NotifyCeaseAdminShutdown || subcode == message.NotifyCeaseAdminReset {
			// RFC 9003: length byte + UTF-8 message
			data = make([]byte, 1+len(msg))
			data[0] = byte(len(msg))
			copy(data[1:], msg)
		}

		_ = s.sendNotification(conn,
			message.NotifyCease,
			subcode,
			data,
		)
	}

	s.closeConn()
	_ = s.fsm.Event(fsm.EventManualStop)

	// Signal the Run loop to exit
	select {
	case s.errChan <- ErrTeardown:
	default:
		// Channel full or closed, that's ok
	}

	return nil
}

// closeConn closes the TCP connection.
func (s *Session) closeConn() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}

// Run is the main session loop. It processes messages until context is
// cancelled or an error occurs.
func (s *Session) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-s.errChan:
			s.closeConn()
			return err
		default:
		}

		// Check if we have a connection.
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		if conn == nil {
			// No connection, nothing to do.
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// Set read deadline to allow context checking.
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		err := s.readAndProcessMessage(conn)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) ||
				isTimeout(err) {
				continue // Timeout, check context and retry.
			}
			return err
		}
	}
}

// ReadAndProcess reads and processes a single message.
// Exposed for testing.
func (s *Session) ReadAndProcess() error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return ErrNotConnected
	}

	// Set read deadline.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	return s.readAndProcessMessage(conn)
}

// readAndProcessMessage reads a message from the connection and processes it.
// Uses clean get/return pool pattern for buffer lifecycle.
func (s *Session) readAndProcessMessage(conn net.Conn) error {
	// Get buffer from pool
	buf := s.getReadBuffer()

	// Read header
	_, err := io.ReadFull(conn, buf[:message.HeaderLen])
	if err != nil {
		s.returnReadBuffer(buf)
		// Handle connection close: EOF or connection reset by peer
		if errors.Is(err, io.EOF) || isConnectionReset(err) {
			s.handleConnectionClose()
			return ErrConnectionClosed
		}
		return err
	}

	hdr, err := message.ParseHeader(buf[:message.HeaderLen])
	if err != nil {
		s.returnReadBuffer(buf)
		_ = s.fsm.Event(fsm.EventBGPHeaderErr)
		return fmt.Errorf("parse header: %w", err)
	}

	// RFC 8654: Validate message length against max (4096 or 65535 if extended).
	if err := hdr.ValidateLengthWithMax(s.extendedMessage); err != nil {
		s.returnReadBuffer(buf)
		// RFC 8654 Section 5: Send NOTIFICATION with Bad Message Length.
		_ = s.sendNotification(conn,
			message.NotifyMessageHeader,
			message.NotifyHeaderBadLength,
			[]byte{byte(hdr.Length >> 8), byte(hdr.Length)},
		)
		_ = s.fsm.Event(fsm.EventBGPHeaderErr)
		s.closeConn()
		return fmt.Errorf("message length %d exceeds max for %s: %w", hdr.Length, hdr.Type, err)
	}

	// Read body
	bodyLen := int(hdr.Length) - message.HeaderLen
	if bodyLen > 0 {
		_, err = io.ReadFull(conn, buf[message.HeaderLen:hdr.Length])
		if err != nil {
			s.returnReadBuffer(buf)
			return fmt.Errorf("read body: %w", err)
		}
	}

	// Process message - callback returns kept=true if it took buffer ownership
	err, kept := s.processMessage(&hdr, buf[message.HeaderLen:hdr.Length], buf)

	// Return buffer to pool only if callback didn't keep it
	if !kept {
		s.returnReadBuffer(buf)
	}

	return err
}

// processMessage handles a received BGP message.
// Returns (error, kept) where kept indicates if callback took buffer ownership.
func (s *Session) processMessage(hdr *message.Header, body []byte, buf []byte) (error, bool) {
	s.mu.RLock()
	ctxID := s.recvCtxID
	sourceID := s.sourceID
	s.mu.RUnlock()

	// For UPDATE: create WireUpdate once, use for callback and handler
	var wireUpdate *plugin.WireUpdate
	if hdr.Type == message.TypeUPDATE {
		wireUpdate = plugin.NewWireUpdate(body, ctxID)
		wireUpdate.SetSourceID(sourceID)
	}

	// Notify callback for all message types.
	// Callback returns true if it took ownership of buf (e.g., cached it).
	var kept bool
	if s.onMessageReceived != nil {
		kept = s.onMessageReceived(s.settings.Address, hdr.Type, body, wireUpdate, ctxID, "received", buf)
	}

	var err error
	switch hdr.Type { //nolint:exhaustive // unknown in default
	case message.TypeUPDATE:
		err = s.handleUpdate(wireUpdate)
	case message.TypeOPEN:
		err = s.handleOpen(body)
	case message.TypeKEEPALIVE:
		err = s.handleKeepalive()
	case message.TypeNOTIFICATION:
		err = s.handleNotification(body)
	case message.TypeROUTEREFRESH:
		err = s.handleRouteRefresh(body)
	default:
		err = s.handleUnknownType(hdr.Type)
	}
	return err, kept
}

// handleUnknownType handles unknown message types (exabgp-compatible).
func (s *Session) handleUnknownType(msgType message.MessageType) error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	// ExaBGP format: Message Header Error (1), subcode 0, text message.
	errMsg := fmt.Sprintf("can not decode update message of type \"%d\"", msgType)
	_ = s.sendNotification(conn,
		message.NotifyMessageHeader,
		0, // ExaBGP uses subcode 0
		[]byte(errMsg),
	)
	_ = s.fsm.Event(fsm.EventBGPHeaderErr)
	s.closeConn()

	return fmt.Errorf("%w: unknown type %d", ErrInvalidMessage, msgType)
}

// handleOpen processes a received OPEN message.
func (s *Session) handleOpen(body []byte) error {
	open, err := message.UnpackOpen(body)
	if err != nil {
		_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
		return fmt.Errorf("unpack OPEN: %w", err)
	}

	// Validate version.
	if open.Version != 4 {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		_ = s.sendNotification(conn,
			message.NotifyOpenMessage,
			message.NotifyOpenUnsupportedVersion,
			[]byte{4}, // We support version 4
		)
		_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return ErrUnsupportedVersion
	}

	// RFC 4271 Section 6.2: "An implementation MUST reject Hold Time values
	// of one or two seconds."
	if err := open.ValidateHoldTime(); err != nil {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		// Send NOTIFICATION with the error (already a *Notification).
		var notif *message.Notification
		if errors.As(err, &notif) {
			_ = s.sendNotification(conn, notif.ErrorCode, notif.ErrorSubcode, notif.Data)
		}
		_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return fmt.Errorf("invalid hold time %d: %w", open.HoldTime, err)
	}

	s.mu.Lock()
	s.peerOpen = open
	s.mu.Unlock()

	// Negotiate capabilities.
	s.negotiate()

	// Validate required families are negotiated.
	s.mu.RLock()
	conn := s.conn
	neg := s.negotiated
	requiredFamilies := s.settings.RequiredFamilies
	s.mu.RUnlock()

	if len(requiredFamilies) > 0 && neg != nil {
		if missing := neg.CheckRequired(requiredFamilies); len(missing) > 0 {
			// Required families not negotiated - send NOTIFICATION and reject.
			// RFC 5492 Section 3: Use Unsupported Capability subcode.
			capData := buildUnsupportedCapabilityData(missing)
			_ = s.sendNotification(conn,
				message.NotifyOpenMessage,
				message.NotifyOpenUnsupportedCapability,
				capData,
			)
			_ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
			s.closeConn()
			return fmt.Errorf("%w: required families not negotiated: %v", ErrInvalidState, missing)
		}
	}

	// Update FSM.
	if err := s.fsm.Event(fsm.EventBGPOpen); err != nil {
		return err
	}

	// Send KEEPALIVE to confirm.
	if err := s.sendKeepalive(conn); err != nil {
		return err
	}

	// Reset and restart hold timer with negotiated value.
	s.timers.ResetHoldTimer()

	return nil
}

// handleKeepalive processes a received KEEPALIVE message.
func (s *Session) handleKeepalive() error {
	// Reset hold timer.
	s.timers.ResetHoldTimer()

	state := s.fsm.State()
	if state == fsm.StateOpenConfirm {
		// Start keepalive timer for sending our keepalives.
		s.timers.StartKeepaliveTimer()
	}

	return s.fsm.Event(fsm.EventKeepaliveMsg)
}

// handleUpdate processes a received UPDATE message.
// RFC 4760 Section 6: validates AFI/SAFI in MP_REACH/MP_UNREACH against negotiated.
// RFC 7606: validates path attributes with revised error handling.
// Accepts WireUpdate for zero-copy processing.
func (s *Session) handleUpdate(wu *plugin.WireUpdate) error {
	// Reset hold timer.
	s.timers.ResetHoldTimer()

	// Get raw payload for validation (zero-copy slice)
	body := wu.Payload()

	// Validate address families in UPDATE.
	if err := s.validateUpdateFamilies(body); err != nil {
		return err
	}

	// RFC 7606: Validate path attributes with revised error handling.
	if err := s.validateUpdateRFC7606(body); err != nil {
		return err
	}

	return s.fsm.Event(fsm.EventUpdateMsg)
}

// validateUpdateRFC7606 performs RFC 7606 attribute validation.
// Returns nil for treat-as-withdraw (session stays up), error for session reset.
func (s *Session) validateUpdateRFC7606(body []byte) error {
	// Parse UPDATE structure
	if len(body) < 4 {
		return nil // Let other validation handle
	}

	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	offset := 2 + withdrawnLen
	if offset+2 > len(body) {
		return nil
	}

	// RFC 7606 Section 5.3: Validate withdrawn routes NLRI syntax (IPv4)
	if withdrawnLen > 0 {
		withdrawn := body[2 : 2+withdrawnLen]
		if result := message.ValidateNLRISyntax(withdrawn, false); result != nil {
			trace.RFC7606TreatAsWithdraw(0, result.Description)
			return nil // treat-as-withdraw
		}
	}

	attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if offset+attrLen > len(body) {
		return nil
	}

	pathAttrs := body[offset : offset+attrLen]
	nlriLen := len(body) - (offset + attrLen)
	hasNLRI := nlriLen > 0

	// RFC 7606 Section 5.3: Validate NLRI syntax (IPv4)
	if nlriLen > 0 {
		nlri := body[offset+attrLen:]
		if result := message.ValidateNLRISyntax(nlri, false); result != nil {
			trace.RFC7606TreatAsWithdraw(0, result.Description)
			return nil // treat-as-withdraw
		}
	}

	// Validate path attributes per RFC 7606
	// IBGP is determined by comparing local and peer AS numbers
	isIBGP := s.settings.LocalAS == s.settings.PeerAS
	// Get asn4 from negotiated capabilities (defaults to false if not negotiated)
	asn4 := false
	if neg := s.Negotiated(); neg != nil {
		asn4 = neg.ASN4
	}
	result := message.ValidateUpdateRFC7606(pathAttrs, hasNLRI, isIBGP, asn4)

	switch result.Action {
	case message.RFC7606ActionNone:
		// No error, continue normally
		return nil

	case message.RFC7606ActionTreatAsWithdraw:
		// RFC 7606: Log and continue (routes treated as withdrawn)
		trace.RFC7606TreatAsWithdraw(result.AttrCode, result.Description)
		return nil // Session stays up

	case message.RFC7606ActionAttributeDiscard:
		// RFC 7606: Log and continue (malformed attribute discarded)
		trace.RFC7606AttributeDiscard(result.AttrCode, result.Description)
		return nil // Session stays up

	case message.RFC7606ActionSessionReset:
		// RFC 7606: Session reset required (e.g., multiple MP_REACH)
		trace.RFC7606SessionReset(result.Description)
		return fmt.Errorf("RFC 7606 session reset: %s", result.Description)
	}

	return nil
}

// validateUpdateFamilies checks that AFI/SAFI in MP_REACH/MP_UNREACH were negotiated.
// RFC 4760 Section 6: "If a BGP speaker receives an UPDATE with MP_REACH_NLRI or
// MP_UNREACH_NLRI where the AFI/SAFI do not match those negotiated in OPEN,
// the speaker MAY treat this as an error.".
func (s *Session) validateUpdateFamilies(body []byte) error {
	// Need at least 4 bytes: withdrawn len (2) + attrs len (2)
	if len(body) < 4 {
		return nil // Let message parsing handle malformed
	}

	// Skip withdrawn routes
	withdrawnLen := binary.BigEndian.Uint16(body[0:2])
	offset := 2 + int(withdrawnLen)
	if offset+2 > len(body) {
		return nil
	}

	// Get path attributes
	attrLen := binary.BigEndian.Uint16(body[offset : offset+2])
	offset += 2
	if offset+int(attrLen) > len(body) {
		return nil
	}
	pathAttrs := body[offset : offset+int(attrLen)]

	// Parse path attributes looking for MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15)
	pos := 0
	for pos < len(pathAttrs) {
		if pos+2 > len(pathAttrs) {
			break
		}

		flags := pathAttrs[pos]
		code := attribute.AttributeCode(pathAttrs[pos+1])
		pos += 2

		// Determine length (1 or 2 bytes based on extended length flag)
		var attrDataLen int
		if flags&0x10 != 0 { // Extended length
			if pos+2 > len(pathAttrs) {
				break
			}
			attrDataLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			pos += 2
		} else {
			if pos+1 > len(pathAttrs) {
				break
			}
			attrDataLen = int(pathAttrs[pos])
			pos++
		}

		if pos+attrDataLen > len(pathAttrs) {
			break
		}

		attrData := pathAttrs[pos : pos+attrDataLen]
		pos += attrDataLen

		// Check MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15)
		if code == attribute.AttrMPReachNLRI || code == attribute.AttrMPUnreachNLRI {
			if len(attrData) < 3 {
				continue // Malformed, let other validation catch it
			}

			afi := capability.AFI(binary.BigEndian.Uint16(attrData[0:2]))
			safi := capability.SAFI(attrData[2])
			family := capability.Family{AFI: afi, SAFI: safi}

			neg := s.Negotiated()
			if neg != nil && !neg.SupportsFamily(family) {
				// Family not negotiated - check if we should ignore
				shouldIgnore := s.settings.IgnoreFamilyMismatch || s.shouldIgnoreFamily(family)
				if shouldIgnore {
					// Lenient mode: log warning and skip
					trace.UpdateFamilyMismatch(uint16(afi), uint8(safi), true)
				} else {
					// Strict mode: return error
					trace.UpdateFamilyMismatch(uint16(afi), uint8(safi), false)
					return fmt.Errorf("%w: %s", ErrFamilyNotNegotiated, family)
				}
			}
		}
	}

	return nil
}

// handleNotification processes a received NOTIFICATION message.
func (s *Session) handleNotification(body []byte) error {
	notif, err := message.UnpackNotification(body)
	if err != nil {
		_ = s.fsm.Event(fsm.EventNotifMsgVerErr)
		return fmt.Errorf("unpack NOTIFICATION: %w", err)
	}

	s.timers.StopAll()
	_ = s.fsm.Event(fsm.EventNotifMsg)
	s.closeConn()

	return fmt.Errorf("%w: %s", ErrNotificationRecv, notif.String())
}

// handleRouteRefresh processes a received ROUTE-REFRESH message.
// RFC 2918: Base Route Refresh capability.
// RFC 7313: Enhanced Route Refresh with BoRR/EoRR markers.
func (s *Session) handleRouteRefresh(body []byte) error {
	// RFC 7313 Section 5: "If the length... is not 4, then the BGP speaker
	// MUST send a NOTIFICATION message with Error Code 'ROUTE-REFRESH Message Error'
	// and subcode 'Invalid Message Length'."
	if len(body) != 4 {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		_ = s.sendNotification(conn,
			message.NotifyRouteRefresh,
			message.NotifyRouteRefreshInvalidLength,
			body,
		)
		_ = s.fsm.Event(fsm.EventBGPHeaderErr)
		s.closeConn()
		return fmt.Errorf("%w: ROUTE-REFRESH invalid length %d", ErrInvalidMessage, len(body))
	}

	rr, err := message.UnpackRouteRefresh(body)
	if err != nil {
		return fmt.Errorf("unpack ROUTE-REFRESH: %w", err)
	}

	// RFC 7313 Section 5: "When the BGP speaker receives a ROUTE-REFRESH message
	// with a 'Message Subtype' field other than 0, 1, or 2, it MUST ignore
	// the received ROUTE-REFRESH message."
	if rr.Subtype > 2 && rr.Subtype != 255 {
		trace.Log(trace.Session, "peer %s: ignoring unknown route-refresh subtype %d",
			s.settings.Address, rr.Subtype)
		return nil
	}

	// Subtype 255 is reserved - also ignore
	if rr.Subtype == 255 {
		trace.Log(trace.Session, "peer %s: ignoring reserved route-refresh subtype 255",
			s.settings.Address)
		return nil
	}

	// Valid subtypes 0, 1, 2 are handled via onMessageReceived callback
	// which already forwarded the message to the API before this handler runs.
	// No additional action needed here - the API processes refresh/borr/eorr events.
	return nil
}

// handleConnectionClose handles TCP connection close.
func (s *Session) handleConnectionClose() {
	s.timers.StopAll()
	_ = s.fsm.Event(fsm.EventTCPConnectionFails)
	s.closeConn()
}

// isConnectionReset checks if an error is a connection reset by peer.
// This happens when the remote side closes the connection abruptly.
func isConnectionReset(err error) bool {
	if err == nil {
		return false
	}
	// Check for common connection close errors
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	// Check error message for connection reset indicators
	errStr := err.Error()
	return strings.Contains(errStr, "connection reset by peer") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "use of closed network connection")
}

// negotiate performs capability negotiation between local and peer OPEN.
func (s *Session) negotiate() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.localOpen == nil || s.peerOpen == nil {
		return
	}

	// Parse capabilities from OPEN messages.
	localCaps := parseCapabilities(s.localOpen.OptionalParams)
	peerCaps := parseCapabilities(s.peerOpen.OptionalParams)

	// Negotiate.
	s.negotiated = capability.Negotiate(
		localCaps,
		peerCaps,
		s.settings.LocalAS,
		s.peerOpen.ASN4,
	)

	// RFC 8654: If extended message is negotiated, track for pool selection.
	// MUST be capable of receiving/sending messages up to 65535 octets.
	if s.negotiated.ExtendedMessage {
		s.extendedMessage = true
		s.writeBuf.Resize(true) // Expand to 65535 if needed
	}

	// Negotiate hold time (minimum of both, but at least 3s if non-zero).
	localHold := s.settings.HoldTime
	peerHold := time.Duration(s.peerOpen.HoldTime) * time.Second

	var negotiatedHold time.Duration
	if localHold == 0 || peerHold == 0 {
		negotiatedHold = 0
	} else {
		negotiatedHold = localHold
		if peerHold < negotiatedHold {
			negotiatedHold = peerHold
		}
		// RFC 4271: Minimum hold time is 3 seconds if non-zero.
		if negotiatedHold > 0 && negotiatedHold < 3*time.Second {
			negotiatedHold = 3 * time.Second
		}
	}

	s.negotiated.HoldTime = uint16(negotiatedHold / time.Second) //nolint:gosec // Hold time max 65535s
	s.timers.SetHoldTime(negotiatedHold)
}

// sendOpen sends an OPEN message.
func (s *Session) sendOpen(conn net.Conn) error {
	// Build capabilities in RFC-expected order:
	// 1. Multiprotocol (from settings.Capabilities)
	// 2. ASN4
	// 3. Other capabilities (FQDN, SoftwareVersion, etc.)
	// 4. Plugin-declared capabilities
	var caps []capability.Capability
	var otherCaps []capability.Capability

	// Separate Multiprotocol capabilities from others.
	for _, c := range s.settings.Capabilities {
		if c.Code() == capability.CodeMultiprotocol {
			caps = append(caps, c)
		} else {
			otherCaps = append(otherCaps, c)
		}
	}

	// Add ASN4 unless disabled in config.
	if !s.settings.DisableASN4 {
		caps = append(caps, &capability.ASN4{ASN: s.settings.LocalAS})
	}

	// Add remaining capabilities.
	caps = append(caps, otherCaps...)

	// Add plugin-declared capabilities (e.g., hostname from RFC 9234 plugin).
	if s.pluginCapGetter != nil {
		caps = append(caps, s.pluginCapGetter()...)
	}

	// Build optional parameters (capabilities).
	optParams := buildOptionalParams(caps)

	// Determine AS to put in header (AS_TRANS if > 65535).
	myAS := uint16(s.settings.LocalAS) //nolint:gosec // Truncation intended for AS_TRANS
	if s.settings.LocalAS > 65535 {
		myAS = 23456 // AS_TRANS
	}

	open := &message.Open{
		Version:        4,
		MyAS:           myAS,
		HoldTime:       uint16(s.settings.HoldTime / time.Second), //nolint:gosec // Hold time max 65535s
		BGPIdentifier:  s.settings.RouterID,
		ASN4:           s.settings.LocalAS,
		OptionalParams: optParams,
	}

	s.mu.Lock()
	s.localOpen = open
	s.mu.Unlock()

	return s.writeMessage(conn, open)
}

// sendKeepalive sends a KEEPALIVE message.
func (s *Session) sendKeepalive(conn net.Conn) error {
	return s.writeMessage(conn, message.NewKeepalive())
}

// sendNotification sends a NOTIFICATION message.
func (s *Session) sendNotification(conn net.Conn, code message.NotifyErrorCode, subcode uint8, data []byte) error {
	notif := &message.Notification{
		ErrorCode:    code,
		ErrorSubcode: subcode,
		Data:         data,
	}
	return s.writeMessage(conn, notif)
}

// writeMessage packs and sends a BGP message.
func (s *Session) writeMessage(conn net.Conn, msg message.Message) error {
	if conn == nil {
		return ErrNotConnected
	}

	// Use WriteTo via PackTo helper. Message types don't use context for
	// basic encoding (KEEPALIVE, OPEN, NOTIFICATION have fixed formats).
	// UPDATE messages have wire bytes pre-built.
	data := message.PackTo(msg, nil)

	_, err := conn.Write(data)
	if err != nil {
		return err
	}

	// Notify callback after successful send.
	// Body is data after the 19-byte header (16-byte marker + 2-byte length + 1-byte type).
	if s.onMessageReceived != nil && len(data) >= message.HeaderLen {
		body := data[message.HeaderLen:]
		_ = s.onMessageReceived(s.settings.Address, msg.Type(), body, nil, s.sendCtxID, "sent", nil)
	}

	return nil
}

// TriggerHoldTimerExpiry triggers the hold timer expiry event.
// Exposed for testing.
func (s *Session) TriggerHoldTimerExpiry() {
	s.timers.StopAll()
	_ = s.fsm.Event(fsm.EventHoldTimerExpires)
	s.closeConn()
}

// parseCapabilities parses capabilities from OPEN optional parameters.
func parseCapabilities(optParams []byte) []capability.Capability {
	var caps []capability.Capability

	i := 0
	for i < len(optParams) {
		if i+2 > len(optParams) {
			break
		}
		paramType := optParams[i]
		paramLen := int(optParams[i+1])
		i += 2

		if i+paramLen > len(optParams) {
			break
		}

		if paramType == 2 { // Capabilities
			parsed, err := capability.Parse(optParams[i : i+paramLen])
			if err == nil {
				caps = append(caps, parsed...)
			}
		}

		i += paramLen
	}

	return caps
}

// buildOptionalParams builds optional parameters from capabilities.
// Each capability is wrapped in its own parameter (type 2) per RFC 5492.
func buildOptionalParams(caps []capability.Capability) []byte {
	if len(caps) == 0 {
		return nil
	}

	var optParams []byte
	for _, c := range caps {
		capBytes := c.Pack()
		// Wrap each capability in its own parameter type 2.
		param := make([]byte, 2+len(capBytes))
		param[0] = 2 // Parameter type: Capabilities
		param[1] = byte(len(capBytes))
		copy(param[2:], capBytes)
		optParams = append(optParams, param...)
	}

	return optParams
}

// SendUpdate sends a BGP UPDATE message.
// Returns ErrInvalidState if the session is not established.
//
// Uses zero-allocation path via Update.WriteTo and session write buffer.
// Note: Concurrent calls to SendUpdate must be externally synchronized.
func (s *Session) SendUpdate(update *message.Update) error {
	s.mu.RLock()
	conn := s.conn
	state := s.fsm.State()
	s.mu.RUnlock()

	if state != fsm.StateEstablished {
		return ErrInvalidState
	}

	if conn == nil {
		return ErrNotConnected
	}

	// RFC 4271 Section 4.3 - Zero-allocation: write UPDATE directly to session buffer
	s.writeBuf.Reset()
	n := update.WriteTo(s.writeBuf.Buffer(), 0, nil) // nil ctx: UPDATE already has wire bytes

	_, err := conn.Write(s.writeBuf.Buffer()[:n])
	if err != nil {
		return err
	}

	// Notify callback after successful send
	sessionLogger.Debug("SendUpdate complete", "peer", s.settings.Address, "hasCallback", s.onMessageReceived != nil, "msgLen", n)
	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
		sessionLogger.Debug("SendUpdate calling onMessageReceived", "peer", s.settings.Address, "direction", "sent", "ctxID", s.sendCtxID, "bodyLen", len(body))
		_ = s.onMessageReceived(s.settings.Address, message.TypeUPDATE, body, nil, s.sendCtxID, "sent", nil)
	}

	return nil
}

// SendAnnounce sends a BGP UPDATE message for announcing a route.
// Eliminates large buffer allocations by writing directly to session buffer.
// Returns ErrInvalidState if the session is not established.
//
// RFC 4271 Section 4.3 - UPDATE Message Format.
// RFC 4760 Section 3 - MP_REACH_NLRI for IPv6 routes.
// RFC 7911 - ADD-PATH encoding when addPath is true.
// RFC 6793 - 4-byte AS encoding when asn4 is true.
//
// Note: Concurrent calls must be externally synchronized.
func (s *Session) SendAnnounce(route plugin.RouteSpec, localAS uint32, isIBGP bool, asn4, addPath bool) error {
	s.mu.RLock()
	conn := s.conn
	state := s.fsm.State()
	s.mu.RUnlock()

	if state != fsm.StateEstablished {
		return ErrInvalidState
	}

	if conn == nil {
		return ErrNotConnected
	}

	// RFC 4271 Section 4.3 - Zero-allocation: write UPDATE directly to session buffer
	s.writeBuf.Reset()
	n := WriteAnnounceUpdate(s.writeBuf.Buffer(), 0, route, localAS, isIBGP, asn4, addPath)

	_, err := conn.Write(s.writeBuf.Buffer()[:n])
	if err != nil {
		return err
	}

	// Notify callback after successful send
	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
		_ = s.onMessageReceived(s.settings.Address, message.TypeUPDATE, body, nil, s.sendCtxID, "sent", nil)
	}

	return nil
}

// SendWithdraw sends a BGP UPDATE message for withdrawing a route.
// Eliminates large buffer allocations by writing directly to session buffer.
// Returns ErrInvalidState if the session is not established.
//
// RFC 4271 Section 4.3 - UPDATE Message Format (Withdrawn Routes for IPv4).
// RFC 4760 Section 4 - MP_UNREACH_NLRI for IPv6 withdrawals.
// RFC 7911 - ADD-PATH encoding when addPath is true.
//
// Note: Concurrent calls must be externally synchronized.
func (s *Session) SendWithdraw(prefix netip.Prefix, addPath bool) error {
	s.mu.RLock()
	conn := s.conn
	state := s.fsm.State()
	s.mu.RUnlock()

	if state != fsm.StateEstablished {
		return ErrInvalidState
	}

	if conn == nil {
		return ErrNotConnected
	}

	// RFC 4271 Section 4.3 - Zero-allocation: write UPDATE directly to session buffer
	s.writeBuf.Reset()
	n := WriteWithdrawUpdate(s.writeBuf.Buffer(), 0, prefix, addPath)

	_, err := conn.Write(s.writeBuf.Buffer()[:n])
	if err != nil {
		return err
	}

	// Notify callback after successful send
	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
		_ = s.onMessageReceived(s.settings.Address, message.TypeUPDATE, body, nil, s.sendCtxID, "sent", nil)
	}

	return nil
}

// SendRawUpdateBody sends a pre-encoded UPDATE message body (without BGP header).
// Used for zero-copy forwarding when encoding contexts match.
// Prepends the BGP header with correct length.
// Returns ErrInvalidState if the session is not established.
func (s *Session) SendRawUpdateBody(body []byte) error {
	s.mu.RLock()
	conn := s.conn
	state := s.fsm.State()
	s.mu.RUnlock()

	if state != fsm.StateEstablished {
		return ErrInvalidState
	}

	if conn == nil {
		return ErrNotConnected
	}

	// Build BGP header + body
	// RFC 4271 Section 4.1 - Length includes header (19 bytes) + body
	totalLen := message.HeaderLen + len(body)
	data := make([]byte, totalLen)

	// Marker (16 bytes of 0xFF)
	copy(data[:message.MarkerLen], message.Marker[:])

	// Length (2 bytes, big-endian)
	data[16] = byte(totalLen >> 8)
	data[17] = byte(totalLen)

	// Type (1 byte) - UPDATE = 2
	data[18] = byte(message.TypeUPDATE)

	// Body
	copy(data[message.HeaderLen:], body)

	_, err := conn.Write(data)
	return err
}

// SendRawMessage sends raw bytes to the peer.
// If msgType is 0, payload is a full BGP packet (user provides marker+header+body).
// If msgType is non-zero, payload is message body only (we add the header).
// ⚠️ No validation - bytes sent exactly as provided.
func (s *Session) SendRawMessage(msgType uint8, payload []byte) error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return ErrNotConnected
	}

	var data []byte
	if msgType == 0 {
		// Full packet mode - send as-is
		data = payload
	} else {
		// Message body mode - add BGP header
		totalLen := message.HeaderLen + len(payload)
		data = make([]byte, totalLen)

		// Marker (16 bytes of 0xFF)
		copy(data[:message.MarkerLen], message.Marker[:])

		// Length (2 bytes, big-endian)
		data[16] = byte(totalLen >> 8)
		data[17] = byte(totalLen)

		// Type (1 byte)
		data[18] = msgType

		// Body
		copy(data[message.HeaderLen:], payload)
	}

	_, err := conn.Write(data)
	return err
}

// isTimeout checks if an error is a timeout.
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

// shouldIgnoreFamily checks if UPDATE validation should be lenient for a family.
// Returns true if the family was configured with "ignore" mode.
func (s *Session) shouldIgnoreFamily(family capability.Family) bool {
	for _, f := range s.settings.IgnoreFamilies {
		if f.AFI == family.AFI && f.SAFI == family.SAFI {
			return true
		}
	}
	return false
}

// buildUnsupportedCapabilityData builds NOTIFICATION data for Unsupported Capability.
//
// RFC 5492 Section 3: The Data field contains one or more capability tuples.
// For Multiprotocol (code 1): AFI (2 bytes) + Reserved (1 byte) + SAFI (1 byte).
func buildUnsupportedCapabilityData(families []capability.Family) []byte {
	// Each Multiprotocol capability: code (1) + length (1) + AFI (2) + Reserved (1) + SAFI (1) = 6 bytes
	data := make([]byte, len(families)*6)
	offset := 0
	for _, f := range families {
		data[offset] = byte(capability.CodeMultiprotocol) // Capability code
		data[offset+1] = 4                                // Capability length
		binary.BigEndian.PutUint16(data[offset+2:], uint16(f.AFI))
		data[offset+4] = 0 // Reserved
		data[offset+5] = byte(f.SAFI)
		offset += 6
	}
	return data
}
