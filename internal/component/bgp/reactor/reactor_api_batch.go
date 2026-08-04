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
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// AnnounceNLRIBatch announces a batch of NLRIs with shared attributes.
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families.
// RFC 8654: Respects peer's max message size (4096 or 65535).
func (a *reactorAPIAdapter) AnnounceNLRIBatch(sel *selector.Selector, batch bgptypes.NLRIBatch) error {
	a.r.mu.RLock()
	peers := a.getMatchingPeersSel(sel)
	a.r.mu.RUnlock()
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
	var acceptedCount int

	// Group-aware path: when update groups are enabled, collect established
	// peers with identical build parameters and build the UPDATE once per group.
	// Falls back to per-peer when disabled or when peers differ.
	type announceBuildKey struct {
		nextHop netip.Addr
		isIBGP  bool
		// rsClient partitions the groups: RFC 7947 S2.2.2.1 suppresses the AS_PATH
		// prepend for RS-clients, so an RS-client and an ordinary eBGP peer no
		// longer produce identical wire and must not share a built UPDATE.
		rsClient bool
		localAS  uint32
		addPath  bool
		asn4     bool
		extended bool // ExtendedMessage negotiated
	}
	type announceBuildGroup struct {
		key     announceBuildKey
		peers   []*Peer
		nextHop netip.Addr
	}

	groupsEnabled := a.r.updateGroups != nil && a.r.updateGroups.Enabled()
	var buildGroups map[announceBuildKey]*announceBuildGroup

	if groupsEnabled {
		buildGroups = make(map[announceBuildKey]*announceBuildGroup)
	}

	for _, peer := range peers {
		// Guarded: this batch-announce path runs on an API/plugin goroutine and may read a
		// dynamic peer still resolving its ASN (sibling PeerAS read at :886 is guarded too).
		isIBGP := peer.IsIBGP()

		// Resolve next-hop per peer using RouteNextHop policy
		nextHop, nhErr := peer.resolveNextHop(batch.NextHop, batch.Family)
		if nhErr != nil {
			routesLogger().Debug("next-hop resolution failed", "peer", peer.Settings().Address, "error", nhErr)
			continue
		}

		if !peer.ShouldQueue() {
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
				switch {
				case sent:
					acceptedCount++
				case failErr != nil:
					// The readvertise could not be carried out: a filter crashed,
					// or its modifications could not be built. Either way the peer
					// decided nothing and the family IS negotiated, so
					// ErrNoPeersAcceptedFamily would state a cause that is untrue
					// (ai/rules/cli.md) and would be downgraded to a warning on
					// that basis. failErr names which one it was.
					lastErr = failErr
				default:
					lastErr = route.ErrNoPeersAcceptedFamily
				}
				continue
			}

			if groupsEnabled {
				// Collect peer into build group for deferred batch build.
				bk := announceBuildKey{
					nextHop:  nextHop,
					isIBGP:   isIBGP,
					rsClient: peer.Settings().RSClient,
					localAS:  peer.Settings().LocalAS,
					addPath:  peer.addPathFor(batch.Family),
					asn4:     peer.asn4(),
					extended: nc.ExtendedMessage,
				}
				bg, ok := buildGroups[bk]
				if !ok {
					bg = &announceBuildGroup{key: bk, nextHop: nextHop}
					buildGroups[bk] = bg
				}
				bg.peers = append(bg.peers, peer)
			} else {
				// Per-peer path (update groups disabled).
				maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, nc.ExtendedMessage))
				addPath := peer.addPathFor(batch.Family)
				asn4 := peer.asn4()

				attrHandle := getBuildBuf()
				nlriHandle := getBuildBuf()
				update := a.buildBatchAnnounceUpdate(attrHandle.Buf, nlriHandle.Buf, batch, nextHop, isIBGP, peer.Settings().RSClient, asn4, addPath, peer.Settings().LocalAS)

				// Build rejected (already logged). Not sent, and not counted as
				// accepted, so the caller gets an error instead of a silent drop.
				if update == nil {
					lastErr = errAnnounceTooLarge
				} else if err := peer.sendUpdateWithSplit(update, maxMsgSize, addPath); err != nil {
					lastErr = err
				} else {
					acceptedCount++
				}
				putBuildBuf(attrHandle)
				putBuildBuf(nlriHandle)
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			// Build AS_PATH only for queue path (iBGP vs eBGP); the established
			// path builds AS_PATH inside the UPDATE wire bytes directly.
			asPath := a.buildBatchASPathAttr(userASPath, batch.OriginAS, isIBGP, peer.Settings().RSClient, peer.Settings().LocalAS)
			for _, n := range batch.NLRIs {
				ribRoute := rib.NewRouteWithASPath(n, nextHop, attrs, asPath)
				peer.QueueAnnounce(ribRoute)
			}
			acceptedCount++ // Queued counts as accepted
		}
	}

	// Build once per group, send to all members.
	// INVARIANT: sendUpdateWithSplit is synchronous -- it blocks until bytes are
	// written to TCP. The shared *message.Update references pooled buffers that
	// are returned after this loop. Async writes would cause use-after-return.
	for _, bg := range buildGroups {
		maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, bg.key.extended))

		attrHandle := getBuildBuf()
		nlriHandle := getBuildBuf()
		update := a.buildBatchAnnounceUpdate(attrHandle.Buf, nlriHandle.Buf, batch, bg.nextHop, bg.key.isIBGP, bg.key.rsClient, bg.key.asn4, bg.key.addPath, bg.key.localAS)

		// Build rejected (already logged): every peer in this group shares the
		// build parameters, so none of them can be sent this batch.
		if update == nil {
			lastErr = errAnnounceTooLarge
			putBuildBuf(attrHandle)
			putBuildBuf(nlriHandle)
			continue
		}

		for _, peer := range bg.peers {
			if err := peer.sendUpdateWithSplit(update, maxMsgSize, bg.key.addPath); err != nil {
				lastErr = err
			} else {
				acceptedCount++
			}
		}
		putBuildBuf(attrHandle)
		putBuildBuf(nlriHandle)
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
			errors.Is(lastErr, errWithdrawTooLarge),
			errors.Is(lastErr, errStaleReadvertiseWithheld):
			return lastErr
		}
		return route.ErrNoPeersAcceptedFamily
	}
	return lastErr
}

