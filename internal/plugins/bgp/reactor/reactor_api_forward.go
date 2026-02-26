// Design: docs/architecture/core-design.md — UPDATE forwarding, grouped sending, watchdog, route refresh
// Related: reactor_api.go — API command handling core
// Related: reactor_api_batch.go — NLRI batch operations
// Related: reactor_api_routes.go — family-specific route types
// Related: reactor_wire.go — zero-allocation wire UPDATE builders
package reactor

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wireu"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/rib"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/selector"
)

// AnnounceEOR sends an End-of-RIB marker for the given address family.
func (a *reactorAPIAdapter) AnnounceEOR(peerSelector string, afi uint16, safi uint8) error {
	update := message.BuildEOR(nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)})
	return a.sendToMatchingPeers(peerSelector, update)
}

// SendRefresh sends a normal ROUTE-REFRESH message to matching peers.
// RFC 2918 Section 3: "A BGP speaker may send a ROUTE-REFRESH message to
// its peer only if it has received the Route Refresh Capability from its peer.".
func (a *reactorAPIAdapter) SendRefresh(peerSelector string, afi uint16, safi uint8) error {
	return a.sendRouteRefresh(peerSelector, afi, safi, message.RouteRefreshNormal)
}

// SendBoRR sends a Beginning of Route Refresh marker to matching peers.
// RFC 7313 Section 4: "Before the speaker starts a route refresh...
// the speaker MUST send a BoRR message.".
func (a *reactorAPIAdapter) SendBoRR(peerSelector string, afi uint16, safi uint8) error {
	return a.sendRouteRefresh(peerSelector, afi, safi, message.RouteRefreshBoRR)
}

// SendEoRR sends an End of Route Refresh marker to matching peers.
// RFC 7313 Section 4: "After the speaker completes the re-advertisement
// of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message.".
func (a *reactorAPIAdapter) SendEoRR(peerSelector string, afi uint16, safi uint8) error {
	return a.sendRouteRefresh(peerSelector, afi, safi, message.RouteRefreshEoRR)
}

// sendRouteRefresh sends a ROUTE-REFRESH message with the specified subtype.
// RFC 2918 Section 3: "A BGP speaker that is willing to receive the
// ROUTE-REFRESH message from its peer SHOULD advertise the Route Refresh
// Capability to the peer using BGP Capabilities advertisement."
// RFC 2918 Section 4: "A BGP speaker may send a ROUTE-REFRESH message to
// its peer only if it has received the Route Refresh Capability from its peer."
//
// RFC 7313 Section 3.2 - Message Subtype values:
//   - 0: Normal Route Refresh (RFC 2918)
//   - 1: Beginning of Route Refresh (BoRR)
//   - 2: End of Route Refresh (EoRR)
//
// RFC 7313: "If peer did not advertise Enhanced Route Refresh Capability:
// Do NOT send BoRR or EoRR." Only subtype 0 is allowed without Enhanced RR.
func (a *reactorAPIAdapter) sendRouteRefresh(peerSelector string, afi uint16, safi uint8, subtype message.RouteRefreshSubtype) error {
	// RFC 7313: BoRR/EoRR require Enhanced Route Refresh capability
	requiresEnhancedRR := subtype == message.RouteRefreshBoRR || subtype == message.RouteRefreshEoRR

	rr := &message.RouteRefresh{
		AFI:     message.AFI(afi),
		SAFI:    message.SAFI(safi),
		Subtype: subtype,
	}

	// WriteTo includes the BGP header
	data := message.PackTo(rr, nil)

	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	var lastErr error
	for addrStr, peer := range a.r.peers {
		if !ipGlobMatch(peerSelector, addrStr) {
			continue
		}

		if peer.State() != PeerStateEstablished {
			continue
		}

		// RFC 7313: "If peer did not advertise Enhanced Route Refresh Capability:
		// Do NOT send BoRR or EoRR."
		if requiresEnhancedRR {
			neg := peer.negotiated.Load()
			if neg == nil || !neg.EnhancedRouteRefresh {
				continue // Skip peers without Enhanced Route Refresh
			}
		}

		// Send full packet (msgType=0 means data includes header)
		if err := peer.SendRawMessage(0, data); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// AnnounceWatchdog announces all routes in the named watchdog group.
// Routes are moved from withdrawn (-) to announced (+) state.
// Checks global pools first, then per-peer WatchdogGroups.
// Returns error only for send failures, not for missing groups.
func (a *reactorAPIAdapter) AnnounceWatchdog(peerSelector, name string) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return nil // No matching peers
	}

	// Check global pool first
	globalPool := a.r.watchdog.GetPool(name)
	if globalPool != nil {
		var lastErr error
		for _, peer := range peers {
			if peer.State() != PeerStateEstablished {
				continue
			}
			peerAddr := peer.Settings().Address.String()
			localAddr := peer.Settings().LocalAddress
			routes := a.r.watchdog.AnnouncePool(name, peerAddr)
			for _, pr := range routes {
				// RFC 4271 Section 4.3 - Send UPDATE (zero-allocation path)
				spec := staticRouteToSpec(&pr.StaticRoute, localAddr)
				if err := peer.SendAnnounce(spec, a.r.config.LocalAS); err != nil {
					lastErr = err
				}
			}
		}
		return lastErr
	}

	// Fall back to per-peer WatchdogGroups
	var lastErr error
	found := false
	for _, peer := range peers {
		err := peer.AnnounceWatchdog(name)
		if err != nil {
			if errors.Is(err, ErrWatchdogNotFound) {
				// This peer doesn't have the group - skip, try others
				continue
			}
			// Real error (send failure) - record but continue with other peers
			lastErr = err
		} else {
			found = true
		}
	}

	// If no peer had the group, return not found error
	if !found && lastErr == nil {
		return fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}
	return lastErr
}

