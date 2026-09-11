// RFC: rfc/short/rfc4271.md
// Design: docs/architecture/core-design.md — NLRI batch announce/withdraw and wire attribute building
// Overview: reactor_api.go — API command handling core
// Related: reactor_api_forward.go — forwarding and grouped sending
// Related: update_group.go — cross-peer UPDATE grouping index
package reactor

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// announceFacts is every per-peer fact that decides the bytes ONE peer receives
// from an announce. It is the argument set of buildBatchAnnounceUpdate AND the
// key that puts two peers on one build, and those being the same struct is the
// point: a new per-peer wire decision reaches the builder only by becoming a
// field here, and a field here is in the key by construction.
//
// The two used to be separate lists, and the key was kept in step by
// remembering. Every field below arrived in its own commit, and three of them
// arrived as a FIX rather than with the feature they belong to (nextHop,
// addPath, propagatePrefixSID; only rsClient landed with its feature). A fact
// the key had not learned put two peers who differ in it into one group: one
// UPDATE was built and every member was sent it, so one peer received another
// peer's bytes, with Go map iteration order deciding whose. Nothing went red,
// because the UPDATE is well formed and the peer that built first is correct.
// ai/rules/principles.md: "a new feature MUST register itself and be
// discovered; it MUST NOT require an edit to a switch, a case, a factory, a
// field list, or any other central enumeration."
//
// It is comparable and holds no pointer, no slice and no string, so it stays a
// cheap map key on the announce path (ai/rules/performance.md). A per-peer fact
// that could not be a map key could not partition the groups either, so the
// constraint is the rail's rather than this type's.
//
// What is NOT a field here is the other half of the decision. attrBuf, nlriBuf
// and the NLRIBatch stay parameters of the builder because they are PER-BATCH:
// one pooled buffer pair is reused for every build in the fan-out, and every
// peer is offered the same batch, so none of the three can tell two peers apart.
type announceFacts struct {
	// nextHop is what resolveNextHop answered for this peer, and it is the
	// NEXT_HOP or the MP_REACH_NLRI next hop the builder writes.
	nextHop netip.Addr
	// isIBGP partitions the groups: RFC 4271 Section 5.1.2 forbids the AS_PATH
	// prepend toward an internal peer, and Section 5.1.5 owes that peer a
	// LOCAL_PREF an external peer must not be sent.
	isIBGP bool
	// rsClient partitions the groups: RFC 7947 S2.2.2.1 suppresses the AS_PATH
	// prepend for RS-clients, so an RS-client and an ordinary eBGP peer no
	// longer produce identical wire and must not share a built UPDATE.
	rsClient bool
	// propagatePrefixSID partitions the groups: RFC 8669 Section 8 removes the
	// Prefix-SID toward an external peer the operator has not placed inside the
	// SR domain, so two external peers that answer it differently no longer
	// produce identical wire and must not share a built UPDATE. It is the
	// operator's leaf rather than the answer, which is what the four rails in
	// forward_prefix_sid.go carry too. An internal peer keeps the attribute
	// whatever the leaf says, so two internal peers that differ only here build
	// the same bytes twice; the leaf has no meaning on an internal session, and
	// paying one build for that is cheaper than a key field that says something
	// other than what the builder is given.
	propagatePrefixSID bool
	// prepend partitions the groups: RFC 7705 Section 3.3 puts a SECOND AS
	// number in front of a route bound for a peer carrying a local-as override
	// with no "Replace Old AS", so two peers sharing a local AS but differing
	// on that option no longer produce identical wire and must not share a
	// built UPDATE. Keying on the AS number alone let them share it, which is
	// how the announce rail gave every configuration the replace-as bytes.
	prepend localASPrepend
	// addPath partitions the groups: RFC 7911 Section 3 puts a four-octet path
	// identifier in front of every NLRI toward a peer that negotiated ADD-PATH,
	// so the NLRI section itself differs. The send reads it as well, because an
	// NLRI's length on the wire is what the splitter walks.
	addPath bool
	// asn4 partitions the groups: RFC 6793 Section 4.2.2 sends an OLD speaker
	// AS_TRANS in the AS_PATH and the real AS numbers behind it in an AS4_PATH,
	// where a NEW speaker is sent neither.
	asn4 bool
	// extended is the one field the BUILDER is not given a use for, and it is
	// here because a group is one build AND one send. RFC 8654 raises this
	// peer's maximum message size from 4096 to 65535 octets, and
	// sendUpdateWithSplit takes that size as its split point. Two peers
	// differing only here are handed one UPDATE and cut it into a different
	// number of messages, so the bytes each one receives still differ and the
	// group must still be partitioned. It is passed to the builder unread
	// rather than kept beside the key, because a second list beside this one is
	// the defect this type exists to remove.
	extended bool
	// groupUpdates partitions the groups: `behavior { group-updates false }`
	// asks for one UPDATE per NLRI toward this peer, so one batch of several
	// prefixes leaves as several messages where a grouping peer is sent one.
	// The two receive a different number of frames from the same batch, so
	// they must not share a build. nlriUnitLen owns the framing itself, and
	// every rail that sends a batch reads it there.
	groupUpdates bool
}

// announceFactsFor reads one peer's answer to every fact above. It is the only
// construction on this rail, so a new field is populated in one place and
// reaches the builder and the group key together.
//
// isIBGP is passed in rather than read here: the caller already read it for the
// queue branch, and a dynamic peer can resolve its ASN between two reads
// (Peer.IsIBGP is guarded for that reason).
func announceFactsFor(peer *Peer, fam family.Family, nextHop netip.Addr, isIBGP bool, nc *NegotiatedCapabilities) announceFacts {
	settings := peer.Settings()
	return announceFacts{
		nextHop:            nextHop,
		isIBGP:             isIBGP,
		rsClient:           settings.RSClient,
		propagatePrefixSID: settings.PropagateSRv6PrefixSID,
		prepend:            localASPrependFor(settings),
		addPath:            peer.addPathFor(fam),
		asn4:               peer.asn4(),
		extended:           nc.ExtendedMessage,
		groupUpdates:       settings.GroupUpdates,
	}
}

// nlriUnitLen is how many NLRIs of one batch a single UPDATE carries toward one
// peer, and it is the ONE place `behavior { group-updates <bool> }` becomes
// framing. Every rail that sends a batch reads it here -- the announce, the
// withdraw and the LLGR readvertise -- so none of them can answer the leaf
// differently. Two rails answering one question separately is what let an
// operator-supplied LOCAL_PREF cross an AS boundary until 2026-08-01
// (buildBatchAnnounceUpdate, below), and the leaf had the same shape: the
// config-driven initial sync read it (peer_initial_sync.go) and the API rails
// did not, so `group-updates false` framed a config route one way and an
// API-announced route the other.
//
// A count below one still answers one, so the callers' framing loop runs once: a
// batch carrying no NLRI built and sent one attributes-only UPDATE before the
// loop existed, and that is not a behavior this framing decides.
func nlriUnitLen(count int, groupUpdates bool) int {
	if !groupUpdates {
		return 1
	}
	if count < 1 {
		return 1
	}
	return count
}

// announceBatchToPeers builds this batch and sends it to every peer given, which
// MUST agree on every announceFact: the facts are what the build reads, so one
// build serves all of them.
//
// facts.groupUpdates decides the FRAMING. A peer that groups updates receives one
// UPDATE carrying every NLRI of the batch; a peer whose `group-updates` leaf is
// false receives one UPDATE per NLRI. The builds are still shared across the
// peers of this set, because the leaf is a field of announceFacts and therefore
// of the group key: a peer that groups and a peer that does not cannot arrive
// here together.
//
// The two build buffers are taken once and reused by every unit. INVARIANT:
// sendUpdateWithSplit is synchronous -- it blocks until the bytes are written to
// TCP -- so a unit is on the wire before the next unit overwrites the buffer its
// *message.Update referenced. An asynchronous write would be a use-after-return
// here and in every caller that hands one build to several peers.
//
// Each peer's Adj-RIB-Out decides what it is actually sent. RFC 4271 Section
// 9.2: "A BGP speaker SHOULD NOT advertise a given feasible BGP route from its
// Adj-RIB-Out if it would produce an UPDATE message containing the same BGP
// route as was previously advertised." So a peer that already holds every prefix
// of a unit, with exactly the bytes this build produced, is sent nothing; a peer
// that holds some of them is served from a build of its own over the rest. Every
// peer of the group still shares the one build in the case the table exists to
// leave alone, which is the case where nothing is suppressed (adj_rib_out.go).
//
// The count returned is ACCEPTANCES: one for each UPDATE written, and one for
// each peer that needed none because it already held the batch. The callers read
// only whether it is zero, and a zero would say "no peer carries this family",
// which is untrue of a peer that has the route already.
func (a *reactorAPIAdapter) announceBatchToPeers(peers []*Peer, batch bgptypes.NLRIBatch, facts announceFacts) (int, error) {
	maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, facts.extended))

	attrHandle := getBuildBuf()
	nlriHandle := getBuildBuf()
	defer putBuildBuf(attrHandle)
	defer putBuildBuf(nlriHandle)

	sent := 0
	var lastErr error
	unitLen := nlriUnitLen(len(batch.NLRIs), facts.groupUpdates)

	// partial holds the peers whose Adj-RIB-Out suppresses SOME of this unit's
	// prefixes. It stays nil in the common case: a unit is one prefix whenever
	// `group-updates false`, so a peer is either sent it or is not.
	var partial []*Peer

	for off := 0; ; off += unitLen {
		end := min(off+unitLen, len(batch.NLRIs))
		unit := batch
		unit.NLRIs = batch.NLRIs[off:end]

		update, buildErr := a.buildBatchAnnounceUpdate(attrHandle.Buf, nlriHandle.Buf, unit, facts)
		if update == nil {
			// Build rejected (already logged). Every unit carries the same
			// attributes and the same facts, so the refusal repeats: stop here
			// rather than send part of the batch, and report the builder's own
			// reason instead of a silent drop.
			return sent, buildErr
		}

		built := newAnnounceUnit(update, nlriHandle.Buf, unit, facts)
		partial = partial[:0]

		for _, peer := range peers {
			held := built.heldBy(peer)
			if held == len(unit.NLRIs) {
				peer.adjOut.recordSuppressed(held)
				logAnnounceSuppressed(peer, unit, held)
				sent++
				continue
			}
			if held > 0 {
				partial = append(partial, peer)
				continue
			}
			if err := peer.sendUpdateWithSplit(update, maxMsgSize, facts.addPath); err != nil {
				lastErr = err
				continue
			}
			built.recordAll(peer)
			sent++
		}

		if len(partial) > 0 {
			n, err := a.announcePartialToPeers(partial, &built, maxMsgSize)
			sent += n
			if err != nil {
				lastErr = err
			}
		}

		if end >= len(batch.NLRIs) {
			break
		}
	}
	return sent, lastErr
}

