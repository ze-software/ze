// Design: plan/spec-rs-gap-1-reactor-fastpath.md -- reactor-native RS forwarding
// Related: reactor_api_forward.go -- ForwardUpdate egress pipeline (shared helpers)
// Related: forward_pool.go -- per-peer forward worker pool
// Related: forward_build.go -- buildModifiedPayload, buildWithdrawalPayload
package reactor

import (
	"net/netip"
	"time"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// tryDirectWrite attempts to write the UPDATE directly to the destination
// peer's TCP socket from the caller's goroutine (source peer's read goroutine),
// bypassing the forward pool channel send and goroutine context switch.
//
// Returns true if the write was handled (success or peer not ready to receive).
// Returns false if the session is unavailable or the write lock is contended
// (caller should fall back to forward pool dispatch).
func tryDirectWrite(item *fwdItem) bool {
	peer := item.peer
	if peer == nil {
		return false
	}

	peer.mu.RLock()
	session := peer.session
	peer.mu.RUnlock()
	if session == nil {
		return false
	}

	session.mu.RLock()
	state := session.fsm.State()
	conn := session.conn
	session.mu.RUnlock()
	if state != fsm.StateEstablished || conn == nil {
		return true
	}

	if !session.writeMu.TryLock() {
		return false
	}
	defer session.writeMu.Unlock()

	if err := conn.SetWriteDeadline(session.clock.Now().Add(fwdWriteDeadline())); err != nil {
		return true
	}
	defer func() {
		session.sentMeta = nil
		_ = conn.SetWriteDeadline(time.Time{})
	}()

	session.sentMeta = item.meta
	for _, body := range item.rawBodies {
		if err := session.writeRawUpdateBody(body); err != nil {
			return true
		}
	}
	for _, update := range item.updates {
		if err := session.writeUpdate(update); err != nil {
			return true
		}
	}
	if err := session.flushWrites(); err != nil {
		return true
	}
	session.resetSendHoldTimer()
	return true
}

// reactorForwardRS forwards a received UPDATE to all RS-eligible peers directly
// from notifyMessageReceiver, bypassing the plugin dispatch chain.
//
// Returns the list of destination peers that were skipped (have ExportFilters).
// The caller stores these on RawMessage.FastPathSkipped so bgp-rs can forward
// to them via ForwardCached.
//
// Buffer lifetime: callers must ensure the cache entry for updateID exists.
// This function calls RetainN before dispatch; each fwdItem.done() calls Release.
func reactorForwardRS(r *Reactor, update *ReceivedUpdate, updateID uint64, sourcePeerAddr netip.Addr) []netip.AddrPort {
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
		if peer.State() != PeerStateEstablished {
			continue
		}
		// Peers with ExportFilters fall back to bgp-rs ForwardCached.
		if len(peer.Settings().ExportFilters) > 0 {
			skipped = append(skipped, peer.Settings().PeerKey())
			continue
		}
		matchingPeers = append(matchingPeers, peer)
	}
	r.mu.RUnlock()

	if len(matchingPeers) == 0 {
		return skipped
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
		wire := wireu.NewWireUpdate(buf.Buf[:n], update.WireUpdate.SourceCtxID())
		wire.SetMessageID(update.WireUpdate.MessageID())
		wire.SetSourceID(update.WireUpdate.SourceID())
		ebgpWireCache[ek] = &ebgpWireEntry{wire: wire}
		return wire, true
	}

	// Build source PeerFilterInfo once for egress filter chain.
	var srcFilter registry.PeerFilterInfo
	if len(r.egressFilters) > 0 {
		srcFilter = registry.PeerFilterInfo{Address: sourcePeerAddr}
		r.mu.RLock()
		if srcPeer, ok := r.findPeerByAddr(sourcePeerAddr); ok {
			srcFilter.PeerAS = srcPeer.Settings().PeerAS
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

	// Group-aware body cache.
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
	groupsEnabled := r.updateGroups != nil && r.updateGroups.Enabled()
	var fwdBodyCache map[fwdBodyCacheKey]*fwdBodyCacheEntry
	if groupsEnabled {
		fwdBodyCache = make(map[fwdBodyCacheKey]*fwdBodyCacheEntry)
	}

	var parseCache fwdParseCache

	for _, peer := range matchingPeers {
		// RFC 4456: Route reflection forwarding rules.
		if srcIsIBGP && !peer.Settings().IsEBGP() {
			dstIsClient := peer.Settings().RouteReflectorClient
			if srcIsRRClient {
				// Client -> client and client -> non-client: always forward.
			} else if !dstIsClient {
				continue // Non-client -> non-client: suppress.
			}
		}

		// Egress filter chain.
		var mods registry.ModAccumulator
		if len(r.egressFilters) > 0 {
			destFilter := registry.PeerFilterInfo{
				Address:   peer.Settings().Address,
				PeerAS:    peer.Settings().PeerAS,
				Name:      peer.Settings().Name,
				GroupName: peer.Settings().GroupName,
			}
			payload := update.WireUpdate.Payload()
			suppressed := false
			for _, filter := range r.egressFilters {
				if !safeEgressFilter(filter, srcFilter, destFilter, payload, update.Meta, &mods) {
					suppressed = true
					break
				}
			}
			if suppressed {
				continue
			}
		}

		// RFC 4456: Route reflection attribute injection for IBGP destinations.
		if srcIsIBGP && peer.Settings().IsIBGP() {
			clusterID := peer.Settings().EffectiveClusterID()
			var origBuf [4]byte
			origBuf[0] = byte(srcRemoteRouterID >> 24)
			origBuf[1] = byte(srcRemoteRouterID >> 16)
			origBuf[2] = byte(srcRemoteRouterID >> 8)
			origBuf[3] = byte(srcRemoteRouterID)
			mods.Op(9, registry.AttrModSet, origBuf[:])

			var clBuf [4]byte
			clBuf[0] = byte(clusterID >> 24)
			clBuf[1] = byte(clusterID >> 16)
			clBuf[2] = byte(clusterID >> 8)
			clBuf[3] = byte(clusterID)
			mods.Op(10, registry.AttrModPrepend, clBuf[:])
		}

		// RFC 4271 Section 5.1.3: Next-hop rewriting.
		applyNextHopMod(peer.Settings(), &mods)

		// Send-community control.
		applySendCommunityFilter(peer.Settings(), &mods)

		// AS-override.
		if peer.Settings().ASOverride && peer.Settings().IsEBGP() {
			applyASOverride(peer.Settings(), update.WireUpdate, peer.asn4(), &mods)
		}

		// Select wire version.
		peerWire := update.WireUpdate
		if peer.Settings().IsEBGP() {
			var secondaryAS uint32
			if peer.Settings().GlobalLocalAS != 0 &&
				peer.Settings().GlobalLocalAS != peer.Settings().LocalAS &&
				!peer.Settings().LocalASNoPrepend &&
				!peer.Settings().LocalASReplaceAS {
				secondaryAS = peer.Settings().GlobalLocalAS
			}
			wire, ok := getEBGPWire(peer.Settings().LocalAS, secondaryAS, peer.asn4())
			if !ok {
				continue
			}
			if wire != nil {
				peerWire = wire
			}
		}

		var modBufIdx int
		var modPoolRef *peerPool

		// RFC 9494: Withdrawal conversion (LLGR egress filter).
		if mods.IsWithdraw() {
			peerKey := fwdKey{peerAddr: peer.Settings().PeerKey()}
			modPool := r.fwdPool.OutgoingPool(peerKey)
			if withdrawal, bufIdx := buildWithdrawalPayload(peerWire.Payload(), modPool); withdrawal != nil {
				peerWire = wireu.NewWireUpdate(withdrawal, peerWire.SourceCtxID())
				modBufIdx = bufIdx
				modPoolRef = modPool
			} else {
				fwdLogger().Warn("withdrawal conversion failed, suppressing route",
					"peer", peer.Settings().Address)
				continue
			}
		} else if mods.Len() > 0 {
			peerKey := fwdKey{peerAddr: peer.Settings().PeerKey()}
			modPool := r.fwdPool.OutgoingPool(peerKey)
			if modified, bufIdx := buildModifiedPayload(peerWire.Payload(), &mods, r.attrModHandlers, modPool, nil); modified != nil {
				peerWire = wireu.NewWireUpdate(modified, peerWire.SourceCtxID())
				modBufIdx = bufIdx
				modPoolRef = modPool
			}
		}

		item := fwdItem{peer: peer, meta: update.Meta, peerBufIdx: modBufIdx, peerPoolRef: modPoolRef}

		nc := peer.negotiated.Load()
		extendedMessage := nc != nil && nc.ExtendedMessage
		maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, extendedMessage))

		destCtxID := peer.SendContextID()
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
			body, ok := buildFwdBody(peerWire, maxMsgSize, destCtxID, peer, peer.Settings().Address, &parseCache)
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
			key:  fwdKey{peerAddr: peer.Settings().PeerKey()},
		})
	}

	// Batch retain + dispatch.
	// Try direct write first: acquire session.writeMu via TryLock and write
	// from this goroutine (source peer's read goroutine), eliminating the
	// channel send and worker goroutine context switch. Falls back to
	// TryDispatch/DispatchOverflow when TryLock fails or session unavailable.
	if len(pending) > 0 {
		r.recentUpdates.RetainN(updateID, len(pending))
		for i := range pending {
			pending[i].item.done = func() { r.recentUpdates.Release(updateID) }
			switch {
			case tryDirectWrite(&pending[i].item):
				pending[i].item.done()
				if pending[i].item.peerBufIdx > 0 && pending[i].item.peerPoolRef != nil {
					pending[i].item.peerPoolRef.Return(pending[i].item.peerBufIdx)
				}
				r.fwdPool.RecordForwarded(sourcePeerAddr)
			case r.fwdPool.TryDispatch(pending[i].key, pending[i].item):
				r.fwdPool.RecordForwarded(sourcePeerAddr)
			case r.fwdPool.DispatchOverflow(pending[i].key, pending[i].item):
				r.fwdPool.RecordOverflowed(sourcePeerAddr)
			}
		}
	}

	return skipped
}
