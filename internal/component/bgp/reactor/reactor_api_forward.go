// Design: docs/architecture/core-design.md — UPDATE forwarding, grouped sending, route refresh
// Design: .claude/rules/design-principles.md — zero-copy, copy-on-modify (shares Incoming Peer Pool buffer across peers)
// Overview: reactor_api.go — API command handling core
// Related: reactor_api_batch.go — NLRI batch operations
// Related: reactor_wire.go — zero-allocation wire UPDATE builders
// Related: reactor_api_forward_batch.go — batch forwarding and dedup
// Related: forward_pool.go — per-peer forward worker pool used by ForwardUpdate
// Related: update_group.go — cross-peer UPDATE grouping index
// Detail: forward_build.go — progressive build for egress attribute modification
package reactor

import (
	"encoding/binary"
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/filterapi"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/core/bgp/context"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/selector"
)

var errNoEstablishedPeersToForwardTo = errors.New("no established peers to forward to")

// AnnounceEOR sends an End-of-RIB marker for the given address family.
// Inlined peer iteration (not sendToMatchingPeers) to count EOR sent per peer.
func (a *reactorAPIAdapter) AnnounceEOR(sel *selector.Selector, afi uint16, safi uint8) error {
	update := message.BuildEOR(family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)})

	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	peers := a.getMatchingPeersSel(sel)

	var errs []error
	sentCount := 0

	for _, peer := range peers {
		if peer.State() != PeerStateEstablished {
			continue
		}
		// During initial route sync, sendInitialRoutes owns EoR ordering: it
		// drains the opQueue (announces/withdraws queued via ShouldQueue) and
		// then emits a per-family EoR for every negotiated family (RFC 4724,
		// peer_initial_sync.go). Announce/withdraw already honor ShouldQueue();
		// AnnounceEOR was the one route-op that wrote directly, so a plugin or
		// route-server EoR could race ahead of the still-queued route NLRI and
		// reach the wire first. Skip here -- the reactor's own EoR covers this
		// family. Counted as handled (so a route-server replay does not see a
		// spurious "no peers" error) but not metered: sendInitialRoutes meters
		// the EoR it sends.
		if peer.ShouldQueue() {
			sentCount++
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
func (a *reactorAPIAdapter) SendRefresh(sel *selector.Selector, afi uint16, safi uint8) error {
	return a.sendRouteRefresh(sel, afi, safi, message.RouteRefreshNormal)
}

// SendBoRR sends a Beginning of Route Refresh marker to matching peers.
// RFC 7313 Section 4: "Before the speaker starts a route refresh...
// the speaker MUST send a BoRR message.".
func (a *reactorAPIAdapter) SendBoRR(sel *selector.Selector, afi uint16, safi uint8) error {
	return a.sendRouteRefresh(sel, afi, safi, message.RouteRefreshBoRR)
}

// SendEoRR sends an End of Route Refresh marker to matching peers.
// RFC 7313 Section 4: "After the speaker completes the re-advertisement
// of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message.".
func (a *reactorAPIAdapter) SendEoRR(sel *selector.Selector, afi uint16, safi uint8) error {
	return a.sendRouteRefresh(sel, afi, safi, message.RouteRefreshEoRR)
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
func (a *reactorAPIAdapter) sendRouteRefresh(sel *selector.Selector, afi uint16, safi uint8, subtype message.RouteRefreshSubtype) error {
	requiresEnhancedRR := subtype == message.RouteRefreshBoRR || subtype == message.RouteRefreshEoRR

	rr := &message.RouteRefresh{
		AFI:     message.AFI(afi),
		SAFI:    message.SAFI(safi),
		Subtype: subtype,
	}

	data := message.PackTo(rr, nil)

	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	peers := a.getMatchingPeersSel(sel)

	var errs []error
	for _, peer := range peers {
		if peer.State() != PeerStateEstablished {
			continue
		}

		neg := peer.negotiated.Load()

		// RFC 2918 Section 4: normal route-refresh requires Route Refresh capability
		if !requiresEnhancedRR {
			if neg == nil || !neg.RouteRefresh {
				continue
			}
		}

		// RFC 7313: BoRR/EoRR require Enhanced Route Refresh capability
		if requiresEnhancedRR {
			if neg == nil || !neg.EnhancedRouteRefresh {
				continue
			}
		}

		if err := peer.SendRawMessage(0, data); err != nil {
			errs = append(errs, err)
		} else {
			peer.IncrRefreshSent()
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
	update, ok := a.r.recentUpdates.Get(updateID)
	if !ok {
		return ErrUpdateExpired
	}
	if pluginName != "" {
		defer func() {
			if ackErr := a.r.recentUpdates.Ack(updateID, pluginName); ackErr != nil {
				cacheLogger().Warn("cache ack after forward failed",
					"id", updateID, "plugin", pluginName, "err", ackErr)
			}
		}()
	}

	// Resolve matching peers and source info under one lock.
	a.r.mu.RLock()
	var peersBuf [16]*Peer
	matchingPeers := peersBuf[:0]
	var srcInfo forwardSourceInfo
	for _, peer := range a.r.peers {
		addr := peer.Settings().Address
		if addr == update.SourcePeerIP {
			s := peer.Settings()
			srcInfo = forwardSourceInfo{
				isIBGP:         s.IsIBGP(),
				isRRClient:     s.RouteReflectorClient,
				remoteRouterID: peer.RemoteRouterID(),
				globalLocalAS:  s.GlobalLocalAS,
			}
			if len(a.r.egressFilters) > 0 {
				srcInfo.filterInfo = filterapi.PeerFilterInfo{
					Address: s.Address,
					PeerAS:  s.PeerAS,
					// Effective per-peer local AS, mirroring the dest filterInfo
					// (peer_forward_facts.go). Kept in sync so no egress filter
					// ever reads a silent zero from src.LocalAS.
					LocalAS:   s.LocalAS,
					Name:      s.Name,
					GroupName: s.GroupName,
				}
			}
			continue
		}
		if sel.Matches(addr) {
			matchingPeers = append(matchingPeers, peer)
		}
	}
	a.r.mu.RUnlock()

	if len(matchingPeers) == 0 {
		return fmt.Errorf("no peers match selector %s", sel)
	}

	return a.forwardUpdateCore(update, updateID, matchingPeers, srcInfo)
}

// forwardSourceInfo holds source-peer facts resolved once per ForwardUpdate call
// (or once per distinct source in a ForwardUpdatesDirect batch). These fields
// drive RFC 4456 route reflection, RFC 7947 community-based RS forwarding,
// and egress filter chain source matching.
type forwardSourceInfo struct {
	isIBGP         bool
	isRRClient     bool
	remoteRouterID uint32
	globalLocalAS  uint32
	filterInfo     filterapi.PeerFilterInfo
}

// forwardUpdateCore is the per-destination dispatch loop shared by ForwardUpdate
// (selector-resolved peers) and ForwardUpdatesDirect (batch-resolved peers).
// matchingPeers must not include the source peer (already excluded by the caller).
func (a *reactorAPIAdapter) forwardUpdateCore(update *ReceivedUpdate, updateID uint64, matchingPeers []*Peer, srcInfo forwardSourceInfo) error {
	// EBGP preparation: lazily generate patched wires keyed by (localAS, secondaryAS, asn4).
	// RFC 4271 S9.1.2: EBGP speakers MUST prepend their own AS to AS_PATH.
	// RFC 6793 S4: ASN4->ASN2 transcoding uses AS_TRANS=23456.
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
		secondaryAS uint32
		asn4        bool
	}
	type ebgpWireEntry struct {
		wire   *wireu.WireUpdate
		failed bool
	}
	var ebgpWireCache map[ebgpWireKey]*ebgpWireEntry
	var srcASN4 bool
	var srcASN4Set bool
	var cachedLocalASN4, cachedLocalASN2 uint32
	var cachedLocalASN4Set, cachedLocalASN2Set bool

	getEBGPWire := func(baseWire *wireu.WireUpdate, localAS, secondaryAS uint32, asn4 bool) (*wireu.WireUpdate, bool) {
		if baseWire == nil {
			return nil, false
		}
		if baseWire != update.WireUpdate {
			srcCtx := bgpctx.Registry.Get(baseWire.SourceCtxID())
			baseSrcASN4 := srcCtx != nil && srcCtx.ASN4()
			extendedMessage := len(baseWire.Payload()) > message.MaxMsgLen-message.HeaderLen
			dst := getReadBuf(extendedMessage)
			if dst.Buf == nil {
				return nil, false
			}
			var n int
			var err error
			if secondaryAS != 0 {
				n, err = wireu.RewriteASPathDual(dst.Buf, baseWire.Payload(), localAS, secondaryAS, baseSrcASN4, asn4)
			} else {
				n, err = wireu.RewriteASPath(dst.Buf, baseWire.Payload(), localAS, baseSrcASN4, asn4)
			}
			if err != nil {
				ReturnReadBuffer(dst)
				fwdLogger().Warn("EBGP wire rewrite failed",
					"id", updateID, "localAS", localAS, "secondaryAS", secondaryAS, "asn4", asn4, "err", err)
				return nil, false
			}
			wire := wireu.NewWireUpdate(dst.Buf[:n], fwdContextIDWithASN4(baseWire.SourceCtxID(), asn4))
			wire.SetMessageID(baseWire.MessageID())
			wire.SetSourceID(baseWire.SourceID())
			return wire, true
		}
		ek := ebgpWireKey{localAS: localAS, secondaryAS: secondaryAS, asn4: asn4}
		if ebgpWireCache == nil {
			ebgpWireCache = make(map[ebgpWireKey]*ebgpWireEntry)
		}
		if entry, ok := ebgpWireCache[ek]; ok {
			return entry.wire, !entry.failed
		}
		if !srcASN4Set {
			srcCtxID := update.WireUpdate.SourceCtxID()
			srcCtx := bgpctx.Registry.Get(srcCtxID)
			srcASN4 = srcCtx != nil && srcCtx.ASN4()
			srcASN4Set = true
		}

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
		wire := wireu.NewWireUpdate(dst.Buf[:n], fwdContextIDWithASN4(update.WireUpdate.SourceCtxID(), asn4))
		wire.SetMessageID(update.WireUpdate.MessageID())
		wire.SetSourceID(update.WireUpdate.SourceID())
		// dst (pool buffer) intentionally not returned: it backs wire for this call's lifetime.
		ebgpWireCache[ek] = &ebgpWireEntry{wire: wire}
		return wire, true
	}

	// Resolve srcASN4 eagerly for the RS-client transcode guard.
	if !srcASN4Set {
		srcCtxID := update.WireUpdate.SourceCtxID()
		srcCtx := bgpctx.Registry.Get(srcCtxID)
		srcASN4 = srcCtx != nil && srcCtx.ASN4()
		srcASN4Set = true
	}

	// RFC 6793 Section 4.2.2: lazily-generated transcode wire for RS-client
	// peers that lack ASN4. Only allocated on the first mismatch peer.
	var rsTranscodeWire *wireu.WireUpdate
	var rsTranscodeSet, rsTranscodeFailed bool

	var parseCache fwdParseCache
	var dispatchedCount int

	type pendingFwd struct {
		item fwdItem
		key  fwdKey
	}
	var pendingBuf [16]pendingFwd
	pending := pendingBuf[:0]

	type fwdBodyCacheKey struct {
		destCtxID bgpctx.ContextID
		wire      *wireu.WireUpdate
		extended  bool
	}
	type fwdBodyCacheEntry struct {
		rawBodies    [][]byte
		updates      []*message.Update
		supersedeKey uint64
		withdrawal   bool
	}
	groupsEnabled := a.r.updateGroups != nil && a.r.updateGroups.Enabled()
	var fwdBodyCache map[fwdBodyCacheKey]*fwdBodyCacheEntry
	if groupsEnabled {
		fwdBodyCache = make(map[fwdBodyCacheKey]*fwdBodyCacheEntry)
	}

	srcFilter := srcInfo.filterInfo
	srcAddr := update.SourcePeerIP

	// RFC 4456 Section 8: ORIGINATOR_ID bytes are per-UPDATE (same source for all peers).
	var origBuf [4]byte
	if srcInfo.isIBGP {
		origBuf[0] = byte(srcInfo.remoteRouterID >> 24)
		origBuf[1] = byte(srcInfo.remoteRouterID >> 16)
		origBuf[2] = byte(srcInfo.remoteRouterID >> 8)
		origBuf[3] = byte(srcInfo.remoteRouterID)
	}

	// RFC 7947: Parse community-based forwarding policy for RS-client peers.
	var communityPolicy *wireu.CommunityPolicy
	var communityStripBytes []byte
	var communityParsed bool
	rsLocalAS := srcInfo.globalLocalAS

	for _, peer := range matchingPeers {
		facts := peer.forwardFacts()
		if facts == nil {
			continue
		}

		// RFC 7947: Community-based selective forwarding for RS-client peers.
		if rsLocalAS != 0 && facts.rsClient && facts.peerAS != 0 {
			if !communityParsed {
				communityParsed = true
				cp := wireu.ParseCommunityPolicy(update.WireUpdate.Payload(), rsLocalAS)
				communityPolicy = &cp
				communityStripBytes = wireu.StripControlCommunities(update.WireUpdate.Payload(), rsLocalAS)
			}
			if !communityPolicy.ShouldForwardTo(facts.peerAS) {
				continue
			}
		}

		// RFC 4456: Route reflection forwarding rules.
		if srcInfo.isIBGP && !facts.isEBGP {
			if !srcInfo.isRRClient && !facts.rrClient {
				continue
			}
		}

		var mods filterapi.ModAccumulator

		if facts.rsClient && len(communityStripBytes) > 0 {
			mods.Op(8, filterapi.AttrModRemove, communityStripBytes)
		}
		// Unified egress filter pass: ONE stage-ordered pipeline over the in-process
		// egress filters (community, gr, role) and the export policy chain
		// (FilterStagePeerChain, which sorts LAST). In-process steps defer their
		// mutations into the shared `mods`; the policy chain step reads the ORIGINAL
		// payload and produces a full wire override. Replaces the former two
		// back-to-back blocks; the cross-system order is now a declared Stage.
		var exportWireOverride *wireu.WireUpdate
		if len(a.r.orderedEgressSteps) > 0 {
			destFilter := facts.filterInfo
			payload := update.WireUpdate.Payload()
			suppressed := false
			for i := range a.r.orderedEgressSteps {
				step := &a.r.orderedEgressSteps[i]
				var res egressStepResult
				if step.policyChain {
					res = a.r.runEgressPolicyChain(facts.exportFilters, facts.addrStr, facts.peerAS, facts.localAS, update.WireUpdate)
				} else {
					res = egressStepResult{accept: safeEgressFilter(step.inproc, srcFilter, destFilter, payload, update.Meta, &mods)}
				}
				if !res.accept {
					suppressed = true
					break
				}
				if res.wireOverride != nil {
					exportWireOverride = res.wireOverride
				}
			}
			if suppressed {
				continue
			}
		}

		// RFC 4456: Route reflection attribute injection.
		if srcInfo.isIBGP && !facts.isEBGP {
			mods.Op(9, filterapi.AttrModSet, origBuf[:])
			mods.Op(10, filterapi.AttrModPrepend, facts.clusterIDBytes[:])
		}

		applyFactsNextHop(facts, &mods)
		applyFactsSendCommunity(facts, &mods)

		peerBaseWire := update.WireUpdate
		if exportWireOverride != nil {
			peerBaseWire = exportWireOverride
		}

		if facts.asOverride && facts.isEBGP {
			applyASOverride(facts.peerAS, facts.localAS, peerBaseWire, facts.sendASN4, &mods)
		}

		// RFC 4271 S9.1.2: EBGP peers get AS-PATH-prepended wire.
		// RFC 7947 Section 2.2.2: RS MUST NOT modify AS_PATH for RS-client peers.
		// RFC 6793 Section 4.2.2: MUST transcode ASN4→ASN2 even for RS-clients.
		peerWire := peerBaseWire
		if facts.isEBGP && !facts.rsClient {
			wire, ok := getEBGPWire(peerBaseWire, facts.localAS, facts.secondaryAS, facts.sendASN4)
			if !ok {
				continue
			}
			if wire != nil {
				peerWire = wire
			}
		} else if facts.isEBGP && facts.rsClient && srcASN4 && !facts.sendASN4 {
			if peerBaseWire != update.WireUpdate {
				// Export-filter override: transcode inline (not cacheable).
				// srcASN4 is still correct: exportWireOverride preserves SourceCtxID.
				extMsg := len(peerBaseWire.Payload()) > message.MaxMsgLen-message.HeaderLen
				buf := getReadBuf(extMsg)
				if buf.Buf == nil {
					continue
				}
				n, err := wireu.TranscodeASPath(buf.Buf, peerBaseWire.Payload(), srcASN4, false)
				if err != nil || n <= 0 {
					ReturnReadBuffer(buf)
					continue
				}
				peerWire = wireu.NewWireUpdate(buf.Buf[:n], fwdContextIDWithASN4(peerBaseWire.SourceCtxID(), false))
				peerWire.SetMessageID(peerBaseWire.MessageID())
				peerWire.SetSourceID(peerBaseWire.SourceID())
			} else {
				if rsTranscodeFailed {
					continue
				}
				if !rsTranscodeSet {
					rsTranscodeSet = true
					payload := update.WireUpdate.Payload()
					extMsg := len(payload) > message.MaxMsgLen-message.HeaderLen
					buf := getReadBuf(extMsg)
					if buf.Buf == nil {
						rsTranscodeFailed = true
						continue
					}
					n, err := wireu.TranscodeASPath(buf.Buf, payload, srcASN4, false)
					if err != nil || n <= 0 {
						ReturnReadBuffer(buf)
						rsTranscodeFailed = true
						continue
					}
					wire := wireu.NewWireUpdate(buf.Buf[:n], fwdContextIDWithASN4(update.WireUpdate.SourceCtxID(), false))
					wire.SetMessageID(update.WireUpdate.MessageID())
					wire.SetSourceID(update.WireUpdate.SourceID())
					rsTranscodeWire = wire
				}
				if rsTranscodeWire != nil {
					peerWire = rsTranscodeWire
				}
			}
		}

		var modBufIdx int
		var modPoolRef *peerPool

		// RFC 9494: Convert announce to withdrawal for this peer (LLGR egress filter).
		if mods.IsWithdraw() {
			peerKey := fwdKey{peerAddr: facts.peerKey}
			modPool := a.r.fwdPool.OutgoingPool(peerKey)
			if withdrawal, bufIdx := buildWithdrawalPayload(peerWire.Payload(), modPool); withdrawal != nil {
				peerWire = wireu.NewWireUpdate(withdrawal, peerWire.SourceCtxID())
				modBufIdx = bufIdx
				modPoolRef = modPool
			} else {
				fwdLogger().Warn("withdrawal conversion failed, suppressing route",
					"peer", facts.addr)
				continue
			}
		} else if mods.HasModifications() {
			peerKey := fwdKey{peerAddr: facts.peerKey}
			modPool := a.r.fwdPool.OutgoingPool(peerKey)
			if modified, bufIdx := buildModifiedPayload(peerWire.Payload(), &mods, a.r.attrModHandlers, modPool, nil); modified != nil {
				peerWire = wireu.NewWireUpdate(modified, peerWire.SourceCtxID())
				modBufIdx = bufIdx
				modPoolRef = modPool
			}
		}

		item := fwdItem{peer: peer, meta: update.Meta, sourcePeerStr: update.SourcePeerStr, peerBufIdx: modBufIdx, peerPoolRef: modPoolRef}

		extendedMessage := facts.extendedMsg
		maxMsgSize := facts.maxMsgSize

		destCtxID := facts.sendCtxID
		if groupsEnabled {
			cacheKey := fwdBodyCacheKey{destCtxID: destCtxID, wire: peerWire, extended: extendedMessage}
			if cached, ok := fwdBodyCache[cacheKey]; ok {
				item.rawBodies = cached.rawBodies
				item.updates = cached.updates
				item.supersedeKey = cached.supersedeKey
				item.withdrawal = cached.withdrawal
				goto dispatch
			}
		}

		{
			body, ok := buildFwdBody(peerWire, maxMsgSize, destCtxID, peer, facts.addr, &parseCache)
			if !ok {
				continue
			}
			item.rawBodies = body.rawBodies
			item.updates = body.updates
			item.supersedeKey = body.supersedeKey
			item.withdrawal = body.withdrawal

			if groupsEnabled {
				cacheKey := fwdBodyCacheKey{destCtxID: destCtxID, wire: peerWire, extended: extendedMessage}
				fwdBodyCache[cacheKey] = &fwdBodyCacheEntry{
					rawBodies:    body.rawBodies,
					updates:      body.updates,
					supersedeKey: body.supersedeKey,
					withdrawal:   body.withdrawal,
				}
			}
		}
	dispatch:

		pending = append(pending, pendingFwd{
			item: item,
			key:  fwdKey{peerAddr: facts.peerKey},
		})
	}

	if len(pending) > 0 {
		a.r.recentUpdates.RetainN(updateID, len(pending))
		for i := range pending {
			pending[i].item.done = func() { a.r.recentUpdates.Release(updateID) }
			if a.r.fwdPool.TryDispatch(pending[i].key, pending[i].item) {
				a.r.fwdPool.RecordForwarded(srcAddr)
				dispatchedCount++
			} else if a.r.fwdPool.DispatchOverflow(pending[i].key, pending[i].item) {
				a.r.fwdPool.RecordOverflowed(srcAddr)
				dispatchedCount++
			}
			// DispatchOverflow false = pool stopped; done() already called (releasing cache ref).
		}
	}

	if dispatchedCount == 0 {
		return errNoEstablishedPeersToForwardTo
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
// For IPv4 local addresses, BOTH legacy NEXT_HOP (type 3) and MP_REACH_NLRI
// (type 14) ops are emitted. The legacy op rewrites IPv4 routes; the MP_REACH
// op uses IPv4-mapped IPv6 (::ffff:a.b.c.d) for IPv6 routes over IPv4 transport.
// Each handler is a no-op when its attribute is absent from the source wire bytes.
//
// For IPv6 local addresses, only the MP_REACH_NLRI op is emitted. IPv4 routes
// over IPv6 transport still need a paired IPv4 local address in the peer config
// (not yet supported).
func applyNextHopMod(dest *PeerSettings, mods *filterapi.ModAccumulator) {
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
			mods.Op(3, filterapi.AttrModSet, nhBytes[:]) // NEXT_HOP (legacy IPv4)
			// Also emit MP_REACH next-hop as IPv4-mapped IPv6 (::ffff:a.b.c.d)
			// for mixed-family sessions carrying IPv6 routes over IPv4 transport.
			// The mpReachNextHopHandler is a no-op when the source UPDATE has no
			// MP_REACH_NLRI, so this is safe for pure-IPv4 UPDATEs.
			mapped := local.As16() // IPv4-mapped IPv6: ::ffff:a.b.c.d
			mods.Op(14, filterapi.AttrModSet, mapped[:])
			return
		}
		// IPv6: rewrite MP_REACH_NLRI next-hop. When the peer config carries
		// a link-local address (RFC 2545 §3) include it as the second 16-byte
		// half of the next-hop so downstream peers on the same link can still
		// reach us.
		if dest.LinkLocal.IsValid() && dest.LinkLocal.Is6() {
			var nh [32]byte
			global := local.As16()
			ll := dest.LinkLocal.As16()
			copy(nh[:16], global[:])
			copy(nh[16:], ll[:])
			mods.OpCopy(14, filterapi.AttrModSet, nh[:])
			mods.Op(40, filterapi.AttrModSuppress, nil)
			return
		}
		nh := local.As16()
		mods.Op(14, filterapi.AttrModSet, nh[:])
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
			mods.Op(3, filterapi.AttrModSet, nhBytes[:]) // NEXT_HOP (legacy IPv4)
			// Also emit MP_REACH next-hop as IPv4-mapped IPv6 for mixed-family sessions.
			mapped := explicit.As16()
			mods.Op(14, filterapi.AttrModSet, mapped[:])
			// RFC 9252 Section 3.3: strip PrefixSID when next-hop changes.
			mods.Op(40, filterapi.AttrModSuppress, nil)
			return
		}
		// Explicit IPv6 next-hop: global-only (16-byte NH). The dual-address
		// 32-byte variant (global + link-local) is only meaningful for "self"
		// where the router knows both addresses. IPv4 explicit is handled above
		// with both legacy NEXT_HOP and IPv4-mapped MP_REACH ops.
		nh := explicit.As16()
		mods.Op(14, filterapi.AttrModSet, nh[:])
	}
	// RFC 9252 Section 3.3: strip PrefixSID when next-hop changes.
	mods.Op(40, filterapi.AttrModSuppress, nil)
}

// applySendCommunityFilter suppresses community attributes not in the peer's send list.
// nil/empty SendCommunity means send all (default). "none" suppresses all.
// Individual types: "standard" (type 8), "large" (type 32), "extended" (type 16).
func applySendCommunityFilter(dest *PeerSettings, mods *filterapi.ModAccumulator) {
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
			mods.Op(8, filterapi.AttrModSuppress, nil)  // COMMUNITIES
			mods.Op(16, filterapi.AttrModSuppress, nil) // EXTENDED_COMMUNITIES
			mods.Op(32, filterapi.AttrModSuppress, nil) // LARGE_COMMUNITIES
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
		mods.Op(8, filterapi.AttrModSuppress, nil) // COMMUNITIES
	}
	if !sendExtended {
		mods.Op(16, filterapi.AttrModSuppress, nil) // EXTENDED_COMMUNITIES
	}
	if !sendLarge {
		mods.Op(32, filterapi.AttrModSuppress, nil) // LARGE_COMMUNITIES
	}
}

// applyASOverride replaces occurrences of the peer's ASN with local ASN in AS_PATH.
// RFC 4271: AS_PATH is type 2. The handler rewrites the AS_PATH segment data.
func applyASOverride(peerAS, localAS uint32, wire *wireu.WireUpdate, asn4 bool, mods *filterapi.ModAccumulator) {
	attrs, err := wire.Attrs()
	if err != nil || attrs == nil {
		return
	}
	raw, err := attrs.GetRaw(attribute.AttrASPath)
	if err != nil || len(raw) == 0 {
		return
	}
	hdrLen := 3
	if len(raw) > 0 && raw[0]&0x10 != 0 {
		hdrLen = 4
	}
	if len(raw) <= hdrLen {
		return
	}
	data := raw[hdrLen:]
	rewritten := rewriteASPathOverride(data, peerAS, localAS, asn4)
	if rewritten != nil {
		mods.Op(2, filterapi.AttrModSet, rewritten)
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

// maxForwardDestinations caps how many destinations a single ForwardUpdatesDirect
// call may resolve. Prevents unbounded allocation under a buggy or malicious
// plugin. Default 4096 matches the typical upper bound on BGP peer count; well
// above practical multi-hundred-peer deployments. Exceeding the cap is an
// explicit error, not a silent truncation (rs-fastpath-3 rules/exact-or-reject).
// Override via `ze.fwd.dest.cap` for very-large deployments.
var _ = env.MustRegister(env.EnvEntry{
	Key: "ze.fwd.dest.cap", Type: "int", Default: "4096",
	Description: "Max destinations per Plugin.ForwardCached call (bounds per-call allocation)",
})