// AnnounceNLRIBatch announces a batch of NLRIs with shared attributes.
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families.
// RFC 8654: Respects peer's max message size (4096 or 65535).
//
// sender is who makes the announce: an attached process, or the operator. It
// is gated on `send [ update ]`: this rail originates routes, which
// is the permission an operator grants when they write that word.
func (a *reactorAPIAdapter) AnnounceNLRIBatch(sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) error {
	a.r.mu.RLock()
	peers, permErr := a.getMatchingPeersSel(sel, announceOrigin(sender))
	a.r.mu.RUnlock()
	if permErr != nil {
		return permErr
	}
	if len(peers) == 0 {
		return route.ErrNoPeersMatch
	}

	// Build attributes for RIB route (used for queueing non-established peers)
	// Prefer Wire (forwarding) over Attrs (builder) when available
	var attrs []attribute.Attribute
	// The caller's AS_PATH is kept as the whole attribute, every segment intact.
	// It used to be flattened to Segments[0].ASNs, which the queue path then
	// re-encoded as ONE AS_SEQUENCE: an AS_SET (RFC 4271 Section 5.1.2, produced by
	// aggregation) silently became a sequence, and any segment after the first was
	// dropped. The established path copies the block verbatim, so the same route
	// carried a different AS_PATH depending only on whether the destination peer
	// had finished its initial sync -- and a flattened AS_SET misstates path length
	// for best-path selection (Section 9.1.2.2) and loop detection.
	var userASPath *attribute.ASPath

	switch {
	case batch.Wire != nil:
		// Parse attributes from wire format
		var err error
		attrs, err = batch.Wire.All()
		if err != nil {
			return fmt.Errorf("failed to parse batch attributes: %w", err)
		}
		// Extract AS_PATH if present
		if asPathAttr, err := batch.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				userASPath = asp
			}
		}
	case batch.Attrs != nil:
		// Use Builder for new routes
		attrs = batch.Attrs.ToAttributes()
		if asns := batch.Attrs.ASPathSlice(); len(asns) > 0 {
			// The Builder models an AS_PATH as one flat AS_SEQUENCE, so there is
			// nothing to lose here; wrapping it keeps one shape for both sources.
			userASPath = &attribute.ASPath{
				Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: asns}},
			}
		}
	default: // no attributes provided — use defaults
		attrs = append(attrs, attribute.OriginIGP)
	}

	var lastErr error
	// acceptedCount is what went out: one for each peer whose operations were
	// queued, and one for each UPDATE written to an established peer. A peer
	// carrying `group-updates false` contributes one per NLRI, because that is
	// how many messages the batch becomes for it. Only the zero test at the end
	// reads it.
	var acceptedCount int

	// Group-aware path: when update groups are enabled, collect established
	// peers whose announceFacts are equal and build the UPDATE once per group.
	// Falls back to per-peer when disabled or when peers differ.
	type announceBuildGroup struct {
		facts announceFacts
		peers []*Peer
	}

	groupsEnabled := a.r.updateGroups != nil && a.r.updateGroups.Enabled()
	var buildGroups map[announceFacts]*announceBuildGroup

	if groupsEnabled {
		buildGroups = make(map[announceFacts]*announceBuildGroup)
	}

	for i := range peers {
		peer := peers[i]
		// Guarded: this batch-announce path runs on an API/plugin goroutine and may read a
		// dynamic peer still resolving its ASN (sibling PeerAS read at :886 is guarded too).
		isIBGP := peer.IsIBGP()

		// Resolve next-hop per peer using RouteNextHop policy. A family that
		// names no forwarding hop is not asked for one: RFC 8955 Section 4 sets
		// the FlowSpec next-hop length to zero, so there is nothing to resolve
		// and a failure to resolve it is not a reason to skip the peer
		// (family.Family.NeedsNextHop).
		nextHop, nhErr := peer.resolveNextHop(batch.NextHop, batch.Family)
		if nhErr != nil && batch.Family.NeedsNextHop() {
			routesLogger().Debug("next-hop resolution failed", "peer", peer.Settings().Address, "error", nhErr)
			continue
		}

		if !peer.shouldQueue() {
			// Check family negotiation
			nc := peer.negotiated.Load()
			if nc == nil || !nc.Has(batch.Family) {
				continue // Skip peer that doesn't support this family
			}

			// LLGR stale readvertise (RFC 9494): run the per-peer readvertise
			// egress filter so the route is kept+marked for LLGR-capable peers,
			// depreferenced for non-LLGR iBGP, or withdrawn for non-LLGR eBGP.
			// Rare (GR-expiry) path; the common Stale==0 path is untouched.
			if batch.Stale > 0 && len(a.r.readvertiseEgressFilters) > 0 {
				sent, failErr := a.sendStaleReadvertise(peer, batch, nextHop, isIBGP, nc)
				acceptedCount += sent
				switch {
				case failErr != nil:
					// The readvertise could not be carried out: a filter crashed,
					// or its modifications could not be built. Either way the peer
					// decided nothing and the family IS negotiated, so
					// ErrNoPeersAcceptedFamily would state a cause that is untrue
					// (ai/rules/cli.md) and would be downgraded to a warning on
					// that basis. failErr names which one it was.
					lastErr = failErr
				case sent == 0:
					lastErr = route.ErrNoPeersAcceptedFamily
				}
				continue
			}

			facts := announceFactsFor(peer, batch.Family, nextHop, isIBGP, nc)

			if groupsEnabled {
				// Collect peer into build group for deferred batch build.
				bg, ok := buildGroups[facts]
				if !ok {
					bg = &announceBuildGroup{facts: facts}
					buildGroups[facts] = bg
				}
				bg.peers = append(bg.peers, peer)
			} else {
				// Per-peer path (update groups disabled). peers[i:i+1] is a view
				// of the slice already in hand, so the one-peer set costs no
				// allocation on the fan-out (ai/rules/performance.md).
				n, sendErr := a.announceBatchToPeers(peers[i:i+1], batch, facts)
				acceptedCount += n
				if sendErr != nil {
					lastErr = sendErr
				}
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			// Build AS_PATH only for queue path (iBGP vs eBGP); the established
			// path builds AS_PATH inside the UPDATE wire bytes directly.
			asPath := a.buildBatchASPathAttr(userASPath, batch.OriginAS, isIBGP, peer.Settings().RSClient, localASPrependFor(peer.Settings()))
			for _, n := range batch.NLRIs {
				ribRoute := rib.NewRouteWithASPath(n, nextHop, attrs, asPath)
				peer.QueueAnnounce(ribRoute)
			}
			acceptedCount++ // Queued counts as accepted
		}
	}

	// Build once per group, send to all members. The build buffers, the framing
	// and the send invariant are announceBatchToPeers's, so a group of peers and
	// a single peer cannot be framed differently.
	for _, bg := range buildGroups {
		n, sendErr := a.announceBatchToPeers(bg.peers, batch, bg.facts)
		acceptedCount += n
		if sendErr != nil {
			lastErr = sendErr
		}
	}

	// Return warning-level error if no peers accepted (all skipped due to family).
	//
	// A rejected BUILD is the one failure that must not be downgraded here.
	// ErrNoPeersAcceptedFamily means "every matching peer was SKIPPED because it
	// does not carry this family", and DispatchNLRIGroups turns it into a warning
	// on that basis. For a batch that could not be encoded the family WAS
	// negotiated, so that cause is untrue (ai/rules/cli.md: leg 3 must
	// be TRUE) and the warning downgrade would hide a route that never went out.
	//
	// Deliberately narrow: every OTHER lastErr keeps the previous behavior. Widening
	// it to `lastErr != nil` also promoted long-standing soft cases -- a send error
	// against a peer that was still coming up -- into hard failures, which turned 19
	// functional tests red. That is a real question about how send errors should be
	// reported, but it is a separate one from this guard.
	if acceptedCount == 0 {
		// Every cause named here is a failure of THIS speaker, and collapsing
		// one into "no peer carries the family" replaces a true cause with a
		// false one that the caller then downgrades to a warning.
		// errStaleReadvertiseWithheld is the shared wrapper over the LLGR rail's
		// two, so one test covers both and a third would ride in free.
		switch {
		case errors.Is(lastErr, errAnnounceTooLarge),
			errors.Is(lastErr, errAnnounceNextHopUnencodable),
			errors.Is(lastErr, errWithdrawTooLarge),
			errors.Is(lastErr, errStaleReadvertiseWithheld):
			return lastErr
		}
		return route.ErrNoPeersAcceptedFamily
	}
	return lastErr
}

// withdrawFactsFor reads one peer's answer to every fact that decides the bytes
// it receives from a withdraw. It is the only construction on this rail, so a new
// field is populated in one place and reaches the send and the group key together.
//
// The answer is an announceFacts, and it used to be a withdrawFacts of its own
// carrying three fields. The two types were correct while a withdrawal was a bare
// MP_UNREACH_NLRI, because nothing but the framing could tell two peers apart.
// Now that a withdrawal carries the attributes the caller named
// (buildBatchWithdrawUpdate), every per-peer decision that shapes an announce's
// attribute block shapes a withdrawal's too, and a second fact set beside
// announceFacts would be a second place to forget one -- which is exactly the
// defect announceFacts exists to make unreachable.
//
// The attribute-shaping fields stay ZERO for a UNICAST withdrawal. That
// withdrawal carries no path attributes whatever the peer answers
// (buildBatchWithdrawUpdate), so no per-peer decision reaches its bytes and none
// belongs in the key: leaving them zero keeps two peers that differ only in them
// on ONE build, exactly as this rail grouped them before it carried attributes at
// all (ai/rules/performance.md).
//
// nextHop is the caller's own, not a resolved one. A withdrawal makes nothing
// reachable, so `next-hop self` has nothing to name and an unset next hop is not
// a reason to skip the peer the way an announce's is (Peer.resolveNextHop returns
// ErrNextHopUnset and AnnounceNLRIBatch skips on it). Reading it from the batch
// rather than from the peer also keeps it out of the per-peer partition.
func withdrawFactsFor(peer *Peer, batch bgptypes.NLRIBatch, isIBGP bool, nc *NegotiatedCapabilities) announceFacts {
	settings := peer.Settings()
	facts := announceFacts{
		addPath:      peer.addPathFor(batch.Family),
		extended:     nc.ExtendedMessage,
		groupUpdates: settings.GroupUpdates,
	}
	if batch.Family.SAFI == family.SAFIUnicast {
		return facts
	}
	if batch.NextHop.Policy == bgptypes.NextHopExplicit {
		facts.nextHop = batch.NextHop.Addr
	}
	facts.isIBGP = isIBGP
	facts.rsClient = settings.RSClient
	facts.propagatePrefixSID = settings.PropagateSRv6PrefixSID
	facts.prepend = localASPrependFor(settings)
	facts.asn4 = peer.asn4()
	return facts
}

