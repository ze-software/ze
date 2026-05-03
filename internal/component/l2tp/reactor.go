// Design: docs/research/l2tpv2-ze-integration.md -- L2TP reactor pattern
// Related: listener.go -- source of incoming packets and outbound sender
// Related: tunnel.go -- dispatch target and FSM state holder
// Related: tunnel_fsm.go -- message handling inside a tunnel
// Related: subsystem.go -- owns the reactor's lifecycle

package l2tp

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"time"

	l2tpevents "codeberg.org/thomas-mangin/ze/internal/component/l2tp/events"
	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// pppDriverIface is the subset of *ppp.Driver the reactor uses. Defined
// as an interface so subsystem_test.go can plug in a fake without
// constructing a real ppp.Driver (which requires an iface backend).
type pppDriverIface interface {
	SessionsIn() chan<- ppp.StartSession
	EventsOut() <-chan ppp.Event
}

// pppEventTypeFreeze pins the set of ppp.Event concrete types the
// reactor's handlePPPEvent switch knows about. Bumping the count here
// without adding the new type to the switch will compile (Go allows
// wider array literals), but every author touching this file is
// expected to treat this assertion as a checklist: the length MUST
// equal the number of ppp.Event cases handled in handlePPPEvent. When
// spec-6b adds EventAuthRequest etc., bump the length AND add a case.
var _ = [6]ppp.Event{
	ppp.EventLCPUp{},
	ppp.EventLCPDown{},
	ppp.EventSessionUp{},
	ppp.EventSessionDown{},
	ppp.EventSessionRejected{},
	ppp.EventEchoRTT{},
}

// reauthIntervalFloor is the minimum PPP periodic re-auth interval the
// reactor will honor from operator config. Below this floor a re-auth
// storm (millisecond-scale Challenge round-trips) would starve the
// session of useful throughput and potentially dominate the log.
// Operators requesting a value below this floor are clamped up with a
// WARN. Programmatic callers (tests constructing `ppp.StartSession`
// directly) bypass this check because they are expected to know what
// they are doing.
const reauthIntervalFloor = 5 * time.Second

// clampReauthInterval parses the operator-supplied
// ze.l2tp.auth.reauth-interval env value, applies the safety floor,
// and returns the duration to thread into StartSession.ReauthInterval.
// An empty string or a zero/negative parsed value disables re-auth
// (returns 0); a malformed duration logs a WARN and disables re-auth.
// A positive value below reauthIntervalFloor is clamped up with a
// WARN. No parse-success, no clamp, no log.
func clampReauthInterval(logger *slog.Logger, raw string) time.Duration {
	if raw == "" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		logger.Warn("l2tp: invalid ze.l2tp.auth.reauth-interval; disabling re-auth",
			"value", raw, "err", err)
		return 0
	}
	if d <= 0 {
		return 0
	}
	if d < reauthIntervalFloor {
		logger.Warn("l2tp: ze.l2tp.auth.reauth-interval below safety floor; clamping",
			"value", raw, "floor", reauthIntervalFloor.String())
		return reauthIntervalFloor
	}
	return d
}

// peerKey uniquely identifies a tunnel during SCCRQ retransmit dedup.
// RFC 2661 S24.17 allows multiple tunnels between the same IP pair, so
// peer address alone is insufficient; the peer's Assigned Tunnel ID AVP
// disambiguates.
type peerKey struct {
	addr netip.AddrPort
	tid  uint16
}

// ReactorParams carries the per-reactor configuration. Constructed by
// the subsystem from parsed Parameters + hardcoded defaults (phase 3
// hardcodes host name and capabilities; phase 7 wires them through
// YANG).
type ReactorParams struct {
	MaxTunnels      uint16         // 0 = unbounded (by this knob; uint16 still caps at 65535)
	MaxSessions     uint16         // 0 = unbounded per-tunnel session limit
	AuthMethod      ppp.AuthMethod // PPP Auth-Protocol first advertised to new sessions
	AuthRequired    bool           // fail if LCP opens with AuthMethodNone
	HelloInterval   time.Duration  // peer silence before HELLO; 0 = no keepalive
	CQMEchoInterval time.Duration  // when >0, overrides PPP echo interval for CQM sampling
	Defaults        TunnelDefaults
	Clock           func() time.Time // injected for tests; time.Now if nil
}

// L2TPReactor is the single goroutine that owns the tunnel map and
// dispatches incoming datagrams to per-tunnel FSMs. All per-tunnel state
// (ReliableEngine, FSM state, peer addr:port) is mutated exclusively
// from this goroutine, which matches the phase-2 contract that
// ReliableEngine is not safe for concurrent use.
//
// tunnelsMu protects the two tunnel maps and the per-tunnel fields
// that tests introspect (state, peerAddr, peerHostName). Runtime hot
// paths (the reactor goroutine itself) still hold the lock for the
// brief moments of map mutation and tunnel state transition; tests
// grab it through TunnelCount/TunnelByLocalID/State accessors.
//
// Caller MUST call Stop after Start. Start is not idempotent; the
// underlying UDPListener must already be Start()ed before the reactor
// runs.
type L2TPReactor struct {
	listener *UDPListener
	logger   *slog.Logger
	params   ReactorParams

	tunnelsMu        sync.Mutex
	tunnelsByLocalID map[uint16]*L2TPTunnel
	tunnelsByPeer    map[peerKey]*L2TPTunnel
	nextLocalTID     uint16

	// Timer channels. Created by the reactor; the tunnelTimer goroutine
	// is owned by the subsystem, not the reactor, but the reactor creates
	// the channels at construction time so tests can work without a
	// subsystem. tickCh receives tick requests from the timer; updateCh
	// sends heap updates back to the timer.
	tickCh   chan tickReq
	updateCh chan heapUpdate

	// Kernel integration channels. nil on non-Linux or when no kernel
	// worker is configured. The reactor checks for nil before use.
	// Phase 5 kernel integration. kernelWorkerSet tracks whether
	// SetKernelWorker has been called so the guard catches second calls
	// even when both pointers happen to be nil.
	kernelWorker    *kernelWorker
	kernelErrCh     <-chan kernelSetupFailed
	kernelSuccessCh <-chan kernelSetupSucceeded
	kernelWorkerSet bool

	// Phase 6a: PPP driver dispatch. nil when no PPP driver is wired
	// (non-Linux, no iface backend, or test paths that exercise only the
	// kernel layer). pppEventsOut mirrors pppDriver.EventsOut() so the
	// run-loop select can safely read it; a nil channel blocks forever
	// when pppDriver is unset, which is the desired semantics.
	pppDriver    pppDriverIface
	pppEventsOut <-chan ppp.Event

	// spec-l2tp-7 Phase 6: optional route observer. When set, the
	// reactor invokes OnSessionIPUp on EventSessionIPAssigned and
	// OnSessionDown at session teardown time. nil when no observer is
	// installed (tests, subsystem disabled).
	routeObserver RouteObserver

	// spec-l2tp-8a: EventBus for emitting (l2tp, session-down) events.
	// Set by subsystem via SetEventBus before Start.
	eventBus ze.EventBus

	// diag-4: control packet capture ring.
	capture *CaptureRing

	mu      sync.Mutex
	stop    chan struct{}
	wg      sync.WaitGroup
	started bool
}

