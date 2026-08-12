// Design: docs/research/l2tpv2-ze-integration.md -- L2TP reactor pattern
// RFC: rfc/short/rfc2661.md -- RFC 2661 Sections 4.4.2, 4.4.3, 5.3, 5.5, 5.8, 7.2, 8.1
// Related: listener.go -- source of incoming packets and outbound sender
// Related: tunnel.go -- dispatch target and FSM state holder
// Related: tunnel_fsm.go -- message handling inside a tunnel
// Related: subsystem.go -- owns the reactor's lifecycle
// Related: reactor_kernel.go -- kernel and PPP event handling

package l2tp

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/pkg/ze"
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

// peerKey uniquely identifies a tunnel during SCCRQ retransmit dedup.
// RFC 2661 S5.3 allows multiple tunnels between the same IP pair, so
// peer address alone is insufficient; the peer's Assigned Tunnel ID AVP
// disambiguates.
type peerKey struct {
	addr netip.AddrPort
	tid  uint16
}

// reactorParams carries the per-reactor configuration. Constructed by
// the subsystem from parsed Parameters + hardcoded defaults (phase 3
// hardcodes host name and capabilities; phase 7 wires them through
// YANG).
type reactorParams struct {
	MaxTunnels      uint16         // 0 = unbounded (by this knob; uint16 still caps at 65535)
	MaxSessions     uint16         // 0 = unbounded per-tunnel session limit
	AuthMethod      ppp.AuthMethod // PPP Auth-Protocol first advertised to new sessions
	AuthRequired    bool           // fail if LCP opens with AuthMethodNone
	AuthTimeout     time.Duration  // PPP auth-phase timeout
	ReauthInterval  time.Duration  // periodic re-auth; 0 = disabled
	HelloInterval   time.Duration  // peer silence before HELLO; 0 = no keepalive
	HelloRetries    uint8          // dead-peer detection: unanswered HELLO intervals before teardown; 0 = disabled
	EnableIPCP      bool           // IPCP NCP enabled
	EnableIPv6CP    bool           // IPv6CP NCP enabled
	NCPTimeout      time.Duration  // NCP negotiation timeout
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
// grab it through TunnelCount/tunnelByLocalID/State accessors.
//
// Caller MUST call Stop after Start. Start is not idempotent; the
// underlying UDPListener must already be Start()ed before the reactor
// runs.
type L2TPReactor struct {
	listener *UDPListener
	logger   *slog.Logger
	params   reactorParams

	tunnelsMu        sync.Mutex
	tunnelsByLocalID map[uint16]*L2TPTunnel
	tunnelsByPeer    map[peerKey]*L2TPTunnel
	nextLocalTID     uint16

	// stopCCNLastSent bounds every StopCCN the reactor sends outside a
	// tunnel (answerZeroTunnelIDSCCRQ). One slot per hash of the source
	// IP, so a spoofed flood allocates nothing and the table never grows.
	// Read and written only on the reactor goroutine, which is the sole
	// caller of handle, so it needs no lock.
	stopCCNLastSent [stopCCNLimitSlots]time.Time

	// Timer channels. Created by the reactor; the tunnelTimer goroutine
	// is owned by the subsystem, not the reactor, but the reactor creates
	// the channels at construction time so tests can work without a
	// subsystem. tickCh receives tick requests from the timer; updateCh
	// sends heap updates back to the timer.
	tickCh   chan tickReq
	updateCh chan heapUpdate

	// dialCh carries Dial requests from foreign goroutines to the single
	// reactor goroutine, which is the sole writer of the tunnel map (R-2).
	// Buffered so brief dial bursts do not block callers before the run
	// loop picks them up.
	dialCh chan dialRequest

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
	capture    *captureRing
	rawCapture atomic.Pointer[RawCaptureRing]

	mu      sync.Mutex
	stop    chan struct{}
	wg      sync.WaitGroup
	started bool
}

// newL2TPReactor constructs a reactor bound to the given listener. The
// listener must be started before the reactor is started; the reactor
// does not manage the listener's lifecycle.
func newL2TPReactor(listener *UDPListener, logger *slog.Logger, params reactorParams) *L2TPReactor {
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
		nextLocalTID:     listener.bind.Port(),
		tunnelsByLocalID: make(map[uint16]*L2TPTunnel),
		tunnelsByPeer:    make(map[peerKey]*L2TPTunnel),
		tickCh:           make(chan tickReq, 1),
		updateCh:         make(chan heapUpdate, 16),
		dialCh:           make(chan dialRequest, dialChanDepth),
	}
}

// EnableCapture allocates the capture ring. Called when YANG
// diagnostics.capture is true. No-op if already enabled.
func (r *L2TPReactor) EnableCapture() {
	if r.capture == nil {
		r.capture = newCaptureRing()
	}
}

