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
	"log/slog"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/wireu"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/memguard"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/source"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sessionLoggerRef holds the session subsystem logger provider (lazy initialization).
// Controlled by ze.log.bgp.reactor.session environment variable.
//
// The provider lives behind an atomic.Value so tests can override it (to measure logging
// cost when Debug is disabled) without racing the cold-path callers on background goroutines
// -- keepalive/hold timer callbacks and the cancel goroutine -- that log through
// sessionLogger(). A plain package var read/written concurrently is a data race that surfaces
// under stress when a test swaps the logger while another session's timer is still firing.
var sessionLoggerRef = func() *atomic.Pointer[sessionLoggerProvider] {
	p := new(atomic.Pointer[sessionLoggerProvider])
	base := sessionLoggerProvider(slogutil.LazyLogger("bgp.reactor.session"))
	p.Store(&base)
	return p
}()

// sessionLoggerProvider resolves the current session subsystem logger (lazy).
type sessionLoggerProvider func() *slog.Logger

// sessionLogger returns the session subsystem logger.
func sessionLogger() *slog.Logger {
	return (*sessionLoggerRef.Load())()
}

// bufMuxBlockSize is the number of buffers per block in the pool multiplexer.
// Each block is one contiguous allocation. Sized for typical concurrent peer counts.
const bufMuxBlockSize = 128

// bufMuxProbeInterval is the number of Get() calls between collapse checks
// and overflow probe callbacks. Normal network traffic drives the interval.
const bufMuxProbeInterval = 100

// bufMuxStd is the block-backed multiplexer for 4K buffers.
// Serves both read (pre-Extended Message) and build (UPDATE attributes) paths.
// Collapse probe wired via withCollapseProbe; overflow probe available via AddProbe.
var bufMuxStd = withCollapseProbe(newProbedPool(message.MaxMsgLen, bufMuxBlockSize), bufMuxProbeInterval)

// bufMuxExt is the block-backed multiplexer for 64K buffers.
// Serves read path after Extended Message capability is negotiated (RFC 8654).
// Collapse probe wired via withCollapseProbe; overflow probe available via AddProbe.
var bufMuxExt = withCollapseProbe(newProbedPool(message.ExtMsgLen, bufMuxBlockSize), bufMuxProbeInterval)

// initBufMuxBudget wires a shared byte budget into both multiplexers.
// Called once from reactor startup, before any concurrent use.
// maxBytes <= 0 means unlimited initially (AC-27). A budget is always
// created so updateBufMuxBudget never needs the create-path concurrently.
func initBufMuxBudget(maxBytes int64) {
	cb := newCombinedBudget(maxBytes) // 0 = unlimited
	bufMuxStd.SetBudget(cb)
	bufMuxExt.SetBudget(cb)
}

// updateBufMuxBudget updates the shared byte budget limit atomically.
// Called by the weight tracker when the peer set changes and
// ze.fwd.pool.maxbytes is not explicitly set (auto-sizing, AC-28).
// maxBytes <= 0 means unlimited (tryReserve treats it as no-limit).
func updateBufMuxBudget(maxBytes int64) {
	bufMuxStd.mux.mu.Lock()
	b := bufMuxStd.mux.budget
	bufMuxStd.mux.mu.Unlock()
	if b != nil {
		b.maxBytes.Store(maxBytes)
	}
}

// CombinedBufMuxStats returns total allocated and in-use byte counts
// across both the 4K and 64K buffer multiplexers. Used by metrics and
// backpressure decisions (AC-27: memory pressure is shared).
func CombinedBufMuxStats() (totalBytes, usedBytes int64) {
	return combinedMuxStats(bufMuxStd.mux, bufMuxExt.mux)
}

// CombinedBufMuxUsedRatio returns the fraction of allocated bytes in use
// across both multiplexers (0.0 to 1.0). Returns 0.0 if nothing is allocated.
func CombinedBufMuxUsedRatio() float64 {
	return combinedMuxUsedRatio(bufMuxStd.mux, bufMuxExt.mux)
}

