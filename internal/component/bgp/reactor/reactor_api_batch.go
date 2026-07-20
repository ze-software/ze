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

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/filterapi"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/route"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/selector"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/rib"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/nlri"
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
	var userASPath []uint32

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
				userASPath = asp.Segments[0].ASNs
			}
		}
	case batch.Attrs != nil:
		// Use Builder for new routes
		attrs = batch.Attrs.ToAttributes()
		userASPath = batch.Attrs.ASPathSlice()
	default: // no attributes provided — use defaults
		attrs = append(attrs, attribute.OriginIGP)
	}

	var lastErr error
	var acceptedCount int

	// Group-aware path: when update groups are enabled, collect established
	// peers with identical build parameters and build the UPDATE once per group.
	// Falls back to per-peer when disabled or when peers differ.
	type announceBuildKey struct {
		nextHop  netip.Addr
		isIBGP   bool
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
		isIBGP := peer.Settings().IsIBGP()

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
				maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))
				addPath := peer.addPathFor(batch.Family)
				asn4 := peer.asn4()

				attrHandle := getBuildBuf()
				nlriHandle := getBuildBuf()
				update := a.buildBatchAnnounceUpdate(attrHandle.Buf, nlriHandle.Buf, batch, nextHop, isIBGP, asn4, addPath, peer.Settings().LocalAS)

				if err := peer.sendUpdateWithSplit(update, maxMsgSize, addPath); err != nil {
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
			asPath := a.buildBatchASPath(userASPath, batch.OriginAS, isIBGP, peer.Settings().LocalAS)
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
		maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, bg.key.extended))

		attrHandle := getBuildBuf()
		nlriHandle := getBuildBuf()
		update := a.buildBatchAnnounceUpdate(attrHandle.Buf, nlriHandle.Buf, batch, bg.nextHop, bg.key.isIBGP, bg.key.asn4, bg.key.addPath, bg.key.localAS)

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

	// Return warning-level error if no peers accepted (all skipped due to family)
	if acceptedCount == 0 {
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
				maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))
				addPath := peer.addPathFor(batch.Family)

				attrHandle := getBuildBuf()
				nlriHandle := getBuildBuf()
				update := a.buildBatchWithdrawUpdate(attrHandle.Buf, nlriHandle.Buf, batch, addPath)

				if err := peer.sendUpdateWithSplit(update, maxMsgSize, addPath); err != nil {
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
		maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, wg.key.extended))

		attrHandle := getBuildBuf()
		nlriHandle := getBuildBuf()
		update := a.buildBatchWithdrawUpdate(attrHandle.Buf, nlriHandle.Buf, batch, wg.key.addPath)

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

	// Return warning-level error if no peers accepted (all skipped due to family)
	if acceptedCount == 0 {
		return route.ErrNoPeersAcceptedFamily
	}
	return lastErr
}