// WithdrawWatchdog withdraws all routes in the named watchdog group.
// Routes are moved from announced (+) to withdrawn (-) state.
// Checks global pools first, then per-peer WatchdogGroups.
// Returns error only for send failures, not for missing groups.
func (a *reactorAPIAdapter) WithdrawWatchdog(peerSelector, name string) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return nil // No matching peers
	}

	// Check global pool first
	globalPool := a.r.watchdog.GetPool(name)
	if globalPool != nil {
		var lastErr error
		for _, peer := range peers {
			if peer.State() != PeerStateEstablished {
				continue
			}
			peerAddr := peer.Settings().Address.String()
			routes := a.r.watchdog.WithdrawPool(name, peerAddr)
			for _, pr := range routes {
				// RFC 4271 Section 4.3 - Send withdrawal UPDATE (zero-allocation path)
				if err := peer.SendWithdraw(pr.Prefix); err != nil {
					lastErr = err
				}
			}
		}
		return lastErr
	}

	// Fall back to per-peer WatchdogGroups
	var lastErr error
	found := false
	for _, peer := range peers {
		err := peer.WithdrawWatchdog(name)
		if err != nil {
			if errors.Is(err, ErrWatchdogNotFound) {
				// This peer doesn't have the group - skip, try others
				continue
			}
			// Real error (send failure) - record but continue with other peers
			lastErr = err
		} else {
			found = true
		}
	}

	// If no peer had the group, return not found error
	if !found && lastErr == nil {
		return fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}
	return lastErr
}

// AddWatchdogRoute adds a route to a global watchdog pool.
// Implements plugin.ReactorLifecycle + bgptypes.BGPReactor.
func (a *reactorAPIAdapter) AddWatchdogRoute(routeSpec bgptypes.RouteSpec, poolName string) error {
	// Convert bgptypes.RouteSpec to StaticRoute
	sr := StaticRoute{
		Prefix:  routeSpec.Prefix,
		NextHop: routeSpec.NextHop, // Already bgptypes.RouteNextHop
	}

	// Extract attributes from Wire (wire-first approach)
	if routeSpec.Wire != nil {
		// Extract ORIGIN
		if originAttr, err := routeSpec.Wire.Get(attribute.AttrOrigin); err == nil {
			if o, ok := originAttr.(attribute.Origin); ok {
				sr.Origin = uint8(o)
			}
		}
		// Extract LOCAL_PREF
		if lpAttr, err := routeSpec.Wire.Get(attribute.AttrLocalPref); err == nil {
			if lp, ok := lpAttr.(attribute.LocalPref); ok {
				sr.LocalPreference = uint32(lp)
			}
		}
		// Extract MED
		if medAttr, err := routeSpec.Wire.Get(attribute.AttrMED); err == nil {
			if m, ok := medAttr.(attribute.MED); ok {
				sr.MED = uint32(m)
			}
		}
		// Extract AS_PATH
		if asPathAttr, err := routeSpec.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				sr.ASPath = asp.Segments[0].ASNs
			}
		}
		// Extract COMMUNITY
		if commAttr, err := routeSpec.Wire.Get(attribute.AttrCommunity); err == nil {
			if comms, ok := commAttr.(attribute.Communities); ok {
				sr.Communities = make([]uint32, len(comms))
				for i, c := range comms {
					sr.Communities[i] = uint32(c)
				}
			}
		}
		// Extract LARGE_COMMUNITY
		if lcAttr, err := routeSpec.Wire.Get(attribute.AttrLargeCommunity); err == nil {
			if lcs, ok := lcAttr.(attribute.LargeCommunities); ok {
				sr.LargeCommunities = make([][3]uint32, len(lcs))
				for i, c := range lcs {
					sr.LargeCommunities[i] = [3]uint32{c.GlobalAdmin, c.LocalData1, c.LocalData2}
				}
			}
		}
	}

	return a.r.AddWatchdogRoute(&sr, poolName)
}

