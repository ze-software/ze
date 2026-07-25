// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc7606.md — attribute discard error handling
// RFC: rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt — in-place attribute discard marker
// Related: rfc7606.go — RFC 7606 revised error handling actions
// Related: ../wireu/tombstone.go — the same marker written on the egress wire path

package message

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// ATTR_TOMBSTONE path attribute implementation.
// draft-mangin-idr-attr-tombstone-00: In-place marker for RFC 7606 attribute discard.
//
// When a BGP speaker applies "attribute discard" per RFC 7606, it overwrites
// the malformed attribute's header and first two value bytes with an
// ATTR_TOMBSTONE marker, preserving the wire layout for zero-copy forwarding.
//
// The marker's type code is the single canonical constant attribute.AttrTombstone
// (252), declared once at the core attribute tier and shared with the wireu egress
// path (wireu.WriteTombstone), so a marker ze writes on egress is the same code its
// own upstream merge searches for. draft-mangin-idr-attr-tombstone-00 Section 8: the
// value is provisional (TBD, IANA allocation pending).

// Discard reason codes per draft-mangin-idr-attr-tombstone-00 Section 4.4.
const (
	DiscardReasonUnspecified    uint8 = 0 // Reason not recorded or not applicable.
	DiscardReasonEBGPInvalid    uint8 = 1 // Attribute invalid in EBGP context (RFC 7606 §7.5, §7.9, §7.10).
	DiscardReasonInvalidLength  uint8 = 2 // Attribute length does not match expected (RFC 7606 §7.6, §7.7).
	DiscardReasonMalformedValue uint8 = 3 // Attribute value syntactically invalid despite correct length.
	DiscardReasonLocalPolicy    uint8 = 4 // Attribute deliberately removed by local policy.
)

// DiscardEntry represents a single attribute discard with reason code.
// draft-mangin-idr-attr-tombstone-00 Section 4.1, 4.4.
type DiscardEntry struct {
	Code   uint8 // Original attribute type code.
	Reason uint8 // Reason code (DiscardReason* constants).
}

// attrDiscardFlags computes the flags byte for an ATTR_TOMBSTONE marker.
// draft-mangin-idr-attr-tombstone-00 Section 4.2:
//
//	new_flags = 0x80 | (original_flags & 0x50)
//
// Sets Optional bit, preserves Transitive and Extended Length bits, clears Partial.
//
// This is the generation-time derivation only. The marker is stamped at receive
// time, where the destination is not yet known, so a transitive original yields a
// transitive marker here. Section 5.3's egress rule (clear the Transitive bit when
// forwarding to an EBGP peer) is enforced per destination on the EBGP wire path,
// in wireu.rewriteASPathPrepend, not here.
func attrDiscardFlags(originalFlags uint8) uint8 {
	return 0x80 | (originalFlags & 0x50)
}

// ApplyAttrDiscard applies ATTR_TOMBSTONE markers to a path attributes section.
//
// draft-mangin-idr-attr-tombstone-00 Section 5.1:
//   - Single discard with value >= 2: in-place overwrite (modifies pathAttrs, returns false)
//   - Multiple discards or value < 2: rebuild (returns new buffer, true)
//   - Upstream ATTR_TOMBSTONE present: merged per RFC 4271 Section 5
//
// Returns (resultAttrs, rebuilt). If rebuilt is false, pathAttrs was modified in-place.
func ApplyAttrDiscard(pathAttrs []byte, entries []DiscardEntry) ([]byte, bool) {
	if len(entries) == 0 {
		return pathAttrs, false
	}

	// Check for upstream ATTR_TOMBSTONE that needs merging.
	upstreamEntries := ExtractUpstreamAttrDiscard(pathAttrs)
	needsMerge := len(upstreamEntries) > 0

	// Single entry, no upstream merge needed, try in-place overwrite.
	if len(entries) == 1 && !needsMerge {
		if applyInPlace(pathAttrs, entries[0]) {
			return pathAttrs, false
		}
	}

	// Rebuild: multiple entries, value too short, or upstream merge needed.
	// Merge upstream + local entries into a single list for the rebuilt ATTR_TOMBSTONE.
	merged := make([]DiscardEntry, 0, len(upstreamEntries)+len(entries))
	merged = append(merged, upstreamEntries...)
	merged = append(merged, entries...)
	return rebuildWithAttrDiscard(pathAttrs, entries, merged), true
}

