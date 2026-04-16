// Design: docs/architecture/core-design.md — progressive build for egress attribute modification
// Design: .claude/rules/design-principles.md — zero-copy, copy-on-modify (Outgoing Peer Pool is the copy point)
// Overview: reactor_api_forward.go — UPDATE forwarding dispatch
// Related: reactor_notify.go — panic recovery helpers

package reactor

import (
	"encoding/binary"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// modBufPool provides reusable buffers for the progressive build.
// Standard UPDATE max body is 4096 - 19 = 4077 bytes.
// Extended messages can reach 65535 - 19 bytes, but these are rare
// and the pool buffer will be replaced with a larger allocation if needed.
var modBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 4096)
		return &buf
	},
}

// buildModifiedPayload applies attribute modifications to a source UPDATE payload
// using a single-pass progressive build into a pooled buffer.
//
// The source payload has the standard UPDATE structure:
//
//	withdrawn_len(2) + withdrawn + attr_len(2) + attrs + nlri
//
// The function walks source attributes, copies unchanged ones verbatim,
// and calls registered handlers for modified ones. New attributes (from ops
// with no matching source attribute) are appended after source attributes.
// The attr_len field is backfilled after all attributes are written.
//
// Copy-on-modify: when pp is non-nil and has a free buffer large enough for
// the payload, the modified data is written directly into the per-peer pool
// buffer. The caller stores the returned peerBufIdx in fwdItem so releaseItem
// returns it after the worker writes to TCP. When no per-peer buffer is
// available, falls back to sync.Pool + a result copy.
//
// nlriOverride: when non-nil, the function replaces the legacy NLRI section
// (payload[attrEnd:]) with the provided bytes. A zero-length (but non-nil)
// slice means "drop every legacy NLRI prefix"; callers use this for the
// per-prefix filter modify path when every prefix in the UPDATE was denied
// but attributes remained intact. A nil slice preserves the original NLRI
// copy. nlriOverride affects ONLY the legacy IPv4 NLRI section; MP_REACH /
// MP_UNREACH rewriting is out of scope for this function (filter plugins
// that need per-NLRI decisions on non-CIDR families must declare raw=true
// and return a full payload rewrite themselves).
//
// Returns (modified payload, peerBufIdx). peerBufIdx > 0 means the returned
// slice is backed by pp and MUST be returned via pp.Return(peerBufIdx).
// peerBufIdx == 0 means the slice is independently allocated (safe to retain).
// Returns (nil, 0) if no modifications were needed.
func buildModifiedPayload(
	payload []byte,
	mods *registry.ModAccumulator,
	handlers map[uint8]registry.AttrModHandler,
	pp *peerPool,
	nlriOverride []byte,
) ([]byte, int) {
	ops := mods.Ops()
	if len(ops) == 0 && nlriOverride == nil {
		return nil, 0
	}

	// Group ops by attribute code.
	opsByCode := groupOpsByCode(ops)

	// Parse source payload structure.
	if len(payload) < 4 {
		fwdLogger().Warn("malformed payload in buildModifiedPayload, skipping mods", "payloadLen", len(payload))
		return nil, 0
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		fwdLogger().Warn("malformed payload in buildModifiedPayload, skipping mods", "payloadLen", len(payload))
		return nil, 0
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	attrStart := attrOffset + 2
	attrEnd := attrStart + attrLen
	if len(payload) < attrEnd {
		fwdLogger().Warn("malformed payload in buildModifiedPayload, skipping mods", "payloadLen", len(payload))
		return nil, 0
	}

	// Try per-peer pool first (copy-on-modify: zero extra allocation).
	// The per-peer buffer is sized to the negotiated message max (4K or 64K),
	// which is always >= the payload. Slack for added attributes is covered
	// because modifications rarely exceed the original payload size.
	needSize := len(payload) + 256 // slack for added attributes
	buf, peerBufIdx, poolBuf := acquireModBuf(pp, needSize)

	// cleanupBuf returns the per-peer buffer on error and the sync.Pool
	// buffer on all exit paths.
	cleanupBuf := func() {
		if peerBufIdx > 0 && pp != nil {
			pp.Return(peerBufIdx)
			peerBufIdx = 0
		}
		if poolBuf != nil {
			modBufPool.Put(poolBuf)
		}
	}

	// Ensure buffers are returned on panic.
	defer func() {
		if peerBufIdx > 0 && pp != nil {
			pp.Return(peerBufIdx)
			peerBufIdx = 0
		}
		if poolBuf != nil {
			modBufPool.Put(poolBuf)
			poolBuf = nil
		}
	}()

	off := 0

	// Step 1: Copy withdrawn section verbatim.
	wdSectionLen := 2 + withdrawnLen
	if !safeCopy(buf, off, payload[:wdSectionLen]) {
		cleanupBuf()
		return nil, 0
	}
	off += wdSectionLen

	// Step 2: Skip attr_len (backfill later).
	attrLenPos := off
	off += 2

	// Step 3-5: Walk source attributes, apply handlers.
	// Stack-allocated: attribute codes are uint8, 256 entries covers all codes.
	var consumed [256]bool
	overflow := false
	srcOff := attrStart
	for srcOff < attrEnd {
		if srcOff+2 > len(payload) {
			break
		}
		flags := payload[srcOff]
		code := payload[srcOff+1]
		var hdrLen int
		var aLen uint16
		if flags&0x10 != 0 { // Extended length.
			if srcOff+4 > len(payload) {
				break
			}
			aLen = binary.BigEndian.Uint16(payload[srcOff+2 : srcOff+4])
			hdrLen = 4
		} else {
			if srcOff+3 > len(payload) {
				break
			}
			aLen = uint16(payload[srcOff+2])
			hdrLen = 3
		}
		attrTotalLen := hdrLen + int(aLen)
		if srcOff+attrTotalLen > attrEnd {
			break
		}

		srcAttr := payload[srcOff : srcOff+attrTotalLen]

		if codeOps, hasOps := opsByCode[code]; hasOps {
			consumed[code] = true
			handler := handlers[code]
			if handler == nil {
				// No handler: copy source unchanged, log warning.
				fwdLogger().Warn("no attr mod handler registered", "code", code)
				if !safeCopy(buf, off, srcAttr) {
					overflow = true
					break
				}
				off += len(srcAttr)
			} else {
				newOff := safeAttrModHandler(handler, code, srcAttr, codeOps, buf, off)
				if newOff < off || newOff > len(buf) {
					fwdLogger().Warn("attr mod handler returned invalid offset",
						"code", code, "off", off, "newOff", newOff, "bufLen", len(buf))
					if !safeCopy(buf, off, srcAttr) {
						overflow = true
						break
					}
					off += len(srcAttr)
				} else {
					off = newOff
				}
			}
		} else {
			// No ops for this attribute: copy verbatim.
			if !safeCopy(buf, off, srcAttr) {
				overflow = true
				break
			}
			off += len(srcAttr)
		}

		srcOff += attrTotalLen
	}

	if overflow {
		cleanupBuf()
		return nil, 0
	}

	// Step 6: Write unconsumed ops (new attributes).
	for code, codeOps := range opsByCode {
		if consumed[code] {
			continue
		}
		handler := handlers[code]
		if handler == nil {
			fwdLogger().Warn("no attr mod handler for new attribute", "code", code)
			continue
		}
		newOff := safeAttrModHandler(handler, code, nil, codeOps, buf, off)
		if newOff < off || newOff > len(buf) {
			fwdLogger().Warn("attr mod handler returned invalid offset for new attr", "code", code)
			continue // Skip this new attribute.
		}
		off = newOff
	}

	// Step 7: Backfill attr_len.
	newAttrLen := off - attrLenPos - 2
	if newAttrLen < 0 || newAttrLen > 65535 {
		fwdLogger().Warn("attr modification abandoned: attr_len out of range", "newAttrLen", newAttrLen)
		cleanupBuf()
		return nil, 0
	}
	binary.BigEndian.PutUint16(buf[attrLenPos:], uint16(newAttrLen)) //nolint:gosec // G115: bounded by check above

	// Step 8: Write NLRI section. When nlriOverride is non-nil the filter
	// chain has rewritten the legacy IPv4 NLRI (per-prefix modify path);
	// copy the override bytes instead of the original NLRI tail. An
	// override of length zero explicitly drops every legacy NLRI prefix.
	if nlriOverride != nil {
		if len(nlriOverride) > 0 {
			if !safeCopy(buf, off, nlriOverride) {
				cleanupBuf()
				return nil, 0
			}
			off += len(nlriOverride)
		}
	} else {
		nlriLen := len(payload) - attrEnd
		if nlriLen > 0 {
			if !safeCopy(buf, off, payload[attrEnd:]) {
				cleanupBuf()
				return nil, 0
			}
			off += nlriLen
		}
	}

	// Per-peer buffer path: return the slice directly. The caller stores
	// peerBufIdx in fwdItem so releaseItem returns it after TCP write.
	// No copy needed -- the buffer IS the result.
	if peerBufIdx > 0 {
		if poolBuf != nil {
			modBufPool.Put(poolBuf)
			poolBuf = nil
		}
		idx := peerBufIdx
		peerBufIdx = 0 // prevent defer from double-returning
		return buf[:off], idx
	}

	// Sync.Pool fallback: copy result so pool buffer can be returned.
	result := make([]byte, off) // pool-fallback
	copy(result, buf[:off])

	return result, 0
}

// buildWithdrawalPayload converts an announce UPDATE payload to a withdrawal.
// RFC 9494: EBGP non-LLGR peers must receive a withdrawal for stale routes.
//
// For IPv4 unicast (legacy NLRI in payload tail):
//
//	Move NLRI bytes to Withdrawn Routes, set attr_len=0.
//	Result: withdrawn_len(2) + nlri_bytes + attr_len(2)=0
//
// For other families (MP_REACH_NLRI attr 14):
//
//	Extract AFI/SAFI + NLRI from MP_REACH, build MP_UNREACH_NLRI (attr 15).
//	Result: withdrawn_len(2)=0 + attr_len(2) + mp_unreach_attr
//
// Copy-on-modify: when pp is non-nil and has a free buffer, the result is
// written directly into the per-peer pool buffer. The caller stores the
// returned index in fwdItem so releaseItem returns it after the worker
// writes to TCP. When no per-peer buffer is available, falls back to
// sync.Pool + a result copy, matching buildModifiedPayload's shape.
//
// Returns (nil, 0) if the payload cannot be converted (malformed or
// unsupported).
func buildWithdrawalPayload(payload []byte, pp *peerPool) ([]byte, int) {
	if len(payload) < 4 {
		return nil, 0
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		return nil, 0
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	attrStart := attrOffset + 2
	attrEnd := attrStart + attrLen
	if len(payload) < attrEnd {
		return nil, 0
	}

	// Acquire a buffer sized to the worst-case withdrawal (<= len(payload)
	// since a withdrawal is strictly shorter than the source announce).
	needSize := len(payload) + 4 // slack for headers
	buf, peerBufIdx, poolBuf := acquireModBuf(pp, needSize)
	defer func() {
		if poolBuf != nil {
			modBufPool.Put(poolBuf)
		}
	}()

	nlriBytes := payload[attrEnd:]
	var n int
	if len(nlriBytes) > 0 {
		// IPv4 unicast: move NLRI to withdrawn routes, no attributes.
		n = writeIPv4Withdrawal(buf, nlriBytes)
	} else {
		// No legacy NLRI: look for MP_REACH_NLRI (attr code 14) to convert.
		n = writeMPUnreachFromReach(buf, payload[attrStart:attrEnd])
	}

	if n == 0 {
		if peerBufIdx > 0 && pp != nil {
			pp.Return(peerBufIdx)
		}
		return nil, 0
	}

	if peerBufIdx > 0 {
		return buf[:n], peerBufIdx
	}
	// Sync.Pool fallback: copy result so pool buffer can be returned.
	result := make([]byte, n) // pool-fallback
	copy(result, buf[:n])
	return result, 0
}

// acquireModBuf returns a buffer sized for modification output. Prefers
// the per-peer pool (zero-copy on hit); falls back to modBufPool for
// small payloads; falls back to a fresh make only for oversized payloads
// when both pools are unavailable. Returns (buf, peerBufIdx, poolBufPtr).
// peerBufIdx > 0 means pp owns the buffer. poolBufPtr != nil means the
// caller MUST return it to modBufPool after use.
func acquireModBuf(pp *peerPool, needSize int) ([]byte, int, *[]byte) {
	if pp != nil {
		b, idx := pp.Get()
		if idx > 0 && len(b) >= needSize {
			return b, idx, nil
		} else if idx > 0 {
			pp.Return(idx)
		}
	}
	if needSize <= 4096 {
		if poolBuf, ok := modBufPool.Get().(*[]byte); ok {
			return *poolBuf, 0, poolBuf
		}
		return make([]byte, 4096), 0, nil
	}
	return make([]byte, needSize), 0, nil
}

// writeIPv4Withdrawal writes an IPv4 withdrawal (withdrawn_len + NLRI +
// attr_len=0) into buf and returns the bytes written. Returns 0 if buf
// is too small or nlri exceeds the uint16 withdrawn_len ceiling.
func writeIPv4Withdrawal(buf, nlri []byte) int {
	need := 2 + len(nlri) + 2
	if len(buf) < need || len(nlri) > 65535 {
		return 0
	}
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(nlri)))
	copy(buf[2:], nlri)
	buf[2+len(nlri)] = 0
	buf[2+len(nlri)+1] = 0
	return need
}

