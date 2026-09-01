// Design: docs/architecture/core-design.md — initial route sending on BGP session establishment
// Overview: peer.go — Peer struct and FSM state machine
// RFC: rfc/short/rfc4724.md — End-of-RIB marker: sent once the initial routing update completes

package reactor

import (
	"net/netip"
	"runtime"
	"sort"
	"strings"
	"unsafe"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// sendInitialRoutes sends static routes configured for this peer.
// Routes with identical attributes are grouped into a single UPDATE message.
// Uses atomic flag to prevent concurrent execution if session reconnects quickly.
func (p *Peer) sendInitialRoutes() {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			peerLogger().Error("sendInitialRoutes panic recovered",
				"peer", p.settings.Address,
				"panic", r,
				"stack", string(buf[:n]),
			)
			// Clear both facts so shouldQueue() returns false, the peer is not
			// stuck, and pendingSync stops reporting a marker this goroutine
			// will never send.
			p.sendingInitialRoutes.Store(0)
			p.initialSyncEOROwed.Store(false)
			p.wakeForwardOverflow()
		}
	}()
	addr := p.settings.Address.String()
	peerLogger().Debug("sendInitialRoutes ENTER", "peer", addr)

	// setState sets sendingInitialRoutes to 1 in the same call that publishes
	// PeerStateEstablished, and before it, so shouldQueue() is already true when
	// the FSM callback notifies plugins. Here we upgrade 1→2 to indicate the
	// goroutine is actively running. CAS guards against concurrent execution from
	// rapid reconnects (flag would be 2 if another goroutine is already
	// processing). If the flag is 0, the session was torn down before we started
	// — don't run.
	if !p.sendingInitialRoutes.CompareAndSwap(1, 2) {
		peerLogger().Debug("sendInitialRoutes skipped", "peer", addr, "flag", p.sendingInitialRoutes.Load())
		return
	}
	// Flag is cleared inside the mutex after the opQueue drain loop completes,
	// NOT via defer. This ensures shouldQueue() sees a consistent state:
	// either the flag is set (routes will be queued and drained by us),
	// or the flag is cleared and the queue is empty (routes can be sent directly).

	peerLogger().Debug("sendInitialRoutes started", "peer", addr)

	// Get negotiated capabilities for family checks.
	nc := p.negotiated.Load()
	if nc == nil {
		peerLogger().Debug("sendInitialRoutes aborted (no negotiated caps)", "peer", addr)
		p.sendingInitialRoutes.Store(0) // Clear flag so shouldQueue() returns false
		p.initialSyncEOROwed.Store(false)
		p.wakeForwardOverflow()
		return
	}

	peerLogger().Debug("sendInitialRoutes sending static routes", "peer", addr, "count", len(p.settings.StaticRoutes))

	// Mark static config routes so the RIB plugin skips ribOut storage.
	// These routes are always re-sent from config on reconnection; storing
	// them in ribOut would cause duplicates (config + replay).
	// Uses atomic flag checked by notifyMessageReceiver to tag sent events.
	p.sendingConfigStatic.Store(true)

	// RFC 8669 Section 8: "The propagation to other ASes MUST be explicitly
	// configured." One answer for this whole send, because it is a property of
	// the session rather than of a route (Peer.prefixSIDAllowed,
	// forward_prefix_sid.go).
	prefixSIDAllowed := p.prefixSIDAllowed()

	// Calculate max message size for this peer
	maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, nc.ExtendedMessage))

	// Send routes - either grouped or individually based on config.
	if p.settings.GroupUpdates {
		// Group routes by attributes (same attributes = same UPDATE).
		groups := groupRoutesByAttributes(p.settings.StaticRoutes)

		for _, routes := range groups {
			addPath := p.addPathFor(routeFamily(&routes[0]))
			if len(routes) == 1 {
				// Single-route group (IPv6, VPN, LabeledUnicast, or solo IPv4)
				// Resolve next-hop from RouteNextHop policy
				nextHop, nhErr := p.resolveNextHop(routes[0].NextHop, routeFamily(&routes[0]))
				if nhErr != nil {
					routesLogger().Debug("next-hop resolution failed", "peer", addr, "error", nhErr)
					continue
				}
				ub := message.GetUpdateBuilder(p.settings.LocalAS, p.IsIBGP(), p.asn4(), addPath)
				update := buildStaticRouteUpdateNew(ub, &routes[0], nextHop, p.linkLocalNextHopFor(nextHop), p.sendCtx.Load(), prefixSIDAllowed)
				err := p.sendUpdateWithSplit(update, maxMsgSize, addPath)
				message.PutUpdateBuilder(ub)
				if err != nil {
					routesLogger().Debug("send error", "peer", addr, "error", err)
					break
				}
			} else {
				// Multi-route group - IPv4 unicast only (routeGroupKey ensures this)
				// Use size-aware builder to respect max message size
				ub := message.GetUpdateBuilder(p.settings.LocalAS, p.IsIBGP(), p.asn4(), addPath)
				params := make([]message.UnicastParams, 0, len(routes))
				for i := range routes {
					r := &routes[i]
					nextHop, nhErr := p.resolveNextHop(r.NextHop, routeFamily(r))
					if nhErr != nil {
						routesLogger().Debug("next-hop resolution failed", "peer", addr, "prefix", r.Prefix, "error", nhErr)
						continue
					}
					params = append(params, toStaticRouteUnicastParams(r, nextHop, p.linkLocalNextHopFor(nextHop), p.sendCtx.Load(), prefixSIDAllowed))
				}
				if len(params) == 0 {
					message.PutUpdateBuilder(ub)
					continue
				}
				err := ub.BuildGroupedUnicast(params, maxMsgSize, p.SendUpdate)
				message.PutUpdateBuilder(ub)
				if err != nil {
					routesLogger().Debug("grouped unicast error", "peer", addr, "error", err)
					break
				}
			}
			for i := range routes {
				route := &routes[i]
				routesLogger().Debug("route sent", "peer", addr, "prefix", route.Prefix.String(), "nextHop", route.NextHop.String())
			}
		}
	} else {
		// Send each route in its own UPDATE.
		for i := range p.settings.StaticRoutes {
			route := &p.settings.StaticRoutes[i]
			// Resolve next-hop from RouteNextHop policy
			nextHop, nhErr := p.resolveNextHop(route.NextHop, routeFamily(route))
			if nhErr != nil {
				routesLogger().Debug("next-hop resolution failed", "peer", addr, "prefix", route.Prefix, "error", nhErr)
				continue
			}
			addPath := p.addPathFor(routeFamily(route))
			ub := message.GetUpdateBuilder(p.settings.LocalAS, p.IsIBGP(), p.asn4(), addPath)
			update := buildStaticRouteUpdateNew(ub, route, nextHop, p.linkLocalNextHopFor(nextHop), p.sendCtx.Load(), prefixSIDAllowed)
			err := p.sendUpdateWithSplit(update, maxMsgSize, addPath)
			message.PutUpdateBuilder(ub)
			if err != nil {
				routesLogger().Debug("send error", "peer", addr, "error", err)
				break
			}
			routesLogger().Debug("route sent", "peer", addr, "prefix", route.Prefix.String(), "nextHop", route.NextHop.String())
		}
	}

	// Send default routes for families with default-originate enabled.
	// RFC 4271: default route is 0.0.0.0/0 (IPv4) or ::/0 (IPv6).
	// Sent after static routes but still under config-static marker.
	if len(p.settings.DefaultOriginate) > 0 {
		p.sendDefaultOriginateRoutes(nc)
	}

	// Clear config-static marker before opQueue drain so plugin-injected routes
	// (from RIB replay, Python plugins, etc.) are stored in ribOut normally.
	p.sendingConfigStatic.Store(false)

	// Hold the End-of-RIB until every plugin that registers this peer on the
	// peer-up event has processed it, so "End-of-RIB sent" means "this peer is
	// a live forward target" -- what a peer waiting on the marker before it
	// sends reads it to mean. Kept SEPARATE from the API-sync wait below: this
	// one is free in the common case (nothing expected, or an in-process plugin
	// that already acknowledged on the FSM callback goroutine before this
	// goroutine was spawned), so it must not drag in that wait's apiSyncTimeout
	// of 2s (api_sync.go). Bounded; on timeout it releases and says so.
	p.waitPeerUpBarrier()

	// The wait for the processes that push routes into this peer's initial
	// routing update is NOT here. It sits after the last route this goroutine
	// owns, and after the queueing flag is cleared -- see waitForAPISync's call
	// site below and Peer.initialSyncEOROwed (peer.go) for why the two facts are
	// separate.

	// Process operation queue in order (maintains announce/withdraw/teardown ordering).
	// Stop at first teardown - remaining items stay for next session.
	//
	// CONCURRENCY NOTE: Uses index-based loop (not range) so that items appended
	// by concurrent QueueAnnounce/QueueWithdraw calls during unlocked sends are
	// picked up by the next iteration's len(p.opQueue) check. This, combined with
	// shouldQueue() in the announce/withdraw paths, ensures strict insertion order:
	// routes arriving while this loop runs are queued (not sent directly) and
	// processed here in FIFO order.
	var teardownSubcode uint8
	var teardownMsg string
	teardownAutomatic := false
	hasTeardown := false

	// Pre-compute max message size for size checking in PeerOpAnnounce
	opMaxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, nc.ExtendedMessage))

	p.mu.Lock()
	queueLen := len(p.opQueue)
	processed := 0
	connError := false
	for processed < len(p.opQueue) && !connError {
		op := p.opQueue[processed]
		switch op.Type {
		case PeerOpTeardown:
			teardownSubcode = op.Subcode
			teardownMsg = op.Message
			teardownAutomatic = op.Automatic
			hasTeardown = true
			processed++

		case PeerOpAnnounce:
			// Send route, splitting if needed.
			fam := op.Route.NLRI().Family()
			addPath := p.addPathFor(fam)
			attrHandle := getBuildBuf()
			// p.settings.IsIBGP() read directly: this is inside a p.mu.Lock section, so it is
			// already synchronized with the resolveDynamicPeerSettings write. Using the
			// p.IsIBGP() accessor here would deadlock (RLock while holding Lock).
			update := buildRIBRouteUpdate(attrHandle.Buf, op.Route, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
			p.mu.Unlock()
			sendErr := p.sendUpdateWithSplit(update, opMaxMsgSize, addPath)
			putBuildBuf(attrHandle)
			if sendErr != nil {
				routesLogger().Debug("send error for queued route", "peer", addr, "nlri", op.Route.NLRI(), "error", sendErr)
				p.mu.Lock()
				processed++
				// Split errors: skip route. Connection errors: stop processing.
				if !isRouteScopedSendError(sendErr) {
					connError = true
				}
				continue
			}
			p.mu.Lock()
			processed++
			continue

		case PeerOpWithdraw:
			// Send withdrawal using pooled buffer.
			fam := op.NLRI.Family()
			addPath := p.addPathFor(fam)
			wdHandle := getBuildBuf()
			update := buildWithdrawNLRI(wdHandle.Buf, op.NLRI, addPath)
			p.mu.Unlock()
			sendErr := p.sendUpdateWithSplit(update, opMaxMsgSize, addPath)
			putBuildBuf(wdHandle)
			if sendErr != nil {
				routesLogger().Debug("send error for withdrawal", "peer", addr, "nlri", op.NLRI, "error", sendErr)
				p.mu.Lock()
				processed++
				if !isRouteScopedSendError(sendErr) {
					connError = true
				}
				continue
			}
			p.mu.Lock()
			processed++
			continue
		}

		// If we get here, it was a teardown - break out of loop
		break
	}
	// Remove processed items but keep sendingInitialRoutes flag set: this
	// goroutine still owns the wire until it has written the family-specific
	// routes below, so a concurrent plugin command must still be queued rather
	// than overtake them.
	if processed > 0 {
		p.opQueue = p.opQueue[processed:]
	}
	p.mu.Unlock()

	if queueLen > 0 {
		routesLogger().Debug("processed queue ops", "peer", addr, "processed", processed, "remaining", len(p.opQueue), "teardown", hasTeardown)
	}

	// If teardown was in queue, send EOR first, then execute teardown.
	// EOR must be sent BEFORE NOTIFICATION per RFC 4724 Section 4.
	if hasTeardown {
		// Send EOR for ALL negotiated families before teardown.
		//
		// eorSent counts frames that reached the socket, never attempts: a failed
		// send leaves the counter alone so `eor-sent` stays usable as the
		// "end-of-RIB is on the wire" barrier (peer_stats.go incrEORSent). One
		// error here is session-level (ErrNotConnected / ErrInvalidState / a write
		// error), so the remaining families would fail identically -- stop, the
		// same way the queue drain above stops on a connection error.
		for _, fam := range nc.Families() {
			if err := p.SendUpdate(message.BuildEOR(fam)); err != nil {
				routesLogger().Warn("end-of-rib send failed",
					"peer", addr, "family", fam, "phase", "teardown", "error", err)
				break
			}
			p.incrEORSent()
			routesLogger().Debug("sent EOR (before teardown)", "peer", addr, "family", fam)
		}

		routesLogger().Debug("executing queued teardown", "peer", addr, "subcode", teardownSubcode)
		p.mu.RLock()
		session := p.session
		p.mu.RUnlock()
		if session != nil {
			// Set state to Connecting BEFORE Teardown to avoid race condition:
			// Teardown closes TCP, peer immediately reconnects, but if peer.State()
			// still shows Established, the new connection is rejected by collision check.
			// The FSM callback will also set this, but may fire too late.
			p.setState(PeerStateConnecting)
			if err := sessionTeardown(session, teardownSubcode, teardownMsg, teardownAutomatic); err != nil {
				routesLogger().Debug("teardown error", "peer", addr, "error", err)
			}
		}
		// Clear remaining opQueue - these routes were never sent, so shouldn't
		// be re-sent on reconnection. Persist plugin tracks actually-sent routes.
		p.mu.Lock()
		if len(p.opQueue) > 0 {
			routesLogger().Debug("clearing unsent queue items after teardown", "peer", addr, "count", len(p.opQueue))
			p.opQueue = p.opQueue[:0]
		}
		// Clear both facts under mutex for teardown path too: the markers above
		// are the only ones this session gets.
		p.sendingInitialRoutes.Store(0)
		p.initialSyncEOROwed.Store(false)
		p.mu.Unlock()
		p.wakeForwardOverflow()
		return // Don't send family-specific routes after teardown
	}

	// Hold the session write lock across the family-specific routes, and again
	// across the EOR. The route-server plugin, when present, sends EOR via
	// AnnounceEOR -> peer.SendUpdate in a separate goroutine. Without holding
	// writeMu, the RS EOR can interleave between config routes and EOR markers.
	//
	// TWO holds rather than one, because the plugin barrier sits between them
	// and a plugin cannot push its routes through a lock this goroutine is
	// holding while it waits for that plugin. AnnounceEOR is shut out across
	// both holds AND the gap by initialSyncEOROwed rather than by the lock
	// (reactor_api_forward.go).
	//
	// HoldWrites is s.writeMu.Lock (session_write.go). While it is held both
	// egress rails are shut out, but by opposite mechanisms, and this comment
	// had them backwards: the RS FAST PATH is the TryLock and gives up
	// (forward_rs.go tryDirectWriteNoFlush, falling back to the pool), while the
	// per-peer FORWARD POOL worker takes a plain blocking Lock (forward_pool.go
	// fwdBatchHandler) and waits. The asymmetry is deliberate -- the fast path
	// runs on the SOURCE peer's read goroutine, where blocking would stall
	// forwarding to every other destination, so it must never wait.
	//
	// Keep the held window short for the same reason writeMu also gates
	// KEEPALIVE (session_write.go writeMessage): a long hold delays this peer's
	// keepalives and risks its hold timer.
	//
	// Route methods use sendHeld (passed as a callback) to write without
	// re-acquiring writeMu.
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	sendFn := p.sendUpdateDirect
	if session != nil {
		session.HoldWrites()
		sendFn = session.SendUpdateHeld
	}

	// Send family-specific routes (config-originated)
	p.sendPluginRoutesVia(sendFn)

	if session != nil {
		session.releaseWrites()
	}

	// Everything this goroutine owns is on the wire, so the queueing gate closes
	// here rather than after the marker. Drain under the lock until the queue is
	// empty and only then clear the flag: an op appended between the last drain
	// and the clear would otherwise sit in a queue nothing will drain, because
	// this goroutine is the only drainer.
	p.drainAndCloseQueueGate(addr, opMaxMsgSize)

	// The initial sync no longer owns the wire order, so the forwarded UPDATEs
	// parked behind it may go out.
	p.wakeForwardOverflow()

	// Now wait for the processes that push routes into this peer's initial
	// routing update, so their routes precede the End-of-RIB. RFC 4724 Section
	// 4 owes the marker "once it completes the initial routing update", and the
	// owner ruled on 2026-08-30 that a plugin-injected route belongs to that
	// update, so every binding MayPushRoutes reports true for is counted
	// (peer_run.go). ONE bounded wait: it returns the moment every expected
	// `plugin session ready` has arrived, and gives up at apiSyncTimeout for a
	// process that never sends one.
	//
	// It runs with sendingInitialRoutes ALREADY CLEAR, and that is the whole
	// reason the barrier can cover every route-pushing binding. A hold taken
	// while that flag was set also held shouldQueue and forwardOrderHold, so
	// widening the barrier widened the queueing window and the forward-rail
	// parking with it: measured on 2026-08-08, a 500ms hold made
	// test/plugin/role-otc-rs-withdraw-eor.ci deliver the same relayed route to
	// the destination peer TWICE. A route a plugin pushes during this wait now
	// goes straight to the wire, ahead of the marker, which is where a route
	// belonging to the initial update belongs.
	//
	// The marker stays owed across the wait through initialSyncEOROwed, so
	// AnnounceEOR does not slip another producer's marker in front of it and
	// `request quiesce` does not report the peer settled (peer.go).
	p.mu.RLock()
	needsAPIWait := len(p.apiSyncExpected) > 0
	p.mu.RUnlock()
	if needsAPIWait {
		p.waitForAPISync()
	}

	sendFn = p.sendUpdateDirect
	if session != nil {
		session.HoldWrites()
		sendFn = session.SendUpdateHeld
	}

	// Send EOR for ALL negotiated families per RFC 4724 Section 4.
	// RFC 4724: "including the case when there is no update to send"
	// Families() returns families in deterministic order (sorted by AFI, then SAFI).
	//
	// sendFn is session.SendUpdateHeld here (writeMu already held): it writes the
	// UPDATE AND flushes bufWriter before returning (session_write.go
	// SendUpdateHeld), so a nil error means the frame left for the socket, not
	// that it was queued behind the hold. A non-nil error means it did NOT --
	// SendUpdateHeld returns ErrInvalidState without writing anything once the FSM
	// has left Established, and the session can go down between the p.session read
	// above and this call. Counting such an attempt would publish an end-of-RIB
	// the peer never receives, exactly the barrier the compiled functional
	// observers wait on.
	families := nc.Families()
	for _, fam := range families {
		// Claim BEFORE sending. A route server announcing EoR when its replay
		// finishes reaches the same wire through AnnounceEOR, and RFC 4724
		// Section 2 allows one End-of-RIB per family per session.
		if !p.claimInitialSyncEOR(fam) {
			routesLogger().Debug("end-of-rib already sent for this session, skipping",
				"peer", addr, "family", fam, "phase", "initial-sync")
			continue
		}
		if err := sendFn(message.BuildEOR(fam)); err != nil {
			// Hand the claim back: nothing reached the wire, so the other
			// producer must still be allowed to deliver the marker.
			p.releaseInitialSyncEOR(fam)
			routesLogger().Warn("end-of-rib send failed",
				"peer", addr, "family", fam, "phase", "initial-sync", "error", err)
			break
		}
		p.incrEORSent()
		routesLogger().Debug("sent EOR", "peer", addr, "family", fam)
	}

	if session != nil {
		session.releaseWrites()
	}

	// The marker is on the wire, or its send failed and said so. Either way this
	// session owes no other one, so pendingSync settles and AnnounceEOR stops
	// deferring to this producer (peer.go, reactor_api_forward.go).
	p.initialSyncEOROwed.Store(false)
}

