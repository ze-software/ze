// Design: docs/architecture/wire/nlri.md -- BGP-MUP NLRI
// RFC: rfc/short/draft-ietf-bess-mup-safi.md -- Section 3.1.3.1, Source Address Length
// Related: types.go -- parseBodyT1ST, which reads the -02 encoding this rewrites
// Related: register.go -- registers the form with nlritype.RegisterCanonical

package mup

import "encoding/binary"

// mupRouteTypeOctets is the 1-octet Architecture Type plus the 2-octet Route Type
// that precede the Length octet of every BGP-MUP NLRI (Section 3.1).
const mupRouteTypeOctets = 3

// t1stFixedOctets is TEID (4), QFI (1) and Endpoint Address Length (1), the
// fixed fields of a Type 1 ST route between its prefix and its endpoint address.
const t1stFixedOctets = 6

// canonicalT1STGrow answers 1 for a Type 1 ST route in the -02 encoding, which
// ends at its Endpoint Address with no Source Address Length octet, and 0 for
// every other route. It is nlritype.Canonical.Grow for both MUP families.
//
// nlritype.RetainUpdate calls it only for a route the recognizer kept, so the
// route parsed (parseBodyT1ST); every offset is still checked, because the
// octets came from a peer.
func canonicalT1STGrow(nlriBytes []byte, addPath bool) int {
	if addPath {
		if len(nlriBytes) < 4 {
			return 0
		}
		nlriBytes = nlriBytes[4:]
	}
	if t1stSourceOctetAbsent(nlriBytes) {
		return 1
	}
	return 0
}

// t1stSourceOctetAbsent reports whether one MUP NLRI, with no path identifier, is
// a 3gpp-5G Type 1 ST route whose Route Type specific field ends exactly at the end
// of its Endpoint Address: the draft-mpmz-bess-mup-safi-02 encoding, read as
// Source Address Length 0 (parseBodyT1ST).
func t1stSourceOctetAbsent(nlriBytes []byte) bool {
	if len(nlriBytes) < mupHeaderLen {
		return false
	}
	if MUPArchType(nlriBytes[0]) != MUPArch3GPP5G {
		return false
	}
	if MUPRouteType(binary.BigEndian.Uint16(nlriBytes[1:mupRouteTypeOctets])) != MUPT1ST {
		return false
	}
	body := nlriBytes[mupHeaderLen:]
	if len(body) != int(nlriBytes[mupRouteTypeOctets]) {
		return false
	}

	// RD (8) | Prefix Length (1) | Prefix | TEID (4) | QFI (1) |
	// Endpoint Address Length (1) | Endpoint Address
	off := mupRDLen
	if len(body) < off+1 {
		return false
	}
	off += 1 + (int(body[off])+7)/8 + t1stFixedOctets
	if len(body) < off {
		return false
	}
	endpointOctets, ok := addressBitsOctets(body[off-1])
	if !ok {
		return false
	}
	return len(body) == off+endpointOctets
}

// writeCanonicalT1ST writes a -02 Type 1 ST route at buf[off:] in the current
// encoding: the Length one larger and a Source Address Length of 0 after the
// Endpoint Address. It returns the octets written, len(nlriBytes)+1. It is
// nlritype.Canonical.WriteTo for both MUP families, called only for a route
// canonicalT1STGrow answered 1 for.
//
// draft-ietf-bess-mup-safi Section 3.1.3.1: "The Source Address Length has a
// fixed length of 1 octet.  Source Address Length is 0 bytes then the Source
// address is not carried within the NLRI." ExaBGP f9bfb853b re-sends such a
// route the same way (with_source_length, bgp/message/update/nlri/mup/t1st.py).
//
//	before: Arch (1) | Route Type (2) | Length L (1) | RD ... Endpoint Address
//	after:  Arch (1) | Route Type (2) | Length L+1 (1) | RD ... Endpoint Address | 0x00
func writeCanonicalT1ST(buf []byte, off int, nlriBytes []byte, addPath bool) int {
	n := copy(buf[off:], nlriBytes)
	lengthAt := off + mupRouteTypeOctets
	if addPath {
		lengthAt += 4
	}
	// A -02 T1ST body is at most 47 octets (RD, prefix, TEID, QFI and two IPv6
	// addresses), so the added octet never passes the 1-octet Length.
	buf[lengthAt]++
	buf[off+n] = 0
	return n + 1
}