// withdrawBatchFromPeers builds this batch's withdrawal and sends it to every
// peer given, which MUST agree on every fact withdrawFactsFor reads. It is the
// withdraw rail's twin of announceBatchToPeers: same pooled buffer pair, same
// framing through nlriUnitLen, same synchronous-send invariant, and the same
// DELIVERIES count.
//
// The withdrawal is what takes a route back OUT of each peer's Adj-RIB-Out, so
// an announce of the same route afterwards is sent again rather than suppressed.
// It is not itself suppressed by that table. The Adj-RIB-Out is this speaker's
// model of what the peer holds, and a withdraw command is the operator saying
// the peer must not hold it: refusing to send one because the model already says
// so would put the model's word above the operator's, and RFC 4271 Section 9.1.2
// has the receiver ignore a withdrawal for a route it does not have.
//
// ONE condition withholds a withdrawal, and it is about the connection rather
// than about the route. RFC 4271 Section 4.3: a withdrawn route "is identified
// by its destination (expressed as an IP prefix), which unambiguously identifies
// the route in the context of the BGP speaker - BGP speaker connection to which
// it has been previously advertised." A connection that has advertised nothing
// has no route for any withdrawal to name, so nothing is written to it and the
// peers it happened to are ANSWERED (route.ErrWithdrawWithheld). The third
// return is those peers, by address, in the order they were met.
func (a *reactorAPIAdapter) withdrawBatchFromPeers(peers []*Peer, batch bgptypes.NLRIBatch, facts announceFacts) (int, []string, error) {
	maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, facts.extended))

	attrHandle := getBuildBuf()
	nlriHandle := getBuildBuf()
	defer putBuildBuf(attrHandle)
	defer putBuildBuf(nlriHandle)

	// The guard is asked ONCE for each peer, above the unit loop, because it is
	// a property of the connection rather than of the framing. A peer counts as
	// SERVED: the command did what it asked for, and no route was named to it.
	writable, unarmed := splitOnAdvertised(peers)
	withheld := make([]string, 0, len(unarmed))
	var lastErr error

	if len(unarmed) > 0 {
		// Built ONCE for the whole batch: it names no route, so it does not vary
		// with the framing the unit loop below applies. It is nil when the
		// withdrawal has no attributes of its own to write. The build region is
		// the attribute buffer the unit loop reuses, and every send below returns
		// before that loop starts.
		withheldUpdate := a.buildWithheldWithdrawUpdate(attrHandle.Buf, batch, facts)
		for _, peer := range unarmed {
			peer.adjOut.recordWithheld(len(batch.NLRIs))
			logWithdrawWithheld(peer, batch)
			withheld = append(withheld, peer.Settings().Address.String())
			if withheldUpdate == nil {
				continue
			}
			if err := peer.SendUpdate(withheldUpdate); err != nil {
				lastErr = err
			}
		}
	}

	sent := len(unarmed)
	unitLen := nlriUnitLen(len(batch.NLRIs), facts.groupUpdates)

	for off := 0; ; off += unitLen {
		end := min(off+unitLen, len(batch.NLRIs))
		unit := batch
		unit.NLRIs = batch.NLRIs[off:end]

		update := a.buildBatchWithdrawUpdate(attrHandle.Buf, nlriHandle.Buf, unit, facts)
		if update == nil {
			// Build rejected (already logged). Every unit is written into the
			// same buffers under the same facts, so the refusal repeats: stop
			// here rather than withdraw part of the batch.
			return sent, withheld, errWithdrawTooLarge
		}

		for _, peer := range writable {
			// Forget first, and whatever the write does. The peer's Adj-RIB-Out
			// is a model of what it holds, and after a withdrawal that failed to
			// write it holds something this speaker can no longer name. Reading
			// that as "the peer does not have it" re-sends a route it may still
			// hold; reading it the other way would suppress one it does not.
			forgetWithdrawn(peer, unit, nlriHandle.Buf, facts.addPath)
			if err := peer.sendUpdateWithSplit(update, maxMsgSize, facts.addPath); err != nil {
				lastErr = err
				continue
			}
			sent++
		}

		if end >= len(batch.NLRIs) {
			break
		}
	}
	return sent, withheld, lastErr
}

// splitOnAdvertised partitions a fan-out into the peers a withdrawal MAY be
// written to and the peers it is withheld from.
//
// RFC 4271 Section 4.3 scopes "previously advertised" to one connection, so the
// question is asked once for each peer rather than once for each unit the batch
// is framed into. The caller's own slice is answered untouched while every peer
// is armed, which is the ordinary case and the one that must not allocate
// (ai/rules/performance.md).
func splitOnAdvertised(peers []*Peer) (writable, unarmed []*Peer) {
	for i, peer := range peers {
		if peer.hasAdvertised() {
			if unarmed != nil {
				writable = append(writable, peer)
			}
			continue
		}
		if unarmed == nil {
			writable = append(make([]*Peer, 0, len(peers)), peers[:i]...)
		}
		unarmed = append(unarmed, peer)
	}
	if unarmed == nil {
		return peers, nil
	}
	return writable, unarmed
}

// logWithdrawWithheld says which routes a peer was not withdrawn from, and why.
//
// A withdrawal withheld because the connection advertised nothing and one
// dropped because the send failed are different outcomes, and an operator
// reading a peer that received no UPDATE must be able to tell them apart
// (ai/rules/principles.md). The counter beside it is adjRIBOut.withheldCount.
func logWithdrawWithheld(peer *Peer, unit bgptypes.NLRIBatch) {
	routesLogger().Warn("withdrawal withheld: this session has advertised no route to the peer",
		"peer", peer.Settings().Address,
		"family", unit.Family,
		"routes", len(unit.NLRIs),
		"rfc", "RFC 4271 Section 4.3")
}

// WithdrawNLRIBatch withdraws a batch of NLRIs.
// RFC 4271 Section 4.3: Withdrawn Routes field.
// RFC 4760: MP_UNREACH_NLRI for non-IPv4-unicast families.
//
// sender is who makes the withdrawal: an attached process, or the operator. It
// is gated on `send [ update ]`, the same permission the announce
// takes: a withdrawal is an UPDATE, and a program allowed to put a route into a
// peer must be allowed to take it out again.
func (a *reactorAPIAdapter) WithdrawNLRIBatch(sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) error {
	a.r.mu.RLock()
	peers, permErr := a.getMatchingPeersSel(sel, announceOrigin(sender))
	a.r.mu.RUnlock()
	if permErr != nil {
		return permErr
	}
	if len(peers) == 0 {
		return route.ErrNoPeersMatch
	}

	var lastErr error
	// acceptedCount is what went out: one for each peer whose operations were
	// queued, and one for each UPDATE written to an established peer, exactly as
	// on the announce rail. Only the zero test at the end reads it.
	var acceptedCount int
	// withheld names the peers this connection had advertised nothing to, so the
	// answer can carry them (RFC 4271 Section 4.3, withdrawBatchFromPeers).
	var withheld []string

	// Group-aware path for withdraw: peers whose facts are equal produce
	// identical withdraw UPDATEs and share one build.
	type withdrawBuildGroup struct {
		facts announceFacts
		peers []*Peer
	}

	groupsEnabled := a.r.updateGroups != nil && a.r.updateGroups.Enabled()
	var wdGroups map[announceFacts]*withdrawBuildGroup

	if groupsEnabled {
		wdGroups = make(map[announceFacts]*withdrawBuildGroup)
	}

	for i := range peers {
		peer := peers[i]
		if !peer.shouldQueue() {
			// Check family negotiation
			nc := peer.negotiated.Load()
			if nc == nil || !nc.Has(batch.Family) {
				continue // Skip peer that doesn't support this family
			}

			// Guarded: this batch-withdraw path runs on an API/plugin goroutine and
			// may read a dynamic peer still resolving its ASN, exactly as the
			// announce rail's read at :306 does.
			facts := withdrawFactsFor(peer, batch, peer.IsIBGP(), nc)

			if groupsEnabled {
				wg, ok := wdGroups[facts]
				if !ok {
					wg = &withdrawBuildGroup{facts: facts}
					wdGroups[facts] = wg
				}
				wg.peers = append(wg.peers, peer)
			} else {
				// Per-peer path (update groups disabled). peers[i:i+1] is a view
				// of the slice already in hand, so the one-peer set costs no
				// allocation on the fan-out (ai/rules/performance.md).
				n, peerWithheld, sendErr := a.withdrawBatchFromPeers(peers[i:i+1], batch, facts)
				acceptedCount += n
				withheld = append(withheld, peerWithheld...)
				if sendErr != nil {
					lastErr = sendErr
				}
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			for _, n := range batch.NLRIs {
				peer.QueueWithdraw(n)
			}
			acceptedCount++ // Queued counts as accepted
		}
	}

	// Build once per group, send to all members. The build buffers, the framing
	// and the send invariant are withdrawBatchFromPeers's, so a group of peers
	// and a single peer cannot be framed differently.
	for _, wg := range wdGroups {
		n, groupWithheld, sendErr := a.withdrawBatchFromPeers(wg.peers, batch, wg.facts)
		acceptedCount += n
		withheld = append(withheld, groupWithheld...)
		if sendErr != nil {
			lastErr = sendErr
		}
	}

	// Return warning-level error if no peers accepted (all skipped due to family).
	// A rejected BUILD is reported as itself rather than downgraded to the
	// "no peer carries this family" warning, for the reason spelled out in
	// AnnounceNLRIBatch: that cause would not be true, and the downgrade would hide
	// a withdrawal that never went out.
	if acceptedCount == 0 {
		if errors.Is(lastErr, errWithdrawTooLarge) {
			return lastErr
		}
		return route.ErrNoPeersAcceptedFamily
	}
	// A withheld withdrawal is REPORTED, never swallowed: the caller turns it
	// into a warning on the command's answer and still answers `done`
	// (ai/rules/principles.md). A real send failure outranks it, because that
	// one condemns the routes rather than explaining why none were owed.
	if lastErr == nil && len(withheld) > 0 {
		return fmt.Errorf("%w: %s, peers %s", route.ErrWithdrawWithheld,
			batch.Family, textbuf.Join(withheld, ", "))
	}
	return lastErr
}

// buildBatchASPathAttr builds the AS_PATH stored on a QUEUED route, preserving
// every segment of a caller-supplied path instead of flattening it.
//
// It is the queue-side twin of announceASPathRewrite, which does the same job
// on the established rail, and it deliberately reuses that function's two
// decisions: aspathLeadsWith for "our AS is already there" (which requires the
// LEADING segment to be an AS_SEQUENCE -- RFC 4271 Section 5.1.2 case 2 prepends a
// NEW sequence in front of an AS_SET, so an AS buried in a leading AS_SET does not
// count), and ASPath.Prepend for the insert itself. The two rails therefore reach
// the same AS_PATH by construction rather than by coincidence.
//
// With no caller-supplied path it delegates to buildBatchASPath, which owns the
// synthesized shapes (origin-as, plain iBGP/eBGP export).
func (a *reactorAPIAdapter) buildBatchASPathAttr(userASPath *attribute.ASPath, originAS uint32, isIBGP, rsClient bool, prepend localASPrepend) *attribute.ASPath {
	if userASPath == nil || len(userASPath.Segments) == 0 {
		return a.buildBatchASPath(nil, originAS, isIBGP, rsClient, prepend)
	}
	// RFC 7947 Section 2.2.2.1 exempts RS-clients; Section 5.1.2 forbids touching
	// the path toward an internal peer; with no local AS there is nothing to add.
	//
	// aspathLeadsWith reads the OUTERMOST AS number alone. An operator who spelled
	// the local AS at the front of the path they supplied gets it left alone,
	// exactly as before, and a local-as peer's second AS number is not inserted
	// behind a path the operator wrote out in full.
	if isIBGP || rsClient || !prepend.owed() || aspathLeadsWith(userASPath, prepend.primary) {
		return userASPath
	}
	// Deep copy: userASPath belongs to the caller's decoded attributes and is
	// shared with every other peer in this batch, so the prepend must not mutate it.
	prepended := &attribute.ASPath{Segments: make([]attribute.ASPathSegment, len(userASPath.Segments))}
	for k, seg := range userASPath.Segments {
		asns := make([]uint32, len(seg.ASNs))
		copy(asns, seg.ASNs)
		prepended.Segments[k] = attribute.ASPathSegment{Type: seg.Type, ASNs: asns}
	}
	prepend.prependTo(prepended)
	return prepended
}

// buildBatchASPath builds AS_PATH for batch operations.
// RFC 4271 §5.1.2: iBGP SHALL NOT modify AS_PATH; eBGP prepends local AS.
// RFC 7947 §2.2.2: a route server does NOT prepend, for RS-client peers only.
func (a *reactorAPIAdapter) buildBatchASPath(userASPath []uint32, originAS uint32, isIBGP, rsClient bool, prepend localASPrepend) *attribute.ASPath {
	switch {
	case len(userASPath) > 0:
		// An operator-supplied as-path used to be emitted verbatim to EVERY peer,
		// justified as route-server transparency. RFC 7947 Section 2.2.2.1 grants
		// that transparency to RS-CLIENTS; it says nothing about ordinary peers.
		// It is a SHOULD NOT, and the RFC says so explicitly ("a recommendation
		// rather than a requirement"), deviating from RFC 4271 Section 5.1.2 only
		// for clients that cannot accept a non-adjacent leftmost AS (Section
		// 2.2.2.2). Ze takes the recommendation for RS-clients and the RFC 4271
		// requirement for everyone else.
		// For a plain eBGP peer RFC 4271 Section 5.1.2 still applies -- "the local
		// system prepends its own AS number as the last element of the sequence"
		// -- so a path that omitted our AS put a non-conformant UPDATE on the
		// wire, invisible to the receiver's loop detection.
		//
		// Prepend only when our AS is not already leading, so an operator who
		// spelled out the full path (the common case when scripting a specific
		// AS_PATH) is not double-prepended. userASPath is the caller's slice and
		// is never mutated.
		asns := userASPath
		if !isIBGP && !rsClient && asns[0] != prepend.primary {
			prefixed := make([]uint32, 0, len(asns)+2)
			prefixed = prepend.asns(prefixed)
			asns = append(prefixed, asns...)
		}
		return &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: asns},
			},
		}
	case originAS != 0:
		// Virtual-router origin: [originAS] on iBGP, and the local prepend in front
		// of it on eBGP -- one AS number, or two toward a local-as peer.
		asns := []uint32{originAS}
		if !isIBGP {
			asns = append(prepend.asns(make([]uint32, 0, 3)), originAS)
		}
		return &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: asns},
			},
		}
	case isIBGP:
		return &attribute.ASPath{Segments: nil}
	default: // eBGP: prepend the local AS, and the globally configured one behind it
		return &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: prepend.asns(make([]uint32, 0, 2))},
			},
		}
	}
}

