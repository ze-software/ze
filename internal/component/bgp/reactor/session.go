// Design: docs/architecture/core-design.md — BGP session struct, constructor, accessors, run loop
// Detail: session_connection.go — connect, accept, teardown
// Detail: session_write.go — wire write primitives and Send* methods
// Detail: session_handlers.go — per-message-type handlers
// Detail: session_negotiate.go — capability negotiation
// Detail: session_read.go — message read loop
// Detail: session_validation.go — RFC 7606 validation
// Detail: session_flow.go — backpressure pause/resume gate

package reactor

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wire"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/source"
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
	ErrPrefixLimitExceeded = errors.New("prefix limit exceeded")
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
	clock      clock.Clock
	dialer     network.Dialer
	conn       net.Conn
	bufReader  *bufio.Reader // Wraps conn to batch kernel read syscalls
	bufWriter  *bufio.Writer // Wraps conn to batch kernel write syscalls
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
	// Set by Peer to link to plugin.Server.GetPluginCapabilitiesForPeer().
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

	// prefixCounts tracks received prefix count per family for prefix limit enforcement.
	// Initialized in NewSession when PrefixMaximum is configured.
	// Only accessed from session's read goroutine (no synchronization needed).
	prefixCounts *prefixCounts
}

// NewSession creates a new BGP session for a peer.
func NewSession(settings *PeerSettings) *Session {
	dialer := &network.RealDialer{}
	if settings.LocalAddress.IsValid() {
		dialer.LocalAddr = &net.TCPAddr{IP: settings.LocalAddress.AsSlice()}
	}
	if settings.MD5Key != "" {
		md5Addr := settings.Address
		if settings.MD5IP.IsValid() {
			md5Addr = settings.MD5IP
		}
		dialer.PeerAddr = md5Addr.AsSlice()
		dialer.MD5Key = settings.MD5Key
	}

	s := &Session{
		settings:     settings,
		fsm:          fsm.New(),
		timers:       fsm.NewTimers(),
		clock:        clock.RealClock{},
		dialer:       dialer,
		writeBuf:     wire.NewSessionBuffer(false), // Start with 4096, resize if Extended Message
		errChan:      make(chan error, 2),          // Buffer 2: normal error + teardown
		done:         make(chan struct{}),
		prefixCounts: &prefixCounts{counts: make(map[string]int64), warned: make(map[string]bool)},
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
		// RFC 4271 Section 8.2.2: Event 11 (KeepaliveTimer_Expires)
		// "sends a KEEPALIVE message" — fire the FSM event first, then send.
		_ = s.fsm.Event(fsm.EventKeepaliveTimerExpires)

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
func (s *Session) SetClock(c clock.Clock) {
	s.clock = c
	s.timers.SetClock(c)
}

// SetDialer sets the dialer used for outbound connections.
// Must be called before Connect.
func (s *Session) SetDialer(d network.Dialer) {
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
// Called by Peer at creation time to link to plugin.Server.GetPluginCapabilitiesForPeer().
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

// ErrTeardown is returned when the session is torn down via API.
var ErrTeardown = errors.New("session teardown")

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
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				sessionLogger().Error("session cancel goroutine panic recovered",
					"peer", s.settings.Address,
					"panic", r,
					"stack", string(buf[:n]),
				)
				// Best-effort cleanup: close connection to unblock read loop.
				s.closeConn()
			}
		}()
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
