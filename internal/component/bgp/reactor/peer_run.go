// Design: docs/architecture/core-design.md — peer run loop and session lifecycle
// Overview: peer.go — Peer struct, accessors, lifecycle API

package reactor

import (
	"errors"
	"fmt"
	"runtime"
	"time"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
)

// run is the main peer loop.
//
// This loop replaces the RFC 4271 ConnectRetryTimer (Event 9) with exponential
// backoff. The RFC's timer assumes non-blocking TCP connections where the FSM
// sits in Connect/Active waiting for both TCP completion and the retry timer.
// Ze uses blocking DialContext, so the session either connects or fails before
// returning. The retry delay between attempts is managed here at the Peer level
// with exponential backoff (min 5s → max 60s), which is more robust than the
// RFC's fixed 120s ConnectRetryTimer.
func (p *Peer) run() {
	defer p.wg.Done()
	defer p.cleanup()

	delay := p.reconnectMin
	attempt := 0

	for {
		select {
		case <-p.ctx.Done():
			return
		default: // no cancellation pending
		}

		attempt++
		attemptStart := p.clock.Now()
		peerLabel := p.peerAddrLabel()
		peerLogger().Debug("timing: connection attempt starting",
			"peer", p.settings.Address,
			"port", p.settings.Port,
			"attempt", attempt,
		)
		if p.reactor != nil && p.reactor.rmetrics != nil {
			p.reactor.rmetrics.peerConnectAttempts.With(peerLabel).Inc()
		}

		// Attempt connection with panic recovery.
		// Any panic within the session lifecycle (connect, FSM, message handling)
		// is caught and treated as a connection error, triggering reconnect with
		// backoff rather than killing the peer goroutine. This matches ExaBGP's
		// failure domain model: per-peer faults cause session teardown, not daemon crash.
		err := p.safeRunOnce()

		select {
		case <-p.ctx.Done():
			return
		default: // no cancellation pending
		}

		sessionElapsed := p.clock.Now().Sub(attemptStart)
		peerLogger().Debug("timing: safeRunOnce returned",
			"peer", p.settings.Address,
			"attempt", attempt,
			"elapsed", sessionElapsed,
			"error", err,
		)
		if p.reactor != nil && p.reactor.rmetrics != nil {
			p.reactor.rmetrics.peerConnectAttemptSeconds.With(peerLabel).Observe(sessionElapsed.Seconds())
		}

		if err != nil {
			// Check if this was a teardown - reconnect immediately
			if errors.Is(err, ErrTeardown) {
				// Teardown means intentional disconnect, reconnect immediately
				// Reset delay and continue without waiting
				delay = p.reconnectMin
				p.setState(PeerStateConnecting)
				continue
			}

			// RFC 4486: Prefix limit teardown. The offending family says whether
			// the peer comes back at all (ze-bgp-conf.yang, prefix reconnect).
			if plan, ok := prefixReconnectDecision(p.settings, err, p.prefixTeardownCount+1); ok {
				switch plan.Mode {
				case PrefixReconnectNever:
					// Stays down. holdDownAfterPrefixTeardown returns only when
					// the peer is stopped, so run ends here.
					p.holdDownAfterPrefixTeardown(plan.Family)
					return
				case PrefixReconnectTimer:
					p.prefixTeardownCount++
					// Cap the count to prevent time.Duration overflow (~62 doublings
					// overflow int64 nanoseconds before the hour cap can fire).
					if p.prefixTeardownCount > maxPrefixTeardownCount {
						p.prefixTeardownCount = maxPrefixTeardownCount
					}
					peerLogger().Warn("prefix limit teardown, waiting before reconnect",
						"peer", p.settings.Address,
						"family", plan.Family,
						"delay", plan.Delay,
						"teardown_count", p.prefixTeardownCount,
					)
					p.setState(PeerStateConnecting)
					select {
					case <-p.ctx.Done():
						return
					case <-p.clock.After(plan.Delay):
					}
					continue
				case PrefixReconnectBackoff, PrefixReconnectUnset:
					// The operator asked for the usual backoff, which is the
					// code below. PrefixReconnectFor never returns Unset.
				}
			}

			// Normal error: Backoff before retry
			p.setState(PeerStateConnecting)
			backoffStart := p.clock.Now()
			peerLogger().Debug("timing: backoff starting",
				"peer", p.settings.Address,
				"delay", delay,
			)

			select {
			case <-p.ctx.Done():
				return
			case <-p.clock.After(delay):
			case <-p.inboundNotify:
				// Inbound connection arrived while session was nil.
				// Restart runOnce immediately without doubling delay.
				if p.reactor != nil && p.reactor.rmetrics != nil {
					p.reactor.rmetrics.peerBackoffSeconds.With(peerLabel).Observe(p.clock.Now().Sub(backoffStart).Seconds())
				}
				delay = p.reconnectMin
				continue
			}

			if p.reactor != nil && p.reactor.rmetrics != nil {
				p.reactor.rmetrics.peerBackoffSeconds.With(peerLabel).Observe(p.clock.Now().Sub(backoffStart).Seconds())
			}

			// Exponential backoff
			delay *= 2
			p.mu.RLock()
			maxDelay := p.reconnectMax
			p.mu.RUnlock()
			if delay > maxDelay {
				delay = maxDelay
			}
		} else {
			// Reset delay on successful session
			delay = p.reconnectMin
			// Reset prefix teardown backoff after stable session.
			p.prefixTeardownCount = 0
		}
	}
}