// buildBatchASPath builds AS_PATH for batch operations.
// RFC 4271 §5.1.2: iBGP SHALL NOT modify AS_PATH; eBGP prepends local AS.
func (a *reactorAPIAdapter) buildBatchASPath(userASPath []uint32, originAS uint32, isIBGP bool, localAS uint32) *attribute.ASPath {
	switch {
	case len(userASPath) > 0:
		// Verbatim explicit as-path (route-server transparency): sent as-is.
		return &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: userASPath},
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

// buildBatchAnnounceUpdate builds an UPDATE message for a batch of NLRIs.
// attrBuf and nlriBuf are caller-provided buffers (from buildBufPool).
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families.
func (a *reactorAPIAdapter) buildBatchAnnounceUpdate(attrBuf, nlriBuf []byte, batch bgptypes.NLRIBatch, nextHop netip.Addr, isIBGP, asn4, addPath bool, localAS uint32) *message.Update {
	// Write NLRIs into caller-provided buffer
	nlriOff := 0
	for _, n := range batch.NLRIs {
		nlriOff += nlri.WriteNLRI(n, nlriBuf, nlriOff, addPath)
	}
	nlriBytes := nlriBuf[:nlriOff]

	// Wire mode: ensure mandatory attributes present, then add NEXT_HOP or MP_REACH_NLRI
	if batch.Wire != nil {
		hadASPath, _ := batch.Wire.Has(attribute.AttrASPath)
		attrOff := a.writeMandatoryAttrs(attrBuf, batch.Wire, isIBGP, asn4, localAS, batch.OriginAS)
		update := a.buildWireModeUpdate(attrBuf, attrOff, nlriBytes, batch.Family, nextHop, isIBGP)
		if !hadASPath {
			a.appendAnnounceAS4Path(update, attrBuf, isIBGP, asn4, localAS, batch.OriginAS)
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
	attrOff := a.writeMandatoryAttrs(attrBuf, wire, isIBGP, asn4, localAS, batch.OriginAS)

	// Add NEXT_HOP or MP_REACH_NLRI
	update := a.buildWireModeUpdate(attrBuf, attrOff, nlriBytes, batch.Family, nextHop, isIBGP)
	if !hadASPath {
		a.appendAnnounceAS4Path(update, attrBuf, isIBGP, asn4, localAS, batch.OriginAS)
	}
	return update
}

// appendAnnounceAS4Path appends an AS4_PATH attribute to update's PathAttributes
// (which alias attrBuf) when writeASPath synthesized a two-octet AS_PATH that had
// to carry AS_TRANS toward an OLD peer. It is called only when this builder
// synthesized the AS_PATH -- a verbatim AS_PATH copied from batch.Wire/Attrs owns
// its own encoding. AS4_PATH's type code (17) is higher than every attribute this
// builder emits (NEXT_HOP 3, LOCAL_PREF 5, communities 8, MP_REACH 14), so
// appending it last keeps the attributes in type-code order.
func (a *reactorAPIAdapter) appendAnnounceAS4Path(update *message.Update, attrBuf []byte, isIBGP, asn4 bool, localAS, originAS uint32) {
	off := len(update.PathAttributes)
	n := writeAnnounceAS4Path(attrBuf, off, isIBGP, asn4, localAS, originAS)
	if n == 0 {
		return
	}
	update.PathAttributes = attrBuf[:off+n]
}

// buildWireModeUpdate builds UPDATE using pre-written attribute bytes in attrBuf[:attrOff].
// Inserts NEXT_HOP (IPv4 unicast) or appends MP_REACH_NLRI (other families).
// attrBuf[:attrOff] must contain mandatory attrs from writeMandatoryAttrs.
// RFC 4271: NEXT_HOP (type 3) must come after AS_PATH (type 2) but before other attrs.
// RFC 4271 §5.1.5: LOCAL_PREF is well-known mandatory for iBGP sessions.
func (a *reactorAPIAdapter) buildWireModeUpdate(attrBuf []byte, attrOff int, nlriBytes []byte, fam family.Family, nextHop netip.Addr, isIBGP bool) *message.Update {
	isIPv4Unicast := fam == (family.IPv4Unicast)

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

			// Insert NEXT_HOP after AS_PATH for correct type code order.
			insertPos := a.findNextHopInsertPosition(attrBuf[:attrOff])
			nhSize := 7 // NEXT_HOP is 7 bytes (3 header + 4 IP)
			// Shift tail right to make room for NEXT_HOP (copy handles overlap).
			copy(attrBuf[insertPos+nhSize:], attrBuf[insertPos:attrOff])
			nh := &attribute.NextHop{Addr: nextHop}
			attribute.WriteAttrTo(nh, attrBuf, insertPos)
			attrOff += nhSize
		}

		// Append LOCAL_PREF=100 at end if needed for iBGP.
		if isIBGP && !a.hasAttribute(attrBuf[:attrOff], attribute.AttrLocalPref) {
			lp := attribute.LocalPref(100)
			attrOff += attribute.WriteAttrTo(lp, attrBuf, attrOff)
		}

		return &message.Update{
			PathAttributes: attrBuf[:attrOff],
			NLRI:           nlriBytes,
		}
	}

	// Non-IPv4 unicast: append LOCAL_PREF and MP_REACH_NLRI to existing attrs. As with
	// NEXT_HOP above, a relayed/replayed block may already carry an MP_REACH_NLRI; drop
	// it before appending the authoritative one so the route is not duplicated (RFC 7606
	// Section 3(g)).
	attrOff = a.stripAttribute(attrBuf, attrOff, attribute.AttrMPReachNLRI)
	hasLocalPref := a.hasAttribute(attrBuf[:attrOff], attribute.AttrLocalPref)
	if isIBGP && !hasLocalPref {
		lp := attribute.LocalPref(100)
		attrOff += attribute.WriteAttrTo(lp, attrBuf, attrOff)
	}

	mpReach := attribute.NewMPReachNLRI(attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI), []netip.Addr{nextHop}, nlriBytes)
	attrOff += attribute.WriteAttrTo(mpReach, attrBuf, attrOff)

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
// writing the result into buf. Returns bytes written.
// RFC 4271 Section 5.1.1: ORIGIN is a well-known mandatory attribute.
// RFC 4271 Section 5.1.2: AS_PATH is a well-known mandatory attribute.
// RFC 4271 Section 5.1: Attributes must appear in type code order.
// If missing, adds defaults: ORIGIN=IGP, AS_PATH per iBGP/eBGP rules.
// localAS is the peer-specific local AS (used for AS_PATH prepend when missing).
func (a *reactorAPIAdapter) writeMandatoryAttrs(buf []byte, wire *attribute.AttributesWire, isIBGP, asn4 bool, localAS, originAS uint32) int {
	hasOrigin, _ := wire.Has(attribute.AttrOrigin)
	hasASPath, _ := wire.Has(attribute.AttrASPath)
	packed := wire.Packed()

	if hasOrigin && hasASPath {
		copy(buf, packed)
		return len(packed)
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

		copy(buf[off:], packed)
		return off + len(packed)
	}

	// Case 2: Only ORIGIN missing - prepend ORIGIN, copy rest
	if !hasOrigin {
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
	copy(buf, packed[:originEnd])
	off = originEnd

	// Insert AS_PATH
	off += a.writeASPath(buf[off:], isIBGP, asn4, localAS, originAS)

	// Copy remaining attributes
	copy(buf[off:], packed[originEnd:])
	return off + len(packed) - originEnd
}

// findNextHopInsertPosition finds where to insert NEXT_HOP (type 3) in wire attrs.
// RFC 4271: attributes should be in type code order.
// Returns position after AS_PATH (type 2) or at end if no attrs with type > 2.
func (a *reactorAPIAdapter) findNextHopInsertPosition(wireAttrs []byte) int {
	pos := 0
	for pos < len(wireAttrs) {
		if pos+2 > len(wireAttrs) {
			break
		}
		flags := wireAttrs[pos]
		typeCode := wireAttrs[pos+1]

		// If we find an attr with type >= 3, insert NEXT_HOP here
		if typeCode >= 3 {
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
	// No attr with type >= 3 found, insert at end
	return pos
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

// writeAnnounceAS4Path appends an AS4_PATH attribute into buf at off when the
// AS_PATH synthesized by announceASPathASNs had to substitute AS_TRANS toward an
// OLD (2-octet) peer, returning bytes written (0 when none is needed).
//
// RFC 6793 §4.2.2: a NEW speaker sending a two-octet AS_PATH that contains a
// non-mappable AS MUST also send the AS4_PATH (four-octet encoding of the same
// sequence); when every AS is mappable it MUST NOT. asn4 == true means the peer
// negotiated 4-octet support, so AS_PATH already carries the real ASNs and no
// AS4_PATH is sent.
func writeAnnounceAS4Path(buf []byte, off int, isIBGP, asn4 bool, localAS, originAS uint32) int {
	var scratch [2]uint32
	return writeAS4PathForASNs(buf, off, asn4, announceASPathASNs(scratch[:0], isIBGP, localAS, originAS))
}

// buildBatchWithdrawUpdate builds an UPDATE message for withdrawing a batch of NLRIs.
// attrBuf and nlriBuf are caller-provided buffers (from buildBufPool).
// RFC 4271 Section 4.3: Withdrawn Routes field.
// RFC 4760: MP_UNREACH_NLRI for non-IPv4-unicast families.
func (a *reactorAPIAdapter) buildBatchWithdrawUpdate(attrBuf, nlriBuf []byte, batch bgptypes.NLRIBatch, addPath bool) *message.Update {
	// Write NLRIs into caller-provided buffer
	nlriOff := 0
	for _, n := range batch.NLRIs {
		nlriOff += nlri.WriteNLRI(n, nlriBuf, nlriOff, addPath)
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
	attrLen := attribute.WriteAttrTo(mpUnreach, attrBuf, 0)
	return &message.Update{
		PathAttributes: attrBuf[:attrLen],
	}
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

		// Write NLRIs into pooled buffer
		nlriHandle := getBuildBuf()
		off := 0
		for _, n := range nlris {
			off += nlri.WriteNLRI(n, nlriHandle.Buf, off, addPath)
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
	maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))
	addPath := peer.addPathFor(batch.Family)
	asn4 := peer.asn4()
	localAS := peer.Settings().LocalAS

	attrHandle := getBuildBuf()
	nlriHandle := getBuildBuf()
	defer putBuildBuf(attrHandle)
	defer putBuildBuf(nlriHandle)
	update := a.buildBatchAnnounceUpdate(attrHandle.Buf, nlriHandle.Buf, batch, nextHop, isIBGP, asn4, addPath, localAS)

	// Run the readvertise egress filters. LLGREgressFilter keys off meta["stale"]
	// and the destination peer's LLGR capability; it writes into mods.
	body := fwdPackUpdateBody(update)
	dest := filterapi.PeerFilterInfo{
		Address: peer.Settings().Address,
		PeerAS:  peer.Settings().PeerAS,
		LocalAS: localAS,
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
		if !safeEgressFilter(f, src, dest, body, meta, &mods) {
			return staleSuppress, nil
		}
	}
	switch {
	case mods.IsWithdraw():
		return staleWithdraw, nil
	case mods.HasModifications():
		modified, _ := buildModifiedPayload(body, &mods, a.r.attrModHandlers, nil, nil)
		return staleModify, modified
	default:
		return staleKeep, nil
	}
}
