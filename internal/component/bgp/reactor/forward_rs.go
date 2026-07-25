// Design: plan/learned/663-rs-gap-0-structural-forwarding.md -- reactor-native RS forwarding
// Related: reactor_api_forward.go -- ForwardUpdate egress pipeline (shared helpers)
// Related: forward_pool.go -- per-peer forward worker pool
// Related: forward_build.go -- buildModifiedPayload, buildWithdrawalPayload
package reactor

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/filterapi"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/core/bgp/context"
)

// tryDirectWriteNoFlush writes UPDATE bodies directly to the destination peer's
// TCP bufWriter from the caller's goroutine (source peer's read goroutine),
// bypassing the forward pool channel send and goroutine context switch.
//
// Does NOT flush or set write deadlines. The caller batches flushes via
// Session.flushFwdDirty when the source bufReader has no more data.
//
// Returns (handled, delivered, dstSession):
//   - (true, true, session): bytes reached the peer's bufWriter; needs deferred flush
//   - (true, false, session): a write FAILED partway; the item is consumed, nothing
//     reliable reached the peer
//   - (true, false, nil): peer not Established; skip (caller must call done)
//   - (false, false, nil): TryLock failed or no session; fall back to pool
//
// handled and delivered are separate on purpose. handled answers "is this item
// finished with, do not re-dispatch it"; delivered answers "did the UPDATE reach
// the peer". Collapsing them is what let reactor_notify.go claim delivery -- and
// so let bgp-rs drop the UPDATE on its `default` arm -- for a peer that was not
// Established or whose write errored.
func tryDirectWriteNoFlush(item *fwdItem) (handled, delivered bool, dst *Session) {
	peer := item.peer
	if peer == nil {
		return false, false, nil
	}

	peer.mu.RLock()
	session := peer.session
	peer.mu.RUnlock()
	if session == nil {
		return false, false, nil
	}

	session.mu.RLock()
	state := session.fsm.State()
	session.mu.RUnlock()
	if state != fsm.StateEstablished {
		return true, false, nil
	}

	if !session.writeMu.TryLock() {
		return false, false, nil
	}

	session.sentMeta = item.meta
	for _, body := range item.rawBodies {
		if err := session.writeRawUpdateBody(body); err != nil {
			session.sentMeta = nil
			session.writeMu.Unlock()
			return true, false, session
		}
	}
	for _, update := range item.updates {
		// Pre-filtered: forwardUpdateCore already ran this peer's export chain
		// (and only then the EBGP prepend). See writeUpdatePreFiltered.
		if err := session.writeUpdatePreFiltered(update); err != nil {
			session.sentMeta = nil
			session.writeMu.Unlock()
			return true, false, session
		}
	}
	session.sentMeta = nil
	session.writeMu.Unlock()
	return true, true, session
}

