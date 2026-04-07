// Design: docs/architecture/wire/messages.md — BGP message types
// Overview: update.go — UPDATE message wire representation
// Related: update_build.go — UPDATE builder infrastructure
// Related: chunk_mp_nlri.go — MP NLRI chunking for multi-family splitting

package message

import (
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// Errors for UPDATE splitting and bounds checking.
var (
	// ErrAttributesTooLarge is returned when path attributes alone exceed maxSize.
	ErrAttributesTooLarge = errors.New("attributes exceed max message size")

	// ErrNLRITooLarge is returned when a single NLRI exceeds available space.
	ErrNLRITooLarge = errors.New("single NLRI exceeds available space")

	// ErrMPOverheadTooLarge is returned when MP attribute overhead exceeds maxAttrSize.
	ErrMPOverheadTooLarge = errors.New("MP attribute overhead exceeds max size")

	// ErrUpdateTooLarge is returned when a single-route UPDATE exceeds maxSize.
	// RFC 4271 Section 4.3: UPDATE max 4096 bytes (standard).
	// RFC 8654: Extended Message raises max to 65535 bytes.
	// Single-route builders return this when the atomic route cannot fit.
	ErrUpdateTooLarge = errors.New("UPDATE message exceeds max size")
)

// SplitUpdate splits an UPDATE into chunks respecting maxSize.
//
// This is a convenience wrapper that assumes Add-Path is not enabled.
// Use SplitUpdateWithAddPath when Add-Path capability is negotiated.
//
// RFC 4271 Section 4.3: Each UPDATE is self-contained with full attributes.
func SplitUpdate(u *Update, maxSize int) ([]*Update, error) {
	return SplitUpdateWithAddPath(u, maxSize, false)
}

// SplitUpdateWithAddPath splits an UPDATE into chunks respecting maxSize.
//
// The addPath parameter indicates whether Add-Path (RFC 7911) is negotiated
// for this family. This affects NLRI boundary detection during splitting.
//
// Handles both IPv4 (NLRI field) and MP families (MP_REACH_NLRI attribute):
// - IPv4 announcements: split u.NLRI via ChunkMPNLRI
// - IPv4 withdrawals: split u.WithdrawnRoutes via ChunkMPNLRI
// - MP announcements: detect MP_REACH_NLRI in PathAttributes, split via SplitMPReachNLRI
// - MP withdrawals: detect MP_UNREACH_NLRI in PathAttributes, split via SplitMPUnreachNLRI
//
// WIRE CACHE PRESERVATION:
// - For IPv4: u.PathAttributes reused directly in all chunks (zero-copy)
// - For MP: other attributes preserved, MP_REACH/UNREACH rebuilt per chunk
//
// Returns error if:
// - Attributes alone exceed maxSize (ErrAttributesTooLarge)
// - Single NLRI exceeds available space (ErrNLRITooLarge)
//
// Note: maxSize is always 4096 or 65535 from MaxMessageLength() - no validation needed.
//
// RFC 4271 Section 4.3: Each UPDATE is self-contained with full attributes.
// RFC 7911: Add-Path requires 4-byte path identifier before each NLRI.
func SplitUpdateWithAddPath(u *Update, maxSize int, addPath bool) ([]*Update, error) {
	// Empty UPDATE (End-of-RIB) - return as-is
	if u.IsEndOfRIB() {
		return []*Update{u}, nil
	}

	// Calculate fixed overhead: header(19) + withdrawn_len(2) + attr_len(2) = 23
	overhead := HeaderLen + 4

	// Calculate current UPDATE size
	attrSize := len(u.PathAttributes)
	currentSize := overhead + len(u.WithdrawnRoutes) + attrSize + len(u.NLRI)

	// Fast path: fits already
	if currentSize <= maxSize {
		return []*Update{u}, nil
	}

	// Check for MP_REACH_NLRI or MP_UNREACH_NLRI in PathAttributes
	mpReachInfo := findMPAttribute(u.PathAttributes, attribute.AttrMPReachNLRI)
	mpUnreachInfo := findMPAttribute(u.PathAttributes, attribute.AttrMPUnreachNLRI)

	// If we have MP attributes that are large, split those
	if mpReachInfo.found || mpUnreachInfo.found {
		return splitUpdateWithMP(u, maxSize, mpReachInfo, mpUnreachInfo, addPath)
	}

	// No MP attributes - handle IPv4 splitting
	return splitUpdateIPv4(u, maxSize, addPath)
}

// ExtractMPFamily returns the address family from MP_REACH_NLRI or MP_UNREACH_NLRI
// in raw PathAttributes bytes. Returns false if no MP attribute is found.
// RFC 4760: MP attribute value starts with AFI(2) + SAFI(1).
func ExtractMPFamily(pathAttrs []byte) (family.Family, bool) {
	// Try MP_REACH_NLRI first (type 14), then MP_UNREACH_NLRI (type 15).
	for _, code := range []attribute.AttributeCode{attribute.AttrMPReachNLRI, attribute.AttrMPUnreachNLRI} {
		info := findMPAttribute(pathAttrs, code)
		if info.found && len(info.value) >= 3 {
			afi := family.AFI(uint16(info.value[0])<<8 | uint16(info.value[1]))
			safi := family.SAFI(info.value[2])
			return family.Family{AFI: afi, SAFI: safi}, true
		}
	}
	return family.Family{}, false
}

// mpAttrInfo holds information about an MP attribute in PathAttributes.
type mpAttrInfo struct {
	found bool   // Whether the attribute was found
	start int    // Offset where attribute starts (including TLV header)
	end   int    // Offset where attribute ends
	value []byte // Attribute value (excluding TLV header)
}

// findMPAttribute locates an MP attribute in raw PathAttributes bytes.
// Returns info about the attribute's location for later extraction/replacement.
// Uses AttrIterator to avoid duplicating TLV walk logic.
func findMPAttribute(pathAttrs []byte, code attribute.AttributeCode) mpAttrInfo {
	iter := attribute.NewAttrIterator(pathAttrs)
	for {
		start := iter.Offset()
		typeCode, _, value, ok := iter.Next()
		if !ok {
			return mpAttrInfo{found: false}
		}
		if typeCode == code {
			return mpAttrInfo{
				found: true,
				start: start,
				end:   iter.Offset(),
				value: value,
			}
		}
	}
}

// splitUpdateWithMP handles splitting when MP_REACH_NLRI or MP_UNREACH_NLRI is present.
func splitUpdateWithMP(u *Update, maxSize int, mpReachInfo, mpUnreachInfo mpAttrInfo, addPath bool) ([]*Update, error) {
	overhead := HeaderLen + 4
	var updates []*Update

	// Build base attributes (everything except MP_REACH and MP_UNREACH)
	baseAttrs := u.PathAttributes
	// Remove both MP attributes from baseAttrs, handling index shifts
	switch {
	case mpUnreachInfo.found && mpReachInfo.found:
		if mpUnreachInfo.start < mpReachInfo.start {
			// MP_UNREACH first: remove it, then adjust MP_REACH indices
			baseAttrs = removeAttribute(baseAttrs, mpUnreachInfo.start, mpUnreachInfo.end)
			shift := mpUnreachInfo.end - mpUnreachInfo.start
			baseAttrs = removeAttribute(baseAttrs, mpReachInfo.start-shift, mpReachInfo.end-shift)
		} else {
			// MP_REACH first: remove it, then adjust MP_UNREACH indices
			baseAttrs = removeAttribute(baseAttrs, mpReachInfo.start, mpReachInfo.end)
			shift := mpReachInfo.end - mpReachInfo.start
			baseAttrs = removeAttribute(baseAttrs, mpUnreachInfo.start-shift, mpUnreachInfo.end-shift)
		}
	case mpUnreachInfo.found:
		baseAttrs = removeAttribute(baseAttrs, mpUnreachInfo.start, mpUnreachInfo.end)
	case mpReachInfo.found:
		baseAttrs = removeAttribute(baseAttrs, mpReachInfo.start, mpReachInfo.end)
	}

	// Handle MP_UNREACH_NLRI first (withdrawals before announcements)
	if mpUnreachInfo.found {
		// Parse MP_UNREACH_NLRI
		mpUnreach, err := attribute.ParseMPUnreachNLRI(mpUnreachInfo.value)
		if err != nil {
			return nil, fmt.Errorf("parsing MP_UNREACH_NLRI: %w", err)
		}

		// Calculate max attribute size for MP_UNREACH_NLRI
		// Attribute header is 3-4 bytes (flags, code, length)
		attrHeaderSize := 4 // Extended length for safety
		maxMPAttrValue := maxSize - overhead - len(baseAttrs) - attrHeaderSize

		if maxMPAttrValue <= 0 {
			return nil, ErrAttributesTooLarge
		}

		// Split MP_UNREACH_NLRI with Add-Path awareness
		mpChunks, err := SplitMPUnreachNLRIWithAddPath(mpUnreach, maxMPAttrValue, addPath)
		if err != nil {
			return nil, fmt.Errorf("splitting MP_UNREACH_NLRI: %w", err)
		}

		// Create UPDATE for each chunk
		for _, chunk := range mpChunks {
			chunkAttrs := append([]byte(nil), baseAttrs...)
			// Write attribute with header — use pre-computed length to avoid
			// double Len() traversal (WriteAttrTo would call Len() again).
			attrLen := chunk.Len()
			hdrLen := 3
			if attrLen > 255 {
				hdrLen = 4
			}
			attrBuf := make([]byte, hdrLen+attrLen)
			attribute.WriteAttrToWithLen(chunk, attrBuf, 0, attrLen)
			chunkAttrs = append(chunkAttrs, attrBuf...)
			updates = append(updates, &Update{
				PathAttributes: chunkAttrs,
			})
		}
	}

	// Handle MP_REACH_NLRI (announcements)
	if mpReachInfo.found {
		// Parse MP_REACH_NLRI
		mpReach, err := attribute.ParseMPReachNLRI(mpReachInfo.value)
		if err != nil {
			return nil, fmt.Errorf("parsing MP_REACH_NLRI: %w", err)
		}

		// Calculate max attribute size for MP_REACH_NLRI
		// baseAttrs already has both MP attributes removed
		attrHeaderSize := 4 // Extended length
		maxMPAttrValue := maxSize - overhead - len(baseAttrs) - attrHeaderSize

		if maxMPAttrValue <= 0 {
			return nil, ErrAttributesTooLarge
		}

		// Split MP_REACH_NLRI with Add-Path awareness
		mpChunks, err := SplitMPReachNLRIWithAddPath(mpReach, maxMPAttrValue, addPath)
		if err != nil {
			return nil, fmt.Errorf("splitting MP_REACH_NLRI: %w", err)
		}

		// Create UPDATE for each chunk
		for _, chunk := range mpChunks {
			chunkAttrs := append([]byte(nil), baseAttrs...)
			// Write attribute with header — use pre-computed length to avoid
			// double Len() traversal (WriteAttrTo would call Len() again).
			attrLen := chunk.Len()
			hdrLen := 3
			if attrLen > 255 {
				hdrLen = 4
			}
			attrBuf := make([]byte, hdrLen+attrLen)
			attribute.WriteAttrToWithLen(chunk, attrBuf, 0, attrLen)
			chunkAttrs = append(chunkAttrs, attrBuf...)
			updates = append(updates, &Update{
				PathAttributes: chunkAttrs,
			})
		}
	}

	// If no updates created (shouldn't happen if mpInfo.found is true), fall back
	if len(updates) == 0 {
		return []*Update{u}, nil
	}

	return updates, nil
}

// removeAttribute returns PathAttributes with the attribute at [start:end] removed.
func removeAttribute(pathAttrs []byte, start, end int) []byte {
	if start >= end || start < 0 || end > len(pathAttrs) {
		return pathAttrs
	}
	result := make([]byte, 0, len(pathAttrs)-(end-start))
	result = append(result, pathAttrs[:start]...)
	result = append(result, pathAttrs[end:]...)
	return result
}

// splitUpdateIPv4 handles IPv4-only UPDATE splitting.
// addPath indicates whether Add-Path is negotiated for IPv4 unicast.
func splitUpdateIPv4(u *Update, maxSize int, addPath bool) ([]*Update, error) {
	overhead := HeaderLen + 4
	attrSize := len(u.PathAttributes)

	// Check if attributes alone exceed limit (cannot split)
	if overhead+attrSize > maxSize {
		return nil, ErrAttributesTooLarge
	}

	// Handle withdrawal-only, announcement-only, or mixed
	hasWithdrawn := len(u.WithdrawnRoutes) > 0
	hasNLRI := len(u.NLRI) > 0

	var updates []*Update

	// Split withdrawals (no attributes needed for withdrawals)
	if hasWithdrawn {
		withdrawnSpace := maxSize - overhead // No attrs for withdrawals
		withdrawnChunks, err := chunkIPv4NLRI(u.WithdrawnRoutes, withdrawnSpace, addPath)
		if err != nil {
			return nil, fmt.Errorf("chunking withdrawn routes: %w", err)
		}

		for _, chunk := range withdrawnChunks {
			updates = append(updates, &Update{
				WithdrawnRoutes: chunk,
			})
		}
	}

	// Split announcements (need attributes)
	if hasNLRI {
		nlriSpace := maxSize - overhead - attrSize

		// Check if single NLRI is too large
		if nlriSpace <= 0 {
			return nil, ErrAttributesTooLarge
		}

		nlriChunks, err := chunkIPv4NLRI(u.NLRI, nlriSpace, addPath)
		if err != nil {
			return nil, fmt.Errorf("chunking NLRI: %w", err)
		}

		for _, chunk := range nlriChunks {
			updates = append(updates, &Update{
				PathAttributes: u.PathAttributes, // Zero-copy: reuse same slice
				NLRI:           chunk,
			})
		}
	}

	return updates, nil
}

// chunkIPv4NLRI splits IPv4 NLRI respecting maxSize and Add-Path state.
//
// When addPath is false, validates and chunks using basic prefix format.
// When addPath is true, uses ChunkMPNLRI with Add-Path awareness.
//
// Returns ErrNLRITooLarge if a single NLRI exceeds maxSize.
// Returns ErrNLRIMalformed if NLRI structure is invalid.
//
// AFI=1 (IPv4), SAFI=1 (Unicast) is used for ChunkMPNLRI.
func chunkIPv4NLRI(nlriData []byte, maxSize int, addPath bool) ([][]byte, error) {
	if len(nlriData) == 0 {
		return nil, nil
	}

	// Use family-aware chunker for both cases (it handles Add-Path correctly)
	// AFI=IPv4, SAFI=Unicast
	return ChunkMPNLRI(nlriData, family.AFIIPv4, family.SAFIUnicast, addPath, maxSize, nil)
}

// =============================================================================
// Phase 2: MP_REACH_NLRI and MP_UNREACH_NLRI Splitting
// =============================================================================

// SplitMPReachNLRI splits an MP_REACH_NLRI attribute into chunks.
//
// maxAttrSize is the maximum size for the attribute VALUE (not including TLV header).
// Returns multiple MPReachNLRI with chunked NLRI, preserving AFI/SAFI/NextHops.
//
// Uses ChunkMPNLRI for family-aware NLRI parsing (handles Add-Path, VPN, EVPN, etc.).
//
// RFC 4760 Section 3: MP_REACH_NLRI wire format:
//
//	AFI(2) + SAFI(1) + NH_Len(1) + NextHops + Reserved(1) + NLRI
//
// The overhead (everything except NLRI) is preserved in each chunk.
func SplitMPReachNLRI(mp *attribute.MPReachNLRI, maxAttrSize int) ([]*attribute.MPReachNLRI, error) {
	return SplitMPReachNLRIWithAddPath(mp, maxAttrSize, false)
}

// SplitMPReachNLRIWithAddPath splits MP_REACH_NLRI with Add-Path awareness.
//
// addPath indicates whether Add-Path is negotiated for this family.
// Required for correct NLRI boundary detection when path-ids are present.
func SplitMPReachNLRIWithAddPath(mp *attribute.MPReachNLRI, maxAttrSize int, addPath bool) ([]*attribute.MPReachNLRI, error) {
	if mp == nil || len(mp.NLRI) == 0 {
		return []*attribute.MPReachNLRI{mp}, nil
	}

	// Calculate overhead: AFI(2) + SAFI(1) + NH_Len(1) + NextHops + Reserved(1)
	nhLen := 0
	for _, nh := range mp.NextHops {
		if nh.Is4() {
			nhLen += 4
		} else {
			nhLen += 16
		}
	}
	overhead := 2 + 1 + 1 + nhLen + 1

	if overhead >= maxAttrSize {
		return nil, fmt.Errorf("%w: overhead %d, max %d", ErrMPOverheadTooLarge, overhead, maxAttrSize)
	}

	nlriSpace := maxAttrSize - overhead

	// Use family-aware chunking
	nlriChunks, err := ChunkMPNLRI(mp.NLRI, family.AFI(mp.AFI), family.SAFI(mp.SAFI), addPath, nlriSpace, nil)
	if err != nil {
		return nil, fmt.Errorf("chunking MP_REACH_NLRI: %w", err)
	}

	// Single chunk or empty? Return as-is
	if len(nlriChunks) <= 1 {
		return []*attribute.MPReachNLRI{mp}, nil
	}

	results := make([]*attribute.MPReachNLRI, 0, len(nlriChunks))
	for _, chunk := range nlriChunks {
		results = append(results, &attribute.MPReachNLRI{
			AFI:      mp.AFI,
			SAFI:     mp.SAFI,
			NextHops: mp.NextHops, // Shared reference (immutable)
			NLRI:     chunk,
		})
	}

	return results, nil
}

// SplitMPUnreachNLRI splits an MP_UNREACH_NLRI attribute into chunks.
//
// maxAttrSize is the maximum size for the attribute VALUE (not including TLV header).
// Returns multiple MPUnreachNLRI with chunked NLRI, preserving AFI/SAFI.
//
// Uses ChunkMPNLRI for family-aware NLRI parsing (handles Add-Path, VPN, EVPN, etc.).
//
// RFC 4760 Section 4: MP_UNREACH_NLRI wire format:
//
//	AFI(2) + SAFI(1) + Withdrawn_NLRI
//
// The overhead (AFI + SAFI) is preserved in each chunk.
func SplitMPUnreachNLRI(mp *attribute.MPUnreachNLRI, maxAttrSize int) ([]*attribute.MPUnreachNLRI, error) {
	return SplitMPUnreachNLRIWithAddPath(mp, maxAttrSize, false)
}

// SplitMPUnreachNLRIWithAddPath splits MP_UNREACH_NLRI with Add-Path awareness.
//
// addPath indicates whether Add-Path is negotiated for this family.
// Required for correct NLRI boundary detection when path-ids are present.
func SplitMPUnreachNLRIWithAddPath(mp *attribute.MPUnreachNLRI, maxAttrSize int, addPath bool) ([]*attribute.MPUnreachNLRI, error) {
	if mp == nil || len(mp.NLRI) == 0 {
		return []*attribute.MPUnreachNLRI{mp}, nil
	}

	// Overhead: AFI(2) + SAFI(1) = 3 bytes
	overhead := 3

	if overhead >= maxAttrSize {
		return nil, fmt.Errorf("%w: overhead %d, max %d", ErrMPOverheadTooLarge, overhead, maxAttrSize)
	}

	nlriSpace := maxAttrSize - overhead

	// Use family-aware chunking
	nlriChunks, err := ChunkMPNLRI(mp.NLRI, family.AFI(mp.AFI), family.SAFI(mp.SAFI), addPath, nlriSpace, nil)
	if err != nil {
		return nil, fmt.Errorf("chunking MP_UNREACH_NLRI: %w", err)
	}

	// Single chunk or empty? Return as-is
	if len(nlriChunks) <= 1 {
		return []*attribute.MPUnreachNLRI{mp}, nil
	}

	results := make([]*attribute.MPUnreachNLRI, 0, len(nlriChunks))
	for _, chunk := range nlriChunks {
		results = append(results, &attribute.MPUnreachNLRI{
			AFI:  mp.AFI,
			SAFI: mp.SAFI,
			NLRI: chunk,
		})
	}

	return results, nil
}
