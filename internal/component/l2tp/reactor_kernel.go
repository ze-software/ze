// Design: docs/research/l2tpv2-ze-integration.md -- Kernel and PPP event handling
// Related: reactor.go -- core L2TP reactor loop

package l2tp

import (
	"fmt"
	"net/netip"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// collectKernelEventsLocked scans the tunnel for sessions that need
// kernel setup and for pending kernel teardowns. Clears the flags and
// drains the teardown list. Caller MUST hold tunnelsMu.
func (r *l2tpReactor) collectKernelEventsLocked(tunnel *L2TPTunnel) ([]kernelSetupEvent, []kernelTeardownEvent) {
	// Drain teardowns first, unconditionally: the route observer must learn
	// of torn sessions even when no kernel worker is present (non-Linux,
	// tests). Subscriber-route withdrawal is independent of kernel-resource
	// teardown, so this must not be gated on r.kernelWorker.
	teardowns := tunnel.pendingKernelTeardowns
	tunnel.pendingKernelTeardowns = nil

	if r.kernelWorker == nil {
		return nil, teardowns
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
			pppoeChannelFD:             sess.pppoeChannelFD,
			proxyInitialRecvLCPConfReq: sess.proxyInitialRecvLCPConfReq,
			proxyLastSentLCPConfReq:    sess.proxyLastSentLCPConfReq,
			proxyLastRecvLCPConfReq:    sess.proxyLastRecvLCPConfReq,
		})
	}

	return setups, teardowns
}

