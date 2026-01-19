package plugin

import (
	"encoding/binary"
	"fmt"

	"codeberg.org/thomas-mangin/zebgp/internal/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/zebgp/internal/bgp/context"
	"codeberg.org/thomas-mangin/zebgp/internal/bgp/message"
	"codeberg.org/thomas-mangin/zebgp/internal/bgp/nlri"
)

// updateLengthFieldsSize is the fixed overhead in UPDATE body:
// 2 bytes for Withdrawn Routes Length + 2 bytes for Total Path Attribute Length.
const updateLengthFieldsSize = 4

// SplitWireUpdate splits a WireUpdate into multiple RFC-compliant UPDATEs.
// Each output fits within maxBodySize (excludes 19-byte header).
// Returns original in single-element slice if no split needed.
// Returns error if single NLRI > maxSize or baseAttrs alone exceeds maxSize.
//
// The srcCtx provides ADD-PATH state per AFI/SAFI for correct NLRI boundary detection.
// Pass nil if ADD-PATH is not enabled for any family.
//
// When the input contains multiple MP_REACH/MP_UNREACH attributes (uncommon but valid),
// IPv4 withdrawn and NLRI are only included in the first output UPDATE. Subsequent
// UPDATEs contain only MP_* attributes for their respective families.
//
// RFC 4271 Section 4.3 - UPDATE Message Handling.
// RFC 4760 Section 4 - MP_REACH_NLRI and MP_UNREACH_NLRI.
// RFC 7911 Section 3 - ADD-PATH encoding.
func SplitWireUpdate(wu *WireUpdate, maxBodySize int, srcCtx *bgpctx.EncodingContext) ([]*WireUpdate, error) {
	payload := wu.Payload()

	// Fast path: no split needed
	if len(payload) <= maxBodySize {
		return []*WireUpdate{wu}, nil
	}

	// Parse structure (offsets only, no allocation)
	if len(payload) < updateLengthFieldsSize {
		return nil, fmt.Errorf("split: %w", ErrUpdateTruncated)
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	withdrawnEnd := 2 + withdrawnLen
	if len(payload) < withdrawnEnd+2 {
		return nil, fmt.Errorf("split withdrawn: %w", ErrUpdateTruncated)
	}
	attrLen := int(binary.BigEndian.Uint16(payload[withdrawnEnd : withdrawnEnd+2]))
	attrStart := withdrawnEnd + 2
	attrEnd := attrStart + attrLen
	if len(payload) < attrEnd {
		return nil, fmt.Errorf("split attrs: %w", ErrUpdateTruncated)
	}

	// Extract components as wire slices
	ipv4Withdraws := payload[2:withdrawnEnd]
	attrs := payload[attrStart:attrEnd]
	ipv4NLRI := payload[attrEnd:]

	// Separate MP_REACH/MP_UNREACH from base attributes
	baseAttrs, mpReaches, mpUnreaches, err := separateMPAttributes(attrs)
	if err != nil {
		return nil, fmt.Errorf("parsing attributes: %w", err)
	}

	var results []*WireUpdate

	// Track remaining IPv4
	remIPv4W := ipv4Withdraws
	remIPv4A := ipv4NLRI

	// Process each MP_* combination (or just IPv4 if no MP_*)
	maxIter := max(len(mpReaches), len(mpUnreaches), 1)

	for i := 0; i < maxIter; i++ {
		var mpReach, mpUnreach []byte
		if i < len(mpReaches) {
			mpReach = mpReaches[i]
		}
		if i < len(mpUnreaches) {
			mpUnreach = mpUnreaches[i]
		}

		// IPv4 withdrawn/NLRI are only included in the first iteration.
		// When multiple MP_* families exist, subsequent iterations handle
		// only their respective MP_REACH/MP_UNREACH attributes.
		var useIPv4W, useIPv4A []byte
		if i == 0 {
			useIPv4W = remIPv4W
			useIPv4A = remIPv4A
		}

		// Build UPDATEs for this combination
		updates, err := buildCombinedUpdates(
			useIPv4W, baseAttrs, mpUnreach, mpReach, useIPv4A,
			maxBodySize, srcCtx, wu.SourceCtxID())
		if err != nil {
			return nil, err
		}
		results = append(results, updates...)
	}

	if len(results) == 0 {
		// Empty UPDATE - return original
		return []*WireUpdate{wu}, nil
	}

	// Copy sourceID to all split chunks (messageID left unset - each chunk is a new message)
	srcID := wu.SourceID()
	for _, chunk := range results {
		chunk.SetSourceID(srcID)
	}

	return results, nil
}

// separateMPAttributes extracts MP_REACH and MP_UNREACH from attributes.
// Returns: baseAttrs (without MP_*), []mpReach, []mpUnreach.
// Each mpReach/mpUnreach is a complete attribute with header.
func separateMPAttributes(attrs []byte) (base []byte, mpReaches, mpUnreaches [][]byte, err error) {
	var baseBuilder []byte
	pos := 0

	for pos < len(attrs) {
		if len(attrs) < pos+2 {
			return nil, nil, nil, fmt.Errorf("separate attrs: %w", ErrUpdateTruncated)
		}

		flags := attrs[pos]
		typeCode := attrs[pos+1]
		headerLen := 3       // flags + type + len(1)
		if flags&0x10 != 0 { // Extended length
			headerLen = 4
		}

		if len(attrs) < pos+headerLen {
			return nil, nil, nil, fmt.Errorf("separate attrs header: %w", ErrUpdateTruncated)
		}

		var attrLen int
		if flags&0x10 != 0 {
			attrLen = int(binary.BigEndian.Uint16(attrs[pos+2 : pos+4]))
		} else {
			attrLen = int(attrs[pos+2])
		}

		totalLen := headerLen + attrLen
		if len(attrs) < pos+totalLen {
			return nil, nil, nil, fmt.Errorf("separate attrs value: %w", ErrUpdateTruncated)
		}

		attrBytes := attrs[pos : pos+totalLen]

		switch attribute.AttributeCode(typeCode) { //nolint:exhaustive // Only MP_* need special handling
		case attribute.AttrMPReachNLRI:
			mpReaches = append(mpReaches, attrBytes)
		case attribute.AttrMPUnreachNLRI:
			mpUnreaches = append(mpUnreaches, attrBytes)
		default:
			baseBuilder = append(baseBuilder, attrBytes...)
		}

		pos += totalLen
	}

	return baseBuilder, mpReaches, mpUnreaches, nil
}

// splitIPv4NLRIs splits IPv4 unicast NLRIs (legacy UPDATE fields).
func splitIPv4NLRIs(data []byte, maxBytes int, ctx *bgpctx.EncodingContext) (fitting, remaining []byte, err error) {
	addPath := ctx.AddPathFor(nlri.IPv4Unicast)
	return message.SplitMPNLRI(data, nlri.AFIIPv4, nlri.SAFIUnicast, addPath, maxBytes)
}

// buildCombinedUpdates builds UPDATEs with mixed components, splitting if needed.
func buildCombinedUpdates(
	ipv4W, baseAttrs, mpUnreach, mpReach, ipv4A []byte,
	maxSize int, srcCtx *bgpctx.EncodingContext, sourceCtxID bgpctx.ContextID,
) ([]*WireUpdate, error) {
	// Fast path: everything fits
	total := updateLengthFieldsSize + len(ipv4W) + len(mpUnreach)
	hasAnnounces := len(mpReach) > 0 || len(ipv4A) > 0
	if hasAnnounces {
		total += len(baseAttrs) + len(mpReach) + len(ipv4A)
	}

	if total <= maxSize {
		if total == updateLengthFieldsSize && len(ipv4W) == 0 && len(mpUnreach) == 0 {
			return nil, nil // Empty
		}
		payload := buildUpdatePayload(ipv4W, baseAttrs, mpUnreach, mpReach, ipv4A)
		return []*WireUpdate{NewWireUpdate(payload, sourceCtxID)}, nil
	}

	// Check if baseAttrs alone exceeds available space (would cause infinite loop)
	minOverhead := updateLengthFieldsSize + len(baseAttrs)
	if hasAnnounces && minOverhead >= maxSize {
		return nil, fmt.Errorf("base attributes (%d bytes) too large for max message size (%d)",
			len(baseAttrs), maxSize)
	}

	// Slow path: iteratively fill and emit
	var results []*WireUpdate
	remIPv4W, remMPU, remMPR, remIPv4A := ipv4W, mpUnreach, mpReach, ipv4A

	for len(remIPv4W) > 0 || len(remMPU) > 0 || len(remMPR) > 0 || len(remIPv4A) > 0 {
		// Calculate overhead for this iteration
		iterHasAnnounces := len(remMPR) > 0 || len(remIPv4A) > 0
		overhead := updateLengthFieldsSize
		if iterHasAnnounces {
			overhead += len(baseAttrs)
		}

		available := maxSize - overhead

		// Guard: must have space for at least something
		if available <= 0 {
			return nil, fmt.Errorf("no space available after overhead (%d bytes)", overhead)
		}

		var fitIPv4W, fitMPU, fitMPR, fitIPv4A []byte
		madeProgress := false

		// Fill in order: IPv4 withdraws, MP_UNREACH, MP_REACH, IPv4 announces
		if len(remIPv4W) > 0 && available > 0 {
			fit, rest, err := splitIPv4NLRIs(remIPv4W, available, srcCtx)
			if err != nil {
				return nil, fmt.Errorf("split IPv4 withdraws: %w", err)
			}
			if len(fit) > 0 {
				fitIPv4W, remIPv4W = fit, rest
				available -= len(fit)
				madeProgress = true
			}
		}

		if len(remMPU) > 0 && available > 0 {
			fit, rest, err := splitMPUnreach(remMPU, available, srcCtx)
			if err != nil {
				return nil, fmt.Errorf("split MP_UNREACH: %w", err)
			}
			if len(fit) > 0 {
				fitMPU, remMPU = fit, rest
				available -= len(fit)
				madeProgress = true
			}
		}

		if len(remMPR) > 0 && available > 0 {
			fit, rest, err := splitMPReach(remMPR, available, srcCtx)
			if err != nil {
				return nil, fmt.Errorf("split MP_REACH: %w", err)
			}
			if len(fit) > 0 {
				fitMPR, remMPR = fit, rest
				available -= len(fit)
				madeProgress = true
			}
		}

		if len(remIPv4A) > 0 && available > 0 {
			fit, rest, err := splitIPv4NLRIs(remIPv4A, available, srcCtx)
			if err != nil {
				return nil, fmt.Errorf("split IPv4 announces: %w", err)
			}
			if len(fit) > 0 {
				fitIPv4A, remIPv4A = fit, rest
				madeProgress = true
			}
		}

		// Safety: ensure we made progress to avoid infinite loop
		if !madeProgress {
			return nil, fmt.Errorf("cannot make progress: remaining data does not fit in available space (%d bytes)", available)
		}

		// Emit UPDATE
		payload := buildUpdatePayload(fitIPv4W, baseAttrs, fitMPU, fitMPR, fitIPv4A)
		results = append(results, NewWireUpdate(payload, sourceCtxID))
	}

	return results, nil
}

// splitMPReach splits MP_REACH_NLRI to fit within maxBytes.
// Returns complete attributes with headers. NextHop preserved in each split.
// Uses srcCtx to determine ADD-PATH state for the attribute's AFI/SAFI.
func splitMPReach(attr []byte, maxBytes int, srcCtx *bgpctx.EncodingContext) (fitting, remaining []byte, err error) {
	if maxBytes <= 0 {
		return nil, nil, fmt.Errorf("invalid maxBytes: %d", maxBytes)
	}

	if len(attr) <= maxBytes {
		return attr, nil, nil
	}

	// Parse MP_REACH structure
	// Header: flags(1) + type(1) + len(1-2)
	flags := attr[0]
	headerLen := 3
	if flags&0x10 != 0 {
		headerLen = 4
	}

	// AFI(2) + SAFI(1) + NH_Len(1) + NextHop(var) + Reserved(1) + NLRIs
	if len(attr) < headerLen+4 {
		return nil, nil, fmt.Errorf("mp_reach: %w", ErrUpdateMalformed)
	}

	afi := binary.BigEndian.Uint16(attr[headerLen : headerLen+2])
	safi := attr[headerLen+2]
	nhLen := int(attr[headerLen+3])
	nlriStart := headerLen + 4 + nhLen + 1 // +1 for reserved byte

	if len(attr) < nlriStart {
		return nil, nil, fmt.Errorf("mp_reach nexthop: %w", ErrUpdateTruncated)
	}

	// Fixed part: AFI/SAFI + NH (must be in every split)
	fixedPart := attr[headerLen:nlriStart]
	nlris := attr[nlriStart:]

	// Available space for NLRIs
	availableForNLRI := maxBytes - len(fixedPart) - headerLen
	if availableForNLRI <= 0 {
		return nil, nil, fmt.Errorf("MP_REACH fixed part (%d) exceeds max (%d)", len(fixedPart)+headerLen, maxBytes)
	}

	// Get ADD-PATH state for this specific AFI/SAFI
	addPath := srcCtx.AddPathFor(nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)})

	fitNLRI, restNLRI, err := message.SplitMPNLRI(nlris, nlri.AFI(afi), nlri.SAFI(safi), addPath, availableForNLRI)
	if err != nil {
		return nil, nil, err
	}

	// Build fitting attribute
	fitting = buildMPAttribute(flags, attribute.AttrMPReachNLRI, fixedPart, fitNLRI)

	// Build remaining if any
	if len(restNLRI) > 0 {
		remaining = buildMPAttribute(flags, attribute.AttrMPReachNLRI, fixedPart, restNLRI)
	}

	return fitting, remaining, nil
}

