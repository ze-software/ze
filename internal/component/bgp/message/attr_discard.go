// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc7606.md — attribute discard error handling

package message

import "encoding/binary"

// ATTR_DISCARD path attribute implementation.
// draft-mangin-idr-attr-discard-00: In-place marker for RFC 7606 attribute discard.
//
// When a BGP speaker applies "attribute discard" per RFC 7606, it overwrites
// the malformed attribute's header and first two value bytes with an
// ATTR_DISCARD marker, preserving the wire layout for zero-copy forwarding.

// attrCodeAttrDiscard is the ATTR_DISCARD type code.
// draft-mangin-idr-attr-discard-00: TBD (IANA allocation pending).
const attrCodeAttrDiscard uint8 = 253

// Discard reason codes per draft-mangin-idr-attr-discard-00 Section 4.4.
const (
	DiscardReasonUnspecified    uint8 = 0 // Reason not recorded or not applicable.
	DiscardReasonEBGPInvalid    uint8 = 1 // Attribute invalid in EBGP context (RFC 7606 §7.5, §7.9, §7.10).
	DiscardReasonInvalidLength  uint8 = 2 // Attribute length does not match expected (RFC 7606 §7.6, §7.7).
	DiscardReasonMalformedValue uint8 = 3 // Attribute value syntactically invalid despite correct length.
	DiscardReasonLocalPolicy    uint8 = 4 // Attribute deliberately removed by local policy.
)

// DiscardEntry represents a single attribute discard with reason code.
// draft-mangin-idr-attr-discard-00 Section 4.1, 4.4.
type DiscardEntry struct {
	Code   uint8 // Original attribute type code.
	Reason uint8 // Reason code (DiscardReason* constants).
}

// attrDiscardFlags computes the flags byte for an ATTR_DISCARD marker.
// draft-mangin-idr-attr-discard-00 Section 4.2:
//
//	new_flags = 0x80 | (original_flags & 0x50)
//
// Sets Optional bit, preserves Transitive and Extended Length bits, clears Partial.
func attrDiscardFlags(originalFlags uint8) uint8 {
	return 0x80 | (originalFlags & 0x50)
}

// ApplyAttrDiscard applies ATTR_DISCARD markers to a path attributes section.
//
// draft-mangin-idr-attr-discard-00 Section 5.1:
//   - Single discard with value >= 2: in-place overwrite (modifies pathAttrs, returns false)
//   - Multiple discards or value < 2: rebuild (returns new buffer, true)
//   - Upstream ATTR_DISCARD present: merged per RFC 4271 Section 5
//
// Returns (resultAttrs, rebuilt). If rebuilt is false, pathAttrs was modified in-place.
func ApplyAttrDiscard(pathAttrs []byte, entries []DiscardEntry) ([]byte, bool) {
	if len(entries) == 0 {
		return pathAttrs, false
	}

	// Check for upstream ATTR_DISCARD that needs merging.
	upstreamEntries := ExtractUpstreamAttrDiscard(pathAttrs)
	needsMerge := len(upstreamEntries) > 0

	// Single entry, no upstream merge needed, try in-place overwrite.
	if len(entries) == 1 && !needsMerge {
		if applyInPlace(pathAttrs, entries[0]) {
			return pathAttrs, false
		}
	}

	// Rebuild: multiple entries, value too short, or upstream merge needed.
	// Merge upstream + local entries into a single list for the rebuilt ATTR_DISCARD.
	merged := make([]DiscardEntry, 0, len(upstreamEntries)+len(entries))
	merged = append(merged, upstreamEntries...)
	merged = append(merged, entries...)
	return rebuildWithAttrDiscard(pathAttrs, entries, merged), true
}

// applyInPlace overwrites a single malformed attribute with ATTR_DISCARD in-place.
// Returns true if successful, false if the attribute's value length < 2.
//
// draft-mangin-idr-attr-discard-00 Section 5.1, steps 1-8:
//  1. Locate the attribute by code
//  2. Overwrite flags: new_flags = 0x80 | (original_flags & 0x50)
//  3. Save original type code
//  4. Overwrite type code with attrCodeAttrDiscard
//  5. Write original code as first value byte
//  6. Write reason code as second value byte
//  7. Zero remaining value bytes
//  8. Length field unchanged
func applyInPlace(pathAttrs []byte, entry DiscardEntry) bool {
	pos := 0
	for pos < len(pathAttrs) {
		if pos+2 > len(pathAttrs) {
			return false
		}

		flags := pathAttrs[pos]
		code := pathAttrs[pos+1]
		hdrStart := pos
		pos += 2

		// Determine attribute value length and offset.
		var valueLen int
		if flags&0x10 != 0 { // Extended length
			if pos+2 > len(pathAttrs) {
				return false
			}
			valueLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			pos += 2
		} else {
			if pos >= len(pathAttrs) {
				return false
			}
			valueLen = int(pathAttrs[pos])
			pos++
		}

		valueStart := pos
		if valueStart+valueLen > len(pathAttrs) {
			return false
		}

		if code == entry.Code {
			// Found the attribute to discard.
			if valueLen < 2 {
				return false // Cannot do in-place overwrite.
			}

			// Overwrite flags.
			pathAttrs[hdrStart] = attrDiscardFlags(flags)
			// Overwrite type code.
			pathAttrs[hdrStart+1] = attrCodeAttrDiscard
			// Write original code and reason into first two value bytes.
			pathAttrs[valueStart] = entry.Code
			pathAttrs[valueStart+1] = entry.Reason
			// Zero remaining value bytes.
			for i := valueStart + 2; i < valueStart+valueLen; i++ {
				pathAttrs[i] = 0
			}
			return true
		}

		pos = valueStart + valueLen
	}
	return false // Attribute not found.
}

