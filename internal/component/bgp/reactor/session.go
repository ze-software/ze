// Design: docs/architecture/core-design.md — BGP session struct, constructor, accessors, run loop
// Design: .claude/rules/design-principles.md — zero-copy, copy-on-modify (Incoming Peer Pool buffer allocated at receive)
// Detail: session_connection.go — connect, accept, teardown
// Detail: session_write.go — wire write primitives and Send* methods
// Detail: session_handlers.go — per-message-type handlers
// Detail: session_negotiate.go — capability negotiation
// Detail: session_read.go — message read loop
// Detail: session_validation.go — RFC 7606 validation
// Detail: session_flow.go — backpressure pause/resume gate
// Detail: session_prefix.go — prefix limit enforcement (RFC 4486)
// Related: bufmux.go — block-backed buffer multiplexer for read/build buffers

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

// bufMuxBlockSize is the number of buffers per block in the pool multiplexer.
// Each block is one contiguous allocation. Sized for typical concurrent peer counts.
const bufMuxBlockSize = 128

// bufMuxProbeInterval is the number of Get() calls between collapse checks
// and overflow probe callbacks. Normal network traffic drives the interval.
const bufMuxProbeInterval = 100

// bufMux4K is the block-backed multiplexer for 4K buffers.
// Serves both read (pre-Extended Message) and build (UPDATE attributes) paths.
// Collapse probe wired via withCollapseProbe; overflow probe available via AddProbe.
var bufMux4K = withCollapseProbe(newProbedPool(message.MaxMsgLen, bufMuxBlockSize), bufMuxProbeInterval)

// bufMux64K is the block-backed multiplexer for 64K buffers.
// Serves read path after Extended Message capability is negotiated (RFC 8654).
// Collapse probe wired via withCollapseProbe; overflow probe available via AddProbe.
var bufMux64K = withCollapseProbe(newProbedPool(message.ExtMsgLen, bufMuxBlockSize), bufMuxProbeInterval)

// initBufMuxBudget wires a shared byte budget into both multiplexers.
// Called once from reactor startup, before any concurrent use.
// maxBytes <= 0 means unlimited initially (AC-27). A budget is always
// created so updateBufMuxBudget never needs the create-path concurrently.
func initBufMuxBudget(maxBytes int64) {
	cb := newCombinedBudget(maxBytes) // 0 = unlimited
	bufMux4K.SetBudget(cb)
	bufMux64K.SetBudget(cb)
}

// updateBufMuxBudget updates the shared byte budget limit atomically.
// Called by the weight tracker when the peer set changes and
// ze.fwd.pool.maxbytes is not explicitly set (auto-sizing, AC-28).
// maxBytes <= 0 means unlimited (tryReserve treats it as no-limit).
func updateBufMuxBudget(maxBytes int64) {
	bufMux4K.mux.mu.Lock()
	b := bufMux4K.mux.budget
	bufMux4K.mux.mu.Unlock()
	if b != nil {
		b.maxBytes.Store(maxBytes)
	}
}

// CombinedBufMuxStats returns total allocated and in-use byte counts
// across both the 4K and 64K buffer multiplexers. Used by metrics and
// backpressure decisions (AC-27: memory pressure is shared).
func CombinedBufMuxStats() (totalBytes, usedBytes int64) {
	return combinedMuxStats(bufMux4K.mux, bufMux64K.mux)
}

// CombinedBufMuxUsedRatio returns the fraction of allocated bytes in use
// across both multiplexers (0.0 to 1.0). Returns 0.0 if nothing is allocated.
func CombinedBufMuxUsedRatio() float64 {
	return combinedMuxUsedRatio(bufMux4K.mux, bufMux64K.mux)
}

// getBuildBuf returns a reusable 4K buffer handle from the 4K multiplexer.
// Caller MUST call putBuildBuf when done.
func getBuildBuf() BufHandle {
	return bufMux4K.Get()
}

// putBuildBuf returns a build buffer handle to the 4K multiplexer.
func putBuildBuf(h BufHandle) {
	bufMux4K.Return(h)
}

// ReturnReadBuffer returns a buffer handle to the appropriate multiplexer.
// Used by cache to return buffers when entries are evicted.
func ReturnReadBuffer(h BufHandle) {
	if h.Buf == nil {
		return
	}
	// Route by len(h.Buf), not cap(). Slices into backing arrays have
	// cap = len(backing) - offset, which varies by position. But len()
	// is always exactly bufSize since get() returns backing[off:off+bufSize].
	if len(h.Buf) >= message.ExtMsgLen {
		bufMux64K.Return(h)
	} else {
		bufMux4K.Return(h)
	}
}

// Session errors.
var (
	ErrNotConnected         = errors.New("not connected")
	ErrAlreadyConnected     = errors.New("already connected")
	ErrInvalidState         = errors.New("invalid FSM state")
	ErrNotificationRecv     = errors.New("notification received")
	ErrConnectionClosed     = errors.New("connection closed")
	ErrHoldTimerExpired     = errors.New("hold timer expired")
	ErrInvalidMessage       = errors.New("invalid message")
	ErrUnsupportedVersion   = errors.New("unsupported BGP version")
	ErrFamilyNotNegotiated  = errors.New("address family not negotiated")
	ErrSessionTearingDown   = errors.New("session is tearing down")
	ErrPrefixLimitExceeded  = errors.New("prefix limit exceeded")
	ErrSendHoldTimerExpired = errors.New("send hold timer expired")
)

// sendHoldTimerMin is the minimum Send Hold Timer duration per RFC 9687.
const sendHoldTimerMin = 8 * time.Minute

// Session manages a single BGP peer connection.
//
// It integrates the FSM, timers, and message I/O to drive the BGP
// state machine through the connection lifecycle.