// splitMPUnreach splits MP_UNREACH_NLRI to fit within maxBytes.
// Uses srcCtx to determine ADD-PATH state for the attribute's AFI/SAFI.
func splitMPUnreach(attr []byte, maxBytes int, srcCtx *bgpctx.EncodingContext) (fitting, remaining []byte, err error) {
	if maxBytes <= 0 {
		return nil, nil, fmt.Errorf("invalid maxBytes: %d", maxBytes)
	}

	if len(attr) <= maxBytes {
		return attr, nil, nil
	}

	// Parse MP_UNREACH structure
	// Header: flags(1) + type(1) + len(1-2)
	flags := attr[0]
	headerLen := 3
	if flags&0x10 != 0 {
		headerLen = 4
	}

	// AFI(2) + SAFI(1) + NLRIs
	if len(attr) < headerLen+3 {
		return nil, nil, fmt.Errorf("mp_unreach: %w", ErrUpdateMalformed)
	}

	afi := binary.BigEndian.Uint16(attr[headerLen : headerLen+2])
	safi := attr[headerLen+2]
	nlriStart := headerLen + 3

	fixedPart := attr[headerLen:nlriStart]
	nlris := attr[nlriStart:]

	availableForNLRI := maxBytes - len(fixedPart) - headerLen
	if availableForNLRI <= 0 {
		return nil, nil, fmt.Errorf("MP_UNREACH fixed part (%d) exceeds max (%d)", len(fixedPart)+headerLen, maxBytes)
	}

	// Get ADD-PATH state for this specific AFI/SAFI
	addPath := srcCtx.AddPathFor(nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)})

	fitNLRI, restNLRI, err := message.SplitMPNLRI(nlris, nlri.AFI(afi), nlri.SAFI(safi), addPath, availableForNLRI)
	if err != nil {
		return nil, nil, err
	}

	fitting = buildMPAttribute(flags, attribute.AttrMPUnreachNLRI, fixedPart, fitNLRI)

	if len(restNLRI) > 0 {
		remaining = buildMPAttribute(flags, attribute.AttrMPUnreachNLRI, fixedPart, restNLRI)
	}

	return fitting, remaining, nil
}

