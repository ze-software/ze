// Design: docs/architecture/core-design.md — peer run loop and session lifecycle
// Overview: peer.go — Peer struct, accessors, lifecycle API
// Related: peer_up_delivery.go — startEstablishedSession, what the Established callback owes a published session

package reactor

import (
	"errors"
	"fmt"
	"slices"
	"time"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/taskguard"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// run is the main peer loop.
//
// This loop replaces the RFC 4271 ConnectRetryTimer (Event 9) with exponential
// backoff. The RFC's timer assumes non-blocking TCP connections where the FSM
// sits in Connect/Active waiting for both TCP completion and the retry timer.
// Ze uses blocking DialContext, so the session either connects or fails before
// returning. The retry delay between attempts is managed here at the Peer level
// with exponential backoff (min 5s → max 60s) for ordinary failures. A TCP
// failure in Active, or while DelayOpen holds Connect, transfers the restarted
// FSM timer's sampled deadline through connectionRetryFailure instead: retiring
// the old Session must not replace that initial-value retry with backoff.
//
// The delay doubles after every wait that ran to its end, up to reconnectMax.
// It returns to reconnectMin after a session that stayed in Established at
// least reconnectMax (cycleStayedEstablished), and on a local teardown
// (ErrTeardown), the administrative hold or wake, and an inbound connection
// during the wait. prefixTeardownCount, which sizes the `reconnect timer` wait
// of a prefix-limit teardown, restarts on the same stable session only.
func (p *Peer) run() {
	defer p.wg.Done()
	defer p.cleanup()

	delay := p.reconnectMin
	attempt := 0

	for {
		// Two things stop this loop dialing again, and p.ctx is only one of them.
		// Reactor.stop marks every peer stopping BEFORE it sends the shutdown
		// NOTIFICATIONs, because the cancel that follows closes the sockets those
		// NOTIFICATIONs need (reactor.go). For that whole window p.ctx is still
		// live, so without this flag the ErrTeardown branch below -- which resets
		// the delay and continues with no wait -- would dial the peer again,
		// reach Established again, and then be killed by the cancel with nothing
		// on the wire to say why. That is a shutdown flap, and a second RFC 4271
		// Section 8.2.2 ManualStop miss on the same stop. The inbound half of
		// that miss is closed at the session: Reactor.stop seals every published
		// session, and connectionEstablished refuses a conn on a sealed one
		// (peer.go, session_connection.go). Nothing on the accept path reads
		// this flag.
		if p.stopping.Load() {
			return
		}

		select {
		case <-p.ctx.Done():
			return
		default: // no cancellation pending
		}

		// A peer held administratively down opens nothing until Enable
		// (peer_admin.go). The hold re-reads the flag itself, so this read is
		// the cheap early exit and runOnce's publish-time read is the correct one.
		if p.adminDown.Load() {
			p.holdAdminDown()
			delay = p.reconnectMin
			continue
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
			// RFC 4271 Section 8.2.2: "The method of preventing persistent peer oscillation is outside the scope of this document."
			// The model is BIRD's bgp_update_startup_delay (proto/bgp/bgp.c):
			// a penalty is forgotten once the peer has been quiet long enough.
			// Ze measures that quiet as time in Established, so a peer that
			// never comes up stays at the cap. The check runs before the
			// ErrTeardown branch, so a teardown of a stable session also
			// clears the prefix teardown count.
			if p.cycleStayedEstablished() {
				delay = p.reconnectMin
				p.prefixTeardownCount = 0
			}

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
				default:
					// Configuration-backed modes form an open set; keep normal backoff for unknown values.
				}
			}

			// RFC 4271 Section 8.2.2, Connect and Active Event 18: the
			// restarted initial-value retry survives the old session's cleanup.
			var retry *connectionRetryFailure
			var wait time.Duration
			if errors.As(err, &retry) {
				wait = retry.budget.Remaining()
			} else {
				wait = fsm.Jitter(delay)
			}

			p.setState(PeerStateConnecting)
			backoffStart := p.clock.Now()
			peerLogger().Debug("timing: backoff starting",
				"peer", p.settings.Address,
				"delay", wait,
			)

			select {
			case <-p.ctx.Done():
				return
			case <-p.clock.After(wait):
			case <-p.adminWake:
				// Disable during the backoff parks now; Enable retries now.
				delay = p.reconnectMin
				continue
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
			if retry != nil {
				// This interval is the FSM's retry, not a failed backoff wait.
				continue
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
			// runOnce returns nil when the peer is stopping or held
			// administratively down at publication, and the loop top ends or
			// parks the loop. Whatever cycle follows starts with no penalty.
			delay = p.reconnectMin
			p.prefixTeardownCount = 0
		}
	}
}

// cycleStayedEstablished reports whether the connection cycle that just ended
// reached Established and stayed there at least reconnectMax, measured on the
// peer clock. run MUST call it after the cycle returned and before the next
// runOnce, which swaps cycleReachedEstablished back to false.
//
// cycleReachedEstablished is read first, without a swap, because nothing
// clears EstablishedAt between sessions: after a cycle that never reached
// Established it holds an older session's entry time, or zero for a peer that
// never came up, and either would read as a long stable session.
func (p *Peer) cycleStayedEstablished() bool {
	if !p.cycleReachedEstablished.Load() {
		return false
	}
	p.mu.RLock()
	stableTime := p.reconnectMax
	p.mu.RUnlock()
	return p.clock.Now().Sub(p.EstablishedAt()) >= stableTime
}

// safeRunOnce wraps runOnce with panic recovery. If runOnce panics, the panic
// is logged with a stack trace and converted to an error so the reconnect loop
// in run() handles it with normal backoff. This is the primary failure domain
// boundary: any bug in session lifecycle triggers reconnection, not daemon crash.
func (p *Peer) safeRunOnce() (err error) {
	// The outer boundary: Session.Run contains its own read loop, so this one
	// catches what runs outside it (dial, OPEN setup, the session's teardown).
	// The cycle ends with the contained panic as its error, so the run loop's
	// backoff applies.
	defer bgpPanics().connect.Contain(p.describeConnect, func(contained *taskguard.Panic) { err = contained })
	return p.runOnce()
}

// abortDeliveryOwnership retires only the IDs this worker never finished
// publishing. The captured reactor owns their cache even if Peer.reactor was
// detached while the session unwound. Existing forward retains survive abort.
func abortDeliveryOwnership(owner *Reactor, batch []deliveryItem) {
	if owner == nil {
		return
	}
	for i := range batch {
		owner.recentUpdates.abortPublication(batch[i].msg.MessageID)
	}
}

// describeConnect names the unit a contained connection-cycle or delivery
// panic ended: the peer.
func (p *Peer) describeConnect() taskguard.Context {
	return taskguard.Context{Unit: p.settings.Address.String()}
}

// runOnce attempts a single connection cycle.
func (p *Peer) runOnce() (runErr error) {
	runOnceStart := p.clock.Now()

	// Reset notification-exchanged flag for the new session lifecycle.
	// incrNotificationSent / incrNotificationReceived will set it true if a
	// NOTIFICATION is sent or received before the FSM leaves Established;
	// the FSM transition handler reads it to decide whether to raise the
	// session-dropped error.
	p.notificationExchanged.Store(false)

	// A winning incoming connection retains its original Session and timers.
	// Its reserved slot remains present until publication and callback setup.
	candidate := p.selectedIncomingCandidate()
	var session *Session
	if candidate != nil {
		session = candidate.acquire()
		if session == nil {
			return ErrNotConnected
		}
	} else {
		session = p.newSession()
	}
	// The factory installed every writer/timer input before TCP. Candidate
	// adoption never replaces a callback a periodic writer may still read.
	capture := session.captureWriter
	// This fallback predates publication, including a direct Accept racing the
	// delivery worker's setup. Normal cleanup closes after the ordered DOWN.
	defer session.closeSourceReceipt()
	published := false
	defer func() {
		if !published {
			p.retireUnpublishedSession(candidate, session)
		}
	}()
	// Avoid opening a strict BFD client for an admission already refused.
	// The publication hold below rechecks both flags after BFD initialization.
	if p.stopping.Load() || p.adminDown.Load() {
		return nil
	}

	// draft-ietf-idr-bgp-bfd-strict-mode Section 7: "Implementations SHOULD
	// start the BFD session associated with the BGP BFD strict-mode session
	// prior to the BGP FSM starting." Keep this outside Peer.mu: starting
	// the BFD subscriber can deliver a state change through this same Peer.
	// Its initial reading is retained on the Peer before the FSM starts, so
	// the first OPEN observes an already-Up session without another event.
	if p.bfdStrict() {
		p.startBFDClient()
	}

	// Construct publication before exposing the Session to direct Accept.
	// The fresh FSM starts without this callback under Peer.mu, then installs
	// it before publication. Every externally admitted TCP transition therefore
	// has its observer, without invoking an owner callback under Peer.mu.
	// The leave-Established arm fills this record and closes its signal; the
	// down defer below waits on it (sessionDown).
	leave := &sessionLeave{signal: make(chan struct{}), selected: candidate == nil}

	// Monitor FSM state
	publishTransition := func(from, to fsm.State, event fsm.Event) {
		addr := p.settings.Address.String()
		peerLogger().Debug("FSM transition", "peer", addr, "from", from.String(), "to", to.String())
		// The engine's own facts first, so a process reads the transition
		// before any event the new state produces (peer_fsm_events.go).
		p.emitFSMEvents(from, to)

		transitionReason := from.String() + " -> " + to.String()

		// The OPENs are exchanged on entry to OpenConfirm, and the KEEPALIVE
		// that answers the peer's OPEN is sent after this callback returns
		// (Session.advanceAfterOpen), so the negotiated event precedes every
		// message of the session that it describes (Peer.announceNegotiated).
		if to == fsm.StateOpenConfirm {
			p.mu.RLock()
			reactor := p.reactor
			p.mu.RUnlock()
			if reactor != nil {
				p.announceNegotiated(reactor, session, session.Negotiated())
			}
		}

		if to == fsm.StateEstablished {
			// Pre-compute negotiated capabilities for O(1) access during route sending
			neg := session.Negotiated()
			p.negotiated.Store(NewNegotiatedCapabilities(neg))
			p.setEncodingContexts(neg)
			// Before Established is published, so the first owed key has a worker.
			session.startRecovery(p)
			p.cycleReachedEstablished.Store(true)
			// RFC 8212 Section 3: establishment joins the import swap's
			// admission boundary before the first UPDATE is read.
			p.enterEstablished(session)
			p.setEstablishedNow()

			// Start EOR timeout if GR was negotiated (AC-11).
			if p.health != nil && neg != nil && neg.GracefulRestart != nil {
				p.health.startEORTimer(neg.GracefulRestart.RestartTime, len(neg.Families()))
			}

			// Dynamic peers: set PeerAS from OPEN and resolve config variables.
			// Re-resolve on every establishment (ASN may change on reconnection).
			if p.settings.IsDynamic {
				p.resolveDynamicPeerSettings(session)
			}
			if p.reactor != nil {
				if err := session.captureSourceReceipt(p, &sourceReceipts); err != nil {
					reportSourceReceiptAdmissionFailure(p, err)
					session.setCloseReason(err)
					session.closeConn()
					return
				}
			}

			peerLogger().Info("session established", "peer", addr, "localAS", p.settings.LocalAS, "peerAS", p.settings.PeerAS)
			// RFC 8212 Section 3: say which direction the absent policy closes,
			// once per session and after the dynamic AS is known.
			p.warnRFC8212AbsentPolicy()

			// The per-session API sync and the replay fence were reset and
			// raised by enterEstablished above, before Established was
			// published (Peer.resetAPISync, Peer.raiseReplayFence).

			// The initial-sync gate is already closed: enterEstablished above
			// closes the session's gate before it publishes PeerStateEstablished,
			// so shouldQueue() is true here and stays true through the
			// notification below. It used to be closed at THIS line instead, which
			// left every line between the publication and here as a window in which
			// an established peer's route ops bypassed opQueue and the
			// bgp-peer-sync quiescer reported the peer settled.

			// The peer-up barrier reset, the BFD client, the initial routing
			// update and the plugin notification, in that order: BFD depends on
			// no plugin, and the initial update can run while a plugin's peer-up
			// handler waits on its gate (peer_up_delivery.go).
			p.startEstablishedSession(session)
			transitionReason = "established"
		} else if from == fsm.StateEstablished {
			p.leaveSession(session, leave, from, to, event)
			// The down owes the history row, after the session's buffered
			// UPDATEs were published (sessionDown).
			return
		}

		p.history.append(FSMTransition{
			Timestamp: p.clock.Now(),
			From:      from.String(),
			To:        to.String(),
			Reason:    transitionReason,
		})
	}

	// This assignment is one of the two places a conn becomes a session, the
	// other being s.conn in Session.connectionEstablished (session_connection.go).
	// The seal is re-read HERE, in the same hold of p.mu that publishes the
	// session, and the pairing is what makes the answer ordered instead of
	// raced. Reactor.stop marks every peer and then reads p.session under this
	// same lock, through sealSession and shutdownNotify (peer.go), so:
	//
	//   - if that read got in first, the mark happened-before this lock and this
	//     load returns true, so no session is ever published and nothing below
	//     dials or accepts;
	//   - otherwise this publish got in first, the stop sees the session and
	//     seals it, and connectionEstablished reads that seal before it
	//     publishes a conn -- for the dial below and for the accept rail alike.
	//
	// One of the two always holds, so after the seal no conn can become a
	// session out of this loop. The loop-top read in run() is the cheap early
	// exit; this one is the correct one, because the loop top can be passed
	// before the mark and the dial below then takes as long as a TCP handshake.
	publishErr := func() error {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.stopping.Load() {
			// nil, not an error: run()'s loop top ends the loop. An error
			// would enter backoff before observing the stop.
			return nil
		}
		// Disable shares this hold: either it refuses publication, or it
		// sees and closes the admitted Session (peer_admin.go).
		if p.adminDown.Load() {
			return nil
		}

		// The first operator cycle takes ManualStart; a cycle following
		// Established takes AutomaticStart; other retries remain damped.
		// Consume that prior-cycle answer only after admission succeeds.
		reachedEstablished := p.cycleReachedEstablished.Swap(false)
		start := session.startDamped
		if reachedEstablished {
			// RFC 4271 Section 8.2.2
			start = session.startAutomatic
		}
		if !p.operatorStarted.Swap(true) {
			start = session.Start
		}
		if candidate == nil {
			// This fresh FSM has no publication callback yet. Start it
			// before exposing it to direct Accept: TCP-confirmed in Idle
			// is ignored, even though Accept would still send local OPEN.
			if err := start(); err != nil {
				return err
			}
			// SetCallback only assigns the observer; it invokes no owner code.
			// The next accept cannot see this Session until both are ready.
			session.fsm.SetCallback(publishTransition)
		}
		p.session = session
		published = true
		// Both candidate receipts already own a validated remote
		// identifier; advancement does not validate it a second time.
		switch arrival := p.inbound.(type) {
		case *candidateOpen:
			p.remoteRouterID.Store(arrival.open.BGPIdentifier)
		case *candidateReady:
			p.remoteRouterID.Store(arrival.remoteID)
		}
		return nil
	}()
	if !published {
		return publishErr
	}

	defer func() {
		// A buffered OPEN can fail before Run installs its finalizers.
		// This Peer owns the session from publication through every exit.
		session.closeConn()
		session.stopSessionTimers()
		// RFC 4271 Section 8.2.2: stopSessionTimers MUST join the Event 18
		// owner before reading its retry; every timer is retired by this point.
		if runErr != nil {
			if session.retryBudget != nil {
				runErr = &connectionRetryFailure{cause: runErr, budget: session.retryBudget}
			}
		}
		p.stopCapture(capture)
		p.negotiated.Store(nil) // Clear negotiated capabilities
		p.negotiatedHoldTime.Store(0)
		p.negotiatedKeepaliveTime.Store(0)
		// RFC 6286 Section 2.1: give up the AS-wide BGP Identifier this session claimed in
		// validateOpen, so another peer may use it once this session is over. Called outside
		// p.mu (the claim registry is a leaf lock, and clearEncodingContexts below takes p.mu).
		p.releaseSessionRouterIDClaim(session)
		// The down defer has run (it was registered later), so the leave
		// callback finished and the down was delivered before this join. It
		// holds neither writeMu nor the forward worker.
		session.joinRecovery()
		// RFC 4724 Section 4: "The End-of-RIB marker MUST be sent by a BGP
		// speaker to its peer once it completes the initial routing update
		// (including the case when there is no update to send) for an address
		// family after the BGP session is established."
		//
		// The session's initial sync is joined here, like every other
		// per-session goroutine, so it cannot outlive its session: no marker,
		// queued operation or plugin route of this session can reach the
		// session that replaces it, and the next runOnce cannot start before
		// it has returned. The latch is moved to ended first, so a convergence
		// hold that releases this session later starts nothing. The join runs
		// after sessionDown (registered later, so it ran first) and with no
		// lock held: the sync takes p.mu, p.staticMu and writeMu on its way out.
		session.endInitialSync()
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
		// The initial-sync facts are this session's (Session.sendingInitialRoutes,
		// Session.initialSyncEOROwed), so nothing resets them for the next
		// session, which starts with its own. Once p.session is cleared below,
		// every reader sees no session, an open gate and no marker owed. The
		// forward worker's lock-free copy is cleared here, after the join.
		p.establishedSession.Store(nil)
		// The session is gone, so nothing will drain an opQueue for it. Release
		// the forwarded UPDATEs parked behind the sync: fwdBatchHandler discards
		// them for a peer that is no longer Established, which is what must
		// happen -- holding them would deliver this session's UPDATEs to its
		// replacement, after that session's own initial sync.
		p.wakeForwardOverflow()
		// A new session owes the peer a fresh End-of-RIB per family. After the
		// join, so no claim of this session's sync lands after the reset.
		p.resetInitialSyncEOR()
		p.mu.Lock()
		p.session = nil
		// The phase belongs to the session that ended. A reload compares the
		// session pointer as well, so this write needs no reactor lock.
		p.upSession = nil
		p.upPhase = sessionUpUnspecified
		p.mu.Unlock()
	}()
	// The preceding epoch joined its capture before this runOnce began. The
	// selected Session's immutable input can now open and drain its own file.
	switch target := capture.(type) {
	case nil, *sessionCapture:
	case *pendingSessionCapture:
		target.selectForPublication()
	}
	// Register before any accept rail can process an OPEN. A retained
	// collision winner can publish negotiation and then fail its KEEPALIVE
	// before the delivery worker (and its Established down) even exists.
	defer p.endNegotiatedSession(session)

	// Update state based on FSM mode
	if p.settings.Connection.IsActive() {
		p.setState(PeerStateConnecting)
	} else {
		p.setState(PeerStateActive)
	}

	// Consume the Peer-owned inbound slot before dialing. A collision winner
	// has already supplied its OPEN and MUST keep this exact socket and wire.
	var readyIncoming *candidateReady
	var openIncoming *candidateOpen
	if p.settings.Connection.IsPassive() {
		inbound := p.takeInboundConnection()
		if inbound != nil {
			var err error
			switch arrival := inbound.(type) {
			case *incomingSocket:
				err = session.Accept(arrival.conn)
			case *bufferedIncoming:
				err = session.acceptWithOpen(arrival.conn, arrival.open, arrival.wire)
			case *candidateOpen:
				openIncoming = arrival
			case *candidateReady:
				readyIncoming = arrival
			}
			if inboundCandidate(inbound) == nil {
				p.mu.Lock()
				p.collisionHandoff = false
				p.mu.Unlock()
			}
			if err != nil {
				closeConnQuietly(inboundConn(inbound))
				if !errors.Is(err, ErrAlreadyConnected) {
					return fmt.Errorf("accepting buffered connection: %w", err)
				}
			}
		}
	}

	peerLogger().Debug("timing: session created, dialing",
		"peer", p.settings.Address,
		"port", p.settings.Port,
		"elapsed_since_runOnce", p.clock.Now().Sub(runOnceStart),
	)

	// A retained collision winner takes precedence in ConnectionBoth too.
	if candidate == nil && p.settings.Connection.IsActive() && session.Conn() == nil {
		dialLabel := p.peerAddrLabel()
		dialStart := p.clock.Now()
		if err := session.Connect(p.ctx); err != nil {
			dialElapsed := p.clock.Now().Sub(dialStart)

			// Race: an inbound connection was accepted between Start() and Connect(),
			// setting s.conn before we could dial. The inbound won; proceed to Run().
			inboundWon := errors.Is(err, ErrAlreadyConnected) && session.Conn() != nil
			if !inboundWon {
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

			peerLogger().Info("inbound connection accepted during dial, using inbound",
				"peer", p.settings.Address,
			)
			if p.reactor != nil && p.reactor.rmetrics != nil {
				p.reactor.rmetrics.peerDialSeconds.With(dialLabel, "ok").Observe(dialElapsed.Seconds())
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

	// Set up per-peer async delivery channel for received UPDATEs.
	// The delivery goroutine drains batches and calls receiver.OnMessageBatchReceived,
	// then Activate per message. This amortizes subscription lookup and format-mode
	// computation across all messages in a batch.
	deliverChan := make(chan deliveryItem, deliveryChannelCapacity)
	p.mu.Lock()
	p.deliverChan = deliverChan
	deliveryOwner := p.reactor
	p.mu.Unlock()
	deliveryDone := make(chan struct{})
	publication := &receivePublication{done: deliveryDone}
	p.receivePublication.Store(publication)

	// Long-lived delivery worker (channel + worker pattern, not per-event).
	go func() { // goroutine-lifecycle: channel worker pattern: reads from its captured deliverChan
		defer close(deliveryDone)
		var batchBuf []deliveryItem
		var activeBatch []deliveryItem
		deliveryFailed := false
		// Failed publication closes lookup admission, but an acquired forward
		// keeps its retain. Seal the source first: its down then closes this
		// channel before waiting for deliveryDone, so draining cannot deadlock
		// against a producer that is still admitting UPDATEs.
		defer bgpPanics().delivery.Contain(p.describeConnect, func(*taskguard.Panic) {
			// A recovered panic is a Ze defect, for which RFC 4486 has no
			// subcode: Unspecific (ceaseAfterPanic has the RFC 4271 quotes).
			failReceivePublication(session, publication, message.NotifyCeaseUnspecific)
			abortDeliveryOwnership(deliveryOwner, activeBatch)
			for item := range deliverChan {
				abortDeliveryOwnership(deliveryOwner, []deliveryItem{item})
			}
		})
		for first := range deliverChan {
			batchBuf = drainDeliveryBatch(batchBuf, &first, deliverChan)
			batch := batchBuf
			activeBatch = batch
			if deliveryFailed {
				abortDeliveryOwnership(deliveryOwner, batch)
				activeBatch = nil
				continue
			}

			p.mu.RLock()
			reactor := p.reactor
			p.mu.RUnlock()
			if reactor == nil {
				failReceivePublication(session, publication, message.NotifyCeaseOutOfResources)
				deliveryFailed = true
				abortDeliveryOwnership(deliveryOwner, batch)
				activeBatch = nil
				continue
			}
			reactor.mu.RLock()
			receiver := reactor.messageReceiver
			reactor.mu.RUnlock()
			receiver = p.sessionEndHooks.Load().receiverFor(receiver)
			if receiver == nil {
				failReceivePublication(session, publication, message.NotifyCeaseOutOfResources)
				deliveryFailed = true
				abortDeliveryOwnership(deliveryOwner, batch)
				activeBatch = nil
				continue
			}

			// Extract typed messages from batch (no []any boxing needed).
			msgs := make([]bgptypes.RawMessage, len(batch))
			for i := range batch {
				msgs[i] = batch[i].msg
			}

			counts := receiver.OnMessageBatchReceived(&batch[0].peerInfo, msgs)
			publication.complete(batch[len(batch)-1].msg.MessageID)
			for i := range batch {
				count := 0
				if i < len(counts) {
					count = counts[i]
				}
				reactor.recentUpdates.Activate(batch[i].msg.MessageID, count)
			}
			activeBatch = nil
		}
	}()

	// The down is a defer, registered after the delivery worker started, so
	// every exit from here drains and announces it, a panic on the read
	// goroutine included. It runs before the earlier defer that clears
	// p.session.
	defer p.sessionDown(session, publication, deliverChan, deliveryDone, leave)

	if candidate != nil {
		err := func() error {
			// The typed receipt owns this reservation through installation,
			// including an error or panic. Do not clear a newer stored winner.
			defer func() {
				p.mu.Lock()
				if p.inbound == nil {
					p.collisionHandoff = false
				}
				p.mu.Unlock()
			}()
			if openIncoming != nil {
				if err := session.adoptIncoming(publishTransition, leave, false); err != nil {
					return err
				}
				return openIncoming.openAdmission.advance()
			}
			if readyIncoming != nil {
				return session.adoptIncoming(publishTransition, leave, true)
			}
			return ErrNotConnected
		}()
		if err != nil {
			return err
		}
	}

	// Run session loop
	return session.Run(p.ctx)
}

// connectionRetryFailure is a failed cycle whose FSM restarted the
// ConnectRetryTimer. Its immutable budget retains the timer's owning clock;
// runOnce constructs it only after joining that session's failure and retirement.
// A nil pointer is not an error value, and a zero value is never constructed.
// The underlying error remains available to teardown and diagnostic callers.
type connectionRetryFailure struct {
	cause  error
	budget fsm.ConnectRetryBudget
}

func (e *connectionRetryFailure) Error() string { return e.cause.Error() }

func (e *connectionRetryFailure) Unwrap() error { return e.cause }

// cleanup runs when peer stops.
func (p *Peer) cleanup() {
	p.negotiated.Store(nil) // Clear negotiated capabilities
	// A held-down peer that stops is no longer held: the warning describes a
	// peer waiting for an operator, and this one is gone. Unconditional, like
	// the other clears here -- ClearWarning on an unraised subject is a no-op.
	clearPrefixHold(p.peerAddrLabel())
	// RFC 6286 Section 2.1: a stopped peer holds no BGP Identifier. Safe under any caller's
	// locks -- the claim registry never takes r.mu or p.mu.
	p.clearEncodingContexts()
	// The last release of a strict peer's BFD session, which
	// draft-ietf-idr-bgp-bfd-strict-mode Section 7 keeps alive across every
	// BGP connection. Idempotent, so a non-strict peer whose session the FSM
	// callback already released reaches a no-op.
	p.stopBFDClient()
	p.mu.Lock()
	p.incomingClosed = true
	// p.session is NOT read here. runOnce's own defer nils it, and that defer
	// runs first: cleanup is `defer p.cleanup()` in run(), so it can only run
	// after runOnce returned. The `if p.session != nil` that used to stand here
	// was therefore always false, and the Session.Close it guarded -- the one
	// place that built the shutdown NOTIFICATION -- had no reachable caller at
	// all. The send now happens in Reactor.Stop, before the cancel that makes
	// every downstream guard false (reactor.go, shutdownNotify in peer.go).
	inbound := p.inbound
	p.inbound = nil
	pending := p.pending
	p.pending = nil
	p.collisionHandoff = false
	p.cancel = nil
	p.mu.Unlock()

	if pending != nil {
		closeConnQuietly(pending.connection())
	}
	if inbound != nil {
		discardInbound(inbound)
	}
	p.incomingWG.Wait()
	p.releaseRouterIDClaim()
	p.ClearStats()

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
		Mode:   settings.prefixReconnectFor(family),
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
			if inbound := p.takeInboundConnection(); inbound != nil {
				discardInbound(inbound)
				peerLogger().Debug("inbound connection refused, peer held down",
					"peer", p.settings.Address,
					"family", fam,
				)
			}
		}
	}
}

// replayFenceOwners names the reporters that declared the replay fence
// (registry.FencesLiveForwards). Caller MUST hold p.mu.
func (p *Peer) replayFenceOwners() []string {
	var owners []string
	for _, name := range p.apiSyncExpected {
		if registry.FencesLiveForwards(name) {
			owners = append(owners, name)
		}
	}
	return owners
}

// initialUpdateReporters names the processes whose routes belong to this peer's
// INITIAL routing update: the session-ready expectation set resetAPISync arms.
// A report from one of them is credited, and the reports of those that also
// declared the replay fence lower it (Peer.SignalAPIReady). The End-of-RIB
// waits for none of them (owner ruling, 2026-09-18): the marker follows ze's
// own table (sendInitialRoutes, peer_initial_sync.go).
//
// Four facts have to hold together, and each one alone admits a process that
// can never report:
//
//   - The compiled delivery graph permits the process to send `bgp:update` or
//     `bgp:raw` to this peer.
//   - The compiled delivery graph feeds this peer's `bgp:state` events to the
//     process.
//   - The plugin DECLARES that its routes belong to the initial update and that
//     it reports when they are out (registry.SignalsSessionReady).
//   - The peer satisfies the plugin's registered ClaimSessionScope (e.g.
//     rs-client for bgp-rs, ibgp for bgp-rr).
func (p *Peer) initialUpdateReporters() []string {
	g := p.reactor.DeliveryGraph()
	if g == nil {
		return nil
	}
	updateSenders := g.SendersForPeer(p.addrString, bgpevents.SendUpdate)
	rawSenders := g.SendersForPeer(p.addrString, bgpevents.SendRaw)
	if len(updateSenders) == 0 && len(rawSenders) == 0 {
		return nil
	}
	candidates := make([]string, 0, len(updateSenders)+len(rawSenders))
	candidates = append(candidates, updateSenders...)
	for _, name := range rawSenders {
		if !slices.Contains(candidates, name) {
			candidates = append(candidates, name)
		}
	}
	slices.Sort(candidates)

	p.mu.RLock()
	settings := p.settings
	peerAS := p.settings.PeerAS
	p.mu.RUnlock()

	var reporters []string
	for _, procName := range candidates {
		if !g.ProcessReceivesState(p.addrString, procName) {
			continue
		}
		regName := resolveRegistrationName(p.reactor, procName)
		if !registry.SignalsSessionReady(regName) && !registry.SignalsSessionReady(procName) {
			continue
		}
		if !peerSettingsMatchesClaimScope(settings, peerAS, registry.ClaimSessionScopeFor(regName)) {
			continue
		}
		reporters = append(reporters, procName)
	}
	return reporters
}