// getBuildBuf returns a reusable 4K buffer handle from the 4K multiplexer.
// Caller MUST call putBuildBuf when done.
func getBuildBuf() BufHandle {
	return bufMuxStd.Get()
}

// putBuildBuf returns a build buffer handle to the 4K multiplexer.
func putBuildBuf(h BufHandle) {
	bufMuxStd.Return(h)
}

// ReturnReadBuffer returns a buffer handle to the appropriate multiplexer.
// Used by cache to return buffers when entries are evicted.
//
// Skips handles carrying the noPoolBufID sentinel: these are backed by a
// standalone make([]byte, ...) slice with no corresponding pool slot -- the
// RFC 7606 treat-as-withdraw extra-family body in production (see session_read.go),
// and test fakes (see testPoolBuf in reactor_test.go). Without this skip, such a
// handle's ID/idx collide with a real slot in bufMuxStd.block[0] and either
// trigger a "double return detected" log or silently free an in-use slot
// (memory corruption).
func ReturnReadBuffer(h BufHandle) {
	if h.Buf == nil || h.ID == noPoolBufID {
		return
	}
	if memguard.Enabled {
		// Poison the whole slot before it re-enters the pool so a retained
		// RawBytes borrow into it (a received-UPDATE slice not owned via
		// WireUpdate.Snapshot) reads poison in debug rather than the next
		// message's bytes. Contract A; docs/architecture/memory/lifetime-contracts.md.
		memguard.Poison(h.Buf)
	}
	// Route by len(h.Buf), not cap(). Slices into backing arrays have
	// cap = len(backing) - offset, which varies by position. But len()
	// is always exactly bufSize since get() returns backing[off:off+bufSize].
	if len(h.Buf) >= message.ExtMsgLen {
		bufMuxExt.Return(h)
	} else {
		bufMuxStd.Return(h)
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
	ErrBadBGPIdentifier     = errors.New("bad BGP identifier (RFC 6286 Section 2.2)")
	ErrFamilyNotNegotiated  = errors.New("address family not negotiated")
	ErrSessionTearingDown   = errors.New("session is tearing down")
	ErrPrefixLimitExceeded  = errors.New("prefix limit exceeded")
	ErrSendHoldTimerExpired = errors.New("send hold timer expired")
)

// sendHoldTimerMin is the minimum Send Hold Timer duration per RFC 9687.
const sendHoldTimerMin = 8 * time.Minute

// holdGraceExtension is the bounded reprieve granted to a hold expiry that
// arrives while the read loop has recently seen traffic: the daemon is
// CPU-congested, not the peer. One grace window only -- the next expiry with no
// intervening read tears the session down (RFC 4271 Section 8.2.2 Event 10).
// Clamped to the negotiated hold time by GraceRearmHoldTimer.
const holdGraceExtension = 10 * time.Second

// Session manages a single BGP peer connection.
//
// It integrates the FSM, timers, and message I/O to drive the BGP
// state machine through the connection lifecycle.

// MessageCallback is called when a BGP message is sent or received.
// peerAddr is the peer's address, msgType is the message type, rawBytes is the body (without header).
// direction is rpc.DirectionSent or rpc.DirectionReceived.
// wireUpdate is non-nil for UPDATE messages (zero-copy), nil for other types.
// ctxID is the encoding context for zero-copy decisions.
// buf is the pool buffer handle for received messages (zero-value for sent).
// meta is route metadata from ReceivedUpdate (sent events only); nil for received.
// sentSourcePeerStr is the forwarding source peer's address string for sent forward-pool
// writes (ribOut stale-scoping), captured at the write site inside the sender's writeMu
// critical section; "" for received messages and non-forward sends. It travels as an
// argument rather than being re-read from peer.session in the receiver, which would race
// the peer run goroutine that nils/replaces peer.session under peer.mu.
// Returns true if callback took ownership of buf (caller should not return to pool).
type MessageCallback func(peerAddr netip.Addr, msgType msgtype.MessageType, rawBytes []byte, wireUpdate *wireu.WireUpdate, ctxID bgpctx.ContextID, direction rpc.MessageDirection, buf BufHandle, meta map[string]any, sentSourcePeerStr string) (kept bool)

// Lock hierarchy (acquire in this order; never reverse):
//
//  1. s.mu         — most fields (settings, conn, bufReader/Writer, peerOpen, sentMeta, ...)
//  2. s.writeMu    — writeBuf and bufWriter; acquired AFTER s.mu when both are needed
//     (see closeConn in session_connection.go and sendOpen in session_negotiate.go)
//  3. s.sendHoldMu — sendHoldTimer; acquired AFTER s.writeMu via resetSendHoldTimer,
//     which Send* methods call from inside their writeMu critical section
//
// s.pauseMu is independent of the other three: it protects only the resumeCh
// create/close pair (see session_flow.go) and is never nested with s.mu,
// s.writeMu, or s.sendHoldMu.
//
// One lock OUTSIDE this Session is ordered against it: Peer.mu comes BEFORE s.mu.
// Peer.ResolvePendingCollision (peer_connection.go) holds p.mu.Lock() across
// DetectCollision, which reads peerOpen under s.mu (see collisionPeerAS). The
// reverse must never appear: no Session code may acquire p.mu, and no callback a
// Peer installs on a Session may be invoked with s.mu held.
//
// Atomics (no lock interaction needed):
//
//	tearingDown   — set by Teardown to block Accept races
//	paused        — fast-path pause check for the read loop
//	closeReason   — first close reason wins (CompareAndSwap from nil)
//	recentRead    — read loop signals "data arrived" to hold-timer callback
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

	// egressRouteFilter, when non-nil, runs the peer's export filter chain on an
	// outbound route UPDATE body just before it is written (writeUpdate /
	// SendAnnounce). Returns suppress=true to drop the route, or an override body
	// to write instead. Set by Peer (capturing the reactor) so originated /
	// injected / replayed routes honor export filters like forwarded ones. EORs
	// and the forwarded path (writeRawUpdateBody) do not call it.
	egressRouteFilter func(body []byte) (suppress bool, override []byte)

	// policyTeardownPending, when non-nil, queues a NOTIFICATION + session close
	// requested by the import policy filter chain (e.g. filter_family tear-down).
	// Set on the session read goroutine inside the onMessageReceived callback;
	// honored in session_read after the callback, before handleUpdate. Accessed
	// only from the session read goroutine — no lock.
	policyTeardownPending *policyTeardownRequest

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

	// sentSourcePeerStr holds the source peer address string for the current
	// forward pool write. Set alongside sentMeta by fwdBatchHandler. Used by
	// sent event callbacks for ribOut stale-scoping without map allocation.
	sentSourcePeerStr string

	// fwdDirty tracks destination sessions with unflushed writes from the RS
	// fast path (tryDirectWriteNoFlush). Flushed by flushFwdDirty when the
	// source bufReader has no more data. Only accessed from this session's
	// read goroutine -- no synchronization needed.
	fwdDirty []*Session

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
	// Always initialized in NewSession, regardless of whether PrefixMaximum is
	// configured: without limits the tally still feeds route-count anomaly detection.
	// Only accessed from session's read goroutine (no synchronization needed).
	prefixCounts *prefixCounts

	// prefixMetrics is a reference to reactor-level Prometheus prefix metrics.
	// Set by Peer in runOnce(). Nil when metrics are not enabled.
	prefixMetrics *reactorMetrics

	// addrLabel caches settings.Address.String() to avoid per-message allocations.
	addrLabel string

	// onNotifSent is called when a NOTIFICATION is sent to the peer.
	// Set by Peer in runOnce() for Prometheus notification counter.
	onNotifSent func(code, subcode uint8)

	// onNotifRecv is called when a NOTIFICATION is received from the peer.
	// Set by Peer in runOnce() for Prometheus notification counter.
	onNotifRecv func(code, subcode uint8)

	// onOpenSent is called after an OPEN message is sent.
	onOpenSent func()

	// onOpenRecv is called after an OPEN message is received.
	onOpenRecv func()

	// onRefreshRecv is called after a ROUTE-REFRESH message is received.
	onRefreshRecv func()

	// onRead is called after any successful message read.
	onRead func()

	// onWrite is called after any successful message write.
	onWrite func()

	// onNegotiated is called after capability negotiation completes.
	// Receives the negotiated hold time and keepalive in seconds.
	onNegotiated func(holdSec, keepaliveSec uint32)

	// recentRead is set to true by the read loop on every successful message read.
	// The hold timer callback checks and clears it: if true, the daemon is
	// CPU-congested (data arrived but wasn't processed in time), so the hold
	// timer is extended instead of tearing down. Atomic for thread safety
	// between read goroutine and timer goroutine.
	recentRead atomic.Bool

	// Send Hold Timer (RFC 9687): detects when the local side cannot send.
	// sendHoldDeadline stores the UnixNano of the next expiry; 0 = not running.
	// Updated atomically on every write (zero-alloc hot path). A single timer
	// checks the deadline on expiry and reschedules if writes pushed it forward.
	sendHoldDeadline atomic.Int64
	sendHoldTimer    clock.Timer
	sendHoldMu       sync.Mutex // protects sendHoldTimer start/stop lifecycle

	// coalesce holds a pending IPv4 unicast UPDATE whose NLRIs may be extended
	// by subsequent UPDATEs with identical path attributes. Only used when
	// ze.bgp.reactor.coalesce=true. Accessed only from the read goroutine (no lock).
	coalesce        coalesceState
	coalesceEnabled bool
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
	if settings.OutTTL != 0 {
		dialer.OutTTL = settings.OutTTL
	}

	s := &Session{
		settings:        settings,
		fsm:             fsm.New(),
		timers:          fsm.NewTimers(),
		clock:           clock.RealClock{},
		dialer:          dialer,
		writeBuf:        wire.NewSessionBuffer(false), // Start with 4096, resize if Extended Message
		errChan:         make(chan error, 2),          // Buffer 2: normal error + teardown
		done:            make(chan struct{}),
		prefixCounts:    &prefixCounts{counts: make(map[uint32]int64), warned: make(map[uint32]bool)},
		coalesceEnabled: coalesceEnabled(),
		addrLabel:       settings.Address.String(),
	}

	// Configure FSM connection mode: passive if active bit is NOT set.
	s.fsm.SetPassive(!settings.Connection.IsActive())

	// Wire FSM to the session timers so the FSM handler for RFC 4271
	// Events 26/27 can restart the HoldTimer directly, per §8.2.2.
	s.fsm.SetTimers(s.timers)

	// Configure timers.
	s.timers.SetHoldTime(settings.ReceiveHoldTime)
	s.timers.SetKeepaliveTime(settings.KeepaliveTime)

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
			// GraceRearmHoldTimer, NOT ResetHoldTimer. fireHold has already
			// cleared holdRunning before invoking this callback, and
			// ResetHoldTimer early-returns on !holdRunning -- so calling it here
			// re-armed nothing and left the session with NO hold timer, silently
			// disabling dead-peer detection for the rest of its life. The grace
			// path is the one re-arm that is allowed to run post-expiry; it is
			// generation-checked, so a racing StopAll still wins.
			s.timers.GraceRearmHoldTimer(holdGraceExtension)
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
	if rd, ok := d.(*network.RealDialer); ok {
		s.dialer = s.mergedRealDialer(rd)
		return
	}
	s.dialer = d
}