// aspathLeadsWith reports whether p already begins with asn in a leading
// AS_SEQUENCE. Only that shape counts: RFC 4271 Section 5.1.2 case 2 prepends a
// NEW AS_SEQUENCE when the first segment is an AS_SET, so an asn buried inside a
// leading AS_SET does not satisfy the requirement.
func aspathLeadsWith(p *attribute.ASPath, asn uint32) bool {
	if p == nil || len(p.Segments) == 0 {
		return false
	}
	seg := p.Segments[0]
	return seg.Type == attribute.ASSequence && len(seg.ASNs) > 0 && seg.ASNs[0] == asn
}

// announceASPathRewrite returns the AS_PATH this announce must emit in place of
// the one already in the caller's block, or nil when the caller's own AS_PATH
// stands.
//
// RFC 4271 Section 5.1.2: an AS_PATH that arrived complete still has to carry OUR
// AS toward an external peer. Emitting an operator-supplied as-path unchanged ships
// a path the receiver's loop detection cannot see itself behind. RFC 7947
// Section 2.2.2.1 excuses RS-clients and Section 5.1.2 forbids touching the path
// toward an internal peer, which is what buildBatchASPathAttr already decides for
// the queued rail -- so both rails reach the same AS_PATH by construction rather
// than by coincidence.
//
// A nil return ALSO covers "the source ASN encoding is unknown". The 2- versus
// 4-octet encoding of the EXISTING path cannot be guessed: read it wrong and the
// rewrite silently corrupts AS_PATH, which is worse than the violation being fixed.
// AttributesWire.Get is not usable here -- it decodes via a REGISTERED source
// context and a builder-built block carries context 0 -- so the caller, which knows
// which mode produced the bytes, passes the answer in.
func (a *reactorAPIAdapter) announceASPathRewrite(existing *attribute.ASPath, isIBGP, rsClient, srcKnown bool, prepend localASPrepend) *attribute.ASPath {
	if existing == nil || !a.prependApplies(isIBGP, rsClient, true, prepend.primary) {
		return nil
	}
	if !srcKnown {
		routesLogger().Warn("as-path prepend skipped: source ASN encoding unknown; sending an explicit as-path unchanged violates RFC 4271 S5.1.2 toward an external peer",
			"localAS", prepend.primary)
		return nil
	}
	rewritten := a.buildBatchASPathAttr(existing, 0, isIBGP, rsClient, prepend)
	if rewritten == existing {
		return nil // already conformant; leave the operator's path alone
	}
	return rewritten
}

// prependApplies reports whether RFC 4271 Section 5.1.2 could owe a prepend at all,
// without decoding anything. It is the cheap half of announceASPathRewrite, split
// out so the announce path can skip the decode entirely on every route whose path
// it is going to keep verbatim.
func (a *reactorAPIAdapter) prependApplies(isIBGP, rsClient, srcKnown bool, localAS uint32) bool {
	return !isIBGP && !rsClient && srcKnown && localAS != 0
}

// baseASPath decodes the AS_PATH already present in a caller-supplied attribute
// block, or returns nil when there is none (or it does not decode).
func baseASPath(base []byte, srcASN4 bool) *attribute.ASPath {
	_, _, value, found := attribute.AttrFind(base, attribute.AttrASPath)
	if !found {
		return nil
	}
	existing, err := attribute.ParseASPath(value, srcASN4)
	if err != nil {
		routesLogger().Warn("as-path prepend skipped: AS_PATH did not decode",
			"srcASN4", srcASN4, "error", err)
		return nil
	}
	return existing
}

// buildBatchAnnounceUpdate builds an UPDATE message for a batch of NLRIs.
//
// The caller's attribute block is the BASE, and everything this rail contributes
// -- the mandatory attributes it must add, the authoritative NEXT_HOP or
// MP_REACH_NLRI, an iBGP LOCAL_PREF, the RFC 6793 AS4_PATH -- is an edit over it.
// announceAttrs materializes both with the same one-pass merge writer the forward
// path uses (announce_build.go), so ascending type-code order (RFC 4271 Section 5)
// and the exact output size are properties of that writer rather than of this
// function.
//
// It replaces a strip-then-merge-insert scheme local to this rail
// (findAttrInsertPosition + insertAttrOrdered, a memmove per inserted attribute
// into the caller's pooled slot) and the verbatim-copy-with-prepends that fed it
// (writeMandatoryAttrs). Both existed because this was the rail that diverged; the
// queued rail happened to be right and had neither.
//
// facts carries every PER-PEER decision (announceFacts, above). attrBuf, nlriBuf
// and batch are the per-batch half: one pooled buffer pair serves every build in
// a fan-out, and every peer is offered the same batch. Taking the per-peer half
// as ONE struct is what stops the caller's grouping key from omitting a fact
// this function reads, which is how four separate defects reached the wire.
//
// attrBuf and nlriBuf are caller-provided buffers (from getBuildBuf, session.go).
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families.
//
// Returns a nil update and the reason when the batch cannot be encoded, having
// written nothing: every query runs before the write, so a truncated, over-long
// or desynchronised block is not a state this function can produce
// (ai/rules/evidence.md). The reason is returned rather than assumed by the
// caller because the two refusals need different operator action: errAnnounceTooLarge
// asks for fewer prefixes per announce, errAnnounceNextHopUnencodable asks for a
// next hop at all.
func (a *reactorAPIAdapter) buildBatchAnnounceUpdate(attrBuf, nlriBuf []byte, batch bgptypes.NLRIBatch, facts announceFacts) (*message.Update, error) {
	// Write NLRIs into caller-provided buffer
	nlriOff := writeBatchNLRI(nlriBuf, batch.NLRIs, facts.addPath)
	if nlriOff < 0 {
		logAnnounceTooLarge(batch, len(nlriBuf), "nlri")
		return nil, errAnnounceTooLarge
	}
	nlriBytes := nlriBuf[:nlriOff]

	plan := getAnnouncePlan()
	defer putAnnouncePlan(plan)

	base := a.planBatchAttrs(plan, batch, facts)

	if batch.Family != family.IPv4Unicast {
		// RFC 4760 Section 3: every other family carries its next-hop and NLRI inside
		// MP_REACH_NLRI. A relayed/replayed block may already carry one; the
		// contribution replaces it.
		//
		// The same validity question the IPv4 NEXT_HOP branch answers in
		// planBatchAttrs, with the same cause: resolveNextHop (peer.go) hands an
		// explicit next-hop back unvalidated, the zero Addr included. What differs is
		// the remedy. The IPv4 branch can leave the base's own NEXT_HOP alone because
		// the NLRI travels in the UPDATE's own field, so the batch's prefixes still go
		// out. Here the NLRI is INSIDE the attribute, and skipping the contribution
		// would announce the BASE's prefixes in place of this batch's. So the whole
		// UPDATE is refused instead.
		// A family that names no forwarding hop carries NONE, and carrying none
		// is what makes the length octet zero. RFC 8955 Section 4: "When
		// advertising Flow Specifications, the Length of the Next-Hop Network
		// Address MUST be set to 0.  The Network Address of the Next-Hop field
		// MUST be ignored." Offering the unresolved address instead made the
		// validator below refuse every FlowSpec announce ze was asked to send.
		var nextHops []netip.Addr
		if batch.Family.NeedsNextHop() {
			nextHops = []netip.Addr{facts.nextHop}
		}
		mpReach := attribute.NewMPReachNLRI(attribute.AFI(batch.Family.AFI), attribute.SAFI(batch.Family.SAFI),
			nextHops, nlriBytes)
		if err := mpReach.ValidateNextHops(); err != nil {
			logAnnounceNextHopUnencodable(batch, facts.nextHop, err)
			return nil, errAnnounceNextHopUnencodable
		}
		plan.add(mpReach, nil)
	}

	n, ok := plan.emit(base, attrBuf)
	if !ok {
		// The plan's own refusals do not all mean oversize. announceAttrs.add is the
		// backstop for an attribute with no wire form, and it reaches this rail for a
		// contribution the branches above did not pre-check -- a filter's or a future
		// contributor's. Reporting that as errAnnounceTooLarge asked the operator to
		// send fewer prefixes on a batch whose size was never the problem
		// (ai/rules/cli.md).
		if cause := plan.refusalCause(); errors.Is(cause, attribute.ErrUnencodableNextHop) {
			logAnnounceNextHopUnencodable(batch, facts.nextHop, cause)
			return nil, errAnnounceNextHopUnencodable
		}
		logAnnounceTooLarge(batch, len(attrBuf), "attributes")
		return nil, errAnnounceTooLarge
	}

	update := &message.Update{PathAttributes: attrBuf[:n]}
	if batch.Family == family.IPv4Unicast {
		update.NLRI = nlriBytes
	}
	return update, nil
}