// writeMPUnreachFromReach extracts AFI/SAFI + NLRI from MP_REACH_NLRI
// (attr 14) and writes an UPDATE body with MP_UNREACH_NLRI (attr 15)
// directly into buf. Returns bytes written, or 0 if no MP_REACH was
// found / the payload is malformed / buf is too small.
//
// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_Len(1) + NH(var) + Reserved(1) + NLRI(var).
// MP_UNREACH_NLRI value: AFI(2) + SAFI(1) + NLRI(var).
func writeMPUnreachFromReach(buf, attrs []byte) int {
	off := 0
	for off < len(attrs) {
		if off+2 > len(attrs) {
			return 0
		}
		flags := attrs[off]
		code := attrs[off+1]
		var hdrLen int
		var aLen uint16
		if flags&0x10 != 0 { // Extended length.
			if off+4 > len(attrs) {
				return 0
			}
			aLen = binary.BigEndian.Uint16(attrs[off+2 : off+4])
			hdrLen = 4
		} else {
			if off+3 > len(attrs) {
				return 0
			}
			aLen = uint16(attrs[off+2])
			hdrLen = 3
		}
		valStart := off + hdrLen
		valEnd := valStart + int(aLen)
		if valEnd > len(attrs) {
			return 0
		}

		if code != 14 { // not MP_REACH_NLRI
			off = valEnd
			continue
		}

		val := attrs[valStart:valEnd]
		if len(val) < 4 { // AFI(2) + SAFI(1) + NH_Len(1) minimum
			return 0
		}
		nhLen := int(val[3])
		nlriStart := 4 + nhLen + 1 // skip NH + reserved byte
		if nlriStart > len(val) {
			return 0
		}
		nlriData := val[nlriStart:]

		// Compute size of MP_UNREACH attribute value (AFI+SAFI+NLRI).
		unreachValLen := 3 + len(nlriData)
		if unreachValLen > 65535 {
			return 0
		}

		// Attribute header size: 3 (short) or 4 (extended) bytes.
		var attrHdrLen int
		var attrFlags byte
		if unreachValLen > 255 {
			attrFlags = 0x90 // Optional, Transitive, Extended Length.
			attrHdrLen = 4
		} else {
			attrFlags = 0x80 // Optional, Transitive.
			attrHdrLen = 3
		}
		attrTotalLen := attrHdrLen + unreachValLen

		// Total wire body: withdrawn_len(2) + attr_len(2) + attr.
		need := 4 + attrTotalLen
		if len(buf) < need {
			return 0
		}

		// withdrawn_len = 0.
		buf[0] = 0
		buf[1] = 0
		// attr_len covers only the attribute (header + value).
		binary.BigEndian.PutUint16(buf[2:4], uint16(attrTotalLen)) //nolint:gosec // G115: bounded by uint16 check
		// MP_UNREACH header.
		w := 4
		buf[w] = attrFlags
		buf[w+1] = 15 // MP_UNREACH_NLRI
		if attrFlags == 0x90 {
			binary.BigEndian.PutUint16(buf[w+2:w+4], uint16(unreachValLen)) //nolint:gosec // G115: bounded above
			w += 4
		} else {
			buf[w+2] = byte(unreachValLen)
			w += 3
		}
		// MP_UNREACH value: AFI(2) + SAFI(1) + NLRI.
		copy(buf[w:w+2], val[0:2])
		buf[w+2] = val[2]
		copy(buf[w+3:], nlriData)
		return need
	}

	return 0 // No MP_REACH_NLRI found.
}