// NewL2TPReactor constructs a reactor bound to the given listener. The
// listener must be started before the reactor is started; the reactor
// does not manage the listener's lifecycle.
func NewL2TPReactor(listener *UDPListener, logger *slog.Logger, params ReactorParams) *L2TPReactor {
	if logger == nil {
		logger = slog.Default()
	}
	if params.Clock == nil {
		params.Clock = time.Now
	}
	if params.Defaults.RecvWindow == 0 {
		params.Defaults.RecvWindow = 16
	}
	if params.Defaults.HostName == "" {
		params.Defaults.HostName = "ze"
	}
	return &L2TPReactor{
		listener:         listener,
		logger:           logger,
		params:           params,
		tunnelsByLocalID: make(map[uint16]*L2TPTunnel),
		tunnelsByPeer:    make(map[peerKey]*L2TPTunnel),
		tickCh:           make(chan tickReq, 1),
		updateCh:         make(chan heapUpdate, 16),
	}
}

// EnableCapture allocates the capture ring. Called when YANG
// diagnostics.capture is true. No-op if already enabled.
func (r *L2TPReactor) EnableCapture() {
	if r.capture == nil {
		r.capture = NewCaptureRing()
	}
}

// CaptureSnapshot returns captured control messages. Nil-safe.
func (r *L2TPReactor) CaptureSnapshot(limit int, tunnelID uint16, peer string) []CaptureEntry {
	if r.capture == nil {
		return nil
	}
	return r.capture.Snapshot(limit, tunnelID, peer)
}

// Start launches the reactor goroutine. Returns an error if already started.
func (r *L2TPReactor) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return errors.New("l2tp: reactor already started")
	}
	r.stop = make(chan struct{})
	r.started = true
	r.wg.Add(1)
	go r.run()
	return nil
}

// Stop signals the reactor to exit and waits for it. Idempotent.
func (r *L2TPReactor) Stop() {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return
	}
	r.started = false
	stop := r.stop
	r.mu.Unlock()

	close(stop)
	r.wg.Wait()
}

// run is the reactor's main loop. It consumes packets from the listener,
// timer tick requests, and dispatches each accordingly. The loop exits
// when r.stop fires. On stop, any packets already buffered in the RX
// channel are drained with release() only so the listener's slot pool
// frees promptly.
func (r *L2TPReactor) run() {
	defer r.wg.Done()
	rx := r.listener.RX()
	for {
		select {
		case pkt, ok := <-rx:
			if !ok {
				return
			}
			r.handle(pkt)
		case tr := <-r.tickCh:
			r.handleTick(tr)
		case kerr := <-r.kernelErrCh:
			r.handleKernelError(kerr)
		case ksucc := <-r.kernelSuccessCh:
			r.handleKernelSuccess(ksucc)
		case ev := <-r.pppEventsOut:
			r.handlePPPEvent(ev)
		case <-r.stop:
			r.drainOnStop(rx)
			return
		}
	}
}

// drainOnStop releases every packet currently buffered in rx without
// dispatching it. Called from run() exclusively after r.stop has fired.
// Keeps the listener's slot pool available for readLoop's own shutdown
// path, which otherwise would wait on GC to reclaim abandoned closures.
// Bounded by rxPoolSize (the listener cannot produce more than that in
// flight at any moment).
func (r *L2TPReactor) drainOnStop(rx <-chan rxPacket) {
	for len(rx) > 0 {
		pkt, ok := <-rx
		if !ok {
			return
		}
		pkt.release()
	}
}

