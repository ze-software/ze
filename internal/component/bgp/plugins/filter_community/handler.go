// Design: docs/architecture/core-design.md — community filter egress AttrModHandlers
// Overview: filter_community.go — plugin entry point
// Related: egress.go — egress filter accumulates ops
// Related: filter.go — ingress filter (direct payload mutation)

package filter_community

import (
	"bytes"
	"encoding/binary"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// communityAttrModHandler handles AttrModAdd/Remove for COMMUNITY (code 8, 4-byte values).
// Called by buildModifiedPayload during the progressive build for egress forwarding.
// src is the FULL attribute (flags+code+len+data), not just the value.
func communityAttrModHandler(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
	return genericCommunityHandler(attribute.AttrCommunity, 4, src, ops, buf, off)
}

// largeCommunityAttrModHandler handles AttrModAdd/Remove for LARGE_COMMUNITY (code 32, 12-byte values).
func largeCommunityAttrModHandler(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
	return genericCommunityHandler(attribute.AttrLargeCommunity, 12, src, ops, buf, off)
}

// extCommunityAttrModHandler handles AttrModAdd/Remove for EXTENDED_COMMUNITY (code 16, 8-byte values).
func extCommunityAttrModHandler(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
	return genericCommunityHandler(attribute.AttrExtCommunity, 8, src, ops, buf, off)
}

// extractAttrValue extracts the value portion from a full attribute (flags+code+len+data).
// Returns the value bytes, or nil if the attribute is malformed or too short.
func extractAttrValue(src []byte) []byte {
	if len(src) < 3 {
		return nil
	}
	flags := src[0]
	if flags&0x10 != 0 { // Extended length (2-byte)
		if len(src) < 4 {
			return nil
		}
		dataLen := int(binary.BigEndian.Uint16(src[2:4]))
		if 4+dataLen > len(src) {
			return nil
		}
		return src[4 : 4+dataLen]
	}
	// Non-extended length (1-byte)
	dataLen := int(src[2])
	if 3+dataLen > len(src) {
		return nil
	}
	return src[3 : 3+dataLen]
}

// genericCommunityHandler implements Add/Remove/Set for any community attribute type.
// src is the FULL attribute (flags+code+len+data) from buildModifiedPayload, or nil
// if the attribute is absent in the source UPDATE.
// Writes complete attribute (flags + code + extlen + value) into buf at off.
// Returns new offset after written bytes, or off unchanged if attribute omitted.
// Buf MUST NOT be retained beyond the call.
func genericCommunityHandler(code attribute.AttributeCode, valueSize int, src []byte, ops []registry.AttrOp, buf []byte, off int) int {
	// Extract value portion from full attribute (strip header).
	var data []byte
	if src != nil {
		if val := extractAttrValue(src); val != nil {
			data = make([]byte, len(val))
			copy(data, val)
		}
	}

	// Apply ops: remove first, then add, then set (strip-before-tag within egress).
	// Set intentionally overrides all prior Remove/Add ops.
	for _, op := range ops {
		if op.Action == registry.AttrModRemove {
			data = removeValues(data, valueSize, op.Buf)
		}
	}
	for _, op := range ops {
		if op.Action == registry.AttrModAdd {
			data = append(data, op.Buf...)
		}
	}
	for _, op := range ops {
		if op.Action == registry.AttrModSet {
			data = make([]byte, len(op.Buf))
			copy(data, op.Buf)
		}
	}

	if len(data) == 0 {
		return off // Attribute omitted entirely.
	}

	// Bounds check: header (4 bytes) + data must fit in buf.
	needed := 4 + len(data)
	if off+needed > len(buf) {
		return off // Buffer too small; skip attribute (fail-safe).
	}

	// Cap data length at uint16 max (BGP extended-length attribute limit).
	if len(data) > 65535 {
		data = data[:65535]
	}

	// Write attribute: flags + code + extended length + data.
	buf[off] = 0xC0 | 0x10 // Optional Transitive + Extended Length
	buf[off+1] = byte(code)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(len(data))) //nolint:gosec // capped above
	copy(buf[off+4:], data)

	return off + 4 + len(data)
}

// removeValues removes all occurrences of toRemove from data, where each value
// is valueSize bytes. Returns the filtered data.
// toRemove MUST be exactly valueSize bytes; mismatched sizes are silently skipped.
func removeValues(data []byte, valueSize int, toRemove []byte) []byte {
	if len(toRemove) != valueSize {
		return data // Size mismatch: caller bug, silently preserve data.
	}
	var result []byte
	for i := 0; i+valueSize <= len(data); i += valueSize {
		if !bytes.Equal(data[i:i+valueSize], toRemove) {
			result = append(result, data[i:i+valueSize]...)
		}
	}
	return result
}