// planBatchAttrs plans every path attribute a batch carries EXCEPT the one that
// says which direction it travels in, and returns the caller's verbatim block for
// plan.emit to merge the plan over.
//
// The two API rails differ in exactly one attribute. An announce adds
// MP_REACH_NLRI and a withdraw adds MP_UNREACH_NLRI, and RFC 4271 Section 4.3
// gives both UPDATEs the same Path Attributes field otherwise. So the mandatory
// synthesis, the AS_PATH prepend, the LOCAL_PREF obligation and prohibition, the
// legacy NEXT_HOP and the Prefix-SID strip are planned HERE, once, and neither
// rail can answer one of those questions differently from the other. Answering
// them separately is what let an operator's LOCAL_PREF cross an AS boundary until
// 2026-08-01, and what left the withdraw rail emitting a bare MP_UNREACH_NLRI for
// a command that named attributes until 2026-09-06.
//
// facts carries every PER-PEER decision (announceFacts, above), so a fact this
// function reads is a fact the caller's grouping key holds.
func (a *reactorAPIAdapter) planBatchAttrs(plan *announceAttrs, batch bgptypes.NLRIBatch, facts announceFacts) []byte {
	dstCtx := announceDstCtx(facts.asn4)

	// The BASE is the caller's verbatim attribute block. A Builder without the
	// raw-wire escape hatch has no block at all: its attributes are contributions
	// over an EMPTY base, which is the whole point of retiring the Builder's own
	// encoder.
	var base []byte
	var builderAttrs []attribute.Attribute
	var builderScratch [attribute.BuilderInlineAttrs]attribute.Attribute
	srcASN4, srcKnown := true, true

	switch {
	case batch.Wire != nil:
		base = batch.Wire.Packed()
		srcASN4, srcKnown = false, false
		if ctx := bgpctx.Registry.Get(batch.Wire.SourceContext()); ctx != nil {
			srcASN4, srcKnown = ctx.ASN4(), true
		}
	case batch.Attrs != nil:
		base = batch.Attrs.RawWire()
		if base == nil {
			builderAttrs = batch.Attrs.AppendAttributes(builderScratch[:0])
		}
		// A Builder writes 4-octet ASNs.
	}

	hasCode := func(code attribute.AttributeCode) bool {
		if plan.planned(uint8(code)) {
			return true
		}
		if _, _, _, found := attribute.AttrFind(base, code); found {
			return true
		}
		for _, attr := range builderAttrs {
			if attr.Code() == code {
				return true
			}
		}
		return false
	}

	// The AS_PATH the announce emits. A caller-supplied one is kept in the caller's
	// own encoding unless RFC 4271 Section 5.1.2 owes a prepend, in which case the
	// rewritten path is re-encoded toward the destination.
	//
	// Presence is answered without decoding. Only the prepend needs the decoded
	// path, and prependApplies settles every reason it cannot apply first: parsing
	// on a route that will keep its path verbatim -- every internal peer, every
	// RS-client -- is an allocation per route for an answer nobody reads.
	_, _, _, hadASPath := attribute.AttrFind(base, attribute.AttrASPath)
	if !hadASPath && batch.Attrs != nil {
		hadASPath = batch.Attrs.ToASPath() != nil
	}
	var rewrittenASPath *attribute.ASPath
	if hadASPath && a.prependApplies(facts.isIBGP, facts.rsClient, srcKnown, facts.prepend.primary) {
		existingASPath := baseASPath(base, srcASN4)
		if existingASPath == nil && batch.Attrs != nil {
			existingASPath = batch.Attrs.ToASPath()
		}
		rewrittenASPath = a.announceASPathRewrite(existingASPath, facts.isIBGP, facts.rsClient, srcKnown, facts.prepend)
	}

	// RFC 4271 Section 5.1.5, the prohibition half. localPrefAllowedTo
	// (forward_local_pref.go) owns the answer and the confederation exception, so
	// this rail and the two forward rails cannot disagree about it -- which they
	// did until 2026-08-04, when only this one stripped.
	localPrefAllowed := localPrefAllowedTo(facts.isIBGP)

	// The Builder's attributes, in the ascending order AppendAttributes declares.
	for _, attr := range builderAttrs {
		if attr.Code() == attribute.AttrASPath && rewrittenASPath != nil {
			continue // the prepended path replaces it below, under the destination context
		}
		if attr.Code() == attribute.AttrLocalPref && !localPrefAllowed {
			continue // Section 5.1.5, above
		}
		plan.add(attr, nil)
	}
	if rewrittenASPath != nil {
		plan.add(rewrittenASPath, dstCtx)
	}

	// RFC 4271 Section 5.1.1 and Section 5.1.2: ORIGIN and AS_PATH are well-known
	// mandatory. Synthesize whichever the caller did not supply.
	if !hasCode(attribute.AttrOrigin) {
		plan.add(attribute.OriginIGP, nil)
	}
	if !hadASPath {
		// Three AS numbers at most: the local-as prepend contributes two toward a
		// migrating peer, and an origin-as adds one behind them.
		var scratch [3]uint32
		asns := announceASPathASNs(scratch[:0], facts.isIBGP, facts.prepend, batch.OriginAS)
		synth := plan.asPathFor(asns)
		plan.add(synth, dstCtx)

		// RFC 6793 Section 4.2.2: a NEW speaker whose two-octet AS_PATH had to carry
		// AS_TRANS MUST also send the four-octet AS4_PATH. Derived from the SAME
		// announceASPathASNs sequence the AS_PATH above encoded, so the two cannot
		// disagree. Only when this rail synthesized the path: a verbatim AS_PATH owns
		// its own encoding.
		if !facts.asn4 && anyNonMappableAS(asns) {
			plan.add(plan.as4PathFor(synth.Segments), nil)
		}
	}

	switch {
	case batch.Family == family.IPv4Unicast:
		// Write exactly one NEXT_HOP, the authoritative resolved address. The base may
		// already carry one -- a relayed/replayed route stores the full received block,
		// NEXT_HOP included -- and a contribution REPLACES it rather than adding a
		// second, which FRR and others treat as a withdraw (RFC 7606 Section 3(g)).
		//
		// Guard on validity and fail closed. resolveNextHop (peer.go) does NOT validate
		// an explicit next-hop -- it deliberately returns whatever Addr was configured,
		// invalid included (see TestResolveNextHop_ExplicitInvalid) -- and an invalid
		// Addr encodes as a zero-LENGTH NEXT_HOP value (attribute/simple.go). If
		// the next hop is invalid, leave the base's own NEXT_HOP alone rather than replace a
		// good address with a malformed one.
		if facts.nextHop.IsValid() {
			plan.add(plan.nextHopFor(facts.nextHop), nil)
		}
	case legacyNextHopApplies(batch.Family, facts.nextHop):
		// The multiprotocol families that ALSO restate an IPv4 next hop as the
		// RFC 4271 Section 5.1.3 attribute (family.Family.LegacyNextHop). Ze's config
		// rail already sends it -- message.(*UpdateBuilder).BuildVPN, nlri/mvpn and
		// nlri/mup each add code 3 for an IPv4 next hop, and 42 `conf-*` fixtures pin
		// it -- while this rail sent MP_REACH_NLRI alone, so the same route reached
		// the wire as two different byte strings depending on whether an operator
		// configured it or announced it. RFC 4760 Section 3 makes both conformant
		// (SHOULD NOT), so the rails agreeing is what the choice is for.
		plan.add(plan.nextHopFor(facts.nextHop), nil)
	}

	// RFC 4271 Section 5.1.5, the obligation half: LOCAL_PREF SHALL be included in
	// every UPDATE toward an internal peer, so one is synthesized when the caller
	// supplied none. A caller-supplied value wins.
	//
	// The prohibition half is the strip. Until 2026-08-01 this rail only ever ADDED
	// the attribute: the caller's block was copied verbatim, so an operator-supplied
	// local-preference crossed the AS boundary on every announce to an external peer
	// that had finished its initial sync. The queued rail writes LOCAL_PREF only
	// under `if isIBGP` and was right; this makes the two agree by construction
	// rather than leaving the answer to Peer.shouldQueue.
	switch {
	case localPrefAllowed:
		if !hasCode(attribute.AttrLocalPref) {
			plan.add(attribute.LocalPref(100), nil)
		}
	default:
		if _, _, _, found := attribute.AttrFind(base, attribute.AttrLocalPref); found {
			plan.drop(uint8(attribute.AttrLocalPref))
		}
	}

	// RFC 8669 Section 8: "The propagation to other ASes MUST be explicitly
	// configured." prefixSIDAllowedTo (forward_prefix_sid.go) owns the answer, and
	// asking it here is what stops this rail from disagreeing with the four that
	// already asked -- the disagreement LOCAL_PREF above cost this same function in
	// August. propagatePrefixSID is the operator's leaf, so an internal destination
	// keeps the attribute whatever the leaf says, by construction rather than by
	// the caller getting it right.
	//
	// Only the BASE can carry a Prefix-SID on this rail: nothing above contributes
	// code 40, and a Builder has no setter for it, so a Builder's Prefix-SID can
	// only arrive as pre-encoded wire in RawWire, which IS the base
	// (attribute.Builder.AppendAttributes). The presence test keeps a destination
	// that was never sent one off the plan, exactly as the LOCAL_PREF strip does.
	if !prefixSIDAllowedTo(facts.isIBGP, facts.propagatePrefixSID) {
		if _, _, _, found := attribute.AttrFind(base, attribute.AttrPrefixSID); found {
			plan.drop(uint8(attribute.AttrPrefixSID))
		}
	}

	return base
}

// legacyNextHopApplies reports whether this batch restates its next hop as the
// RFC 4271 Section 5.1.3 NEXT_HOP attribute beside MP_REACH_NLRI or
// MP_UNREACH_NLRI. The family answers whether the attribute belongs
// (family.Family.LegacyNextHop); the address answers whether one can be written.
//
// RFC 4271 Section 6.3: "Syntactic correctness means that the NEXT_HOP attribute
// represents a valid IP host address." The unspecified address is not one, and a
// receiver that finds it MUST answer with an Invalid NEXT_HOP Attribute
// NOTIFICATION, so `next-hop 0.0.0.0` -- which an operator writes to say a
// withdrawal names no next hop at all -- contributes nothing rather than
// resetting the session.
func legacyNextHopApplies(fam family.Family, nextHop netip.Addr) bool {
	if !nextHop.Is4() {
		return false
	}
	if nextHop.IsUnspecified() {
		return false
	}
	return fam.LegacyNextHop()
}

// writeBatchNLRI writes every NLRI of a batch into nlriBuf, in order, and returns
// the bytes written -- or -1, having written nothing past the last whole NLRI,
// when they do not all fit.
//
// The bound is the other half of the announce writer's region bound. Both build buffers come
// from getBuildBuf, so both are backing[off:off+4096] out of a 128-slot slab whose
// CAP runs into the next peer's buffer (session.go); but where an oversize
// ATTRIBUTE block silently walked into the neighbor, an oversize NLRI block took
// the daemon down: nlri.WriteNLRI ends in an index expression (INET.WriteTo writes
// buf[pos] directly, and WriteNLRI's ADD-PATH path-id is a PutUint32), so it
// panics at len rather than clamping at cap. 250 IPv6 /128s or 820 IPv4 /32s in
// one `update ... nlri add` is enough, and nothing upstream caps the count --
// parseWireAttrSection (plugins/cmd/update/update_wire.go) bounds neither the
// token count nor the hex length.
//
// LenWithContext is the size WriteNLRI writes: for INET it is the same
// 1+PrefixBytes(bits) (+4 with ADD-PATH), and for WireNLRI it is len(data) either
// way. TestNLRILenMatchesWriteNLRI pins that identity, because a Len() that
// under-reports what WriteTo writes would re-open the panic through this guard.
func writeBatchNLRI(nlriBuf []byte, nlris []nlri.NLRI, addPath bool) int {
	off := 0
	for _, n := range nlris {
		need := nlri.LenWithContext(n, addPath)
		if need < 0 || off+need > len(nlriBuf) {
			return -1
		}
		off += nlri.WriteNLRI(n, nlriBuf, off, addPath)
	}
	return off
}

// errAnnounceTooLarge is what a caller reports when buildBatchAnnounceUpdate could
// not encode the batch into its pooled build buffer.
var errAnnounceTooLarge = errors.New("announce attributes exceed the build buffer; split the batch into smaller announcements")

// errAnnounceNextHopUnencodable is what a caller reports when the resolved next
// hop has no MP_REACH_NLRI wire form, so the batch's own prefixes cannot be
// carried at all (attribute.ErrUnencodableNextHop).
//
// Separate from errAnnounceTooLarge because the operator action differs: nothing
// about the batch size would help, and the next hop is what must change
// (ai/rules/cli.md).
//
// The text names UNRESOLVED, which is exactly what attribute.ValidateNextHops
// tests, and no more. An earlier wording said "configure a next-hop the family can
// carry", which claimed a family check nobody wrote: a VALID IPv4 address on an
// IPv6 batch passes ValidateNextHops and encodes four octets under AFI 2, a length
// RFC 2545 Section 3 does not define. Closing that needs the valid next-hop lengths
// to be data each NLRI family registers instead of the central switch
// attribute.ValidNextHopLens is today, so the message was narrowed to the truth
// rather than the check widened onto that switch (ai/rules/cli.md: leg 3 must be
// TRUE).
var errAnnounceNextHopUnencodable = errors.New("announce next-hop is unresolved and has no wire form; set a next-hop for this announcement")

