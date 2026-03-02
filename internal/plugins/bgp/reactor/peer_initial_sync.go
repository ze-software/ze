// Design: docs/architecture/core-design.md — initial route sending on BGP session establishment
// Related: peer.go — Peer struct and FSM state machine

package reactor

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// sendInitialRoutes sends static routes configured for this peer.
// Routes with identical attributes are grouped into a single UPDATE message.
// Uses atomic flag to prevent concurrent execution if session reconnects quickly.
func (p *Peer) sendInitialRoutes() {
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
				update := buildStaticRouteUpdateNew(&routes[0], nextHop, p.settings.LinkLocal, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath, p.sendCtx)
				if err := p.sendUpdateWithSplit(update, maxMsgSize, routeFamily(&routes[0])); err != nil {
					routesLogger().Debug("send error", "peer", addr, "error", err)
					break
				}
			} else {
				// Multi-route group - IPv4 unicast only (routeGroupKey ensures this)
				// Use size-aware builder to respect max message size
				ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
				params := make([]message.UnicastParams, 0, len(routes))
				for i := range routes {
					r := &routes[i]
					nextHop, nhErr := p.resolveNextHop(r.NextHop, routeFamily(r))
					if nhErr != nil {
						routesLogger().Debug("next-hop resolution failed", "peer", addr, "prefix", r.Prefix, "error", nhErr)
						continue
					}
					params = append(params, toStaticRouteUnicastParams(r, nextHop, p.settings.LinkLocal, p.sendCtx))
				}
				if len(params) == 0 {
					continue
				}
				updates, err := ub.BuildGroupedUnicastWithLimit(params, maxMsgSize)
				if err != nil {
					routesLogger().Debug("build error", "peer", addr, "error", err)
					break
				}
				for _, update := range updates {
					if err := p.SendUpdate(update); err != nil {
						routesLogger().Debug("send error", "peer", addr, "error", err)
						break
					}
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
			update := buildStaticRouteUpdateNew(route, nextHop, p.settings.LinkLocal, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath, p.sendCtx)
			if err := p.sendUpdateWithSplit(update, maxMsgSize, routeFamily(route)); err != nil {
				routesLogger().Debug("send error", "peer", addr, "error", err)
				break
			}
			routesLogger().Debug("route sent", "peer", addr, "prefix", route.Prefix.String(), "nextHop", route.NextHop.String())
		}
	}

	// Wait for API processes to send initial routes before processing queue.
	// Only delay if there are API processes that may send routes (SendUpdate permission).
	// This prevents unnecessary delay for tests without persist/route-injection APIs.
	p.mu.RLock()
	needsAPIWait := p.apiSyncExpected > 0
	p.mu.RUnlock()
	if needsAPIWait {
		routesLogger().Debug("sleeping for API routes", "peer", addr, "duration", "500ms")
		p.clock.Sleep(500 * time.Millisecond)
		routesLogger().Debug("woke from sleep, processing queue", "peer", addr)
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
			hasTeardown = true
			processed++

		case PeerOpAnnounce:
			// Send route, splitting if needed.
			family := op.Route.NLRI().Family()
			addPath := p.addPathFor(family)
			attrBuf := getBuildBuf()
			update := buildRIBRouteUpdate(attrBuf, op.Route, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
			p.mu.Unlock()
			sendErr := p.sendUpdateWithSplit(update, opMaxMsgSize, family)
			putBuildBuf(attrBuf)
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
			family := op.NLRI.Family()
			addPath := p.addPathFor(family)
			wdBuf := getBuildBuf()
			update := buildWithdrawNLRI(wdBuf, op.NLRI, addPath)
			p.mu.Unlock()
			sendErr := p.sendUpdateWithSplit(update, opMaxMsgSize, family)
			putBuildBuf(wdBuf)
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
	// Remove processed items and clear the sendingInitialRoutes flag atomically.
	// This ensures ShouldQueue() sees a consistent state: either the flag is set
	// (new routes will be queued and drained by our loop), or the flag is cleared
	// and the queue is empty (new routes can be sent directly).
	if processed > 0 {
		p.opQueue = p.opQueue[processed:]
	}
	if !hasTeardown {
		p.sendingInitialRoutes.Store(0)
	}
	p.mu.Unlock()

	if queueLen > 0 {
		routesLogger().Debug("processed queue ops", "peer", addr, "processed", processed, "remaining", len(p.opQueue), "teardown", hasTeardown)
	}

	// If teardown was in queue, send EOR first, then execute teardown.
	// EOR must be sent BEFORE NOTIFICATION per RFC 4724 Section 4.
	if hasTeardown {
		// Send EOR for ALL negotiated families before teardown
		for _, family := range nc.Families() {
			_ = p.SendUpdate(message.BuildEOR(family))
			routesLogger().Debug("sent EOR (before teardown)", "peer", addr, "family", family)
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
			if err := session.Teardown(teardownSubcode); err != nil {
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
	for _, family := range nc.Families() {
		_ = p.SendUpdate(message.BuildEOR(family))
		routesLogger().Debug("sent EOR", "peer", addr, "family", family)
	}
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
			if nc.Has(nlri.IPv6MVPN) {
				ipv6Routes = append(ipv6Routes, *route)
			} else {
				skippedIPv6++
			}
		} else {
			if nc.Has(nlri.IPv4MVPN) {
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
		ipv4MVPNFamily := nlri.Family{AFI: 1, SAFI: 5} // IPv4 MVPN
		addPath := p.addPathFor(ipv4MVPNFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		ipv4Groups := groupMVPNRoutesByKey(ipv4Routes)
		for _, key := range sortedKeys(ipv4Groups) {
			routes := ipv4Groups[key]
			// Use size-aware builder to respect max message size
			updates, err := ub.BuildMVPNWithLimit(toMVPNParams(routes), maxMsgSize)
			if err != nil {
				routesLogger().Debug("MVPN build error", "peer", addr, "error", err)
				continue
			}
			for _, update := range updates {
				if err := p.SendUpdate(update); err != nil {
					routesLogger().Debug("MVPN send error", "peer", addr, "error", err)
					break
				}
			}
			routesLogger().Debug("sent IPv4 MVPN routes", "peer", addr, "routes", len(routes), "updates", len(updates))
		}
	}

	// Send IPv6 MVPN routes grouped by attributes (sorted for deterministic order)
	if len(ipv6Routes) > 0 {
		ipv6MVPNFamily := nlri.Family{AFI: 2, SAFI: 5} // IPv6 MVPN
		addPath := p.addPathFor(ipv6MVPNFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		ipv6Groups := groupMVPNRoutesByKey(ipv6Routes)
		for _, key := range sortedKeys(ipv6Groups) {
			routes := ipv6Groups[key]
			// Use size-aware builder to respect max message size
			updates, err := ub.BuildMVPNWithLimit(toMVPNParams(routes), maxMsgSize)
			if err != nil {
				routesLogger().Debug("MVPN build error", "peer", addr, "error", err)
				continue
			}
			for _, update := range updates {
				if err := p.SendUpdate(update); err != nil {
					routesLogger().Debug("MVPN send error", "peer", addr, "error", err)
					break
				}
			}
			routesLogger().Debug("sent IPv6 MVPN routes", "peer", addr, "routes", len(routes), "updates", len(updates))
		}
	}
	// Note: EORs are sent by the generic loop in sendInitialRoutes() for ALL
	// negotiated families, so we don't send family-specific EORs here.
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
	if nc == nil || !nc.Has(nlri.L2VPNVPLS) {
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
		vplsFamily := nlri.Family{AFI: 25, SAFI: 65}
		addPath := p.addPathFor(vplsFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
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

		var family nlri.Family
		switch {
		case !isIPv6 && !isVPN:
			family = nlri.IPv4FlowSpec
		case !isIPv6 && isVPN:
			family = nlri.IPv4FlowSpecVPN
		case isIPv6 && !isVPN:
			family = nlri.IPv6FlowSpec
		case isIPv6 && isVPN:
			family = nlri.IPv6FlowSpecVPN
		}

		if !nc.Has(family) {
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
		addPath := p.addPathFor(nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)})
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
		// RFC 5575 Section 4: FlowSpec NLRI max 4095 bytes.
		// Single FlowSpec rule is atomic - cannot be split across UPDATEs.
		update, err := ub.BuildFlowSpecWithMaxSize(toFlowSpecParams(*route), maxMsgSize)
		if err != nil {
			routesLogger().Debug("FlowSpec build error (too large?)", "peer", addr, "error", err)
			continue
		}
		if err := p.SendUpdate(update); err != nil {
			routesLogger().Debug("FlowSpec send error", "peer", addr, "error", err)
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
			if nc.Has(nlri.IPv6MUP) {
				ipv6Routes = append(ipv6Routes, route)
			} else {
				skippedIPv6++
			}
		} else {
			if nc.Has(nlri.IPv4MUP) {
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
		ipv4MUPFamily := nlri.Family{AFI: 1, SAFI: 85}
		addPath := p.addPathFor(ipv4MUPFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
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
		ipv6MUPFamily := nlri.Family{AFI: 2, SAFI: 85}
		addPath := p.addPathFor(ipv6MUPFamily)
		ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath)
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