// drainAndCloseQueueGate writes every operation the opQueue still holds, then
// closes the queueing gate, both under one p.mu hold.
//
// The two MUST settle together, because sendInitialRoutes is the only drainer.
// Clearing the flag first would let QueueAnnounce append behind the last pass
// and park that route for the life of the session; draining first and clearing
// after an unlock would leave the same hole in a smaller window. The loop
// re-reads len(p.opQueue) each pass for the reason the main drain does: a send
// runs unlocked, so a concurrent QueueAnnounce can append while it runs.
//
// Called with p.mu NOT held. addr and opMaxMsgSize are the caller's, so the peer
// address is formatted once per sync rather than once per operation.
func (p *Peer) drainAndCloseQueueGate(addr string, opMaxMsgSize int) {
	p.mu.Lock()
	finalProcessed := 0
	for finalProcessed < len(p.opQueue) {
		op := p.opQueue[finalProcessed]
		switch op.Type {
		case PeerOpAnnounce:
			fam := op.Route.NLRI().Family()
			addPath := p.addPathFor(fam)
			attrHandle := getBuildBuf()
			// p.settings.IsIBGP() read directly: this is inside a p.mu.Lock section, so it is
			// already synchronized with the resolveDynamicPeerSettings write. Using the
			// p.IsIBGP() accessor here would deadlock (RLock while holding Lock).
			update := buildRIBRouteUpdate(attrHandle.Buf, op.Route, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
			p.mu.Unlock()
			sendErr := p.sendUpdateWithSplit(update, opMaxMsgSize, addPath)
			putBuildBuf(attrHandle)
			if sendErr != nil {
				routesLogger().Debug("send error for a queued route", "peer", addr, "error", sendErr)
				p.mu.Lock()
				finalProcessed++
				// Every remaining operation is attempted, a connection error
				// included, so this is the one drain that does not sort the two
				// kinds of send error apart (isRouteScopedSendError,
				// peer_send.go). The gate below closes on an EMPTY queue and this
				// goroutine is the only drainer, so an operation left behind is
				// one nothing will ever drain. The main drain loop stops on a
				// connection error instead, because the session still owns the
				// wire there.
				continue
			}
			p.mu.Lock()
			finalProcessed++

		case PeerOpWithdraw:
			fam := op.NLRI.Family()
			addPath := p.addPathFor(fam)
			wdHandle := getBuildBuf()
			update := buildWithdrawNLRI(wdHandle.Buf, op.NLRI, addPath)
			p.mu.Unlock()
			sendErr := p.sendUpdateWithSplit(update, opMaxMsgSize, addPath)
			putBuildBuf(wdHandle)
			if sendErr != nil {
				routesLogger().Debug("send error for a queued withdrawal", "peer", addr, "error", sendErr)
				p.mu.Lock()
				finalProcessed++
				// Attempted to the end, for the reason the announce case above
				// states.
				continue
			}
			p.mu.Lock()
			finalProcessed++

		case PeerOpTeardown:
			// Teardown should not appear here — teardown is handled in the main
			// drain loop, which returns early.
			routesLogger().Error("unexpected teardown in the closing drain queue", "peer", addr)
			finalProcessed++
		}
	}
	if finalProcessed > 0 {
		p.opQueue = p.opQueue[finalProcessed:]
		routesLogger().Debug("drained queued ops before the end-of-rib", "peer", addr, "count", finalProcessed)
	}
	p.sendingInitialRoutes.Store(0)
	p.mu.Unlock()
}

// sendUpdateDirect is the default send callback when writeMu is not held.
func (p *Peer) sendUpdateDirect(update *message.Update) error {
	return p.SendUpdate(update)
}

// pluginRouteGroup accumulates the NLRIs of same-family same-attribute routes
// that share a single UPDATE (MVPN, RFC 6514).
type pluginRouteGroup struct {
	fam   family.Family
	rep   *PluginRoute
	nlris [][]byte
}

// pluginUpdateSize returns the on-wire size of a built plugin-route UPDATE.
// Plugin/MP_REACH families carry no inline NLRI, so the size is the 19-byte
// header plus the withdrawn-routes(2) + total-path-attribute-length(2) fields
// plus the path attributes.
func pluginUpdateSize(u *message.Update) int {
	return message.HeaderLen + 4 + len(u.PathAttributes)
}

// packNLRIs greedily groups NLRIs into batches whose built UPDATE size (per the
// measure callback) stays within maxSize (RFC 8654 max message size). A single
// NLRI that alone exceeds maxSize is emitted in its own batch -- it cannot be
// split further. Each returned batch is the concatenation of its NLRIs.
func packNLRIs(nlris [][]byte, maxSize int, measure func(batch []byte) int) [][]byte {
	var batches [][]byte
	var cur []byte
	for _, nlri := range nlris {
		if len(cur) == 0 {
			cur = append([]byte(nil), nlri...)
			continue
		}
		trial := make([]byte, 0, len(cur)+len(nlri))
		trial = append(trial, cur...)
		trial = append(trial, nlri...)
		if measure(trial) > maxSize {
			batches = append(batches, cur)
			cur = append([]byte(nil), nlri...)
			continue
		}
		cur = trial
	}
	if len(cur) > 0 {
		batches = append(batches, cur)
	}
	return batches
}

// pluginRouteGroupKey returns a grouping key: routes with the same key share an
// UPDATE. Family encodes AFI/SAFI; the remaining fields are the shared path
// attributes (next-hop, AS_PATH, LOCAL_PREF, and the pre-built raw attrs which
// carry ORIGIN/MED/ext-community/originator/cluster/NEXT_HOP).
func pluginRouteGroupKey(r *PluginRoute) string {
	var b keyBuilder
	b.Grow(96)
	b.WriteString(r.Family)
	b.Sep()
	b.Addr(r.NextHop)
	b.Sep()
	b.uint32Slice(r.ASPath)
	b.Sep()
	b.Uint(uint64(r.LocalPreference))
	for _, raw := range r.RawAttrs {
		b.Sep()
		b.Hex(raw)
	}
	return b.String()
}

// sendPluginRoutesVia sends generic plugin-registered routes. Routes whose
// plugin sets Group=true (MVPN) are packed by shared attributes into a single
// UPDATE (one MP_REACH, multiple NLRIs); all others are sent one per UPDATE.
func (p *Peer) sendPluginRoutesVia(sendFn func(*message.Update) error) {
	nc := p.negotiated.Load()
	if nc == nil || len(p.settings.PluginRoutes) == 0 {
		return
	}

	addr := p.settings.Address.String()
	// RFC 8654: respect the negotiated max message size (4096 or 65535).
	maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, nc.ExtendedMessage))

	// Pass 1: grouped routes packed by shared attributes, sent in deterministic
	// key order (family first, so IPv4 families precede IPv6, then by attributes).
	groups := map[string]*pluginRouteGroup{}
	var groupOrder []string
	for i := range p.settings.PluginRoutes {
		route := &p.settings.PluginRoutes[i]
		if !route.Group {
			continue
		}
		fam, ok := family.LookupFamily(route.Family)
		if !ok || !nc.Has(fam) {
			continue
		}
		key := pluginRouteGroupKey(route)
		g := groups[key]
		if g == nil {
			g = &pluginRouteGroup{fam: fam, rep: route}
			groups[key] = g
			groupOrder = append(groupOrder, key)
		}
		g.nlris = append(g.nlris, route.NLRI)
	}
	sort.Strings(groupOrder)
	for _, key := range groupOrder {
		p.sendPluginRouteGroup(groups[key], maxMsgSize, sendFn, addr)
	}

	// Pass 2: non-grouped routes, one per UPDATE, in config order.
	for i := range p.settings.PluginRoutes {
		route := &p.settings.PluginRoutes[i]
		if route.Group {
			continue
		}
		fam, ok := family.LookupFamily(route.Family)
		if !ok {
			routesLogger().Debug("skipping plugin route (unknown family)", "peer", addr, "family", route.Family)
			continue
		}
		if !nc.Has(fam) {
			routesLogger().Debug("skipping plugin route (not negotiated)", "peer", addr, "family", route.Family)
			continue
		}

		addPath := p.addPathFor(fam)
		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.IsIBGP(), p.asn4(), addPath)
		// RFC 8669 Section 8, as for the static routes above: a plugin route's
		// pre-built attribute bytes can carry type code 40.
		update := ub.BuildPlugin(toPluginParams(*route, fam, p.prefixSIDAllowed()))
		// A single plugin route (e.g. an atomic FlowSpec rule) cannot be split;
		// skip it rather than emit an UPDATE the peer would reject.
		if pluginUpdateSize(update) > maxMsgSize {
			message.PutUpdateBuilder(ub)
			routesLogger().Debug("skipping plugin route (exceeds max message size)", "peer", addr, "family", route.Family)
			continue
		}
		if err := sendFn(update); err != nil {
			routesLogger().Debug("plugin route send error", "peer", addr, "family", route.Family, "error", err)
		}
		message.PutUpdateBuilder(ub)
	}
}