// applyInPlace overwrites a single malformed attribute with ATTR_TOMBSTONE in-place.
// Returns true if successful, false if the attribute is not found or value length < 2.
//
// Zero allocation — uses AttrFind (standalone function, no pointer receiver escape).
//
// draft-mangin-idr-attr-tombstone-00 Section 5.1, steps 1-8:
//  1. Locate the attribute by code
//  2. Overwrite flags: new_flags = 0x80 | (original_flags & 0x50)
//  3. Save original type code
//  4. Overwrite type code with attribute.AttrTombstone
//  5. Write original code as first value byte
//  6. Write reason code as second value byte
//  7. Zero remaining value bytes
//  8. Length field unchanged
func applyInPlace(pathAttrs []byte, entry DiscardEntry) bool {
	hdrStart, flags, value, found := attribute.AttrFind(pathAttrs, attribute.AttributeCode(entry.Code))
	if !found || len(value) < 2 {
		return false
	}

	// Overwrite flags.
	pathAttrs[hdrStart] = attrDiscardFlags(uint8(flags))
	// Overwrite type code with the single canonical ATTR_TOMBSTONE code point.
	pathAttrs[hdrStart+1] = byte(attribute.AttrTombstone)
	// Write original code and reason into first two value bytes.
	// value is a subslice of pathAttrs — writes go through to the original buffer.
	value[0] = entry.Code
	value[1] = entry.Reason
	// Zero remaining value bytes.
	for i := 2; i < len(value); i++ {
		value[i] = 0
	}
	return true
}

// ExtractUpstreamAttrDiscard finds an existing ATTR_TOMBSTONE and extracts its (code, reason) pairs.
// Returns nil if no upstream ATTR_TOMBSTONE is present.
// Uses AttrFind (zero allocation when no upstream present — the happy path).
func ExtractUpstreamAttrDiscard(pathAttrs []byte) []DiscardEntry {
	_, _, value, found := attribute.AttrFind(pathAttrs, attribute.AttrTombstone)
	if !found {
		return nil
	}
	// Parse (code, reason) pairs from value.
	var entries []DiscardEntry
	for i := 0; i+1 < len(value); i += 2 {
		entries = append(entries, DiscardEntry{
			Code:   value[i],
			Reason: value[i+1],
		})
	}
	return entries
}

