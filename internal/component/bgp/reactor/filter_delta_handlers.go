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
		// Find the last Set or Suppress op (last wins).
		var setOp *registry.AttrOp
		for i := range ops {
			if ops[i].Action == registry.AttrModSet || ops[i].Action == registry.AttrModSuppress {
				setOp = &ops[i]
			}
		}

		// Suppress: write nothing, effectively removing the attribute.
		if setOp != nil && setOp.Action == registry.AttrModSuppress {
			return off
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

// mpReachNextHopHandler handles next-hop rewriting inside MP_REACH_NLRI
// (type 14, RFC 4760 Section 3).
//
// The op Buf carries the new next-hop bytes (16 bytes for an IPv6 global
// address, or 32 bytes for a global + link-local pair per RFC 2545). The
// handler parses the source attribute to locate the existing next-hop field,
// replaces it in place, and copies the surrounding bytes (AFI/SAFI header,
// reserved byte, NLRI) unchanged. The attribute length is adjusted only when
// the new next-hop length differs from the existing one.
//
// If the source attribute is absent or malformed, the handler leaves the
// output untouched: MP_REACH_NLRI carries the route's NLRI itself, so the
// rewrite only makes sense when a source attribute already exists.
//
// Only AttrModSet ops are honored (last-wins). AttrModSuppress on a
// MP_REACH_NLRI would strip the entire route, which is a withdraw -- that is
// expressed via ModAccumulator.SetWithdraw(), not via this handler.
func mpReachNextHopHandler() registry.AttrModHandler {
	return func(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
		// Pick the last Set op.
		var setBuf []byte
		for i := range ops {
			if ops[i].Action == registry.AttrModSet && len(ops[i].Buf) > 0 {
				setBuf = ops[i].Buf
			}
		}

		// No rewrite requested: copy source unchanged.
		if setBuf == nil {
			if len(src) > 0 && off+len(src) <= len(buf) {
				copy(buf[off:], src)
				return off + len(src)
			}
			return off
		}

		// Cannot construct MP_REACH_NLRI from nothing -- the NLRI portion
		// lives in the source attribute value. Drop the rewrite request when
		// the source is absent or too short to parse.
		if len(src) < 3 {
			return off
		}

		// Parse source header to find the value region.
		flags := src[0]
		code := src[1]
		if code != byte(attribute.AttrMPReachNLRI) {
			// Belt-and-braces: src was routed to this handler, code should match.
			if len(src) > 0 && off+len(src) <= len(buf) {
				copy(buf[off:], src)
				return off + len(src)
			}
			return off
		}
		hdrLen := 3
		srcValLen := int(src[2])
		if flags&0x10 != 0 { // extended length
			if len(src) < 4 {
				return off
			}
			hdrLen = 4
			srcValLen = int(binary.BigEndian.Uint16(src[2:4]))
		}
		if hdrLen+srcValLen > len(src) {
			return off // truncated source, bail
		}
		val := src[hdrLen : hdrLen+srcValLen]

		// Value layout: AFI(2) + SAFI(1) + NHLen(1) + NH(NHLen) + Reserved(1) + NLRI.
		if len(val) < 5 {
			return off
		}
		nhLen := int(val[3])
		nhStart := 4
		nhEnd := nhStart + nhLen
		if nhEnd+1 > len(val) { // +1 for the reserved byte
			return off
		}

		// The new next-hop must be exactly one of the allowed lengths:
		//   - 4  bytes: IPv4 next-hop (used by labeled unicast / VPN families).
		//   - 16 bytes: IPv6 global-only next-hop.
		//   - 32 bytes: IPv6 global + link-local per RFC 2545 Section 3.
		// A mismatched op length is a caller bug; the route is left unchanged
		// (the caller should have produced a valid op).
		newNHLen := len(setBuf)
		if newNHLen != 4 && newNHLen != 16 && newNHLen != 32 {
			if len(src) > 0 && off+len(src) <= len(buf) {
				copy(buf[off:], src)
				return off + len(src)
			}
			return off
		}
		newValLen := srcValLen - nhLen + newNHLen

		// BGP attribute value length is capped at 65535 (RFC 4271 §4.3).
		// A near-full source MP_REACH combined with NH growth (e.g. 0 -> 32)
		// could push the new value past that cap. Refuse the rewrite and
		// preserve the source: the alternative (uint16 truncation) corrupts
		// the wire output by claiming fewer bytes than we actually wrote.
		if newValLen < 0 || newValLen > 65535 {
			if len(src) > 0 && off+len(src) <= len(buf) {
				copy(buf[off:], src)
				return off + len(src)
			}
			return off
		}

		// Decide output header width based on the new value length.
		outFlags := flags
		outHdrLen := 3
		if newValLen > 255 {
			outFlags |= 0x10
			outHdrLen = 4
		} else {
			outFlags &^= 0x10
		}

		needed := outHdrLen + newValLen
		if off+needed > len(buf) {
			return off
		}

		// Write new header.
		buf[off] = outFlags
		buf[off+1] = code
		if outHdrLen == 4 {
			binary.BigEndian.PutUint16(buf[off+2:], uint16(newValLen)) //nolint:gosec // capped by BGP
		} else {
			buf[off+2] = byte(newValLen)
		}

		// Write value: AFI(2) + SAFI(1) + new NHLen + new NH + Reserved(1) + NLRI
		w := off + outHdrLen
		copy(buf[w:], val[:3]) // AFI + SAFI
		w += 3
		buf[w] = byte(newNHLen)
		w++
		copy(buf[w:], setBuf)
		w += newNHLen
		// Reserved byte + NLRI come after the old next-hop in the source.
		copy(buf[w:], val[nhEnd:])
		w += len(val) - nhEnd
		return w
	}
}

// aspathHandler handles AS_PATH (type 2) with support for AttrModPrepend.
// Prepend inserts a new AS_SEQUENCE segment before the existing AS_PATH value.
// Set replaces the entire attribute (via genericAttrSetHandler fallback).
func aspathHandler() registry.AttrModHandler {
	setHandler := genericAttrSetHandler(0x40, byte(attribute.AttrASPath))

	return func(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
		// Check for prepend ops.
		var prependBufs [][]byte
		hasPrepend := false
		for i := range ops {
			if ops[i].Action == registry.AttrModPrepend && len(ops[i].Buf) > 0 {
				prependBufs = append(prependBufs, ops[i].Buf)
				hasPrepend = true
			}
		}

		if !hasPrepend {
			// No prepend: delegate to generic set handler.
			return setHandler(src, ops, buf, off)
		}

		// Prepend: extract existing value from source, prepend new segment(s).
		var existingVal []byte
		if len(src) > 2 {
			hdrLen := 3
			if src[0]&0x10 != 0 {
				hdrLen = 4
			}
			if len(src) > hdrLen {
				existingVal = src[hdrLen:]
			}
		}

		prependLen := 0
		for _, pb := range prependBufs {
			prependLen += len(pb)
		}
		valLen := prependLen + len(existingVal)

		if valLen > 65535 {
			// Cannot exceed BGP attribute value limit; copy source unchanged.
			if len(src) > 0 && off+len(src) <= len(buf) {
				copy(buf[off:], src)
				return off + len(src)
			}
			return off
		}

		if valLen > 255 {
			needed := 4 + valLen
			if off+needed > len(buf) {
				return off
			}
			buf[off] = 0x40 | 0x10 // Well-known, Transitive, Extended
			buf[off+1] = byte(attribute.AttrASPath)
			binary.BigEndian.PutUint16(buf[off+2:], uint16(valLen)) //nolint:gosec // capped at 65535
			w := off + 4
			for _, pb := range prependBufs {
				copy(buf[w:], pb)
				w += len(pb)
			}
			copy(buf[w:], existingVal)
			return off + needed
		}

		needed := 3 + valLen
		if off+needed > len(buf) {
			return off
		}
		buf[off] = 0x40 // Well-known, Transitive
		buf[off+1] = byte(attribute.AttrASPath)
		buf[off+2] = byte(valLen)
		w := off + 3
		for _, pb := range prependBufs {
			copy(buf[w:], pb)
			w += len(pb)
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
	// Override AS_PATH with handler supporting prepend (policy as-path-prepend).
	handlers[byte(attribute.AttrASPath)] = aspathHandler()
	// Override ORIGINATOR_ID and CLUSTER_LIST with specialized handlers
	// that support set-if-absent and prepend semantics (RFC 4456).
	handlers[byte(attribute.AttrOriginatorID)] = originatorIDHandler()
	handlers[byte(attribute.AttrClusterList)] = clusterListHandler()
	// MP_REACH_NLRI next-hop rewriting (RFC 4760 §3, RFC 2545 §3).
	handlers[byte(attribute.AttrMPReachNLRI)] = mpReachNextHopHandler()
	return handlers
}