// concatNLRIs joins a group's per-route NLRIs into one contiguous buffer.
func concatNLRIs(nlris [][]byte) []byte {
	total := 0
	for _, n := range nlris {
		total += len(n)
	}
	out := make([]byte, 0, total)
	for _, n := range nlris {
		out = append(out, n...)
	}
	return out
}

// sendPluginRouteGroup emits a grouped route (MVPN). The common case -- the whole
// group fits one UPDATE -- builds exactly once. Only when the concatenated group
// exceeds maxMsgSize does it fall back to packNLRIs to split across UPDATEs (the
// rare path; that split does O(n^2) sizing builds, acceptable since oversized
// same-attribute groups are uncommon and this runs only at session establishment).
func (p *Peer) sendPluginRouteGroup(g *pluginRouteGroup, maxMsgSize int, sendFn func(*message.Update) error, addr string) {
	addPath := p.addPathFor(g.fam)
	// RFC 8669 Section 8, as in the non-grouped pass (sendPluginRoutesVia).
	base := toPluginParams(*g.rep, g.fam, p.prefixSIDAllowed())
	emit := func(nlri []byte) {
		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.IsIBGP(), p.asn4(), addPath)
		params := base
		params.NLRI = nlri
		update := ub.BuildPlugin(params)
		if err := sendFn(update); err != nil {
			routesLogger().Debug("plugin route group send error", "peer", addr, "family", g.rep.Family, "error", err)
		}
		message.PutUpdateBuilder(ub)
	}

	// Common case: build the whole group once and send it if it fits.
	full := concatNLRIs(g.nlris)
	ub := message.GetUpdateBuilder(p.settings.LocalAS, p.IsIBGP(), p.asn4(), addPath)
	params := base
	params.NLRI = full
	update := ub.BuildPlugin(params)
	fits := pluginUpdateSize(update) <= maxMsgSize
	if fits {
		if err := sendFn(update); err != nil {
			routesLogger().Debug("plugin route group send error", "peer", addr, "family", g.rep.Family, "error", err)
		}
	}
	message.PutUpdateBuilder(ub)
	if fits {
		return
	}

	// Oversized group: split NLRIs across multiple size-bounded UPDATEs.
	batches := packNLRIs(g.nlris, maxMsgSize, func(batch []byte) int {
		mub := message.GetUpdateBuilder(p.settings.LocalAS, p.IsIBGP(), p.asn4(), addPath)
		mparams := base
		mparams.NLRI = batch
		sz := pluginUpdateSize(mub.BuildPlugin(mparams))
		message.PutUpdateBuilder(mub)
		return sz
	})
	for _, batch := range batches {
		emit(batch)
	}
}

