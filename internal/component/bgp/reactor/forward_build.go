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
		return nil // Malformed.
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		return nil // Malformed.
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	attrStart := attrOffset + 2
	attrEnd := attrStart + attrLen
	if len(payload) < attrEnd {
		return nil // Malformed.
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