// ExtractUpstreamAttrDiscard finds an existing ATTR_DISCARD and extracts its (code, reason) pairs.
// Returns nil if no upstream ATTR_DISCARD is present.
func ExtractUpstreamAttrDiscard(pathAttrs []byte) []DiscardEntry {
	pos := 0
	for pos < len(pathAttrs) {
		if pos+2 > len(pathAttrs) {
			return nil
		}

		flags := pathAttrs[pos]
		code := pathAttrs[pos+1]
		pos += 2

		var valueLen int
		if flags&0x10 != 0 {
			if pos+2 > len(pathAttrs) {
				return nil
			}
			valueLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			pos += 2
		} else {
			if pos >= len(pathAttrs) {
				return nil
			}
			valueLen = int(pathAttrs[pos])
			pos++
		}

		valueStart := pos
		if valueStart+valueLen > len(pathAttrs) {
			return nil
		}

		if code == attrCodeAttrDiscard {
			// Parse (code, reason) pairs from value.
			var entries []DiscardEntry
			for i := 0; i+1 < valueLen; i += 2 {
				entries = append(entries, DiscardEntry{
					Code:   pathAttrs[valueStart+i],
					Reason: pathAttrs[valueStart+i+1],
				})
			}
			return entries
		}

		pos = valueStart + valueLen
	}
	return nil
}

// rebuildWithAttrDiscard rebuilds the path attributes section, removing discarded
// attributes and any upstream ATTR_DISCARD, then inserting a single merged ATTR_DISCARD.
//
// draft-mangin-idr-attr-discard-00 Section 5.1 / Section 5.3:
// "remove the upstream ATTR_DISCARD and all locally-discarded attributes,
// then insert a single ATTR_DISCARD whose value contains all (code, reason)
// pairs -- upstream pairs followed by local pairs."
//
// Parameters:
//   - pathAttrs: original path attributes bytes
//   - localEntries: attributes being discarded in this pass (used to identify which to remove)
//   - allEntries: merged upstream + local entries (written into the new ATTR_DISCARD value)
func rebuildWithAttrDiscard(pathAttrs []byte, localEntries, allEntries []DiscardEntry) []byte {
	// Build set of codes to remove.
	removeCodes := make(map[uint8]bool)
	for _, e := range localEntries {
		removeCodes[e.Code] = true
	}

	// Calculate new size: copy non-removed, non-ATTR_DISCARD attributes,
	// then append new ATTR_DISCARD.
	// First pass: measure.
	var keepSize int
	pos := 0
	for pos < len(pathAttrs) {
		if pos+2 > len(pathAttrs) {
			break
		}
		flags := pathAttrs[pos]
		attrCode := pathAttrs[pos+1]
		pos += 2

		var valueLen int
		var hdrLen int
		if flags&0x10 != 0 {
			if pos+2 > len(pathAttrs) {
				break
			}
			valueLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			hdrLen = 4
			pos += 2
		} else {
			if pos >= len(pathAttrs) {
				break
			}
			valueLen = int(pathAttrs[pos])
			hdrLen = 3
			pos++
		}

		attrTotalLen := hdrLen + valueLen

		if attrCode == attrCodeAttrDiscard || removeCodes[attrCode] {
			// Skip this attribute.
			pos += valueLen
			continue
		}

		keepSize += attrTotalLen
		pos += valueLen
	}

	// ATTR_DISCARD value: 2 bytes per entry.
	discardValueLen := len(allEntries) * 2
	discardHdrLen := 3
	if discardValueLen > 255 {
		discardHdrLen = 4
	}
	discardTotalLen := discardHdrLen + discardValueLen

	// Compute flags for the merged ATTR_DISCARD.
	// draft-mangin-idr-attr-discard-00 Section 5.10:
	// ALL transitive → MUST 0xC0, ALL non-transitive → MUST 0x80,
	// mixed → SHOULD 0x80 (conservative default).
	mergedFlags := uint8(0x80) // Default: optional non-transitive.

	// Determine transitivity from upstream ATTR_DISCARD (if present).
	upstreamFlags := findAttrFlags(pathAttrs, attrCodeAttrDiscard)
	hasUpstream := upstreamFlags != 0
	upstreamTransitive := upstreamFlags&0x40 != 0

	// Determine transitivity from local entries' original attributes.
	allLocalTransitive := true
	hasLocal := false
	for _, e := range localEntries {
		hasLocal = true
		origFlags := findAttrFlags(pathAttrs, e.Code)
		if origFlags&0x40 == 0 { // Not transitive
			allLocalTransitive = false
			break
		}
	}
	if !hasLocal {
		allLocalTransitive = false
	}

	// Section 5.10: only set Transitive when ALL sources agree.
	if hasUpstream {
		if upstreamTransitive && allLocalTransitive {
			mergedFlags |= 0x40
		}
	} else if allLocalTransitive {
		mergedFlags |= 0x40
	}
	if discardValueLen > 255 {
		mergedFlags |= 0x10 // Extended length.
	}

	// Allocate and fill.
	result := make([]byte, keepSize+discardTotalLen)
	wpos := 0

	// Second pass: copy kept attributes.
	pos = 0
	for pos < len(pathAttrs) {
		if pos+2 > len(pathAttrs) {
			break
		}
		flags := pathAttrs[pos]
		attrCode := pathAttrs[pos+1]
		attrStart := pos
		pos += 2

		var valueLen int
		var hdrLen int
		if flags&0x10 != 0 {
			if pos+2 > len(pathAttrs) {
				break
			}
			valueLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			hdrLen = 4
			pos += 2
		} else {
			if pos >= len(pathAttrs) {
				break
			}
			valueLen = int(pathAttrs[pos])
			hdrLen = 3
			pos++
		}

		if attrCode == attrCodeAttrDiscard || removeCodes[attrCode] {
			pos += valueLen
			continue
		}

		totalLen := hdrLen + valueLen
		copy(result[wpos:], pathAttrs[attrStart:attrStart+totalLen])
		wpos += totalLen
		pos += valueLen
	}

	// Write ATTR_DISCARD attribute.
	result[wpos] = mergedFlags
	result[wpos+1] = attrCodeAttrDiscard
	if discardHdrLen == 4 {
		//nolint:gosec // discardValueLen is bounded by number of BGP attributes (max ~256 * 2 = 512)
		binary.BigEndian.PutUint16(result[wpos+2:wpos+4], uint16(discardValueLen))
		wpos += 4
	} else {
		result[wpos+2] = byte(discardValueLen)
		wpos += 3
	}

	// Write (code, reason) pairs.
	for _, e := range allEntries {
		result[wpos] = e.Code
		result[wpos+1] = e.Reason
		wpos += 2
	}

	return result
}