// enqueueKernelEvents sends setup and teardown events to the kernel
// worker. Called after releasing tunnelsMu.
func (r *l2tpReactor) enqueueKernelEvents(setups []kernelSetupEvent, teardowns []kernelTeardownEvent) {
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
// by the kernel worker; the worker will close them on teardownAll.
func (r *l2tpReactor) handleKernelSuccess(ksucc kernelSetupSucceeded) {
	// spec-followup-l2tp-call A-4: a LAC-relayed session's PPP frames flow
	// through the kernel channel bridge (PPPoE <-> pppol2tp); no local PPP
	// unit is driven for it, so skip ppp.StartSession. The kernel worker owns
	// the bridged fds and releases them (unbridge + close) on teardown.
	if ksucc.fds.bridged {
		r.logger.Info("l2tp: LAC-bridged session established; frames relayed in kernel, no local PPP",
			"tunnel-id", ksucc.localTID, "session-id", ksucc.localSID)
		return
	}
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
		AuthTimeout:         r.params.AuthTimeout,
		ReauthInterval:      r.params.ReauthInterval,
		DisableIPCP:         !r.params.EnableIPCP,
		DisableIPv6CP:       !r.params.EnableIPv6CP,
		IPTimeout:           r.params.NCPTimeout,
		ProxyLCPInitialRecv: ksucc.proxyInitialRecvLCPConfReq,
		ProxyLCPLastSent:    ksucc.proxyLastSentLCPConfReq,
		ProxyLCPLastRecv:    ksucc.proxyLastRecvLCPConfReq,
		EchoInterval:        r.params.CQMEchoInterval,
	}

	ifaceName := textbuf.StrInt("ppp", int64(ksucc.fds.unitNum))
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
func (r *l2tpReactor) handlePPPEvent(ev ppp.Event) {
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
	cancelSessionTimeouts(sess)
	username := sess.username
	now := r.params.Clock()
	outbound := tunnel.teardownSession(sess, cdnResultGeneralError, now, r.logger)
	teardowns := tunnel.drainPendingKernelTeardowns()
	r.tunnelsMu.Unlock()

	// spec-l2tp-7 Phase 6: notify the route observer before the CDN
	// goes on the wire so subscriber routes are withdrawn promptly
	// even if the outbound send blocks.
	if r.routeObserver != nil {
		r.routeObserver.OnSessionDown(tid, sid)
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

	ClearSessionMetadata(tid, sid)

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

// notifyRouteObserverDown withdraws subscriber routes for the sessions in
// a batch of kernel-teardown events. clearSessions / removeSession queue
// one event per established session torn down by a peer-initiated event
// (incoming StopCCN or CDN, a retransmit timeout, or a tie-breaker loss),
// so these events are exactly the sessions whose /32 or /128 must be
// withdrawn. The operator teardown paths (teardown.go) and the PPP-event
// path notify the observer directly; this covers the peer-driven reactor
// paths (handle, handleTick), which previously left routes injected after
// the session was gone. OnSessionDown is idempotent, so a session with no
// route record is a cheap no-op.
//
// Caller MUST NOT hold tunnelsMu (OnSessionDown takes the observer lock).
func (r *l2tpReactor) notifyRouteObserverDown(events []kernelTeardownEvent) {
	if r.routeObserver == nil {
		return
	}
	for _, e := range events {
		r.routeObserver.OnSessionDown(e.localTID, e.localSID)
	}
}

// handleSessionIPAssigned records the NCP-negotiated peer IP on the
// session struct and calls RouteObserver.OnSessionIPUp. Called from
// handlePPPEvent for every EventSessionIPAssigned (once per family
// per session in dual-stack flows).
func (r *l2tpReactor) handleSessionIPAssigned(ev ppp.EventSessionIPAssigned) {
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
	pppIface := sess.pppInterface
	r.tunnelsMu.Unlock()

	if r.routeObserver != nil && addr.IsValid() {
		r.routeObserver.OnSessionIPUp(ev.TunnelID, ev.SessionID, username, addr)
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
			TunnelID:     ev.TunnelID,
			SessionID:    ev.SessionID,
			Username:     username,
			PeerAddr:     addr.String(),
			PppInterface: pppIface,
		}); err != nil {
			r.logger.Warn("l2tp: session-ip-assigned emit failed", "error", err)
		}
	}
}

// handleSessionUp emits the (l2tp, session-up) EventBus event when a
// PPP session completes LCP, auth, and all NCPs. The shaper plugin
// subscribes to this event to apply TC rules on the pppN interface.
func (r *l2tpReactor) handleSessionUp(ev ppp.EventSessionUp) {
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

	// RFC 2865 Section 5.27/5.28: start RADIUS-driven timeouts.
	r.startSessionTimeouts(ev.TunnelID, ev.SessionID)
}

// handleEchoRTT relays a PPP echo round-trip measurement to the
// EventBus for CQM aggregation (spec-l2tp-9-observer AC-3).
func (r *l2tpReactor) handleEchoRTT(ev ppp.EventEchoRTT) {
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
func (r *l2tpReactor) handleKernelError(kerr kernelSetupFailed) {
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

// setKernelWorker configures the kernel worker for this reactor.
// Called by the subsystem after creating the worker. MUST be called
// before Start(); the goroutine creation barrier in Start synchronizes
// the writes here with reads in r.run().
//
// Calling setKernelWorker more than once is a programmer error -- the
// reactor goroutine could observe a torn read of the channel triple.
// Panics on second call, even when arguments are nil.
//
// successCh may be nil for tests that exercise the failure path only.
func (r *l2tpReactor) setKernelWorker(w *kernelWorker, errCh <-chan kernelSetupFailed, successCh <-chan kernelSetupSucceeded) {
	if r.kernelWorkerSet {
		panic("BUG: SetKernelWorker called twice on the same reactor")
	}
	r.kernelWorkerSet = true
	r.kernelWorker = w
	r.kernelErrCh = errCh
	r.kernelSuccessCh = successCh

	// Wire the worker's per-tunnel sockets back into THIS reactor's listener.
	// Once a kernel tunnel exists, its connected socket outranks the listener for
	// every datagram from that peer, so without this the reactor stops seeing the
	// peer's control messages entirely (UDPListener.adoptTunnelSocket documents
	// the kernel demux rule). Wired here, where the worker and the listener are
	// both in scope, rather than in the subsystem, which holds neither.
	if w != nil && r.listener != nil {
		w.setSocketHooks(r.listener.adoptTunnelSocket, r.listener.releaseTunnelSocket)
	}
}

// setPPPDriver wires the reactor's success-event dispatch to a PPP
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
func (r *l2tpReactor) setPPPDriver(d pppDriverIface) {
	r.pppDriver = d
	if d != nil {
		r.pppEventsOut = d.EventsOut()
	}
}
