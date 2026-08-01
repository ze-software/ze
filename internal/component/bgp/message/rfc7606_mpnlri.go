// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc7606.md — Section 5.4 typed NLRI; rfc/short/rfc4760.md — MP attributes
// Overview: rfc7606.go — the UPDATE validation walk that records these locations

package message

import "encoding/binary"

// MPNLRILocation reports that an UPDATE carried an MP_REACH_NLRI or
// MP_UNREACH_NLRI attribute, and which family its NLRI belongs to.
//
// Recorded by RFC 7606 validation walk so Section 5.4 typed-NLRI check
// can decide whether it has anything to do without walking the attribute section
// a second time. Most UPDATEs carry no MP attribute, and most that do belong to a
// family with no Section 5.4 ruling, so this answers both questions before any
// further work.
//
// It carries no byte offsets on purpose. Section 3.g keep-first strip can
// rebuild the attribute section after this walk, which would shift them; the
// family is invariant under that rebuild, because Section 3.g session-resets a
// duplicate MP attribute rather than stripping one.
type MPNLRILocation struct {
	Present bool
	AFI     uint16
	SAFI    uint8
}

// MPNLRIStart returns the offset within an MP attribute's VALUE at which its
// NLRI bytes begin, and whether the value is long enough to hold that header.
//
// RFC 4760 Section 3: MP_REACH_NLRI is AFI(2) SAFI(1) NextHopLen(1)
// NextHop(NextHopLen) Reserved(1) then NLRI. Section 4: MP_UNREACH_NLRI is
// AFI(2) SAFI(1) then withdrawn NLRI.
//
// One implementation, so Section 5.3 syntax walk and Section 5.4
// typed-NLRI rewrite cannot drift on where the NLRI starts.
func MPNLRIStart(code uint8, attrData []byte) (int, bool) {
	if code == attrCodeMPReachNLRI {
		if len(attrData) < 4 {
			return 0, false
		}
		start := 4 + int(attrData[3]) + 1
		if start > len(attrData) {
			return 0, false
		}
		return start, true
	}
	if len(attrData) < 3 {
		return 0, false
	}
	return 3, true
}

// locateMPNLRI records the family of an MP attribute's NLRI.
//
// Returns a zero location (Present false) when the value is too short to hold
// its own header. That is not silence: such an attribute is malformed, and the
// Section 5.3 checks on the same walk return session-reset for it, so no caller
// acts on the absent location as though the NLRI were merely empty.
func locateMPNLRI(code uint8, attrData []byte) MPNLRILocation {
	if _, ok := MPNLRIStart(code, attrData); !ok {
		return MPNLRILocation{}
	}
	return MPNLRILocation{
		Present: true,
		AFI:     binary.BigEndian.Uint16(attrData[0:2]),
		SAFI:    attrData[2],
	}
}
