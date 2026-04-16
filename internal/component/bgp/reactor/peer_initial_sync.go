// Design: docs/architecture/core-design.md — initial route sending on BGP session establishment
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"runtime"
	"sort"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
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
			// Clear flag so ShouldQueue() returns false and peer isn't stuck.
			p.sendingInitialRoutes.Store(0)
		}
	}()
	addr := p.settings.Address.String()
	peerLogger().Debug("sendInitialRoutes ENTER", "peer", addr)

	// The FSM callback sets sendingInitialRoutes to 1 before notifying plugins,
	// ensuring ShouldQueue() returns true during event delivery. Here we upgrade
	// 1→2 to indicate the goroutine is actively running. CAS guards against
	// concurrent execution from rapid reconnects (flag would be 2 if another
	// goroutine is already processing). If the flag is 0, the session was torn
	// down before we started — don't run.
	if !p.sendingInitialRoutes.CompareAndSwap(1, 2) {
		peerLogger().Debug("sendInitialRoutes skipped", "peer", addr, "flag", p.sendingInitialRoutes.Load())
		return
	}
	// Flag is cleared inside the mutex after the opQueue drain loop completes,
	// NOT via defer. This ensures ShouldQueue() sees a consistent state:
	// either the flag is set (routes will be queued and drained by us),
	// or the flag is cleared and the queue is empty (routes can be sent directly).

	peerLogger().Debug("sendInitialRoutes started", "peer", addr)

	// Get negotiated capabilities for family checks.
	nc := p.negotiated.Load()
	if nc == nil {
		peerLogger().Debug("sendInitialRoutes aborted (no negotiated caps)", "peer", addr)
		p.sendingInitialRoutes.Store(0) // Clear flag so ShouldQueue() returns false
		return
	}

	peerLogger().Debug("sendInitialRoutes sending static routes", "peer", addr, "count", len(p.settings.StaticRoutes))

	// Mark static config routes so the RIB plugin skips ribOut storage.
	// These routes are always re-sent from config on reconnection; storing
	// them in ribOut would cause duplicates (config + replay).
	// Uses atomic flag checked by notifyMessageReceiver to tag sent events.
	p.sendingConfigStatic.Store(true)

	// Calculate max message size for this peer
	maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))

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
				ub := message.GetUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
				update := buildStaticRouteUpdateNew(ub, &routes[0], nextHop, p.settings.LinkLocal, p.sendCtx.Load())
				err := p.sendUpdateWithSplit(update, maxMsgSize, addPath)
				message.PutUpdateBuilder(ub)
				if err != nil {
					routesLogger().Debug("send error", "peer", addr, "error", err)
					break
				}
			} else {
				// Multi-route group - IPv4 unicast only (routeGroupKey ensures this)
				// Use size-aware builder to respect max message size
				ub := message.GetUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
				params := make([]message.UnicastParams, 0, len(routes))
				for i := range routes {
					r := &routes[i]
					nextHop, nhErr := p.resolveNextHop(r.NextHop, routeFamily(r))
					if nhErr != nil {
						routesLogger().Debug("next-hop resolution failed", "peer", addr, "prefix", r.Prefix, "error", nhErr)
						continue
					}
					params = append(params, toStaticRouteUnicastParams(r, nextHop, p.settings.LinkLocal, p.sendCtx.Load()))
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
			ub := message.GetUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
			update := buildStaticRouteUpdateNew(ub, route, nextHop, p.settings.LinkLocal, p.sendCtx.Load())
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

	// Wait for API processes to send initial routes before processing queue.
	// Two-phase wait:
	// 1. Minimum 500ms delay for external plugins that send routes via IPC
	//    (their state=up handling involves pipe round-trips not tracked by apiSync).
	// 2. Channel-based sync for internal plugins like bgp-rib that signal
	//    "plugin session ready" after replaying routes (may take longer than 500ms
	//    under load).
	p.mu.RLock()
	needsAPIWait := p.apiSyncExpected > 0
	p.mu.RUnlock()
	if needsAPIWait {
		routesLogger().Debug("sleeping for API routes", "peer", addr, "duration", "500ms")
		p.clock.Sleep(500 * time.Millisecond)
		p.waitForAPISync(2 * time.Second)
	}

	// Process operation queue in order (maintains announce/withdraw/teardown ordering).
	// Stop at first teardown - remaining items stay for next session.
	//
	// CONCURRENCY NOTE: Uses index-based loop (not range) so that items appended
	// by concurrent QueueAnnounce/QueueWithdraw calls during unlocked sends are
	// picked up by the next iteration's len(p.opQueue) check. This, combined with
	// ShouldQueue() in the announce/withdraw paths, ensures strict insertion order:
	// routes arriving while this loop runs are queued (not sent directly) and
	// processed here in FIFO order.
	var teardownSubcode uint8
	var teardownMsg string
	hasTeardown := false

	// Pre-compute max message size for size checking in PeerOpAnnounce
	opMaxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))

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
			hasTeardown = true
			processed++

		case PeerOpAnnounce:
			// Send route, splitting if needed.
			fam := op.Route.NLRI().Family()
			addPath := p.addPathFor(fam)
			attrHandle := getBuildBuf()
			update := buildRIBRouteUpdate(attrHandle.Buf, op.Route, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
			p.mu.Unlock()
			sendErr := p.sendUpdateWithSplit(update, opMaxMsgSize, addPath)
			putBuildBuf(attrHandle)
			if sendErr != nil {
				routesLogger().Debug("send error for queued route", "peer", addr, "nlri", op.Route.NLRI(), "error", sendErr)
				p.mu.Lock()
				processed++
				// Split errors: skip route. Connection errors: stop processing.
				if !errors.Is(sendErr, message.ErrAttributesTooLarge) && !errors.Is(sendErr, message.ErrNLRITooLarge) {
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
				if !errors.Is(sendErr, message.ErrAttributesTooLarge) && !errors.Is(sendErr, message.ErrNLRITooLarge) {
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
	// Remove processed items but keep sendingInitialRoutes flag set.
	// The flag is cleared AFTER EOR to prevent a race where a plugin command
	// arrives between flag-clear and EOR-send, bypasses the queue, and races
	// with EOR for the session write lock. With the flag set, concurrent
	// plugin commands are queued (ShouldQueue=true) and drained after EOR.
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
		// Send EOR for ALL negotiated families before teardown
		for _, fam := range nc.Families() {
			_ = p.SendUpdate(message.BuildEOR(fam))
			p.IncrEORSent()
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
			if err := session.Teardown(teardownSubcode, teardownMsg); err != nil {
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
		// Clear flag under mutex for teardown path too
		p.sendingInitialRoutes.Store(0)
		p.mu.Unlock()
		return // Don't send family-specific routes after teardown
	}

	// Send family-specific routes (config-originated)
	p.sendMVPNRoutes()
	p.sendVPLSRoutes()
	p.sendFlowSpecRoutes()
	p.sendMUPRoutes()

	// Send EOR for ALL negotiated families per RFC 4724 Section 4.
	// RFC 4724: "including the case when there is no update to send"
	// IMPORTANT: EORs must be sent AFTER all routes for each family.
	// Families() returns families in deterministic order (sorted by AFI, then SAFI).
	for _, fam := range nc.Families() {
		_ = p.SendUpdate(message.BuildEOR(fam))
		p.IncrEORSent()
		routesLogger().Debug("sent EOR", "peer", addr, "family", fam)
	}

	// Drain any commands that were queued while EOR was being sent.
	// The sendingInitialRoutes flag was kept set during EOR to ensure
	// concurrent plugin commands were queued (not sent directly).
	// Routes drained here arrive at the peer after EOR, which is correct:
	// they are incremental updates after the initial RIB dump.
	p.mu.Lock()
	finalProcessed := 0
	for finalProcessed < len(p.opQueue) {
		op := p.opQueue[finalProcessed]
		switch op.Type {
		case PeerOpAnnounce:
			fam := op.Route.NLRI().Family()
			addPath := p.addPathFor(fam)
			attrHandle := getBuildBuf()
			update := buildRIBRouteUpdate(attrHandle.Buf, op.Route, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
			p.mu.Unlock()
			sendErr := p.sendUpdateWithSplit(update, opMaxMsgSize, addPath)
			putBuildBuf(attrHandle)
			if sendErr != nil {
				routesLogger().Debug("send error for late-queued route", "peer", addr, "error", sendErr)
				p.mu.Lock()
				finalProcessed++
				if !errors.Is(sendErr, message.ErrAttributesTooLarge) && !errors.Is(sendErr, message.ErrNLRITooLarge) {
					break // Connection error — stop processing
				}
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
				routesLogger().Debug("send error for late-queued withdrawal", "peer", addr, "error", sendErr)
				p.mu.Lock()
				finalProcessed++
				if !errors.Is(sendErr, message.ErrAttributesTooLarge) && !errors.Is(sendErr, message.ErrNLRITooLarge) {
					break
				}
				continue
			}
			p.mu.Lock()
			finalProcessed++

		case PeerOpTeardown:
			// Teardown should not appear in the post-EOR queue — teardown
			// is handled in the main drain loop and returns early.
			routesLogger().Error("unexpected teardown in post-EOR queue", "peer", addr)
			finalProcessed++
		}
	}
	if finalProcessed > 0 {
		p.opQueue = p.opQueue[finalProcessed:]
		routesLogger().Debug("drained late-queued ops after EOR", "peer", addr, "count", finalProcessed)
	}
	p.sendingInitialRoutes.Store(0)
	p.mu.Unlock()
}

// sendMVPNRoutes sends MVPN routes configured for this peer.
func (p *Peer) sendMVPNRoutes() {
	nc := p.negotiated.Load()
	if nc == nil {
		return
	}

	addr := p.settings.Address.String()

	// Group MVPN routes by AFI, filtering by negotiated families
	var ipv4Routes, ipv6Routes []MVPNRoute
	var skippedIPv4, skippedIPv6 int

	for i := range p.settings.MVPNRoutes {
		route := &p.settings.MVPNRoutes[i]
		if route.IsIPv6 {
			if nc.Has(family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIMVPN}) {
				ipv6Routes = append(ipv6Routes, *route)
			} else {
				skippedIPv6++
			}
		} else {
			if nc.Has(family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMVPN}) {
				ipv4Routes = append(ipv4Routes, *route)
			} else {
				skippedIPv4++
			}
		}
	}

	if skippedIPv4 > 0 {
		routesLogger().Debug("skipping IPv4 MVPN routes (not negotiated)", "peer", addr, "count", skippedIPv4)
	}
	if skippedIPv6 > 0 {
		routesLogger().Debug("skipping IPv6 MVPN routes (not negotiated)", "peer", addr, "count", skippedIPv6)
	}

	// RFC 8654: Respect peer's max message size (4096 or 65535)
	maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))

	// Send IPv4 MVPN routes grouped by attributes (sorted for deterministic order)
	if len(ipv4Routes) > 0 {
		ipv4MVPNFamily := family.Family{AFI: 1, SAFI: 5} // IPv4 MVPN
		addPath := p.addPathFor(ipv4MVPNFamily)
		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		defer message.PutUpdateBuilder(ub)
		ipv4Groups := groupMVPNRoutesByKey(ipv4Routes)
		for _, key := range sortedKeys(ipv4Groups) {
			routes := ipv4Groups[key]
			if err := ub.BuildGroupedMVPN(toMVPNParams(routes), maxMsgSize, p.SendUpdate); err != nil {
				if isMVPNBuildError(err) {
					routesLogger().Debug("MVPN build error", "peer", addr, "error", err)
				} else {
					routesLogger().Debug("MVPN send error", "peer", addr, "error", err)
				}
				continue
			}
			routesLogger().Debug("sent IPv4 MVPN routes", "peer", addr, "routes", len(routes))
		}
	}

	// Send IPv6 MVPN routes grouped by attributes (sorted for deterministic order)
	if len(ipv6Routes) > 0 {
		ipv6MVPNFamily := family.Family{AFI: 2, SAFI: 5} // IPv6 MVPN
		addPath := p.addPathFor(ipv6MVPNFamily)
		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		defer message.PutUpdateBuilder(ub)
		ipv6Groups := groupMVPNRoutesByKey(ipv6Routes)
		for _, key := range sortedKeys(ipv6Groups) {
			routes := ipv6Groups[key]
			if err := ub.BuildGroupedMVPN(toMVPNParams(routes), maxMsgSize, p.SendUpdate); err != nil {
				if isMVPNBuildError(err) {
					routesLogger().Debug("MVPN build error", "peer", addr, "error", err)
				} else {
					routesLogger().Debug("MVPN send error", "peer", addr, "error", err)
				}
				continue
			}
			routesLogger().Debug("sent IPv6 MVPN routes", "peer", addr, "routes", len(routes))
		}
	}
	// Note: EORs are sent by the generic loop in sendInitialRoutes() for ALL
	// negotiated families, so we don't send family-specific EORs here.
}

// isMVPNBuildError reports whether err originates from BuildGroupedMVPN's
// build-side validation (attrs/NLRI/update size) rather than from the emit
// callback's send-side failure. Distinguishes operator-visible categories so a
// flapping session isn't confused with a malformed MVPN config.
//
// ASSUMES the emit callback (p.SendUpdate) NEVER returns or wraps any of these
// size sentinels -- today true because SendUpdate's error surface is limited to
// ErrNotConnected, ErrInvalidState, and raw TCP write errors. If SendUpdate
// ever grows to propagate a build sentinel, this classifier would misattribute;
// switch to a `sentAny bool` flag in the emit closure if that changes.
//
// Matches the idiom at peer_initial_sync.go:214,236,338,358 which uses the same
// sentinels (minus ErrUpdateTooLarge -- sendUpdateWithSplit never produces that;
// only BuildGroupedMVPN's defense-in-depth check does) to gate connError.
func isMVPNBuildError(err error) bool {
	return errors.Is(err, message.ErrAttributesTooLarge) ||
		errors.Is(err, message.ErrNLRITooLarge) ||
		errors.Is(err, message.ErrUpdateTooLarge)
}

// mvpnRouteGroupKey generates a grouping key for MVPN routes.
// Routes with identical keys can share path attributes in one UPDATE.
//
// Fields in key (shared UPDATE attributes per RFC 4271 Section 4.3):
// - NextHop, Origin, LocalPreference, MED: Standard path attributes.
// - ExtCommunityBytes: Route Targets for VPN isolation (RFC 4360).
// - OriginatorID, ClusterList: Route reflector attributes (RFC 4456).
//
// Fields NOT in key (per-NLRI, not per-UPDATE):
// - IsIPv6: Routes pre-separated by AFI before grouping.
// - RouteType: Multiple types allowed in same UPDATE.
// - RD: Per-NLRI field in MP_REACH_NLRI.
// - SourceAS, Source, Group: Per-NLRI fields.
//
// RFC 4456 Section 8: ClusterList is ordered (RRs prepend their CLUSTER_ID).
// Routes with same cluster IDs in different order traversed different paths
// and MUST NOT be grouped together. ClusterList is intentionally not sorted.
func mvpnRouteGroupKey(r MVPNRoute) string {
	return fmt.Sprintf("%s|%d|%d|%d|%s|%d|%v",
		r.NextHop.String(),
		r.Origin,
		r.LocalPreference,
		r.MED,
		hex.EncodeToString(r.ExtCommunityBytes),
		r.OriginatorID,
		r.ClusterList,
	)
}

// groupMVPNRoutesByKey groups MVPN routes by attribute key.
// Routes with same key can share path attributes in a single UPDATE message.
func groupMVPNRoutesByKey(routes []MVPNRoute) map[string][]MVPNRoute {
	groups := make(map[string][]MVPNRoute)
	for i := range routes {
		key := mvpnRouteGroupKey(routes[i])
		groups[key] = append(groups[key], routes[i])
	}
	return groups
}

// sortedKeys returns map keys in sorted order for deterministic iteration.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sendVPLSRoutes sends VPLS routes configured for this peer.
func (p *Peer) sendVPLSRoutes() {
	nc := p.negotiated.Load()
	if nc == nil || !nc.Has(family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIVPLS}) {
		if len(p.settings.VPLSRoutes) > 0 {
			addr := p.settings.Address.String()
			routesLogger().Debug("skipping VPLS routes (L2VPN VPLS not negotiated)", "peer", addr, "count", len(p.settings.VPLSRoutes))
		}
		return
	}

	addr := p.settings.Address.String()

	if len(p.settings.VPLSRoutes) > 0 {
		routesLogger().Debug("sending VPLS routes", "peer", addr, "count", len(p.settings.VPLSRoutes))
		// VPLS family: AFI=25 (L2VPN), SAFI=65 (VPLS)
		// Note: VPLS doesn't support ADD-PATH
		vplsFamily := family.Family{AFI: 25, SAFI: 65}
		addPath := p.addPathFor(vplsFamily)
		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		defer message.PutUpdateBuilder(ub)
		for i := range p.settings.VPLSRoutes {
			update := ub.BuildVPLS(toVPLSParams(p.settings.VPLSRoutes[i]))
			if err := p.SendUpdate(update); err != nil {
				routesLogger().Debug("VPLS send error", "peer", addr, "error", err)
			}
		}
	}
	// Note: EORs are sent by the generic loop in sendInitialRoutes() for ALL
	// negotiated families, so we don't send family-specific EORs here.
}

