// Design: docs/architecture/core-design.md -- generic attribute modification handlers
// Related: filter_delta.go -- textDeltaToModOps produces AttrModSet ops consumed by these handlers
// Related: forward_build.go -- buildModifiedPayload dispatches to registered handlers

package reactor

import (
	"encoding/binary"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// genericAttrSetHandler returns an AttrModHandler that supports AttrModSet for any
// attribute code. On Set, it writes the attribute header (flags + code + length) plus
// the value bytes from the op. If no Set op is present, it copies the source unchanged.
//
// This enables the policy filter text-delta-to-wire bridge for attributes that don't
// have specialized handlers (community types and OTC have their own).
func genericAttrSetHandler(flags, code byte) registry.AttrModHandler {
	return func(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
		// Find the last Set op (last wins).
		var setOp *registry.AttrOp
		for i := range ops {
			if ops[i].Action == registry.AttrModSet {
				setOp = &ops[i]
			}
		}

		if setOp == nil {
			// No set op: copy source unchanged.
			if len(src) > 0 && off+len(src) <= len(buf) {
				copy(buf[off:], src)
				return off + len(src)
			}
			return off
		}

		valLen := len(setOp.Buf)
		if valLen > 65535 {
			return off // BGP attribute value cannot exceed 65535 bytes.
		}

		// Extended length for values > 255 bytes.
		if valLen > 255 {
			needed := 4 + valLen
			if off+needed > len(buf) {
				return off
			}
			buf[off] = flags | 0x10 // Add extended length flag.
			buf[off+1] = code
			binary.BigEndian.PutUint16(buf[off+2:], uint16(valLen)) //nolint:gosec // capped at 65535 by BGP
			copy(buf[off+4:], setOp.Buf)
			return off + needed
		}

		needed := 3 + valLen
		if off+needed > len(buf) {
			return off
		}
		buf[off] = flags
		buf[off+1] = code
		buf[off+2] = byte(valLen)
		copy(buf[off+3:], setOp.Buf)
		return off + needed
	}
}

// Attribute flags per RFC 4271 Section 4.3:
//   - 0x40 = Well-known, Transitive (ORIGIN, AS_PATH, NEXT_HOP, LOCAL_PREF, ATOMIC_AGGREGATE)
//   - 0x80 = Optional, Non-transitive (MED)
//   - 0xC0 = Optional, Transitive (AGGREGATOR, ORIGINATOR_ID, CLUSTER_LIST)

// genericAttrCodes lists attribute codes that need generic set handlers.
// Community types (8, 16, 32) and OTC (35) already have specialized handlers
// registered by their plugins.
var genericAttrCodes = []struct {
	code  attribute.AttributeCode
	flags byte
}{
	{attribute.AttrOrigin, 0x40},          // Well-known mandatory
	{attribute.AttrASPath, 0x40},          // Well-known mandatory
	{attribute.AttrNextHop, 0x40},         // Well-known mandatory
	{attribute.AttrMED, 0x80},             // Optional non-transitive
	{attribute.AttrLocalPref, 0x40},       // Well-known (IBGP)
	{attribute.AttrAtomicAggregate, 0x40}, // Well-known discretionary
	{attribute.AttrAggregator, 0xC0},      // Optional transitive
	{attribute.AttrOriginatorID, 0x80},    // Optional non-transitive (RFC 4456)
	{attribute.AttrClusterList, 0x80},     // Optional non-transitive (RFC 4456)
}

// originatorIDHandler handles ORIGINATOR_ID (type 9, RFC 4456 Section 8).
// AttrModSet: set only if the attribute is absent in the source. If already present,
// copies the existing value unchanged (the original originator is preserved).
func originatorIDHandler() registry.AttrModHandler {
	return func(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
		// If source already has ORIGINATOR_ID, preserve it.
		if len(src) > 0 {
			if off+len(src) <= len(buf) {
				copy(buf[off:], src)
				return off + len(src)
			}
			return off
		}
		// Source absent: write new ORIGINATOR_ID from the Set op.
		for i := range ops {
			if ops[i].Action != registry.AttrModSet || len(ops[i].Buf) != 4 {
				continue
			}
			// Flags: 0x80 = Optional, Non-transitive (RFC 4456).
			needed := 3 + 4
			if off+needed > len(buf) {
				return off
			}
			buf[off] = 0x80
			buf[off+1] = byte(attribute.AttrOriginatorID)
			buf[off+2] = 4
			copy(buf[off+3:], ops[i].Buf)
			return off + needed
		}
		return off
	}
}

// clusterListHandler handles CLUSTER_LIST (type 10, RFC 4456 Section 8).
// AttrModPrepend: prepends a 4-byte cluster-id before any existing cluster-list values.
// AttrModSet: replaces the entire cluster-list.
func clusterListHandler() registry.AttrModHandler {
	return func(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
		// Collect prepend ops (cluster-ids to prepend in order).
		var prependBufs [][]byte
		var setBuf []byte
		for i := range ops {
			if ops[i].Action == registry.AttrModPrepend && len(ops[i].Buf) == 4 {
				prependBufs = append(prependBufs, ops[i].Buf)
			} else if ops[i].Action == registry.AttrModSet {
				setBuf = ops[i].Buf
			}
		}

		// Set overrides everything.
		if setBuf != nil {
			valLen := len(setBuf)
			needed := 3 + valLen
			if off+needed > len(buf) {
				return off
			}
			buf[off] = 0x80 // Optional, Non-transitive
			buf[off+1] = byte(attribute.AttrClusterList)
			buf[off+2] = byte(valLen)
			copy(buf[off+3:], setBuf)
			return off + needed
		}

		if len(prependBufs) == 0 {
			// No modifications: copy source unchanged.
			if len(src) > 0 && off+len(src) <= len(buf) {
				copy(buf[off:], src)
				return off + len(src)
			}
			return off
		}

		// Prepend: extract existing value from source, prepend new cluster-ids.
		var existingVal []byte
		if len(src) > 2 {
			// Source format: flags(1) + code(1) + len(1) + value(N)
			// or flags(1) + code(1) + len(2) + value(N) if extended.
			hdrLen := 3
			if src[0]&0x10 != 0 { // Extended length flag
				hdrLen = 4
			}
			if len(src) > hdrLen {
				existingVal = src[hdrLen:]
			}
		}

		prependLen := len(prependBufs) * 4
		valLen := prependLen + len(existingVal)

		// Use extended length if value > 255 bytes.
		if valLen > 255 {
			needed := 4 + valLen
			if off+needed > len(buf) {
				return off
			}
			buf[off] = 0x80 | 0x10 // Optional, Non-transitive, Extended
			buf[off+1] = byte(attribute.AttrClusterList)
			binary.BigEndian.PutUint16(buf[off+2:], uint16(valLen)) //nolint:gosec // capped by BGP
			w := off + 4
			for _, pb := range prependBufs {
				copy(buf[w:], pb)
				w += 4
			}
			copy(buf[w:], existingVal)
			return off + needed
		}

		needed := 3 + valLen
		if off+needed > len(buf) {
			return off
		}
		buf[off] = 0x80 // Optional, Non-transitive
		buf[off+1] = byte(attribute.AttrClusterList)
		buf[off+2] = byte(valLen)
		w := off + 3
		for _, pb := range prependBufs {
			copy(buf[w:], pb)
			w += 4
		}
		copy(buf[w:], existingVal)
		return off + needed
	}
}

// attrModHandlersWithDefaults returns the registered AttrModHandler map with
// generic set handlers filled in for attribute codes that lack specialized handlers.
// Called by the reactor at startup instead of registry.AttrModHandlers() directly.
func attrModHandlersWithDefaults() map[uint8]registry.AttrModHandler {
	handlers := registry.AttrModHandlers()
	for _, entry := range genericAttrCodes {
		code := byte(entry.code)
		if handlers[code] == nil {
			handlers[code] = genericAttrSetHandler(entry.flags, code)
		}
	}
	// Override ORIGINATOR_ID and CLUSTER_LIST with specialized handlers
	// that support set-if-absent and prepend semantics (RFC 4456).
	handlers[byte(attribute.AttrOriginatorID)] = originatorIDHandler()
	handlers[byte(attribute.AttrClusterList)] = clusterListHandler()
	return handlers
}