// reactorForwardRS forwards a received UPDATE to all RS-eligible peers directly
// from notifyMessageReceiver, bypassing the plugin dispatch chain.
//
// Returns the list of destination peers that were skipped (have ExportFilters)
// and the number of peers this call actually dispatched to. The caller stores
// the skipped list on RawMessage.FastPathSkipped so bgp-rs can forward to them
// via ForwardCached.
//
// dispatched exists so the caller can tell "the fast path delivered this UPDATE"
// from "the fast path matched nobody". Both used to look identical to bgp-rs,
// which then took its `default: releaseCache` arm and forwarded to nobody
// either -- a silent drop. See the caller in reactor_notify.go.
//
// Buffer lifetime: callers must ensure the cache entry for updateID exists.
// This function calls RetainN before dispatch; each fwdItem.done() calls Release.
func reactorForwardRS(r *Reactor, update *ReceivedUpdate, updateID uint64, sourcePeerAddr netip.Addr, sourcePeer *Peer) ([]netip.AddrPort, int) {
	// Get source session for deferred flush tracking.
	// Stable because we're on this session's read goroutine; RLock for formal correctness.
	var srcSession *Session
	if sourcePeer != nil {
		sourcePeer.mu.RLock()
		srcSession = sourcePeer.session
		sourcePeer.mu.RUnlock()
	}

	r.mu.RLock()
	var peersBuf [16]*Peer
	matchingPeers := peersBuf[:0]
	var srcIsRRClient, srcIsIBGP bool
	var srcRemoteRouterID uint32
	var skippedBuf [4]netip.AddrPort
	skipped := skippedBuf[:0]

	for _, peer := range r.peers {
		addr := peer.Settings().Address
		if addr == sourcePeerAddr {
			srcIsIBGP = peer.Settings().IsIBGP()
			srcIsRRClient = peer.Settings().RouteReflectorClient
			srcRemoteRouterID = peer.RemoteRouterID()
			continue
		}
		pf := peer.forwardFacts()
		if pf == nil {
			continue
		}
		if len(pf.exportFilters) > 0 {
			skipped = append(skipped, pf.peerKey)
			continue
		}
		matchingPeers = append(matchingPeers, peer)
	}
	r.mu.RUnlock()

	if len(matchingPeers) == 0 {
		return skipped, 0
	}

	// EBGP wire cache: lazily generate AS-PATH-prepended wires per (localAS, secondaryAS, asn4).
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

	getEBGPWire := func(localAS, secondaryAS uint32, asn4 bool) (*wireu.WireUpdate, bool) {
		ek := ebgpWireKey{localAS: localAS, secondaryAS: secondaryAS, asn4: asn4}
		if ebgpWireCache != nil {
			if e, ok := ebgpWireCache[ek]; ok {
				return e.wire, !e.failed
			}
		} else {
			ebgpWireCache = make(map[ebgpWireKey]*ebgpWireEntry)
		}

		if !srcASN4Set {
			srcASN4Set = true
			if srcCtxID := update.WireUpdate.SourceCtxID(); srcCtxID != 0 {
				if srcCtx := bgpctx.Registry.Get(srcCtxID); srcCtx != nil {
					srcASN4 = srcCtx.ASN4()
				}
			}
		}

		// Single-prepend fast path via ReceivedUpdate cache.
		if secondaryAS == 0 {
			cachedLocal := &cachedLocalASN4
			cachedSet := &cachedLocalASN4Set
			if !asn4 {
				cachedLocal = &cachedLocalASN2
				cachedSet = &cachedLocalASN2Set
			}
			if !*cachedSet {
				*cachedSet = true
				*cachedLocal = localAS
				wire, err := update.EBGPWire(localAS, srcASN4, asn4)
				if err != nil {
					ebgpWireCache[ek] = &ebgpWireEntry{failed: true}
					return nil, false
				}
				ebgpWireCache[ek] = &ebgpWireEntry{wire: wire}
				return wire, true
			}
			if *cachedLocal == localAS {
				if e, ok := ebgpWireCache[ek]; ok {
					return e.wire, !e.failed
				}
			}
		}

		// Generate wire via RewriteASPath / RewriteASPathDual.
		payload := update.WireUpdate.Payload()
		extendedMessage := len(payload) > message.MaxMsgLen-message.HeaderLen
		buf := getReadBuf(extendedMessage)
		if buf.Buf == nil {
			ebgpWireCache[ek] = &ebgpWireEntry{failed: true}
			return nil, false
		}
		var n int
		var err error
		if secondaryAS != 0 {
			n, err = wireu.RewriteASPathDual(buf.Buf, payload, localAS, secondaryAS, srcASN4, asn4)
		} else {
			n, err = wireu.RewriteASPath(buf.Buf, payload, localAS, srcASN4, asn4)
		}
		if err != nil || n == 0 {
			ReturnReadBuffer(buf)
			ebgpWireCache[ek] = &ebgpWireEntry{failed: true}
			return nil, false
		}
		wire := wireu.NewWireUpdate(buf.Buf[:n], fwdContextIDWithASN4(update.WireUpdate.SourceCtxID(), asn4))
		wire.SetMessageID(update.WireUpdate.MessageID())
		wire.SetSourceID(update.WireUpdate.SourceID())
		// Site 5: buf backs the per-key local-AS / dual-AS variant aliased zero-copy
		// into async writes; adopt onto the entry, return at eviction (D-1/D-2).
		update.adoptFwdHandle(buf)
		ebgpWireCache[ek] = &ebgpWireEntry{wire: wire}
		return wire, true
	}

	// Resolve srcASN4 eagerly: one registry lookup, used by both getEBGPWire
	// and the RS-client transcode guard. Zero cost on the common path where
	// all peers share the same ASN4 capability.
	if !srcASN4Set {
		srcASN4Set = true
		if srcCtxID := update.WireUpdate.SourceCtxID(); srcCtxID != 0 {
			if srcCtx := bgpctx.Registry.Get(srcCtxID); srcCtx != nil {
				srcASN4 = srcCtx.ASN4()
			}
		}
	}

	// RFC 6793 Section 4.2.2: lazily-generated transcode wire for RS-client
	// peers that lack ASN4. Only allocated on the first mismatch peer.
	var rsTranscodeWire *wireu.WireUpdate
	var rsTranscodeSet, rsTranscodeFailed bool

	// Build source PeerFilterInfo once for egress filter chain.
	var srcFilter filterapi.PeerFilterInfo
	if len(r.egressFilters) > 0 {
		srcFilter = filterapi.PeerFilterInfo{Address: sourcePeerAddr}
		r.mu.RLock()
		if srcPeer, ok := r.findPeerByAddr(sourcePeerAddr); ok {
			// Guarded PeerAS: the source peer may be dynamic and still resolving its ASN
			// on another goroutine; Name/GroupName are immutable after construction.
			srcFilter.PeerAS = srcPeer.PeerAS()
			srcFilter.Name = srcPeer.Settings().Name
			srcFilter.GroupName = srcPeer.Settings().GroupName
		}
		r.mu.RUnlock()
	}

	// Pending dispatch buffer.
	type pendingFwd struct {
		item fwdItem
		key  fwdKey
	}
	var pendingBuf [16]pendingFwd
	pending := pendingBuf[:0]
	// delivered counts peers the UPDATE actually REACHED, which is what the caller
	// turns into a delivery claim. len(pending) counts items built, and the two
	// differ whenever a peer leaves Established, a write fails, or no rail accepts.
	delivered := 0

	// Group-aware body cache: stack-allocated slots avoid per-UPDATE map allocation.
	// Typical RS deployments have 1-2 unique body cache keys (shared encoding context).
	type fwdBodyCacheKey struct {
		destCtxID bgpctx.ContextID
		wire      *wireu.WireUpdate
		extended  bool
	}
	type fwdBodyCacheSlot struct {
		key        fwdBodyCacheKey
		rawBodies  [][]byte
		updates    []*message.Update
		supersedeK uint64
		withdrawal bool
	}
	groupsEnabled := r.updateGroups != nil && r.updateGroups.Enabled()
	var bodySlots [4]fwdBodyCacheSlot
	var bodySlotCount int

	var parseCache fwdParseCache

	// RFC 7947: Parse community-based forwarding policy for RS-client peers.
	// Parsed once before the peer loop; zero-cost when no RS-client peers exist.
	var communityPolicy *wireu.CommunityPolicy
	var communityStripBytes []byte
	var communityParsed bool
	var rsLocalAS uint32
	if sourcePeer != nil {
		rsLocalAS = sourcePeer.Settings().GlobalLocalAS
	}

	// RFC 4456 Section 8: ORIGINATOR_ID bytes are per-UPDATE constant.
	var origBuf [4]byte
	if srcIsIBGP {
		origBuf[0] = byte(srcRemoteRouterID >> 24)
		origBuf[1] = byte(srcRemoteRouterID >> 16)
		origBuf[2] = byte(srcRemoteRouterID >> 8)
		origBuf[3] = byte(srcRemoteRouterID)
	}

	for _, peer := range matchingPeers {
		facts := peer.forwardFacts()
		if facts == nil {
			continue
		}

		// RFC 7947: Community-based selective forwarding for RS-client peers.
		if facts.rsClient && facts.peerAS != 0 {
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
		if srcIsIBGP && !facts.isEBGP {
			if !srcIsRRClient && !facts.rrClient {
				continue
			}
		}

		var mods filterapi.ModAccumulator

		if facts.rsClient && len(communityStripBytes) > 0 {
			mods.Op(8, filterapi.AttrModRemove, communityStripBytes)
		}
		if len(r.egressFilters) > 0 {
			destFilter := facts.filterInfo
			payload := update.WireUpdate.Payload()
			suppressed := false
			for _, filter := range r.egressFilters {
				if accept, _ := safeEgressFilter(filter, srcFilter, destFilter, payload, update.Meta, &mods); !accept {
					suppressed = true
					break
				}
			}
			if suppressed {
				continue
			}
		}

		// RFC 4456: Route reflection attribute injection for IBGP destinations.
		if srcIsIBGP && !facts.isEBGP {
			mods.Op(9, filterapi.AttrModSet, origBuf[:])
			mods.Op(10, filterapi.AttrModPrepend, facts.clusterIDBytes[:])
		}

		applyFactsNextHop(facts, &mods)
		applyFactsSendCommunity(facts, &mods)

		if facts.asOverride && facts.isEBGP {
			applyASOverride(facts.peerAS, facts.localAS, update.WireUpdate, facts.sendASN4, &mods)
		}

		// RFC 7947 Section 2.2.2: RS MUST NOT modify AS_PATH for RS-client peers.
		// RFC 6793 Section 4.2.2: MUST transcode ASN4→ASN2 even for RS-clients.
		peerWire := update.WireUpdate
		if facts.isEBGP && !facts.rsClient {
			wire, ok := getEBGPWire(facts.localAS, facts.secondaryAS, facts.sendASN4)
			if !ok {
				continue
			}
			if wire != nil {
				peerWire = wire
			}
		} else if facts.isEBGP && facts.rsClient && srcASN4 && !facts.sendASN4 {
			if rsTranscodeFailed {
				continue
			}
			if !rsTranscodeSet {
				rsTranscodeSet = true
				payload := update.WireUpdate.Payload()
				extendedMessage := len(payload) > message.MaxMsgLen-message.HeaderLen
				buf := getReadBuf(extendedMessage)
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
				// Site 6: buf backs the per-call RS-client transcode wire aliased
				// zero-copy into async writes; adopt onto the entry, return at
				// eviction (D-1/D-2).
				update.adoptFwdHandle(buf)
				rsTranscodeWire = wire
			}
			if rsTranscodeWire != nil {
				peerWire = rsTranscodeWire
			}
		}

		var modBufIdx int
		var modPoolRef *peerPool

		if mods.IsWithdraw() {
			peerKey := fwdKey{peerAddr: facts.peerKey}
			modPool := r.fwdPool.OutgoingPool(peerKey)
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
			modPool := r.fwdPool.OutgoingPool(peerKey)
			if modified, bufIdx := buildModifiedPayload(peerWire.Payload(), &mods, r.attrModHandlers, modPool, nil); modified != nil {
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
			for j := range bodySlotCount {
				if bodySlots[j].key != cacheKey {
					continue
				}
				item.rawBodies = bodySlots[j].rawBodies
				item.updates = bodySlots[j].updates
				item.supersedeKey = bodySlots[j].supersedeK
				item.withdrawal = bodySlots[j].withdrawal
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

			if groupsEnabled && bodySlotCount < len(bodySlots) {
				bodySlots[bodySlotCount] = fwdBodyCacheSlot{
					key:        fwdBodyCacheKey{destCtxID: destCtxID, wire: peerWire, extended: extendedMessage},
					rawBodies:  body.rawBodies,
					updates:    body.updates,
					supersedeK: body.supersedeKey,
					withdrawal: body.withdrawal,
				}
				bodySlotCount++
			}
		}
	dispatch:

		pending = append(pending, pendingFwd{
			item: item,
			key:  fwdKey{peerAddr: facts.peerKey},
		})
	}

	// Batch retain + dispatch.
	// Write to destination bufWriters without flushing (pure memory copies).
	// Dirty sessions are tracked on srcSession.fwdDirty for deferred flush
	// when the source bufReader has no more data (natural batch boundary).
	// Falls back to TryDispatch/DispatchOverflow when TryLock fails.
	if len(pending) > 0 {
		r.recentUpdates.RetainN(updateID, len(pending))
		for i := range pending {
			pending[i].item.done = func() { r.recentUpdates.Release(updateID) }
			handled, written, dstSession := tryDirectWriteNoFlush(&pending[i].item)
			switch {
			case handled:
				pending[i].item.done()
				if pending[i].item.peerBufIdx > 0 && pending[i].item.peerPoolRef != nil {
					pending[i].item.peerPoolRef.Return(pending[i].item.peerBufIdx)
				}
				if written {
					delivered++
					r.fwdPool.RecordForwarded(sourcePeerAddr)
				} else {
					// Consumed but NOT delivered: the peer left Established, or a
					// write failed partway. Name it -- this is the peer's route
					// silently going missing, and the caller must not report the
					// UPDATE as forwarded on the strength of it.
					fwdLogger().Warn("rs fast path: item consumed without reaching the peer",
						"id", updateID, "peer", pending[i].key.peerAddr, "src", sourcePeerAddr)
				}
				if dstSession != nil && srcSession != nil {
					srcSession.appendFwdDirty(dstSession)
				}
			case r.fwdPool.TryDispatch(pending[i].key, pending[i].item):
				delivered++
				r.fwdPool.RecordForwarded(sourcePeerAddr)
			case r.fwdPool.DispatchOverflow(pending[i].key, pending[i].item):
				delivered++
				r.fwdPool.RecordOverflowed(sourcePeerAddr)
			default:
				// Neither rail took it. Without this arm the item vanished with no
				// trace and no accounting.
				fwdLogger().Warn("rs fast path: no rail accepted the item",
					"id", updateID, "peer", pending[i].key.peerAddr, "src", sourcePeerAddr)
			}
		}
	}

	return skipped, delivered
}
