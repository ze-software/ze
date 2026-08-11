// Design: docs/architecture/core-design.md — UPDATE forwarding, grouped sending, route refresh
// Design: .claude/rules/design-principles.md — zero-copy, copy-on-modify (shares Incoming Peer Pool buffer across peers)
// RFC: rfc/short/rfc4271.md — LOCAL_PREF is internal-only (Section 5.1.5)
// RFC: rfc/short/rfc4456.md — route reflection attribute injection (Section 8)
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

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"

	"github.com/ze-software/ze/internal/component/bgp/message"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

var errNoEstablishedPeersToForwardTo = errors.New("no established peers to forward to")

// errAllDestinationsSuppressed reports that every destination was skipped by a
// POLICY decision -- RFC 7947 community forwarding, RFC 4456 reflection rules, or
// an egress filter step -- rather than by a failure to build or dispatch the wire.
//
// The two are indistinguishable from dispatchedCount alone, and conflating them
// is not cosmetic: a caller that treats "nothing dispatched" as success in order
// to tolerate suppression then also swallows read-buffer exhaustion and
// wire-build failures, which are exactly the load-dependent drops worth
// reporting. Callers that must tell them apart test for this sentinel; callers
// that only care whether anything was sent can keep testing for the other.
var errAllDestinationsSuppressed = errors.New("all destinations suppressed by egress policy")

// errForwardNoSource refuses to forward a cached UPDATE whose source peer is no
// longer an established peer of this reactor. It is the forward rail's twin of
// errRelayNoSource (reactor_api_relay.go), which already made this call.
//
// Every fact that decides the egress transform comes from the SOURCE peer, and a
// missing source leaves each of them at a zero that downstream reads as a
// legitimate answer rather than as "unknown" (ai/rules/evidence.md):
//
//   - isIBGP=false disables BOTH halves of RFC 4456 Section 8 at once: the
//     non-client-to-non-client suppression that stops the route being reflected at
//     all, AND the ORIGINATOR_ID / CLUSTER_LIST injection. A route reflector that
//     forwards without those two attributes has removed the loop prevention RFC
//     4456 exists to provide, and a reflector-to-reflector loop is persistent, not
//     transient.
//   - remoteRouterID=0 would put 0.0.0.0 in ORIGINATOR_ID even where the injection
//     did run -- never a valid BGP Identifier, so no reflector downstream can
//     recognize its own id in it. Peer.clearEncodingContexts zeroes this field on
//     teardown (peer.go), which is why "present in r.peers" is not enough and the
//     guard requires an ESTABLISHED source.
//   - globalLocalAS=0 skips RFC 7947 community-based route-server forwarding
//     entirely, so control communities stop being honored.
//   - a zero source PeerFilterInfo hands every egress filter an empty address and
//     AS, so source-matching export policy silently stops matching.
//
// The window is live, not theoretical: peers leave r.peers on dynamic teardown
// (reactor_dynamic.go removeDynamicPeer), on API removal and on config reload
// (reactor_peers.go doRemovePeer), while the UPDATEs they sent stay in the
// recent-update cache for a plugin to forward. Such a route is about to be
// withdrawn anyway, so sending it under the WRONG transform is strictly worse
// than not sending it.
var errForwardNoSource = errors.New("forward: source peer is not an established peer, refusing to forward")