// safeRunOnce wraps runOnce with panic recovery. If runOnce panics, the panic
// is logged with a stack trace and converted to an error so the reconnect loop
// in run() handles it with normal backoff. This is the primary failure domain
// boundary: any bug in session lifecycle triggers reconnection, not daemon crash.
func (p *Peer) safeRunOnce() (err error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			peerLogger().Error("session panic recovered",
				"peer", p.settings.Address,
				"panic", r,
				"stack", string(buf[:n]),
			)
			err = fmt.Errorf("panic in session: %v", r)
		}
	}()
	return p.runOnce()
}

// runOnce attempts a single connection cycle.
func (p *Peer) runOnce() error {
	runOnceStart := p.clock.Now()

	// Reset notification-exchanged flag for the new session lifecycle.
	// IncrNotificationSent / IncrNotificationReceived will set it true if a
	// NOTIFICATION is sent or received before the FSM leaves Established;
	// the FSM transition handler reads it to decide whether to raise the
	// session-dropped error.
	p.notificationExchanged.Store(false)

	// Create session
	session := NewSession(p.settings)
	session.SetClock(p.clock)
	session.SetDialer(p.dialer)
	session.onMessageReceived = p.messageCallback
	// Originated / injected / replayed routes run the peer's export filter chain at
	// the session write gate, the same as forwarded routes do in forwardUpdateCore.
	if p.reactor != nil {
		session.egressRouteFilter = func(body []byte) (bool, []byte) {
			return p.reactor.exportFilterForBody(p, body)
		}
	}
	if p.reactor != nil {
		session.prefixMetrics = p.reactor.rmetrics
	}
	// Protocol event capture (capture_replay.go), opt-in per peer. Opened per
	// session so each session lands in a file whose header names the peer, the
	// start time, and whether the coalesced read path was active.
	capture := p.startCapture(session)
	session.onNotifSent = p.IncrNotificationSent
	session.onNotifRecv = p.IncrNotificationReceived
	session.onOpenSent = p.incrOpensSent
	session.onOpenRecv = p.incrOpensReceived
	session.onRefreshRecv = p.incrRefreshReceived
	session.onRead = p.touchLastRead
	session.onWrite = p.touchLastWrite
	session.onNegotiated = func(holdSec, keepaliveSec uint32) {
		p.negotiatedHoldTime.Store(holdSec)
		p.negotiatedKeepaliveTime.Store(keepaliveSec)
	}
	session.SetSourceID(p.sourceID)
	session.SetPluginCapabilityGetter(p.getPluginCapabilities)
	session.setConfigCapabilityGetter(p.configuredCapabilities)
	session.SetPluginFamiliesGetter(p.getPluginFamilies)
	session.SetOpenValidator(p.validateOpen)

	p.mu.Lock()
	p.session = session
	p.mu.Unlock()

	defer func() {
		p.stopCapture(capture)
		p.negotiated.Store(nil) // Clear negotiated capabilities
		p.negotiatedHoldTime.Store(0)
		p.negotiatedKeepaliveTime.Store(0)
		// RFC 6286 Section 2.1: give up the AS-wide BGP Identifier this session claimed in
		// validateOpen, so another peer may use it once this session is over. Called outside
		// p.mu (the claim registry is a leaf lock, and clearEncodingContexts below takes p.mu).
		p.releaseRouterIDClaim()
		p.clearEncodingContexts()
		// Clear prefix-threshold warnings raised by this session from the report
		// bus so they do not linger after the session ends. Must be called before
		// p.session is set to nil below.
		//
		// The unlocked read of p.session here is safe because:
		//   1. We are inside runOnce's deferred function, running in the SAME
		//      goroutine that wrote p.session at line 201.
		//   2. The session's read goroutine has already exited by the time the
		//      defer runs (Session.Run has returned), so prefixCounts.warned is
		//      no longer being mutated. See ClearReportedWarnings godoc.
		if sess := p.session; sess != nil {
			sess.clearReportedWarnings()
		}
		// Reset sendingInitialRoutes flag so next session can run sendInitialRoutes().
		// This is needed because session.Teardown() may return before the old
		// sendInitialRoutes() goroutine finishes its 500ms sleep.
		p.sendingInitialRoutes.Store(0)
		// A new session owes the peer a fresh End-of-RIB per family.
		p.resetInitialSyncEOR()
		p.mu.Lock()
		p.session = nil
		p.mu.Unlock()
	}()

	// Update state based on FSM mode
	if p.settings.Connection.IsActive() {
		p.setState(PeerStateConnecting)
	} else {
		p.setState(PeerStateActive)
	}

	// Start FSM.
	//
	// RFC 4271 §8.1.1 ConnectRetryCounter: hand this cycle's FSM the PEER's
	// counter before the start event, so the §8.2.2 clauses land on a value
	// that survives the cycle. A counter created here would be destroyed with
	// the Session and could never count a retry.
	session.SetConnectRetryCounter(&p.connectRetryCounter)

	// The operator's start is Event 1 (ManualStart), whose §8.2.2 clause sets
	// the ConnectRetryCounter to zero. Every cycle after it is a retry the
	// backoff above already damped, which is Event 6
	// (AutomaticStart_with_DampPeerOscillations) and carries no such clause.
	// Firing Event 1 on every cycle would zero the counter before each attempt
	// and leave it structurally unable to read more than one.
	start := session.StartDamped
	if !p.operatorStarted.Swap(true) {
		start = session.Start
	}
	if err := start(); err != nil {
		return err
	}

	peerLogger().Debug("timing: session created, dialing",
		"peer", p.settings.Address,
		"port", p.settings.Port,
		"elapsed_since_runOnce", p.clock.Now().Sub(runOnceStart),
	)

	// Dial out if active bit is set (active or both).
	if p.settings.Connection.IsActive() {
		dialLabel := p.peerAddrLabel()
		dialStart := p.clock.Now()
		if err := session.Connect(p.ctx); err != nil {
			dialElapsed := p.clock.Now().Sub(dialStart)

			// Race: an inbound connection was accepted between Start() and Connect(),
			// setting s.conn before we could dial. The inbound won; proceed to Run().
			if errors.Is(err, ErrAlreadyConnected) && session.Conn() != nil {
				peerLogger().Info("inbound connection accepted during dial, using inbound",
					"peer", p.settings.Address,
				)
				if p.reactor != nil && p.reactor.rmetrics != nil {
					p.reactor.rmetrics.peerDialSeconds.With(dialLabel, "ok").Observe(dialElapsed.Seconds())
				}
			} else {
				peerLogger().Debug("timing: dial failed",
					"peer", p.settings.Address,
					"port", p.settings.Port,
					"elapsed_dial", dialElapsed,
					"error", err,
				)
				if p.reactor != nil && p.reactor.rmetrics != nil {
					p.reactor.rmetrics.peerDialSeconds.With(dialLabel, "fail").Observe(dialElapsed.Seconds())
				}
				return err
			}
		} else {
			dialElapsed := p.clock.Now().Sub(dialStart)
			peerLogger().Debug("timing: dial succeeded",
				"peer", p.settings.Address,
				"port", p.settings.Port,
				"elapsed_dial", dialElapsed,
			)
			if p.reactor != nil && p.reactor.rmetrics != nil {
				p.reactor.rmetrics.peerDialSeconds.With(dialLabel, "ok").Observe(dialElapsed.Seconds())
			}
		}
	}

	// For peers that accept inbound, check if a connection arrived while session was nil.
	// This handles the race where a remote peer reconnects faster than our backoff.
	// If Accept fails because a connection already exists (outbound dial won), discard
	// the stale inbound and continue. Other Accept errors are fatal.
	if p.settings.Connection.IsPassive() {
		if conn := p.takeInboundConnection(); conn != nil {
			if err := session.Accept(conn); err != nil {
				closeConnQuietly(conn)
				if errors.Is(err, ErrAlreadyConnected) {
					peerLogger().Debug("discarding stale inbound, outbound dial won", "peer", p.settings.Address)
				} else {
					peerLogger().Debug("stale inbound connection", "peer", p.settings.Address, "error", err)
					return fmt.Errorf("accepting buffered connection: %w", err)
				}
			}
		}
	}

	// Monitor FSM state
	session.fsm.SetCallback(func(from, to fsm.State) {
		addr := p.settings.Address.String()
		peerLogger().Debug("FSM transition", "peer", addr, "from", from.String(), "to", to.String())

		transitionReason := from.String() + " -> " + to.String()

		if to == fsm.StateEstablished {
			// Pre-compute negotiated capabilities for O(1) access during route sending
			neg := session.Negotiated()
			p.negotiated.Store(NewNegotiatedCapabilities(neg))
			p.setEncodingContexts(neg)
			p.setState(PeerStateEstablished)
			p.SetEstablishedNow()

			// Start EOR timeout if GR was negotiated (AC-11).
			if p.health != nil && neg != nil && neg.GracefulRestart != nil {
				p.health.startEORTimer(neg.GracefulRestart.RestartTime, len(neg.Families()))
			}

			// Dynamic peers: set PeerAS from OPEN and resolve config variables.
			// Re-resolve on every establishment (ASN may change on reconnection).
			if p.settings.IsDynamic {
				p.resolveDynamicPeerSettings(session)
			}

			peerLogger().Info("session established", "peer", addr, "localAS", p.settings.LocalAS, "peerAS", p.settings.PeerAS)

			// Reset per-session API sync: count plugins with SendUpdate permission.
			// They will signal "plugin session ready" after replaying routes.
			//
			// This count is ALSO what gates the initial-sync hold that keeps ze's
			// End-of-RIB behind plugin-injected routes, and for that it is the
			// wrong key: nothing on the injection path reads SendUpdate, so a
			// plugin bound as a bare `process X { }` never appears here and its
			// routes can land after the marker. See the KNOWN DEFECT note in
			// sendInitialRoutes (peer_initial_sync.go) before changing either.
			apiSendCount := 0
			for _, binding := range p.settings.ProcessBindings {
				if binding.SendUpdate {
					apiSendCount++
				}
			}
			p.ResetAPISync(apiSendCount)

			// Reset the peer-up barrier for this session BEFORE plugins are
			// notified below: the dispatcher raises its expected count and the
			// plugins acknowledge inside notifyPeerEstablished, so the barrier
			// state those calls land on must already belong to this session.
			p.ResetPeerUpBarrier()

			// The initial-sync gate is already closed: p.setState above closes it
			// in the same call that publishes PeerStateEstablished, so ShouldQueue()
			// is true here and stays true through the notifications below. It used
			// to be closed at THIS line instead, which left every line between the
			// publication and here as a window in which an established peer's route
			// ops bypassed opQueue and the bgp-peer-sync quiescer reported the peer
			// settled -- see setState (peer.go).

			// Notify reactor of peer established and negotiated capabilities
			p.mu.RLock()
			reactor := p.reactor
			p.mu.RUnlock()
			if reactor != nil {
				reactor.notifyPeerEstablished(p)
				reactor.notifyPeerNegotiated(p, neg)
			}

			// Open a BFD session for this peer if the operator
			// opted in via `bgp peer connection bfd { ... }` and
			// the BFD plugin is loaded. No-op otherwise. Runs
			// before sendInitialRoutes so the BFD subscriber is
			// live before the first UPDATE leaves -- a BFD Down
			// during initial flood should still tear the peer.
			p.startBFDClient()

			// Send static routes from config (one-time per-session lifecycle goroutine).
			peerLogger().Debug("spawning sendInitialRoutes", "peer", addr)
			go p.sendInitialRoutes() //nolint:goroutine-lifecycle // per-session lifecycle, not per-event
			transitionReason = "established"
		} else if from == fsm.StateEstablished {
			// Release any BFD session opened on Established. Runs
			// before clearEncodingContexts so the subscriber
			// goroutine has observed the final StateChange
			// (closed channel) before the handle is released.
			p.stopBFDClient()

			// Determine reason based on target state
			reason := "session closed"
			if to == fsm.StateIdle {
				reason = "connection lost"
			}

			// Raise session-dropped on the report bus only when no NOTIFICATION
			// was exchanged during this session. If one was sent or received,
			// the operator already sees that event in `ze show errors` and a
			// duplicate session-dropped would be noise.
			if !p.notificationExchanged.Load() {
				raiseSessionDropped(p.peerAddrLabel(), reason)
			}

			// Notify reactor of peer closed
			p.mu.RLock()
			reactor := p.reactor
			p.mu.RUnlock()
			if reactor != nil {
				reactor.notifyPeerClosed(p, reason)
			}

			// Clear negotiated capabilities and encoding contexts on session teardown
			p.negotiated.Store(nil)
			p.clearEncodingContexts()
			p.setState(PeerStateConnecting)
			peerLogger().Info("session closed", "peer", addr, "reason", reason)
			transitionReason = reason
		}

		p.history.append(FSMTransition{
			Timestamp: p.clock.Now(),
			From:      from.String(),
			To:        to.String(),
			Reason:    transitionReason,
		})
	})

	// Set up per-peer async delivery channel for received UPDATEs.
	// The delivery goroutine drains batches and calls receiver.OnMessageBatchReceived,
	// then Activate per message. This amortizes subscription lookup and format-mode
	// computation across all messages in a batch.
	p.deliverChan = make(chan deliveryItem, deliveryChannelCapacity)
	deliveryDone := make(chan struct{})

	// Long-lived delivery worker (channel + worker pattern, not per-event).
	go func() { //nolint:goroutine-lifecycle // channel worker pattern: reads from p.deliverChan
		defer close(deliveryDone)
		// Recovery exits the loop — remaining buffered items are dropped.
		// This is intentional: a panic indicates a bug, and the session will
		// be torn down (runOnce waits on <-deliveryDone). The recovery ensures
		// deliveryDone closes so shutdown isn't blocked, not continued processing.
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				peerLogger().Error("delivery goroutine panic recovered",
					"peer", p.settings.Address,
					"panic", r,
					"stack", string(buf[:n]),
				)
			}
		}()
		var batchBuf []deliveryItem
		for first := range p.deliverChan {
			batchBuf = drainDeliveryBatch(batchBuf, &first, p.deliverChan)
			batch := batchBuf

			p.mu.RLock()
			reactor := p.reactor
			p.mu.RUnlock()
			if reactor == nil {
				continue
			}
			reactor.mu.RLock()
			receiver := reactor.messageReceiver
			reactor.mu.RUnlock()
			if receiver == nil {
				continue
			}

			// Extract typed messages from batch (no []any boxing needed).
			msgs := make([]bgptypes.RawMessage, len(batch))
			for i := range batch {
				msgs[i] = batch[i].msg
			}

			counts := receiver.OnMessageBatchReceived(&batch[0].peerInfo, msgs)
			for i := range batch {
				count := 0
				if i < len(counts) {
					count = counts[i]
				}
				reactor.recentUpdates.Activate(batch[i].msg.MessageID, count)
			}
		}
	}()

	// Run session loop
	err := session.Run(p.ctx)

	// Drain delivery channel: close stops accepting new items, range loop in
	// goroutine processes remaining buffered items before exiting.
	close(p.deliverChan)
	<-deliveryDone
	p.deliverChan = nil

	return err
}