// handle processes one received datagram:
//   - Drop too-short and unsupported-version datagrams (phase 2 behavior).
//   - TunnelID == 0: parse the AVP body first; if the body is a
//     well-formed SCCRQ the reactor looks up or creates a tunnel keyed
//     by (peer addr:port, peer's Assigned Tunnel ID). A malformed body
//     is dropped BEFORE any state is allocated, so a peer cannot fill
//     our tunnel map with half-formed SCCRQs.
//   - TunnelID != 0: look up by local TID; update peerAddr per S24.19.
//   - Hand the packet to the tunnel's Process method which runs the
//     reliable engine and FSM, then send every resulting outbound
//     datagram AFTER releasing tunnelsMu so a slow UDP write does not
//     serialize inbound dispatch.
//
// The pool slot is released before return. `bytes` MUST NOT be retained
// past this call.
func (r *L2TPReactor) handle(pkt rxPacket) {
	defer pkt.release()

	if len(pkt.bytes) < 6 {
		r.logger.Debug("l2tp: short datagram dropped", "from", pkt.from.String(), "len", len(pkt.bytes))
		return
	}

	hdr, err := ParseMessageHeader(pkt.bytes)
	if err != nil {
		if errors.Is(err, ErrUnsupportedVersion) {
			ver := pkt.bytes[1] & 0x0F
			if ver == 3 {
				r.logger.Warn("l2tp: L2TPv3 peer rejected (StopCCN emission arrives in later phase)", "from", pkt.from.String())
				return
			}
			r.logger.Debug("l2tp: unsupported version dropped", "from", pkt.from.String(), "version", ver)
			return
		}
		r.logger.Debug("l2tp: malformed header dropped", "from", pkt.from.String(), "error", err.Error())
		return
	}
	if !hdr.IsControl {
		// Phase 3 does not touch data-plane packets; Linux's l2tp_ppp
		// kernel module handles those. Drop silently.
		return
	}
	payload := pkt.bytes[hdr.PayloadOff:int(hdr.Length)]

	if r.capture != nil {
		r.capture.AppendInbound(hdr.TunnelID, hdr.SessionID, extractMsgType(payload), pkt.from, int(hdr.Length), 0)
	}

	// For TunnelID=0 (expected to be SCCRQ) parse the full AVP body
	// BEFORE grabbing tunnelsMu. A malformed body is rejected here,
	// without allocating a tunnel entry or consuming a local TID.
	var sccrq *sccrqInfo
	if hdr.TunnelID == 0 {
		info, perr := parseSCCRQ(payload)
		if perr != nil {
			r.logger.Debug("l2tp: TunnelID=0 packet with malformed body dropped",
				"from", pkt.from.String(), "error", perr.Error())
			return
		}
		if info.MessageType != MsgSCCRQ {
			r.logger.Debug("l2tp: TunnelID=0 packet that is not SCCRQ dropped",
				"from", pkt.from.String(), "message-type", uint16(info.MessageType))
			return
		}
		sccrq = &info
	}

	// Hold tunnelsMu across dispatch so every per-tunnel mutation (map
	// insert, FSM state change, peerAddr update) is race-free with
	// test introspection. We release the lock BEFORE sending outbound
	// bytes because listener.Send may block on a full kernel TX queue.
	r.tunnelsMu.Lock()
	tunnel, discardTeardowns := r.locateTunnelLocked(pkt.from, hdr, sccrq)
	if tunnel == nil {
		r.tunnelsMu.Unlock()
		// Even if no tunnel is dispatched (peer lost the tie-breaker), the
		// loser tunnel may have queued kernel teardowns we must drain.
		r.enqueueKernelEvents(nil, discardTeardowns)
		return
	}
	tunnel.peerAddr = pkt.from
	now := r.params.Clock()
	stateBefore := tunnel.state
	outbound := tunnel.Process(hdr, payload, now, r.params.Defaults, sccrq)
	// lastActivity is set inside Process only when the engine delivers
	// at least one new message (not on duplicates/out-of-window).

	// Phase 5: collect kernel events while still holding tunnelsMu.
	// Tie-breaker losers add their teardowns into discardTeardowns above.
	setupEvents, teardownEvents := r.collectKernelEventsLocked(tunnel)
	teardownEvents = append(teardownEvents, discardTeardowns...)

	// Capture the tunnel's new deadline for the timer heap update.
	// If the tunnel just reached established and the engine has no
	// pending retransmits, schedule a HELLO deadline so the keepalive
	// timer is armed from the start.
	newDeadline := tunnel.engine.NextDeadline()
	if tunnel.state == L2TPTunnelEstablished && r.params.HelloInterval > 0 {
		helloDeadline := now.Add(r.params.HelloInterval)
		if newDeadline.IsZero() || helloDeadline.Before(newDeadline) {
			newDeadline = helloDeadline
		}
	}
	// spec-l2tp-9: capture tunnel info for event emission after unlock.
	stateAfter := tunnel.state
	localTID := tunnel.localTID
	peerAddr := tunnel.peerAddr.String()
	peerHostName := tunnel.peerHostName
	r.tunnelsMu.Unlock()

	// spec-l2tp-9 AC-1: emit tunnel lifecycle events after releasing lock.
	if r.eventBus != nil {
		if stateBefore != L2TPTunnelEstablished && stateAfter == L2TPTunnelEstablished {
			if _, err := l2tpevents.TunnelUp.Emit(r.eventBus, &l2tpevents.TunnelUpPayload{
				TunnelID:     localTID,
				PeerAddr:     peerAddr,
				PeerHostName: peerHostName,
			}); err != nil {
				r.logger.Warn("l2tp: tunnel-up emit failed", "error", err)
			}
		}
		if stateBefore != L2TPTunnelClosed && stateAfter == L2TPTunnelClosed {
			if _, err := l2tpevents.TunnelDown.Emit(r.eventBus, &l2tpevents.TunnelDownPayload{
				TunnelID: localTID,
				Reason:   "peer",
			}); err != nil {
				r.logger.Warn("l2tp: tunnel-down emit failed", "error", err)
			}
		}
	}

	// Phase 5: enqueue kernel events after releasing the lock.
	r.enqueueKernelEvents(setupEvents, teardownEvents)

	for _, req := range outbound {
		if r.capture != nil && len(req.bytes) > 12 {
			outSID := uint16(req.bytes[8])<<8 | uint16(req.bytes[9])
			r.capture.AppendOutbound(localTID, outSID, extractMsgType(req.bytes[12:]), req.to, len(req.bytes))
		}
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed",
				"to", req.to.String(), "len", len(req.bytes), "error", err.Error())
		}
	}
	// Notify the timer of the tunnel's new deadline. Non-blocking because
	// updateCh is buffered (16 slots); if it is full, the timer will catch
	// up on the next drain. A dropped update only delays a tick by one
	// retransmit interval, which is acceptable.
	select {
	case r.updateCh <- heapUpdate{tunnelID: localTID, deadline: newDeadline}:
	case <-r.stop:
	}
}

