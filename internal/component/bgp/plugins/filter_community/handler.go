// Design: docs/architecture/core-design.md — community filter egress AttrModHandlers
// Overview: filter_community.go — plugin entry point
// Related: egress.go — egress filter accumulates ops
// Related: filter.go — ingress filter (direct payload mutation)

package filter_community

import (
	"bytes"
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// communityAttrModHandler handles AttrModAdd/Remove for COMMUNITY (code 8, 4-byte values).
// Called by buildModifiedPayload during the progressive build for egress forwarding.
// src is the FULL attribute (flags+code+len+data), not just the value.
func communityAttrModHandler(src []byte, ops []filterapi.AttrOp, buf []byte, off int) int {
	return genericCommunityHandler(attribute.AttrCommunity, 4, src, ops, buf, off)
}

// largeCommunityAttrModHandler handles AttrModAdd/Remove for LARGE_COMMUNITY (code 32, 12-byte values).
func largeCommunityAttrModHandler(src []byte, ops []filterapi.AttrOp, buf []byte, off int) int {
	return genericCommunityHandler(attribute.AttrLargeCommunity, 12, src, ops, buf, off)
}

// extCommunityAttrModHandler handles AttrModAdd/Remove for EXTENDED_COMMUNITY (code 16, 8-byte values).
func extCommunityAttrModHandler(src []byte, ops []filterapi.AttrOp, buf []byte, off int) int {
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
func genericCommunityHandler(code attribute.AttributeCode, valueSize int, src []byte, ops []filterapi.AttrOp, buf []byte, off int) int {
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
		if op.Action != filterapi.AttrModRemove {
			continue
		}
		next, ok := removeValues(data, valueSize, op.Buf)
		if !ok {
			// Refuse THIS operation and say so, rather than preserving the data
			// silently as the original size guard did. The offending producer is
			// identifiable from one line, which is the whole point: the
			// route-server arity violation this replaced went unnoticed from the
			// day it was introduced because the guard never spoke
			// (ai/rules/fail-closed-guards.md).
			//
			// continue, not return: the attribute's other operations are
			// well-formed and unrelated, and dropping them would turn one
			// producer's bug into a second, wider behavior change.
			logger().Warn("attribute-modification remove buffer is not a whole number of wire values; operation refused",
				"attribute-code", int(code),
				"value-size", valueSize,
				"buffer-length", len(op.Buf))
			// Counted as well as logged: the refusal is silent at the wire (the
			// route goes out with the attribute unchanged), so without a metric
			// you have to already suspect the contract is being violated before
			// you would go looking for the log line.
			filterapi.RecordRemoveBufferRefused(byte(code))
			continue
		}
		data = next
	}
	for _, op := range ops {
		if op.Action == filterapi.AttrModAdd {
			data = append(data, op.Buf...)
		}
	}
	for _, op := range ops {
		if op.Action == filterapi.AttrModSet {
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

// removeValues removes from data every value that appears anywhere in toRemove,
// where each value is valueSize bytes. Returns the filtered data.
//
// toRemove is a SET: a whole number of valueSize-byte wire values, concatenated.
// One value is the common case and is a whole number, so every producer that
// emits one operation per value (reactor/filter_delta.go:221-224 splits on
// valueSize; egress.go:28-30 is per value) is unaffected.
//
// ok is false when len(toRemove) is not a whole multiple of valueSize. That is a
// caller-contract violation: the buffer cannot be interpreted as wire values at
// all, so no removal is safe and data is returned untouched. The BOOL, not a log
// line, is the signal, so the caller decides how loud to be and a unit test can
// assert the refusal without depending on logging configuration
// (ai/rules/fail-closed-guards.md: a guard must fail closed or say something --
// this one does both, and genericCommunityHandler emits the warning).
//
// The multi-value form is what made this function's original single-value rule a
// live defect rather than a style point. wireu.StripControlCommunities
// (internal/component/bgp/wireu/community.go:141) accumulates EVERY matching
// control community into one slice, and both route-server rails
// (reactor/reactor_api_forward.go:635, reactor/forward_rs.go:342) pass that slice
// as a single Remove operation. Under the old `len(toRemove) != valueSize` guard
// a route carrying two or more control communities had NONE of them removed and
// leaked the route server's internal tags to its clients.
func removeValues(data []byte, valueSize int, toRemove []byte) ([]byte, bool) {
	if valueSize <= 0 || len(toRemove)%valueSize != 0 {
		return data, false
	}
	var result []byte
	for i := 0; i+valueSize <= len(data); i += valueSize {
		if !containsValue(toRemove, valueSize, data[i:i+valueSize]) {
			result = append(result, data[i:i+valueSize]...)
		}
	}
	return result, true
}

// containsValue reports whether want appears as one of the valueSize-byte values
// in set. set is assumed to be a whole number of values; removeValues checks that
// before calling.
//
// A linear scan rather than a map: the sets here hold a handful of values (the
// control communities on one route, or one configured strip value), so building a
// map per attribute would cost more than it saves on a per-UPDATE forwarding path.
func containsValue(set []byte, valueSize int, want []byte) bool {
	for i := 0; i+valueSize <= len(set); i += valueSize {
		if bytes.Equal(set[i:i+valueSize], want) {
			return true
		}
	}
	return false
}
