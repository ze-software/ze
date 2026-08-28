// Design: docs/architecture/bgp/structural-forwarding.md -- reactor-native RS forwarding
// RFC: rfc/short/rfc4271.md -- LOCAL_PREF is internal-only (Section 5.1.5)
// RFC: rfc/short/rfc4456.md -- route reflection attribute injection (Section 8)
// RFC: rfc/short/rfc7947.md -- route server transparency (Section 2.2.2)
// Related: reactor_api_forward.go -- ForwardUpdate egress pipeline (shared helpers)
// Related: forward_pool.go -- per-peer forward worker pool
// Related: forward_build.go -- buildModifiedPayload, buildWithdrawalPayload
package reactor

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
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
// Returns the list of destination peers this rail did not decide for, and the
// number of peers it actually dispatched to. The caller stores the skipped list
// on RawMessage.FastPathSkipped so bgp-rs can forward to them via ForwardCached.
//
// A peer is skipped for one of two reasons: it carries ExportFilters, which this
// policy-agnostic rail cannot apply, or an in-process egress filter PANICKED for
// it, which decides nothing. A policy suppression is not a skip -- that IS a
// decision, and it is final here.
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

	// The source's ASN width, resolved once for the whole client fan-out: it is
	// the SrcASN4 half of every client's AS-path intent.
	srcASN4 := false
	if srcCtxID := update.WireUpdate.SourceCtxID(); srcCtxID != 0 {
		if srcCtx := bgpctx.Registry.Get(srcCtxID); srcCtx != nil {
			srcASN4 = srcCtx.ASN4()
		}
	}

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
	}
	groupsEnabled := r.updateGroups != nil && r.updateGroups.Enabled()
	var bodySlots [4]fwdBodyCacheSlot
	var bodySlotCount int

	// Same gate, same table type, same lifetime as the general rail
	// (reactor_api_forward.go). One implementation for both rails is the point:
	// keeping two was how this rail's body cache came to stop at four slots
	// while the other one grew a map.
	var dedup *fwdDedupTable
	if groupsEnabled && !r.forwardDedupOff && len(matchingPeers) > 1 {
		dedup = getFwdDedupTable()
		defer putFwdDedupTable(dedup)
	}

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

	// One accumulator for the whole fan-out; see the identical hoist on the
	// general rail (reactor_api_forward.go) for the isolation contract. The two
	// rails MUST stay behaviorally identical: hoisting one only would leave the
	// other paying the per-destination zeroing that umbrella child 2 makes
	// roughly eight times worse.
	var mods filterapi.ModAccumulator

	// Hoisted with the accumulator: the resolver holds its generators inline, so
	// one value serves the whole client fan-out without allocating per client.
	var aspathEdit wireu.ASPathEdit
	var prependBuf [2]uint32

	// RFC 4271 Section 5.1.5 needs one bit per UPDATE, not one scan per client:
	// whether the source carries LOCAL_PREF at all. See applyFactsLocalPref
	// (forward_local_pref.go) for why the answer gates the operation.
	srcHasLocalPref := payloadHasLocalPref(update.WireUpdate.Payload())

	// RFC 4271 Section 5.1.4 needs one read per UPDATE, not one per client:
	// which MULTI_EXIT_DISC the source sent. See applyFactsMED (forward_med.go)
	// for why RFC 7947 Section 2.2.3 leaves an RS client's metric alone, and why
	// this rail still asks: reactorForwardRS also serves the non-client
	// destinations a route server peers with.
	srcMED := payloadMED(update.WireUpdate.Payload())

	// RFC 4271 Section 5.1.3 needs one read per UPDATE, not one per client: the
	// NEXT_HOP the source sent. A route server relays a client's third-party next
	// hop untouched, so the client that OWNS that address is the everyday way a
	// peer gets told to send traffic to itself (egressNextHopIsPeerOwn,
	// forward_next_hop.go).
	srcNextHop := payloadNextHop(update.WireUpdate.Payload())

	// RFC 1997 needs one scan per UPDATE, not one per client; see the identical
	// hoist on the general rail (reactor_api_forward.go). The two rails MUST stay
	// behaviorally identical: honoring the well-known communities on one only
	// would leak on whichever path the deployment happens to select.
	srcWellKnown := r.scanWellKnownEgress(update.WireUpdate.Payload(), sourcePeerAddr)

	// The withdrawal half, for the clients an egress gate refuses. Same derivation
	// and same nil meaning as the general rail. Nil means the UPDATE withdraws
	// nothing, and then a refused client receives nothing at all
	// (wireu.WithdrawalsOnly).
	//
	// TWO gates share it, so a flag guards the derivation rather than either gate's
	// own condition. RFC 1997 asks its question of every client, so the part is
	// derived up front for an UPDATE carrying a well-known community. RFC 7947
	// below derives it on its first refusal, because a control community refuses a
	// subset of the clients rather than all of them.
	var srcWithdrawOnly *wireu.WireUpdate
	withdrawOnlyDerived := false
	if srcWellKnown != 0 {
		withdrawOnlyDerived = true
		srcWithdrawOnly = wireu.WithdrawalsOnly(update.WireUpdate)
	}

	for _, peer := range matchingPeers {
		facts := peer.forwardFacts()
		if facts == nil {
			continue
		}

		// RFC 1997: an unconditional prohibition, asked before every operator
		// policy. A route-server client is an external peer, so a route received
		// carrying NO_EXPORT reaches none of them.
		//
		// The prohibition covers the ANNOUNCEMENT only, so a refused client still
		// receives the withdrawal half of a mixed UPDATE; see the general rail for
		// why that is not optional.
		destBaseWire := update.WireUpdate
		if !r.wellKnownAllowsEgress(srcWellKnown, !facts.isEBGP) {
			if srcWithdrawOnly == nil {
				continue
			}
			destBaseWire = srcWithdrawOnly
		}

		// RFC 7947: Community-based selective forwarding for RS-client peers.
		if facts.rsClient && facts.peerAS != 0 {
			if !communityParsed {
				communityParsed = true
				cp := wireu.ParseCommunityPolicy(update.WireUpdate.Payload(), rsLocalAS)
				communityPolicy = &cp
				communityStripBytes = wireu.StripControlCommunities(update.WireUpdate.Payload(), rsLocalAS)
			}
			// THE CONTROL COMMUNITIES DECIDE ABOUT A ROUTE, NOT ABOUT A MESSAGE.
			// ShouldForwardTo reads RSBlackhole, WhitelistASNs and BlacklistASNs off
			// the policy parsed from the ANNOUNCED route's communities
			// (wireu.CommunityPolicy). The withdrawn routes traveling in the same
			// UPDATE carry no attribute and were tagged by nobody. Refusing the whole
			// message would leave an excluded client holding a prefix an earlier
			// UPDATE did advertise to it, and ze can no longer take that prefix back
			// until the session resets. Same reasoning, and the same repair, as
			// RFC 1997 above and on the general rail.
			if !communityPolicy.ShouldForwardTo(facts.peerAS) {
				if !withdrawOnlyDerived {
					withdrawOnlyDerived = true
					srcWithdrawOnly = wireu.WithdrawalsOnly(update.WireUpdate)
				}
				if srcWithdrawOnly == nil {
					continue
				}
				destBaseWire = srcWithdrawOnly
			}
		}

		// RFC 4456: Route reflection forwarding rules.
		if srcIsIBGP && !facts.isEBGP {
			if !srcIsRRClient && !facts.rrClient {
				continue
			}
		}

		mods.Reset()

		// ONE operation carrying EVERY control community; see the identical site
		// on the general rail (reactor_api_forward.go) for why the multi-value
		// form is the contract rather than a shortcut. The two rails MUST stay
		// behaviorally identical: a fix applied to one only would leak on
		// whichever path the deployment happens to select.
		if facts.rsClient && len(communityStripBytes) > 0 {
			mods.Op(8, filterapi.AttrModRemove, communityStripBytes)
		}
		if len(r.egressFilters) > 0 {
			destFilter := facts.filterInfo
			payload := destBaseWire.Payload()
			suppressed := false
			filterFailed := false
			for _, filter := range r.egressFilters {
				accept, panicked := safeEgressFilter(filter, srcFilter, destFilter, payload, update.Meta, &mods)
				if !accept {
					suppressed = true
					filterFailed = panicked
					break
				}
			}
			if suppressed {
				// Only a genuine policy decision is this rail's to keep. A step
				// that could not run -- a recovered filter panic is the one such
				// state safeEgressFilter can report -- decided nothing about this
				// destination, so consuming it here would make the crash
				// indistinguishable from a policy suppression: the caller sets
				// ReactorForwarded as soon as any other destination is dispatched,
				// and bgp-rs then takes `default: releaseCache` and forwards to
				// nobody (rs/server_withdrawal.go).
				//
				// Hand it to the plugin rail instead, through the same skipped
				// list an export-filtered peer uses. There it reaches
				// forwardUpdateCore, which reads BOTH returns and classifies a
				// failure as a drop rather than as suppression -- the same
				// fallback the cache-eviction decline above takes, and the same
				// fail-closed outcome if the filter panics again.
				if filterFailed {
					fwdLogger().Warn("rs fast path: egress filter panicked, deferring the destination to the plugin rail",
						"id", updateID, "peer", facts.addr, "src", sourcePeerAddr)
					skipped = append(skipped, facts.peerKey)
				}
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

		// RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT
		// be advertised to a peer using an address of that peer as NEXT_HOP."
		//
		// The general rail (reactor_api_forward.go) carries the full reasoning:
		// asked after every rewrite so it reads the address about to be written,
		// the announcement withheld rather than rewritten, and the withdrawal half
		// still delivered. The two rails MUST answer this the same way -- which
		// one runs is the deployment's rs-fast-path setting, not a policy.
		//
		// RFC 7947 Section 2.2.2 requires a route server to pass NEXT_HOP through
		// untouched, and withholding keeps that promise: no client is ever sent a
		// next hop this speaker invented, and the one client the address names is
		// the one client the address is useless to.
		baseNextHop := srcNextHop
		if destBaseWire != update.WireUpdate {
			baseNextHop = payloadNextHop(destBaseWire.Payload())
		}
		if egressNextHopIsPeerOwn(facts, &mods, baseNextHop) {
			if !withdrawOnlyDerived {
				withdrawOnlyDerived = true
				srcWithdrawOnly = wireu.WithdrawalsOnly(update.WireUpdate)
			}
			fwdLogger().Warn("withholding route: its next hop is this peer's own address",
				"peer", facts.addrStr, "next-hop", facts.addr, "src", sourcePeerAddr,
				"rfc", "RFC 4271 Section 5.1.3",
				"action", "announcement not sent to this peer; withdrawals in the same UPDATE still are")
			if srcWithdrawOnly == nil {
				continue
			}
			destBaseWire = srcWithdrawOnly
		}

		// RFC 4271 Section 5.1.5: LOCAL_PREF never crosses to an external peer.
		// Recorded AFTER the egress filter pass above so the Suppress is the last
		// operation on code 5 and wins (filterapi.LastSetOrSuppress). This rail
		// has no wire override, so the source payload is the base the rebuild runs
		// over -- except for a destination RFC 1997 refuses, whose base is the
		// withdrawal part and carries no attribute at all.
		baseHasLocalPref := srcHasLocalPref
		if destBaseWire != update.WireUpdate {
			baseHasLocalPref = payloadHasLocalPref(destBaseWire.Payload())
		}
		applyFactsLocalPref(facts, baseHasLocalPref, &mods)

		// RFC 4271 Section 5.1.4: a MED received from one neighboring AS never
		// reaches another, and RFC 7947 Section 2.2.3 exempts a route server
		// client. Same two payloads as the sibling above, and the same base for
		// a destination RFC 1997 refuses (applyFactsMED, forward_med.go).
		baseMED := srcMED
		if destBaseWire != update.WireUpdate {
			baseMED = payloadMED(destBaseWire.Payload())
		}
		applyFactsMED(facts, srcMED, baseMED, destBaseWire.Payload(), &mods)

		// The AS-path family is recorded as INTENT, exactly as on the general
		// forward rail, so the one-pass writer emits it into the client's buffer
		// alongside every other edit and no intermediate payload is produced.
		// Recorded BEFORE the AS-override so that override's Set still wins.
		aspathWidthChanged := false
		if facts.isEBGP {
			intent := wireu.ASPathIntent{SrcASN4: srcASN4, DstASN4: facts.sendASN4}
			if !facts.rsClient {
				// RFC 4271 Section 9.1.2, with RFC 7705 Section 3.3 ordering: the
				// override ends up outermost, so it is the LAST element.
				if facts.secondaryAS != 0 {
					prependBuf[0] = facts.secondaryAS
					prependBuf[1] = facts.localAS
					intent.Prepend = prependBuf[:2]
				} else {
					prependBuf[0] = facts.localAS
					intent.Prepend = prependBuf[:1]
				}
			}
			// RFC 7947 Section 2.2.2: an RS client's AS_PATH is never modified, so
			// Prepend stays empty and Record transcodes only.
			changed, aspErr := aspathEdit.Record(&mods, destBaseWire.Payload(), intent)
			if aspErr != nil {
				fwdLogger().Warn("AS_PATH resolve failed, suppressing route",
					"peer", facts.addr, "localAS", facts.localAS,
					"secondaryAS", facts.secondaryAS, "asn4", facts.sendASN4, "err", aspErr)
				continue
			}
			aspathWidthChanged = changed && srcASN4 != facts.sendASN4
		}

		if facts.asOverride && facts.isEBGP {
			applyASOverride(facts.peerAS, facts.localAS, destBaseWire, facts.sendASN4, &mods)
		}

		// No intermediate rewritten payload, so no read buffer is borrowed here and
		// nothing is adopted onto the entry.
		peerWire := destBaseWire

		var modBufIdx int
		var modPoolRef *peerPool

		if mods.IsWithdraw() {
			peerKey := fwdKey{peerAddr: facts.peerKey}
			modPool := r.fwdPool.outgoingPool(peerKey)
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
			modPool := r.fwdPool.outgoingPool(peerKey)

			// The same dedup as the general rail, from the same implementation.
			// This is the rail where it matters most: a route server fans every
			// route out to every client, and RFC 7947 gives clients with the
			// same community policy an identical edit set by construction.
			var modified []byte
			var bufIdx int
			shared, cand := dedup.begin(fwdDedupIdentity{base: peerWire}, &mods)
			if shared != nil {
				modified, bufIdx = copyMaterialization(shared, modPool)
			} else {
				var modFail modifyFailure
				modified, bufIdx, modFail = buildModifiedPayload(peerWire.Payload(), &mods, r.attrModHandlers, modPool, nil)
				// Counts AND says it, once per reason per second; see
				// recordModifyFailure. The route server fans one UPDATE out to every
				// client, so this is the rail where an unbounded line hurt most.
				r.recordModifyFailureAddr(modFail, modifySiteRouteServer, facts.addr)
				if modFail.failed() {
					// Fail closed, as on the general forward rail. On the route
					// server this is the path that strips control communities
					// (RFC 7947), so a silent unmodified forward leaks them to
					// every client.
					dedup.abandon(cand)
					continue
				}
				if modified != nil {
					recordMaterialization()
				}
				dedup.commit(cand, modified)
			}
			if modified != nil {
				ctxID := peerWire.SourceCtxID()
				if aspathWidthChanged {
					ctxID = fwdContextIDWithASN4(peerWire.SourceCtxID(), facts.sendASN4)
				}
				peerWire = wireu.NewWireUpdate(modified, ctxID)
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
				goto dispatch
			}
		}

		{
			body, ok := buildFwdBody(peerWire, maxMsgSize, destCtxID, peer, facts.addr, &parseCache)
			if !ok {
				// Same obligation as the general rail (reactor_api_forward.go):
				// this is the one exit between the rebuild that took the client's
				// Outgoing Peer Pool buffer and the forward pool that returns it.
				r.fwdPool.releaseItem(&item)
				continue
			}
			// Site 8: body.transcodeBuf backs the cross-context RFC 6793 transcode,
			// whose sections body.updates aliases zero-copy -- and the body slots
			// below hand those same sections to later destinations. Adopt onto the
			// entry, return at eviction (D-1/D-2).
			update.adoptFwdHandle(body.transcodeBuf)
			item.rawBodies = body.rawBodies
			item.updates = body.updates
			item.supersedeKey = body.supersedeKey

			if groupsEnabled && bodySlotCount < len(bodySlots) {
				bodySlots[bodySlotCount] = fwdBodyCacheSlot{
					key:        fwdBodyCacheKey{destCtxID: destCtxID, wire: peerWire, extended: extendedMessage},
					rawBodies:  body.rawBodies,
					updates:    body.updates,
					supersedeK: body.supersedeKey,
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
	// Falls back to TryDispatch/dispatchOverflow when TryLock fails.
	if len(pending) > 0 {
		r.recentUpdates.RetainN(updateID, len(pending))
		for i := range pending {
			pending[i].item.done = func() { r.recentUpdates.Release(updateID) }
			// Ordering gate, ahead of the direct write for the same reason the
			// plugin rail gates ahead of TryDispatch (reactor_api_forward.go):
			// a destination inside its initial sync has route operations of its
			// own still to reach the wire, and this UPDATE must not overtake
			// them. The direct write is the rail that would, since it goes
			// straight into the destination's bufWriter.
			//
			// Pending overflow gates it too, and only this rail has to ask. Once
			// the sync ends, items released from the hold are on their way to
			// the destination through the worker, and a direct write would
			// overtake them in the same way. TryDispatch refuses its channel for
			// exactly this reason (forward_pool.go, the FIFO gate); the direct
			// write bypasses the pool, so it consults the same count here. It
			// reads that count through the destination, which is already in
			// hand: four atomic loads for the whole gate, where a pool lookup
			// took fp.mu.RLock and hashed a fwdKey per destination per UPDATE.
			dst := pending[i].item.peer
			if dst != nil && (dst.forwardOrderHold() || dst.forwardOverflowPending()) {
				if r.fwdPool.dispatchOverflow(pending[i].key, pending[i].item) {
					delivered++
					r.fwdPool.recordOverflowed(sourcePeerAddr)
				} else {
					fwdLogger().Warn("rs fast path: no rail accepted the deferred item",
						"id", updateID, "peer", pending[i].key.peerAddr, "src", sourcePeerAddr)
				}
				continue
			}
			handled, written, dstSession := tryDirectWriteNoFlush(&pending[i].item)
			switch {
			case handled:
				pending[i].item.done()
				if pending[i].item.peerBufIdx > 0 && pending[i].item.peerPoolRef != nil {
					pending[i].item.peerPoolRef.Return(pending[i].item.peerBufIdx)
				}
				if written {
					delivered++
					r.fwdPool.recordForwarded(sourcePeerAddr)
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
				r.fwdPool.recordForwarded(sourcePeerAddr)
			case r.fwdPool.dispatchOverflow(pending[i].key, pending[i].item):
				delivered++
				r.fwdPool.recordOverflowed(sourcePeerAddr)
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