// handleTick processes a tick request from the timer goroutine. It runs
// the engine's Tick for the specified tunnel, sends any retransmits,
// checks the HELLO keepalive interval for established tunnels, handles
// TeardownRequired, and reaps expired closed tunnels. After processing,
// it sends a heapUpdate back to the timer with the tunnel's new deadline.
//
// The tick also serves as the reaper sweep: every tick examines ALL
// closed tunnels for expiry, not just the one that fired. This is cheap
// at phase-5 scale (tens of tunnels).
func (r *L2TPReactor) handleTick(tr tickReq) {
	now := r.params.Clock()
	r.tunnelsMu.Lock()

	// Reaper sweep: check all closed tunnels for expiry. The returned
	// IDs are notified to the timer AFTER releasing the lock.
	reaped, reapTeardowns := r.reapExpiredLocked(now)

	tunnel, ok := r.tunnelsByLocalID[tr.tunnelID]
	if !ok {
		// Tunnel was reaped or discarded between the timer firing and
		// this dispatch. Send a zero-deadline update to remove the
		// stale heap entry.
		r.tunnelsMu.Unlock()
		r.enqueueKernelEvents(nil, reapTeardowns)
		r.notifyReaped(reaped)
		select {
		case r.updateCh <- heapUpdate{tunnelID: tr.tunnelID}:
		case <-r.stop:
		}
		return
	}

	// Run engine.Tick for retransmission.
	result := tunnel.engine.Tick(now)
	stateBefore := tunnel.state
	var outbound []sendRequest

	if result.TeardownRequired {
		// Retransmit limit exhausted. Tear down the tunnel.
		if tunnel.state != L2TPTunnelClosed {
			outbound = append(outbound, tunnel.teardownStopCCN(now, resultGeneralError)...)
		}
	} else {
		// Queue retransmits produced by the engine.
		for _, wire := range result.Retransmits {
			outbound = append(outbound, sendRequest{to: tunnel.peerAddr, bytes: wire})
		}

		// HELLO keepalive check for established tunnels. Skip if the
		// engine already has outstanding retransmits: those serve as
		// keepalive signals, and adding a HELLO would consume an extra
		// retransmit slot that could cause premature TeardownRequired.
		if tunnel.state == L2TPTunnelEstablished && r.params.HelloInterval > 0 && tunnel.engine.Outstanding() == 0 {
			if !tunnel.lastActivity.IsZero() && now.Sub(tunnel.lastActivity) >= r.params.HelloInterval {
				outbound = append(outbound, tunnel.handleHelloTimer(now)...)
			}
		}
	}

	// Phase 5: collect kernel events (teardownStopCCN may have cleared sessions).
	_, tickTeardowns := r.collectKernelEventsLocked(tunnel)

	newDeadline := tunnel.engine.NextDeadline()
	// If the tunnel is established and has a HELLO interval, ensure
	// the deadline is at most helloInterval from now so the timer
	// fires for the next keepalive check.
	if tunnel.state == L2TPTunnelEstablished && r.params.HelloInterval > 0 {
		helloDeadline := now.Add(r.params.HelloInterval)
		if newDeadline.IsZero() || helloDeadline.Before(newDeadline) {
			newDeadline = helloDeadline
		}
	}

	stateAfter := tunnel.state
	localTID := tunnel.localTID
	r.tunnelsMu.Unlock()

	// spec-l2tp-9: emit tunnel-down when tick-driven teardown closes the tunnel.
	if r.eventBus != nil && stateBefore != L2TPTunnelClosed && stateAfter == L2TPTunnelClosed {
		if _, err := l2tpevents.TunnelDown.Emit(r.eventBus, &l2tpevents.TunnelDownPayload{
			TunnelID: localTID,
			Reason:   "retransmit-timeout",
		}); err != nil {
			r.logger.Warn("l2tp: tunnel-down emit failed", "error", err)
		}
	}

	// Phase 5: enqueue kernel teardown events after releasing the lock.
	r.enqueueKernelEvents(nil, append(reapTeardowns, tickTeardowns...))

	r.notifyReaped(reaped)
	for _, req := range outbound {
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed",
				"to", req.to.String(), "len", len(req.bytes), "error", err.Error())
		}
	}

	select {
	case r.updateCh <- heapUpdate{tunnelID: localTID, deadline: newDeadline}:
	case <-r.stop:
	}
}

// notifyReaped sends zero-deadline heap updates for reaped tunnel IDs.
// Called AFTER releasing tunnelsMu.
func (r *L2TPReactor) notifyReaped(ids []uint16) {
	for _, tid := range ids {
		select {
		case r.updateCh <- heapUpdate{tunnelID: tid}:
		case <-r.stop:
			return
		}
	}
}

// reapExpiredLocked removes all tunnels in the closed state whose
// engine retention window has elapsed. Returns the IDs of reaped
// tunnels so the caller can notify the timer AFTER releasing the lock,
// plus any kernel teardown events from reaped tunnels whose sessions
// had kernel resources.
// Caller MUST hold tunnelsMu.
func (r *L2TPReactor) reapExpiredLocked(now time.Time) ([]uint16, []kernelTeardownEvent) {
	// Collect IDs first to avoid modifying the map during iteration.
	var expired []uint16
	for tid, t := range r.tunnelsByLocalID {
		if t.state == L2TPTunnelClosed && t.engine.Expired(now) {
			expired = append(expired, tid)
		}
	}
	teardowns := make([]kernelTeardownEvent, 0, len(expired))
	for _, tid := range expired {
		t := r.tunnelsByLocalID[tid]
		// discardTunnelLocked drains and returns the tunnel's
		// pendingKernelTeardowns; the tunnel is about to become unreachable.
		teardowns = append(teardowns, r.discardTunnelLocked(t, "retention expired")...)
	}
	return expired, teardowns
}

