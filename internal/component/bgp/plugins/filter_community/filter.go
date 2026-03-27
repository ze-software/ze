// Design: docs/architecture/core-design.md — community filter ingress path
// Overview: filter_community.go — plugin entry point
// Related: egress.go — egress filter (ModAccumulator ops)
// Related: config.go — config parsing
// Related: handler.go — AttrModHandlers for progressive build

package filter_community

import (
	"bytes"
	"encoding/binary"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
)

// applyIngressFilter applies community strip then tag for a peer's ingress config.
// Returns modified payload, or nil if no changes needed.
func applyIngressFilter(payload []byte, defs communityDefs, fc filterConfig) []byte {
	if len(payload) < 4 {
		return nil
	}

	current := payload

	// Strip first (free space, less data to iterate).
	current = applyIngressOps(current, defs, fc.ingressStrip, false)

	// Then tag.
	current = applyIngressOps(current, defs, fc.ingressTag, true)

	if bytes.Equal(current, payload) {
		return nil
	}
	return current
}

// applyIngressOps applies either tag or strip operations for the given community names.
func applyIngressOps(payload []byte, defs communityDefs, names []string, isTag bool) []byte {
	for _, name := range names {
		def, ok := defs[name]
		if !ok {
			continue
		}
		code := communityAttrCode(def.typ)
		valueSize := communityValueSize(def.typ)
		var modified []byte
		if isTag {
			modified = ingressTagCommunities(payload, code, def.wireValues)
		} else {
			modified = ingressStripCommunities(payload, code, valueSize, def.wireValues)
		}
		if modified != nil {
			payload = modified
		}
	}
	return payload
}

// communityAttrCode returns the BGP attribute code for a community type.
func communityAttrCode(typ int) attribute.AttributeCode {
	switch typ {
	case communityTypeStandard:
		return attribute.AttrCommunity
	case communityTypeLarge:
		return attribute.AttrLargeCommunity
	case communityTypeExtended:
		return attribute.AttrExtCommunity
	}
	return attribute.AttrCommunity // unreachable: typ is always one of the three constants
}

// communityValueSize returns the wire size of one community value for the type.
func communityValueSize(typ int) int {
	switch typ {
	case communityTypeStandard:
		return 4
	case communityTypeLarge:
		return 12
	case communityTypeExtended:
		return 8
	}
	return 4 // unreachable: typ is always one of the three constants
}