// buildMPAttribute constructs MP_REACH or MP_UNREACH with correct length/flags.
func buildMPAttribute(origFlags byte, typeCode attribute.AttributeCode, afiSafiNH []byte, nlris []byte) []byte {
	valueLen := len(afiSafiNH) + len(nlris)

	// Determine if extended length needed
	useExtended := valueLen > 255

	headerLen := 3
	if useExtended {
		headerLen = 4
	}

	buf := make([]byte, headerLen+valueLen)

	// Flags: preserve Optional/Transitive/Partial, set Extended if needed
	flags := origFlags & 0xE0 // Keep O/T/P bits
	if useExtended {
		flags |= 0x10
	}
	buf[0] = flags
	buf[1] = byte(typeCode)

	if useExtended {
		binary.BigEndian.PutUint16(buf[2:4], uint16(valueLen)) //nolint:gosec // G115: valueLen bounded by BGP max attr size
	} else {
		buf[2] = byte(valueLen)
	}

	copy(buf[headerLen:], afiSafiNH)
	copy(buf[headerLen+len(afiSafiNH):], nlris)

	return buf
}

// buildUpdatePayload builds UPDATE body with any combination of components.
// Includes baseAttrs only if announces present (MP_REACH or IPv4 NLRI).
func buildUpdatePayload(ipv4Withdraws, baseAttrs, mpUnreach, mpReach, ipv4NLRI []byte) []byte {
	wLen := len(ipv4Withdraws)
	hasAnnounces := len(mpReach) > 0 || len(ipv4NLRI) > 0

	// Attrs: baseAttrs only if announces present
	var aLen int
	if hasAnnounces {
		aLen = len(baseAttrs) + len(mpUnreach) + len(mpReach)
	} else {
		aLen = len(mpUnreach)
	}

	buf := make([]byte, 2+wLen+2+aLen+len(ipv4NLRI))

	// Withdrawn routes
	binary.BigEndian.PutUint16(buf[0:], uint16(wLen)) //nolint:gosec // G115: wLen bounded by BGP max message size
	copy(buf[2:], ipv4Withdraws)

	// Attributes
	pos := 2 + wLen
	binary.BigEndian.PutUint16(buf[pos:], uint16(aLen)) //nolint:gosec // G115: aLen bounded by BGP max message size
	pos += 2

	if hasAnnounces {
		copy(buf[pos:], baseAttrs)
		pos += len(baseAttrs)
	}
	copy(buf[pos:], mpUnreach)
	pos += len(mpUnreach)
	copy(buf[pos:], mpReach)
	pos += len(mpReach)

	// NLRI
	copy(buf[pos:], ipv4NLRI)

	return buf
}