// CaptureSnapshot returns captured control messages. Nil-safe.
func (r *L2TPReactor) CaptureSnapshot(limit int, tunnelID uint16, peer string) []CaptureEntry {
	if r.capture == nil {
		return nil
	}
	return r.capture.Snapshot(limit, tunnelID, peer)
}

// EnableRawCapture allocates the raw byte capture ring for pcap export.
func (r *L2TPReactor) EnableRawCapture() {
	r.rawCapture.CompareAndSwap(nil, NewRawCaptureRing())
}

// DisableRawCapture releases the raw capture ring.
func (r *L2TPReactor) DisableRawCapture() {
	r.rawCapture.Store(nil)
}

// RawCaptureSnapshot returns raw captured bytes. Nil-safe.
func (r *L2TPReactor) RawCaptureSnapshot(limit int) []RawCaptureEntry {
	rc := r.rawCapture.Load()
	if rc == nil {
		return nil
	}
	return rc.Snapshot(limit)
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
		case dr := <-r.dialCh:
			r.handleDial(dr)
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
//   - TunnelID != 0: look up by local TID; update peerAddr to the
//     datagram source. RFC 2661 S8.1 requires the peer to keep its
//     address and port static for the life of the tunnel, so this is a
//     TOLERANCE of a peer that does not (a NAT rebind, most often), not
//     a rule S8.1 states.
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
		r.capture.appendInbound(hdr.TunnelID, hdr.SessionID, extractMsgType(payload), pkt.from, int(hdr.Length), 0)
	}
	if rc := r.rawCapture.Load(); rc != nil {
		rc.Append(0, pkt.bytes[:hdr.Length])
	}

	// For TunnelID=0 (expected to be SCCRQ) parse the full AVP body
	// BEFORE grabbing tunnelsMu. A malformed body is rejected here,
	// without allocating a tunnel entry or consuming a local TID.
	var sccrq *sccrqInfo
	if hdr.TunnelID == 0 {
		info, perr := parseSCCRQ(payload)
		if perr != nil {
			// RFC 2661 Section 4.4.3: the Assigned Tunnel ID is "a 2 octet
			// non-zero unsigned integer". A zero one is the single parse
			// failure ze answers; every other one keeps its silent drop.
			// The answer is emitted here, still before tunnelsMu, so it
			// allocates no tunnel entry and consumes no local TID.
			if errors.Is(perr, errZeroAssignedTunnelID) {
				r.answerZeroTunnelIDSCCRQ(pkt.from, hdr.Ns)
			}
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

	// Initiator orchestration: a dialed tunnel that just reached established
	// originates its pending call (ICRQ for a LAC incoming call, OCRQ for an
	// LNS outgoing call). Runs on the reactor goroutine under tunnelsMu, so
	// no second writer touches the tunnel map (R-2); the ICRQ/OCRQ send joins
	// this dispatch's outbound batch.
	if stateBefore != L2TPTunnelEstablished && tunnel.state == L2TPTunnelEstablished {
		outbound = append(outbound, r.placePendingCallLocked(tunnel, now)...)
	}

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

	// Withdraw subscriber routes for sessions a peer-initiated event just
	// tore down (incoming StopCCN clears the tunnel's sessions; CDN clears
	// one; tie-breaker losses discard a tunnel). Without this the redistribute
	// source keeps the /32 injected after the session is gone.
	r.notifyRouteObserverDown(teardownEvents)

	// Phase 5: enqueue kernel events after releasing the lock.
	r.enqueueKernelEvents(setupEvents, teardownEvents)

	for _, req := range outbound {
		if r.capture != nil && len(req.bytes) > 12 {
			outSID := uint16(req.bytes[8])<<8 | uint16(req.bytes[9])
			r.capture.appendOutbound(localTID, outSID, extractMsgType(req.bytes[12:]), req.to, len(req.bytes))
		}
		if rc := r.rawCapture.Load(); rc != nil {
			rc.Append(1, req.bytes)
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

// stopCCNLimitSlots is the width of the table that bounds every StopCCN
// ze sends outside a tunnel, and stopCCNLimitInterval is the minimum gap
// between two emissions charged to one slot. A source is
// mapped to a slot by hashing its IP address, so two addresses can share
// one budget: the sharing is deliberate, because it fails towards sending
// LESS. Fixed width is what keeps a spoofed flood from allocating.
const (
	stopCCNLimitSlots    = 256
	stopCCNLimitInterval = time.Second
)

// tidNoTunnel is the Assigned Tunnel ID ze puts in a StopCCN it sends
// before any tunnel exists. RFC 2661 Section 4.4.3 requires the AVP to
// carry a non-zero value and requires a StopCCN to repeat "the Assigned
// Tunnel ID AVP first sent to the receiving peer" -- ze has sent none
// here, so it needs a value it will never hand to a real tunnel.
// allocateLocalTID skips this one, so a peer that keeps talking to it
// reaches no tunnel of ours.
const tidNoTunnel uint16 = 0xFFFF

// stopCCNSlot maps a source IP address to a slot in stopCCNLastSent
// (FNV-1a over the 16-byte form, so v4 and v6 hash alike).
func stopCCNSlot(a netip.Addr) int {
	const (
		offset64 uint64 = 14695981039346656037
		prime64  uint64 = 1099511628211
	)
	b := a.As16()
	h := offset64
	for _, c := range b {
		h ^= uint64(c)
		h *= prime64
	}
	return int(h % stopCCNLimitSlots)
}

// answerZeroTunnelIDSCCRQ replies to an SCCRQ whose Assigned Tunnel ID
// AVP carried 0, which RFC 2661 Section 4.4.3 makes a protocol error, and
// bounds how often it does so.
//
// The datagram that reaches here is unauthenticated and its source
// address is spoofable, so an unconditional reply would make ze a
// reflector. One rule bounds it and it carries no exception: a slot is
// answered at most once per stopCCNLimitInterval, and every datagram
// above that is dropped in silence. A source address maps to exactly one
// slot, so no victim receives more than one StopCCN per interval, and the
// whole reactor emits at most stopCCNLimitSlots of them per interval.
//
// A source that owns a live tunnel gets no exemption. Nothing here proves
// return-routability: an attacker who knows the address of any current
// peer spoofs it, and an exemption keyed on that address would answer him
// at his own packet rate, aimed at that peer. What a real peer loses when
// it shares a busy slot is one diagnostic StopCCN. It retransmits, and a
// zero Assigned Tunnel ID could not have opened a tunnel anyway.
//
// Runs on the reactor goroutine, before tunnelsMu is taken for dispatch,
// and allocates no tunnel entry. A suppressed datagram takes no lock and
// walks no table. An emitted one takes the two capture-ring locks inside
// sendUnassociatedStopCCN, and no other.
//
// The refused datagram is not logged here. handle logs every dropped
// TunnelID=0 body, this sentinel included, immediately after this call
// returns, so a second line would only add one netip.AddrPort.String and
// one slog call to each datagram of a flood.
func (r *L2TPReactor) answerZeroTunnelIDSCCRQ(from netip.AddrPort, peerNs uint16) {
	now := r.params.Clock()
	slot := stopCCNSlot(from.Addr())
	if last := r.stopCCNLastSent[slot]; !last.IsZero() && now.Sub(last) < stopCCNLimitInterval {
		return
	}
	r.stopCCNLastSent[slot] = now
	r.sendUnassociatedStopCCN(from, peerNs)
}

// sendUnassociatedStopCCN transmits one StopCCN to a peer for which no
// tunnel exists and none is created. Every other StopCCN in ze leaves
// through teardownStopCCN, which needs a tunnel to hold the reliable
// engine; this one carries its own sequence numbers instead.
//
// RFC 2661 Section 4.4.3: "Before the Assigned Tunnel ID AVP is received
// from a peer, messages MUST be sent to that peer with a Tunnel ID value
// of 0 in the header of all control messages." The peer's Assigned Tunnel
// ID was 0, so ze has none and the header carries 0.
//
// Ns is 0 because this is the first control message ze sends on a control
// connection that never opened. Nr is the peer's Ns plus one, which is the
// sequence number ze would expect next (RFC 2661 Section 5.8).
func (r *L2TPReactor) sendUnassociatedStopCCN(to netip.AddrPort, peerNs uint16) {
	buf := GetBuf()
	defer PutBuf(buf)
	b := *buf
	// Result Code 2 says the Error Code names the problem; Error Code 3 is
	// "One of the field values was out of range or reserved field was
	// non-zero" (RFC 2661 Section 4.4.2).
	n := writeStopCCNBody(b[ControlHeaderLen:], tidNoTunnel, ResultCodeValue{
		Result:       resultProtocolError,
		ErrorPresent: true,
		Error:        errorValueOutOfRange,
	})
	total := ControlHeaderLen + n
	WriteControlHeader(b, 0, uint16(total), 0, 0, 0, peerNs+1)

	// Both capture rings, as every other outbound send does, so `ze diag
	// l2tp` shows this reply beside the SCCRQ that drew it. The header
	// carries Tunnel ID 0 and Session ID 0 because no tunnel exists.
	if r.capture != nil {
		r.capture.appendOutbound(0, 0, MsgStopCCN, to, total)
	}
	if rc := r.rawCapture.Load(); rc != nil {
		rc.Append(1, b[:total])
	}
	if err := r.listener.Send(to, b[:total]); err != nil {
		r.logger.Warn("l2tp: StopCCN for zero Assigned Tunnel ID SCCRQ not sent",
			"to", to.String(), "error", err.Error())
		return
	}
	r.logger.Info("l2tp: zero Assigned Tunnel ID SCCRQ answered with StopCCN",
		"to", to.String(), "result-code", resultProtocolError, "error-code", errorValueOutOfRange)
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

	// Dead-peer detection: an Established tunnel whose peer has not proven
	// liveness for HelloRetries * HelloInterval is declared dead. Liveness
	// is a delivered control message OR an acknowledgement of one of our
	// outstanding messages (including a ZLB ACK of a HELLO), tracked as
	// tunnel.lastLiveness. This fires before the reliable engine's
	// retransmit exhaustion (~31s with defaults) whenever the threshold is
	// shorter, giving fast teardown when a peer (e.g. xl2tpd) dies without
	// sending StopCCN. It is deliberately kept separate from the retransmit
	// backoff and gated on Established, so setup (pre-Established) and
	// teardown (Closed) retain the full retransmit budget.
	// RFC 2661 Section 5.5: HELLO is used as a keepalive for the control
	// channel.
	deadPeer := false
	if !result.TeardownRequired && tunnel.state == L2TPTunnelEstablished &&
		r.params.HelloRetries > 0 && r.params.HelloInterval > 0 &&
		!tunnel.lastLiveness.IsZero() {
		deadline := time.Duration(r.params.HelloRetries) * r.params.HelloInterval
		if now.Sub(tunnel.lastLiveness) >= deadline {
			deadPeer = true
		}
	}

	teardownReason := "retransmit-timeout"
	if result.TeardownRequired || deadPeer {
		// Retransmit limit exhausted, or dead-peer keepalive timeout. Tear
		// down the tunnel.
		if deadPeer {
			teardownReason = "keepalive-timeout"
			r.logger.Info("l2tp: dead peer; keepalive timeout teardown",
				"tunnel", tunnel.localTID,
				"hello-retries", r.params.HelloRetries,
				"hello-interval", r.params.HelloInterval.String(),
				"since-liveness", now.Sub(tunnel.lastLiveness).String())
		}
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
			Reason:   teardownReason,
		}); err != nil {
			r.logger.Warn("l2tp: tunnel-down emit failed", "error", err)
		}
	}

	// Withdraw subscriber routes for sessions this tick-driven teardown just
	// cleared (retransmit limit exhausted -> StopCCN). reapTeardowns belong
	// to tunnels closed in an earlier tick/packet and already withdrawn then.
	r.notifyRouteObserverDown(tickTeardowns)

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
	// Tie breaker resolution (RFC 2661 S4.4.3; S7.2 names the collision).
	// When a second SCCRQ arrives
	// from the same peer address with a Tie Breaker AVP, compare it
	// byte-wise against tie breakers stored on any existing tunnel from
	// the same peer address. Lower value wins and keeps its tunnel;
	// higher value's SCCRQ is dropped. Equal means both discard (RFC 2661
	// S4.4.3: "both sides MUST discard their tunnels").
	//
	// This runs only when BOTH the new SCCRQ and an existing tunnel carry
	// tie breakers; a peer that omits the AVP on either SCCRQ keeps both
	// tunnels per RFC 2661 S5.3 (multiple concurrent tunnels between
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
		// Equal -> both sides discard (RFC 2661 S4.4.3).
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
	// AC-4: a dialed tunnel that loses the tie-breaker (or expires in
	// retention) is discarded before it establishes, so its pending call is
	// never placed on a session and would otherwise be dropped silently. Fail
	// the blocking RPC with the discard reason (e.g. "tie-breaker lost").
	t.resolvePendingCall(callOutcome{err: fmt.Errorf("%w (%s)", errCallTunnelSetupFailed, reason)})
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
// skips zero and tidNoTunnel; on collision it scans forward up to 8
// slots. Returns an error only if the 65534 address space is fully
// occupied (which coincides with max-tunnels at its ceiling).
func (r *L2TPReactor) allocateLocalTID() (uint16, error) {
	const maxProbe = 8
	for range maxProbe {
		r.nextLocalTID++
		// tidNoTunnel is the Assigned Tunnel ID a tunnel-free StopCCN
		// carries, so no real tunnel may hold it: a peer that answers one
		// must not reach a stranger's tunnel.
		if r.nextLocalTID == 0 || r.nextLocalTID == tidNoTunnel {
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

// tunnelByLocalID returns the tunnel with the given local TID, or nil
// if none. Intended for tests; thread-safe.
func (r *L2TPReactor) tunnelByLocalID(tid uint16) *L2TPTunnel {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	return r.tunnelsByLocalID[tid]
}
