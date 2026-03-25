// Design: docs/architecture/core-design.md — UPDATE forwarding, grouped sending, route refresh
// Overview: reactor_api.go — API command handling core
// Related: reactor_api_batch.go — NLRI batch operations
// Related: reactor_wire.go — zero-allocation wire UPDATE builders
// Related: forward_pool.go — per-peer forward worker pool used by ForwardUpdate
package reactor

import (
	"errors"
	"fmt"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"

	"codeberg.org/thomas-mangin/ze/internal/core/selector"
)

// AnnounceEOR sends an End-of-RIB marker for the given address family.
// Inlined peer iteration (not sendToMatchingPeers) to count EOR sent per peer.
func (a *reactorAPIAdapter) AnnounceEOR(peerSelector string, afi uint16, safi uint8) error {
	update := message.BuildEOR(nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)})

	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	var lastErr error
	sentCount := 0

	for addrStr, peer := range a.r.peers {
		if !ipGlobMatch(peerSelector, addrStr) {
			continue
		}
		if peer.State() != PeerStateEstablished {
			continue
		}
		if err := peer.SendUpdate(update); err != nil {
			lastErr = err
		} else {
			peer.IncrEORSent()
			sentCount++
		}
	}

	if sentCount == 0 && lastErr == nil {
		return errors.New("no established peers to send to")
	}

	return lastErr
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
		srcASN4 := srcCtx != nil && srcCtx.ASN4()

		if hasEBGPasn4 {
			var err error
			ebgpWireASN4, err = update.EBGPWire(ebgpLocalAS, srcASN4, true)
			if err != nil {
				fwdLogger().Warn("EBGP ASN4 wire rewrite failed", "id", updateID, "err", err)
			}
		}
		if hasEBGPasn2 {
			var err error
			ebgpWireASN2, err = update.EBGPWire(ebgpLocalAS, srcASN4, false)
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

	// Build source PeerFilterInfo once for egress filter chain.
	var srcFilter registry.PeerFilterInfo
	if len(a.r.egressFilters) > 0 {
		srcFilter = registry.PeerFilterInfo{Address: update.SourcePeerIP, PeerAS: 0}
		// Look up source peer's ASN from peers map (may have disconnected).
		a.r.mu.RLock()
		if srcPeer, ok := a.r.findPeerByAddr(update.SourcePeerIP); ok {
			srcFilter.PeerAS = srcPeer.Settings().PeerAS
		}
		a.r.mu.RUnlock()
	}

	// Source peer address for overflow ratio tracking (AC-16).
	// Hoisted outside the loop — loop-invariant, avoids N redundant String() allocations.
	srcAddr := update.SourcePeerIP.String()

	for _, peer := range matchingPeers {
		if peer.State() != PeerStateEstablished {
			continue // Skip non-established peers
		}

		// Egress peer filter chain: check if route should be sent to this peer.
		// mods accumulates per-peer modifications; fresh for each peer.
		var mods registry.ModAccumulator
		if len(a.r.egressFilters) > 0 {
			destFilter := registry.PeerFilterInfo{
				Address: peer.Settings().Address,
				PeerAS:  peer.Settings().PeerAS,
			}
			payload := update.WireUpdate.Payload()
			suppressed := false
			for _, filter := range a.r.egressFilters {
				if !safeEgressFilter(filter, srcFilter, destFilter, payload, update.Meta, &mods) {
					suppressed = true
					break
				}
			}
			if suppressed {
				continue // Route suppressed by egress filter for this peer.
			}
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

		// Apply accumulated modifications from egress filters.
		// Runs AFTER wire selection so mods apply to the correct wire version
		// (e.g., EBGP wire with AS-PATH prepended, not the original).
		// Handlers are post-accept transformations: they cannot reject, only transform.
		// Each handler receives the output of the previous, composing sequentially.
		if mods.Len() > 0 {
			payload := peerWire.Payload()
			modified := false
			mods.Range(func(key string, val any) {
				handler := a.r.modHandlers[key]
				if handler == nil {
					reactorLogger().Warn("no mod handler registered", "key", key)
					return
				}
				result := safeModHandler(handler, key, payload, val)
				if len(result) > 0 {
					payload = result
					modified = true
				}
			})
			if modified {
				peerWire = wireu.NewWireUpdate(payload, peerWire.SourceCtxID())
			}
		}

		// Build the fwdItem with pre-computed send operations for this peer.
		item := fwdItem{peer: peer, meta: update.Meta}

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
					// Split via parsed UPDATE using destination's ADD-PATH state.
					// RFC 7911: ADD-PATH is negotiated per AFI/SAFI, so determine
					// the UPDATE's dominant family and query that.
					destSendCtx := peer.SendContext()
					addPath := addPathForUpdate(destSendCtx, parsedUpdate)

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
		if a.r.fwdPool.TryDispatch(key, item) {
			a.r.fwdPool.RecordForwarded(srcAddr)
			dispatchedCount++
		} else if a.r.fwdPool.DispatchOverflow(key, item) {
			// Channel full — item buffered in overflow for deferred processing.
			a.r.fwdPool.RecordOverflowed(srcAddr)
			dispatchedCount++
		}
		// If DispatchOverflow returned false, pool is stopped — done() was
		// called immediately (releasing cache ref). Don't count as dispatched.
	}

	if dispatchedCount == 0 {
		return fmt.Errorf("no established peers to forward to")
	}

	return nil
}

// addPathForUpdate determines the ADD-PATH flag for splitting a parsed UPDATE.
// RFC 7911: ADD-PATH is negotiated per AFI/SAFI. UPDATEs contain either:
//   - IPv4 unicast NLRIs in the legacy NLRI field (no MP attributes)
//   - MP_REACH_NLRI/MP_UNREACH_NLRI for other families
//
// This extracts the dominant family and queries the destination's context.
func addPathForUpdate(ctx *bgpctx.EncodingContext, u *message.Update) bool {
	if ctx == nil {
		return false
	}

	// Check for MP_REACH_NLRI (type 14) to determine family.
	// Attribute format: [flags:1][type:1][len:1-2][AFI:2][SAFI:1]...
	if family, ok := message.ExtractMPFamily(u.PathAttributes); ok {
		return ctx.AddPathFor(family)
	}

	// No MP attributes — IPv4 unicast (legacy NLRI field).
	return ctx.AddPathFor(nlri.IPv4Unicast)
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
