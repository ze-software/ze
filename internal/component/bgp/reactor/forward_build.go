// Design: docs/architecture/core-design.md — progressive build for egress attribute modification
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
// Returns the modified payload, or nil if no modifications were needed
// (caller should use the original payload). The returned slice is newly
// allocated and safe to retain.
func buildModifiedPayload(
	payload []byte,
	mods *registry.ModAccumulator,
	handlers map[uint8]registry.AttrModHandler,
) []byte {
	ops := mods.Ops()
	if len(ops) == 0 {
		return nil
	}

	// Group ops by attribute code.
	opsByCode := groupOpsByCode(ops)

	// Parse source payload structure.
	if len(payload) < 4 {
		fwdLogger().Warn("malformed payload in buildModifiedPayload, skipping mods", "payloadLen", len(payload))
		return nil
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		fwdLogger().Warn("malformed payload in buildModifiedPayload, skipping mods", "payloadLen", len(payload))
		return nil
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	attrStart := attrOffset + 2
	attrEnd := attrStart + attrLen
	if len(payload) < attrEnd {
		fwdLogger().Warn("malformed payload in buildModifiedPayload, skipping mods", "payloadLen", len(payload))
		return nil
	}

	// Get pooled buffer. If payload is larger than standard, allocate directly.
	bufSize := len(payload) + 256 // Slack for added attributes.
	var buf []byte
	var poolBuf *[]byte
	if bufSize <= 4096 {
		poolBuf, _ = modBufPool.Get().(*[]byte)
		if poolBuf != nil {
			buf = *poolBuf
		} else {
			buf = make([]byte, 4096)
		}
	} else {
		buf = make([]byte, bufSize)
	}

	// Ensure pool buffer is returned on all exit paths (including panic).
	defer func() {
		if poolBuf != nil {
			modBufPool.Put(poolBuf)
		}
	}()

	off := 0

	// Step 1: Copy withdrawn section verbatim.
	wdSectionLen := 2 + withdrawnLen
	if !safeCopy(buf, off, payload[:wdSectionLen]) {
		return nil
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
		return nil
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
		return nil
	}
	binary.BigEndian.PutUint16(buf[attrLenPos:], uint16(newAttrLen)) //nolint:gosec // G115: bounded by check above

	// Step 8: Copy NLRI section verbatim.
	nlriLen := len(payload) - attrEnd
	if nlriLen > 0 {
		if !safeCopy(buf, off, payload[attrEnd:]) {
			return nil // Buffer overflow: abandon modification.
		}
		off += nlriLen
	}

	// Make a result copy so we can return the pool buffer.
	result := make([]byte, off)
	copy(result, buf[:off])

	return result
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
// Returns nil if the payload cannot be converted (malformed or unsupported).
func buildWithdrawalPayload(payload []byte) []byte {
	if len(payload) < 4 {
		return nil
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		return nil
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	attrStart := attrOffset + 2
	attrEnd := attrStart + attrLen
	if len(payload) < attrEnd {
		return nil
	}

	// Check for legacy IPv4 NLRI (bytes after attributes).
	nlriBytes := payload[attrEnd:]
	if len(nlriBytes) > 0 {
		// IPv4 unicast: move NLRI to withdrawn routes, no attributes.
		result := make([]byte, 2+len(nlriBytes)+2)
		binary.BigEndian.PutUint16(result[0:2], uint16(len(nlriBytes)))
		copy(result[2:], nlriBytes)
		// attr_len = 0 (last 2 bytes are already zero)
		return result
	}

	// No legacy NLRI: look for MP_REACH_NLRI (attr code 14) to convert.
	return buildMPUnreachFromReach(payload[attrStart:attrEnd])
}

// buildMPUnreachFromReach extracts AFI/SAFI + NLRI from MP_REACH_NLRI (attr 14)
// and builds an MP_UNREACH_NLRI (attr 15) withdrawal.
//
// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_Len(1) + NH(var) + Reserved(1) + NLRI(var).
// MP_UNREACH_NLRI value: AFI(2) + SAFI(1) + NLRI(var).
func buildMPUnreachFromReach(attrs []byte) []byte {
	off := 0
	for off < len(attrs) {
		if off+2 > len(attrs) {
			return nil
		}
		flags := attrs[off]
		code := attrs[off+1]
		var hdrLen int
		var aLen uint16
		if flags&0x10 != 0 { // Extended length.
			if off+4 > len(attrs) {
				return nil
			}
			aLen = binary.BigEndian.Uint16(attrs[off+2 : off+4])
			hdrLen = 4
		} else {
			if off+3 > len(attrs) {
				return nil
			}
			aLen = uint16(attrs[off+2])
			hdrLen = 3
		}
		valStart := off + hdrLen
		valEnd := valStart + int(aLen)
		if valEnd > len(attrs) {
			return nil
		}

		if code == 14 { // MP_REACH_NLRI
			val := attrs[valStart:valEnd]
			if len(val) < 4 { // AFI(2) + SAFI(1) + NH_Len(1) minimum
				return nil
			}
			afi := val[0:2]
			safi := val[2]
			nhLen := int(val[3])
			nlriStart := 4 + nhLen + 1 // skip NH + reserved byte
			if nlriStart > len(val) {
				return nil
			}
			nlriData := val[nlriStart:]

			// Build MP_UNREACH_NLRI: AFI(2) + SAFI(1) + NLRI
			unreachVal := make([]byte, 3+len(nlriData))
			copy(unreachVal[0:2], afi)
			unreachVal[2] = safi
			copy(unreachVal[3:], nlriData)

			// Build attribute header for MP_UNREACH (code 15, optional transitive)
			var unreachAttr []byte
			if len(unreachVal) > 255 {
				unreachAttr = make([]byte, 4+len(unreachVal))
				unreachAttr[0] = 0x90 // Optional, Transitive, Extended Length
				unreachAttr[1] = 15
				binary.BigEndian.PutUint16(unreachAttr[2:4], uint16(len(unreachVal)))
				copy(unreachAttr[4:], unreachVal)
			} else {
				unreachAttr = make([]byte, 3+len(unreachVal))
				unreachAttr[0] = 0x80 // Optional, Transitive
				unreachAttr[1] = 15
				unreachAttr[2] = byte(len(unreachVal))
				copy(unreachAttr[3:], unreachVal)
			}

			// Build payload: withdrawn_len=0, attr_len=unreachAttr, no NLRI
			result := make([]byte, 2+2+len(unreachAttr))
			// withdrawn_len = 0 (first 2 bytes already zero)
			binary.BigEndian.PutUint16(result[2:4], uint16(len(unreachAttr)))
			copy(result[4:], unreachAttr)
			return result
		}

		off = valEnd
	}

	return nil // No MP_REACH_NLRI found.
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
