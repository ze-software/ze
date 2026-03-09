// Design: docs/architecture/core-design.md — NLRI batch announce/withdraw and wire attribute building
// Overview: reactor_api.go — API command handling core
// Related: reactor_api_forward.go — forwarding and grouped sending
package reactor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/route"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/rib"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
)

// AnnounceNLRIBatch announces a batch of NLRIs with shared attributes.
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families.
// RFC 8654: Respects peer's max message size (4096 or 65535).
func (a *reactorAPIAdapter) AnnounceNLRIBatch(peerSelector string, batch bgptypes.NLRIBatch) error {
	peers := a.getMatchingPeers(peerSelector)
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

	for _, peer := range peers {
		isIBGP := peer.Settings().IsIBGP()

		// Resolve next-hop per peer using RouteNextHop policy
		nextHop, nhErr := peer.resolveNextHop(batch.NextHop, batch.Family)
		if nhErr != nil {
			// Log but continue - skip this peer if next-hop can't be resolved
			routesLogger().Debug("next-hop resolution failed", "peer", peer.Settings().Address, "error", nhErr)
			continue
		}

		// Build AS_PATH per peer (iBGP vs eBGP)
		asPath := a.buildBatchASPath(userASPath, isIBGP)

		if !peer.ShouldQueue() {
			// Check family negotiation
			nc := peer.negotiated.Load()
			if nc == nil || !nc.Has(batch.Family) {
				continue // Skip peer that doesn't support this family
			}

			// Get max message size from capabilities
			// RFC 8654: 65535 if ExtendedMessage, else 4096
			maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))
			addPath := peer.addPathFor(batch.Family)
			asn4 := peer.asn4()

			// Build UPDATE message for this batch using pooled buffers
			attrBuf := getBuildBuf()
			nlriBuf := getBuildBuf()
			update := a.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch, nextHop, isIBGP, asn4, addPath)

			// Send with splitting for large batches
			// RFC 4271: Each split UPDATE is self-contained with full attributes
			if err := peer.sendUpdateWithSplit(update, maxMsgSize, addPath); err != nil {
				lastErr = err
			} else {
				acceptedCount++
			}
			putBuildBuf(attrBuf)
			putBuildBuf(nlriBuf)
		} else {
			// Session not established or queue draining: queue to preserve order
			for _, n := range batch.NLRIs {
				ribRoute := rib.NewRouteWithASPath(n, nextHop, attrs, asPath)
				peer.QueueAnnounce(ribRoute)
			}
			acceptedCount++ // Queued counts as accepted
		}
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
func (a *reactorAPIAdapter) WithdrawNLRIBatch(peerSelector string, batch bgptypes.NLRIBatch) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return route.ErrNoPeersMatch
	}

	var lastErr error
	var acceptedCount int

	for _, peer := range peers {
		if !peer.ShouldQueue() {
			// Check family negotiation
			nc := peer.negotiated.Load()
			if nc == nil || !nc.Has(batch.Family) {
				continue // Skip peer that doesn't support this family
			}

			// Get max message size from capabilities
			maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))
			addPath := peer.addPathFor(batch.Family)

			// Build withdraw UPDATE for this batch using pooled buffers
			attrBuf := getBuildBuf()
			nlriBuf := getBuildBuf()
			update := a.buildBatchWithdrawUpdate(attrBuf, nlriBuf, batch, addPath)

			// Send with splitting for large batches
			if err := peer.sendUpdateWithSplit(update, maxMsgSize, addPath); err != nil {
				lastErr = err
			} else {
				acceptedCount++
			}
			putBuildBuf(attrBuf)
			putBuildBuf(nlriBuf)
		} else {
			// Session not established or queue draining: queue to preserve order
			for _, n := range batch.NLRIs {
				peer.QueueWithdraw(n)
			}
			acceptedCount++ // Queued counts as accepted
		}
	}

	// Return warning-level error if no peers accepted (all skipped due to family)
	if acceptedCount == 0 {
		return route.ErrNoPeersAcceptedFamily
	}
	return lastErr
}

