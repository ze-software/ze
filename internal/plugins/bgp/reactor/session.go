// Design: docs/architecture/core-design.md — BGP reactor event loop

package reactor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wireu"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wire"
	"codeberg.org/thomas-mangin/ze/internal/sim"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/source"
)

// sessionLogger is the session subsystem logger (lazy initialization).
// Controlled by ze.log.bgp.reactor.session environment variable.
var sessionLogger = slogutil.LazyLogger("bgp.reactor.session")

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

// buildBufPool provides reusable 4K buffers for building UPDATE path attributes.
// Get before buildRIBRouteUpdate / buildWithdrawNLRI, put after SendUpdate returns.
var buildBufPool = sync.Pool{
	New: func() any {
		return make([]byte, message.MaxMsgLen) // 4096
	},
}

// getBuildBuf returns a reusable 4K buffer from buildBufPool.
//
//nolint:forcetypeassert,errcheck // pool New always returns []byte
func getBuildBuf() []byte {
	return buildBufPool.Get().([]byte)
}

// putBuildBuf returns a buffer to the build pool.
func putBuildBuf(buf []byte) {
	buildBufPool.Put(buf) //nolint:staticcheck // SA6002: slice in pool is idiomatic Go
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
type MessageCallback func(peerAddr netip.Addr, msgType message.MessageType, rawBytes []byte, wireUpdate *wireu.WireUpdate, ctxID bgpctx.ContextID, direction string, buf []byte) (kept bool)

type Session struct {
	mu sync.RWMutex

	settings   *PeerSettings
	fsm        *fsm.FSM
	timers     *fsm.Timers
	clock      sim.Clock
	dialer     sim.Dialer
	conn       net.Conn
	bufReader  *bufio.Reader // Wraps conn to batch kernel read syscalls
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

	// writeMu serializes all access to writeBuf.
	// Multiple goroutines send concurrently (keepalive timer, forward pool workers,
	// sendInitialRoutes, plugin RPC handlers) — this mutex prevents races on the
	// shared buffer. Lock ordering: s.mu before s.writeMu (never reverse).
	writeMu sync.Mutex

	// Write buffer for zero-allocation message building.
	// Allocated at 4096 bytes initially, resized to 65535 if Extended Message negotiated.
	// All access must hold writeMu.
	writeBuf *wire.SessionBuffer

	// Error channel for timer callbacks to signal errors.
	errChan chan error

	// tearingDown is set when Teardown starts, preventing Accept race.
	tearingDown atomic.Bool

	// Backpressure pause gate: pauses the read loop without closing the connection.
	// When paused, TCP recv buffer fills → kernel shrinks window → sender throttles.
	// Write path (KEEPALIVE) is independent and continues during pause.
	// RFC 4271 §6.5: hold timer expires if paused long enough (safety valve).
	paused   atomic.Bool   // Fast-path check — false in normal operation
	pauseMu  sync.Mutex    // Protects resumeCh create/close
	resumeCh chan struct{} // Closed by Resume() to unblock the read loop

	// closeReason stores why the connection was closed (context cancel, hold timer,
	// teardown, etc.). Set atomically before closeConn() so the read loop can
	// distinguish close reasons after ReadFull returns an error.
	// Only the first reason wins (CompareAndSwap from nil).
	closeReason atomic.Pointer[error]

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

	// pluginFamiliesGetter retrieves families from plugins that declared decode.
	// Used to auto-add Multiprotocol capabilities for plugin-provided families.
	// Set by Peer to link to plugin.Server registry.
	pluginFamiliesGetter func() []string

	// openValidator is called during OPEN processing to let plugins validate the OPEN pair.
	// Returns nil to accept, or an OpenValidationError to reject with NOTIFICATION.
	// Set by Peer to link to Server.BroadcastValidateOpen().
	openValidator func(peerAddr string, local, remote *message.Open) error

	// done is closed when the Run loop exits.
	done chan struct{}
}

// NewSession creates a new BGP session for a peer.
func NewSession(settings *PeerSettings) *Session {
	dialer := &sim.RealDialer{}
	if settings.LocalAddress.IsValid() {
		dialer.LocalAddr = &net.TCPAddr{IP: settings.LocalAddress.AsSlice()}
	}

	s := &Session{
		settings: settings,
		fsm:      fsm.New(),
		timers:   fsm.NewTimers(),
		clock:    sim.RealClock{},
		dialer:   dialer,
		writeBuf: wire.NewSessionBuffer(false), // Start with 4096, resize if Extended Message
		errChan:  make(chan error, 2),          // Buffer 2: normal error + teardown
		done:     make(chan struct{}),
	}

	// Configure FSM connection mode: passive if active bit is NOT set.
	s.fsm.SetPassive(!settings.Connection.IsActive())

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

// SetClock sets the clock used for deadline and sleep operations.
// Must be called before Run.
func (s *Session) SetClock(c sim.Clock) {
	s.clock = c
	s.timers.SetClock(c)
}

// SetDialer sets the dialer used for outbound connections.
// Must be called before Connect.
func (s *Session) SetDialer(d sim.Dialer) {
	s.dialer = d
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

// Done returns a channel that is closed when the session Run loop exits.
func (s *Session) Done() <-chan struct{} {
	return s.done
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

// SetPluginFamiliesGetter sets the callback for retrieving plugin decode families.
// Called by Peer at creation time to link to plugin.Server registry.
// Used to auto-add Multiprotocol capabilities for families that plugins can decode.
func (s *Session) SetPluginFamiliesGetter(getter func() []string) {
	s.pluginFamiliesGetter = getter
}

// SetOpenValidator sets the callback for validating OPEN message pairs.
// Called by Peer at creation time to link to Server.BroadcastValidateOpen().
// Plugins that register WantsValidateOpen will be consulted during OPEN processing.
func (s *Session) SetOpenValidator(validator func(string, *message.Open, *message.Open) error) {
	s.openValidator = validator
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

	conn, err := s.dialer.DialContext(ctx, "tcp", addr)
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
	localOpen := s.localOpen
	s.mu.Unlock()

	// Parse capabilities once from both OPENs.
	var localCaps []capability.Capability
	if localOpen != nil {
		localCaps = capability.ParseFromOptionalParams(localOpen.OptionalParams)
	}
	peerCaps := capability.ParseFromOptionalParams(open.OptionalParams)

	// Negotiate capabilities.
	s.negotiateWith(localCaps, peerCaps)

	// Validate required families and capabilities.
	s.mu.RLock()
	conn := s.conn
	neg := s.negotiated
	requiredFamilies := s.settings.RequiredFamilies
	requiredCaps := s.settings.RequiredCapabilities
	refusedCaps := s.settings.RefusedCapabilities
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

	// RFC 5492 Section 3: Validate required/refused capability codes.
	if err := s.validateCapabilityModes(conn, neg, requiredCaps, refusedCaps); err != nil {
		return err
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
	s.bufReader = bufio.NewReaderSize(conn, 65536)
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

	// Set close reason BEFORE closing conn so the read loop can identify this
	// as a teardown (not just a connection reset) after ReadFull returns error.
	s.setCloseReason(ErrTeardown)
	s.closeConn()
	_ = s.fsm.Event(fsm.EventManualStop)

	// Signal errChan so the cancel goroutine in Run() exits cleanly.
	// Non-blocking: channel may be full if cancel goroutine already consumed
	// a signal, or Run() may have already exited.
	select {
	case s.errChan <- ErrTeardown:
	default: // errChan full or closed — cancel goroutine already processed a signal
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
		// bufReader is NOT nilled here: Run() may have captured conn (non-nil)
		// before this lock and will call readAndProcessMessage next. The stale
		// bufReader wrapping the closed conn returns a proper read error,
		// which readAndProcessMessage handles as ErrConnectionClosed.
		// connectionEstablished() replaces bufReader on reconnection.
	}
}

// setCloseReason atomically stores why the connection is being closed.
// Only the first reason wins — subsequent calls are no-ops.
func (s *Session) setCloseReason(err error) {
	s.closeReason.CompareAndSwap(nil, &err)
}

// Run is the main session loop. It processes messages until context is
// canceled or an error occurs.
//
// Uses close-on-cancel pattern: a cancel goroutine watches ctx.Done() and
// errChan, then closes the connection to unblock any pending io.ReadFull.
// This replaces the previous 100ms SetReadDeadline polling approach, providing
// instant cancellation response on all connection types (including net.Pipe).
func (s *Session) Run(ctx context.Context) error {
	defer close(s.done)

	// Cancel goroutine: watches for shutdown signals and closes the connection
	// to unblock ReadFull. Sets closeReason before closing so the read loop
	// can distinguish cancel from hold timer from teardown.
	go func() {
		var reason error
		select {
		case <-ctx.Done():
			reason = ctx.Err()
		case err := <-s.errChan:
			reason = err
		case <-s.done:
			return // Run already exited, nothing to do.
		}
		s.setCloseReason(reason)
		s.closeConn()
		s.Resume() // Unblock pause gate if paused.
	}()

	for {
		// Backpressure pause gate: if paused, block until resumed or shutdown.
		// Fast path: atomic load returns false, zero overhead when not paused.
		if s.paused.Load() {
			if err := s.waitForResume(ctx); err != nil {
				return err
			}
		}

		// Check if we have a connection.
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		if conn == nil {
			// No connection yet (waiting for Accept). Check for shutdown.
			if reason := s.closeReason.Load(); reason != nil {
				return *reason
			}
			s.clock.Sleep(10 * time.Millisecond)
			continue
		}

		// ReadFull blocks until data arrives or conn is closed by cancel goroutine.
		err := s.readAndProcessMessage(conn)
		if err != nil {
			// Connection was closed — check if a specific reason was set.
			if errors.Is(err, ErrConnectionClosed) {
				if reason := s.closeReason.Load(); reason != nil {
					return *reason
				}
				return err
			}
			return err
		}
	}
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

// writeMessage writes a BGP message directly into the session buffer.
func (s *Session) writeMessage(conn net.Conn, msg message.Message) error {
	if conn == nil {
		return ErrNotConnected
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Zero-allocation: write directly into session buffer.
	// Message types don't use context for basic encoding
	// (KEEPALIVE, OPEN, NOTIFICATION have fixed formats).
	s.writeBuf.Reset()
	n := msg.WriteTo(s.writeBuf.Buffer(), 0, nil)

	_, err := conn.Write(s.writeBuf.Buffer()[:n])
	if err != nil {
		return err
	}

	// Notify callback after successful send.
	// Body is data after the 19-byte header (16-byte marker + 2-byte length + 1-byte type).
	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
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

// SendUpdate sends a BGP UPDATE message.
// Returns ErrInvalidState if the session is not established.
//
// Uses zero-allocation path via Update.WriteTo and session write buffer.
// Concurrent calls are serialized by writeMu.
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

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// RFC 4271 Section 4.3 - Zero-allocation: write UPDATE directly to session buffer
	s.writeBuf.Reset()
	n := update.WriteTo(s.writeBuf.Buffer(), 0, nil) // nil ctx: UPDATE already has wire bytes

	_, err := conn.Write(s.writeBuf.Buffer()[:n])
	if err != nil {
		return err
	}

	// Notify callback after successful send
	sessionLogger().Debug("SendUpdate complete", "peer", s.settings.Address, "hasCallback", s.onMessageReceived != nil, "msgLen", n)
	if s.onMessageReceived != nil && n >= message.HeaderLen {
		body := s.writeBuf.Buffer()[message.HeaderLen:n]
		sessionLogger().Debug("SendUpdate calling onMessageReceived", "peer", s.settings.Address, "direction", "sent", "ctxID", s.sendCtxID, "bodyLen", len(body))
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
// Concurrent calls are serialized by writeMu.
func (s *Session) SendAnnounce(route bgptypes.RouteSpec, localAS uint32, isIBGP, asn4, addPath bool) error {
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

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

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
// Concurrent calls are serialized by writeMu.
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

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

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
// Concurrent calls are serialized by writeMu.
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

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// RFC 4271 Section 4.1 - Zero-allocation: write header + body into session buffer
	totalLen := message.HeaderLen + len(body)
	s.writeBuf.Reset()
	buf := s.writeBuf.Buffer()

	// Marker (16 bytes of 0xFF)
	copy(buf[:message.MarkerLen], message.Marker[:])

	// Length (2 bytes, big-endian)
	buf[16] = byte(totalLen >> 8)
	buf[17] = byte(totalLen)

	// Type (1 byte) - UPDATE = 2
	buf[18] = byte(message.TypeUPDATE)

	// Body
	copy(buf[message.HeaderLen:], body)

	_, err := conn.Write(buf[:totalLen])
	return err
}

// SendRawMessage sends raw bytes to the peer.
// If msgType is 0, payload is a full BGP packet (user provides marker+header+body).
// If msgType is non-zero, payload is message body only (we add the header).
// Concurrent calls are serialized by writeMu (when msgType != 0).
func (s *Session) SendRawMessage(msgType uint8, payload []byte) error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return ErrNotConnected
	}

	if msgType == 0 {
		// Full packet mode - send as-is (no writeBuf used)
		_, err := conn.Write(payload)
		return err
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Message body mode - write header + body into session buffer
	totalLen := message.HeaderLen + len(payload)
	s.writeBuf.Reset()
	buf := s.writeBuf.Buffer()

	// Marker (16 bytes of 0xFF)
	copy(buf[:message.MarkerLen], message.Marker[:])

	// Length (2 bytes, big-endian)
	buf[16] = byte(totalLen >> 8)
	buf[17] = byte(totalLen)

	// Type (1 byte)
	buf[18] = msgType

	// Body
	copy(buf[message.HeaderLen:], payload)

	_, err := conn.Write(buf[:totalLen])
	return err
}