// defaultRouteForAFI returns the default prefix and a valid next-hop for the given AFI.
// Returns ok=false if the AFI is not IPv4 or IPv6 unicast.
func defaultRouteForAFI(afi family.AFI, hint netip.Addr) (prefix netip.Prefix, nextHop netip.Addr, ok bool) {
	if afi == family.AFIIPv4 {
		prefix = netip.MustParsePrefix("0.0.0.0/0")
		nextHop = hint
		if !nextHop.IsValid() {
			nextHop = netip.MustParseAddr("0.0.0.0")
		}
		return prefix, nextHop, true
	}
	if afi == family.AFIIPv6 {
		prefix = netip.MustParsePrefix("::/0")
		nextHop = hint
		if !nextHop.IsValid() || nextHop.Is4() {
			nextHop = netip.IPv6Loopback()
		}
		return prefix, nextHop, true
	}
	return netip.Prefix{}, netip.Addr{}, false
}

// sendDefaultOriginateRoutes sends default routes (0.0.0.0/0 or ::/0) for families
// that have default-originate enabled in config.
// RFC 4271: default route originated as a normal UPDATE with ORIGIN IGP.
// When a per-family default-originate-filter is configured, the synthetic default
// route is run through that single named filter as a dry-run; if the filter
// rejects, the default route is not originated (AC-7/AC-8 of cmd-2).
func (p *Peer) sendDefaultOriginateRoutes(nc *NegotiatedCapabilities) {
	addr := p.settings.Address.String()

	for familyKey, enabled := range p.settings.DefaultOriginate {
		if !enabled {
			continue
		}

		fam, ok := family.LookupFamily(familyKey)
		if !ok {
			routesLogger().Debug("default-originate: unknown family", "peer", addr, "family", familyKey)
			continue
		}

		if !nc.Has(fam) {
			routesLogger().Debug("default-originate: family not negotiated", "peer", addr, "family", familyKey)
			continue
		}

		// Resolve default prefix / next-hop BEFORE acquiring the builder so
		// early-return paths don't need a Put.
		var nextHop netip.Addr
		if p.settings.LocalAddress.IsValid() {
			nextHop = p.settings.LocalAddress
		}

		defaultPrefix, nh, ok := defaultRouteForAFI(fam.AFI, nextHop)
		if !ok {
			routesLogger().Debug("default-originate: unsupported family AFI", "peer", addr, "family", familyKey)
			continue
		}
		nextHop = nh

		// Per-family conditional filter check (dry-run).
		// An empty filter name means unconditional origination.
		if filterName := p.settings.DefaultOriginateFilter[familyKey]; filterName != "" {
			if !p.defaultOriginateFilterAccepts(filterName, fam, defaultPrefix, nextHop) {
				routesLogger().Debug("default-originate: filter rejected",
					"peer", addr, "family", familyKey, "filter", filterName)
				continue
			}
		}

		// Build a default route UPDATE: 0.0.0.0/0 for IPv4, ::/0 for IPv6.
		addPath := p.addPathFor(fam)
		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.IsIBGP(), p.asn4(), addPath)
		params := message.UnicastParams{
			Prefix:  defaultPrefix,
			NextHop: nextHop,
			Origin:  attribute.OriginIGP,
			// RFC 2545 Section 3 binds every advertisement of an IPv6 route, and a
			// default route is one: the link-local address is appended after the
			// global one when the speaker shares a subnet with both the next-hop
			// entity and this peer. linkLocalNextHopFor returns the zero Addr in
			// every other case, and buildMPReach then writes the 16-octet form.
			LinkLocalNextHop: p.linkLocalNextHopFor(nextHop),
		}
		update := ub.BuildUnicast(&params)
		err := p.SendUpdate(update)
		message.PutUpdateBuilder(ub)
		if err != nil {
			routesLogger().Debug("default-originate send error", "peer", addr, "family", familyKey, "error", err)
			continue
		}
		routesLogger().Debug("sent default route", "peer", addr, "family", familyKey)
	}
}