// buildBatchASPath builds AS_PATH for batch operations.
// RFC 4271 §5.1.2: iBGP SHALL NOT modify AS_PATH; eBGP prepends local AS.
func (a *reactorAPIAdapter) buildBatchASPath(userASPath []uint32, isIBGP bool) *attribute.ASPath {
	switch {
	case len(userASPath) > 0:
		return &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: userASPath},
			},
		}
	case isIBGP:
		return &attribute.ASPath{Segments: nil}
	default: // eBGP: prepend local AS
		return &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{a.r.config.LocalAS}},
			},
		}
	}
}

// buildBatchAnnounceUpdate builds an UPDATE message for a batch of NLRIs.
// attrBuf and nlriBuf are caller-provided buffers (from buildBufPool).
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families.
func (a *reactorAPIAdapter) buildBatchAnnounceUpdate(attrBuf, nlriBuf []byte, batch bgptypes.NLRIBatch, nextHop netip.Addr, isIBGP, asn4, addPath bool) *message.Update {
	// Write NLRIs into caller-provided buffer
	nlriOff := 0
	for _, n := range batch.NLRIs {
		nlriOff += nlri.WriteNLRI(n, nlriBuf, nlriOff, addPath)
	}
	nlriBytes := nlriBuf[:nlriOff]

	// Wire mode: ensure mandatory attributes present, then add NEXT_HOP or MP_REACH_NLRI
	if batch.Wire != nil {
		attrOff := a.writeMandatoryAttrs(attrBuf, batch.Wire, isIBGP, asn4)
		return a.buildWireModeUpdate(attrBuf, attrOff, nlriBytes, batch.Family, nextHop, isIBGP)
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
	attrOff := a.writeMandatoryAttrs(attrBuf, wire, isIBGP, asn4)

	// Add NEXT_HOP or MP_REACH_NLRI
	return a.buildWireModeUpdate(attrBuf, attrOff, nlriBytes, batch.Family, nextHop, isIBGP)
}

// buildWireModeUpdate builds UPDATE using pre-written attribute bytes in attrBuf[:attrOff].
// Inserts NEXT_HOP (IPv4 unicast) or appends MP_REACH_NLRI (other families).
// attrBuf[:attrOff] must contain mandatory attrs from writeMandatoryAttrs.
// RFC 4271: NEXT_HOP (type 3) must come after AS_PATH (type 2) but before other attrs.
// RFC 4271 §5.1.5: LOCAL_PREF is well-known mandatory for iBGP sessions.
func (a *reactorAPIAdapter) buildWireModeUpdate(attrBuf []byte, attrOff int, nlriBytes []byte, family nlri.Family, nextHop netip.Addr, isIBGP bool) *message.Update {
	isIPv4Unicast := family == nlri.IPv4Unicast

	if isIPv4Unicast {
		// IPv4 unicast: insert NEXT_HOP after AS_PATH for correct type code order
		wireAttrs := attrBuf[:attrOff]
		insertPos := a.findNextHopInsertPosition(wireAttrs)
		hasLocalPref := a.hasAttribute(wireAttrs, attribute.AttrLocalPref)

		nhSize := 7 // NEXT_HOP is 7 bytes (3 header + 4 IP)

		// Shift tail right to make room for NEXT_HOP (copy handles overlap)
		copy(attrBuf[insertPos+nhSize:], attrBuf[insertPos:attrOff])

		// Write NEXT_HOP at insert position
		nh := &attribute.NextHop{Addr: nextHop}
		attribute.WriteAttrTo(nh, attrBuf, insertPos)
		attrOff += nhSize

		// Append LOCAL_PREF=100 at end if needed for iBGP
		if isIBGP && !hasLocalPref {
			lp := attribute.LocalPref(100)
			attrOff += attribute.WriteAttrTo(lp, attrBuf, attrOff)
		}

		return &message.Update{
			PathAttributes: attrBuf[:attrOff],
			NLRI:           nlriBytes,
		}
	}

	// Non-IPv4 unicast: append LOCAL_PREF and MP_REACH_NLRI to existing attrs
	hasLocalPref := a.hasAttribute(attrBuf[:attrOff], attribute.AttrLocalPref)
	if isIBGP && !hasLocalPref {
		lp := attribute.LocalPref(100)
		attrOff += attribute.WriteAttrTo(lp, attrBuf, attrOff)
	}

	mpReach := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(family.AFI),
		SAFI:     attribute.SAFI(family.SAFI),
		NextHops: []netip.Addr{nextHop},
		NLRI:     nlriBytes,
	}
	attrOff += attribute.WriteAttrTo(mpReach, attrBuf, attrOff)

	return &message.Update{
		PathAttributes: attrBuf[:attrOff],
	}
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
func (a *reactorAPIAdapter) writeMandatoryAttrs(buf []byte, wire *attribute.AttributesWire, isIBGP, asn4 bool) int {
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
		off += a.writeASPath(buf[off:], isIBGP, asn4)

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
	off += a.writeASPath(buf[off:], isIBGP, asn4)

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

// writeASPath writes AS_PATH attribute to buf, returning bytes written.
func (a *reactorAPIAdapter) writeASPath(buf []byte, isIBGP, asn4 bool) int {
	switch {
	case isIBGP:
		buf[0] = 0x40 // Transitive
		buf[1] = 2    // AS_PATH
		buf[2] = 0    // Length = 0 (empty)
		return 3
	case asn4:
		buf[0] = 0x40 // Transitive
		buf[1] = 2    // AS_PATH
		buf[2] = 6    // Length: 2 (segment header) + 4 (ASN)
		buf[3] = byte(attribute.ASSequence)
		buf[4] = 1 // Count = 1
		binary.BigEndian.PutUint32(buf[5:], a.r.config.LocalAS)
		return 9
	default: // ASN2 eBGP
		buf[0] = 0x40 // Transitive
		buf[1] = 2    // AS_PATH
		buf[2] = 4    // Length: 2 (segment header) + 2 (ASN)
		buf[3] = byte(attribute.ASSequence)
		buf[4] = 1                                                      // Count = 1
		binary.BigEndian.PutUint16(buf[5:], uint16(a.r.config.LocalAS)) //nolint:gosec // LocalAS validated ≤ 65535 in ASN2 path
		return 7
	}
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

	if batch.Family == nlri.IPv4Unicast {
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
func (a *reactorAPIAdapter) SendRoutes(peerSelector string, routes []*rib.Route, withdrawals []nlri.NLRI, sendEOR bool) (bgptypes.TransactionResult, error) {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return bgptypes.TransactionResult{}, errors.New("no peers match selector")
	}

	var totalResult bgptypes.TransactionResult

	// Collect families for EOR (from both routes and withdrawals)
	seen := make(map[nlri.Family]bool)
	for _, r := range routes {
		seen[r.NLRI().Family()] = true
	}
	for _, n := range withdrawals {
		seen[n.Family()] = true
	}
	families := make([]nlri.Family, 0, len(seen))
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
	byFamily := make(map[nlri.Family][]nlri.NLRI)
	for _, n := range withdrawals {
		f := n.Family()
		byFamily[f] = append(byFamily[f], n)
	}

	updatesSent := 0
	ipv4Unicast := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	for family, nlris := range byFamily {
		// RFC 7911: Get ADD-PATH encoding setting
		addPath := peer.addPathFor(family)
		var update *message.Update

		// Write NLRIs into pooled buffer
		nlriBuf := getBuildBuf()
		off := 0
		for _, n := range nlris {
			off += nlri.WriteNLRI(n, nlriBuf, off, addPath)
		}
		nlriBytes := nlriBuf[:off]

		if family == ipv4Unicast {
			// IPv4 unicast: use WithdrawnRoutes field
			update = &message.Update{
				WithdrawnRoutes: nlriBytes,
			}
		} else {
			// Other families: use MP_UNREACH_NLRI attribute
			mpUnreach := &attribute.MPUnreachNLRI{
				AFI:  attribute.AFI(family.AFI),
				SAFI: attribute.SAFI(family.SAFI),
				NLRI: nlriBytes,
			}
			attrBuf := getBuildBuf()
			attrLen := attribute.WriteAttrTo(mpUnreach, attrBuf, 0)
			update = &message.Update{
				PathAttributes: attrBuf[:attrLen],
			}
			// Send then return attr buffer (nlri already copied into attrBuf by WriteAttrTo)
			if err := peer.SendUpdate(update); err == nil {
				updatesSent++
			}
			putBuildBuf(attrBuf)
			putBuildBuf(nlriBuf)
			continue
		}

		if err := peer.SendUpdate(update); err == nil {
			updatesSent++
		}
		putBuildBuf(nlriBuf)
	}

	return updatesSent
}
