// Design: docs/architecture/core-design.md — UPDATE forwarding, grouped sending, route refresh
// Design: .claude/rules/design-principles.md — zero-copy, copy-on-modify (shares Incoming Peer Pool buffer across peers)
// Overview: reactor_api.go — API command handling core
// Related: reactor_api_batch.go — NLRI batch operations
// Related: reactor_wire.go — zero-allocation wire UPDATE builders
// Related: forward_pool.go — per-peer forward worker pool used by ForwardUpdate
// Related: update_group.go — cross-peer UPDATE grouping index
// Detail: forward_build.go — progressive build for egress attribute modification
package reactor

import (
	"encoding/binary"
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/selector"
)

// AnnounceEOR sends an End-of-RIB marker for the given address family.
// Inlined peer iteration (not sendToMatchingPeers) to count EOR sent per peer.
func (a *reactorAPIAdapter) AnnounceEOR(peerSelector string, afi uint16, safi uint8) error {
	update := message.BuildEOR(family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)})

	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	var errs []error
	sentCount := 0

	for addrPort, peer := range a.r.peers {
		if !ipGlobMatch(peerSelector, addrPort.Addr().String()) {
			continue
		}
		if peer.State() != PeerStateEstablished {
			continue
		}
		if err := peer.SendUpdate(update); err != nil {
			errs = append(errs, err)
		} else {
			peer.IncrEORSent()
			sentCount++
		}
	}

	if sentCount == 0 && len(errs) == 0 {
		return errors.New("no established peers to send to")
	}

	return errors.Join(errs...)
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

	var errs []error
	for addrPort, peer := range a.r.peers {
		if !ipGlobMatch(peerSelector, addrPort.Addr().String()) {
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
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
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

	// Get matching peers. Stack array avoids heap allocation for <= 16 peers.
	// Also determine source peer type for route reflection (RFC 4456).
	a.r.mu.RLock()
	var peersBuf [16]*Peer
	matchingPeers := peersBuf[:0]
	var srcIsRRClient, srcIsIBGP bool
	var srcRemoteRouterID uint32 // Source peer's BGP Identifier (for ORIGINATOR_ID)
	for _, peer := range a.r.peers {
		addr := peer.Settings().Address
		if addr == update.SourcePeerIP {
			// Record source peer type for RR forwarding decisions.
			srcIsIBGP = peer.Settings().IsIBGP()
			srcIsRRClient = peer.Settings().RouteReflectorClient
			srcRemoteRouterID = peer.RemoteRouterID()
			continue // Don't forward back to source peer (implicit loop prevention)
		}
		if sel.Matches(addr) {
			matchingPeers = append(matchingPeers, peer)
		}
	}
	a.r.mu.RUnlock()

	if len(matchingPeers) == 0 {
		return fmt.Errorf("no peers match selector %s", sel)
	}

	// EBGP preparation: lazily generate patched wires keyed by (localAS, secondaryAS, asn4).
	// RFC 4271 §9.1.2: EBGP speakers MUST prepend their own AS to AS_PATH.
	// RFC 6793 §4: ASN4->ASN2 transcoding uses AS_TRANS=23456.
	//
	// LocalAS can differ per peer (RFC 7705 local-as override), so wire variants
	// are cached per (localAS, secondaryAS, dstASN4) combination rather than assuming
	// a single LocalAS for all EBGP peers.
	//
	// secondaryAS != 0 enables dual-AS prepend: the peer sees AS_PATH starting
	// with localAS (the override it expects) followed by secondaryAS (the router's
	// real global AS). This is the default behavior when a peer has a local-as
	// override and neither no-prepend nor replace-as modifier is set.
	//
	// The first (localAS, 0) per dstASN4 variant uses ReceivedUpdate.EBGPWire
	// (which caches in the ReceivedUpdate for reuse across ForwardUpdate calls).
	// Additional keys are generated directly via wireu.RewriteASPath /
	// RewriteASPathDual, since ReceivedUpdate's cache is keyed by dstASN4 only
	// and cannot hold dual-prepended variants.
	type ebgpWireKey struct {
		localAS     uint32
		secondaryAS uint32 // 0 = single prepend; non-zero = dual prepend behind localAS
		asn4        bool
	}
	type ebgpWireEntry struct {
		wire   *wireu.WireUpdate
		failed bool
	}
	var ebgpWireCache map[ebgpWireKey]*ebgpWireEntry
	var srcASN4 bool // computed once if any EBGP peer exists
	var srcASN4Set bool
	// Track the first localAS used per dstASN4 variant for ReceivedUpdate cache.
	// Only single-prepend keys (secondaryAS == 0) are eligible.
	var cachedLocalASN4, cachedLocalASN2 uint32
	var cachedLocalASN4Set, cachedLocalASN2Set bool

	// getEBGPWire returns the cached EBGP wire for the given
	// (localAS, secondaryAS, asn4) combination, generating it lazily on first access.
	// secondaryAS == 0 means single prepend; non-zero enables dual-AS prepend where
	// localAS ends up closest to the peer and secondaryAS sits behind it.
	getEBGPWire := func(localAS, secondaryAS uint32, asn4 bool) (*wireu.WireUpdate, bool) {
		ek := ebgpWireKey{localAS: localAS, secondaryAS: secondaryAS, asn4: asn4}
		if ebgpWireCache == nil {
			ebgpWireCache = make(map[ebgpWireKey]*ebgpWireEntry)
		}
		if entry, ok := ebgpWireCache[ek]; ok {
			return entry.wire, !entry.failed
		}
		// Compute srcASN4 once.
		if !srcASN4Set {
			srcCtxID := update.WireUpdate.SourceCtxID()
			srcCtx := bgpctx.Registry.Get(srcCtxID)
			srcASN4 = srcCtx != nil && srcCtx.ASN4()
			srcASN4Set = true
		}

		// Use ReceivedUpdate cache only for the first single-prepend localAS per dstASN4.
		// Dual-prepend variants cannot use it because its cache is keyed only by dstASN4.
		canUseUpdateCache := false
		if secondaryAS == 0 {
			if asn4 {
				if !cachedLocalASN4Set {
					cachedLocalASN4 = localAS
					cachedLocalASN4Set = true
					canUseUpdateCache = true
				} else if cachedLocalASN4 == localAS {
					canUseUpdateCache = true
				}
			} else {
				if !cachedLocalASN2Set {
					cachedLocalASN2 = localAS
					cachedLocalASN2Set = true
					canUseUpdateCache = true
				} else if cachedLocalASN2 == localAS {
					canUseUpdateCache = true
				}
			}
		}

		if canUseUpdateCache {
			wire, err := update.EBGPWire(localAS, srcASN4, asn4)
			if err != nil {
				fwdLogger().Warn("EBGP wire rewrite failed",
					"id", updateID, "localAS", localAS, "asn4", asn4, "err", err)
				ebgpWireCache[ek] = &ebgpWireEntry{failed: true}
				return nil, false
			}
			ebgpWireCache[ek] = &ebgpWireEntry{wire: wire}
			return wire, true
		}

		// Direct generation: either a secondary localAS (multi-override) or a dual-prepend variant.
		payload := update.WireUpdate.Payload()
		extendedMessage := len(payload) > message.MaxMsgLen-message.HeaderLen
		dst := getReadBuf(extendedMessage)
		if dst.Buf == nil {
			ebgpWireCache[ek] = &ebgpWireEntry{failed: true}
			return nil, false
		}
		var n int
		var err error
		if secondaryAS != 0 {
			n, err = wireu.RewriteASPathDual(dst.Buf, payload, localAS, secondaryAS, srcASN4, asn4)
		} else {
			n, err = wireu.RewriteASPath(dst.Buf, payload, localAS, srcASN4, asn4)
		}
		if err != nil {
			ReturnReadBuffer(dst)
			fwdLogger().Warn("EBGP wire rewrite failed",
				"id", updateID, "localAS", localAS, "secondaryAS", secondaryAS, "asn4", asn4, "err", err)
			ebgpWireCache[ek] = &ebgpWireEntry{failed: true}
			return nil, false
		}
		wire := wireu.NewWireUpdate(dst.Buf[:n], update.WireUpdate.SourceCtxID())
		wire.SetMessageID(update.WireUpdate.MessageID())
		wire.SetSourceID(update.WireUpdate.SourceID())
		// Note: dst (pool buffer) is intentionally not returned here.
		// It backs wire for the duration of this ForwardUpdate call.
		// The buffer will be GC'd when the WireUpdate is no longer referenced.
		// This is acceptable for the rare multi-LocalAS or dual-prepend case.
		ebgpWireCache[ek] = &ebgpWireEntry{wire: wire}
		return wire, true
	}

	// Pre-compute send operations per peer, then dispatch to pool.
	// CPU work (split/context comparison/lazy parsing) is fast and done here.
	// TCP writes happen asynchronously in per-peer workers.
	var parsedUpdate *message.Update
	var parsedWire *wireu.WireUpdate
	var dispatchedCount int

	// Group-aware forward cache: when update groups are enabled, peers with
	// the same sendCtxID receiving the same peerWire (no per-peer mods) get
	// identical fwdItem bodies. Cache the computed rawBodies/updates per
	// (destCtxID, peerWire) to avoid redundant context checks and parsing.
	type fwdBodyCacheKey struct {
		destCtxID bgpctx.ContextID
		wire      *wireu.WireUpdate // pointer identity (same wire = same payload)
		extended  bool              // ExtendedMessage affects maxMsgSize and split decisions
	}
	type fwdBodyCacheEntry struct {
		rawBodies [][]byte
		updates   []*message.Update
	}
	groupsEnabled := a.r.updateGroups != nil && a.r.updateGroups.Enabled()
	var fwdBodyCache map[fwdBodyCacheKey]*fwdBodyCacheEntry
	if groupsEnabled {
		fwdBodyCache = make(map[fwdBodyCacheKey]*fwdBodyCacheEntry)
	}

	// Build source PeerFilterInfo once for egress filter chain.
	var srcFilter registry.PeerFilterInfo
	if len(a.r.egressFilters) > 0 {
		srcFilter = registry.PeerFilterInfo{Address: update.SourcePeerIP, PeerAS: 0}
		// Look up source peer's ASN and identity from peers map (may have disconnected).
		a.r.mu.RLock()
		if srcPeer, ok := a.r.findPeerByAddr(update.SourcePeerIP); ok {
			srcFilter.PeerAS = srcPeer.Settings().PeerAS
			srcFilter.Name = srcPeer.Settings().Name
			srcFilter.GroupName = srcPeer.Settings().GroupName
		}
		a.r.mu.RUnlock()
	}

	// Source peer address for overflow ratio tracking (AC-16).
	// Hoisted outside the loop — loop-invariant.
	srcAddr := update.SourcePeerIP

	for _, peer := range matchingPeers {
		if peer.State() != PeerStateEstablished {
			continue // Skip non-established peers
		}

		// RFC 4456: Route reflection forwarding rules.
		// When source is iBGP, apply RR forwarding constraints:
		//   - From client: forward to all clients + all non-clients (reflected)
		//   - From non-client: forward to clients only (not to other non-clients)
		// eBGP sources are forwarded to all peers (existing behavior, no RR filter).
		if srcIsIBGP && !peer.Settings().IsEBGP() {
			dstIsClient := peer.Settings().RouteReflectorClient
			if srcIsRRClient {
				// Client -> client and client -> non-client: always forward.
			} else if !dstIsClient {
				// Non-client -> non-client: suppress (standard iBGP split-horizon).
				continue
			}
			// Non-client -> client: forward (this is the reflection).
		}

		// Egress peer filter chain: check if route should be sent to this peer.
		// mods accumulates per-peer modifications; fresh for each peer.
		var mods registry.ModAccumulator
		if len(a.r.egressFilters) > 0 {
			destFilter := registry.PeerFilterInfo{
				Address:   peer.Settings().Address,
				PeerAS:    peer.Settings().PeerAS,
				Name:      peer.Settings().Name,
				GroupName: peer.Settings().GroupName,
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
		// Policy export filter chain: external plugin filters (after in-process filters).
		if exportFilters := peer.Settings().ExportFilters; len(exportFilters) > 0 && a.r.api != nil {
			attrsWire, attrErr := update.WireUpdate.Attrs()
			if attrErr != nil {
				fwdLogger().Debug("attrs extraction for export filter",
					"peer", peer.Settings().Address, "error", attrErr)
			}
			updateText := FormatUpdateForFilter(attrsWire, update.WireUpdate, nil)
			action, _ := PolicyFilterChain(exportFilters, "export", peer.Settings().Address.String(), peer.Settings().PeerAS,
				updateText, a.r.policyFilterFunc(update.WireUpdate.Payload()),
			)
			if action == PolicyReject {
				continue // Route suppressed by policy export filter for this peer.
			}
		}

		// RFC 4456: Route reflection attribute injection.
		// When reflecting to iBGP peers, add ORIGINATOR_ID (if absent) and
		// prepend own cluster-id to CLUSTER_LIST.
		if srcIsIBGP && peer.Settings().IsIBGP() {
			clusterID := peer.Settings().EffectiveClusterID()
			// ORIGINATOR_ID (type 9): set to source peer's BGP Identifier if not already present.
			// RFC 4456 Section 8: "If the ORIGINATOR_ID is not present, it MUST be set
			// to the BGP Identifier of the originator of the route to the local AS."
			// The handler checks if the attribute already exists and skips if so.
			var origBuf [4]byte
			origBuf[0] = byte(srcRemoteRouterID >> 24)
			origBuf[1] = byte(srcRemoteRouterID >> 16)
			origBuf[2] = byte(srcRemoteRouterID >> 8)
			origBuf[3] = byte(srcRemoteRouterID)
			mods.Op(9, registry.AttrModSet, origBuf[:]) // ORIGINATOR_ID

			// CLUSTER_LIST (type 10): prepend own cluster-id.
			// RFC 4456 Section 8: "The local CLUSTER_ID MUST be prepended to the
			// CLUSTER_LIST."
			var clBuf [4]byte
			clBuf[0] = byte(clusterID >> 24)
			clBuf[1] = byte(clusterID >> 16)
			clBuf[2] = byte(clusterID >> 8)
			clBuf[3] = byte(clusterID)
			mods.Op(10, registry.AttrModPrepend, clBuf[:]) // CLUSTER_LIST
		}

		// RFC 4271 Section 5.1.3: Next-hop rewriting per destination peer.
		applyNextHopMod(peer.Settings(), &mods)

		// Send-community control: suppress community types not in the peer's send list.
		applySendCommunityFilter(peer.Settings(), &mods)

		// AS-override: replace peer's ASN with local ASN in outbound AS_PATH.
		if peer.Settings().ASOverride && peer.Settings().IsEBGP() {
			applyASOverride(peer.Settings(), update.WireUpdate, peer.asn4(), &mods)
		}

		// Select wire version for this peer.
		// RFC 4271 §9.1.2: EBGP peers get AS-PATH-prepended wire.
		// IBGP peers get original wire unchanged.
		peerWire := update.WireUpdate
		if peer.Settings().IsEBGP() {
			// Local-AS dual-AS mode: when the peer has a local-as override
			// (LocalAS != GlobalLocalAS) and neither no-prepend nor replace-as
			// is set, outbound AS_PATH dual-prepends both the override (closest
			// to peer) and the real global AS behind it. Either modifier falls
			// back to single prepend of the override.
			var secondaryAS uint32
			if peer.Settings().GlobalLocalAS != 0 &&
				peer.Settings().GlobalLocalAS != peer.Settings().LocalAS &&
				!peer.Settings().LocalASNoPrepend &&
				!peer.Settings().LocalASReplaceAS {
				secondaryAS = peer.Settings().GlobalLocalAS
			}
			wire, ok := getEBGPWire(peer.Settings().LocalAS, secondaryAS, peer.asn4())
			if !ok {
				continue // Skip: cannot forward without AS_PATH prepend (RFC 4271 §9.1.2)
			}
			if wire != nil {
				peerWire = wire
			}
		}

		// Track per-peer pool buffer from copy-on-modify (set in mods.Len() > 0 branch).
		var modBufIdx int
		var modPoolRef *peerPool

		// RFC 9494: Convert announce to withdrawal for this peer (LLGR egress filter).
		// Checked before attribute mods since withdrawal replaces the entire payload.
		if mods.IsWithdraw() {
			if withdrawal := buildWithdrawalPayload(peerWire.Payload()); withdrawal != nil {
				peerWire = wireu.NewWireUpdate(withdrawal, peerWire.SourceCtxID())
			} else {
				fwdLogger().Warn("withdrawal conversion failed, suppressing route",
					"peer", peer.Settings().Address)
				continue
			}
		} else if mods.Len() > 0 {
			// Apply accumulated attribute modifications from egress filters.
			// Runs AFTER wire selection so mods apply to the correct wire version
			// (e.g., EBGP wire with AS-PATH prepended, not the original).
			// Copy-on-modify: uses per-peer pool buffer when available, avoiding
			// sync.Pool allocation. Zero-cost when mods.Len() == 0 (common case).
			peerKey := fwdKey{peerAddr: peer.Settings().PeerKey()}
			modPool := a.r.fwdPool.OutgoingPool(peerKey)
			if modified, bufIdx := buildModifiedPayload(peerWire.Payload(), &mods, a.r.attrModHandlers, modPool, nil); modified != nil {
				peerWire = wireu.NewWireUpdate(modified, peerWire.SourceCtxID())
				modBufIdx = bufIdx
				modPoolRef = modPool
			}
		}

		// Build the fwdItem with pre-computed send operations for this peer.
		item := fwdItem{peer: peer, meta: update.Meta, peerBufIdx: modBufIdx, peerPoolRef: modPoolRef}

		// Get max message size for this peer (RFC 8654)
		nc := peer.negotiated.Load()
		extendedMessage := nc != nil && nc.ExtendedMessage
		maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, extendedMessage))

		// Group-aware forward: check cache for peers with identical context
		// and wire version. Avoids redundant context checks and parsing.
		destCtxID := peer.SendContextID()
		if groupsEnabled {
			cacheKey := fwdBodyCacheKey{destCtxID: destCtxID, wire: peerWire, extended: extendedMessage}
			if cached, ok := fwdBodyCache[cacheKey]; ok {
				item.rawBodies = cached.rawBodies
				item.updates = cached.updates
				goto dispatch
			}
		}

		{
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
							fwdLogger().Warn("parsing cached update",
								"peer", peer.Settings().Address, "error", parseErr)
							continue // Skip this peer, consistent with split failures
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

			// Store in cache for subsequent group members with same context.
			if groupsEnabled {
				cacheKey := fwdBodyCacheKey{destCtxID: destCtxID, wire: peerWire, extended: extendedMessage}
				fwdBodyCache[cacheKey] = &fwdBodyCacheEntry{
					rawBodies: item.rawBodies,
					updates:   item.updates,
				}
			}
		}
	dispatch:

		// Route superseding key (AC-23): FNV hash of raw body content.
		// Zero for re-encode path items (updates only, no raw bodies).
		item.supersedeKey = fwdSupersedeKey(item.rawBodies)

		// Withdrawal flag (AC-25): true if item contains only withdrawals.
		item.withdrawal = fwdIsWithdrawal(&item)

		// Retain cache buffer for this peer's worker BEFORE dispatch.
		// Must happen before TryDispatch because a worker may call done()
		// (Release) immediately after receiving the item. If Release ran
		// before Retain, retainCount would go negative and trigger premature
		// eviction -- a use-after-free on the pool buffer.
		a.r.recentUpdates.Retain(updateID)
		item.done = func() { a.r.recentUpdates.Release(updateID) }

		key := fwdKey{peerAddr: peer.Settings().PeerKey()}
		if a.r.fwdPool.TryDispatch(key, item) {
			a.r.fwdPool.RecordForwarded(srcAddr)
			dispatchedCount++
		} else if a.r.fwdPool.DispatchOverflow(key, item) {
			// Channel full -- item buffered in overflow for deferred processing.
			a.r.fwdPool.RecordOverflowed(srcAddr)
			dispatchedCount++
		}
		// If DispatchOverflow returned false, pool is stopped -- done() was
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
	if fam, ok := message.ExtractMPFamily(u.PathAttributes); ok {
		return ctx.AddPathFor(fam)
	}

	// No MP attributes — IPv4 unicast (legacy NLRI field).
	return ctx.AddPathFor(family.IPv4Unicast)
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

// applyNextHopMod adds a NEXT_HOP (type 3) or MP_REACH_NLRI (type 14)
// modification to the accumulator based on the destination peer's NextHopMode.
// RFC 4271 Section 5.1.3: next-hop handling for UPDATE messages.
// RFC 4760 Section 3 / RFC 2545 Section 3: IPv6 next-hop lives inside MP_REACH.
//
// For IPv4 destinations the legacy NEXT_HOP attribute (code 3) is rewritten.
// For IPv6 destinations the MP_REACH_NLRI attribute (code 14) is rewritten
// via the mpReachNextHopHandler which patches the next-hop field in place
// while preserving AFI/SAFI/Reserved/NLRI. When the handler sees a source
// attribute that does not match the op's target code it leaves it unchanged,
// so emitting only one of the two ops per peer is sufficient.
//
// TODO(cmd-1-phase5): Mixed-family sessions. A BGP session between IPv6
// endpoints can carry IPv4 routes (and vice versa). Today this function
// emits exactly one op -- matching the session's own address family -- so
// the OTHER family's next-hop on the same UPDATE is left at the source's
// value, which is wrong for "next-hop self" on mixed-family peers. Fixing
// this requires the peer config to expose a paired IPv4/IPv6 local
// address so we can construct BOTH ops and let the handlers ignore
// whichever attribute is absent from the source wire bytes.
func applyNextHopMod(dest *PeerSettings, mods *registry.ModAccumulator) {
	switch dest.NextHopMode {
	case NextHopAuto:
		// Default: rewrite for eBGP, preserve for iBGP. No mod needed --
		// eBGP next-hop is handled by AS-PATH rewriting path which already
		// sets next-hop, and iBGP preserves the original.
		return
	case NextHopSelf:
		if !dest.LocalAddress.IsValid() {
			return
		}
		// Unmap IPv4-mapped IPv6 addresses (::ffff:a.b.c.d) to their native
		// 4-byte form so Is4() takes the legacy path. Without this, a
		// mis-configured LocalAddress in mapped form would fall into the
		// IPv6 branch below and produce a 16-byte next-hop whose global
		// prefix is the IPv4-mapped sentinel, which some peers reject.
		local := dest.LocalAddress.Unmap()
		if local.Is4() {
			nhBytes := local.As4()
			mods.Op(3, registry.AttrModSet, nhBytes[:]) // NEXT_HOP (legacy IPv4)
			return
		}
		// IPv6: rewrite MP_REACH_NLRI next-hop. When the peer config carries
		// a link-local address (RFC 2545 §3) include it as the second 16-byte
		// half of the next-hop so downstream peers on the same link can still
		// reach us.
		if dest.LinkLocal.IsValid() && dest.LinkLocal.Is6() {
			nh := make([]byte, 32)
			global := local.As16()
			ll := dest.LinkLocal.As16()
			copy(nh[:16], global[:])
			copy(nh[16:], ll[:])
			mods.Op(14, registry.AttrModSet, nh)
			return
		}
		nh := local.As16()
		mods.Op(14, registry.AttrModSet, nh[:])
	case NextHopUnchanged:
		// Explicitly preserve: no mod needed -- the original wire bytes
		// already contain the source next-hop.
		return
	case NextHopExplicit:
		if !dest.NextHopAddress.IsValid() {
			return
		}
		explicit := dest.NextHopAddress.Unmap()
		if explicit.Is4() {
			nhBytes := explicit.As4()
			mods.Op(3, registry.AttrModSet, nhBytes[:]) // NEXT_HOP (legacy IPv4)
			return
		}
		// Explicit IPv6 next-hop: always global-only (16-byte NH). The
		// dual-address 32-byte variant is only meaningful for "self" where
		// the router knows both its global and its link-local address.
		nh := explicit.As16()
		mods.Op(14, registry.AttrModSet, nh[:])
	}
}

// applySendCommunityFilter suppresses community attributes not in the peer's send list.
// nil/empty SendCommunity means send all (default). "none" suppresses all.
// Individual types: "standard" (type 8), "large" (type 32), "extended" (type 16).
func applySendCommunityFilter(dest *PeerSettings, mods *registry.ModAccumulator) {
	if len(dest.SendCommunity) == 0 {
		return // Default: send all community types.
	}

	// Build a set of allowed types.
	sendStandard, sendLarge, sendExtended := false, false, false
	for _, v := range dest.SendCommunity {
		switch v {
		case "all":
			return // Explicit "all" means send everything.
		case "none":
			// Suppress all three community types.
			mods.Op(8, registry.AttrModSuppress, nil)  // COMMUNITIES
			mods.Op(16, registry.AttrModSuppress, nil) // EXTENDED_COMMUNITIES
			mods.Op(32, registry.AttrModSuppress, nil) // LARGE_COMMUNITIES
			return
		case "standard":
			sendStandard = true
		case "large":
			sendLarge = true
		case "extended":
			sendExtended = true
		}
	}

	// Suppress types not in the allowed set.
	if !sendStandard {
		mods.Op(8, registry.AttrModSuppress, nil) // COMMUNITIES
	}
	if !sendExtended {
		mods.Op(16, registry.AttrModSuppress, nil) // EXTENDED_COMMUNITIES
	}
	if !sendLarge {
		mods.Op(32, registry.AttrModSuppress, nil) // LARGE_COMMUNITIES
	}
}

// applyASOverride replaces occurrences of the peer's ASN with local ASN in AS_PATH.
// RFC 4271: AS_PATH is type 2. The handler rewrites the AS_PATH segment data.
func applyASOverride(dest *PeerSettings, wire *wireu.WireUpdate, asn4 bool, mods *registry.ModAccumulator) {
	attrs, err := wire.Attrs()
	if err != nil || attrs == nil {
		return
	}
	raw, err := attrs.GetRaw(attribute.AttrASPath)
	if err != nil || len(raw) == 0 {
		return
	}
	// GetRaw returns header+value; extract value only (skip flags+code+len).
	hdrLen := 3
	if len(raw) > 0 && raw[0]&0x10 != 0 {
		hdrLen = 4
	}
	if len(raw) <= hdrLen {
		return
	}
	data := raw[hdrLen:]
	rewritten := rewriteASPathOverride(data, dest.PeerAS, dest.LocalAS, asn4)
	if rewritten != nil {
		mods.Op(2, registry.AttrModSet, rewritten)
	}
}

// rewriteASPathOverride replaces all occurrences of peerAS with localAS in AS_PATH segment data.
// asn4 determines whether ASNs are 4-byte (true) or 2-byte (false).
// Returns nil if no replacement was needed.
func rewriteASPathOverride(data []byte, peerAS, localAS uint32, asn4 bool) []byte {
	asnSize := 4
	if !asn4 {
		asnSize = 2
	}

	// Check if any replacement is needed first (avoid allocation in common case).
	found := false
	pos := 0
	for pos < len(data) {
		if pos+2 > len(data) {
			break
		}
		segLen := int(data[pos+1])
		pos += 2
		for range segLen {
			if pos+asnSize > len(data) {
				return nil // malformed
			}
			var asn uint32
			if asn4 {
				asn = binary.BigEndian.Uint32(data[pos:])
			} else {
				asn = uint32(binary.BigEndian.Uint16(data[pos:]))
			}
			if asn == peerAS {
				found = true
			}
			pos += asnSize
		}
	}

	if !found {
		return nil
	}

	// Make a copy and replace.
	result := make([]byte, len(data))
	copy(result, data)
	pos = 0
	for pos < len(result) {
		if pos+2 > len(result) {
			break
		}
		segLen := int(result[pos+1])
		pos += 2
		for range segLen {
			if pos+asnSize > len(result) {
				return result
			}
			var asn uint32
			if asn4 {
				asn = binary.BigEndian.Uint32(result[pos:])
			} else {
				asn = uint32(binary.BigEndian.Uint16(result[pos:]))
			}
			if asn == peerAS {
				if asn4 {
					binary.BigEndian.PutUint32(result[pos:], localAS)
				} else {
					binary.BigEndian.PutUint16(result[pos:], uint16(localAS)) //nolint:gosec // 2-byte ASN context
				}
			}
			pos += asnSize
		}
	}
	return result
}