// cleanup runs when peer stops.
func (p *Peer) cleanup() {
	p.negotiated.Store(nil) // Clear negotiated capabilities
	// A held-down peer that stops is no longer held: the warning describes a
	// peer waiting for an operator, and this one is gone. Unconditional, like
	// the other clears here -- ClearWarning on an unraised subject is a no-op.
	clearPrefixHold(p.peerAddrLabel())
	// RFC 6286 Section 2.1: a stopped peer holds no BGP Identifier. Safe under any caller's
	// locks -- the claim registry never takes r.mu or p.mu.
	p.releaseRouterIDClaim()
	p.clearEncodingContexts()
	p.ClearStats()
	p.mu.Lock()
	if p.session != nil {
		if err := p.session.Close(); err != nil {
			peerLogger().Debug("session close error", "error", err)
		}
		p.session = nil
	}
	inbound := p.inboundConn
	p.inboundConn = nil
	p.cancel = nil
	p.mu.Unlock()

	if inbound != nil {
		closeConnQuietly(inbound)
	}

	p.setState(PeerStateStopped)
}

// maxPrefixTeardownCount caps the teardown counter that drives the prefix
// reconnect backoff. Around 62 doublings overflow int64 nanoseconds, which is
// reached before the one-hour cap can fire, so the counter stops here.
const maxPrefixTeardownCount = 60