// findAttrFlags finds the flags byte for an attribute by its type code.
// Returns 0 if the attribute is not found (e.g., upstream ATTR_DISCARD entry
// whose original attribute is no longer in the path attributes section).
func findAttrFlags(pathAttrs []byte, code uint8) uint8 {
	pos := 0
	for pos < len(pathAttrs) {
		if pos+2 > len(pathAttrs) {
			return 0
		}
		flags := pathAttrs[pos]
		attrCode := pathAttrs[pos+1]
		pos += 2

		var valueLen int
		if flags&0x10 != 0 {
			if pos+2 > len(pathAttrs) {
				return 0
			}
			valueLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			pos += 2
		} else {
			if pos >= len(pathAttrs) {
				return 0
			}
			valueLen = int(pathAttrs[pos])
			pos++
		}

		if attrCode == code {
			return flags
		}

		pos += valueLen
	}
	return 0
}

// RebuildUpdateBody reconstructs an UPDATE message body with new path attributes.
// Used when ATTR_DISCARD rebuild changes the path attributes section size.
//
// UPDATE body layout (RFC 4271 Section 4.3):
//
//	[withdrawn-len: 2][withdrawn: N][attr-len: 2][attrs: M][nlri: R]
func RebuildUpdateBody(body, newPathAttrs []byte) []byte {
	if len(body) < 4 {
		return body
	}

	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	withdrawnEnd := 2 + withdrawnLen
	if withdrawnEnd+2 > len(body) {
		return body
	}

	oldAttrLen := int(binary.BigEndian.Uint16(body[withdrawnEnd : withdrawnEnd+2]))
	nlriStart := withdrawnEnd + 2 + oldAttrLen
	var nlri []byte
	if nlriStart < len(body) {
		nlri = body[nlriStart:]
	}

	// Build new body.
	newAttrLen := len(newPathAttrs)
	newBody := make([]byte, withdrawnEnd+2+newAttrLen+len(nlri))

	// Copy withdrawn section (including length field).
	copy(newBody[0:], body[0:withdrawnEnd])
	// Write new path attributes length.
	//nolint:gosec // newAttrLen is bounded by BGP message size (max 65535)
	binary.BigEndian.PutUint16(newBody[withdrawnEnd:withdrawnEnd+2], uint16(newAttrLen))
	// Copy new path attributes.
	copy(newBody[withdrawnEnd+2:], newPathAttrs)
	// Copy NLRI.
	if len(nlri) > 0 {
		copy(newBody[withdrawnEnd+2+newAttrLen:], nlri)
	}

	return newBody
}