// AnnounceEOR sends an End-of-RIB marker for the given address family.
// Inlined peer iteration (not sendToMatchingPeers) to count EOR sent per peer.
func (a *reactorAPIAdapter) AnnounceEOR(sel *selector.Selector, afi uint16, safi uint8) error {
	fam := family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)}
	update := message.BuildEOR(fam)

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
		//
		// RFC 4724 Section 2 is why this is a DROP and not a defer: the marker
		// means "my initial routing update is complete". Emitting a caller's EoR
		// at its position in a still-draining queue would assert that falsely, so
		// the marker has to come from the producer that knows the dump is done.
		//
		// The suppression is logged because it is otherwise invisible: the caller
		// is told the EoR was handled, so a plugin author whose EoR never reached
		// the wire has nothing to grep for (ai/rules/evidence.md -- a
		// guard that neither denies nor speaks does not exist). Cold path: once
		// per EoR command per peer, never per UPDATE. No counter is added -- the
		// package's only EoR counters are eorSent/eorReceived, whose documented
		// contract (peer_stats.go IncrEORSent) is "markers that reached the
		// socket", and both operators and test/scripts/ze_api.py
		// wait_peer_eor_sent read them as exactly that. A suppression must not
		// touch them.
		if peer.ShouldQueue() {
			logEORSuppressed(peer, fam)
			sentCount++
			continue
		}
		// Second gate, and NOT a replacement for the first. ShouldQueue keeps the
		// ORDER right (this EoR must not overtake still-queued route NLRI); the
		// claim keeps the COUNT right (RFC 4724 Section 2: one End-of-RIB per
		// family per session). They fire in different windows: once initial sync
		// clears ShouldQueue, sendInitialRoutes has already sent and claimed this
		// family, so a route-server replay finishing later used to sail through
		// and put a second identical marker on the wire. Claiming here is also the
		// recovery path -- if the initial-sync send failed it released the claim,
		// so this producer legitimately takes it.
		if !peer.ClaimInitialSyncEOR(fam) {
			sentCount++
			continue
		}
		if err := peer.SendUpdate(update); err != nil {
			// Release, or the family stays marked and the peer never gets it.
			peer.ReleaseInitialSyncEOR(fam)
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

// logEORSuppressed records an End-of-RIB that AnnounceEOR declined to send, and
// says which of the three ShouldQueue conditions caused it.
//
// One message covering all three would have to hedge, and the obvious wording --
// "the marker will be emitted when the drain completes" -- is FALSE in two of
// them: sendInitialRoutes iterates nc.Families() only, so a family that is not
// negotiated never gets a marker at all; and when ShouldQueue is true merely
// because route operations are still queued after the sync finished, the marker
// was already sent rather than pending. ai/rules/cli.md requires the
// "what to do next" leg to be TRUE, not merely present.
func logEORSuppressed(peer *Peer, fam family.Family) {
	addr := peer.Settings().Address
	nc := peer.negotiated.Load()

	switch {
	case nc == nil || !nc.Has(fam):
		routesLogger().Warn("end-of-rib suppressed: family is not negotiated on this session",
			"peer", addr, "family", fam,
			"reason", "family-not-negotiated",
			"effect", "no end-of-rib is emitted for this family; sendInitialRoutes covers negotiated families only")
	case peer.initialSyncInProgress():
		routesLogger().Warn("end-of-rib suppressed: peer is still draining its initial route sync",
			"peer", addr, "family", fam,
			"reason", "initial-sync-in-progress",
			"effect", "marker will be emitted by sendInitialRoutes when the drain completes")
	default:
		routesLogger().Warn("end-of-rib suppressed: route operations are still queued for this peer",
			"peer", addr, "family", fam,
			"reason", "operations-queued-after-initial-sync",
			"effect", "initial sync already completed for this peer and emitted its marker; this one is redundant")
	}
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
			peer.incrRefreshSent()
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
			// Established only, matching resolveRelaySource: a torn-down peer keeps
			// its Settings but Peer.clearEncodingContexts has already zeroed
			// remoteRouterID, so its facts would inject ORIGINATOR_ID 0.0.0.0.
			if peer.State() != PeerStateEstablished {
				continue
			}
			s := peer.Settings()
			srcInfo = forwardSourceInfo{
				// Guarded: source may be a dynamic peer still resolving its ASN.
				isIBGP:         peer.IsIBGP(),
				isRRClient:     s.RouteReflectorClient,
				remoteRouterID: peer.RemoteRouterID(),
				globalLocalAS:  s.GlobalLocalAS,
				resolved:       true,
			}
			if len(a.r.egressFilters) > 0 {
				srcInfo.filterInfo = filterapi.PeerFilterInfo{
					Address: s.Address,
					PeerAS:  peer.PeerAS(),
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

	// Fail CLOSED before considering destinations: without the source peer we
	// cannot reproduce the egress transform a live forward would have applied, and
	// the zero value would silently look like "IBGP=no, no reflection needed".
	// See errForwardNoSource.
	if !srcInfo.resolved {
		fwdLogger().Warn("forward refused: source peer is not an established peer",
			"id", updateID, "source", update.SourcePeerIP, "plugin", pluginName)
		return errForwardNoSource
	}

	if len(matchingPeers) == 0 {
		return fmt.Errorf("no peers match selector %s", sel)
	}

	return a.forwardUpdateCore(update, updateID, matchingPeers, srcInfo)
}

// forwardSourceInfo holds source-peer facts resolved once per ForwardUpdate call
// (or once per distinct source in a ForwardUpdatesDirect batch). These fields
// drive RFC 4456 route reflection, RFC 7947 community-based RS forwarding,
// and egress filter chain source matching.
//
// resolved is what makes the rest of the struct trustworthy, and every producer
// MUST set it. Without it the zero value is indistinguishable from a legitimately
// resolved EBGP non-RS source with no filters, and forwardUpdateCore would then
// quietly skip the RFC 4456 reflection rules and the RFC 7947 policy for a source
// it never actually found. See errForwardNoSource.
type forwardSourceInfo struct {
	isIBGP         bool
	isRRClient     bool
	remoteRouterID uint32
	globalLocalAS  uint32
	filterInfo     filterapi.PeerFilterInfo
	resolved       bool
}

// forwardUpdateCore is the per-destination dispatch loop shared by ForwardUpdate
// (selector-resolved peers) and ForwardUpdatesDirect (batch-resolved peers).
// matchingPeers must not include the source peer (already excluded by the caller).
func (a *reactorAPIAdapter) forwardUpdateCore(update *ReceivedUpdate, updateID uint64, matchingPeers []*Peer, srcInfo forwardSourceInfo) error {
	// Unreachable from any production caller -- ForwardUpdate, ForwardUpdatesDirect
	// and RelayStoredRoute each refuse an unresolved source before getting here.
	// Kept so the zero value can never become a valid-looking answer at the one
	// place that acts on it: this function reads srcInfo six times below, and every
	// one of those reads treats "false"/"0" as a decision rather than as a miss.
	if !srcInfo.resolved {
		fwdLogger().Error("BUG: forward refused: unresolved source facts reached forwardUpdateCore",
			"id", updateID, "source", update.SourcePeerIP, "destinations", len(matchingPeers))
		return errForwardNoSource
	}

	// RFC 4271 Section 9.1.2 (prepend the local AS to an EBGP peer) and RFC 6793
	// Section 4.2.2 (ASN4 transcoding, AS_TRANS) are recorded as INTENT on the
	// per-destination accumulator in the loop below, not produced here as a whole
	// rewritten payload.
	//
	// The (localAS, secondaryAS, dstASN4) wire cache that used to live here, and
	// the two atomic per-update EBGP wire slots it fell back on, existed only to
	// amortize a full payload copy across destinations. With the AS-path family
	// folded into the edit set there is no intermediate payload to share, so both
	// caches and the read-buffer adoption they required are gone.
	// The source's ASN width, resolved once for the whole fan-out: it is the
	// SrcASN4 half of every destination's AS-path intent.
	srcCtx := bgpctx.Registry.Get(update.WireUpdate.SourceCtxID())
	srcASN4 := srcCtx != nil && srcCtx.ASN4()

	var parseCache fwdParseCache
	var dispatchedCount int
	// Destinations skipped by a policy decision rather than by a failure. Only
	// when EVERY matching peer was skipped this way is "nothing dispatched" a
	// correct outcome rather than a drop -- see errAllDestinationsSuppressed.
	var suppressedCount int

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
	}
	groupsEnabled := a.r.updateGroups != nil && a.r.updateGroups.Enabled()
	var fwdBodyCache map[fwdBodyCacheKey]*fwdBodyCacheEntry
	if groupsEnabled {
		fwdBodyCache = make(map[fwdBodyCacheKey]*fwdBodyCacheEntry)
	}

	// The edit-set dedup rides the SAME gate the body cache already had, so the
	// feature inherits an existing off switch instead of adding one, and a
	// deployment with update groups disabled behaves exactly as it did before.
	// A fan-out of one has nothing to share, so it never pays for a digest.
	var dedup *fwdDedupTable
	if groupsEnabled && !a.r.forwardDedupOff && len(matchingPeers) > 1 {
		dedup = getFwdDedupTable()
		defer putFwdDedupTable(dedup)
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

	// One accumulator for the whole fan-out, Reset at the top of each
	// destination instead of a fresh value per destination. Reset clears every
	// field a later destination can read and leaves the inline arena alone, so
	// the cost is independent of the arena size (filterapi.ModAccumulator.Reset).
	//
	// This is an ISOLATION BOUNDARY, not a micro-optimisation: the storage is
	// now shared across destinations, so anything that outlived one iteration
	// would send this peer the previous peer's attributes. Nothing may retain a
	// slice returned by Ops() past the Reset that follows it -- the obligation
	// is stated on Reset, and buildModifiedPayload honors it by copying every
	// op value into the destination's own output buffer before returning.
	var mods filterapi.ModAccumulator

	// Hoisted for the same reason the accumulator is: the resolver holds every
	// generator it can record inline, so one value serves the whole fan-out
	// without allocating per destination. Same isolation contract, too -- nothing
	// may read a generator after the Reset that follows it.
	var aspathEdit wireu.ASPathEdit
	var prependBuf [2]uint32

	// RFC 4271 Section 5.1.5 needs one bit per UPDATE, not one scan per
	// destination: whether the source carries LOCAL_PREF at all. A destination
	// whose policy chain returned a full wire override re-asks over THAT payload
	// below, because the override is the base its rebuild runs over.
	srcHasLocalPref := payloadHasLocalPref(update.WireUpdate.Payload())

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
				suppressedCount++
				continue
			}
		}

		// RFC 4456: Route reflection forwarding rules.
		if srcInfo.isIBGP && !facts.isEBGP {
			if !srcInfo.isRRClient && !facts.rrClient {
				suppressedCount++
				continue
			}
		}

		mods.Reset()

		// ONE operation carrying EVERY control community, not one per value.
		// filterapi.ModAccumulator.Op documents a Remove buffer as a whole number
		// of wire values, and filter_community's handler removes each of them.
		// Splitting here would work too, but it would leave the contract resting
		// on both route-server rails remembering to do it -- which is exactly how
		// this leaked: the handler used to accept a single value only, and
		// silently returned the list untouched for anything longer, so a route
		// tagged with two or more control communities kept all of them.
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
			stepFailed := false
			for i := range a.r.orderedEgressSteps {
				step := &a.r.orderedEgressSteps[i]
				var res egressStepResult
				if step.policyChain {
					res = a.r.runEgressPolicyChain(facts.exportFilters, facts.addrStr, facts.peerAS, facts.localAS, update.WireUpdate)
				} else {
					accept, panicked := safeEgressFilter(step.inproc, srcFilter, destFilter, payload, update.Meta, &mods)
					res = egressStepResult{accept: accept, failed: panicked}
				}
				if !res.accept {
					suppressed = true
					stepFailed = res.failed
					break
				}
				if res.wireOverride != nil {
					exportWireOverride = res.wireOverride
				}
			}
			if suppressed {
				// Only a genuine policy decision counts as suppression. A step
				// that could not run (filter IPC error or unparseable response,
				// missing API server, filter panic) still drops the route
				// fail-closed, but must be reported as a DROP so the relay's
				// completeness check cannot read a plugin timeout under load as
				// a complete replay.
				if !stepFailed {
					suppressedCount++
				}
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

		// RFC 4271 Section 5.1.5: LOCAL_PREF never crosses to an external peer.
		// Recorded AFTER the egress step pass so the Suppress is the last
		// operation on code 5 and wins (filterapi.LastSetOrSuppress), and over
		// peerBaseWire rather than the source, because a policy chain's wire
		// override replaces the payload the rebuild reads.
		baseHasLocalPref := srcHasLocalPref
		if exportWireOverride != nil {
			baseHasLocalPref = payloadHasLocalPref(peerBaseWire.Payload())
		}
		applyFactsLocalPref(facts, baseHasLocalPref, &mods)

		// The AS-path family is recorded as INTENT, so the exactly-sized one-pass
		// writer emits it into the destination buffer alongside every other edit.
		// It used to be produced as a whole rewritten payload first, which made an
		// EBGP destination carrying any policy pay two full payload copies.
		//
		// Recorded BEFORE the AS-override on purpose: both write AS_PATH, the last
		// Set wins, and the override winning is the order these two have always had.
		peerBaseSrcASN4 := srcASN4
		if peerBaseWire != update.WireUpdate {
			if c := bgpctx.Registry.Get(peerBaseWire.SourceCtxID()); c != nil {
				peerBaseSrcASN4 = c.ASN4()
			}
		}
		aspathWidthChanged := false
		if facts.isEBGP {
			intent := wireu.ASPathIntent{SrcASN4: peerBaseSrcASN4, DstASN4: facts.sendASN4}
			if !facts.rsClient {
				// RFC 7705 Section 3.3: the globally configured AS is appended first
				// and the override immediately after, so the override ends up
				// outermost. The intent carries innermost first.
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
			// Prepend stays empty and Record transcodes only -- which RFC 6793
			// Section 4.2.2 still requires when the widths differ.
			changed, aspErr := aspathEdit.Record(&mods, peerBaseWire.Payload(), intent)
			if aspErr != nil {
				// Fail closed: an EBGP peer receiving an unprepended path is a
				// routing-loop risk, and a two-octet peer reads a four-octet path as
				// garbage (ai/rules/evidence.md).
				fwdLogger().Warn("AS_PATH resolve failed, suppressing route",
					"id", updateID, "peer", facts.addr, "localAS", facts.localAS,
					"secondaryAS", facts.secondaryAS, "asn4", facts.sendASN4, "err", aspErr)
				continue
			}
			aspathWidthChanged = changed && peerBaseSrcASN4 != facts.sendASN4
		}

		if facts.asOverride && facts.isEBGP {
			applyASOverride(facts.peerAS, facts.localAS, peerBaseWire, facts.sendASN4, &mods)
		}

		// The prepend and the transcode are already recorded as intent above, so no
		// intermediate rewritten payload is produced here, no read buffer is
		// borrowed, and nothing is adopted onto the entry.
		peerWire := peerBaseWire

		var modBufIdx int
		var modPoolRef *peerPool

		// RFC 9494: Convert announce to withdrawal for this peer (LLGR egress filter).
		if mods.IsWithdraw() {
			peerKey := fwdKey{peerAddr: facts.peerKey}
			modPool := a.r.fwdPool.outgoingPool(peerKey)
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
			modPool := a.r.fwdPool.outgoingPool(peerKey)

			// Ask, before rebuilding, whether an earlier destination in this
			// same fan-out already produced exactly these bytes. A route server
			// or reflector sends one route to every client in a group, and the
			// group's members share their policy by construction, so the answer
			// is usually yes and the rebuild it skips is the most expensive step
			// on this path.
			//
			// The reuse is a COPY into this destination's own buffer, not a
			// shared one: the rebuild costs 416ns and the copy 2ns
			// (BenchmarkFanoutRebuildOnly), so the entire ownership question --
			// one buffer, several items, released after the last worker -- buys
			// 0.5% and is not worth its blast radius.
			var modified []byte
			var bufIdx int
			shared, cand := dedup.begin(fwdDedupIdentity{base: peerWire}, &mods)
			if shared != nil {
				modified, bufIdx = copyMaterialization(shared, modPool)
			} else {
				var modFail modifyFailure
				modified, bufIdx, modFail = buildModifiedPayload(peerWire.Payload(), &mods, a.r.attrModHandlers, modPool, nil)
				// Counts AND says it, once per reason per second; see
				// recordModifyFailure. This fires once per DESTINATION, so an
				// unbounded line here scaled with fan-out.
				a.r.recordModifyFailureAddr(modFail, modifySiteEgressForward, facts.addr)
				if modFail.failed() {
					// Fail closed. The policy asked for a change we could not make,
					// so forwarding this route sends exactly what the policy exists
					// to prevent (ai/rules/evidence.md). This is a step
					// that COULD NOT RUN, not a policy decision, so it is not
					// counted as a policy suppression -- same distinction the
					// egress chain draws with egressStepResult.failed.
					dedup.abandon(cand)
					continue
				}
				if modified != nil {
					recordMaterialization()
				}
				dedup.commit(cand, modified)
			}
			if modified != nil {
				// An ASN4 transcode folded into the rebuild changed the AS number
				// width of these bytes, so the wire must carry the DESTINATION's
				// context. Labeling it with the source context would let buildFwdBody
				// read the two as matching and forward re-encoded bytes as if they were
				// still the source's, or transcode them a second time.
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
			if cached, ok := fwdBodyCache[cacheKey]; ok {
				item.rawBodies = cached.rawBodies
				item.updates = cached.updates
				item.supersedeKey = cached.supersedeKey
				goto dispatch
			}
		}

		{
			body, ok := buildFwdBody(peerWire, maxMsgSize, destCtxID, peer, facts.addr, &parseCache)
			if !ok {
				// The rebuild above can have put this destination's Outgoing Peer
				// Pool buffer on the item, and this is the ONE exit between that
				// acquire and the forward pool that returns it. Dropping the item
				// here loses the buffer for the life of the session, one per
				// failing UPDATE, out of the 64 the destination has.
				a.r.fwdPool.releaseItem(&item)
				continue
			}
			// Site 7: body.transcodeBuf backs the cross-context RFC 6793 transcode,
			// whose sections body.updates aliases zero-copy -- and the body cache
			// below hands those same sections to later destinations. Adopt onto the
			// entry, return at eviction (D-1/D-2).
			update.adoptFwdHandle(body.transcodeBuf)
			item.rawBodies = body.rawBodies
			item.updates = body.updates
			item.supersedeKey = body.supersedeKey

			if groupsEnabled {
				cacheKey := fwdBodyCacheKey{destCtxID: destCtxID, wire: peerWire, extended: extendedMessage}
				fwdBodyCache[cacheKey] = &fwdBodyCacheEntry{
					rawBodies:    body.rawBodies,
					updates:      body.updates,
					supersedeKey: body.supersedeKey,
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
			// Ordering gate: a destination inside its initial sync still has
			// route operations of its own to put on the wire. Sending this
			// UPDATE now would overtake them, and a forwarded withdraw landing
			// before the queued announce of the same prefix leaves the peer
			// holding a route that was withdrawn. Park it in overflow instead;
			// drainOverflow releases it when the sync ends, on this same
			// predicate (peer.go forwardOrderHold, forward_pool.go overflowHeld).
			if dst := pending[i].item.peer; dst != nil && dst.forwardOrderHold() {
				if a.r.fwdPool.DispatchOverflow(pending[i].key, pending[i].item) {
					a.r.fwdPool.recordOverflowed(srcAddr)
					dispatchedCount++
				}
				continue
			}
			if a.r.fwdPool.TryDispatch(pending[i].key, pending[i].item) {
				a.r.fwdPool.recordForwarded(srcAddr)
				dispatchedCount++
			} else if a.r.fwdPool.DispatchOverflow(pending[i].key, pending[i].item) {
				a.r.fwdPool.recordOverflowed(srcAddr)
				dispatchedCount++
			}
			// DispatchOverflow false = pool stopped; done() already called (releasing cache ref).
		}
	}

	if dispatchedCount == 0 {
		// Every destination skipped by policy is a correct outcome; anything else
		// reaching zero is a drop the caller must be able to see.
		if suppressedCount > 0 && suppressedCount == len(matchingPeers) {
			return errAllDestinationsSuppressed
		}
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
// The ordering is declared BEFORE the consumer is registered, and the order of
// these two calls is load-bearing. Each takes the cache lock on its own, so the
// other order leaves a window in which the consumer is registered and reads as
// FIFO, and one cumulative ack landing in that window evicts an entry this
// consumer has not handled. Marked-but-unregistered is the harmless half: the
// registration is what UnregisterConsumer and the walk key on.
func (a *reactorAPIAdapter) RegisterCacheConsumer(name string, unordered bool) {
	if unordered {
		a.r.recentUpdates.setConsumerUnordered(name)
	}
	a.r.recentUpdates.RegisterConsumer(name)
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
		//
		// This function has no production caller: applyFactsNextHop
		// (peer_forward_facts.go) is the live egress rail, and it decides the
		// link-local against RFC 2545 Section 3's shared-subnet condition
		// (applyLinkLocalNextHop, link_scope.go). The branch below reads the
		// config leaf alone, which Section 3 does not permit, so it must not be
		// wired to a peer without taking that condition first.
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