// MessageCallback is called when a BGP message is sent or received.
// peerAddr is the peer's address, msgType is the message type, rawBytes is the body (without header).
// direction is "sent" or "received".
// wireUpdate is non-nil for UPDATE messages (zero-copy), nil for other types.
// ctxID is the encoding context for zero-copy decisions.
// buf is the pool buffer handle for received messages (zero-value for sent).
// meta is route metadata from ReceivedUpdate (sent events only); nil for received.
// Returns true if callback took ownership of buf (caller should not return to pool).
type MessageCallback func(peerAddr netip.Addr, msgType message.MessageType, rawBytes []byte, wireUpdate *wireu.WireUpdate, ctxID bgpctx.ContextID, direction string, buf BufHandle, meta map[string]any) (kept bool)

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

	// sentMeta holds route metadata for the current forward pool write operation.
	// Lifecycle: set per-item by fwdBatchHandler, read by writeRawUpdateBody/writeUpdate
	// within the same writeMu critical section, cleared to nil by defer on all exit paths.
	// MUST NOT be read outside writeMu. Zero-value (nil) for non-forward writes.
	sentMeta map[string]any

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

	// prefixMetrics is a reference to reactor-level Prometheus prefix metrics.
	// Set by Peer in runOnce(). Nil when metrics are not enabled.
	prefixMetrics *reactorMetrics

	// prefixWarningNotifier is called when a family's warning state changes.
	// Set by Peer in runOnce() to propagate warning state for API visibility.
	prefixWarningNotifier func(family string, warned bool)

	// onNotifSent is called when a NOTIFICATION is sent to the peer.
	// Set by Peer in runOnce() for Prometheus notification counter.
	onNotifSent func(code, subcode uint8)

	// onNotifRecv is called when a NOTIFICATION is received from the peer.
	// Set by Peer in runOnce() for Prometheus notification counter.
	onNotifRecv func(code, subcode uint8)

	// recentRead is set to true by the read loop on every successful message read.
	// The hold timer callback checks and clears it: if true, the daemon is
	// CPU-congested (data arrived but wasn't processed in time), so the hold
	// timer is extended instead of tearing down. Atomic for thread safety
	// between read goroutine and timer goroutine.
	recentRead atomic.Bool

	// sendHoldTimer detects when the local side cannot send any data to the
	// peer (RFC 9687). Reset on every successful write. On expiry, the session
	// is torn down with NOTIFICATION Error Code 8 (Send Hold Timer Expired).
	// Started when session enters ESTABLISHED, stopped on close.
	sendHoldTimer clock.Timer
	sendHoldMu    sync.Mutex // protects sendHoldTimer
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
		prefixCounts: &prefixCounts{counts: make(map[uint32]int64), warned: make(map[uint32]bool)},
	}

	// Configure FSM connection mode: passive if active bit is NOT set.
	s.fsm.SetPassive(!settings.Connection.IsActive())

	// Configure timers.
	s.timers.SetHoldTime(settings.ReceiveHoldTime)

	// Wire up timer callbacks.
	s.timers.OnHoldTimerExpires(func() {
		// BIRD technique: if data was recently read, the daemon is
		// CPU-congested (busy processing other peers' UPDATEs), not the
		// remote peer. Extend hold timer by 10s instead of tearing down.
		// The remote peer IS sending data -- we just haven't processed it yet.
		if s.recentRead.Swap(false) {
			sessionLogger().Info("hold timer extended: recent read activity (CPU congestion)",
				"peer", s.settings.Address,
			)
			s.timers.ResetHoldTimer()
			return
		}

		s.mu.Lock()
		s.logFSMEvent(fsm.EventHoldTimerExpires)
		s.mu.Unlock()
		select {
		case s.errChan <- ErrHoldTimerExpired:
		default: // errChan full -- cancel goroutine already has a signal
		}
	})

	s.timers.OnKeepaliveTimerExpires(func() {
		// RFC 4271 Section 8.2.2: Event 11 (KeepaliveTimer_Expires)
		// "sends a KEEPALIVE message" — fire the FSM event first, then send.
		s.logFSMEvent(fsm.EventKeepaliveTimerExpires)

		s.mu.Lock()
		conn := s.conn
		s.mu.Unlock()

		if conn != nil {
			if err := s.sendKeepalive(conn); err != nil {
				sessionLogger().Debug("keepalive send failed", "peer", s.settings.Address, "error", err)
			}
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
func (s *Session) getReadBuffer() BufHandle {
	if s.extendedMessage {
		return bufMux64K.Get()
	}
	return bufMux4K.Get()
}

// returnReadBuffer returns buffer to the appropriate multiplexer.
func (s *Session) returnReadBuffer(h BufHandle) {
	ReturnReadBuffer(h)
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

// logNotifyErr sends a NOTIFICATION and logs if the send fails.
// Used on error/shutdown paths where the connection may already be dead.
func (s *Session) logNotifyErr(conn net.Conn, code message.NotifyErrorCode, subcode uint8, data []byte) {
	if err := s.sendNotification(conn, code, subcode, data); err != nil {
		sessionLogger().Debug("notification send failed",
			"peer", s.settings.Address,
			"code", uint8(code), "subcode", subcode,
			"error", err,
		)
	}
}

// logFSMEvent fires an FSM event and logs if the transition fails.
// FSM transition failures on error paths indicate unexpected state.
func (s *Session) logFSMEvent(event fsm.Event) {
	if err := s.fsm.Event(event); err != nil {
		sessionLogger().Warn("FSM event failed",
			"peer", s.settings.Address,
			"event", event,
			"state", s.fsm.State(),
			"error", err,
		)
	}
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