// locateTunnelLocked resolves the target tunnel for an inbound control
// datagram. Returns nil if the packet should be dropped (unknown TID or
// max-tunnels limit reached). The caller MUST hold tunnelsMu; mutations
// to tunnelsByLocalID / tunnelsByPeer happen inline. For TunnelID=0
// the caller MUST pass a pre-validated sccrqInfo (parseSCCRQ has
// already run); no tunnel is created for malformed input.
//
// The second return value carries any kernel teardowns produced by
// discarding a tunnel during tie-breaker resolution (Phase 5). The
// caller MUST enqueue these to the kernel worker after releasing
// tunnelsMu, even when the tunnel return is nil.
func (r *L2TPReactor) locateTunnelLocked(from netip.AddrPort, hdr MessageHeader, sccrq *sccrqInfo) (*L2TPTunnel, []kernelTeardownEvent) {
	if hdr.TunnelID != 0 {
		t, ok := r.tunnelsByLocalID[hdr.TunnelID]
		if !ok {
			r.logger.Debug("l2tp: packet for unknown tunnel dropped",
				"from", from.String(), "tunnel-id", hdr.TunnelID)
			return nil, nil
		}
		return t, nil
	}
	// TunnelID=0: caller has already parsed and validated the SCCRQ.
	// sccrq.AssignedTunnelID is guaranteed non-zero by parseSCCRQ.
	if sccrq == nil {
		panic("BUG: locateTunnelLocked called with TunnelID=0 and nil sccrqInfo")
	}
	key := peerKey{addr: from, tid: sccrq.AssignedTunnelID}
	if existing, ok := r.tunnelsByPeer[key]; ok {
		// Retransmitted SCCRQ. Let the existing tunnel's reliable engine
		// handle dedup + ACK. INFO level so operators can see retransmit
		// pressure in a log -- legitimate first-contact retransmits are
		// rare enough that this does not flood.
		r.logger.Info("l2tp: SCCRQ retransmit matched existing tunnel",
			"from", from.String(), "peer-tid", sccrq.AssignedTunnelID, "local-tid", existing.localTID)
		return existing, nil
	}
	// Tie breaker resolution (RFC 2661 S9.5). When a second SCCRQ arrives
	// from the same peer address with a Tie Breaker AVP, compare it
	// byte-wise against tie breakers stored on any existing tunnel from
	// the same peer address. Lower value wins and keeps its tunnel;
	// higher value's SCCRQ is dropped. Equal means both discard (RFC:
	// "both peers' tunnels MUST be silently torn down").
	//
	// This runs only when BOTH the new SCCRQ and an existing tunnel carry
	// tie breakers; a peer that omits the AVP on either SCCRQ keeps both
	// tunnels per RFC 2661 S24.17 (multiple concurrent tunnels between
	// the same addr pair are legitimate).
	var teardowns []kernelTeardownEvent
	if sccrq.TieBreakerPresent {
		tunnel, tieTeardowns := r.resolveTieBreakerLocked(from, sccrq.TieBreakerValue)
		teardowns = tieTeardowns
		if tunnel == nil {
			return nil, teardowns
		}
	}
	// Max-tunnels enforcement. MaxTunnels == 0 means unbounded by this
	// knob; the map can still grow to 65535 (the uint16 TID ceiling).
	if r.params.MaxTunnels != 0 && uint16(len(r.tunnelsByLocalID)) >= r.params.MaxTunnels {
		r.logger.Warn("l2tp: max-tunnels limit reached; SCCRQ rejected",
			"from", from.String(), "limit", r.params.MaxTunnels)
		// Phase 3 drops; phase 4 will emit StopCCN Result Code 2.
		return nil, teardowns
	}
	localTID, err := r.allocateLocalTID()
	if err != nil {
		r.logger.Warn("l2tp: local tunnel ID allocation failed; SCCRQ dropped",
			"from", from.String(), "error", err.Error())
		return nil, teardowns
	}
	t := newTunnel(localTID, sccrq.AssignedTunnelID, from, ReliableConfig{RecvWindow: r.params.Defaults.RecvWindow}, r.logger, r.params.Clock())
	t.maxSessions = r.params.MaxSessions
	r.tunnelsByLocalID[localTID] = t
	r.tunnelsByPeer[key] = t
	r.logger.Info("l2tp: new tunnel created from SCCRQ",
		"from", from.String(), "local-tid", localTID, "peer-tid", sccrq.AssignedTunnelID)
	return t, teardowns
}

// resolveTieBreakerLocked compares the new SCCRQ's Tie Breaker value
// against every existing tunnel from the same peer address that carries
// a Tie Breaker. Returns nil tunnel if the new SCCRQ must be dropped (it
// lost the comparison, or the values were equal and both sides discard).
// On success (new SCCRQ wins or has no conflict) returns a non-nil
// sentinel. The teardowns return value carries kernel cleanup events
// from any discarded loser tunnels; the caller MUST enqueue them.
//
// Caller MUST hold tunnelsMu. Called only when sccrq.TieBreakerPresent
// is true and newTB is non-nil.
func (r *L2TPReactor) resolveTieBreakerLocked(from netip.AddrPort, newTB []byte) (*L2TPTunnel, []kernelTeardownEvent) {
	sentinel := &L2TPTunnel{} // non-nil "proceed" return value
	var losers []*L2TPTunnel
	newLoses := false
	for _, existing := range r.tunnelsByLocalID {
		if existing.peerAddr.Addr() != from.Addr() {
			continue
		}
		if existing.tieBreaker == nil {
			continue
		}
		cmp := bytes.Compare(newTB, existing.tieBreaker)
		if cmp < 0 {
			// New SCCRQ's tie breaker is lower -> new wins, existing discarded.
			losers = append(losers, existing)
			continue
		}
		if cmp > 0 {
			// Existing is lower -> existing wins, new SCCRQ dropped.
			newLoses = true
			continue
		}
		// Equal -> both sides discard (RFC 2661 S9.5).
		losers = append(losers, existing)
		newLoses = true
	}
	teardowns := make([]kernelTeardownEvent, 0, len(losers))
	for _, loser := range losers {
		teardowns = append(teardowns, r.discardTunnelLocked(loser, "tie-breaker lost")...)
	}
	if newLoses {
		r.logger.Info("l2tp: new SCCRQ discarded by tie breaker",
			"from", from.String())
		return nil, teardowns
	}
	return sentinel, teardowns
}

// discardTunnelLocked removes a tunnel from both lookup maps and marks it
// closed. Returns any kernel teardown events queued by clearSessions for
// established sessions that had kernel resources; the caller MUST
// enqueue them to the kernel worker. Caller MUST hold tunnelsMu.
func (r *L2TPReactor) discardTunnelLocked(t *L2TPTunnel, reason string) []kernelTeardownEvent {
	// Phase 5: clear sessions so kernel teardown events are queued.
	t.clearSessions()
	teardowns := t.pendingKernelTeardowns
	t.pendingKernelTeardowns = nil
	pk := peerKey{addr: t.peerAddr, tid: t.remoteTID}
	delete(r.tunnelsByLocalID, t.localTID)
	delete(r.tunnelsByPeer, pk)
	t.state = L2TPTunnelClosed
	r.logger.Info("l2tp: tunnel discarded",
		"local-tid", t.localTID, "peer", t.peerAddr.String(), "reason", reason)
	return teardowns
}