// WithdrawNLRIBatch withdraws a batch of NLRIs.
// RFC 4271 Section 4.3: Withdrawn Routes field.
// RFC 4760: MP_UNREACH_NLRI for non-IPv4-unicast families.
func (a *reactorAPIAdapter) WithdrawNLRIBatch(sel *selector.Selector, batch bgptypes.NLRIBatch) error {
	a.r.mu.RLock()
	peers := a.getMatchingPeersSel(sel)
	a.r.mu.RUnlock()
	if len(peers) == 0 {
		return route.ErrNoPeersMatch
	}

	var lastErr error
	var acceptedCount int

	// Group-aware path for withdraw: peers with the same addPath and
	// ExtendedMessage produce identical withdraw UPDATEs.
	type withdrawBuildKey struct {
		addPath  bool
		extended bool
	}
	type withdrawBuildGroup struct {
		key   withdrawBuildKey
		peers []*Peer
	}

	groupsEnabled := a.r.updateGroups != nil && a.r.updateGroups.Enabled()
	var wdGroups map[withdrawBuildKey]*withdrawBuildGroup

	if groupsEnabled {
		wdGroups = make(map[withdrawBuildKey]*withdrawBuildGroup)
	}

	for _, peer := range peers {
		if !peer.ShouldQueue() {
			// Check family negotiation
			nc := peer.negotiated.Load()
			if nc == nil || !nc.Has(batch.Family) {
				continue // Skip peer that doesn't support this family
			}

			if groupsEnabled {
				wk := withdrawBuildKey{
					addPath:  peer.addPathFor(batch.Family),
					extended: nc.ExtendedMessage,
				}
				wg, ok := wdGroups[wk]
				if !ok {
					wg = &withdrawBuildGroup{key: wk}
					wdGroups[wk] = wg
				}
				wg.peers = append(wg.peers, peer)
			} else {
				// Per-peer path (update groups disabled).
				maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, nc.ExtendedMessage))
				addPath := peer.addPathFor(batch.Family)

				attrHandle := getBuildBuf()
				nlriHandle := getBuildBuf()
				update := a.buildBatchWithdrawUpdate(attrHandle.Buf, nlriHandle.Buf, batch, addPath)

				// Build rejected (already logged): not sent, and not counted as
				// accepted, so the caller gets an error instead of a silent drop.
				if update == nil {
					lastErr = errWithdrawTooLarge
				} else if err := peer.sendUpdateWithSplit(update, maxMsgSize, addPath); err != nil {
					lastErr = err
				} else {
					acceptedCount++
				}
				putBuildBuf(attrHandle)
				putBuildBuf(nlriHandle)
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			for _, n := range batch.NLRIs {
				peer.QueueWithdraw(n)
			}
			acceptedCount++ // Queued counts as accepted
		}
	}

	// Build once per group, send to all members.
	// INVARIANT: sendUpdateWithSplit is synchronous -- it blocks until bytes are
	// written to TCP. The shared *message.Update references pooled buffers that
	// are returned after this loop. Async writes would cause use-after-return.
	for _, wg := range wdGroups {
		maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, wg.key.extended))

		attrHandle := getBuildBuf()
		nlriHandle := getBuildBuf()
		update := a.buildBatchWithdrawUpdate(attrHandle.Buf, nlriHandle.Buf, batch, wg.key.addPath)

		// Build rejected (already logged): every peer in this group shares the
		// build parameters, so none of them can be sent this batch.
		if update == nil {
			lastErr = errWithdrawTooLarge
			putBuildBuf(attrHandle)
			putBuildBuf(nlriHandle)
			continue
		}

		for _, peer := range wg.peers {
			if err := peer.sendUpdateWithSplit(update, maxMsgSize, wg.key.addPath); err != nil {
				lastErr = err
			} else {
				acceptedCount++
			}
		}
		putBuildBuf(attrHandle)
		putBuildBuf(nlriHandle)
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
func (a *reactorAPIAdapter) buildBatchASPathAttr(userASPath *attribute.ASPath, originAS uint32, isIBGP, rsClient bool, localAS uint32) *attribute.ASPath {
	if userASPath == nil || len(userASPath.Segments) == 0 {
		return a.buildBatchASPath(nil, originAS, isIBGP, rsClient, localAS)
	}
	// RFC 7947 Section 2.2.2.1 exempts RS-clients; Section 5.1.2 forbids touching
	// the path toward an internal peer; with no local AS there is nothing to add.
	if isIBGP || rsClient || localAS == 0 || aspathLeadsWith(userASPath, localAS) {
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
	prepended.Prepend(localAS)
	return prepended
}

// buildBatchASPath builds AS_PATH for batch operations.
// RFC 4271 §5.1.2: iBGP SHALL NOT modify AS_PATH; eBGP prepends local AS.
// RFC 7947 §2.2.2: a route server does NOT prepend, for RS-client peers only.
func (a *reactorAPIAdapter) buildBatchASPath(userASPath []uint32, originAS uint32, isIBGP, rsClient bool, localAS uint32) *attribute.ASPath {
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
		if !isIBGP && !rsClient && asns[0] != localAS {
			prefixed := make([]uint32, 0, len(asns)+1)
			prefixed = append(prefixed, localAS)
			asns = append(prefixed, asns...)
		}
		return &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: asns},
			},
		}
	case originAS != 0:
		// Virtual-router origin: [originAS] on iBGP, [localAS, originAS] on eBGP.
		asns := []uint32{originAS}
		if !isIBGP {
			asns = []uint32{localAS, originAS}
		}
		return &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: asns},
			},
		}
	case isIBGP:
		return &attribute.ASPath{Segments: nil}
	default: // eBGP: prepend local AS
		return &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
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
func (a *reactorAPIAdapter) announceASPathRewrite(existing *attribute.ASPath, isIBGP, rsClient, srcKnown bool, localAS uint32) *attribute.ASPath {
	if existing == nil || !a.prependApplies(isIBGP, rsClient, true, localAS) {
		return nil
	}
	if !srcKnown {
		routesLogger().Warn("as-path prepend skipped: source ASN encoding unknown; sending an explicit as-path unchanged violates RFC 4271 S5.1.2 toward an external peer",
			"localAS", localAS)
		return nil
	}
	rewritten := a.buildBatchASPathAttr(existing, 0, isIBGP, rsClient, localAS)
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
// attrBuf and nlriBuf are caller-provided buffers (from buildBufPool).
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families.
//
// Returns nil when the batch cannot be encoded into attrBuf, having written
// nothing: the size query runs before the write, so a truncated or over-long block
// is not a state this function can produce (ai/rules/evidence.md). The
// caller reports errAnnounceTooLarge rather than sending a short UPDATE.
func (a *reactorAPIAdapter) buildBatchAnnounceUpdate(attrBuf, nlriBuf []byte, batch bgptypes.NLRIBatch, nextHop netip.Addr, isIBGP, rsClient, asn4, addPath bool, localAS uint32) *message.Update {
	// Write NLRIs into caller-provided buffer
	nlriOff := writeBatchNLRI(nlriBuf, batch.NLRIs, addPath)
	if nlriOff < 0 {
		logAnnounceTooLarge(batch, len(nlriBuf), "nlri")
		return nil
	}
	nlriBytes := nlriBuf[:nlriOff]

	dstCtx := announceDstCtx(asn4)

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

	plan := getAnnouncePlan()
	defer putAnnouncePlan(plan)

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
	if hadASPath && a.prependApplies(isIBGP, rsClient, srcKnown, localAS) {
		existingASPath := baseASPath(base, srcASN4)
		if existingASPath == nil && batch.Attrs != nil {
			existingASPath = batch.Attrs.ToASPath()
		}
		rewrittenASPath = a.announceASPathRewrite(existingASPath, isIBGP, rsClient, srcKnown, localAS)
	}

	// RFC 4271 Section 5.1.5, the prohibition half. localPrefAllowedTo
	// (forward_local_pref.go) owns the answer and the confederation exception, so
	// this rail and the two forward rails cannot disagree about it -- which they
	// did until 2026-08-04, when only this one stripped.
	localPrefAllowed := localPrefAllowedTo(isIBGP)

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
		var scratch [2]uint32
		asns := announceASPathASNs(scratch[:0], isIBGP, localAS, batch.OriginAS)
		synth := plan.asPathFor(asns)
		plan.add(synth, dstCtx)

		// RFC 6793 Section 4.2.2: a NEW speaker whose two-octet AS_PATH had to carry
		// AS_TRANS MUST also send the four-octet AS4_PATH. Derived from the SAME
		// announceASPathASNs sequence the AS_PATH above encoded, so the two cannot
		// disagree. Only when this rail synthesized the path: a verbatim AS_PATH owns
		// its own encoding.
		if !asn4 && anyNonMappableAS(asns) {
			plan.add(plan.as4PathFor(synth.Segments), nil)
		}
	}

	if batch.Family == family.IPv4Unicast {
		// Write exactly one NEXT_HOP, the authoritative resolved address. The base may
		// already carry one -- a relayed/replayed route stores the full received block,
		// NEXT_HOP included -- and a contribution REPLACES it rather than adding a
		// second, which FRR and others treat as a withdraw (RFC 7606 Section 3(g)).
		//
		// Guard on validity and fail closed. resolveNextHop (peer.go) does NOT validate
		// an explicit next-hop -- it deliberately returns whatever Addr was configured,
		// invalid included (see TestResolveNextHop_ExplicitInvalid) -- and an invalid
		// Addr encodes as a zero-LENGTH NEXT_HOP value (attribute/simple.go). If
		// nextHop is invalid, leave the base's own NEXT_HOP alone rather than replace a
		// good address with a malformed one.
		if nextHop.IsValid() {
			plan.add(plan.nextHopFor(nextHop), nil)
		}
	} else {
		// RFC 4760 Section 3: every other family carries its next-hop and NLRI inside
		// MP_REACH_NLRI. A relayed/replayed block may already carry one; the
		// contribution replaces it.
		plan.add(attribute.NewMPReachNLRI(attribute.AFI(batch.Family.AFI), attribute.SAFI(batch.Family.SAFI),
			[]netip.Addr{nextHop}, nlriBytes), nil)
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
	// rather than leaving the answer to Peer.ShouldQueue.
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

	n, ok := plan.emit(base, attrBuf)
	if !ok {
		logAnnounceTooLarge(batch, len(attrBuf), "attributes")
		return nil
	}

	update := &message.Update{PathAttributes: attrBuf[:n]}
	if batch.Family == family.IPv4Unicast {
		update.NLRI = nlriBytes
	}
	return update
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
	routesLogger().Warn("announce rejected: attributes do not fit the build buffer",
		"family", batch.Family, "nlri-count", len(batch.NLRIs),
		"buffer-bytes", bufLen, "stage", stage,
		"action", "route not sent to this peer; send fewer prefixes per announce")
}

// announceASPathASNs appends to dst, and returns, the AS_PATH ASN sequence this
// builder synthesizes for an announce that does not carry a verbatim AS_PATH:
//
//   - origin-as (originAS != 0): [originAS] for iBGP, [localAS, originAS] for
//     eBGP -- the normal export rule, so a real eBGP peer sees a well-formed
//     first AS (enforce-first-as).
//   - plain export (originAS == 0): empty for iBGP, [localAS] for eBGP.
//
// dst is normally a stack-allocated scratch array (the sequence is at most two
// ASNs) so the announce path stays allocation-free. This is the single source of
// the synthesized AS_PATH shape, shared by writeASPath (which two-octet-encodes
// it, mapping a non-mappable AS to AS_TRANS) and writeAnnounceAS4Path (which
// four-octet-encodes the same sequence into an AS4_PATH when that mapping happens
// toward an OLD peer), so the AS_PATH and AS4_PATH can never disagree.
func announceASPathASNs(dst []uint32, isIBGP bool, localAS, originAS uint32) []uint32 {
	if originAS != 0 {
		if !isIBGP {
			dst = append(dst, localAS)
		}
		return append(dst, originAS)
	}
	if isIBGP {
		return dst // empty AS_PATH
	}
	return append(dst, localAS)
}

// buildBatchWithdrawUpdate builds an UPDATE message for withdrawing a batch of NLRIs.
// attrBuf and nlriBuf are caller-provided buffers (from buildBufPool).
// RFC 4271 Section 4.3: Withdrawn Routes field.
// RFC 4760: MP_UNREACH_NLRI for non-IPv4-unicast families.
//
// Returns nil when the batch does not fit its pooled build buffers, exactly as
// buildBatchAnnounceUpdate does. The withdraw rail carried the SAME two unbounded
// writes the announce rail did: an NLRI loop that panics past len (WriteNLRI ends
// in an index expression), and an MP_UNREACH_NLRI whose declared length comes from
// Len() while its value copy clamps, so an oversize batch produced an attribute
// claiming more octets than it contained. A short withdraw is not a lesser failure
// than a short announce -- the peer keeps forwarding to prefixes it was never told
// about.
func (a *reactorAPIAdapter) buildBatchWithdrawUpdate(attrBuf, nlriBuf []byte, batch bgptypes.NLRIBatch, addPath bool) *message.Update {
	// Write NLRIs into caller-provided buffer
	nlriOff := writeBatchNLRI(nlriBuf, batch.NLRIs, addPath)
	if nlriOff < 0 {
		logWithdrawTooLarge(batch, len(nlriBuf), "nlri")
		return nil
	}
	nlriBytes := nlriBuf[:nlriOff]

	if batch.Family == (family.IPv4Unicast) {
		// IPv4 unicast: Use WithdrawnRoutes field
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
	if attribute.AttrWireLen(mpUnreach) > len(attrBuf) {
		logWithdrawTooLarge(batch, len(attrBuf), "mp-unreach")
		return nil
	}
	attrLen := attribute.WriteAttrTo(mpUnreach, attrBuf, 0)
	return &message.Update{
		PathAttributes: attrBuf[:attrLen],
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
// This bypasses OutgoingRIB transaction and is used for named commits.
func (a *reactorAPIAdapter) SendRoutes(sel *selector.Selector, routes []*rib.Route, withdrawals []nlri.NLRI, sendEOR bool) (bgptypes.TransactionResult, error) {
	a.r.mu.RLock()
	peers := a.getMatchingPeersSel(sel)
	a.r.mu.RUnlock()
	if len(peers) == 0 {
		return bgptypes.TransactionResult{}, errors.New("no peers match selector")
	}

	var totalResult bgptypes.TransactionResult

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

	// Track stats once (not per-peer)
	totalResult.RoutesAnnounced = len(routes)
	totalResult.RoutesWithdrawn = len(withdrawals)

	for _, peer := range peers {
		// Get encoding context for CommitService
		ctx := peer.SendContext()
		if ctx == nil {
			continue // Peer not established
		}

		// Use CommitService with two-level grouping for announcements
		cs := rib.NewCommitService(peer, ctx, true)

		// Send announcements
		if len(routes) > 0 {
			stats, err := cs.Commit(routes, rib.CommitOptions{SendEOR: false})
			if err != nil {
				continue
			}
			totalResult.UpdatesSent += stats.UpdatesSent
		}

		// Send withdrawals
		if len(withdrawals) > 0 {
			updatesSent := a.sendWithdrawals(peer, withdrawals)
			totalResult.UpdatesSent += updatesSent
		}

		// Send EOR for each family if requested
		if sendEOR {
			for _, f := range families {
				eor := message.BuildEOR(f)
				if err := peer.SendUpdate(eor); err == nil {
					peer.IncrEORSent()
					totalResult.UpdatesSent++
				}
			}
		}
	}

	// Build family strings for result
	familyStrs := make([]string, len(families))
	for i, f := range families {
		familyStrs[i] = f.String()
	}
	totalResult.Families = familyStrs

	return totalResult, nil
}

// sendWithdrawals sends withdrawal UPDATE messages for the given NLRIs.
// Groups by family for efficient packing.
// RFC 7911: Uses WriteNLRI for ADD-PATH aware encoding.
func (a *reactorAPIAdapter) sendWithdrawals(peer *Peer, withdrawals []nlri.NLRI) int {
	if len(withdrawals) == 0 {
		return 0
	}

	// Group withdrawals by family
	byFamily := make(map[family.Family][]nlri.NLRI)
	for _, n := range withdrawals {
		f := n.Family()
		byFamily[f] = append(byFamily[f], n)
	}

	updatesSent := 0
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
			routesLogger().Warn("withdraw rejected: NLRIs do not fit the build buffer",
				"family", fam, "nlri-count", len(nlris), "buffer-bytes", len(nlriHandle.Buf),
				"stage", "send-routes",
				"action", "routes not withdrawn from this peer; send fewer prefixes per commit")
			putBuildBuf(nlriHandle)
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
				continue
			}
			attrLen := attribute.WriteAttrTo(mpUnreach, attrHandle.Buf, 0)
			update = &message.Update{
				PathAttributes: attrHandle.Buf[:attrLen],
			}
			// Send then return attr buffer (nlri already copied into attrBuf by WriteAttrTo)
			if err := peer.SendUpdate(update); err == nil {
				updatesSent++
			}
			putBuildBuf(attrHandle)
			putBuildBuf(nlriHandle)
			continue
		}

		if err := peer.SendUpdate(update); err == nil {
			updatesSent++
		}
		putBuildBuf(nlriHandle)
	}

	return updatesSent
}

// sendStaleReadvertise handles one destination peer on a stale (LLGR) announce
// batch. It builds the announce, runs the registered readvertise egress filters
// (RFC 9494 LLGR) with meta["stale"] and the peer as destination, then realizes
// the per-peer decision: withdrawal for a non-LLGR eBGP peer (mods.IsWithdraw),
// a depreferenced announce for a non-LLGR iBGP peer (attribute mods), or the
// unchanged announce for an LLGR-capable peer. The filter chain here is ONLY the
// Readvertise-opted filters, never the full egress chain, so a readvertise does
// not re-apply OTC/community/policy that already ran at the original announce.
//
// Returns (sent, failErr). sent reports that a message was accepted for sending.
// failErr is non-nil when the re-advertise could not be carried out at all -- a
// filter that could not run, or modifications that could not be built -- and it
// wraps errStaleReadvertiseWithheld. The route is withheld either way; failErr
// exists so the caller does not report a defect in Ze as a peer that declined
// the family. A policy suppression yields (false, nil): that IS a decision.
func (a *reactorAPIAdapter) sendStaleReadvertise(peer *Peer, batch bgptypes.NLRIBatch, nextHop netip.Addr, isIBGP bool, nc *NegotiatedCapabilities) (sent bool, failErr error) {
	maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, nc.ExtendedMessage))
	addPath := peer.addPathFor(batch.Family)
	asn4 := peer.asn4()
	localAS := peer.Settings().LocalAS
	rsClient := peer.Settings().RSClient

	attrHandle := getBuildBuf()
	nlriHandle := getBuildBuf()
	defer putBuildBuf(attrHandle)
	defer putBuildBuf(nlriHandle)
	update := a.buildBatchAnnounceUpdate(attrHandle.Buf, nlriHandle.Buf, batch, nextHop, isIBGP, rsClient, asn4, addPath, localAS)
	if update == nil {
		// The announce itself could not be encoded (already logged). Report the
		// same cause the non-stale rail reports for this exact failure rather
		// than a family mismatch: the family IS negotiated, and this rail sits
		// beside one that has always said errAnnounceTooLarge here.
		return false, errAnnounceTooLarge
	}

	// Run the readvertise egress filters. LLGREgressFilter keys off meta["stale"]
	// and the destination peer's LLGR capability; it writes into mods.
	body := fwdPackUpdateBody(update)
	dest := filterapi.PeerFilterInfo{
		Address: peer.Settings().Address,
		PeerAS:  peer.PeerAS(), // guarded: dest may be a dynamic peer still resolving its ASN
		LocalAS: localAS,
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
		wd := a.buildBatchWithdrawUpdate(wdAttr.Buf, wdNlri.Buf, batch, addPath)
		if wd == nil {
			// Same reasoning as the announce build above, under the withdraw
			// rail's own cause (already logged); nothing was sent.
			return false, errWithdrawTooLarge
		}
		return peer.sendUpdateWithSplit(wd, maxMsgSize, addPath) == nil, nil
	case staleModify:
		// Non-LLGR iBGP peer: apply the depreference mods (NO_EXPORT + LOCAL_PREF=0).
		if modified == nil {
			return peer.sendUpdateWithSplit(update, maxMsgSize, addPath) == nil, nil
		}
		return peer.sendBodyWithSplit(modified, maxMsgSize, addPath) == nil, nil
	default: // staleKeep
		// LLGR-capable peer: send the stale route unchanged.
		return peer.sendUpdateWithSplit(update, maxMsgSize, addPath) == nil, nil
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
			// modified == nil with no named failure is unreachable today (the
			// "nothing to apply" early return needs an EMPTY accumulator), and
			// is folded in here so a future path cannot make it a silent leak.
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