// ingressTagCommunities appends wire values to the target attribute in payload.
// Creates the attribute if absent. Always uses extended-length format.
// Returns modified payload or nil if unchanged.
func ingressTagCommunities(payload []byte, code attribute.AttributeCode, wireValues [][]byte) []byte {
	if len(wireValues) == 0 {
		return nil
	}

	// Calculate total bytes to add.
	var addLen int
	for _, v := range wireValues {
		addLen += len(v)
	}

	attrStart, attrEnd, dataStart, dataEnd, found := findAttribute(payload, code)

	if !found {
		// Validate payload structure before using helpers.
		pae := safePathAttrEnd(payload)
		if pae < 0 {
			return nil
		}
		pal := safePathAttrLen(payload)
		if pal < 0 {
			return nil
		}

		// Create new attribute: flags(1) + code(1) + extlen(2) + data.
		newAttrLen := 4 + addLen

		// Cap at uint16 max for both attribute data length and path attr total length.
		if addLen > 65535 || pal+newAttrLen > 65535 {
			return nil // Would exceed BGP limits; skip modification.
		}

		result := make([]byte, len(payload)+newAttrLen)

		// Copy everything up to end of path attributes.
		copy(result, payload[:pae])

		// Write new attribute at the end of path attributes section.
		off := pae
		result[off] = 0xC0 | 0x10 // Optional Transitive + Extended Length
		result[off+1] = byte(code)
		binary.BigEndian.PutUint16(result[off+2:], uint16(addLen)) //nolint:gosec // capped above
		off += 4
		for _, v := range wireValues {
			copy(result[off:], v)
			off += len(v)
		}

		// Copy any remaining bytes (NLRI after path attrs).
		copy(result[off:], payload[pae:])

		// Update path attributes length field.
		safeUpdatePathAttrLen(result, pal+newAttrLen)

		return result
	}

	// Extend existing attribute. Always promote to extended-length format.
	oldDataLen := dataEnd - dataStart
	newDataLen := oldDataLen + addLen

	// Determine if we need to promote from 1-byte to 2-byte length.
	oldIsExtended := payload[attrStart]&0x10 != 0
	extraBytes := 0
	if !oldIsExtended {
		extraBytes = 1 // Need 1 extra byte for extended-length header.
	}

	// Cap at uint16 max for both attribute data length and total path attr length.
	pal := safePathAttrLen(payload)
	if newDataLen > 65535 || (pal >= 0 && pal+addLen+extraBytes > 65535) {
		return nil // Would exceed BGP length limits.
	}

	result := make([]byte, len(payload)+addLen+extraBytes)

	if oldIsExtended {
		// Copy header + existing data verbatim.
		copy(result, payload[:dataEnd])
	} else {
		// Promote: copy up to attrStart, rewrite header with extended length.
		copy(result, payload[:attrStart])
		off := attrStart
		result[off] = payload[attrStart] | 0x10 // Set extended-length flag
		result[off+1] = payload[attrStart+1]    // Code
		// Leave 2 bytes for length (filled below).
		off += 4
		// Copy existing data.
		copy(result[off:], payload[dataStart:dataEnd])
	}

	// Append new values after existing data.
	writeOff := attrStart + 4 + oldDataLen // header(4) + existing data
	for _, v := range wireValues {
		copy(result[writeOff:], v)
		writeOff += len(v)
	}

	// Copy everything after the old attribute.
	copy(result[writeOff:], payload[attrEnd:])

	// Update attribute data length (always extended now).
	binary.BigEndian.PutUint16(result[attrStart+2:], uint16(newDataLen)) //nolint:gosec // capped above

	// Update path attributes total length (pal validated above).
	safeUpdatePathAttrLen(result, pal+addLen+extraBytes)

	return result
}

// ingressStripCommunities removes matching wire values from the target attribute.
// Returns modified payload or nil if no changes needed.
func ingressStripCommunities(payload []byte, code attribute.AttributeCode, valueSize int, wireValues [][]byte) []byte {
	attrStart, attrEnd, dataStart, dataEnd, found := findAttribute(payload, code)
	if !found {
		return nil
	}

	// Build set of values to strip for O(1) lookup.
	stripSet := make(map[string]bool, len(wireValues))
	for _, v := range wireValues {
		stripSet[string(v)] = true
	}

	// Scan existing values, keep those not in strip set.
	// Trailing partial-value bytes (malformed attribute) are silently dropped.
	oldData := payload[dataStart:dataEnd]
	var kept [][]byte
	stripped := false
	for i := 0; i+valueSize <= len(oldData); i += valueSize {
		val := oldData[i : i+valueSize]
		if stripSet[string(val)] {
			stripped = true
		} else {
			valCopy := make([]byte, valueSize)
			copy(valCopy, val)
			kept = append(kept, valCopy)
		}
	}

	if !stripped {
		return nil
	}

	pal := safePathAttrLen(payload)
	if pal < 0 {
		return nil
	}

	if len(kept) == 0 {
		// Remove entire attribute.
		removedLen := attrEnd - attrStart
		result := make([]byte, len(payload)-removedLen)
		copy(result, payload[:attrStart])
		copy(result[attrStart:], payload[attrEnd:])
		safeUpdatePathAttrLen(result, pal-removedLen)
		return result
	}

	// Rebuild with kept values only.
	newDataLen := len(kept) * valueSize
	oldDataLen := dataEnd - dataStart
	diff := newDataLen - oldDataLen

	result := make([]byte, len(payload)+diff)
	copy(result, payload[:dataStart])

	off := dataStart
	for _, v := range kept {
		copy(result[off:], v)
		off += len(v)
	}
	copy(result[off:], payload[attrEnd:])

	// Update attribute data length (preserves original length format -- stripping
	// can only reduce size, so no promotion from 1-byte to 2-byte length is needed).
	safeUpdateAttrDataLen(result, attrStart, newDataLen)
	safeUpdatePathAttrLen(result, pal+diff)

	return result
}

