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
)

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
	MaxTunnels    uint16        // 0 = unbounded (by this knob; uint16 still caps at 65535)
	MaxSessions   uint16        // 0 = unbounded per-tunnel session limit
	HelloInterval time.Duration // peer silence before HELLO; 0 = no keepalive
	Defaults      TunnelDefaults
	Clock         func() time.Time // injected for tests; time.Now if nil
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
	// Phase 5 kernel integration.
	kernelWorker *kernelWorker
	kernelErrCh  <-chan kernelSetupFailed

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
	tunnel := r.locateTunnelLocked(pkt.from, hdr, sccrq)
	if tunnel == nil {
		r.tunnelsMu.Unlock()
		return
	}
	tunnel.peerAddr = pkt.from
	now := r.params.Clock()
	outbound := tunnel.Process(hdr, payload, now, r.params.Defaults, sccrq)
	// lastActivity is set inside Process only when the engine delivers
	// at least one new message (not on duplicates/out-of-window).

	// Phase 5: collect kernel events while still holding tunnelsMu.
	setupEvents, teardownEvents := r.collectKernelEventsLocked(tunnel)

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
	localTID := tunnel.localTID
	r.tunnelsMu.Unlock()

	// Phase 5: enqueue kernel events after releasing the lock.
	r.enqueueKernelEvents(setupEvents, teardownEvents)

	for _, req := range outbound {
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

	localTID := tunnel.localTID
	r.tunnelsMu.Unlock()

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
		r.discardTunnelLocked(t, "retention expired")
		// Collect kernel teardowns queued by clearSessions inside
		// discardTunnelLocked. The tunnel is about to become unreachable.
		teardowns = append(teardowns, t.pendingKernelTeardowns...)
		t.pendingKernelTeardowns = nil
	}
	return expired, teardowns
}

// locateTunnelLocked resolves the target tunnel for an inbound control
// datagram. Returns nil if the packet should be dropped (unknown TID or
// max-tunnels limit reached). The caller MUST hold tunnelsMu; mutations
// to tunnelsByLocalID / tunnelsByPeer happen inline. For TunnelID=0
// the caller MUST pass a pre-validated sccrqInfo (parseSCCRQ has
// already run); no tunnel is created for malformed input.
func (r *L2TPReactor) locateTunnelLocked(from netip.AddrPort, hdr MessageHeader, sccrq *sccrqInfo) *L2TPTunnel {
	if hdr.TunnelID != 0 {
		t, ok := r.tunnelsByLocalID[hdr.TunnelID]
		if !ok {
			r.logger.Debug("l2tp: packet for unknown tunnel dropped",
				"from", from.String(), "tunnel-id", hdr.TunnelID)
			return nil
		}
		return t
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
		return existing
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
	if sccrq.TieBreakerPresent {
		if tunnel := r.resolveTieBreakerLocked(from, sccrq.TieBreakerValue); tunnel == nil {
			return nil
		}
	}
	// Max-tunnels enforcement. MaxTunnels == 0 means unbounded by this
	// knob; the map can still grow to 65535 (the uint16 TID ceiling).
	if r.params.MaxTunnels != 0 && uint16(len(r.tunnelsByLocalID)) >= r.params.MaxTunnels {
		r.logger.Warn("l2tp: max-tunnels limit reached; SCCRQ rejected",
			"from", from.String(), "limit", r.params.MaxTunnels)
		// Phase 3 drops; phase 4 will emit StopCCN Result Code 2.
		return nil
	}
	localTID, err := r.allocateLocalTID()
	if err != nil {
		r.logger.Warn("l2tp: local tunnel ID allocation failed; SCCRQ dropped",
			"from", from.String(), "error", err.Error())
		return nil
	}
	t := newTunnel(localTID, sccrq.AssignedTunnelID, from, ReliableConfig{RecvWindow: r.params.Defaults.RecvWindow}, r.logger)
	t.maxSessions = r.params.MaxSessions
	r.tunnelsByLocalID[localTID] = t
	r.tunnelsByPeer[key] = t
	r.logger.Info("l2tp: new tunnel created from SCCRQ",
		"from", from.String(), "local-tid", localTID, "peer-tid", sccrq.AssignedTunnelID)
	return t
}

// resolveTieBreakerLocked compares the new SCCRQ's Tie Breaker value
// against every existing tunnel from the same peer address that carries
// a Tie Breaker. Returns nil if the new SCCRQ must be dropped (it lost
// the comparison, or the values were equal and both sides discard). On
// success (new SCCRQ wins or has no conflict) returns a non-nil sentinel
// and performs any winning-side map cleanup inline.
//
// Caller MUST hold tunnelsMu. Called only when sccrq.TieBreakerPresent
// is true and newTB is non-nil.
func (r *L2TPReactor) resolveTieBreakerLocked(from netip.AddrPort, newTB []byte) *L2TPTunnel {
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
	for _, loser := range losers {
		r.discardTunnelLocked(loser, "tie-breaker lost")
	}
	if newLoses {
		r.logger.Info("l2tp: new SCCRQ discarded by tie breaker",
			"from", from.String())
		return nil
	}
	return sentinel
}

// discardTunnelLocked removes a tunnel from both lookup maps and marks it
// closed. Used by tie-breaker resolution; no StopCCN is emitted because
// the peer will observe its own tie-breaker loss symmetrically (or time
// out if our tunnel was the only one tracking it). Caller MUST hold
// tunnelsMu.
func (r *L2TPReactor) discardTunnelLocked(t *L2TPTunnel, reason string) {
	// Phase 5: clear sessions so kernel teardown events are queued.
	t.clearSessions()
	pk := peerKey{addr: t.peerAddr, tid: t.remoteTID}
	delete(r.tunnelsByLocalID, t.localTID)
	delete(r.tunnelsByPeer, pk)
	t.state = L2TPTunnelClosed
	r.logger.Info("l2tp: tunnel discarded",
		"local-tid", t.localTID, "peer", t.peerAddr.String(), "reason", reason)
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
			localTID:   tunnel.localTID,
			remoteTID:  tunnel.remoteTID,
			peerAddr:   tunnel.peerAddr,
			localSID:   sess.localSID,
			remoteSID:  sess.remoteSID,
			socketFD:   socketFD,
			lnsMode:    sess.lnsMode,
			sequencing: sess.sequencingRequired,
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
	for _, ev := range setups {
		r.kernelWorker.Enqueue(ev)
	}
	for _, ev := range teardowns {
		r.kernelWorker.Enqueue(ev)
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
// Called by the subsystem after creating the worker. Must be called
// before Start().
func (r *L2TPReactor) SetKernelWorker(w *kernelWorker, errCh <-chan kernelSetupFailed) {
	r.kernelWorker = w
	r.kernelErrCh = errCh
}
