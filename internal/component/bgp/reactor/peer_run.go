// Design: docs/architecture/core-design.md — peer run loop and session lifecycle
// Overview: peer.go — Peer struct, accessors, lifecycle API

package reactor

import (
	"errors"
	"fmt"
	"runtime"
	"time"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
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

	for {
		select {
		case <-p.ctx.Done():
			return
		default: // no cancellation pending
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

		if err != nil {
			// Check if this was a teardown - reconnect immediately
			if errors.Is(err, ErrTeardown) {
				// Teardown means intentional disconnect, reconnect immediately
				// Reset delay and continue without waiting
				delay = p.reconnectMin
				p.setState(PeerStateConnecting)
				continue
			}

			// RFC 4486: Prefix limit teardown with idle-timeout.
			// Uses separate backoff from normal reconnect: idle-timeout x 2^(N-1), capped at 1 hour.
			if errors.Is(err, ErrPrefixLimitExceeded) && p.settings.PrefixIdleTimeout > 0 {
				p.prefixTeardownCount++
				idleBase := time.Duration(p.settings.PrefixIdleTimeout) * time.Second
				prefixDelay := idleBase
				for i := uint32(1); i < p.prefixTeardownCount; i++ {
					prefixDelay *= 2
					if prefixDelay > time.Hour {
						prefixDelay = time.Hour
						break
					}
				}
				peerLogger().Warn("prefix limit teardown, waiting before reconnect",
					"peer", p.settings.Address,
					"delay", prefixDelay,
					"teardown_count", p.prefixTeardownCount,
				)
				p.setState(PeerStateConnecting)
				select {
				case <-p.ctx.Done():
					return
				case <-p.clock.After(prefixDelay):
				}
				continue
			}

			// Normal error: Backoff before retry
			p.setState(PeerStateConnecting)

			select {
			case <-p.ctx.Done():
				return
			case <-p.clock.After(delay):
			case <-p.inboundNotify:
				// Inbound connection arrived while session was nil.
				// Restart runOnce immediately without doubling delay.
				delay = p.reconnectMin
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
	// Create session
	session := NewSession(p.settings)
	session.SetClock(p.clock)
	session.SetDialer(p.dialer)
	session.onMessageReceived = p.messageCallback
	if p.reactor != nil {
		session.prefixMetrics = p.reactor.rmetrics
	}
	session.prefixWarningNotifier = p.SetPrefixWarned
	session.onNotifSent = p.IncrNotificationSent
	session.onNotifRecv = p.IncrNotificationReceived
	session.SetSourceID(p.sourceID)
	session.SetPluginCapabilityGetter(p.getPluginCapabilities)
	session.SetPluginFamiliesGetter(p.getPluginFamilies)
	session.SetOpenValidator(p.validateOpen)

	p.mu.Lock()
	p.session = session
	p.mu.Unlock()

	defer func() {
		p.negotiated.Store(nil) // Clear negotiated capabilities
		p.clearEncodingContexts()
		p.clearPrefixWarned()
		// Reset sendingInitialRoutes flag so next session can run sendInitialRoutes().
		// This is needed because session.Teardown() may return before the old
		// sendInitialRoutes() goroutine finishes its 500ms sleep.
		p.sendingInitialRoutes.Store(0)
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

	// Start FSM
	if err := session.Start(); err != nil {
		return err
	}

	// Dial out if active bit is set (active or both).
	if p.settings.Connection.IsActive() {
		if err := session.Connect(p.ctx); err != nil {
			return err
		}
	}

	// For peers that accept inbound, check if a connection arrived while session was nil.
	// This handles the race where a remote peer reconnects faster than our backoff.
	// If Accept fails (stale connection), return error so run() retries with a clean
	// session rather than entering Run() with a partially-initialized FSM state.
	if p.settings.Connection.IsPassive() {
		if conn := p.takeInboundConnection(); conn != nil {
			if err := session.Accept(conn); err != nil {
				peerLogger().Debug("stale inbound connection", "peer", p.settings.Address, "error", err)
				closeConnQuietly(conn)
				return fmt.Errorf("accepting buffered connection: %w", err)
			}
		}
	}

	// Monitor FSM state
	session.fsm.SetCallback(func(from, to fsm.State) {
		addr := p.settings.Address.String()
		peerLogger().Debug("FSM transition", "peer", addr, "from", from.String(), "to", to.String())

		if to == fsm.StateEstablished {
			// Pre-compute negotiated capabilities for O(1) access during route sending
			neg := session.Negotiated()
			p.negotiated.Store(NewNegotiatedCapabilities(neg))
			p.setEncodingContexts(neg)
			p.setState(PeerStateEstablished)
			p.SetEstablishedNow()
			peerLogger().Info("session established", "peer", addr, "localAS", p.settings.LocalAS, "peerAS", p.settings.PeerAS)

			// Reset per-session API sync: count plugins with SendUpdate permission.
			// They will signal "plugin session ready" after replaying routes.
			apiSendCount := 0
			for _, binding := range p.settings.ProcessBindings {
				if binding.SendUpdate {
					apiSendCount++
				}
			}
			p.ResetAPISync(apiSendCount)

			// Set sendingInitialRoutes flag BEFORE notifying plugins.
			// This ensures ShouldQueue() returns true during event delivery,
			// preventing a race where a plugin receives state=up, sends a route
			// command, and the route bypasses the queue (because the flag wasn't
			// set yet). Without this, the route could be sent to the peer before
			// sendInitialRoutes runs, causing duplicates when the RIB plugin
			// replays on state=up.
			p.sendingInitialRoutes.Store(1)

			// Notify reactor of peer established and negotiated capabilities
			p.mu.RLock()
			reactor := p.reactor
			p.mu.RUnlock()
			if reactor != nil {
				reactor.notifyPeerEstablished(p)
				reactor.notifyPeerNegotiated(p, neg)
			}

			// Send static routes from config (one-time per-session lifecycle goroutine).
			peerLogger().Debug("spawning sendInitialRoutes", "peer", addr)
			go p.sendInitialRoutes() //nolint:goroutine-lifecycle // per-session lifecycle, not per-event
		} else if from == fsm.StateEstablished {
			// Determine reason based on target state
			reason := "session closed"
			if to == fsm.StateIdle {
				reason = "connection lost"
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
		}
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

			counts := receiver.OnMessageBatchReceived(batch[0].peerInfo, msgs)
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
