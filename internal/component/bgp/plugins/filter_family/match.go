// Design: docs/architecture/wire/nlri.md -- family extraction + whether a family is all an UPDATE carries
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

// carriesOnlyFamily reports whether fam is all the reachability an UPDATE
// body carries: no withdrawn routes, no legacy IPv4 NLRI, and every
// MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15) attribute of fam. Stripping fam
// then empties the UPDATE, and the caller suppresses it whole rather than
// have an UPDATE with no NLRI sent (RFC 4760 Section 2: the family leaves with
// its MP attributes). ok is false on a malformed body.
func carriesOnlyFamily(payload []byte, fam family.Family) (only, ok bool) {
	wdLen, attrStart, attrEnd, valid := payloadSections(payload)
	if !valid {
		return false, false
	}
	only = wdLen == 0 && attrEnd == len(payload)
	// The walk is bounded by the attribute section the length field declared.
	off := attrStart
	for off < attrEnd {
		if off+2 > attrEnd {
			return false, false // malformed attribute header
		}
		flags := payload[off]
		code := payload[off+1]
		var hdrLen, dataLen int
		if flags&0x10 == 0 {
			if off+3 > attrEnd {
				return false, false
			}
			dataLen = int(payload[off+2])
			hdrLen = 3
		} else { // Extended length.
			if off+4 > attrEnd {
				return false, false
			}
			dataLen = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
			hdrLen = 4
		}
		if off+hdrLen+dataLen > attrEnd {
			return false, false
		}
		if code == attrMPReachNLRI || code == attrMPUnreachNLRI {
			// RFC 4760 Section 3: the attribute opens with its AFI (2 octets)
			// and SAFI (1 octet).
			if dataLen < 3 {
				return false, false
			}
			value := payload[off+hdrLen:]
			afi := family.AFI(binary.BigEndian.Uint16(value[0:2]))
			safi := family.SAFI(value[2])
			if (family.Family{AFI: afi, SAFI: safi}) != fam {
				only = false
			}
		}
		off += hdrLen + dataLen
	}
	return only, true
}