// RemoveWatchdogRoute removes a route from a global watchdog pool.
// Implements plugin.ReactorLifecycle + bgptypes.BGPReactor.
func (a *reactorAPIAdapter) RemoveWatchdogRoute(routeKey, poolName string) error {
	return a.r.RemoveWatchdogRoute(routeKey, poolName)
}

// staticRouteToSpec converts a StaticRoute to bgptypes.RouteSpec.
// localAddress is used to resolve "next-hop self" routes.
func staticRouteToSpec(sr *StaticRoute, localAddress netip.Addr) bgptypes.RouteSpec {
	// Resolve next-hop from RouteNextHop policy
	var nextHop netip.Addr
	if sr.NextHop.IsSelf() && localAddress.IsValid() {
		nextHop = localAddress
	} else if sr.NextHop.IsExplicit() {
		nextHop = sr.NextHop.Addr
	}
	// If neither, nextHop remains zero value (invalid)

	spec := bgptypes.RouteSpec{
		Prefix:  sr.Prefix,
		NextHop: bgptypes.NewNextHopExplicit(nextHop),
	}

	// Build wire-format attributes using Builder (wire-first approach)
	b := attribute.NewBuilder()

	// Origin (0=IGP by default)
	b.SetOrigin(sr.Origin)

	// LocalPreference
	if sr.LocalPreference != 0 {
		b.SetLocalPref(sr.LocalPreference)
	}

	// MED
	if sr.MED != 0 {
		b.SetMED(sr.MED)
	}

	// ASPath
	if len(sr.ASPath) > 0 {
		b.SetASPath(sr.ASPath)
	}

	// Communities
	for _, c := range sr.Communities {
		b.AddCommunityValue(c)
	}

	// LargeCommunities
	for _, lc := range sr.LargeCommunities {
		b.AddLargeCommunity(lc[0], lc[1], lc[2])
	}

	// Build wire bytes and wrap
	wireBytes := b.Build()
	if len(wireBytes) > 0 {
		spec.Wire = attribute.NewAttributesWire(wireBytes, bgpctx.APIContextID)
	}

	return spec
}