// allocateLocalTID picks a non-zero uint16 not already present in
// tunnelsByLocalID. It uses a monotonic counter with wrap-around that
// skips zero; on collision it scans forward up to 8 slots. Returns an
// error only if the 65535 address space is fully occupied (which
// coincides with max-tunnels at its ceiling).
func (r *L2TPReactor) allocateLocalTID() (uint16, error) {
	const maxProbe = 8
	for range maxProbe {
		r.nextLocalTID++
		if r.nextLocalTID == 0 {
			r.nextLocalTID = 1
		}
		if _, taken := r.tunnelsByLocalID[r.nextLocalTID]; !taken {
			return r.nextLocalTID, nil
		}
	}
	return 0, fmt.Errorf("l2tp: no free tunnel IDs after %d probes", maxProbe)
}

// TunnelCount returns the number of tunnels currently tracked by the
// reactor. Acquires tunnelsMu so tests may call it concurrently with
// reactor-goroutine map mutations.
func (r *L2TPReactor) TunnelCount() int {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	return len(r.tunnelsByLocalID)
}

// TunnelByLocalID returns the tunnel with the given local TID, or nil
// if none. Intended for tests; thread-safe.
func (r *L2TPReactor) TunnelByLocalID(tid uint16) *L2TPTunnel {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	return r.tunnelsByLocalID[tid]
}

// collectKernelEventsLocked scans the tunnel for sessions that need
// kernel setup and for pending kernel teardowns. Clears the flags and
// drains the teardown list. Caller MUST hold tunnelsMu.
func (r *L2TPReactor) collectKernelEventsLocked(tunnel *L2TPTunnel) ([]kernelSetupEvent, []kernelTeardownEvent) {
	if r.kernelWorker == nil {
		return nil, nil
	}

	var setups []kernelSetupEvent
	socketFD := -1 // resolved lazily below

	for _, sess := range tunnel.sessions {
		if !sess.kernelSetupNeeded {
			continue
		}
		if socketFD < 0 {
			fd, err := r.listener.SocketFD()
			if err != nil {
				r.logger.Warn("l2tp: cannot get socket fd for kernel setup", "error", err.Error())
				// Do NOT clear kernelSetupNeeded: retry on next dispatch.
				continue
			}
			socketFD = fd
		}
		sess.kernelSetupNeeded = false
		setups = append(setups, kernelSetupEvent{
			localTID:                   tunnel.localTID,
			remoteTID:                  tunnel.remoteTID,
			peerAddr:                   tunnel.peerAddr,
			localSID:                   sess.localSID,
			remoteSID:                  sess.remoteSID,
			socketFD:                   socketFD,
			lnsMode:                    sess.lnsMode,
			sequencing:                 sess.sequencingRequired,
			proxyInitialRecvLCPConfReq: sess.proxyInitialRecvLCPConfReq,
			proxyLastSentLCPConfReq:    sess.proxyLastSentLCPConfReq,
			proxyLastRecvLCPConfReq:    sess.proxyLastRecvLCPConfReq,
		})
	}

	teardowns := tunnel.pendingKernelTeardowns
	tunnel.pendingKernelTeardowns = nil

	return setups, teardowns
}

// enqueueKernelEvents sends setup and teardown events to the kernel
// worker. Called after releasing tunnelsMu.
func (r *L2TPReactor) enqueueKernelEvents(setups []kernelSetupEvent, teardowns []kernelTeardownEvent) {
	if r.kernelWorker == nil {
		return
	}
	// Index rather than range-copy: kernelSetupEvent grew past 128 bytes
	// when it gained the proxy LCP slices, making a value copy per
	// iteration wasteful (gocritic rangeValCopy).
	for i := range setups {
		r.kernelWorker.Enqueue(setups[i])
	}
	for i := range teardowns {
		r.kernelWorker.Enqueue(teardowns[i])
	}
}

// handleKernelSuccess processes a successful kernel-side session setup
// reported by the kernel worker. Builds a ppp.StartSession from the
// event and writes it to the PPP driver's SessionsIn channel.
//
// When pppDriver is nil (no iface backend configured, test paths,
// non-Linux platforms), the success is logged and the fds remain owned
// by the kernel worker; the worker will close them on TeardownAll.
func (r *L2TPReactor) handleKernelSuccess(ksucc kernelSetupSucceeded) {
	if r.pppDriver == nil {
		r.logger.Warn("l2tp: kernel session ready but no PPP driver wired; fds remain in worker",
			"tunnel-id", ksucc.localTID, "session-id", ksucc.localSID,
			"ppp-unit", ksucc.fds.unitNum)
		return
	}

	// PeerAddr is informational for ppp logs only. Look it up under
	// tunnelsMu so the read is consistent; if the tunnel was discarded
	// in the meantime, fall back to a zero-value addr.
	var peerAddr netip.AddrPort
	r.tunnelsMu.Lock()
	if tunnel, ok := r.tunnelsByLocalID[ksucc.localTID]; ok {
		peerAddr = tunnel.peerAddr
	}
	r.tunnelsMu.Unlock()

	// ze.l2tp.auth.timeout (registered in internal/component/config/environment.go)
	// bounds the PPP auth phase. spec-l2tp-7-subsystem will wire this to a
	// YANG leaf; until then the env var is the only config surface. Inline
	// parse rather than env.GetDuration so malformed operator input surfaces
	// as a WARN instead of silently falling back to 30s.
	authTimeout := 30 * time.Second
	if raw := env.Get("ze.l2tp.auth.timeout"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			authTimeout = d
		} else {
			r.logger.Warn("l2tp: invalid ze.l2tp.auth.timeout; falling back to 30s",
				"value", raw, "err", err)
		}
	}

	// ze.l2tp.auth.reauth-interval (spec-l2tp-6b-auth Phase 9). Zero
	// disables periodic CHAP re-auth (default). Same inline-parse
	// pattern as auth.timeout so operator typos are visible at WARN.
	// A positive value below reauthIntervalFloor (5s) is clamped up
	// to that floor with a WARN: values in the microsecond or
	// millisecond range would create a reauth storm that starves the
	// session of useful throughput.
	reauthInterval := clampReauthInterval(r.logger, env.Get("ze.l2tp.auth.reauth-interval"))

	// spec-l2tp-6c-ncp: NCP enablement and timeout drawn from env vars.
	// Defaults: both NCPs enabled, 30s ip-timeout. Inline parsing so
	// operator typos log at WARN instead of silently defaulting.
	disableIPCP := !env.GetBool("ze.l2tp.ncp.enable-ipcp", true)
	disableIPv6CP := !env.GetBool("ze.l2tp.ncp.enable-ipv6cp", true)
	ipTimeout := 30 * time.Second
	if raw := env.Get("ze.l2tp.ncp.ip-timeout"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			ipTimeout = d
		} else {
			r.logger.Warn("l2tp: invalid ze.l2tp.ncp.ip-timeout; falling back to 30s",
				"value", raw, "err", err)
		}
	}

	start := ppp.StartSession{
		TunnelID:            ksucc.localTID,
		SessionID:           ksucc.localSID,
		ChanFD:              ksucc.fds.chanFD,
		UnitFD:              ksucc.fds.unitFD,
		UnitNum:             ksucc.fds.unitNum,
		LNSMode:             ksucc.lnsMode,
		PeerAddr:            peerAddr,
		AuthMethod:          r.params.AuthMethod,
		AuthRequired:        r.params.AuthRequired,
		AuthTimeout:         authTimeout,
		ReauthInterval:      reauthInterval,
		DisableIPCP:         disableIPCP,
		DisableIPv6CP:       disableIPv6CP,
		IPTimeout:           ipTimeout,
		ProxyLCPInitialRecv: ksucc.proxyInitialRecvLCPConfReq,
		ProxyLCPLastSent:    ksucc.proxyLastSentLCPConfReq,
		ProxyLCPLastRecv:    ksucc.proxyLastRecvLCPConfReq,
		EchoInterval:        r.params.CQMEchoInterval,
	}

	ifaceName := fmt.Sprintf("ppp%d", ksucc.fds.unitNum)
	r.tunnelsMu.Lock()
	if tunnel, ok := r.tunnelsByLocalID[ksucc.localTID]; ok {
		if sess := tunnel.lookupSession(ksucc.localSID); sess != nil {
			sess.pppInterface = ifaceName
		}
	}
	r.tunnelsMu.Unlock()

	select {
	case r.pppDriver.SessionsIn() <- start:
	case <-r.stop:
	}
}