// safeCopy copies src into buf at offset off, returning false if it would overflow.
func safeCopy(buf []byte, off int, src []byte) bool {
	if off+len(src) > len(buf) {
		return false
	}
	copy(buf[off:], src)
	return true
}

// groupOpsByCode groups AttrOps by attribute code.
func groupOpsByCode(ops []registry.AttrOp) map[uint8][]registry.AttrOp {
	m := make(map[uint8][]registry.AttrOp, len(ops))
	for i := range ops {
		m[ops[i].Code] = append(m[ops[i].Code], ops[i])
	}
	return m
}

// safeAttrModHandler calls an AttrModHandler with panic recovery.
// Returns the new offset on success, or the original offset on panic.
func safeAttrModHandler(handler registry.AttrModHandler, code uint8, src []byte, ops []registry.AttrOp, buf []byte, off int) (newOff int) {
	defer func() {
		if r := recover(); r != nil {
			fwdLogger().Error("attr mod handler panic, skipping modification",
				"code", code, "panic", r)
			// On panic with source attr, copy it unchanged if buffer has room.
			if len(src) > 0 && off+len(src) <= len(buf) {
				copy(buf[off:], src)
				newOff = off + len(src)
			} else {
				newOff = off
			}
		}
	}()
	return handler(src, ops, buf, off)
}