// errStaleReadvertiseWithheld is the shared cause of an LLGR stale re-advertise
// (RFC 9494) that this speaker could not carry out for a destination peer. The
// route is withheld fail-closed: what the readvertise decision turns on is the
// destination's LLGR capability, and nothing may be advertised on a guess at it.
//
// Separate from route.ErrNoPeersAcceptedFamily because that cause is untrue here
// -- the family IS negotiated -- and callers downgrade it to a warning on the
// strength of it (ai/rules/cli.md, and ai/rules/evidence.md: a guard that cannot
// deny must speak).
//
// Never returned bare. The two wrapped errors below name what actually happened,
// because the operator action differs, and one errors.Is on this sentinel still
// catches the pair.
var errStaleReadvertiseWithheld = errors.New("stale re-advertise withheld the route")

// errStaleReadvertiseFilterPanic: a readvertise egress filter panicked, so
// nothing was decided for this peer. A plugin bug.
var errStaleReadvertiseFilterPanic = fmt.Errorf("%w: an egress filter panicked", errStaleReadvertiseWithheld)

// errStaleReadvertiseBuildFailed: a filter decided, and buildModifiedPayload
// could not encode the modifications it asked for. Not a filter failure -- the
// depreferenced UPDATE body is what could not be produced. The build's own named
// reason is counted and logged at its producer (recordModifyFailureAddr); this
// error is the caller-facing half.
var errStaleReadvertiseBuildFailed = fmt.Errorf("%w: the modified UPDATE body could not be built", errStaleReadvertiseWithheld)

// errWithdrawTooLarge is the withdraw-rail sibling: buildBatchWithdrawUpdate could
// not encode the batch's NLRIs (or the MP_UNREACH_NLRI carrying them) into its
// pooled build buffer. Separate from errAnnounceTooLarge so the operator-facing
// cause names the operation that actually failed (ai/rules/cli.md).
var errWithdrawTooLarge = errors.New("withdraw NLRIs exceed the build buffer; split the batch into smaller withdrawals")

// logAnnounceTooLarge records a rejected announce. This is the "or say something"
// half of the announce writer's fail-closed guard: the build is abandoned
// rather than truncated, so without this line an operator would see routes simply
// not arrive. The plugin that issued the command also sees it -- AnnounceNLRIBatch
// returns errAnnounceTooLarge, which DispatchNLRIGroups turns into a StatusError
// response -- so the failure is observable from both ends
// (ai/rules/evidence.md, ai/rules/cli.md).
func logAnnounceTooLarge(batch bgptypes.NLRIBatch, bufLen int, stage string) {
	// Counted as well as logged. The route not arriving is the whole symptom, and
	// a Warn line is not something an operator can alert on
	// (announce_metrics.go).
	recordAnnounceDroppedOversize(announceRailBatch, stage)
	routesLogger().Warn("announce rejected: attributes do not fit the build buffer",
		"family", batch.Family, "nlri-count", len(batch.NLRIs),
		"buffer-bytes", bufLen, "stage", stage,
		"action", "route not sent to this peer; send fewer prefixes per announce")
}

// logAnnounceNextHopUnencodable records an announce refused because its next hop
// has no MP_REACH_NLRI wire form.
//
// This is the "or say something" half of the second fail-closed guard on the
// batch rail. The build is abandoned before a single octet is written, so no
// desynchronised attribute leaves this speaker; without this line the route would
// simply not arrive. The plugin that issued the command sees it too, because
// AnnounceNLRIBatch returns errAnnounceNextHopUnencodable and DispatchNLRIGroups
// turns that into a StatusError response (ai/rules/evidence.md, ai/rules/cli.md).
func logAnnounceNextHopUnencodable(batch bgptypes.NLRIBatch, nextHop netip.Addr, cause error) {
	routesLogger().Warn("announce rejected: next-hop is unresolved and has no wire form",
		"family", batch.Family, "nlri-count", len(batch.NLRIs),
		"next-hop", nextHop, "error", cause,
		"action", "route not sent to this peer; set a next-hop for this announcement")
}

// announceASPathASNs appends to dst, and returns, the AS_PATH ASN sequence this
// builder synthesizes for an announce that does not carry a verbatim AS_PATH:
//
//   - origin-as (originAS != 0): [originAS] for iBGP, the local prepend then
//     originAS for eBGP -- the normal export rule, so a real eBGP peer sees a
//     well-formed first AS (enforce-first-as).
//   - plain export (originAS == 0): empty for iBGP, the local prepend for eBGP.
//
// The local prepend is ONE AS number for an ordinary peer and TWO toward a peer
// carrying a local-as override with no "Replace Old AS", which is RFC 7705
// Section 3.3: the speaker "SHOULD first append the globally configured ASN to
// the AS_PATH immediately followed by the 'Local AS' value". A route ze
// ORIGINATES is the case this rail owns, and until 2026-09-05 it prepended the
// override alone, so `local-as` behaved as `replace-as` here and the two options
// were indistinguishable on this rail exactly as they had been on the forward
// one.
//
// dst is normally a stack-allocated scratch array (the sequence is at most three
// ASNs) so the announce path stays allocation-free. This is the single source of
// the synthesized AS_PATH shape, shared by writeASPath (which two-octet-encodes
// it, mapping a non-mappable AS to AS_TRANS) and writeAnnounceAS4Path (which
// four-octet-encodes the same sequence into an AS4_PATH when that mapping happens
// toward an OLD peer), so the AS_PATH and AS4_PATH can never disagree.
func announceASPathASNs(dst []uint32, isIBGP bool, prepend localASPrepend, originAS uint32) []uint32 {
	if originAS != 0 {
		if !isIBGP {
			dst = prepend.asns(dst)
		}
		return append(dst, originAS)
	}
	if isIBGP {
		return dst // empty AS_PATH
	}
	return prepend.asns(dst)
}

// buildBatchWithdrawUpdate builds an UPDATE message for withdrawing a batch of NLRIs.
// attrBuf and nlriBuf are caller-provided buffers (from getBuildBuf, session.go).
// RFC 4271 Section 4.3: Withdrawn Routes field.
// RFC 4760: MP_UNREACH_NLRI for non-IPv4-unicast families.
//
// The withdrawal carries the attributes the CALLER named, and UNICAST carries
// none. RFC 4760 Section 4: "An UPDATE message that contains the MP_UNREACH_NLRI
// is not required to carry any other path attributes", so both shapes are
// conformant and the family decides which one this speaker sends.
//
// Until 2026-09-06 every withdrawal took the bare shape, so
// `send bgp <selector> update text <attributes> nlri <family> del <nlri>`
// acknowledged attributes that never reached the wire, and an operator could not
// tell that from having asked for none (ai/rules/principles.md).
//
// Unicast is bare because RFC 4271 Section 4.3 gives the Withdrawn Routes field
// no attributes, and the IPv6 unicast withdrawal is the SAME withdrawal in the
// RFC 4760 encoding. Sending the two different blocks would make the AFI decide
// what an operator's `withdraw <prefix> next-hop X local-preference 200` means.
// ExaBGP splits them at the same seam, and the ported fixtures pin both halves:
// api-ipv6.ci withdraws an IPv6 unicast prefix that named a next hop and a
// local-preference and carries nothing, while api-flow.ci withdraws a FlowSpec
// rule that named neither and carries ORIGIN, AS_PATH and LOCAL_PREF.
//
// What every other family carries is planBatchAttrs's answer, which is the
// announce rail's answer: the caller's block, plus the RFC 4271 Section 4.3
// well-known mandatory ORIGIN and AS_PATH, plus the Section 5.1.5 LOCAL_PREF
// toward an internal peer, plus the legacy NEXT_HOP where the family carries one.
//
// Returns nil when the batch does not fit its pooled build buffers, exactly as
// buildBatchAnnounceUpdate does. The withdraw rail carried the SAME two unbounded
// writes the announce rail did: an NLRI loop that panics past len (WriteNLRI ends
// in an index expression), and an MP_UNREACH_NLRI whose declared length comes from
// Len() while its value copy clamps, so an oversize batch produced an attribute
// claiming more octets than it contained. A short withdraw is not a lesser failure
// than a short announce -- the peer keeps forwarding to prefixes it was never told
// about.
func (a *reactorAPIAdapter) buildBatchWithdrawUpdate(attrBuf, nlriBuf []byte, batch bgptypes.NLRIBatch, facts announceFacts) *message.Update {
	// Write NLRIs into caller-provided buffer
	nlriOff := writeBatchNLRI(nlriBuf, batch.NLRIs, facts.addPath)
	if nlriOff < 0 {
		logWithdrawTooLarge(batch, len(nlriBuf), "nlri")
		return nil
	}
	nlriBytes := nlriBuf[:nlriOff]

	if batch.Family == (family.IPv4Unicast) {
		// IPv4 unicast: Use WithdrawnRoutes field, which RFC 4271 Section 4.3 gives
		// no attributes of its own. api-healthcheck-withdraw.ci pins the empty block
		// for a withdrawal that named `next-hop self`.
		return &message.Update{
			WithdrawnRoutes: nlriBytes,
		}
	}

	// Non-IPv4 unicast: Use MP_UNREACH_NLRI (RFC 4760)
	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(batch.Family.AFI),
		SAFI: attribute.SAFI(batch.Family.SAFI),
		NLRI: nlriBytes,
	}

	if batch.Family.SAFI == family.SAFIUnicast {
		// IPv6 unicast: the same withdrawal the IPv4 branch above writes into the
		// Withdrawn Routes field, so it carries the same attributes -- none. This is
		// also the cheapest shape, and it is the one every large withdrawal takes
		// (ai/rules/performance.md).
		if attribute.AttrWireLen(mpUnreach) > len(attrBuf) {
			logWithdrawTooLarge(batch, len(attrBuf), "mp-unreach")
			return nil
		}
		attrLen := attribute.WriteAttrTo(mpUnreach, attrBuf, 0)
		return &message.Update{
			PathAttributes: attrBuf[:attrLen],
		}
	}

	plan := getAnnouncePlan()
	defer putAnnouncePlan(plan)

	base := a.planBatchAttrs(plan, batch, facts)
	plan.add(mpUnreach, nil)

	n, ok := plan.emit(base, attrBuf)
	if !ok {
		// Every refusal reachable here is a size refusal. The one cause that is not,
		// an attribute with no wire form, belongs to the NEXT_HOP the plan may
		// contribute, and legacyNextHopApplies has already refused an address that
		// has none (buildBatchAnnounceUpdate carries the same reasoning for
		// MP_REACH_NLRI, where the address is the operator's to fix).
		logWithdrawTooLarge(batch, len(attrBuf), "attributes")
		return nil
	}
	return &message.Update{
		PathAttributes: attrBuf[:n],
	}
}