// rebuildWithAttrDiscard rebuilds the path attributes section, removing discarded
// attributes and any upstream ATTR_TOMBSTONE, then inserting a single merged marker.
//
// draft-mangin-idr-attr-tombstone-00 Section 5.1:
// "the local speaker MUST use the rebuild procedure: remove the upstream
// ATTR_TOMBSTONE and all locally- discarded attributes, then insert a single
// ATTR_TOMBSTONE whose value contains all (code, reason) pairs -- upstream pairs
// followed by local pairs."
//
// Parameters:
//   - pathAttrs: original path attributes bytes
//   - localEntries: attributes being discarded in this pass (used to identify which to remove)
//   - allEntries: merged upstream + local entries (written into the new ATTR_TOMBSTONE value)
func rebuildWithAttrDiscard(pathAttrs []byte, localEntries, allEntries []DiscardEntry) []byte {
	// Build set of codes to remove.
	var removeCodes [256]bool
	for _, e := range localEntries {
		removeCodes[e.Code] = true
	}

	// Calculate new size: copy non-removed, non-ATTR_TOMBSTONE attributes,
	// then append new ATTR_TOMBSTONE.
	// First pass: measure using AttrIterator.
	var keepSize int
	iter := attribute.NewAttrIterator(pathAttrs)
	for {
		start := iter.Offset()
		typeCode, _, _, ok := iter.Next()
		if !ok {
			break
		}
		if typeCode == attribute.AttrTombstone || removeCodes[uint8(typeCode)] {
			continue
		}
		keepSize += iter.Offset() - start
	}

	// ATTR_TOMBSTONE value: 2 bytes per entry.
	discardValueLen := len(allEntries) * 2
	discardHdrLen := 3
	if discardValueLen > 255 {
		discardHdrLen = 4
	}
	discardTotalLen := discardHdrLen + discardValueLen

	// Compute flags for the merged ATTR_TOMBSTONE.
	// draft-mangin-idr-attr-tombstone-00 Section 5.7: "if all discarded attributes
	// were transitive, the result is transitive (0xC0); if all were non-transitive,
	// non-transitive (0x80); if mixed, the result MUST be non-transitive (0x80)."
	mergedFlags := uint8(0x80) // Default: optional non-transitive.

	// Determine transitivity from upstream ATTR_TOMBSTONE (if present).
	upstreamFlags := findAttrFlags(pathAttrs, uint8(attribute.AttrTombstone))
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

	// draft-mangin-idr-attr-tombstone-00 Section 5.7: "The merged ATTR_TOMBSTONE is
	// transitive (0xC0) only if the upstream ATTR_TOMBSTONE was transitive AND all
	// locally discarded attributes were also transitive."
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

	// Second pass: copy kept attributes using AttrIterator.
	iter.Reset()
	for {
		start := iter.Offset()
		typeCode, _, _, ok := iter.Next()
		if !ok {
			break
		}
		if typeCode == attribute.AttrTombstone || removeCodes[uint8(typeCode)] {
			continue
		}
		attrLen := iter.Offset() - start
		copy(result[wpos:], pathAttrs[start:start+attrLen])
		wpos += attrLen
	}

	// Write ATTR_TOMBSTONE attribute.
	result[wpos] = mergedFlags
	result[wpos+1] = byte(attribute.AttrTombstone)
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
// Returns 0 if the attribute is not found (e.g., upstream ATTR_TOMBSTONE entry
// whose original attribute is no longer in the path attributes section).
// Uses AttrFind (zero allocation).
func findAttrFlags(pathAttrs []byte, code uint8) uint8 {
	_, flags, _, found := attribute.AttrFind(pathAttrs, attribute.AttributeCode(code))
	if !found {
		return 0
	}
	return uint8(flags)
}

// StripAttrRanges returns a copy of pathAttrs with the given byte ranges removed.
//
// RFC 7606 Section 3.g: "Discard all but first occurrence" of a duplicate non-MP
// attribute. The ranges are the later occurrences recorded by the validator
// (RFC7606ValidationResult.DuplicateRanges); they are in ascending order and
// non-overlapping (a single forward parse), each covering one whole attribute.
// Removing them enforces keep-first at the byte level so downstream consumers
// (RIB, filters, cross-context re-encode, the attribute index) see one copy.
//
// Returns pathAttrs unchanged (no copy) when ranges is empty. Uses make+copy
// rather than append, matching rebuildWithAttrDiscard: this runs only on the
// malformed-UPDATE path, and the result is a caller-owned copy.
func StripAttrRanges(pathAttrs []byte, ranges []AttrRange) []byte {
	if len(ranges) == 0 {
		return pathAttrs
	}
	removed := 0
	for _, r := range ranges {
		removed += r.End - r.Start
	}
	out := make([]byte, len(pathAttrs)-removed)
	wpos := 0
	prev := 0
	for _, r := range ranges {
		wpos += copy(out[wpos:], pathAttrs[prev:r.Start])
		prev = r.End
	}
	copy(out[wpos:], pathAttrs[prev:])
	return out
}

// RebuildUpdateBody reconstructs an UPDATE message body with new path attributes.
// Used when ATTR_TOMBSTONE rebuild changes the path attributes section size.
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
