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
// kernel setup and drains everything a session removal queued on it.
// Caller MUST hold tunnelsMu, and MUST deliver the downs as
// drainPendingTeardowns states.
func (r *l2tpReactor) collectKernelEventsLocked(tunnel *L2TPTunnel) ([]kernelSetupEvent, []kernelTeardownEvent, []sessionDown) {
	var setups []kernelSetupEvent
	if r.kernelWorker != nil {
		setups = r.kernelSetupsLocked(tunnel)
	}
	// Drain last, so the order ticket it issues is in the caller's hands the
	// moment it exists: a panic in the setup scan above takes no ticket with
	// it. Unconditionally: the route observer and the session-down readers
	// must learn of torn sessions even when no kernel worker is present
	// (non-Linux, tests), so the drain cannot be gated on r.kernelWorker.
	teardowns, downs := tunnel.drainPendingTeardowns()
	return setups, teardowns, downs
}

// kernelSetupsLocked answers the kernel setup event of every session of the
// tunnel that needs one, and clears its need. Caller MUST hold tunnelsMu, and
// MUST enqueue every event it answers.
func (r *l2tpReactor) kernelSetupsLocked(tunnel *L2TPTunnel) []kernelSetupEvent {
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
			session:                    sess,
			socketFD:                   socketFD,
			lnsMode:                    sess.lnsMode,
			sequencing:                 sess.sequencingRequired,
			pppoeChannelFD:             sess.pppoeChannelFD,
			realm:                      sess.realm,
			proxyInitialRecvLCPConfReq: sess.proxyInitialRecvLCPConfReq,
			proxyLastSentLCPConfReq:    sess.proxyLastSentLCPConfReq,
			proxyLastRecvLCPConfReq:    sess.proxyLastRecvLCPConfReq,
		})
	}
	return setups
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

	// A delayed setup may arrive after CDN or tunnel removal. Only the
	// still-live session may claim the worker's descriptors.
	var peerAddr netip.AddrPort
	var policy reactorParams
	r.tunnelsMu.Lock()
	tunnel := r.tunnelsByLocalID[ksucc.localTID]
	live := tunnel != nil && ksucc.session != nil && tunnel.lookupSession(ksucc.localSID) == ksucc.session
	if live {
		peerAddr = tunnel.peerAddr
		policy = r.params
	}
	r.tunnelsMu.Unlock()
	if !live || r.kernelWorker == nil {
		return
	}
	chanFD, unitFD, claimed := r.kernelWorker.takePPPDescriptors(ksucc)
	if !claimed {
		return
	}

	start := ppp.StartSession{
		TunnelID:            ksucc.localTID,
		SessionID:           ksucc.localSID,
		ChanFD:              chanFD,
		UnitFD:              unitFD,
		UnitNum:             ksucc.fds.unitNum,
		LNSMode:             ksucc.lnsMode,
		PeerAddr:            peerAddr,
		AuthMethod:          policy.AuthMethod,
		MaxMRU:              policy.MRU,
		AuthFallbackOrder:   policy.AuthAllow,
		AuthRequired:        policy.AuthRequired,
		AuthTimeout:         policy.AuthTimeout,
		ReauthInterval:      policy.ReauthInterval,
		DisableIPCP:         !policy.EnableIPCP,
		DisableIPv6CP:       !policy.EnableIPv6CP,
		IPTimeout:           policy.NCPTimeout,
		ProxyLCPInitialRecv: ksucc.proxyInitialRecvLCPConfReq,
		ProxyLCPLastSent:    ksucc.proxyLastSentLCPConfReq,
		ProxyLCPLastRecv:    ksucc.proxyLastRecvLCPConfReq,
		// RFC 2661 Section 4.4.5: the operator's LNS policy for proxy LCP.
		RenegotiateProxyLCP: policy.ProxyLCP == ProxyLCPRenegotiate,
		EchoInterval:        policy.CQMEchoInterval,
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
		_ = ppp.NewFDFile(unitFD, "ppp.canceled.unit").Close() //nolint:errcheck // canceled ownership transfer
		_ = ppp.NewFDFile(chanFD, "ppp.canceled.chan").Close() //nolint:errcheck // canceled ownership transfer
	}
}