// prefixReconnectPlan is what Peer.run does with a session that a prefix limit
// stopped: which family stopped it, whether the peer comes back, and when.
type prefixReconnectPlan struct {
	// Family is the "afi/safi" string of the family that exceeded its maximum.
	Family string
	// Mode is the resolved per-family answer. Never PrefixReconnectUnset.
	Mode PrefixReconnectMode
	// Delay is the wait before the next attempt. Meaningful only for
	// PrefixReconnectTimer.
	Delay time.Duration
}

// prefixReconnectDecision decides what happens after a prefix-limit teardown.
// ok is false when err is not one, and the caller then keeps its normal backoff.
//
// RFC 4486 Section 4: the session was stopped because one family exceeded its
// configured upper bound on prefixes. The `idle-timeout` and `reconnect` leaves
// sit inside the per-family `prefix` container (ze-bgp-conf.yang), so both come
// from the family that overflowed. Another family's values never size the wait.
//
// The timer delay is idle-timeout x 2^(count-1), capped at one hour.
//
// This function DECIDES and never waits: it holds no context and no clock, so
// it cannot park a peer. Peer.run executes the decision, which is the only place
// that can block on the peer context.
//
// A prefix-limit teardown that names NO family still decides here, and it
// resolves through PrefixReconnectFor with an empty key, which reads as never.
// prefixTeardownCause (session_prefix.go) is the only producer today and it
// always names the family, so the branch is unreachable. It exists because the
// alternative is the fail-OPEN direction: falling through on ok=false hands the
// peer its normal connect backoff, and a session stopped for flooding the RIB
// that comes straight back re-floods it. A future producer of the bare sentinel
// must not silently buy that (ai/rules/evidence.md).
func prefixReconnectDecision(settings *PeerSettings, err error, count uint32) (prefixReconnectPlan, bool) {
	var prefixErr *prefixLimitError
	family := ""
	switch {
	case errors.As(err, &prefixErr):
		family = prefixErr.Family
	case errors.Is(err, ErrPrefixLimitExceeded):
		// A prefix teardown carrying no family. Decided, not waved through.
	default:
		return prefixReconnectPlan{}, false
	}

	plan := prefixReconnectPlan{
		Family: family,
		Mode:   settings.PrefixReconnectFor(family),
	}
	if plan.Mode != PrefixReconnectTimer {
		return plan, true
	}

	if count > maxPrefixTeardownCount {
		count = maxPrefixTeardownCount
	}
	plan.Delay = time.Duration(settings.prefixIdleTimeoutFor(family)) * time.Second
	for i := uint32(1); i < count; i++ {
		plan.Delay *= 2
		if plan.Delay > time.Hour {
			plan.Delay = time.Hour
			return plan, true
		}
	}
	return plan, true
}