// findAttribute locates an attribute by code in the path attributes section.
// Returns (attrStart, attrEnd, dataStart, dataEnd, found).
func findAttribute(payload []byte, code attribute.AttributeCode) (int, int, int, int, bool) {
	if len(payload) < 4 {
		return 0, 0, 0, 0, false
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenPos := 2 + withdrawnLen
	if attrLenPos+2 > len(payload) {
		return 0, 0, 0, 0, false
	}
	attrTotalLen := int(binary.BigEndian.Uint16(payload[attrLenPos : attrLenPos+2]))
	pos := attrLenPos + 2
	end := pos + attrTotalLen
	if end > len(payload) {
		return 0, 0, 0, 0, false
	}

	for pos < end {
		attrStart := pos
		if pos+2 > end {
			break
		}
		flags := payload[pos]
		attrCode := payload[pos+1]
		pos += 2
		var dataLen int
		if flags&0x10 != 0 { // Extended length
			if pos+2 > end {
				break
			}
			dataLen = int(binary.BigEndian.Uint16(payload[pos : pos+2]))
			pos += 2
		} else {
			if pos >= end {
				break
			}
			dataLen = int(payload[pos])
			pos++
		}
		dataStart := pos
		if pos+dataLen > end {
			break
		}
		pos += dataLen

		if attribute.AttributeCode(attrCode) == code {
			return attrStart, pos, dataStart, dataStart + dataLen, true
		}
	}
	return 0, 0, 0, 0, false
}

// safePathAttrLen returns the total path attributes length, or -1 if payload is malformed.
func safePathAttrLen(payload []byte) int {
	if len(payload) < 4 {
		return -1
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	if 4+withdrawnLen > len(payload) {
		return -1
	}
	return int(binary.BigEndian.Uint16(payload[2+withdrawnLen : 4+withdrawnLen]))
}

// safePathAttrEnd returns the offset just past the path attributes section, or -1 if malformed.
func safePathAttrEnd(payload []byte) int {
	pal := safePathAttrLen(payload)
	if pal < 0 {
		return -1
	}
	if len(payload) < 4 {
		return -1
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	end := 4 + withdrawnLen + pal
	if end > len(payload) {
		return -1
	}
	return end
}

// safeUpdatePathAttrLen writes the new path attributes length into the payload.
// No-op if payload is too short or newLen exceeds uint16 max.
func safeUpdatePathAttrLen(payload []byte, newLen int) {
	if newLen < 0 || newLen > 65535 || len(payload) < 4 {
		return
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	if 4+withdrawnLen > len(payload) {
		return
	}
	binary.BigEndian.PutUint16(payload[2+withdrawnLen:], uint16(newLen)) //nolint:gosec // capped above
}

// safeUpdateAttrDataLen updates the data length field of an attribute at attrStart.
// Handles both regular (1-byte) and extended (2-byte) length formats.
func safeUpdateAttrDataLen(payload []byte, attrStart, newDataLen int) {
	if attrStart+3 > len(payload) || newDataLen < 0 {
		return
	}
	flags := payload[attrStart]
	if flags&0x10 != 0 { // Extended length
		if attrStart+4 > len(payload) || newDataLen > 65535 {
			return
		}
		binary.BigEndian.PutUint16(payload[attrStart+2:], uint16(newDataLen)) //nolint:gosec // capped
	} else {
		if newDataLen > 255 {
			return // Would need promotion; caller should use extended length.
		}
		payload[attrStart+2] = byte(newDataLen) //nolint:gosec // capped above
	}
}