// buildWithheldWithdrawUpdate builds the UPDATE a WITHHELD withdrawal writes: the
// same message buildBatchWithdrawUpdate would have built, with the routes removed.
// MP_UNREACH_NLRI is not contributed and no NLRI is written, so what reaches the
// peer is the withdrawal's path attributes and nothing else.
//
// RFC 4271 Section 6.3: "An UPDATE message that contains correct path attributes,
// but no NLRI, SHALL be treated as a valid UPDATE message." So the message is one
// a conformant receiver accepts, and it names no route, which is the whole point:
// withdrawBatchFromPeers withholds the routes because RFC 4271 Section 4.3 scopes
// a withdrawn route to the connection it was previously advertised on.
//
// Nothing in an RFC asks a speaker to SEND it. It is an ExaBGP compatibility
// contract, pinned by `test/exabgp-compat/api/api-flow.ci`, whose second frame is
// this message: upstream drops each withdrawn NLRI while a session's
// include_withdraw is still False and yields the packed attributes regardless
// (`src/exabgp/bgp/message/update/collection.py`, UpdateCollection.messages).
//
// Returns nil when there is nothing left once the routes are gone. IPv4 unicast
// carries its withdrawal in the Withdrawn Routes field with no path attributes,
// and the multiprotocol unicast families carry a bare MP_UNREACH_NLRI, so both
// leave an empty message rather than an attributes-only one. Upstream sends
// nothing for those too, which `api-fast` records.
func (a *reactorAPIAdapter) buildWithheldWithdrawUpdate(attrBuf []byte, batch bgptypes.NLRIBatch, facts announceFacts) *message.Update {
	if batch.Family == family.IPv4Unicast || batch.Family.SAFI == family.SAFIUnicast {
		return nil
	}

	plan := getAnnouncePlan()
	defer putAnnouncePlan(plan)

	base := a.planBatchAttrs(plan, batch, facts)

	n, ok := plan.emit(base, attrBuf)
	if !ok {
		logWithdrawTooLarge(batch, len(attrBuf), "withheld-attributes")
		return nil
	}
	return &message.Update{
		PathAttributes: attrBuf[:n],
	}
}

// logWithdrawTooLarge records a rejected withdraw: the "or say something" half of
// buildBatchWithdrawUpdate's guard. WithdrawNLRIBatch also returns
// errWithdrawTooLarge to the issuing plugin, so a withdrawal that never reached
// the wire is visible from both ends (ai/rules/evidence.md).
func logWithdrawTooLarge(batch bgptypes.NLRIBatch, bufLen int, stage string) {
	routesLogger().Warn("withdraw rejected: NLRIs do not fit the build buffer",
		"family", batch.Family, "nlri-count", len(batch.NLRIs),
		"buffer-bytes", bufLen, "stage", stage,
		"action", "routes not withdrawn from this peer; send fewer prefixes per withdrawal")
}

// SendRoutes sends routes directly to matching peers using CommitService.
// It is used for named commits, and it builds and sends inside this call: the
// engine holds no Adj-RIB-Out to queue into.
//
// sender is who commits: an attached process, or the operator. It is gated on
// `send [ update ]`: a named commit announces routes, withdraws
// them and can close with an End-of-RIB marker, and all three are UPDATEs.
//
// Every count in the result is DERIVED from the loop below: one row per matched
// peer holding what that peer's own sends returned, and the totals summed from
// the rows. Nothing is assigned from a queue length, because three outcomes in
// this loop -- a peer that is not established, a commit that stops part way, and
// a family the build buffer refuses -- each leave a peer short of the queue
// while the queue length says otherwise (ai/rules/principles.md).
func (a *reactorAPIAdapter) SendRoutes(sel *selector.Selector, routes []*rib.Route, withdrawals []nlri.NLRI, sendEOR bool, sender plugin.Sender) (bgptypes.TransactionResult, error) {
	a.r.mu.RLock()
	peers, permErr := a.getMatchingPeersSel(sel, announceOrigin(sender))
	a.r.mu.RUnlock()
	if permErr != nil {
		return bgptypes.TransactionResult{}, permErr
	}
	if len(peers) == 0 {
		return bgptypes.TransactionResult{}, errors.New("no peers match selector")
	}

	// Collect families for EOR (from both routes and withdrawals)
	seen := make(map[family.Family]bool)
	for _, r := range routes {
		seen[r.NLRI().Family()] = true
	}
	for _, n := range withdrawals {
		seen[n.Family()] = true
	}
	families := make([]family.Family, 0, len(seen))
	for f := range seen {
		families = append(families, f)
	}

	result := bgptypes.TransactionResult{
		RoutesQueued:      len(routes),
		WithdrawalsQueued: len(withdrawals),
		EORRequested:      sendEOR,
		Peers:             make([]bgptypes.PeerCommitResult, 0, len(peers)),
	}

	for _, peer := range peers {
		row := a.commitToPeer(peer, routes, withdrawals, families, sendEOR)
		result.RoutesAnnounced += row.RoutesAnnounced
		result.RoutesWithdrawn += row.RoutesWithdrawn
		result.UpdatesSent += row.UpdatesSent
		result.EORSent += row.EORSent
		result.Peers = append(result.Peers, row)
	}

	// Build family strings for result
	familyStrs := make([]string, len(families))
	for i, f := range families {
		familyStrs[i] = f.String()
	}
	result.Families = familyStrs

	return result, nil
}

// commitToPeer runs one matched peer's half of a named commit and states what
// that peer took.
//
// The refusals it meets are unchanged and still fail closed: no partial UPDATE
// reaches the wire, and a refused family is withdrawn from nobody. What changes
// is that each refusal now leaves a reason on the row instead of a bare
// `continue`, so the operator who caused it reads it in the answer rather than
// in a log they were not watching.
//
// A row carries a reason if and only if this peer took less than the commit
// offered it. TestSendRoutesReasonsAndShortfallAgree holds both directions.
func (a *reactorAPIAdapter) commitToPeer(peer *Peer, routes []*rib.Route, withdrawals []nlri.NLRI, families []family.Family, sendEOR bool) bgptypes.PeerCommitResult {
	settings := peer.Settings()
	row := bgptypes.PeerCommitResult{
		Name:    settings.Name,
		Address: peer.addrString,
		State:   peer.State().String(),
	}

	// The encoding context is stored at Established and nowhere else
	// (setEncodingContexts, peer.go), so a nil one IS the not-established case.
	ctx := peer.sendContext()
	if ctx == nil {
		row.Reasons = append(row.Reasons, bgptypes.CommitReasonNotEstablished)
		return row
	}

	if len(withdrawals) > 0 {
		outcome := a.sendWithdrawals(peer, withdrawals)
		row.UpdatesSent += outcome.updatesSent
		row.RoutesWithdrawn += outcome.withdrawn
		if outcome.refused > 0 {
			row.Reasons = append(row.Reasons, bgptypes.CommitReasonWithdrawRefused)
		}
		if outcome.failed > 0 {
			row.Reasons = append(row.Reasons, bgptypes.CommitReasonSendFailed)
		}
	}
	if len(routes) > 0 {
		// Two-level grouping, one UPDATE per attribute-and-AS_PATH group.
		sender := pathsLimitCommitSender{peer: peer}
		cs := rib.NewCommitService(&sender, ctx, true)

		// Commit returns its stats BESIDE the error, and those stats are
		// partial: a refusal in the third group leaves two groups already on
		// the wire. Counting them is the whole point -- discarding the error
		// used to discard the UPDATEs that did leave along with it.
		stats, err := cs.Commit(routes, rib.CommitOptions{SendEOR: false})
		row.UpdatesSent += stats.UpdatesSent - int(sender.withheld.updates)
		row.RoutesAnnounced += stats.RoutesAnnounced - int(sender.withheld.routes)
		switch {
		case err != nil:
			row.Reasons = append(row.Reasons, bgptypes.CommitReasonAnnounceRefused)
		case row.RoutesAnnounced < len(routes):
			// The session can withhold routes across commits as well as within
			// one batch. Report only paths actually accepted by its writer.
			row.Reasons = append(row.Reasons, bgptypes.CommitReasonRoutesDropped)
		}
	}

	if sendEOR {
		for _, f := range families {
			eor := message.BuildEOR(f)
			if err := peer.SendUpdate(eor); err != nil {
				continue
			}
			peer.incrEORSent()
			row.UpdatesSent++
			row.EORSent++
		}
		if row.EORSent < len(families) {
			row.Reasons = append(row.Reasons, bgptypes.CommitReasonEORRefused)
		}
	}

	return row
}

// withdrawOutcome is what the withdrawal half of a named commit produced for ONE
// peer. Every field counts what the peer accepted, so a family the build buffer
// refused and an UPDATE the peer rejected both leave `withdrawn` short of the
// list the caller passed in.
type withdrawOutcome struct {
	updatesSent int // UPDATE messages the peer accepted.
	withdrawn   int // NLRIs carried by those UPDATEs.
	refused     int // Families whose NLRIs or MP_UNREACH_NLRI did not fit the build buffer.
	failed      int // Families whose UPDATE the peer did not accept.
}

// sendWithdrawals sends withdrawal UPDATE messages for the given NLRIs.
// Groups by family for efficient packing.
// RFC 7911: Uses WriteNLRI for ADD-PATH aware encoding.
//
// A family is all-or-nothing: its NLRIs travel in one UPDATE, so a family that
// is refused or that the peer rejects contributes nothing to `withdrawn`. The
// two counters are kept apart because the operator's next action differs --
// `refused` says to send fewer prefixes per commit, `failed` says the session
// is not carrying messages.
func (a *reactorAPIAdapter) sendWithdrawals(peer *Peer, withdrawals []nlri.NLRI) withdrawOutcome {
	var outcome withdrawOutcome
	if len(withdrawals) == 0 {
		return outcome
	}

	// Group withdrawals by family
	byFamily := make(map[family.Family][]nlri.NLRI)
	for _, n := range withdrawals {
		f := n.Family()
		byFamily[f] = append(byFamily[f], n)
	}

	ipv4Unicast := family.IPv4Unicast

	for fam, nlris := range byFamily {
		// RFC 7911: Get ADD-PATH encoding setting
		addPath := peer.addPathFor(fam)
		var update *message.Update

		// Write NLRIs into pooled buffer. Bounded for the same reason the batch
		// rails are: WriteNLRI panics past len(buf), and this loop is driven by a
		// caller-supplied withdrawal list of unbounded length.
		nlriHandle := getBuildBuf()
		off := writeBatchNLRI(nlriHandle.Buf, nlris, addPath)
		if off < 0 {
			// Logged AND counted. The Warn line is not something an operator
			// can act on mid-command, so the refusal also reaches the answer
			// through outcome.refused (commitToPeer).
			routesLogger().Warn("withdraw rejected: NLRIs do not fit the build buffer",
				"family", fam, "nlri-count", len(nlris), "buffer-bytes", len(nlriHandle.Buf),
				"stage", "send-routes",
				"action", "routes not withdrawn from this peer; send fewer prefixes per commit")
			putBuildBuf(nlriHandle)
			outcome.refused++
			continue
		}
		nlriBytes := nlriHandle.Buf[:off]

		if fam == ipv4Unicast {
			// IPv4 unicast: use WithdrawnRoutes field
			update = &message.Update{
				WithdrawnRoutes: nlriBytes,
			}
		} else {
			// Other families: use MP_UNREACH_NLRI attribute
			mpUnreach := &attribute.MPUnreachNLRI{
				AFI:  attribute.AFI(fam.AFI),
				SAFI: attribute.SAFI(fam.SAFI),
				NLRI: nlriBytes,
			}
			attrHandle := getBuildBuf()
			if attribute.AttrWireLen(mpUnreach) > len(attrHandle.Buf) {
				// The NLRI fitted its own slot but the attribute wrapping it does
				// not fit this one: WriteAttrTo would write a header declaring
				// more octets than the clamped value copy carries.
				routesLogger().Warn("withdraw rejected: MP_UNREACH_NLRI does not fit the build buffer",
					"family", fam, "nlri-count", len(nlris), "buffer-bytes", len(attrHandle.Buf),
					"stage", "send-routes",
					"action", "routes not withdrawn from this peer; send fewer prefixes per commit")
				putBuildBuf(attrHandle)
				putBuildBuf(nlriHandle)
				outcome.refused++
				continue
			}
			attrLen := attribute.WriteAttrTo(mpUnreach, attrHandle.Buf, 0)
			update = &message.Update{
				PathAttributes: attrHandle.Buf[:attrLen],
			}
			// Send then return attr buffer (nlri already copied into attrBuf by WriteAttrTo)
			outcome.record(peer.SendUpdate(update), len(nlris))
			putBuildBuf(attrHandle)
			putBuildBuf(nlriHandle)
			continue
		}

		outcome.record(peer.SendUpdate(update), len(nlris))
		putBuildBuf(nlriHandle)
	}

	return outcome
}