// holdDownAfterPrefixTeardown parks the peer after a prefix limit stopped the
// session and the offending family asked for no reconnect. It returns only when
// the peer is stopped, so Peer.run ends with it.
//
// The wait is a blocking select, never a loop with a timer: a held peer costs
// nothing and reconnects on no schedule. Inbound connections are refused while
// the hold lasts, because a peer that gets back in through its own retry is not
// held down at all.
//
// The operator sees three things: the peer state reads `idle-hold`, the log line
// below names the family and the reason, and `ze show warnings` carries the
// prefix-hold warning until the peer is recreated.
func (p *Peer) holdDownAfterPrefixTeardown(fam string) {
	p.setState(PeerStateIdleHold)
	raisePrefixHold(p.peerAddrLabel(), fam)
	peerLogger().Error("prefix limit teardown, peer held down",
		"peer", p.settings.Address,
		"family", fam,
		"reconnect", PrefixReconnectNever,
		"action", "raise the family prefix maximum, or set prefix reconnect backoff, then commit the peer config",
	)

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.inboundNotify:
			if conn := p.takeInboundConnection(); conn != nil {
				closeConnQuietly(conn)
				peerLogger().Debug("inbound connection refused, peer held down",
					"peer", p.settings.Address,
					"family", fam,
				)
			}
		}
	}
}