// ForwardUpdate forwards a cached UPDATE to peers matching the selector.
// Looks up the update by ID from the cache and sends to matching peers.
//
// If pluginName is non-empty (cache consumer), records plugin ack after forwarding.
// Non-cache-consumer callers can still forward but don't participate in ack tracking.
//
// RFC 4271 §9.1.2 compliance: For EBGP peers, the local AS is prepended to
// AS_PATH in the wire bytes before forwarding. IBGP peers receive the original
// bytes unchanged. EBGP wire versions are lazily cached per ASN4/ASN2 variant.
//
// Zero-copy optimization: When source and destination encoding contexts match
// (same ASN4, ADD-PATH capabilities), the raw UPDATE bytes are forwarded
// directly without re-encoding.
//
// RFC 8654 compliance: If the UPDATE exceeds a peer's max message size
// (4096 without Extended Message, 65535 with), it is split into multiple
// smaller UPDATEs that each fit within the limit.
func (a *reactorAPIAdapter) ForwardUpdate(sel *selector.Selector, updateID uint64, pluginName string) error {
	// Get read-only access to cached update (non-destructive)
	// Cache retains buffer ownership; Ack() when done to record plugin acknowledgment
	update, ok := a.r.recentUpdates.Get(updateID)
	if !ok {
		return ErrUpdateExpired
	}
	// Ack the entry after forwarding if caller is a cache consumer.
	// Deferred so ack happens even on partial forwarding failures.
	// Non-cache-consumer callers (pluginName == "") skip the ack — they
	// are not tracked in the consumer set and must not pollute pluginLastAck.
	if pluginName != "" {
		defer func() {
			if ackErr := a.r.recentUpdates.Ack(updateID, pluginName); ackErr != nil {
				cacheLogger().Warn("cache ack after forward failed",
					"id", updateID, "plugin", pluginName, "err", ackErr)
			}
		}()
	}

	// Get matching peers
	a.r.mu.RLock()
	var matchingPeers []*Peer
	for _, peer := range a.r.peers {
		addr := peer.Settings().Address
		if sel.Matches(addr) && addr != update.SourcePeerIP {
			// Don't forward back to source peer (implicit loop prevention)
			matchingPeers = append(matchingPeers, peer)
		}
	}
	a.r.mu.RUnlock()

	if len(matchingPeers) == 0 {
		return fmt.Errorf("no peers match selector %s", sel)
	}

	// EBGP preparation: scan for EBGP peers and pre-generate patched wires.
	// RFC 4271 §9.1.2: EBGP speakers MUST prepend their own AS to AS_PATH.
	// RFC 6793 §4: ASN4→ASN2 transcoding uses AS_TRANS=23456.
	var ebgpWireASN4, ebgpWireASN2 *wireu.WireUpdate
	var hasEBGPasn4, hasEBGPasn2 bool
	var ebgpLocalAS uint32
	for _, peer := range matchingPeers {
		if peer.State() != PeerStateEstablished {
			continue
		}
		if peer.Settings().IsEBGP() {
			ebgpLocalAS = peer.Settings().LocalAS
			if peer.asn4() {
				hasEBGPasn4 = true
			} else {
				hasEBGPasn2 = true
			}
		}
	}
	if hasEBGPasn4 || hasEBGPasn2 {
		srcCtxID := update.WireUpdate.SourceCtxID()
		srcCtx := bgpctx.Registry.Get(srcCtxID)
		srcAsn4 := srcCtx != nil && srcCtx.ASN4()

		if hasEBGPasn4 {
			var err error
			ebgpWireASN4, err = update.EBGPWire(ebgpLocalAS, srcAsn4, true)
			if err != nil {
				fwdLogger().Warn("EBGP ASN4 wire rewrite failed", "id", updateID, "err", err)
			}
		}
		if hasEBGPasn2 {
			var err error
			ebgpWireASN2, err = update.EBGPWire(ebgpLocalAS, srcAsn4, false)
			if err != nil {
				fwdLogger().Warn("EBGP ASN2 wire rewrite failed", "id", updateID, "err", err)
			}
		}
	}

	// Pre-compute send operations per peer, then dispatch to pool.
	// CPU work (split/context comparison/lazy parsing) is fast and done here.
	// TCP writes happen asynchronously in per-peer workers.
	var parsedUpdate *message.Update
	var parsedWire *wireu.WireUpdate
	var dispatchedCount int

	for _, peer := range matchingPeers {
		if peer.State() != PeerStateEstablished {
			continue // Skip non-established peers
		}

		// Select wire version for this peer.
		// RFC 4271 §9.1.2: EBGP peers get AS-PATH-prepended wire.
		// IBGP peers get original wire unchanged.
		peerWire := update.WireUpdate
		if peer.Settings().IsEBGP() {
			if peer.asn4() && ebgpWireASN4 != nil {
				peerWire = ebgpWireASN4
			} else if !peer.asn4() && ebgpWireASN2 != nil {
				peerWire = ebgpWireASN2
			}
			// If EBGP wire generation failed, peerWire stays as original (graceful degradation)
		}

		// Build the fwdItem with pre-computed send operations for this peer.
		item := fwdItem{peer: peer}

		// Get max message size for this peer (RFC 8654)
		nc := peer.negotiated.Load()
		extendedMessage := nc != nil && nc.ExtendedMessage
		maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, extendedMessage))

		// Calculate update size for this peer's wire version (header + body)
		updateSize := message.HeaderLen + len(peerWire.Payload())

		// Check if UPDATE exceeds peer's max message size
		if updateSize > maxMsgSize {
			// Wire-level split: get source context for per-family ADD-PATH lookup
			srcCtxID := peerWire.SourceCtxID()
			srcCtx := bgpctx.Registry.Get(srcCtxID) // May be nil if not registered

			maxBodySize := maxMsgSize - message.HeaderLen
			splits, err := wireu.SplitWireUpdate(peerWire, maxBodySize, srcCtx)
			if err != nil {
				fwdLogger().Warn("forward split failed",
					"peer", peer.Settings().Address,
					"err", err,
				)
				continue
			}
			for _, split := range splits {
				item.rawBodies = append(item.rawBodies, split.Payload())
			}
		} else {
			// Normal path: UPDATE fits within peer's message limit
			destCtxID := peer.SendContextID()

			// Zero-copy path: use raw bytes when contexts match
			// Both must be non-zero (registered) and equal
			srcCtxID := peerWire.SourceCtxID()
			if srcCtxID != 0 && destCtxID != 0 && srcCtxID == destCtxID {
				item.rawBodies = append(item.rawBodies, peerWire.Payload())
			} else {
				// Re-encode path: parse (lazily) and send.
				// Reset cached parse if wire version changed (IBGP vs EBGP use different payloads).
				if parsedUpdate == nil || parsedWire != peerWire {
					var parseErr error
					parsedUpdate, parseErr = message.UnpackUpdate(peerWire.Payload())
					if parseErr != nil {
						return fmt.Errorf("parsing cached update: %w", parseErr)
					}
					parsedWire = peerWire
				}

				// Check repacked size - may differ from original due to ASN4 encoding changes
				// Size = Header(19) + WithdrawnLen(2) + Withdrawn + AttrLen(2) + Attrs + NLRI
				repackedSize := message.HeaderLen + 4 + len(parsedUpdate.WithdrawnRoutes) +
					len(parsedUpdate.PathAttributes) + len(parsedUpdate.NLRI)
				if repackedSize > maxMsgSize {
					// Split via parsed UPDATE using destination's ADD-PATH state
					// TODO: SplitUpdateWithAddPath uses single addPath for all families.
					// For mixed-family UPDATEs, this may be incorrect. Consider updating
					// SplitUpdateWithAddPath to accept EncodingContext in future.
					destSendCtx := peer.SendContext()
					addPath := destSendCtx != nil && destSendCtx.AddPathFor(nlri.IPv4Unicast)

					chunks, splitErr := message.SplitUpdateWithAddPath(parsedUpdate, maxMsgSize, addPath)
					if splitErr != nil {
						fwdLogger().Warn("forward split failed",
							"peer", peer.Settings().Address,
							"err", splitErr,
						)
						continue
					}
					item.updates = append(item.updates, chunks...)
				} else {
					item.updates = append(item.updates, parsedUpdate)
				}
			}
		}

		// Retain cache buffer for this peer's worker. Released by done callback
		// after worker completes all send ops. Ack (deferred above) fires when
		// ForwardUpdate returns — before workers finish — but retainCount keeps
		// the buffer alive until all workers call Release.
		a.r.recentUpdates.Retain(updateID)
		item.done = func() { a.r.recentUpdates.Release(updateID) }

		key := fwdKey{peerAddr: peer.Settings().Address.String()}
		if !a.r.fwdPool.Dispatch(key, item) {
			// Pool stopped — release cache ref ourselves
			a.r.recentUpdates.Release(updateID)
			continue
		}
		dispatchedCount++
	}

	if dispatchedCount == 0 {
		return fmt.Errorf("no established peers to forward to")
	}

	return nil
}