// handlePPPEvent reacts to a PPP lifecycle event. EventSessionDown and
// EventSessionRejected mean the L2TP session is no longer carrying PPP
// traffic; emit a CDN to the peer so the L2TP-side state matches.
// EventLCPUp / EventLCPDown / EventSessionUp are informational in 6a;
// subsystem-level metrics consume them in later phases.
//
// EXHAUSTIVENESS: every ppp.Event concrete type MUST appear in this
// switch. Adding a new event type (e.g., spec-6b's auth events) without
// updating this switch would silently fall through and hit the WARN
// below. The compile-time assertion in `var _ [...]` below freezes the
// set at the count the reactor knows about; bumping the count in a
// future spec forces the author to handle the new type here too.
func (r *L2TPReactor) handlePPPEvent(ev ppp.Event) {
	// spec-l2tp-9: EventEchoRTT carries LCP echo round-trip time
	// for CQM aggregation. Relay to EventBus.
	if echoRTT, ok := ev.(ppp.EventEchoRTT); ok {
		r.handleEchoRTT(echoRTT)
		return
	}

	// spec-l2tp-7 Phase 6: EventSessionIPAssigned drives the route
	// observer. Handled before the teardown switch so it does not
	// accidentally reach the "unknown ppp.Event" fallback.
	if ipAssigned, ok := ev.(ppp.EventSessionIPAssigned); ok {
		r.handleSessionIPAssigned(ipAssigned)
		return
	}

	var tid, sid uint16
	var reason string
	switch e := ev.(type) {
	case ppp.EventSessionDown:
		tid, sid, reason = e.TunnelID, e.SessionID, e.Reason
	case ppp.EventSessionRejected:
		tid, sid, reason = e.TunnelID, e.SessionID, e.Reason
	case ppp.EventLCPUp, ppp.EventLCPDown:
		return
	case ppp.EventSessionUp:
		r.handleSessionUp(e)
		return
	}
	if tid == 0 && sid == 0 {
		r.logger.Warn("l2tp: unknown ppp.Event type ignored; handlePPPEvent needs an updated switch",
			"type", fmt.Sprintf("%T", ev))
		return
	}

	r.tunnelsMu.Lock()
	tunnel, ok := r.tunnelsByLocalID[tid]
	if !ok {
		r.tunnelsMu.Unlock()
		return
	}
	sess := tunnel.lookupSession(sid)
	if sess == nil {
		r.tunnelsMu.Unlock()
		return
	}
	username := sess.username
	now := r.params.Clock()
	outbound := tunnel.teardownSession(sess, cdnResultGeneralError, now, r.logger)
	teardowns := tunnel.drainPendingKernelTeardowns()
	r.tunnelsMu.Unlock()

	// spec-l2tp-7 Phase 6: notify the route observer before the CDN
	// goes on the wire so subscriber routes are withdrawn promptly
	// even if the outbound send blocks.
	if r.routeObserver != nil {
		r.routeObserver.OnSessionDown(sid)
	}

	// spec-l2tp-8a: emit (l2tp, session-down) so the pool plugin
	// can release the allocated IP address.
	if r.eventBus != nil {
		if _, err := l2tpevents.SessionDown.Emit(r.eventBus, &l2tpevents.SessionDownPayload{
			TunnelID:  tid,
			SessionID: sid,
			Username:  username,
		}); err != nil {
			r.logger.Warn("l2tp: session-down emit failed", "error", err)
		}
	}

	r.logger.Info("l2tp: PPP requested session teardown; sending CDN",
		"tunnel-id", tid, "session-id", sid, "reason", reason)
	for _, req := range outbound {
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed (PPP teardown CDN)",
				"to", req.to.String(), "error", err.Error())
		}
	}
	r.enqueueKernelEvents(nil, teardowns)
}

