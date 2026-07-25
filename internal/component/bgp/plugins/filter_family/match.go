// Design: docs/architecture/wire/nlri.md -- family extraction + MP attribute surgery
// Related: config.go -- family-filter instance parsing
// RFC: rfc/short/rfc4271.md -- UPDATE message body layout (Section 4.3)
// RFC: rfc/short/rfc4760.md -- MP_REACH/MP_UNREACH NLRI, AFI/SAFI (Section 3, 6)

package filter_family

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/family"
)

// BGP path attribute type codes for multiprotocol NLRI (RFC 4760 §3, §4).
const (
	attrMPReachNLRI   = 14
	attrMPUnreachNLRI = 15
)

// payloadSections splits an UPDATE body into its sections (RFC 4271 §4.3):
//
//	[withdrawn-routes-length:2][withdrawn][total-path-attr-length:2][attrs][NLRI]
//
// Returns ok=false on a malformed or too-short body.
func payloadSections(payload []byte) (wdLen, attrStart, attrEnd int, ok bool) {
	if len(payload) < 4 {
		return 0, 0, 0, false
	}
	wdLen = int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenPos := 2 + wdLen
	if attrLenPos+2 > len(payload) {
		return 0, 0, 0, false
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenPos : attrLenPos+2]))
	attrStart = attrLenPos + 2
	attrEnd = attrStart + attrLen
	if attrEnd > len(payload) {
		return 0, 0, 0, false
	}
	return wdLen, attrStart, attrEnd, true
}

// familyFromPayload returns the UPDATE's address family. fromMP is true when the
// family came from an MP_REACH/MP_UNREACH attribute (RFC 4760 §6); when no MP
// attribute is present the UPDATE is legacy ipv4/unicast. ok is false on a
// malformed body (the caller then leaves the UPDATE untouched).
func familyFromPayload(payload []byte) (fam family.Family, fromMP, ok bool) {
	_, attrStart, attrEnd, valid := payloadSections(payload)
	if !valid {
		return family.Family{}, false, false
	}
	if f, found := message.ExtractMPFamily(payload[attrStart:attrEnd]); found {
		return f, true, true
	}
	return family.IPv4Unicast, false, true
}

// stripMPAttrs removes the MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15) attributes
// from an UPDATE body, preserving withdrawn routes, every other path attribute,
// and the legacy IPv4 NLRI tail. emptied is true when the result carries no
// reachability (no withdrawn routes and no legacy NLRI) — the caller then
// suppresses the whole UPDATE rather than send an empty one. ok is false on a
// malformed body.
func stripMPAttrs(payload []byte) (newPayload []byte, emptied, ok bool) {
	wdLen, attrStart, attrEnd, valid := payloadSections(payload)
	if !valid {
		return nil, false, false
	}

	kept := make([]byte, 0, attrEnd-attrStart)
	off := attrStart
	for off < attrEnd {
		if off+2 > attrEnd {
			return nil, false, false // malformed attribute header
		}
		flags := payload[off]
		code := payload[off+1]
		var hdrLen, dataLen int
		if flags&0x10 != 0 { // Extended length.
			if off+4 > attrEnd {
				return nil, false, false
			}
			dataLen = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
			hdrLen = 4
		} else {
			if off+3 > attrEnd {
				return nil, false, false
			}
			dataLen = int(payload[off+2])
			hdrLen = 3
		}
		total := hdrLen + dataLen
		if off+total > attrEnd {
			return nil, false, false
		}
		if code != attrMPReachNLRI && code != attrMPUnreachNLRI {
			kept = append(kept, payload[off:off+total]...)
		}
		off += total
	}

	keptLen := len(kept)
	if keptLen > 0xFFFF {
		return nil, false, false // cannot happen (only removed); fail-safe
	}
	nlri := payload[attrEnd:]
	var lenBuf [2]byte
	binary.BigEndian.PutUint16(lenBuf[:], uint16(keptLen)) //nolint:gosec // bounded above

	out := make([]byte, 0, 2+wdLen+2+keptLen+len(nlri))
	out = append(out, payload[:2+wdLen]...) // withdrawn-routes-length + withdrawn routes
	out = append(out, lenBuf[:]...)         // new total path attribute length
	out = append(out, kept...)
	out = append(out, nlri...)

	emptied = wdLen == 0 && len(nlri) == 0
	return out, emptied, true
}
