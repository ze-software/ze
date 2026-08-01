// RFC: rfc/short/rfc4271.md
// Design: docs/architecture/core-design.md — NLRI batch announce/withdraw and wire attribute building
// Overview: reactor_api.go — API command handling core
// Related: reactor_api_forward.go — forwarding and grouped sending
// Related: update_group.go — cross-peer UPDATE grouping index
package reactor

import (
	"encoding/binary"
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
				if a.sendStaleReadvertise(peer, batch, nextHop, isIBGP, nc) {
					acceptedCount++
				} else {
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
	// negotiated, so that cause is untrue (ai/rules/error-messages.md: leg 3 must
	// be TRUE) and the warning downgrade would hide a route that never went out.
	//
	// Deliberately narrow: every OTHER lastErr keeps the previous behavior. Widening
	// it to `lastErr != nil` also promoted long-standing soft cases -- a send error
	// against a peer that was still coming up -- into hard failures, which turned 19
	// functional tests red. That is a real question about how send errors should be
	// reported, but it is a separate one from this guard.
	if acceptedCount == 0 {
		if errors.Is(lastErr, errAnnounceTooLarge) {
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
// It is the queue-side twin of packedWithLocalASPrepended, which does the same job
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

// packedWithLocalASPrepended copies packed into dst with the AS_PATH attribute
// replaced by one carrying localAS at the front (RFC 4271 Section 5.1.2), leaving
// every other attribute byte-identical and in its original type order.
//
// Returns ok=false when no rewrite applies -- an internal peer (Section 5.1.2
// forbids modifying the path), an RS-client (RFC 7947 Section 2.2.2.1 grants
// transparency), no local AS, or a path that already leads with ours -- and the
// caller then copies packed verbatim, which is correct in every one of those
// cases.
//
// ok=false ALSO covers an AS_PATH that cannot be parsed or re-encoded. That path
// logs: shipping a path without our AS to an external peer is a conformance
// defect, and a guard that cannot act must at least say so
// (ai/rules/fail-closed-guards.md).
//
// ok=true with n == -1 is the third answer, and it is NOT the same as ok=false:
// the rewrite applies but does not fit in dst. Falling back to "copy packed
// verbatim" there would ship the RFC 4271 Section 5.1.2 violation this function
// exists to remove, so it is reported as a rejected build instead. The rewrite
// GROWS the block (a prepend adds an ASN, or a whole AS_SEQUENCE when the leading
// segment is an AS_SET), so `len(packed) <= len(dst)` does not imply the result
// fits and every write below is bounded on its own.
func (a *reactorAPIAdapter) packedWithLocalASPrepended(dst, packed []byte, isIBGP, rsClient, srcKnown, srcASN4, dstASN4 bool, localAS uint32) (int, bool) {
	if isIBGP || rsClient || localAS == 0 {
		return 0, false
	}
	// The 2-octet vs 4-octet encoding of the EXISTING path cannot be guessed: read
	// it wrong and the rewrite silently corrupts AS_PATH, which is worse than the
	// violation being fixed. AttributesWire.Get is not usable here -- it decodes
	// via a REGISTERED source context and the builder-built wire carries context 0
	// (buildBatchAnnounceUpdate), so it always errors. The caller, which knows
	// which mode produced the bytes, passes the answer in.
	if !srcKnown {
		routesLogger().Warn("as-path prepend skipped: source ASN encoding unknown; sending an explicit as-path unchanged violates RFC 4271 S5.1.2 toward an external peer",
			"localAS", localAS)
		return 0, false
	}

	dstCtx := bgpctx.EncodingContextForASN4(dstASN4)
	off := 0
	for i := 0; i+3 <= len(packed); {
		flags := packed[i]
		code := packed[i+1]
		hdr, vlen := 3, int(packed[i+2])
		if flags&byte(attribute.FlagExtLength) != 0 {
			if i+4 > len(packed) {
				return 0, false
			}
			hdr, vlen = 4, int(packed[i+2])<<8|int(packed[i+3])
		}
		end := i + hdr + vlen
		if end > len(packed) {
			routesLogger().Warn("as-path prepend skipped: attribute length runs past the packed block",
				"code", code, "localAS", localAS)
			return 0, false
		}
		if attribute.AttributeCode(code) == attribute.AttrASPath {
			existing, err := attribute.ParseASPath(packed[i+hdr:end], srcASN4)
			if err != nil {
				routesLogger().Warn("as-path prepend skipped: AS_PATH did not decode",
					"localAS", localAS, "srcASN4", srcASN4, "error", err)
				return 0, false
			}
			if aspathLeadsWith(existing, localAS) {
				return 0, false // already conformant; leave the operator's path alone
			}
			prepended := &attribute.ASPath{Segments: make([]attribute.ASPathSegment, len(existing.Segments))}
			for k, seg := range existing.Segments {
				asns := make([]uint32, len(seg.ASNs))
				copy(asns, seg.ASNs)
				prepended.Segments[k] = attribute.ASPathSegment{Type: seg.Type, ASNs: asns}
			}
			prepended.Prepend(localAS)
			// WriteAttrToWithContext writes through index expressions
			// (WriteHeaderTo), so it panics rather than clamps past len(dst).
			// LenWithContext is the same value it uses internally to size the
			// header and the value.
			valueLen := prepended.LenWithContext(nil, dstCtx)
			hdrLen := 3
			if valueLen > 255 || prepended.Flags().IsExtLength() {
				hdrLen = 4
			}
			if off+hdrLen+valueLen > len(dst) {
				return -1, true
			}
			off += attribute.WriteAttrToWithContext(prepended, dst, off, nil, dstCtx)
		} else {
			if off+(end-i) > len(dst) {
				return -1, true
			}
			off += copy(dst[off:], packed[i:end])
		}
		i = end
	}
	return off, true
}

// buildBatchAnnounceUpdate builds an UPDATE message for a batch of NLRIs.
// attrBuf and nlriBuf are caller-provided buffers (from buildBufPool).
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families.
func (a *reactorAPIAdapter) buildBatchAnnounceUpdate(attrBuf, nlriBuf []byte, batch bgptypes.NLRIBatch, nextHop netip.Addr, isIBGP, rsClient, asn4, addPath bool, localAS uint32) *message.Update {
	// Write NLRIs into caller-provided buffer
	nlriOff := writeBatchNLRI(nlriBuf, batch.NLRIs, addPath)
	if nlriOff < 0 {
		logAnnounceTooLarge(batch, len(nlriBuf), "nlri")
		return nil
	}
	nlriBytes := nlriBuf[:nlriOff]

	// Wire mode: ensure mandatory attributes present, then add NEXT_HOP or MP_REACH_NLRI
	if batch.Wire != nil {
		hadASPath, _ := batch.Wire.Has(attribute.AttrASPath)
		srcASN4, srcKnown := false, false
		if ctx := bgpctx.Registry.Get(batch.Wire.SourceContext()); ctx != nil {
			srcASN4, srcKnown = ctx.ASN4(), true
		}
		attrOff := a.writeMandatoryAttrs(attrBuf, batch.Wire, isIBGP, rsClient, srcKnown, srcASN4, asn4, localAS, batch.OriginAS)
		if attrOff < 0 {
			logAnnounceTooLarge(batch, len(attrBuf), "wire-mandatory")
			return nil
		}
		update := a.buildWireModeUpdate(attrBuf, attrOff, nlriBytes, batch.Family, nextHop, isIBGP)
		if update == nil {
			logAnnounceTooLarge(batch, len(attrBuf), "wire")
			return nil
		}
		if !hadASPath && !a.insertAnnounceAS4Path(update, attrBuf, isIBGP, asn4, localAS, batch.OriginAS) {
			logAnnounceTooLarge(batch, len(attrBuf), "wire-as4path")
			return nil
		}
		return update
	}

	// Builder mode or default: build attributes from Builder or defaults
	var builtBytes []byte
	if batch.Attrs != nil {
		builtBytes = batch.Attrs.Build()
	} else {
		// Default: just ORIGIN=IGP
		b := attribute.NewBuilder()
		b.SetOrigin(uint8(attribute.OriginIGP))
		builtBytes = b.Build()
	}

	// Ensure ORIGIN and AS_PATH are present (Builder may not include AS_PATH)
	wire := attribute.NewAttributesWire(builtBytes, 0)
	hadASPath, _ := wire.Has(attribute.AttrASPath)
	attrOff := a.writeMandatoryAttrs(attrBuf, wire, isIBGP, rsClient, true /*srcKnown*/, true /*Builder writes 4-octet ASNs*/, asn4, localAS, batch.OriginAS)
	if attrOff < 0 {
		logAnnounceTooLarge(batch, len(attrBuf), "builder-mandatory")
		return nil
	}

	// Add NEXT_HOP or MP_REACH_NLRI
	update := a.buildWireModeUpdate(attrBuf, attrOff, nlriBytes, batch.Family, nextHop, isIBGP)
	if update == nil {
		logAnnounceTooLarge(batch, len(attrBuf), "builder")
		return nil
	}
	if !hadASPath && !a.insertAnnounceAS4Path(update, attrBuf, isIBGP, asn4, localAS, batch.OriginAS) {
		logAnnounceTooLarge(batch, len(attrBuf), "builder-as4path")
		return nil
	}
	return update
}

// writeBatchNLRI writes every NLRI of a batch into nlriBuf, in order, and returns
// the bytes written -- or -1, having written nothing past the last whole NLRI,
// when they do not all fit.
//
// The bound is the other half of insertAttrOrdered's. Both build buffers come
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

// errWithdrawTooLarge is the withdraw-rail sibling: buildBatchWithdrawUpdate could
// not encode the batch's NLRIs (or the MP_UNREACH_NLRI carrying them) into its
// pooled build buffer. Separate from errAnnounceTooLarge so the operator-facing
// cause names the operation that actually failed (ai/rules/error-messages.md).
var errWithdrawTooLarge = errors.New("withdraw NLRIs exceed the build buffer; split the batch into smaller withdrawals")

// logAnnounceTooLarge records a rejected announce. This is the "or say something"
// half of the fail-closed guard in insertAttrOrdered: the build is abandoned
// rather than truncated, so without this line an operator would see routes simply
// not arrive. The plugin that issued the command also sees it -- AnnounceNLRIBatch
// returns errAnnounceTooLarge, which DispatchNLRIGroups turns into a StatusError
// response -- so the failure is observable from both ends
// (ai/rules/fail-closed-guards.md, ai/rules/error-messages.md).
func logAnnounceTooLarge(batch bgptypes.NLRIBatch, bufLen int, stage string) {
	routesLogger().Warn("announce rejected: attributes do not fit the build buffer",
		"family", batch.Family, "nlri-count", len(batch.NLRIs),
		"buffer-bytes", bufLen, "stage", stage,
		"action", "route not sent to this peer; send fewer prefixes per announce")
}

// insertAnnounceAS4Path adds an AS4_PATH attribute to update's PathAttributes
// (which alias attrBuf) when writeASPath synthesized a two-octet AS_PATH that had
// to carry AS_TRANS toward an OLD peer. It is called only when this builder
// synthesized the AS_PATH -- a verbatim AS_PATH copied from batch.Wire/Attrs owns
// its own encoding.
//
// It goes in at its type-code position (17), not at the end. The comment this
// replaces claimed 17 was "higher than every attribute this builder emits" and
// listed only the attributes the builder ITSELF writes; the block also carries the
// caller's attributes verbatim, and IPV6_EXT_COMMUNITIES (25) and
// LARGE_COMMUNITIES (32) both outrank AS4_PATH.
// Returns false when the AS4_PATH is owed but does not fit, so the caller aborts
// the build rather than send an UPDATE missing an attribute RFC 6793 §4.2.2 makes
// mandatory in that case.
func (a *reactorAPIAdapter) insertAnnounceAS4Path(update *message.Update, attrBuf []byte, isIBGP, asn4 bool, localAS, originAS uint32) bool {
	off := len(update.PathAttributes)
	n := writeAnnounceAS4Path(attrBuf, off, isIBGP, asn4, localAS, originAS)
	switch {
	case n < 0:
		return false
	case n == 0:
		return true
	}
	update.PathAttributes = attrBuf[:off+n]
	return true
}

// buildWireModeUpdate builds UPDATE using pre-written attribute bytes in attrBuf[:attrOff].
// Inserts NEXT_HOP (IPv4 unicast) or appends MP_REACH_NLRI (other families).
// attrBuf[:attrOff] must contain mandatory attrs from writeMandatoryAttrs.
// RFC 4271: NEXT_HOP (type 3) must come after AS_PATH (type 2) but before other attrs.
// RFC 4271 §5.1.5: LOCAL_PREF is well-known mandatory for iBGP sessions.
//
// Returns nil when the attributes do not fit in attrBuf. Every insert below is a
// fallible write into a pooled slot whose cap runs into the next peer's buffer
// (see insertAttrOrdered), so "does not fit" has to abort the build rather than
// produce a truncated or over-long block.
func (a *reactorAPIAdapter) buildWireModeUpdate(attrBuf []byte, attrOff int, nlriBytes []byte, fam family.Family, nextHop netip.Addr, isIBGP bool) *message.Update {
	isIPv4Unicast := fam == (family.IPv4Unicast)
	var ok bool

	if isIPv4Unicast {
		// Write exactly one NEXT_HOP, the authoritative resolved address. The wire block
		// may already carry one -- a relayed/replayed route stores the full received block,
		// NEXT_HOP included (writeMandatoryAttrs copied it) -- so strip any existing one
		// before inserting ours, or FRR and others treat the duplicate as a withdraw
		// (RFC 7606 Section 3(g)).
		//
		// Guard on validity and fail closed. resolveNextHop (peer.go) does NOT validate an
		// explicit next-hop -- it deliberately returns whatever Addr was configured, invalid
		// included (see TestResolveNextHop_ExplicitInvalid) -- and an invalid Addr encodes
		// as a zero-LENGTH NEXT_HOP value (attribute/simple.go). So the strip's safety
		// cannot rest on the resolver. If nextHop is invalid, leave the block's own NEXT_HOP
		// untouched rather than strip a good address and write a malformed one. No reachable
		// caller feeds an invalid explicit next-hop with a Wire block today; the guard makes
		// the strip safe regardless.
		if nextHop.IsValid() {
			attrOff = a.stripAttribute(attrBuf, attrOff, attribute.AttrNextHop)

			// Insert NEXT_HOP (type 3) after AS_PATH for correct type code order.
			nh := &attribute.NextHop{Addr: nextHop}
			if attrOff, ok = insertAttrOrdered(attrBuf, attrOff, nh); !ok {
				return nil
			}
		}

		// LOCAL_PREF=100 for iBGP, inserted in type-code order (5): the block may
		// already carry COMMUNITIES (8) or EXTENDED_COMMUNITIES (16) copied
		// verbatim from the caller's attributes, which appending would follow.
		if isIBGP && !a.hasAttribute(attrBuf[:attrOff], attribute.AttrLocalPref) {
			if attrOff, ok = insertAttrOrdered(attrBuf, attrOff, attribute.LocalPref(100)); !ok {
				return nil
			}
		}

		return &message.Update{
			PathAttributes: attrBuf[:attrOff],
			NLRI:           nlriBytes,
		}
	}

	// Non-IPv4 unicast: add LOCAL_PREF and MP_REACH_NLRI to the existing attrs. As with
	// NEXT_HOP above, a relayed/replayed block may already carry an MP_REACH_NLRI; drop
	// it before writing the authoritative one so the route is not duplicated (RFC 7606
	// Section 3(g)).
	//
	// Both go in at their type-code position, not at the end. The block here is the
	// caller's attributes copied verbatim by writeMandatoryAttrs, so it routinely holds
	// codes above 14 -- a FlowSpec announce carries its traffic-rate in
	// EXTENDED_COMMUNITIES (16) -- and appending put MP_REACH after it.
	attrOff = a.stripAttribute(attrBuf, attrOff, attribute.AttrMPReachNLRI)
	hasLocalPref := a.hasAttribute(attrBuf[:attrOff], attribute.AttrLocalPref)
	if isIBGP && !hasLocalPref {
		if attrOff, ok = insertAttrOrdered(attrBuf, attrOff, attribute.LocalPref(100)); !ok {
			return nil
		}
	}

	mpReach := attribute.NewMPReachNLRI(attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI), []netip.Addr{nextHop}, nlriBytes)
	if attrOff, ok = insertAttrOrdered(attrBuf, attrOff, mpReach); !ok {
		return nil
	}

	return &message.Update{
		PathAttributes: attrBuf[:attrOff],
	}
}

// stripAttribute removes every occurrence of typeCode from attrBuf[:attrOff],
// shifting later attributes left to close the gap, and returns the new length.
// Used before this builder writes an authoritative NEXT_HOP or MP_REACH_NLRI so a
// relayed/replayed attribute block that already carried one does not end up with
// the attribute twice (RFC 7606 Section 3(g): duplicate attribute is
// treat-as-withdraw). A well-formed block carries at most one; the loop also
// tolerates a malformed input that carried more.
func (a *reactorAPIAdapter) stripAttribute(attrBuf []byte, attrOff int, typeCode attribute.AttributeCode) int {
	pos := 0
	for pos+2 <= attrOff {
		flags := attrBuf[pos]
		tc := attribute.AttributeCode(attrBuf[pos+1])

		var attrLen int
		if flags&0x10 != 0 { // Extended length
			if pos+4 > attrOff {
				break
			}
			attrLen = 4 + int(binary.BigEndian.Uint16(attrBuf[pos+2:]))
		} else {
			if pos+3 > attrOff {
				break
			}
			attrLen = 3 + int(attrBuf[pos+2])
		}
		if pos+attrLen > attrOff {
			break // truncated; leave the tail as-is rather than corrupt it
		}

		if tc == typeCode {
			copy(attrBuf[pos:], attrBuf[pos+attrLen:attrOff])
			attrOff -= attrLen
			continue // re-examine at the same position after the shift
		}
		pos += attrLen
	}
	return attrOff
}

// hasAttribute checks if an attribute type is present in wire attrs.
func (a *reactorAPIAdapter) hasAttribute(wireAttrs []byte, typeCode attribute.AttributeCode) bool {
	pos := 0
	for pos < len(wireAttrs) {
		if pos+2 > len(wireAttrs) {
			break
		}
		flags := wireAttrs[pos]
		tc := wireAttrs[pos+1]
		_ = flags // used for length calculation below

		if attribute.AttributeCode(tc) == typeCode {
			return true
		}

		// Calculate attribute length to skip to next
		var attrLen int
		if flags&0x10 != 0 { // Extended length
			if pos+4 > len(wireAttrs) {
				break
			}
			attrLen = 4 + int(binary.BigEndian.Uint16(wireAttrs[pos+2:]))
		} else {
			if pos+3 > len(wireAttrs) {
				break
			}
			attrLen = 3 + int(wireAttrs[pos+2])
		}
		pos += attrLen
	}
	return false
}

// writeMandatoryAttrs ensures ORIGIN and AS_PATH are present in wire attributes,
// writing the result into buf. Returns bytes written, or -1 when the result does
// not fit in buf.
// RFC 4271 Section 5.1.1: ORIGIN is a well-known mandatory attribute.
// RFC 4271 Section 5.1.2: AS_PATH is a well-known mandatory attribute.
// RFC 4271 Section 5.1: Attributes must appear in type code order.
// If missing, adds defaults: ORIGIN=IGP, AS_PATH per iBGP/eBGP rules.
// localAS is the peer-specific local AS (used for AS_PATH prepend when missing).
//
// The -1 is the third capacity guard on this rail, and the one that closes the
// disclosure insertAttrOrdered's guard left open. Every arm below ends in
// `copy(buf, packed)` -- a CLAMPED copy -- but returned an offset derived from
// len(packed), which is not clamped. So an oversize caller block yielded
// attrOff > len(buf) while writing only len(buf) bytes, and buildWireModeUpdate
// then handed `attrBuf[:attrOff]` to the peer. That reslice does not panic: attrBuf
// is backing[off:off+4096] out of a 128-slot slab (session.go) and its CAP runs
// into the next peer's buffer, so the UPDATE carried the neighboring session's
// bytes. insertAttrOrdered rejects a bad attrOff, which covers the paths that
// insert something -- but the IPv4 branch inserts NEXT_HOP only when the next-hop
// is VALID (resolveNextHop deliberately passes an invalid explicit next-hop
// through) and LOCAL_PREF only for iBGP, so an eBGP announce with an invalid
// next-hop reached the reslice with nothing having checked attrOff.
// Failing here, at the producer of attrOff, is what makes every consumer below
// safe by construction (ai/rules/fail-closed-guards.md: make the miss explicit at
// the producer).
func (a *reactorAPIAdapter) writeMandatoryAttrs(buf []byte, wire *attribute.AttributesWire, isIBGP, rsClient, srcKnown, srcASN4, asn4 bool, localAS, originAS uint32) int {
	hasOrigin, _ := wire.Has(attribute.AttrOrigin)
	hasASPath, _ := wire.Has(attribute.AttrASPath)
	packed := wire.Packed()

	if hasOrigin && hasASPath {
		// RFC 4271 Section 5.1.2: an AS_PATH that arrived complete still has to
		// carry OUR AS toward an external peer. Copying `packed` straight through
		// (what this arm used to do unconditionally) shipped an operator-supplied
		// as-path without ze in it, so the receiver's loop detection could not see
		// itself behind us. RFC 7947 Section 2.2.2.1 excuses RS-clients, and
		// Section 5.1.2 forbids touching the path toward an internal peer.
		if n, ok := a.packedWithLocalASPrepended(buf, packed, isIBGP, rsClient, srcKnown, srcASN4, asn4, localAS); ok {
			return n // may be -1: the rewrite applied but did not fit
		}
		if len(packed) > len(buf) {
			return -1
		}
		copy(buf, packed)
		return len(packed)
	}

	// The synthesized ORIGIN (4 bytes) plus the synthesized AS_PATH this builder
	// may prepend. announceASPathASNs yields at most two ASNs, so the AS_PATH is at
	// most 3 header + 2 segment + 2*4 = 13 octets; 32 leaves room and keeps the
	// bound a constant rather than a second copy of writeASPathAttr's arithmetic.
	// Checked once, up front, so the fixed-position header writes below cannot
	// index past a short buffer either.
	const synthesizedMandatoryMax = 4 + 32
	if len(buf) < synthesizedMandatoryMax {
		return -1
	}

	off := 0

	// Case 1: Both missing - prepend ORIGIN + AS_PATH
	if !hasOrigin && !hasASPath {
		// ORIGIN=IGP
		buf[off] = 0x40 // Transitive
		buf[off+1] = 1  // ORIGIN
		buf[off+2] = 1  // Length
		buf[off+3] = 0  // IGP
		off += 4

		// AS_PATH
		off += a.writeASPath(buf[off:], isIBGP, asn4, localAS, originAS)

		if off+len(packed) > len(buf) {
			return -1
		}
		copy(buf[off:], packed)
		return off + len(packed)
	}

	// Case 2: Only ORIGIN missing - prepend ORIGIN, copy rest
	if !hasOrigin {
		if 4+len(packed) > len(buf) {
			return -1
		}
		buf[0] = 0x40 // Transitive
		buf[1] = 1    // ORIGIN
		buf[2] = 1    // Length
		buf[3] = 0    // IGP
		copy(buf[4:], packed)
		return 4 + len(packed)
	}

	// Case 3: Only AS_PATH missing - insert after ORIGIN
	// RFC 4271: attributes must be in type code order (ORIGIN=1, AS_PATH=2)
	originEnd := 4 // ORIGIN is always 4 bytes
	if len(packed) < originEnd {
		// hasOrigin said an ORIGIN is present, so a block shorter than one is
		// malformed. Reject rather than slice past it.
		return -1
	}
	copy(buf, packed[:originEnd])
	off = originEnd

	// Insert AS_PATH
	off += a.writeASPath(buf[off:], isIBGP, asn4, localAS, originAS)

	// Copy remaining attributes
	if off+len(packed)-originEnd > len(buf) {
		return -1
	}
	copy(buf[off:], packed[originEnd:])
	return off + len(packed) - originEnd
}

// findAttrInsertPosition finds where an attribute of type `code` belongs in wire
// attrs so the block stays in ascending type-code order.
// RFC 4271 Section 5: "The sender of an UPDATE message SHOULD order path
// attributes within the UPDATE message in ascending order of attribute type."
// Returns the offset of the first attribute whose type code is >= code, or the
// end of the block when every attribute present is lower-coded.
func findAttrInsertPosition(wireAttrs []byte, code attribute.AttributeCode) int {
	pos := 0
	for pos < len(wireAttrs) {
		if pos+2 > len(wireAttrs) {
			break
		}
		flags := wireAttrs[pos]
		typeCode := wireAttrs[pos+1]

		// First attribute at or above our type code: we belong here.
		if attribute.AttributeCode(typeCode) >= code {
			return pos
		}

		// Calculate attribute length
		var attrLen int
		if flags&0x10 != 0 { // Extended length
			if pos+4 > len(wireAttrs) {
				break
			}
			attrLen = 4 + int(binary.BigEndian.Uint16(wireAttrs[pos+2:]))
		} else {
			if pos+3 > len(wireAttrs) {
				break
			}
			attrLen = 3 + int(wireAttrs[pos+2])
		}

		pos += attrLen
	}
	// Nothing at or above our type code: append at the end.
	return pos
}

// attrWireLen returns the total wire size (header + value) that WriteAttrTo will
// write for attr. It has to agree with WriteHeaderTo, which promotes to the
// 4-octet extended-length header when the value exceeds 255 octets: an ordered
// insert shifts the tail by exactly this many bytes before writing, so a
// disagreement would corrupt the block rather than merely misorder it.
func attrWireLen(attr attribute.Attribute) int {
	valueLen := attr.Len()
	if valueLen > 255 || attr.Flags().IsExtLength() {
		return 4 + valueLen
	}
	return 3 + valueLen
}

// insertAttrOrdered writes attr into attrBuf[:attrOff] at the position that keeps
// the block in ascending attribute-type-code order (RFC 4271 Section 5), shifting
// any higher-coded attributes right to make room, and returns the new length.
//
// Appending at the end is only correct when nothing higher-coded is already
// present, and on this rail that does not hold: writeMandatoryAttrs copies the
// caller's attribute block VERBATIM, so an announce carrying EXTENDED_COMMUNITIES
// (16), IPV6_EXT_COMMUNITIES (25) or LARGE_COMMUNITIES (32) used to receive
// LOCAL_PREF (5) and MP_REACH_NLRI (14) after them. The queued rail
// (peer_rib_routes.go buildRIBRouteUpdate) and message/update_build.go both emit
// ascending, and which rail runs is decided by Peer.ShouldQueue() -- that is, by
// scheduling -- so the same route encoded to two different byte strings depending
// on timing.
//
// copy is a memmove and is defined for overlapping ranges, which is what the
// right-shift needs. No allocation: everything is written at an offset into the
// caller's pooled buffer (ai/rules/buffer-first.md).
// It returns ok=false, and writes nothing, when the attribute does not fit in
// attrBuf. That guard is not defensive tidiness, it is the difference between a
// rejected announce and a cross-session memory disclosure.
//
// attrBuf comes from getBuildBuf, which hands out backing[off:off+4096] from a
// 128-slot slab (session.go), so its CAP runs into the next peer's buffer while
// its LEN is the slot. Without the check, an announce whose attributes exceed the
// slot would: shift the tail into the neighboring slot
// (attrBuf[pos+n:attrOff+n] is within cap, so no panic to stop it), have
// MPReachNLRI.WriteTo silently clamp its final copy at len(attrBuf) so the NLRI
// is short, and then return attrOff+n past len -- which the caller reslices as
// attrBuf[:attrOff], handing the next peer's bytes to sendUpdateWithSplit. For
// the LAST slot in a slab cap == len, so the same reslice panics instead.
//
// Returning attrOff+written instead would be no better: MPReachNLRI's length
// field is written from Len() before the clamped copy, so a short write yields an
// attribute whose declared length exceeds its content -- malformed wire rather
// than leaked wire. There is no truncation that is correct here, which is why
// this fails closed and the caller rejects the announce
// (ai/rules/exact-or-reject.md, ai/rules/fail-closed-guards.md).
func insertAttrOrdered(attrBuf []byte, attrOff int, attr attribute.Attribute) (int, bool) {
	n := attrWireLen(attr)
	if attrOff < 0 || n < 0 || attrOff+n > len(attrBuf) {
		return attrOff, false
	}
	pos := findAttrInsertPosition(attrBuf[:attrOff], attr.Code())
	if pos < attrOff {
		copy(attrBuf[pos+n:attrOff+n], attrBuf[pos:attrOff])
	}
	attribute.WriteAttrTo(attr, attrBuf, pos)
	return attrOff + n, true
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

// writeASPath writes the AS_PATH attribute to buf, returning bytes written.
// localAS is the peer-specific local AS number (may differ from reactor global
// config). A non-mappable four-octet AS (localAS or originAS > 65535) toward a
// peer that did not negotiate 4-octet support (asn4 == false) is encoded as
// AS_TRANS here (via writeASPathAttr); buildBatchAnnounceUpdate then emits the
// matching AS4_PATH (RFC 6793 §4.2.2) so the OLD peer can recover the real AS.
func (a *reactorAPIAdapter) writeASPath(buf []byte, isIBGP, asn4 bool, localAS, originAS uint32) int {
	var scratch [2]uint32
	return writeASPathAttr(buf, 0, announceASPathASNs(scratch[:0], isIBGP, localAS, originAS), asn4)
}

// writeAnnounceAS4Path adds an AS4_PATH attribute to the block in buf[:off] when
// the AS_PATH synthesized by announceASPathASNs had to substitute AS_TRANS toward
// an OLD (2-octet) peer, returning bytes added (0 when none is needed).
//
// RFC 6793 §4.2.2: a NEW speaker sending a two-octet AS_PATH that contains a
// non-mappable AS MUST also send the AS4_PATH (four-octet encoding of the same
// sequence); when every AS is mappable it MUST NOT. asn4 == true means the peer
// negotiated 4-octet support, so AS_PATH already carries the real ASNs and no
// AS4_PATH is sent.
//
// The attribute goes in at its type-code position (17), not at the end: the block
// carries the caller's attributes verbatim, and IPV6_EXT_COMMUNITIES (25) and
// LARGE_COMMUNITIES (32) both outrank AS4_PATH. With an empty or all-lower-coded
// block the insert IS an append, which is why the sibling appenders in
// reactor_wire.go keep using writeAS4PathForASNs directly.
//
// Returns -1, distinct from the 0 meaning "none owed", when the attribute IS owed
// but does not fit in buf. RFC 6793 §4.2.2 makes it a MUST once the AS_PATH
// carries AS_TRANS, so quietly returning 0 there would ship a path the OLD peer
// cannot reconstruct -- a fail-open the caller must turn into a rejected build.
func writeAnnounceAS4Path(buf []byte, off int, isIBGP, asn4 bool, localAS, originAS uint32) int {
	var scratch [2]uint32
	as4 := as4PathForASNs(asn4, announceASPathASNs(scratch[:0], isIBGP, localAS, originAS))
	if as4 == nil {
		return 0
	}
	newOff, ok := insertAttrOrdered(buf, off, as4)
	if !ok {
		return -1
	}
	return newOff - off
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
	if attrWireLen(mpUnreach) > len(attrBuf) {
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
// the wire is visible from both ends (ai/rules/fail-closed-guards.md).
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
			if attrWireLen(mpUnreach) > len(attrHandle.Buf) {
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
// unchanged announce for an LLGR-capable peer. Returns true when a message was
// accepted for sending. The filter chain here is ONLY the Readvertise-opted
// filters, never the full egress chain, so a readvertise does not re-apply
// OTC/community/policy that already ran at the original announce.
func (a *reactorAPIAdapter) sendStaleReadvertise(peer *Peer, batch bgptypes.NLRIBatch, nextHop netip.Addr, isIBGP bool, nc *NegotiatedCapabilities) bool {
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
		// Build rejected (already logged). Report not-accepted so the caller
		// surfaces route.ErrNoPeersAcceptedFamily rather than counting a send
		// that never happened.
		return false
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
		// ai/rules/fail-closed-guards.md, and the same shape that let the OTC
		// gates go permissive on an unresolved lookup. Both are immutable
		// after peer construction (see forward_rs.go), so no lock is needed.
		Name:      peer.Settings().Name,
		GroupName: peer.Settings().GroupName,
	}
	outcome, modified := a.decideStaleReadvertise(dest, body, batch.Stale)

	switch outcome {
	case staleSuppress:
		return false // filter suppressed the route for this peer
	case staleWithdraw:
		// Non-LLGR eBGP peer: send a withdrawal for the same NLRIs.
		wdAttr := getBuildBuf()
		wdNlri := getBuildBuf()
		defer putBuildBuf(wdAttr)
		defer putBuildBuf(wdNlri)
		wd := a.buildBatchWithdrawUpdate(wdAttr.Buf, wdNlri.Buf, batch, addPath)
		if wd == nil {
			return false // build rejected (already logged); nothing was sent
		}
		return peer.sendUpdateWithSplit(wd, maxMsgSize, addPath) == nil
	case staleModify:
		// Non-LLGR iBGP peer: apply the depreference mods (NO_EXPORT + LOCAL_PREF=0).
		if modified == nil {
			return peer.sendUpdateWithSplit(update, maxMsgSize, addPath) == nil
		}
		return peer.sendBodyWithSplit(modified, maxMsgSize, addPath) == nil
	default: // staleKeep
		// LLGR-capable peer: send the stale route unchanged.
		return peer.sendUpdateWithSplit(update, maxMsgSize, addPath) == nil
	}
}

// staleOutcome is the per-peer decision of the readvertise egress filters.
type staleOutcome int

const (
	staleKeep     staleOutcome = iota // send the stale route unchanged (LLGR-capable peer)
	staleModify                       // send with attribute mods (non-LLGR iBGP depreference)
	staleWithdraw                     // send a withdrawal (non-LLGR eBGP)
	staleSuppress                     // a filter rejected the route for this peer
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
		if accept, _ := safeEgressFilter(f, src, dest, body, meta, &mods); !accept {
			return staleSuppress, nil
		}
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
				fwdLogger().Warn("stale re-advertise produced no payload, suppressing route",
					"peer", dest.Address)
			}
			return staleSuppress, nil
		}
		return staleModify, modified
	default:
		return staleKeep, nil
	}
}