// defaultOriginateFilterAccepts runs the named filter as a dry-run against a
// synthetic default-route update and returns true if the filter accepts.
// Fail-closed: missing reactor, missing API server, or a malformed filter
// reference all return false. This matches the existing policy filter chain
// behavior (filter_chain.go policyFilterFunc) and the principle that an
// unreachable filter must not silently emit unfiltered routes.
func (p *Peer) defaultOriginateFilterAccepts(filterName string, fam family.Family, prefix netip.Prefix, nextHop netip.Addr) bool {
	// Reject malformed filter ref -- expect "<plugin>:<filter>".
	// Checked first so operators learn about typos before any transport lookup.
	if !strings.Contains(filterName, ":") {
		routesLogger().Warn("default-originate: invalid filter ref (expected plugin:filter) -- fail-closed",
			"peer", p.settings.Address.String(), "filter", filterName)
		return false
	}
	if p.reactor == nil || p.reactor.api == nil {
		routesLogger().Warn("default-originate: no reactor API -- fail-closed",
			"peer", p.settings.Address.String(), "filter", filterName)
		return false
	}
	// A raw filter operates on the received UPDATE's wire bytes, but the
	// default-originate route is synthetic and has none: the filter would
	// evaluate empty hex and decide on nothing. Fail-closed so the operator
	// switches to a text filter rather than shipping a default gated on nothing.
	if defaultOriginateRejectsRawFilter(p.reactor.api, filterName, p.settings.Address.String()) {
		return false
	}
	// Synthesize the update text the filter would see for this default route.
	// Format matches the ingress/egress policy text contract:
	//   "origin igp next-hop <ip> nlri <family> add <prefix>"
	var scratchArr [256]byte
	scratch := append(scratchArr[:0], "origin igp next-hop "...)
	scratch = nextHop.AppendTo(scratch)
	scratch = append(scratch, " nlri "...)
	scratch = append(scratch, fam.String()...)
	scratch = append(scratch, " add "...)
	scratch = prefix.AppendTo(scratch)
	updateText := unsafe.String(unsafe.SliceData(scratch), len(scratch)) //nolint:gosec // audited: scratch outlives synchronous PolicyFilterChain+CallRPC

	res := PolicyFilterChain(
		[]filterapi.FilterRef{{Name: filterName}},
		"export",
		p.settings.Address.String(),
		// Guarded PeerAS: sendInitialRoutes runs on its own goroutine and can outlive a
		// teardown, so a stale run may read PeerAS while a new establishment writes it.
		p.PeerAS(),
		updateText,
		p.reactor.policyFilterFunc(nil), // nil payload -- synthetic update
	)
	return res.Action != PolicyReject
}