// sendFlowSpecRoutes sends FlowSpec routes configured for this peer.
// Only sends routes for families that were successfully negotiated.
// Per RFC 4724 Section 4, EOR is sent for all negotiated families,
// "including the case when there is no update to send".
func (p *Peer) sendFlowSpecRoutes() {
	nc := p.negotiated.Load()
	if nc == nil {
		return
	}

	addr := p.settings.Address.String()

	// RFC 8654: Respect peer's max message size (4096 or 65535)
	maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))

	// Send routes only for negotiated families
	var sentCount int
	for i := range p.settings.FlowSpecRoutes {
		route := &p.settings.FlowSpecRoutes[i]
		// Check if this route's family is negotiated
		isIPv6 := route.IsIPv6
		isVPN := route.RD != [8]byte{}

		var fam family.Family
		switch {
		case !isIPv6 && !isVPN:
			fam = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
		case !isIPv6 && isVPN:
			fam = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN}
		case isIPv6 && !isVPN:
			fam = family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}
		case isIPv6 && isVPN:
			fam = family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpecVPN}
		}

		if !nc.Has(fam) {
			routesLogger().Debug("skipping FlowSpec route (family not negotiated)", "peer", addr)
			continue
		}

		// Determine FlowSpec family: AFI 1/2, SAFI 133 (unicast) or 134 (VPN)
		afi := uint16(1)
		if isIPv6 {
			afi = 2
		}
		safi := uint8(133)
		if isVPN {
			safi = 134
		}
		addPath := p.addPathFor(family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)})
		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		// RFC 5575 Section 4: FlowSpec NLRI max 4095 bytes.
		// Single FlowSpec rule is atomic - cannot be split across UPDATEs.
		update, err := ub.BuildFlowSpecWithMaxSize(toFlowSpecParams(*route), maxMsgSize)
		if err != nil {
			message.PutUpdateBuilder(ub)
			routesLogger().Debug("FlowSpec build error (too large?)", "peer", addr, "error", err)
			continue
		}
		sendErr := p.SendUpdate(update)
		message.PutUpdateBuilder(ub)
		if sendErr != nil {
			routesLogger().Debug("FlowSpec send error", "peer", addr, "error", sendErr)
			continue
		}
		sentCount++
	}
	if sentCount > 0 {
		routesLogger().Debug("sent FlowSpec routes", "peer", addr, "count", sentCount)
	}

	// Note: EOR for FlowSpec families is now sent by the main sendInitialRoutes loop
	// which iterates over all negotiated families using nc.Families().
}