func (s *Session) mergedRealDialer(base *network.RealDialer) *network.RealDialer {
	merged := *base
	if s.settings.LocalAddress.IsValid() {
		merged.LocalAddr = &net.TCPAddr{IP: s.settings.LocalAddress.AsSlice()}
	}
	if s.settings.MD5Key != "" {
		md5Addr := s.settings.Address
		if s.settings.MD5IP.IsValid() {
			md5Addr = s.settings.MD5IP
		}
		merged.PeerAddr = md5Addr.AsSlice()
		merged.MD5Key = s.settings.MD5Key
	}
	if s.settings.OutTTL != 0 {
		merged.OutTTL = s.settings.OutTTL
	}
	return &merged
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
		return bufMuxExt.Get()
	}
	return bufMuxStd.Get()
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

		if localID == remoteBGPID {
			// RFC 6286 §2.3: "If the BGP Identifiers of the peers involved in the
			// connection collision are identical, then the connection initiated by
			// the BGP speaker with the larger AS number is preserved."
			//
			// The pending connection is the one the remote initiated (it arrived
			// on our listener); the existing session is the one we initiated. So
			// the remote's connection is preserved exactly when its AS is larger.
			//
			// Do NOT assume an internal peer cannot reach this branch. An earlier
			// version of this comment claimed §2.2 (validateOpenIdentifier) had
			// already rejected one, and that equal AS numbers were therefore
			// impossible here. The ORDER is the other way round: DetectCollision
			// is called from ResolvePendingCollision (peer_connection.go) on the
			// pending OPEN, and §2.2 runs later, only on the connection that WINS
			// (session_connection.go). An internal peer's colliding connection
			// does arrive here with PeerAS == LocalAS.
			//
			// The outcome is still correct, which is why this was a wrong
			// justification rather than a wrong result: PeerAS > LocalAS is false
			// for equal AS numbers, so we keep the existing connection and reject
			// the pending one, and the §2.2 check still rejects the internal
			// peer's identifier afterwards on the winning connection. Keep that
			// reasoning intact if you touch this: it is the comparison, not an
			// upstream guarantee, that makes the equal-AS case safe.
			preserveRemote := s.collisionPeerAS() > s.settings.LocalAS
			return preserveRemote, preserveRemote
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

// collisionPeerAS returns the peer's AS number for the RFC 6286 Section 2.3
// equal-identifier tie-break, falling back to the AS the peer ADVERTISES when no
// AS is configured.
//
// A DYNAMIC peer carries PeerAS 0: buildDynamicPeerSettings sets it to 0
// (reactor_dynamic.go) and resolveDynamicPeerSettings only fills it on the
// Established transition (peer_run.go), which is strictly AFTER the OpenConfirm
// state this branch runs in. Reading that 0 raw makes `PeerAS > LocalAS` false
// against any real LocalAS, so the tie-break silently always preserved the local
// connection -- the zero value selecting a valid-looking answer
// (ai/rules/fail-closed-guards.md).
//
// This is the same fallback validateOpenIdentifier applies one function away
// (session_open_validation.go); that call site was fixed and this sibling was
// missed (ai/rules/before-writing-code.md, Sibling Call-Site Audit).
//
// Both colliding connections belong to the SAME peer, so the OPEN already
// received on this connection carries the AS that the pending connection would
// advertise too. openAdvertisedAS reads it through the AS4 capability rather
// than My AS, so a 4-byte-AS peer is judged on its real ASN and not on AS_TRANS
// (RFC 6793). peerOpen is guarded by s.mu (see the lock hierarchy above).
//
// LOCK ORDER: this takes s.mu while the caller holds p.mu -- the sole production
// caller is Peer.ResolvePendingCollision (peer_connection.go), which holds
// p.mu.Lock() across DetectCollision. That p.mu -> s.mu edge is outside the
// hierarchy documented above, which orders only the Session's own three locks, so
// it is recorded here (and in that block) rather than left to be re-derived.
//
// It cannot deadlock, because the reverse edge does not exist: no file under
// session*.go acquires p.mu, and the two callbacks a Peer installs on a Session
// (onMessageReceived, egressRouteFilter) are invoked from the write path inside
// writeMu and from the read goroutine, never with s.mu held. A future edit that
// takes p.mu from Session code -- or fires a peer callback under s.mu -- closes
// the cycle and deadlocks connection collision resolution.
func (s *Session) collisionPeerAS() uint32 {
	if as := s.settings.PeerAS; as != 0 {
		return as
	}

	s.mu.RLock()
	open := s.peerOpen
	s.mu.RUnlock()

	if open == nil {
		return 0
	}
	return openAdvertisedAS(open)
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

// ErrPolicyTeardown is returned when an import policy filter asked for the
// session to be torn down (e.g. filter_family). Deliberately DISTINCT from
// ErrTeardown: peer_run.go classifies ErrTeardown as an immediate reconnect,
// and a peer whose UPDATE tripped the filter typically re-offends on the first
// UPDATE of the next session, which turns immediate reconnect into a
// NOTIFICATION storm. This sentinel falls through to the exponential-backoff
// arm instead (spec-fixit-bgp-session-fsm-lifecycle D-7, resolved Q-4).
var ErrPolicyTeardown = errors.New("session teardown requested by import policy")

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

// policyTeardownRequest carries a NOTIFICATION code/subcode for a deferred
// session teardown requested by a policy filter (honored in session_read).
type policyTeardownRequest struct {
	code    message.NotifyErrorCode
	subcode uint8
}

// requestPolicyTeardown queues a NOTIFICATION + session close to run after the
// current received UPDATE's filter chain (honored in session_read, before
// handleUpdate). Called on the session read goroutine from the import policy
// filter chain (notifyMessageReceiver via onMessageReceived). The first request
// for a given UPDATE wins.
func (s *Session) requestPolicyTeardown(code message.NotifyErrorCode, subcode uint8) {
	if s.policyTeardownPending == nil {
		s.policyTeardownPending = &policyTeardownRequest{code: code, subcode: subcode}
	}
}

// takePolicyTeardown returns and clears any pending policy teardown request.
// Called on the session read goroutine after the onMessageReceived callback.
func (s *Session) takePolicyTeardown() *policyTeardownRequest {
	req := s.policyTeardownPending
	s.policyTeardownPending = nil
	return req
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

	// When the read loop exits the session is finished, so stop its timers: no keepalive,
	// hold, or send-hold goroutine may outlive Run. The message-driven teardown paths
	// (handleConnectionClose, handleNotification, Close) already StopAll, but a plain
	// ctx-cancel exit does not — which would leave the PERIODIC keepalive timer firing after
	// the session is dead, its callback reading state (e.g. the package-global sessionLogger)
	// long after Run returned. StopAll and stopSendHoldTimer are idempotent, so the redundant
	// call on the paths that already stop timers is harmless. Not a hot path (once per session
	// lifecycle).
	defer s.stopSendHoldTimer()
	defer s.timers.StopAll()

	// Same discipline for the CONNECTION: when the read loop exits, the TCP
	// connection is closed, whatever the exit path. Several error returns send
	// a NOTIFICATION and then return without closing — the OPEN unpack error
	// (session_handlers.go:87-91), the openValidator rejection (runOpenValidator
	// at session_open_validation.go:81-117, reached from session_handlers.go:139-141)
	// and the local-capability parse error (session_handlers.go:150-153) — and the
	// cancel goroutine exits on <-s.done without closing, so on those paths nothing
	// closed the socket at all. Per-site closeConn calls are the shape that produced
	// this bug class; one defer covers current and future exits. closeConn is
	// idempotent (nil-checked under s.mu), so the paths that already close are
	// unaffected (spec-fixit-bgp-session-fsm-lifecycle AC-7 / D-8).
	defer s.closeConn()

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

	defer s.resetCoalesce()

	for {
		// Backpressure pause gate: if paused, block until resumed or shutdown.
		// Fast path: atomic load returns false, zero overhead when not paused.
		if s.paused.Load() {
			if err := s.waitForResume(ctx); err != nil {
				return err
			}
		}

		// Capture conn + bufReader atomically. connectionEstablished writes
		// both under s.mu.Lock(); we MUST read both under s.mu.RLock() to get
		// a consistent pair. Passing bufReader through readAndProcessMessage
		// as a parameter (rather than reading s.bufReader inside it) closes
		// the data race between this reader and the locked write in
		// connectionEstablished.
		s.mu.RLock()
		conn := s.conn
		bufReader := s.bufReader
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
		var err error
		if s.coalesceEnabled {
			err = s.readAndProcessCoalesced(conn, bufReader)
		} else {
			err = s.readAndProcessMessage(conn, bufReader)
		}

		// Flush RS fast path writes when no more source data is available.
		// bufReader.Buffered() == 0 is the natural batch boundary (same
		// signal used by UPDATE coalescing). On error, flush unconditionally
		// so data does not sit in destination bufWriters.
		if err != nil || bufReader.Buffered() == 0 {
			s.flushFwdDirty()
		}

		if err != nil {
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