// sendRoutesWithLimit sends routes in batches that fit within maxMsgSize.
//
// When GroupUpdates is enabled (default), routes with identical attributes are
// grouped into single UPDATE messages. This reduces UPDATE count from O(routes)
// to O(routes/capacity), dramatically improving efficiency for large route sets.
//
// When GroupUpdates is disabled, routes are sent individually (legacy behavior).
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) sendRoutesWithLimit(peer *Peer, routes []*rib.Route, maxMsgSize int) error {
	if len(routes) == 0 {
		return nil
	}

	// Fall back to individual sending if grouping disabled
	if !peer.settings.GroupUpdates {
		return a.sendRoutesIndividually(peer, routes, maxMsgSize)
	}

	// Group routes by attributes + AS_PATH
	attrGroups := rib.GroupByAttributesTwoLevel(routes)

	var errs []error
	for _, attrGroup := range attrGroups {
		for _, aspGroup := range attrGroup.ByASPath {
			if err := a.sendASPathGroup(peer, &attrGroup, &aspGroup, maxMsgSize); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// sendRoutesIndividually sends routes one at a time (legacy behavior).
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) sendRoutesIndividually(peer *Peer, routes []*rib.Route, maxMsgSize int) error {
	var errs []error

	for _, r := range routes {
		family := r.NLRI().Family()
		addPath := peer.addPathFor(family)
		asn4 := peer.asn4()
		attrBuf := getBuildBuf()
		update := buildRIBRouteUpdate(attrBuf, r, peer.settings.LocalAS, peer.settings.IsIBGP(), asn4, addPath)
		sendErr := peer.sendUpdateWithSplit(update, maxMsgSize, family)
		putBuildBuf(attrBuf)
		if sendErr != nil {
			errs = append(errs, fmt.Errorf("route %s: %w", r.NLRI(), sendErr))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// sendASPathGroup sends routes in an AS_PATH group as efficiently as possible.
// For IPv4 unicast: uses BuildGroupedUnicastWithLimit to pack multiple NLRIs.
// For MP families: builds UPDATE with MP_REACH_NLRI containing grouped NLRIs.
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) sendASPathGroup(peer *Peer, attrGroup *rib.AttributeGroup, aspGroup *rib.ASPathGroup, maxMsgSize int) error {
	if len(aspGroup.Routes) == 0 {
		return nil
	}

	family := attrGroup.Family
	addPath := peer.addPathFor(family)
	asn4 := peer.asn4()

	// IPv4 unicast: use BuildGroupedUnicastWithLimit
	if family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast {
		return a.sendGroupedIPv4Unicast(peer, aspGroup.Routes, asn4, addPath, maxMsgSize)
	}

	// MP families: build UPDATE with MP_REACH_NLRI containing grouped NLRIs
	return a.sendGroupedMPFamily(peer, aspGroup.Routes, family, asn4, addPath, maxMsgSize)
}

// sendGroupedIPv4Unicast sends grouped IPv4 unicast routes using BuildGroupedUnicastWithLimit.
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) sendGroupedIPv4Unicast(peer *Peer, routes []*rib.Route, asn4, addPath bool, maxMsgSize int) error {
	// Check if any route has complex AS_PATH (AS_SET, CONFED, multiple segments)
	// that can't be represented in UnicastParams.ASPath (which is just []uint32).
	// Fall back to individual sending for such routes.
	if slices.ContainsFunc(routes, hasComplexASPath) {
		return a.sendRoutesIndividually(peer, routes, maxMsgSize)
	}

	// Convert to UnicastParams
	params := make([]message.UnicastParams, len(routes))
	for i, r := range routes {
		params[i] = toRIBRouteUnicastParams(r)
	}

	// Build grouped UPDATEs respecting size limits
	ub := message.NewUpdateBuilder(peer.settings.LocalAS, peer.settings.IsIBGP(), asn4, addPath)
	updates, err := ub.BuildGroupedUnicastWithLimit(params, maxMsgSize)
	if err != nil {
		return fmt.Errorf("building grouped IPv4 unicast: %w", err)
	}

	// Send all UPDATEs
	var errs []error
	for _, update := range updates {
		if err := peer.SendUpdate(update); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// hasComplexASPath returns true if the route's AS_PATH can't be represented
// as a simple []uint32 (has AS_SET, CONFED segments, or multiple segments).
func hasComplexASPath(r *rib.Route) bool {
	asPath := r.ASPath()
	if asPath == nil || len(asPath.Segments) == 0 {
		return false
	}

	// Multiple segments = complex
	if len(asPath.Segments) > 1 {
		return true
	}

	// Single segment: only AS_SEQUENCE is simple
	seg := asPath.Segments[0]
	return seg.Type != attribute.ASSequence
}

// sendGroupedMPFamily sends grouped MP family routes (IPv6, VPN, etc.).
// Writes multiple NLRIs into MP_REACH_NLRI attribute.
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) sendGroupedMPFamily(peer *Peer, routes []*rib.Route, family nlri.Family, asn4, addPath bool, maxMsgSize int) error {
	if len(routes) == 0 {
		return nil
	}

	// Write all NLRIs into pooled buffer
	nlriBuf := getBuildBuf()
	defer putBuildBuf(nlriBuf)
	off := 0
	for _, r := range routes {
		off += nlri.WriteNLRI(r.NLRI(), nlriBuf, off, addPath)
	}
	nlriBytes := nlriBuf[:off]

	// Build grouped UPDATE with all NLRIs
	firstRoute := routes[0]
	attrBuf := getBuildBuf()
	defer putBuildBuf(attrBuf)
	groupedUpdate := a.buildGroupedMPUpdate(attrBuf, firstRoute, nlriBytes, family, peer.settings.LocalAS, peer.settings.IsIBGP(), asn4)

	// Check actual size of grouped update
	msgSize := message.HeaderLen + 4 + len(groupedUpdate.PathAttributes)
	if msgSize <= maxMsgSize {
		return peer.SendUpdate(groupedUpdate)
	}

	// Need to split - calculate available space for NLRI
	// MP_REACH_NLRI overhead: header(3-4) + AFI(2) + SAFI(1) + NH-len(1) + NH + Reserved(1)
	// Next-hop sizes: IPv4=4, IPv6=16 or 32 (global+link-local), VPN=12 or 24
	nhLen := nextHopLength(family, firstRoute.NextHop())
	mpReachOverhead := 4 + 2 + 1 + 1 + nhLen + 1 // extended header + AFI + SAFI + NH-len + NH + reserved

	// Base attributes (without MP_REACH_NLRI's NLRI portion)
	baseAttrSize := len(groupedUpdate.PathAttributes) - len(nlriBytes)
	availableNLRISpace := maxMsgSize - message.HeaderLen - 4 - baseAttrSize - mpReachOverhead

	if availableNLRISpace <= 0 {
		return fmt.Errorf("attributes too large for MP family: %d bytes, max %d", baseAttrSize+mpReachOverhead, maxMsgSize-message.HeaderLen-4)
	}

	// Split NLRIs into chunks
	chunks, err := message.ChunkMPNLRI(nlriBytes, family.AFI, family.SAFI, addPath, availableNLRISpace)
	if err != nil {
		return fmt.Errorf("chunking MP NLRI: %w", err)
	}

	var errs []error
	for _, chunk := range chunks {
		chunkUpdate := a.buildGroupedMPUpdate(attrBuf, firstRoute, chunk, family, peer.settings.LocalAS, peer.settings.IsIBGP(), asn4)
		if err := peer.SendUpdate(chunkUpdate); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// nextHopLength returns the wire length of next-hop for a given family.
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func nextHopLength(family nlri.Family, nh netip.Addr) int {
	switch {
	case family.AFI == nlri.AFIIPv4:
		return 4
	case family.AFI == nlri.AFIIPv6:
		// Could be 16 (global only) or 32 (global + link-local)
		// Conservative: assume 32 for safety
		return 32
	case family.SAFI == nlri.SAFIVPN:
		// VPN: RD (8) + address (4 or 16)
		if family.AFI == nlri.AFIIPv4 {
			return 12 // RD + IPv4
		}
		return 24 // RD + IPv6
	default: // conservative default
		if nh.Is6() {
			return 32
		}
		return 4
	}
}

// buildGroupedMPUpdate builds an UPDATE with MP_REACH_NLRI containing multiple NLRIs.
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) buildGroupedMPUpdate(attrBuf []byte, templateRoute *rib.Route, nlriBytes []byte, family nlri.Family, localAS uint32, isIBGP, asn4 bool) *message.Update {
	off := 0

	// Create encoding context for ASPath encoding
	dstCtx := bgpctx.EncodingContextForASN4(asn4)

	// 1. ORIGIN
	origin := attribute.OriginIGP
	for _, attr := range templateRoute.Attributes() {
		if o, ok := attr.(attribute.Origin); ok {
			origin = o
			break
		}
	}
	off += attribute.WriteAttrTo(origin, attrBuf, off)

	// 2. AS_PATH
	storedASPath := templateRoute.ASPath()
	hasStoredASPath := storedASPath != nil && len(storedASPath.Segments) > 0

	var asPath *attribute.ASPath
	switch {
	case hasStoredASPath:
		asPath = storedASPath
	case isIBGP || localAS == 0:
		asPath = &attribute.ASPath{Segments: nil}
	default: // eBGP: prepend local AS
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{{
				Type: attribute.ASSequence,
				ASNs: []uint32{localAS},
			}},
		}
	}
	off += attribute.WriteAttrToWithContext(asPath, attrBuf, off, nil, dstCtx)

	// MP_REACH_NLRI with grouped NLRIs
	mpReach := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(family.AFI),
		SAFI:     attribute.SAFI(family.SAFI),
		NextHops: []netip.Addr{templateRoute.NextHop()},
		NLRI:     nlriBytes,
	}
	off += attribute.WriteAttrTo(mpReach, attrBuf, off)

	// LOCAL_PREF for iBGP
	if isIBGP {
		off += attribute.WriteAttrTo(attribute.LocalPref(100), attrBuf, off)
	}

	// Copy optional attributes
	for _, attr := range templateRoute.Attributes() {
		switch attr.(type) {
		case attribute.Origin, *attribute.ASPath, *attribute.NextHop, attribute.LocalPref:
			continue
		case attribute.MED, attribute.Communities,
			attribute.ExtendedCommunities, attribute.LargeCommunities,
			attribute.IPv6ExtendedCommunities,
			attribute.AtomicAggregate, *attribute.Aggregator,
			attribute.OriginatorID, attribute.ClusterList:
			off += attribute.WriteAttrTo(attr, attrBuf, off)
		}
	}

	return &message.Update{
		PathAttributes: attrBuf[:off],
	}
}

// toRIBRouteUnicastParams converts a RIB route to UnicastParams for grouped building.
// Extracts attributes from the route's attribute slice for use with BuildGroupedUnicastWithLimit.
func toRIBRouteUnicastParams(r *rib.Route) message.UnicastParams {
	params := message.UnicastParams{
		NextHop: r.NextHop(),
		Origin:  attribute.OriginIGP, // Default
	}

	// Extract prefix and path-id from NLRI
	if n := r.NLRI(); n != nil {
		if inet, ok := n.(*nlri.INET); ok {
			params.Prefix = inet.Prefix()
			params.PathID = inet.PathID()
		}
	}

	// Extract AS_PATH if present
	if asPath := r.ASPath(); asPath != nil {
		for _, seg := range asPath.Segments {
			if seg.Type == attribute.ASSequence {
				params.ASPath = append(params.ASPath, seg.ASNs...)
			}
		}
	}

	// Extract attributes from the route's attribute slice
	for _, attr := range r.Attributes() {
		switch a := attr.(type) {
		case attribute.Origin:
			params.Origin = a
		case attribute.MED:
			params.MED = uint32(a)
		case attribute.LocalPref:
			params.LocalPreference = uint32(a)
		case attribute.Communities:
			params.Communities = make([]uint32, len(a))
			for i, c := range a {
				params.Communities[i] = uint32(c)
			}
		case attribute.ExtendedCommunities:
			buf := make([]byte, a.Len())
			a.WriteTo(buf, 0)
			params.ExtCommunityBytes = buf
		case attribute.LargeCommunities:
			params.LargeCommunities = make([][3]uint32, len(a))
			for i, lc := range a {
				params.LargeCommunities[i] = [3]uint32{lc.GlobalAdmin, lc.LocalData1, lc.LocalData2}
			}
		case attribute.AtomicAggregate:
			params.AtomicAggregate = true
		case *attribute.Aggregator:
			params.HasAggregator = true
			params.AggregatorASN = a.ASN
			if a.Address.Is4() {
				params.AggregatorIP = a.Address.As4()
			}
		case attribute.OriginatorID:
			if addr := netip.Addr(a); addr.Is4() {
				ip4 := addr.As4()
				params.OriginatorID = uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
			}
		case attribute.ClusterList:
			params.ClusterList = make([]uint32, len(a))
			copy(params.ClusterList, a)
		}
	}

	return params
}

// sendWithdrawalsWithLimit sends withdrawals using SplitUpdate for size limiting.
// Groups withdrawals by family to ensure correct Add-Path detection for each.
// Uses the same splitting infrastructure as announcements for consistency.
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) sendWithdrawalsWithLimit(peer *Peer, withdraws []nlri.NLRI, maxMsgSize int) error {
	if len(withdraws) == 0 {
		return nil
	}

	// Group withdrawals by family for correct Add-Path detection
	// BGP spec requires same-family NLRIs in each UPDATE, and Add-Path is per-family
	byFamily := make(map[nlri.Family][]byte)
	for _, n := range withdraws {
		family := n.Family()
		byFamily[family] = append(byFamily[family], n.Bytes()...)
	}

	var errs []error
	for family, withdrawnBytes := range byFamily {
		// Build withdrawal-only UPDATE for this family
		update := &message.Update{
			WithdrawnRoutes: withdrawnBytes,
		}

		// Use sendUpdateWithSplit for consistent splitting and Add-Path handling
		if err := peer.sendUpdateWithSplit(update, maxMsgSize, family); err != nil {
			errs = append(errs, fmt.Errorf("sending %s withdrawals: %w", family, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// DeleteUpdate removes an update from the cache without forwarding.
// Used when controller decides not to forward (filtering).
func (a *reactorAPIAdapter) DeleteUpdate(updateID uint64) error {
	if !a.r.recentUpdates.Delete(updateID) {
		return ErrUpdateExpired
	}
	return nil
}

// RetainUpdate prevents eviction of a cached UPDATE.
// Used by API for graceful restart - retain routes for replay.
func (a *reactorAPIAdapter) RetainUpdate(updateID uint64) error {
	if !a.r.recentUpdates.Retain(updateID) {
		return ErrUpdateExpired
	}
	return nil
}

// ReleaseUpdate handles cache release with two paths based on caller identity.
// Cache consumer (pluginName non-empty): acks the entry (FIFO validated),
// decrementing the pending consumer count. Does NOT decrement retain count.
// Non-consumer (pluginName empty): decrements API-level retain count only.
func (a *reactorAPIAdapter) ReleaseUpdate(updateID uint64, pluginName string) error {
	// If called by a plugin, ack the entry (decrements pending consumer count, FIFO validated).
	if pluginName != "" {
		if err := a.r.recentUpdates.Ack(updateID, pluginName); err != nil {
			return err
		}
		return nil
	}
	// Non-plugin caller: just decrement retain count
	if !a.r.recentUpdates.Release(updateID) {
		return ErrUpdateExpired
	}
	return nil
}

// ListUpdates returns all cached msg-ids (retained or non-expired).
func (a *reactorAPIAdapter) ListUpdates() []uint64 {
	return a.r.recentUpdates.List()
}

// RegisterCacheConsumer initializes tracking for a cache-consumer plugin.
// unordered=false: FIFO consumer (cumulative ack — existing behavior).
// unordered=true: per-entry ack only, no cumulative sweep. Required for
// consumers like bgp-rs that process entries out of global message ID order.
func (a *reactorAPIAdapter) RegisterCacheConsumer(name string, unordered bool) {
	a.r.recentUpdates.RegisterConsumer(name)
	if unordered {
		a.r.recentUpdates.SetConsumerUnordered(name)
	}
}

// UnregisterCacheConsumer removes a cache-consumer plugin and adjusts pending counts.
func (a *reactorAPIAdapter) UnregisterCacheConsumer(name string) {
	a.r.recentUpdates.UnregisterConsumer(name)
}