// handleSessionIPAssigned records the NCP-negotiated peer IP on the
// session struct and calls RouteObserver.OnSessionIPUp. Called from
// handlePPPEvent for every EventSessionIPAssigned (once per family
// per session in dual-stack flows).
func (r *L2TPReactor) handleSessionIPAssigned(ev ppp.EventSessionIPAssigned) {
	r.tunnelsMu.Lock()
	tunnel, ok := r.tunnelsByLocalID[ev.TunnelID]
	if !ok {
		r.tunnelsMu.Unlock()
		return
	}
	sess := tunnel.lookupSession(ev.SessionID)
	if sess == nil {
		r.tunnelsMu.Unlock()
		return
	}
	var addr netip.Addr
	switch {
	case ev.Peer.IsValid():
		addr = ev.Peer
		sess.assignedAddr = ev.Peer
	case ev.Local.IsValid() && ev.InterfaceID != [8]byte{}:
		// IPv6CP negotiates only an interface identifier; derive an
		// fe80::/64 link-local for snapshot display.
		addr = ev.Local
		sess.assignedAddr = ev.Local
	}
	username := sess.username
	r.tunnelsMu.Unlock()

	if r.routeObserver != nil && addr.IsValid() {
		r.routeObserver.OnSessionIPUp(ev.SessionID, username, addr)
	}

	if addr.IsValid() {
		r.logger.Info("l2tp: session IP assigned",
			"tunnel-id", ev.TunnelID,
			"session-id", ev.SessionID,
			"username", username,
			"address", addr.String())
	}

	if r.eventBus != nil && addr.IsValid() {
		if _, err := l2tpevents.SessionIPAssigned.Emit(r.eventBus, &l2tpevents.SessionIPAssignedPayload{
			TunnelID:  ev.TunnelID,
			SessionID: ev.SessionID,
			Username:  username,
			PeerAddr:  addr.String(),
		}); err != nil {
			r.logger.Warn("l2tp: session-ip-assigned emit failed", "error", err)
		}
	}
}

// handleSessionUp emits the (l2tp, session-up) EventBus event when a
// PPP session completes LCP, auth, and all NCPs. The shaper plugin
// subscribes to this event to apply TC rules on the pppN interface.
func (r *L2TPReactor) handleSessionUp(ev ppp.EventSessionUp) {
	var ifaceName string
	r.tunnelsMu.Lock()
	if tunnel, ok := r.tunnelsByLocalID[ev.TunnelID]; ok {
		if sess := tunnel.lookupSession(ev.SessionID); sess != nil {
			ifaceName = sess.pppInterface
		}
	}
	r.tunnelsMu.Unlock()
	if ifaceName == "" {
		return
	}
	r.logger.Info("l2tp: PPP session up",
		"tunnel-id", ev.TunnelID,
		"session-id", ev.SessionID,
		"interface", ifaceName)
	if r.eventBus == nil {
		return
	}
	if _, err := l2tpevents.SessionUp.Emit(r.eventBus, &l2tpevents.SessionUpPayload{
		TunnelID:  ev.TunnelID,
		SessionID: ev.SessionID,
		Interface: ifaceName,
	}); err != nil {
		r.logger.Warn("l2tp: session-up emit failed", "error", err)
	}
}

// handleEchoRTT relays a PPP echo round-trip measurement to the
// EventBus for CQM aggregation (spec-l2tp-9-observer AC-3).
func (r *L2TPReactor) handleEchoRTT(ev ppp.EventEchoRTT) {
	if r.eventBus == nil {
		return
	}
	var username string
	r.tunnelsMu.Lock()
	if tunnel, ok := r.tunnelsByLocalID[ev.TunnelID]; ok {
		if sess := tunnel.lookupSession(ev.SessionID); sess != nil {
			username = sess.username
		}
	}
	r.tunnelsMu.Unlock()
	if _, err := l2tpevents.EchoRTT.Emit(r.eventBus, &l2tpevents.EchoRTTPayload{
		TunnelID:  ev.TunnelID,
		SessionID: ev.SessionID,
		RTT:       ev.RTT,
		Username:  username,
	}); err != nil {
		r.logger.Warn("l2tp: echo-rtt emit failed", "error", err)
	}
}

// handleKernelError processes a setup failure reported by the kernel
// worker. Grabs tunnelsMu, looks up the session, and sends a CDN to
// the peer if the session still exists.
func (r *L2TPReactor) handleKernelError(kerr kernelSetupFailed) {
	r.tunnelsMu.Lock()
	tunnel, ok := r.tunnelsByLocalID[kerr.localTID]
	if !ok {
		r.tunnelsMu.Unlock()
		return
	}
	sess := tunnel.lookupSession(kerr.localSID)
	if sess == nil {
		// Session was already removed (CDN arrived from peer concurrently).
		r.tunnelsMu.Unlock()
		return
	}
	now := r.params.Clock()
	outbound := tunnel.teardownSession(sess, cdnResultGeneralError, now, r.logger)
	r.tunnelsMu.Unlock()

	for _, req := range outbound {
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed (kernel error CDN)",
				"to", req.to.String(), "error", err.Error())
		}
	}
}

// SetKernelWorker configures the kernel worker for this reactor.
// Called by the subsystem after creating the worker. MUST be called
// before Start(); the goroutine creation barrier in Start synchronizes
// the writes here with reads in r.run().
//
// Calling SetKernelWorker more than once is a programmer error -- the
// reactor goroutine could observe a torn read of the channel triple.
// Panics on second call, even when arguments are nil.
//
// successCh may be nil for tests that exercise the failure path only.
func (r *L2TPReactor) SetKernelWorker(w *kernelWorker, errCh <-chan kernelSetupFailed, successCh <-chan kernelSetupSucceeded) {
	if r.kernelWorkerSet {
		panic("BUG: SetKernelWorker called twice on the same reactor")
	}
	r.kernelWorkerSet = true
	r.kernelWorker = w
	r.kernelErrCh = errCh
	r.kernelSuccessCh = successCh
}

// SetPPPDriver wires the reactor's success-event dispatch to a PPP
// driver. The reactor sends ppp.StartSession on the driver's
// SessionsIn() channel after every kernelSetupSucceeded event, and
// reads ppp.Event values from EventsOut() to react to peer-side
// teardown signals.
//
// MUST be called before Start(); the goroutine creation barrier in
// Start synchronizes the writes here with reads in r.run(). If never
// called, the reactor falls back to logging success events without
// dispatching, which is acceptable on non-Linux or when the iface
// backend is unavailable.
func (r *L2TPReactor) SetPPPDriver(d pppDriverIface) {
	r.pppDriver = d
	if d != nil {
		r.pppEventsOut = d.EventsOut()
	}
}
