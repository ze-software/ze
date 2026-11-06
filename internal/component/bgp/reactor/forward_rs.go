// Design: docs/architecture/bgp/structural-forwarding.md -- reactor-native RS forwarding
// RFC: rfc/short/rfc4271.md -- LOCAL_PREF is internal-only (Section 5.1.5)
// RFC: rfc/short/rfc4456.md -- route reflection attribute injection (Section 8)
// RFC: rfc/short/rfc7947.md -- route server transparency (Section 2.2.2)
// RFC: rfc/short/rfc8669.md -- the Prefix-SID leaves the SR domain only when configured (Section 8)
// Related: reactor_api_forward.go -- ForwardUpdate egress pipeline (shared helpers)
// Related: forward_pool.go -- per-peer forward worker pool
// Related: forward_build.go -- buildModifiedPayload, buildWithdrawalPayload
// Related: forward_rs_direct.go -- tryDirectWriteNoFlush, the direct write
// Related: forward_rs_source.go -- sourceUsesCachedForward, the source classification
// Related: forward_rs_community.go -- recordRSCommunityEdits, the control-community counters
package reactor

import (
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// reactorForwardRS forwards a received UPDATE to all RS-eligible peers directly
// from notifyMessageReceiver, bypassing the plugin dispatch chain.
//
// Returns the list of destination peers this rail did not decide for, and the
// number of peers it actually dispatched to. The caller stores the skipped list
// on RawMessage.FastPathSkipped so bgp-rs can forward to them via ForwardCached.
//
// A peer is skipped when it carries a rejecting final, which this rail leaves
// to the plugin rail, when its export chain could not run, or when its chain ran and its verdict must be
// withdrawn one NLRI-bearing field at a time (withdrawalBySection). An active export chain is run here, whoever
// placed its members (exportChainActive), and genuine policy suppression by
// either is decided here.
//
// dispatched exists so the caller can tell "the fast path delivered this UPDATE"
// from "the fast path matched nobody". Both used to look identical to bgp-rs,
// which then took its `default: releaseCache` arm and forwarded to nobody
// either -- a silent drop. See the caller in reactor_notify.go.
//
// Buffer lifetime: callers must ensure the cache entry for updateID exists.
// This function calls retainN before dispatch; each fwdItem.done() calls Release.
//
// A client a withhold gate refuses the announcement of an UPDATE that mixes
// NLRI-bearing fields is forwarded again one field at a time
// (withdrawalBySection), the way the general rail does it
// (forwardUpdateSection): each section meets the gates on its own, so the client
// is withdrawn only the field a gate refused, and a source whose MP_REACH_NLRI
// and MP_UNREACH_NLRI name two families is withdrawn both rather than sent
// nothing.
func reactorForwardRS(r *Reactor, update *ReceivedUpdate, updateID uint64, sourcePeerAddr netip.Addr, sourcePeer *Peer) ([]netip.AddrPort, int) {
	// notifyMessageReceiver already classified live entries at receipt.
	var cached bool
	if received := update.receivedPeer; received != nil {
		cached = received.forwardCached.Load()
	} else {
		cached = sourceUsesCachedForward(sourcePeer, update.WireUpdate)
	}
	if cached {
		return nil, 0
	}
	skipped, dispatched, bySection := reactorForwardRSSection(r, update, update.WireUpdate, nil, updateID, sourcePeerAddr, sourcePeer)
	if len(bySection) == 0 {
		return skipped, dispatched
	}
	srcCtx := bgpctx.Registry.Get(update.WireUpdate.SourceCtxID())
	sections, err := wireu.SplitWireUpdate(update.WireUpdate, len(update.WireUpdate.Payload()), srcCtx)
	if err != nil {
		fwdLogger().Warn("withdrawal conversion failed, suppressing route",
			"id", updateID, "peers", len(bySection), "err", err)
		return skipped, dispatched
	}
	// The first section's worker can finish before the remaining sections have
	// borrowed their buffers. This retain MUST survive the whole split fan-out.
	r.recentUpdates.retainN(updateID, 1)
	defer r.recentUpdates.Release(updateID)
	// dispatched counts peers. Every section is offered to the same clients, so
	// the most clients any one section reached is the count this redo adds.
	//
	// A client a section hands back (a reload revoked it, or its replay fence
	// went up, between the passes) joins the skipped list once and leaves the
	// redo, so the route server decides the whole UPDATE for it and no later
	// section reaches it from this rail as well.
	reached := 0
	for _, section := range sections {
		handedBack, sectionDispatched, _ := reactorForwardRSSection(r, update, section, bySection, updateID, sourcePeerAddr, sourcePeer)
		reached = max(reached, sectionDispatched)
		if len(handedBack) > 0 {
			skipped = append(skipped, handedBack...)
			bySection = withoutPeerKeys(bySection, handedBack)
		}
	}
	return skipped, dispatched + reached
}

// withoutPeerKeys answers peers less those whose key is in keys, in order. It
// filters peers in place: the caller MUST NOT read the slice it passed in
// afterwards. Both lists are a redo's destinations, a handful at most, so the
// nested scan costs less than a set would.
func withoutPeerKeys(peers []*Peer, keys []netip.AddrPort) []*Peer {
	kept := peers[:0]
	for _, peer := range peers {
		if !slices.Contains(keys, peer.Settings().PeerKey()) {
			kept = append(kept, peer)
		}
	}
	return kept
}

// reactorForwardRSSection forwards wire, the received UPDATE or one section of
// it, and answers as reactorForwardRS does. A non-nil only limits the
// destinations to those peers. It also returns the clients a withhold gate
// refused whose withdrawal must be sent one field at a time
// (withdrawalBySection); wire is single-field when only is non-nil, so that list
// is then empty.
// Selection scans current peers once for source reflection facts and skipped
// policies, then visits only directly on a retry: O(current peers + selected
// destinations), with no membership map. The retry MUST recheck current peer
// identity and live facts before accepting a pointer from the previous pass.
func reactorForwardRSSection(r *Reactor, update *ReceivedUpdate, wire *wireu.WireUpdate, only []*Peer, updateID uint64, sourcePeerAddr netip.Addr, sourcePeer *Peer) ([]netip.AddrPort, int, []*Peer) {
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
		// The redo reads only the source's facts here: the first pass already
		// handed back what its policy skipped, and the loop over only below
		// decides each redone destination again, so a redo's skipped list
		// names redone destinations alone.
		if only != nil {
			continue
		}
		pf := peer.forwardFacts()
		if pf == nil {
			continue
		}
		// The fast path forwards in place of the route server's cached
		// forward, so it sends nothing to a peer granting the route server no
		// `send [ update ]`. It hands that peer back on the skipped list, as
		// it does a peer its policy skips, so the cached rail decides it the
		// way it would with no fast path: filterPermittedPeers refuses the
		// forward and sendPermissionDenied reports it. Passing over it here
		// instead made the two rails differ: with no granted destination the
		// UPDATE fell back to the route server, which reported the refusal,
		// and with one granted destination the refused one went unreported.
		if !pf.rsRelaySend {
			skipped = append(skipped, pf.peerKey)
			continue
		}
		// RFC 8212 Section 3: cached forwarding applies the same destination final.
		if !pf.exportPolicy.AcceptsAtEnd(pf.isEBGP) {
			skipped = append(skipped, pf.peerKey)
			continue
		}
		// A destination whose replay fence is up is the route server's to
		// decide. Its peer-up replay carries every UPDATE at or below the cut it
		// took when it saw the peer up (rs PeerState.ForwardFrom), and only the
		// plugin rail knows that cut: this rail decides at receipt, possibly
		// before the route server has seen the peer at all. Parked here, an
		// UPDATE the replay also carries reached the peer twice, and the copy
		// released from the fence after the replay overtook nothing but still
		// landed behind it, out of the source's order (RFC 4271 Section 9.2:
		// "All newly installed routes and all newly unfeasible routes for which
		// there is no replacement route SHALL be advertised to its peers by
		// means of an UPDATE message."). Handed back, the plugin rail applies
		// the cut (batchForwardUpdateSkipped), and what it forwards waits behind
		// the same fence.
		if peer.replayOwns(updateID) {
			skipped = append(skipped, pf.peerKey)
			continue
		}
		matchingPeers = append(matchingPeers, peer)
	}
	for _, peer := range only {
		settings := peer.Settings()
		if r.peers[settings.PeerKey()] != peer {
			continue
		}
		if settings.Address == sourcePeerAddr {
			continue
		}
		pf := peer.forwardFacts()
		if pf == nil {
			continue
		}
		// The retry rechecks the send permission with the other live facts:
		// a reload between passes can revoke the grant. A refused client is
		// handed back as the first pass hands it back.
		if !pf.rsRelaySend {
			skipped = append(skipped, pf.peerKey)
			continue
		}
		// RFC 8212 Section 3: retry uses the current coherent destination facts.
		if !pf.exportPolicy.AcceptsAtEnd(pf.isEBGP) {
			skipped = append(skipped, pf.peerKey)
			continue
		}
		// A session that came up between the passes raised its replay fence,
		// and the route server's peer-up replay now owns the UPDATE for it,
		// exactly as on the first pass above: hand it back rather than park a
		// section the replay also carries.
		if peer.replayOwns(updateID) {
			skipped = append(skipped, pf.peerKey)
			continue
		}
		matchingPeers = append(matchingPeers, peer)
	}
	r.mu.RUnlock()

	if len(matchingPeers) == 0 {
		return skipped, 0, nil
	}

	// The source's ASN width, resolved once for the whole client fan-out: it is
	// the SrcASN4 half of every client's AS-path intent.
	srcASN4 := false
	if srcCtxID := wire.SourceCtxID(); srcCtxID != 0 {
		if srcCtx := bgpctx.Registry.Get(srcCtxID); srcCtx != nil {
			srcASN4 = srcCtx.ASN4()
		}
	}

	// Build source PeerFilterInfo once for egress filter chain.
	var srcFilter filterapi.PeerFilterInfo
	var srcKnown bool
	if runsExportChain(matchingPeers) {
		srcFilter = filterapi.PeerFilterInfo{Address: sourcePeerAddr}
		r.mu.RLock()
		if srcPeer, ok := r.findPeerByAddr(sourcePeerAddr); ok {
			srcKnown = true
			// Guarded PeerAS: the source peer may be dynamic and still resolving its ASN
			// on another goroutine; Name/GroupName are immutable after construction.
			srcFilter.PeerAS = srcPeer.PeerAS()
			srcFilter.LocalAS = srcPeer.Settings().LocalAS
			srcFilter.Name = srcPeer.Settings().Name
			srcFilter.GroupName = srcPeer.Settings().GroupName
			// The one IBGP rule (PeerSettings.IsIBGP), so no filter reads a
			// silent false here or compares PeerAS with LocalAS instead.
			srcFilter.IsIBGP = srcPeer.IsIBGP()
		}
		r.mu.RUnlock()
	}

	// Pending dispatch buffer.
	type pendingFwd struct {
		item fwdItem
		key  fwdKey
		// edits are counted at dispatch, past the fence check that can still
		// hand the destination back (recordRSCommunityEdits).
		edits    rsCommunityEdits
		withdraw bool
	}
	var pendingBuf [16]pendingFwd
	pending := pendingBuf[:0]
	// delivered counts peers the UPDATE actually REACHED, which is what the caller
	// turns into a delivery claim. len(pending) counts items built, and the two
	// differ whenever a peer leaves Established, a write fails, or no rail accepts.
	delivered := 0

	// Group-aware body cache: stack-allocated slots avoid per-UPDATE map allocation.
	// Typical RS deployments have 1-2 unique body cache keys (shared encoding context).
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
	var largeCommunityStripBytes []byte
	var communityParsed bool
	var rsLocalAS uint32
	if sourcePeer != nil {
		rsLocalAS = sourcePeer.Settings().GlobalLocalAS
	}
	srcAIGP := payloadAIGP(wire.Payload())
	var srcAIGPLinkMetric uint64
	sourceMessageID := update.sourceMessageID()
	if len(srcAIGP) != 0 {
		srcAIGPLinkMetric = r.sourceAIGPLinkMetric(sourcePeerAddr)
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
	// Three slots: the RS-client prepend a route requests is one to three
	// copies of the route server's AS (wireu.CommunityPolicy.RSPrepend).
	var prependBuf [3]uint32

	// RFC 4271 Section 5.1.5 needs one bit per UPDATE, not one scan per client:
	// whether the source carries LOCAL_PREF at all. See applyFactsLocalPref
	// (forward_local_pref.go) for why the answer gates the operation.
	srcHasLocalPref := payloadHasLocalPref(wire.Payload())

	// RFC 8669 Section 8 needs the same one bit per UPDATE: whether the source
	// carries a Prefix-SID attribute at all. It matters most here. An RS client
	// keeps the source next-hop (RFC 7947 Section 2.2.1), so applyFactsNextHop
	// records nothing for one and the attribute reached every RS client until
	// applyFactsPrefixSID existed (forward_prefix_sid.go).
	srcHasPrefixSID := payloadHasAttr(wire.Payload(), attribute.AttrPrefixSID)

	// RFC 4271 Section 5.1.4 needs one read per UPDATE, not one per client:
	// which MULTI_EXIT_DISC the source sent. See applyFactsMED (forward_med.go)
	// for why RFC 7947 Section 2.2.3 leaves an RS client's metric alone, and why
	// this rail still asks: reactorForwardRS also serves the non-client
	// destinations a route server peers with.
	srcMED := payloadMED(wire.Payload())

	// RFC 4271 Section 5.1.3 needs one read per UPDATE, not one per client: the
	// NEXT_HOP the source sent. A route server relays a client's third-party next
	// hop untouched, so the client that OWNS that address is the everyday way a
	// peer gets told to send traffic to itself (egressNextHopIsPeerOwn,
	// forward_next_hop.go).
	// A FlowSpec UPDATE never reaches this rail (sourceUsesCachedForward sends
	// it to the general rail), so the redirect-to-IP EBGP refusal, which the
	// general rail's validation split carries, has nothing to act on here.
	srcNextHop := payloadNextHop(wire.Payload())

	// RFC 1997 needs one scan per UPDATE, not one per client; see the identical
	// hoist on the general rail (reactor_api_forward.go). The two rails MUST stay
	// behaviorally identical: honoring the well-known communities on one only
	// would leak on whichever path the deployment happens to select.
	srcWellKnown := r.scanWellKnownEgress(wire.Payload(), sourcePeerAddr)

	// A client an egress gate refuses the announcement is sent a WITHDRAWAL of
	// every route this UPDATE names instead. The final writer sends it only
	// where the client was advertised the route, and never over another owner.
	var withdrawals fwdWithdrawal
	var bySection []*Peer
	for _, peer := range matchingPeers {
		session := peer.currentSession()
		nextHopWithheld := false
		facts := peer.forwardFacts()
		if facts == nil {
			continue
		}

		// RFC 1997: an unconditional prohibition, asked before every operator
		// policy. A route-server client is an external peer, so a route received
		// carrying NO_EXPORT reaches none of them.
		//
		// The prohibition covers the ANNOUNCEMENT only, so a refused client is
		// sent the withdrawal of every route the UPDATE names; see the general
		// rail for why that is not optional.
		withhold := !r.wellKnownAllowsEgress(srcWellKnown, !facts.isEBGP)

		// RFC 7947: Community-based selective forwarding for RS-client peers.
		if facts.rsClient && facts.peerAS != 0 {
			if !communityParsed {
				communityParsed = true
				cp := wireu.ParseCommunityPolicy(wire.Payload(), rsLocalAS)
				communityPolicy = &cp
				communityStripBytes = wireu.StripControlCommunities(wire.Payload(), rsLocalAS)
				largeCommunityStripBytes = wireu.StripLargeControlCommunities(wire.Payload(), rsLocalAS)
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
				withhold = true
			}
		}

		// RFC 4456: Route reflection forwarding rules. reflected says this
		// destination receives the route by reflection, which the RFC 4456
		// attribute injection and the draft-ietf-idr-linklocal-capability
		// Section 4 reflection gate both read.
		reflected := srcIsIBGP && !facts.isEBGP
		if reflected {
			if !srcIsRRClient && !facts.rrClient {
				continue
			}
		}

		mods.Reset()
		if withhold {
			mods.SetWithdraw()
		}

		// ONE operation carrying EVERY control community; see the identical site
		// on the general rail (reactor_api_forward.go) for why the multi-value
		// form is the contract rather than a shortcut. The two rails MUST stay
		// behaviorally identical: a fix applied to one only would leak on
		// whichever path the deployment happens to select.
		if facts.rsClient && len(communityStripBytes) > 0 {
			mods.Op(8, filterapi.AttrModRemove, communityStripBytes)
		}
		// The large-community control values in the route server's own namespace
		// leave the same way, as ONE operation for code 32
		// (wireu.StripLargeControlCommunities). Both rails MUST record it.
		var rsEdits rsCommunityEdits
		if facts.rsClient && len(largeCommunityStripBytes) > 0 {
			mods.Op(32, filterapi.AttrModRemove, largeCommunityStripBytes)
			rsEdits.largeStrippedValues = len(largeCommunityStripBytes) / largeCommunityOctets
		}

		// The destination's export chain runs here, whoever placed its
		// members, through the same implementation as the general rail
		// (forwardUpdateCore). The rail shares nothing a member's verdict could
		// leak between destinations: each destination owns its edit set, a
		// rewrite becomes that destination's base for every edit below, and
		// both the dedup and the withdrawal conversion key on the base. A
		// received End-of-RIB or a withdrawal reaches the chain like any UPDATE,
		// and a member that adds nothing to it leaves it byte-identical.
		base := rsBaseOf(wire, srcNextHop, srcHasLocalPref, srcHasPrefixSID, srcMED)
		// treatments is what the destination's export chain set
		// (egressStepResult.treatments); a chain that did not run sets none.
		var treatments uint8
		if facts.exportChainActive && !mods.IsWithdraw() {
			res := r.runEgressPolicyChain(facts.exportPolicy, facts.addrStr, facts.peerAS, facts.localAS, !facts.isEBGP,
				policyview.ExportSession(facts.filterInfo, srcFilter, srcKnown).WithExportMetadata(update.Meta), wire)
			if res.accept {
				treatments = res.treatments
				if res.wireOverride != nil {
					// A policy replaces bytes, never the ingress path identity.
					res.wireOverride.SetSourceID(wire.SourceID())
					base = rsBaseOverride(res.wireOverride, srcNextHop)
				}
			} else {
				if res.failed {
					// A chain that could not run decided nothing: the plugin
					// rail judges it again and drops it fail-closed.
					fwdLogger().Warn("rs fast path: export chain failed, deferring the destination to the plugin rail",
						"id", updateID, "peer", facts.addr, "src", sourcePeerAddr)
					skipped = append(skipped, facts.peerKey)
					continue
				}
				// A reject or a withdraw verdict: the client may hold the
				// previous generation of the route, so it is sent the withdrawal.
				// RFC 4724 Section 2: "An UPDATE message with no reachable
				// Network Layer Reachability Information (NLRI) and empty
				// withdrawn NLRI is specified as the End-of-RIB marker that can
				// be used by a BGP speaker to indicate to its peer the
				// completion of the initial routing update after the session is
				// established." The marker names no route to refuse and
				// converts to no withdrawal, so the client is sent it unchanged
				// rather than nothing.
				if _, isEOR := wire.IsEOR(); !isEOR {
					mods.SetWithdraw()
				}
			}
		}

		// RFC 7947 Section 2.2: a filter that sets preserve-opaque-attributes
		// for this destination (bgp-rs toward a route-server client) passes it
		// the optional attributes Ze does not recognize, unchanged.
		preserveOpaque := treatments&policyview.TreatmentPreserveOpaqueAttributes.Bit() != 0

		// RFC 4456: Route reflection attribute injection for IBGP destinations.
		if reflected {
			mods.Op(9, filterapi.AttrModSet, origBuf[:])
			mods.Op(10, filterapi.AttrModPrepend, facts.clusterIDBytes[:])
		}

		applyFactsNextHop(facts, &mods, base.nextHop.mpFamily)
		applyNextHopFamily(facts, &mods, base.nextHop, base.wire.Payload())
		applyFactsSendCommunity(facts, &mods)
		applyFactsAIGP(facts, srcAIGP, srcNextHop, base.wire.Payload(), sourcePeerAddr, srcAIGPLinkMetric, &mods)

		// RFC 2545 Section 3; draft-ietf-idr-linklocal-capability Section 4.
		// Keep the effective Global unchanged (RFC 7947 Section 2.2.1),
		// normalizing its Link-Local exactly as the cached forwarding rail.
		applyEgressNextHopScope(peer, facts, &mods, base.wire.Payload(), base.nextHop.mpFamily)

		// The egress gates that refuse an announcement because of the next hop
		// about to be written. The general rail (forwardUpdateCore) asks the same
		// function, which carries every quote, so the two rails cannot answer
		// differently. RFC 7947 Section 2.2.1 requires a route server to pass
		// NEXT_HOP through untouched, and withholding keeps that promise: no
		// client is ever sent a next hop this speaker invented.
		if !mods.IsWithdraw() {
			// RFC 4271 Section 5.1.3, RFC 8950 Section 4,
			// draft-ietf-idr-linklocal-capability Sections 2 and 4.
			if gate := egressNextHopWithheld(peer, facts, &mods, base.nextHop, reflected, sourcePeerAddr); gate != withholdNone {
				nextHopWithheld = true
				// A mixed UPDATE is judged again per section
				// (withdrawalBySection), and the section logs the refusal.
				if !base.wire.MixesNLRIFields() {
					gate.warn(facts, sourcePeerAddr, &mods, base.nextHop)
				}
				mods.SetWithdraw()
			}
		}

		// RFC 4271 Section 5.1.5: LOCAL_PREF never crosses to an external peer.
		// Recorded AFTER the egress filter pass above so the Suppress is the last
		// operation on code 5 and wins (filterapi.LastSetOrSuppress). This rail
		// has no wire override, so the source payload is the base the rebuild
		// runs over.
		applyFactsLocalPref(facts, base.hasLocalPref, &mods)

		// RFC 8669 Section 8: the Prefix-SID leaves this AS only toward an EBGP
		// neighbor the operator has placed inside the SR domain. Same base as
		// the sibling above.
		applyFactsPrefixSID(facts, base.hasPrefixSID, &mods)
		if !mods.IsWithdraw() {
			// RFC 9252 Section 2: both rails share the effective-identity
			// decision; configured mode alone never proves a change.
			applyEgressPrefixSIDNextHop(facts, &mods, srcNextHop, base.nextHop, base.hasPrefixSID)
		}

		// RFC 4271 Section 5.1.4: a MED received from one neighboring AS never
		// reaches another, and RFC 7947 Section 2.2.3 exempts a route server
		// client. Same base as the sibling above (applyFactsMED, forward_med.go).
		applyFactsMED(facts, srcMED, base.med, base.wire.Payload(), &mods)

		// The AS-path family is recorded as INTENT, exactly as on the general
		// forward rail, so the one-pass writer emits it into the client's buffer
		// alongside every other edit and no intermediate payload is produced.
		// Policy, override and protocol prepend are resolved as one path.
		aspathWidthChanged := false
		// A client sent the withdrawal carries no AS_PATH, so the path is not
		// resolved for it: a resolve failure must not cost it the withdrawal.
		if facts.isEBGP && !mods.IsWithdraw() {
			intent := wireu.ASPathIntent{SrcASN4: srcASN4, DstASN4: facts.sendASN4}
			if facts.asOverride {
				intent.OverridePeerAS, intent.OverrideLocalAS = facts.peerAS, facts.localAS
			}
			if !facts.rsClient {
				// RFC 4271 Section 9.1.2, with RFC 7705 Section 3.3 ordering: the
				// override ends up outermost, so it is the LAST element.
				if facts.secondaryAS == 0 {
					prependBuf[0] = facts.localAS
					intent.Prepend = prependBuf[:1]
				} else {
					prependBuf[0] = facts.secondaryAS
					prependBuf[1] = facts.localAS
					intent.Prepend = prependBuf[:2]
				}
			}
			if facts.rsClient && communityPolicy != nil {
				// RFC 7947 Section 2.2.2.1, RFC 8195 Section 4.2.1: the one prepend an RS
				// client receives is the one the route requests for it. No request, no
				// change (wireu.CommunityPolicy.RSPrepend).
				intent.Prepend = communityPolicy.RSPrepend(prependBuf[:0], facts.peerAS, rsLocalAS)
				rsEdits.prepended = len(intent.Prepend) > 0
			}
			// RFC 7947 Section 2.2.2: RS clients omit the protocol prepend unless the
			// route requests one; explicit override and width projection share the
			// same intent.
			changed, aspErr := aspathEdit.Record(&mods, base.wire.Payload(), intent)
			if aspErr != nil {
				fwdLogger().Warn("AS_PATH resolve failed, suppressing route",
					"peer", facts.addr, "localAS", facts.localAS,
					"secondaryAS", facts.secondaryAS, "asn4", facts.sendASN4, "err", aspErr)
				continue
			}
			aspathWidthChanged = changed && srcASN4 != facts.sendASN4
		}

		// No intermediate rewritten payload, so no read buffer is borrowed here and
		// nothing is adopted onto the entry.
		peerWire := base.wire
		opaqueReady := false
		var candidate fwdDedupCand
		var materialization []byte

		var modBufIdx int
		var modPoolRef *peerPool

		// A destination refused the announcement is sent the withdrawal of the
		// routes instead: RFC 9494 (the LLGR egress filter, toward an EBGP peer
		// that did not negotiate LLGR) and every withhold gate above (RFC 7606
		// Section 2 treat-as-withdraw). The conversion reads only the base
		// payload, so it is built once per base and shared (fwdWithdrawal), and
		// the shared wire lets the body cache below build its body once too.
		if mods.IsWithdraw() {
			if withdrawalBySection(peerWire, nextHopWithheld) {
				// The redo below splits the RECEIVED UPDATE and judges each
				// section again, so it would ask the export chain about every
				// section: a section the chain never saw alone could be
				// accepted after the whole UPDATE was rejected, and a rewrite
				// of the whole UPDATE would be applied to one section of it.
				// The general rail partitions the judged output instead and
				// invokes no policy again (forwardUpdateCore), so a destination
				// whose chain ran is handed to it, as the plugin rail.
				if facts.exportChainActive {
					skipped = append(skipped, facts.peerKey)
					continue
				}
				bySection = append(bySection, peer)
				continue
			}
			withdrawal := withdrawals.of(update, peerWire)
			if withdrawal == nil {
				fwdLogger().Warn("withdrawal conversion failed, suppressing route",
					"id", updateID, "peer", facts.addr)
				continue
			}
			peerWire = withdrawal
		} else if mods.HasModifications() {
			peerKey := fwdKey{peerAddr: facts.peerKey}
			modPool := r.fwdPool.outgoingPool(peerKey)

			// The same dedup as the general rail, from the same implementation.
			// This is the rail where it matters most: a route server fans every
			// route out to every client, and RFC 7947 gives clients with the
			// same community policy an identical edit set by construction.
			var modified []byte
			var bufIdx int
			shared, cand := dedup.begin(fwdDedupIdentity{base: peerWire, preserveOpaque: preserveOpaque}, &mods)
			if shared != nil {
				modified, bufIdx = copyMaterialization(shared, modPool)
			} else {
				var modFail modifyFailure
				modified, bufIdx, modFail = buildForwardPayload(peerWire.Payload(), &mods, r.attrModHandlers, modPool, preserveOpaque)
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
				candidate, materialization = cand, modified
			}
			if modified != nil {
				opaqueReady = true
				ctxID := peerWire.SourceCtxID()
				if aspathWidthChanged {
					ctxID = fwdContextIDWithASN4(peerWire.SourceCtxID(), facts.sendASN4)
				}
				srcID := peerWire.SourceID()
				peerWire = wireu.NewWireUpdate(modified, ctxID)
				// Same obligation as the general rail, and it matters most here:
				// this is the rail a route server takes, and the community strip
				// RFC 7947 asks for puts every client that carries a policy on
				// this branch. A rebuilt wire that lost its source would key
				// every such client's paths under the singleton config source,
				// so two clients that both chose identifier 1 for different
				// prefixes reach a destination under ONE identifier and the
				// second replaces the first (RFC 7911 Section 5).
				peerWire.SetSourceID(srcID)
				modBufIdx = bufIdx
				modPoolRef = modPool
			}
		}

		item := fwdItem{peer: peer, meta: update.Meta, sourcePeerStr: update.SourcePeerStr, sourceMessageID: sourceMessageID, peerBufIdx: modBufIdx, peerPoolRef: modPoolRef}
		item.session, item.authority = session, adjOutForwarded
		item.receivedPeer, item.receivedGeneration = update.receivedPeer, update.receivedGeneration
		item.sourcePolicyEpoch = update.SourcePolicyEpoch
		item.exportGeneration = facts.exportGeneration

		extendedMessage := facts.extendedMsg
		maxMsgSize := facts.maxMsgSize

		destCtxID := facts.sendCtxID
		if groupsEnabled {
			cacheKey := fwdBodyCacheKey{destCtxID: destCtxID, wire: peerWire, extended: extendedMessage, preserveOpaque: preserveOpaque}
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
			effectiveWire := peerWire
			if !opaqueReady {
				// RFC 4271 Section 5: unmodified input is shared and MUST stay immutable.
				var attrErr error
				effectiveWire, attrErr = parseCache.forwardWire(peerWire, preserveOpaque, update)
				if attrErr != nil {
					fwdLogger().Warn("normalizing forwarding attributes", "peer", facts.addr, "error", attrErr)
					dedup.abandon(candidate)
					r.fwdPool.releaseItem(&item)
					continue
				}
			}
			body, ok := buildFwdBody(effectiveWire, maxMsgSize, destCtxID, peer, facts.addr, &parseCache)
			if !ok {
				// A failed build MUST release any outgoing-pool buffer acquired by
				// the rebuild, just as a failed attribute treatment above does.
				dedup.abandon(candidate)
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
					key:        fwdBodyCacheKey{destCtxID: destCtxID, wire: peerWire, extended: extendedMessage, preserveOpaque: preserveOpaque},
					rawBodies:  body.rawBodies,
					updates:    body.updates,
					supersedeK: body.supersedeKey,
				}
				bodySlotCount++
			}
		}
	dispatch:
		// RFC 7911 Section 2: preserve ingress identity before final admission.
		if err := prepareFwdProvenance(&item, update.WireUpdate, peerWire,
			mods.IsWithdraw() || wire != update.WireUpdate || base.wire != wire); err != nil {
			fwdLogger().Warn("forward provenance failed", "peer", facts.addr, "err", err)
			dedup.abandon(candidate)
			r.fwdPool.releaseItem(&item)
			continue
		}
		// Dedup borrows this item's buffer. Publish only after the body can
		// dispatch; an earlier failure MUST abandon before returning its slot.
		if candidate.valid {
			dedup.commit(candidate, materialization)
		}

		pending = append(pending, pendingFwd{
			item:     item,
			key:      fwdKey{peerAddr: facts.peerKey},
			edits:    rsEdits,
			withdraw: mods.IsWithdraw(),
		})
	}

	// Batch retain + dispatch.
	// Write to destination bufWriters without flushing (pure memory copies).
	// Dirty sessions are tracked on srcSession.fwdDirty for deferred flush
	// when the source bufReader has no more data (natural batch boundary).
	// Falls back to TryDispatch/dispatchOverflow when TryLock fails.
	if len(pending) > 0 {
		r.recentUpdates.retainN(updateID, len(pending))
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
			//
			// A session that came up since selection raised its replay fence
			// while this pass built the item, so the route server's replay now
			// owns the UPDATE for it, as at selection: hand it back rather than
			// park a copy the replay also carries (Peer.initialUpdateOwed).
			// This check is the one that decides. The checks at selection
			// spare the build in the common case, a destination fenced
			// before the UPDATE arrived. The community edits are counted
			// after it, so a destination handed back counts nothing here.
			if dst != nil && dst.replayOwns(updateID) {
				pending[i].item.done()
				r.fwdPool.releaseItem(&pending[i].item)
				skipped = append(skipped, pending[i].key.peerAddr)
				continue
			}
			if !pending[i].withdraw {
				r.recordRSCommunityEdits(pending[i].edits)
			}
			//
			// This rail carries live UPDATEs only, never part of a
			// destination's initial update, so the hold reads it as live.
			if dst != nil && (dst.forwardOrderHold(false) || dst.forwardOverflowPending()) {
				if r.fwdPool.dispatchOverflow(pending[i].key, &pending[i].item) {
					delivered++
					r.fwdPool.recordOverflowed(sourcePeerAddr)
				} else {
					fwdLogger().Warn("rs fast path: no rail accepted the deferred item",
						"id", updateID, "peer", pending[i].key.peerAddr, "src", sourcePeerAddr)
				}
				continue
			}
			// Items already on the destination's worker channel gate the
			// direct write too. TryDispatch took them when this rail's TryLock
			// failed, and the overflow count does not see them, so a direct
			// write now would reach the wire first (Peer.forwardChannelPending).
			// TryDispatch below queues this item behind them instead.
			var handled, written bool
			var dstSession *Session
			if dst == nil || !dst.forwardChannelPending() {
				handled, written, dstSession = tryDirectWriteNoFlush(&pending[i].item)
			}
			switch {
			case handled:
				pending[i].item.done()
				r.fwdPool.releaseItem(&pending[i].item)
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
					srcSession.appendFwdDirty(pending[i].item.peer, dstSession)
				}
			case r.fwdPool.TryDispatch(pending[i].key, &pending[i].item):
				delivered++
				r.fwdPool.recordForwarded(sourcePeerAddr)
			case r.fwdPool.dispatchOverflow(pending[i].key, &pending[i].item):
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

	return skipped, delivered, bySection
}