// sendMUPRoutes sends MUP routes configured for this peer.
func (p *Peer) sendMUPRoutes() {
	nc := p.negotiated.Load()
	if nc == nil {
		return
	}

	addr := p.settings.Address.String()

	// Separate routes by AFI, filtering by negotiated families
	var ipv4Routes, ipv6Routes []MUPRoute
	var skippedIPv4, skippedIPv6 int

	for _, route := range p.settings.MUPRoutes {
		if route.IsIPv6 {
			if nc.Has(family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIMUP}) {
				ipv6Routes = append(ipv6Routes, route)
			} else {
				skippedIPv6++
			}
		} else {
			if nc.Has(family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMUP}) {
				ipv4Routes = append(ipv4Routes, route)
			} else {
				skippedIPv4++
			}
		}
	}

	if skippedIPv4 > 0 {
		routesLogger().Debug("skipping IPv4 MUP routes (not negotiated)", "peer", addr, "count", skippedIPv4)
	}
	if skippedIPv6 > 0 {
		routesLogger().Debug("skipping IPv6 MUP routes (not negotiated)", "peer", addr, "count", skippedIPv6)
	}

	// Send IPv4 MUP routes
	if len(ipv4Routes) > 0 {
		routesLogger().Debug("sending IPv4 MUP routes", "peer", addr, "count", len(ipv4Routes))
		ipv4MUPFamily := family.Family{AFI: 1, SAFI: 85}
		addPath := p.addPathFor(ipv4MUPFamily)
		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		defer message.PutUpdateBuilder(ub)
		for _, route := range ipv4Routes {
			update := ub.BuildMUP(toMUPParams(route))
			if err := p.SendUpdate(update); err != nil {
				routesLogger().Debug("MUP send error", "peer", addr, "error", err)
			}
		}
	}

	// Send IPv6 MUP routes
	if len(ipv6Routes) > 0 {
		routesLogger().Debug("sending IPv6 MUP routes", "peer", addr, "count", len(ipv6Routes))
		ipv6MUPFamily := family.Family{AFI: 2, SAFI: 85}
		addPath := p.addPathFor(ipv6MUPFamily)
		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		defer message.PutUpdateBuilder(ub)
		for _, route := range ipv6Routes {
			update := ub.BuildMUP(toMUPParams(route))
			if err := p.SendUpdate(update); err != nil {
				routesLogger().Debug("MUP send error", "peer", addr, "error", err)
			}
		}
	}
	// Note: EORs are sent by the generic loop in sendInitialRoutes() for ALL
	// negotiated families, so we don't send family-specific EORs here.
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
		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		params := message.UnicastParams{
			Prefix:  defaultPrefix,
			NextHop: nextHop,
			Origin:  attribute.OriginIGP,
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
	// Synthesize the update text the filter would see for this default route.
	// Format matches the ingress/egress policy text contract:
	//   "origin igp next-hop <ip> nlri <family> add <prefix>"
	updateText := fmt.Sprintf("origin igp next-hop %s nlri %s add %s",
		nextHop.String(), fam.String(), prefix.String())

	action, _ := PolicyFilterChain(
		[]string{filterName},
		"export",
		p.settings.Address.String(),
		p.settings.PeerAS,
		updateText,
		p.reactor.policyFilterFunc(nil), // nil payload -- synthetic update
	)
	return action != PolicyReject
}