// handlePPPEvent reacts to a PPP lifecycle event. EventSessionDown and
// EventSessionRejected mean the L2TP session is no longer carrying PPP
// traffic; emit a CDN to the peer so the L2TP-side state matches.
// LCP events publish and withdraw the negotiated session values;
// EventSessionUp publishes the authenticated network lifetime.
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
	var cause l2tpevents.TerminateCause
	switch e := ev.(type) {
	case ppp.EventSessionDown:
		tid, sid, reason, cause = e.TunnelID, e.SessionID, e.Reason, e.Cause
	case ppp.EventSessionRejected:
		// A rejected StartSession never ran a session goroutine: the refusals
		// are invalid file descriptors and duplicate session keys, which are
		// the NAS's own errors. RFC 2866 Section 5.10 value 9.
		tid, sid, reason, cause = e.TunnelID, e.SessionID, e.Reason, l2tpevents.TerminateCauseNASError
	case ppp.EventLCPDown:
		r.handleLCPDown(e)
		return
	case ppp.EventLCPUp:
		r.handleLCPUp(e)
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
	now := r.params.Clock()
	outbound := tunnel.teardownSession(sess, cdnResultGeneralError, cause, now, r.logger)
	teardowns, downs := tunnel.drainPendingTeardowns()
	r.tunnelsMu.Unlock()

	// spec-l2tp-7 Phase 6: withdraw the subscriber route before the CDN
	// goes on the wire so it is withdrawn promptly even if the outbound
	// send blocks. spec-l2tp-8a: then publish (l2tp, session-down) so
	// the session's readers release what they hold.
	r.deliverSessionDowns(downs)

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

// handleLCPUp retains the actual peer receive MRU and selected method from the
// driver. Normal and accepted proxy negotiation use the same event contract.
func (r *l2tpReactor) handleLCPUp(ev ppp.EventLCPUp) {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	tunnel, ok := r.tunnelsByLocalID[ev.TunnelID]
	if !ok {
		return
	}
	sess := tunnel.lookupSession(ev.SessionID)
	if sess == nil {
		return
	}
	sess.negotiatedMRU = ev.NegotiatedMRU
	sess.negotiatedAuthMethod = ev.NegotiatedAuthMethod
	// The outcome is decided once per session, before LCP first opens, and
	// every later reopen reports the same value; an Unknown recorded outcome
	// is therefore what marks the first open, which the metric counts.
	firstOpen := sess.proxyLCPOutcome == ppp.ProxyLCPOutcomeUnknown
	sess.proxyLCPOutcome = ev.ProxyLCP
	if !firstOpen {
		return
	}
	// refuse-by-default: guards countSessionLCPOpened -- an event with no outcome is no session-establishment fact to count
	if ev.ProxyLCP == ppp.ProxyLCPOutcomeUnknown {
		r.logger.Warn("l2tp: LCP opened with no proxy LCP outcome; not counted",
			"tunnel-id", ev.TunnelID, "session-id", ev.SessionID)
		return
	}
	countSessionLCPOpened(ev.ProxyLCP)
}

// handleLCPDown clears negotiated values and withdraws any network lifetime
// while keeping the L2TP transport available for the resumed LCP exchange.
// The withdrawal and its (l2tp, session-down) are delivered in the turn of the
// tunnel's order ticket taken under tunnelsMu; the deferred Acknowledge runs
// after that delivery returned.
func (r *l2tpReactor) handleLCPDown(ev ppp.EventLCPDown) {
	defer ev.Acknowledge()
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
	sess.negotiatedMRU = 0
	sess.negotiatedAuthMethod = ppp.AuthMethodNone
	if !ev.NetworkPhase {
		r.tunnelsMu.Unlock()
		return
	}
	username := sess.username
	sess.assignedAddr = netip.Addr{}
	sess.username = ""
	cancelSessionTimeouts(sess)
	if !r.hasSessionNoticeReader() {
		r.tunnelsMu.Unlock()
		ClearSessionMetadata(ev.TunnelID, ev.SessionID)
		return
	}
	ticket := tunnel.issueNoticeTicket()
	r.tunnelsMu.Unlock()

	ticket.deliver(func() { r.notifyNetworkLifetimeDown(ev.TunnelID, ev.SessionID, username) })
	ClearSessionMetadata(ev.TunnelID, ev.SessionID)
}

// hasSessionNoticeReader reports whether a session notification has anybody
// to reach: the route observer or the event bus. A path with nobody to reach
// takes no order ticket.
func (r *l2tpReactor) hasSessionNoticeReader() bool {
	if r.routeObserver != nil {
		return true
	}
	return r.eventBus != nil
}

// notifyNetworkLifetimeDown withdraws the subscriber route of a session whose
// LCP went down in the network phase, and publishes the (l2tp, session-down)
// that ends its network lifetime. Runs inside handleLCPDown's ticket turn.
func (r *l2tpReactor) notifyNetworkLifetimeDown(tid, sid uint16, username string) {
	if r.routeObserver != nil {
		r.routeObserver.OnSessionDown(tid, sid)
	}
	if r.eventBus == nil {
		return
	}
	if _, err := l2tpevents.SessionDown.Emit(r.eventBus, &l2tpevents.SessionDownPayload{
		TunnelID: tid, SessionID: sid, Username: username,
		Cause: l2tpevents.TerminateCauseUserRequest,
	}); err != nil {
		r.logger.Warn("l2tp: network lifetime withdrawal failed", "error", err)
	}
}

// handleSessionIPAssigned records the NCP-negotiated peer IP on the
// session struct, calls RouteObserver.OnSessionIPUp and publishes
// (l2tp, session-ip-assigned), both in the turn of the tunnel's order ticket
// it took under tunnelsMu, so a teardown that follows on another goroutine
// is delivered after them. Called from handlePPPEvent for every
// EventSessionIPAssigned (once per family per session in dual-stack flows).
// The consumer MUST acknowledge after synchronous publication completes, before
// PPP can enable forwarding: the deferred Acknowledge runs after the
// delivery returned.
func (r *l2tpReactor) handleSessionIPAssigned(ev ppp.EventSessionIPAssigned) {
	defer ev.Acknowledge()

	if ev.Peer.IsValid() && !ev.Peer.Is4() || !ev.Peer.IsValid() && ev.InterfaceID == [8]byte{} {
		r.logger.Warn("l2tp: invalid IP-assigned event", "address", ev.Peer, "session", ev.SessionID)
		return
	}
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
	if ev.Peer.Is4() {
		addr = ev.Peer
		sess.assignedAddr = ev.Peer
	}
	if ev.Username != "" {
		sess.username = ev.Username
	}
	username := sess.username
	pppIface := sess.pppInterface
	callingNumber := sess.callingNumber
	tellObserver := r.routeObserver != nil && addr.IsValid()
	publish := r.eventBus != nil && (addr.IsValid() || ev.InterfaceID != [8]byte{})
	owed := tellObserver || publish
	if !owed {
		r.tunnelsMu.Unlock()
		r.logIPAssigned(ev, username, addr)
		return
	}
	// The ticket is the last step under tunnelsMu and its delivery the first
	// after it, so nothing that can panic runs between the two.
	ticket := tunnel.issueNoticeTicket()
	r.tunnelsMu.Unlock()

	ticket.deliver(func() {
		r.logIPAssigned(ev, username, addr)
		if tellObserver {
			r.routeObserver.OnSessionIPUp(ev.TunnelID, ev.SessionID, username, addr)
		}
		if !publish {
			return
		}
		peerAddr := ""
		if addr.IsValid() {
			peerAddr = addr.String()
		}
		if _, err := l2tpevents.SessionIPAssigned.Emit(r.eventBus, &l2tpevents.SessionIPAssignedPayload{
			TunnelID:         ev.TunnelID,
			SessionID:        ev.SessionID,
			Username:         username,
			PeerAddr:         peerAddr,
			CallingStationID: callingNumber,
			PppInterface:     pppIface,
			InterfaceID:      ev.InterfaceID,
			DNSPrimary:       ev.DNSPrimary,
			DNSSecondary:     ev.DNSSecondary,
		}); err != nil {
			r.logger.Warn("l2tp: session-ip-assigned emit failed", "error", err)
		}
	})
}

// logIPAssigned records the IPv4 address IPCP assigned a session. An event
// that carries only an IPv6 interface identifier logs nothing.
func (r *l2tpReactor) logIPAssigned(ev ppp.EventSessionIPAssigned, username string, addr netip.Addr) {
	if !addr.IsValid() {
		return
	}
	r.logger.Info("l2tp: session IP assigned",
		"tunnel-id", ev.TunnelID,
		"session-id", ev.SessionID,
		"username", username,
		"address", addr.String())
}

// handleSessionUp emits the (l2tp, session-up) EventBus event when a
// PPP session completes LCP, auth, and all NCPs. The shaper plugin
// subscribes to this event to apply TC rules on the pppN interface. The event
// is published in the turn of the tunnel's order ticket taken under
// tunnelsMu, so a teardown that follows on another goroutine is delivered
// after it. The RADIUS timeouts start after the publication returned.
func (r *l2tpReactor) handleSessionUp(ev ppp.EventSessionUp) {
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
	ifaceName := sess.pppInterface
	if ifaceName == "" {
		r.tunnelsMu.Unlock()
		return
	}
	if r.eventBus == nil {
		r.tunnelsMu.Unlock()
		r.logSessionUp(ev, ifaceName)
		return
	}
	// The ticket is the last step under tunnelsMu and its delivery the first
	// after it, so nothing that can panic runs between the two.
	ticket := tunnel.issueNoticeTicket()
	r.tunnelsMu.Unlock()

	ticket.deliver(func() {
		r.logSessionUp(ev, ifaceName)
		if _, err := l2tpevents.SessionUp.Emit(r.eventBus, &l2tpevents.SessionUpPayload{
			TunnelID:  ev.TunnelID,
			SessionID: ev.SessionID,
			Interface: ifaceName,
		}); err != nil {
			r.logger.Warn("l2tp: session-up emit failed", "error", err)
		}
	})

	// RFC 2865 Section 5.27/5.28: start RADIUS-driven timeouts.
	r.startSessionTimeouts(ev.TunnelID, ev.SessionID)
}

// logSessionUp records that a session's PPP reached session-up.
func (r *l2tpReactor) logSessionUp(ev ppp.EventSessionUp, ifaceName string) {
	r.logger.Info("l2tp: PPP session up",
		"tunnel-id", ev.TunnelID,
		"session-id", ev.SessionID,
		"interface", ifaceName)
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
	// RFC 2866 Section 5.10 value 9, NAS Error: "NAS detected some error
	// (other than on the port) which required ending the session." Kernel
	// setup failed inside ze, before PPP ran and before any address was
	// assigned, so no subscriber holds state for this session and every
	// subscriber of the event will find none. It is emitted anyway because
	// the session did end, and a removal that tells nobody is the defect
	// this path is drained to prevent.
	outbound := tunnel.teardownSession(sess, cdnResultGeneralError,
		l2tpevents.TerminateCauseNASError, now, r.logger)
	teardowns, downs := tunnel.drainPendingTeardowns()
	r.tunnelsMu.Unlock()

	r.deliverSessionDowns(downs)
	for _, req := range outbound {
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed (kernel error CDN)",
				"to", req.to.String(), "error", err.Error())
		}
	}
	r.enqueueKernelEvents(nil, teardowns)
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
