// Design: docs/architecture/core-design.md — progressive build for egress attribute modification
// Design: .claude/rules/design-principles.md — zero-copy, copy-on-modify (Outgoing Peer Pool is the copy point)
// RFC: rfc/short/rfc4271.md — UPDATE body layout, Total Path Attribute Length
// RFC: rfc/short/rfc9494.md — announce-to-withdrawal conversion for non-LLGR EBGP peers
// Overview: reactor_api_forward.go — UPDATE forwarding dispatch
// Detail: forward_modify_failure.go — why a modification could not be applied
// Related: reactor_notify.go — panic recovery helpers

package reactor

import (
	"encoding/binary"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
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
// Returns (modified payload, peerBufIdx, failure). peerBufIdx > 0 means the
// returned slice is backed by pp and MUST be returned via pp.Return(peerBufIdx).
// peerBufIdx == 0 means the slice is independently allocated (safe to retain).
//
// A nil payload has TWO meanings and the caller MUST tell them apart by the
// third value (ai/rules/fail-closed-guards.md):
//
//   - (nil, 0, modifyFailureNone): no modifications were needed. Forward the
//     route as it stands.
//   - (nil, 0, anything else): the modifications were needed and could NOT be
//     applied. The caller MUST suppress the route for this destination.
//     Forwarding it would send a route the policy was supposed to change.
//
// The third return value is not optional bookkeeping. For every path that
// RETURNS, the compiler forces a reason to be named, so a new returning failure
// cannot be added silently.
//
// That guarantee does NOT extend to paths that keep building. A missing handler,
// a faulting handler and a truncated attribute section all warn and carry on,
// and an independent review found they still reported success while forwarding a
// route the policy had not changed -- the same fail-open on a different shape.
// Those paths now set `unapplied`, which is checked before the payload is handed
// out. The compiler cannot enforce that, so a future edit adding a
// keep-building failure path MUST set `unapplied` by hand.
func buildModifiedPayload(
	payload []byte,
	mods *filterapi.ModAccumulator,
	handlers map[uint8]filterapi.AttrModHandler,
	pp *peerPool,
	nlriOverride []byte,
) ([]byte, int, modifyFailure) {
	ops := mods.Ops()
	// The ModAccumulator can also carry per-peer NLRI rewrites. An explicit
	// nlriOverride argument (the legacy per-prefix modify path) takes precedence;
	// otherwise the accumulator's announce-NLRI rewrite applies. The withdrawn
	// rewrite has no legacy argument, so it comes only from the accumulator.
	if nlriOverride == nil {
		nlriOverride = mods.NLRIRewrite()
	}
	withdrawnOverride := mods.WithdrawnRewrite()
	if len(ops) == 0 && nlriOverride == nil && withdrawnOverride == nil {
		// The ONE legitimate nil: there was nothing to apply.
		return nil, 0, modifyFailureNone
	}

	// Group ops by attribute code.
	opsByCode := groupOpsByCode(ops)

	// Parse source payload structure. From here on every nil return is a
	// FAILURE: modifications were required and could not be applied.
	if len(payload) < 4 {
		fwdLogger().Warn("malformed payload in buildModifiedPayload, suppressing route", "payloadLen", len(payload))
		return nil, 0, modifyFailureMalformed
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		fwdLogger().Warn("malformed payload in buildModifiedPayload, suppressing route", "payloadLen", len(payload))
		return nil, 0, modifyFailureMalformed
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	attrStart := attrOffset + 2
	attrEnd := attrStart + attrLen
	if len(payload) < attrEnd {
		fwdLogger().Warn("malformed payload in buildModifiedPayload, suppressing route", "payloadLen", len(payload))
		return nil, 0, modifyFailureMalformed
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

	// Step 1: Copy the withdrawn section. When withdrawnOverride is non-nil the
	// egress filter has rewritten the withdrawn NLRI (per-peer prefix translation
	// on withdrawal, keeping adj-rib-out consistent): write a fresh 2-byte length
	// plus the override bytes. A zero-length (non-nil) override drops every
	// withdrawn prefix. A nil override copies the original withdrawn section.
	if withdrawnOverride != nil {
		if len(withdrawnOverride) > 65535 {
			fwdLogger().Warn("withdrawn rewrite exceeds the 2-octet length field, suppressing route",
				"len", len(withdrawnOverride), "max", 65535)
			cleanupBuf()
			return nil, 0, modifyFailureWithdrawnSize
		}
		binary.BigEndian.PutUint16(buf[off:], uint16(len(withdrawnOverride))) //nolint:gosec // G115: bounded by check above
		off += 2
		if len(withdrawnOverride) > 0 {
			if !safeCopy(buf, off, withdrawnOverride) {
				cleanupBuf()
				return nil, 0, modifyFailureOverflow
			}
			off += len(withdrawnOverride)
		}
	} else {
		wdSectionLen := 2 + withdrawnLen
		if !safeCopy(buf, off, payload[:wdSectionLen]) {
			cleanupBuf()
			return nil, 0, modifyFailureOverflow
		}
		off += wdSectionLen
	}

	// Step 2: Skip attr_len (backfill later).
	attrLenPos := off
	off += 2

	// Step 3-5: Walk source attributes, apply handlers.
	// Stack-allocated: attribute codes are uint8, 256 entries covers all codes.
	var consumed [256]bool
	overflow := false
	// unapplied records a modification that was REQUIRED but did not land, on a
	// path that keeps building rather than returning. Those paths used to warn
	// and carry on, so the function returned a well-formed payload plus
	// modifyFailureNone and every caller forwarded a route the policy had not
	// actually changed -- the same fail-open T1-1 closed on the returning paths.
	// The compiler cannot catch these, because they fall through rather than
	// return; this variable is what makes them reportable.
	unapplied := modifyFailureNone
	// attrCount is reported in the suppression warning: an operator seeing a
	// route dropped needs the shape of the UPDATE that could not be rebuilt,
	// not just that it failed (ai/rules/error-messages.md).
	attrCount := 0
	srcOff := attrStart
	for srcOff < attrEnd {
		// Every break below leaves the remaining attributes UNCOPIED. The
		// output would be a truncated attribute section that still looks
		// well-formed, so each one is a failure rather than an early exit.
		// A well-formed section leaves this loop by srcOff reaching attrEnd.
		if srcOff+2 > len(payload) {
			unapplied = modifyFailureTruncated
			break
		}
		flags := payload[srcOff]
		code := payload[srcOff+1]
		var hdrLen int
		var aLen uint16
		if flags&0x10 != 0 { // Extended length.
			if srcOff+4 > len(payload) {
				unapplied = modifyFailureTruncated
				break
			}
			aLen = binary.BigEndian.Uint16(payload[srcOff+2 : srcOff+4])
			hdrLen = 4
		} else {
			if srcOff+3 > len(payload) {
				unapplied = modifyFailureTruncated
				break
			}
			aLen = uint16(payload[srcOff+2])
			hdrLen = 3
		}
		attrTotalLen := hdrLen + int(aLen)
		if srcOff+attrTotalLen > attrEnd {
			unapplied = modifyFailureTruncated
			break
		}

		srcAttr := payload[srcOff : srcOff+attrTotalLen]

		if codeOps := opsByCode[code]; len(codeOps) > 0 {
			consumed[code] = true
			handler := handlers[code]
			if handler == nil {
				// No handler: the operations for this code are NOT applied.
				// Copying the source through and reporting success would forward
				// the route with the very attribute the policy meant to change,
				// so record the failure and let the caller suppress.
				fwdLogger().Warn("no attr mod handler registered, suppressing route", "code", code)
				unapplied = modifyFailureNoHandler
				if !safeCopy(buf, off, srcAttr) {
					overflow = true
					break
				}
				off += len(srcAttr)
			} else {
				newOff, panicked := safeAttrModHandler(handler, code, srcAttr, codeOps, buf, off)
				if panicked {
					unapplied = modifyFailureHandlerFault
				}
				if newOff < off || newOff > len(buf) {
					// The handler panicked (safeAttrModHandler recovered and
					// returned the original offset) or returned an offset
					// outside the buffer. Either way its operations did not
					// land, so this is a failure, not a fallback.
					fwdLogger().Warn("attr mod handler faulted, suppressing route",
						"code", code, "off", off, "newOff", newOff, "bufLen", len(buf))
					unapplied = modifyFailureHandlerFault
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
		attrCount++
	}

	if overflow {
		fwdLogger().Warn("attribute modification overflowed the output buffer, suppressing route",
			"payloadLen", len(payload), "bufLen", len(buf), "attrCount", attrCount)
		cleanupBuf()
		return nil, 0, modifyFailureOverflow
	}

	// Step 6: Write unconsumed ops (new attributes).
	for codeInt := range opsByCode {
		codeOps := opsByCode[codeInt]
		code := uint8(codeInt)
		if len(codeOps) == 0 || consumed[code] {
			continue
		}
		handler := handlers[code]
		if handler == nil {
			// The attribute the policy asked to ADD is never written. Silently
			// skipping it forwards a route missing, for example, the RFC 9234
			// OTC marker the role plugin exists to stamp.
			fwdLogger().Warn("no attr mod handler for new attribute, suppressing route", "code", code)
			unapplied = modifyFailureNoHandler
			continue
		}
		newOff, panicked := safeAttrModHandler(handler, code, nil, codeOps, buf, off)
		if panicked || newOff < off || newOff > len(buf) {
			fwdLogger().Warn("attr mod handler faulted on new attribute, suppressing route",
				"code", code, "panicked", panicked)
			unapplied = modifyFailureHandlerFault
			continue // The attribute is not written; the route must not go out.
		}
		off = newOff
	}

	// Step 7: Backfill attr_len.
	newAttrLen := off - attrLenPos - 2
	if newAttrLen < 0 || newAttrLen > 65535 {
		// RFC 4271 Section 4.3: Total Path Attribute Length is a 2-octet field.
		fwdLogger().Warn("modified attribute section does not fit the 2-octet length field, suppressing route",
			"newAttrLen", newAttrLen, "max", 65535, "attrCount", attrCount)
		cleanupBuf()
		return nil, 0, modifyFailureAttrLenRange
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
				return nil, 0, modifyFailureOverflow
			}
			off += len(nlriOverride)
		}
	} else {
		nlriLen := len(payload) - attrEnd
		if nlriLen > 0 {
			if !safeCopy(buf, off, payload[attrEnd:]) {
				cleanupBuf()
				return nil, 0, modifyFailureOverflow
			}
			off += nlriLen
		}
	}

	// A modification the policy required did not land, on one of the paths that
	// keeps building. The payload is well-formed but is MISSING that change, so
	// it must not go out. Returning here, before the per-peer buffer is handed
	// to the caller, also keeps the buffer contract simple: a caller that
	// suppresses never has to return a buffer it never stored.
	if unapplied.failed() {
		cleanupBuf()
		return nil, 0, unapplied
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
		// unapplied is normally modifyFailureNone. When it is not, the payload
		// is well-formed but does NOT carry a modification the policy required,
		// so the caller must suppress rather than send it.
		// unapplied is normally modifyFailureNone. When it is not, the payload
		// is well-formed but is MISSING a modification the policy asked for.
		// The caller counts it and, per AC-18, still forwards. See
		// modifyFailure.failed for why this is reported rather than acted on.
		return buf[:off], idx, modifyFailureNone
	}

	// Sync.Pool fallback: copy result so pool buffer can be returned.
	result := make([]byte, off) // pool-fallback
	copy(result, buf[:off])

	return result, 0, modifyFailureNone
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

// groupOpsByCode groups AttrOps by attribute code into a fixed array.
// Two-pass: count first, then pre-allocate and fill.
func groupOpsByCode(ops []filterapi.AttrOp) [256][]filterapi.AttrOp {
	var counts [256]int
	for i := range ops {
		counts[ops[i].Code]++
	}
	var m [256][]filterapi.AttrOp
	for code := range counts {
		if counts[code] > 0 {
			m[code] = make([]filterapi.AttrOp, counts[code]) // pool-fallback
			counts[code] = 0
		}
	}
	for i := range ops {
		c := ops[i].Code
		m[c][counts[c]] = ops[i]
		counts[c]++
	}
	return m
}

// safeAttrModHandler calls an AttrModHandler with panic recovery.
// Returns the new offset on success, or the original offset on panic.
// The panicked return is load-bearing. Recovery copies the source attribute
// through, which produces a VALID offset, so the caller's offset-range check
// cannot tell a recovered panic from a successful modification. Without this
// flag the route goes out carrying the attribute the handler was meant to
// change -- an independent review found exactly that leak.
func safeAttrModHandler(handler filterapi.AttrModHandler, code uint8, src []byte, ops []filterapi.AttrOp, buf []byte, off int) (newOff int, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			fwdLogger().Error("attr mod handler panic, suppressing route",
				"code", code, "panic", r)
			panicked = true
			// On panic with source attr, copy it unchanged if buffer has room.
			// The bytes keep the walk's offset arithmetic consistent; the
			// caller discards the whole payload on panicked.
			if len(src) > 0 && off+len(src) <= len(buf) {
				copy(buf[off:], src)
				newOff = off + len(src)
			} else {
				newOff = off
			}
		}
	}()
	return handler(src, ops, buf, off), false
}