// record folds one family's send into the outcome. sendErr is what
// (*Peer).SendUpdate answered, and nlriCount is how many NLRIs that one UPDATE
// carried. A send that failed carried none of them.
func (o *withdrawOutcome) record(sendErr error, nlriCount int) {
	if sendErr != nil {
		o.failed++
		return
	}
	o.updatesSent++
	o.withdrawn += nlriCount
}

// sendStaleReadvertise handles one destination peer on a stale (LLGR) announce
// batch. It frames the batch as the announce rail frames it, and for each unit
// builds the announce, runs the registered readvertise egress filters (RFC 9494
// LLGR) with meta["stale"] and the peer as destination, then realizes
// the per-peer decision: withdrawal for a non-LLGR eBGP peer (mods.IsWithdraw),
// a depreferenced announce for a non-LLGR iBGP peer (attribute mods), or the
// unchanged announce for an LLGR-capable peer. The filter chain here is ONLY the
// Readvertise-opted filters, never the full egress chain, so a readvertise does
// not re-apply OTC/community/policy that already ran at the original announce.
//
// Returns (sent, failErr). sent counts the messages accepted for sending, which
// is one for a peer that groups updates and one per NLRI for a peer carrying
// `group-updates false`. failErr is non-nil when the re-advertise could not be
// carried out at all -- a filter that could not run, or modifications that could
// not be built -- and it wraps errStaleReadvertiseWithheld. The route is
// withheld either way; failErr exists so the caller does not report a defect in
// Ze as a peer that declined the family. A policy suppression yields (0, nil):
// that IS a decision.
func (a *reactorAPIAdapter) sendStaleReadvertise(peer *Peer, batch bgptypes.NLRIBatch, nextHop netip.Addr, isIBGP bool, nc *NegotiatedCapabilities) (sent int, failErr error) {
	facts := announceFactsFor(peer, batch.Family, nextHop, isIBGP, nc)
	maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, facts.extended))

	attrHandle := getBuildBuf()
	nlriHandle := getBuildBuf()
	defer putBuildBuf(attrHandle)
	defer putBuildBuf(nlriHandle)

	// The framing is the announce rail's, through the same nlriUnitLen: a peer
	// carrying `group-updates false` is re-advertised one prefix per UPDATE, and
	// the filter decides once for each of them. The decision reads the
	// destination and the stale level rather than the prefixes, so every unit of
	// one batch gets the same answer.
	unitLen := nlriUnitLen(len(batch.NLRIs), facts.groupUpdates)

	for off := 0; ; off += unitLen {
		end := min(off+unitLen, len(batch.NLRIs))
		unit := batch
		unit.NLRIs = batch.NLRIs[off:end]

		unitSent, unitErr := a.sendStaleReadvertiseUnit(peer, unit, facts, maxMsgSize, attrHandle.Buf, nlriHandle.Buf)
		if unitErr != nil {
			return sent, unitErr
		}
		if unitSent {
			sent++
		}

		if end >= len(batch.NLRIs) {
			break
		}
	}
	return sent, nil
}

// sendStaleReadvertiseUnit runs the readvertise egress filters over ONE built
// UPDATE and carries out what they decided. attrBuf and nlriBuf are the caller's
// pooled build buffers, reused by every unit of the batch.
func (a *reactorAPIAdapter) sendStaleReadvertiseUnit(peer *Peer, batch bgptypes.NLRIBatch, facts announceFacts, maxMsgSize int, attrBuf, nlriBuf []byte) (sent bool, failErr error) {
	update, buildErr := a.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch, facts)
	if update == nil {
		// The announce itself could not be encoded (already logged). Report the
		// builder's own cause rather than a family mismatch: the family IS
		// negotiated, and this rail sits beside one that reports the same reason.
		return false, buildErr
	}

	// Run the readvertise egress filters. LLGREgressFilter keys off meta["stale"]
	// and the destination peer's LLGR capability; it writes into mods.
	body := fwdPackUpdateBody(update)
	dest := filterapi.PeerFilterInfo{
		Address: peer.Settings().Address,
		PeerAS:  peer.PeerAS(), // guarded: dest may be a dynamic peer still resolving its ASN
		LocalAS: facts.prepend.primary,
		// Name/GroupName complete the destination identity. The other six
		// PeerFilterInfo fills in this package carry them
		// (reactor_api_forward.go, reactor_api_forward_batch.go,
		// reactor_api_relay.go, peer_forward_facts.go, forward_rs.go,
		// reactor_notify.go); this readvertise rail was the one that did not,
		// so a filter that looks a peer up by name read a silent empty string
		// from here alone -- the zero-value trap of
		// ai/rules/evidence.md, and the same shape that let the OTC
		// gates go permissive on an unresolved lookup. Both are immutable
		// after peer construction (see forward_rs.go), so no lock is needed.
		Name:      peer.Settings().Name,
		GroupName: peer.Settings().GroupName,
	}
	outcome, modified := a.decideStaleReadvertise(dest, body, batch.Stale)

	switch outcome {
	case staleFilterFailed:
		// A filter crashed, so no decision exists for this peer. Withheld
		// fail-closed like a suppression, reported apart from one.
		return false, errStaleReadvertiseFilterPanic
	case staleBuildFailed:
		// The filter DID decide; realizing its decision failed. Withheld for the
		// same reason and reported under its own cause, because the operator
		// action differs: a crashed filter is a plugin bug, an unbuildable body
		// is a payload this speaker could not encode.
		return false, errStaleReadvertiseBuildFailed
	case staleSuppress:
		return false, nil // filter suppressed the route for this peer
	case staleWithdraw:
		// Non-LLGR eBGP peer: send a withdrawal for the same NLRIs.
		wdAttr := getBuildBuf()
		wdNlri := getBuildBuf()
		defer putBuildBuf(wdAttr)
		defer putBuildBuf(wdNlri)

		// The withdrawal is THIS speaker's, not the operator's, so it names no
		// attributes: RFC 9494 asks Ze to remove a stale route from a peer that
		// cannot hold one, and the route's own attribute block describes the route
		// this UPDATE is taking away. Clearing it here is what keeps
		// buildBatchWithdrawUpdate's rule -- carry what the caller named -- true for
		// a caller that named nothing (ai/rules/principles.md). A non-unicast family
		// still gets the mandatory ORIGIN and AS_PATH that rail owes every
		// withdrawal of its own.
		bare := batch
		bare.Wire = nil
		bare.Attrs = nil
		bare.NextHop = bgptypes.RouteNextHop{}
		wd := a.buildBatchWithdrawUpdate(wdAttr.Buf, wdNlri.Buf, bare, announceFacts{addPath: facts.addPath})
		if wd == nil {
			// Same reasoning as the announce build above, under the withdraw
			// rail's own cause (already logged); nothing was sent.
			return false, errWithdrawTooLarge
		}
		return peer.sendUpdateWithSplit(wd, maxMsgSize, facts.addPath) == nil, nil
	case staleModify:
		// Non-LLGR iBGP peer: apply the depreference mods (NO_EXPORT + LOCAL_PREF=0).
		if modified == nil {
			return peer.sendUpdateWithSplit(update, maxMsgSize, facts.addPath) == nil, nil
		}
		return peer.sendBodyWithSplit(modified, maxMsgSize, facts.addPath) == nil, nil
	default: // staleKeep
		// LLGR-capable peer: send the stale route unchanged.
		return peer.sendUpdateWithSplit(update, maxMsgSize, facts.addPath) == nil, nil
	}
}

// staleOutcome is the per-peer decision of the readvertise egress filters.
type staleOutcome int

// Exactly one of these is a policy decision. staleSuppress is the filter saying
// no; the two failure outcomes are this speaker saying "I could not". All three
// withhold the route, and telling them apart is what stops a defect in Ze being
// reported to the operator as a peer's policy.
const (
	staleKeep         staleOutcome = iota // send the stale route unchanged (LLGR-capable peer)
	staleModify                           // send with attribute mods (non-LLGR iBGP depreference)
	staleWithdraw                         // send a withdrawal (non-LLGR eBGP)
	staleSuppress                         // a filter rejected the route for this peer
	staleFilterFailed                     // a filter could not run: nothing was decided for this peer
	staleBuildFailed                      // a filter decided, but its modifications could not be built
)

// decideStaleReadvertise runs the registered readvertise egress filters for one
// destination peer over the packed announce body and returns the outcome plus,
// for staleModify, the modified UPDATE body. It is the pure decision half of
// sendStaleReadvertise, split out so the filter->outcome mapping is unit-testable
// without a live session. LLGREgressFilter (RFC 9494) is the sole registered
// filter today; it reads dest + meta["stale"] and writes into mods.
func (a *reactorAPIAdapter) decideStaleReadvertise(dest filterapi.PeerFilterInfo, body []byte, stale uint8) (staleOutcome, []byte) {
	meta := map[string]any{"stale": stale}
	var src filterapi.PeerFilterInfo
	var mods filterapi.ModAccumulator
	for _, f := range a.r.readvertiseEgressFilters {
		accept, panicked := safeEgressFilter(f, src, dest, body, meta, &mods)
		if accept {
			continue
		}
		// Both outcomes withhold the route, and they must: the fact the crashed
		// filter was reading is the destination's LLGR capability, so RFC 9494
		// Section 4.3 ("SHOULD NOT be advertised to peers that have not
		// advertised the LLGR capability") and Section 4.6's NO_EXPORT +
		// LOCAL_PREF=0 obligations cannot be applied on a guess. What differs is
		// what the caller may SAY about it: staleSuppress is the filter's
		// decision for this peer, staleFilterFailed is no decision at all.
		if panicked {
			return staleFilterFailed, nil
		}
		return staleSuppress, nil
	}
	switch {
	case mods.IsWithdraw():
		return staleWithdraw, nil
	case mods.HasModifications():
		modified, _, modFail := buildModifiedPayload(body, &mods, a.r.attrModHandlers, nil, nil)
		a.r.recordModifyFailureAddr(modFail, modifySiteStaleReadvertise, dest.Address)
		if modFail.failed() || modified == nil {
			// Fail closed. mods.HasModifications() is true, so a nil payload
			// here can only mean the build refused; re-advertising the stale
			// route unmodified would undo the RFC 9494 egress filter's decision.
			// modified == nil with no named failure has TWO producers since
			// 2026-08-04: an empty accumulator, and a rebuild whose every
			// operation the advertise gate refused (advertiseGate,
			// forward_build.go). Neither reaches here -- a stale
			// re-advertisement carries the stored route's NLRI, so the gate
			// answers "advertises" and refuses nothing -- and it is folded in
			// anyway so a future path cannot make it a silent leak.
			//
			// The nil-with-no-failure half reaches recordModifyFailure as
			// modifyFailureNone, which does not log, so it keeps its own line.
			if !modFail.failed() {
				fwdLogger().Warn("stale re-advertise produced no payload, withholding route",
					"peer", dest.Address)
			}
			// NOT staleSuppress. The filter above DECIDED -- it asked for the
			// RFC 9494 Section 4.6 depreference -- and what failed is realizing
			// that decision. Reporting it as the filter choosing to drop the
			// route is the same conflation staleFilterFailed exists to end, and
			// it reached the operator as "no peers have family negotiated".
			// recordModifyFailureAddr above still counts and names the build's
			// own reason; this is the caller-facing half, not a second copy.
			return staleBuildFailed, nil
		}
		return staleModify, modified
	default:
		return staleKeep, nil
	}
}
