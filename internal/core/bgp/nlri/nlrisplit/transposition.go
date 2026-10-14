// Design: docs/architecture/rib/unified-locrib.md -- per-family NLRI split
// RFC: rfc/short/rfc9252.md -- Section 4 transposition scheme; Sections 5.1 and 5.2, the VPN label field
// Related: cidr.go -- splitVPN, which frames the NLRI this reads from
// Related: labeled.go -- ExtractLabels, the RFC 8277 read this deliberately does not reuse

package nlrisplit

import "github.com/ze-software/ze/internal/core/family"

// vpnLabelStackOffset is the distance from the start of a VPN NLRI to its
// label stack: one octet of length, per RFC 4364 Section 4.3.4.
const vpnLabelStackOffset = 1

// Label field widths, in bits, of the families the Transposition Scheme
// applies to.
const (
	vpnTranspositionLabelWidth  = 20
	evpnTranspositionLabelWidth = 24
)

// TranspositionLabelWidth returns the width in bits of the label field that
// may carry transposed SRv6 SID bits for fam, and 0 for a family the
// Transposition Scheme does not apply to. It is the one declaration of that
// family set: TranspositionLabel and the RIB's SID validity both read it.
//
// RFC 9252 Sections 5.1 and 5.2 give the IPv4 and IPv6 VPN families an RFC
// 8277 field with "the 20-bit Label Value set to the whole or a portion of
// the Function part of the SRv6 SID", and Section 3.2.1 says "the size of the
// MPLS Label field is 20 bits in [RFC4364] and [RFC8277], and the size is 24
// bits in [RFC7432]", the EVPN field. Every other family has no label field
// for the scheme, labeled unicast included: RFC 9252 defines no SRv6 service
// over it. Section 7: "The transposition offset and length MUST be 0 when the
// Sub-Sub-TLV is advertised along with routes where the Transposition Scheme
// is not applicable".
func TranspositionLabelWidth(fam family.Family) uint8 {
	switch {
	case fam.SAFI == family.SAFIVPN && (fam.AFI == family.AFIIPv4 || fam.AFI == family.AFIIPv6):
		return vpnTranspositionLabelWidth
	case fam.SAFI == family.SAFIEVPN && fam.AFI == family.AFIL2VPN:
		return evpnTranspositionLabelWidth
	}
	return 0
}

// TranspositionLabel returns the value of the label field that carries
// transposed SRv6 SID bits for one NLRI of fam, right-aligned in that
// field, and reports whether ze can read it.
//
// RFC 9252 Section 4 lets a sender move part of the SRv6 Service SID out of
// the Prefix-SID attribute and into an existing label field, so a receiver
// that reads only the attribute holds a SID with zeros where those bits
// belong. Sections 5.1 and 5.2 put that field in the IPv4-VPN and IPv6-VPN
// NLRI, encoded as RFC 8277 does, "with the 20-bit Label Value set to the
// whole or a portion of the Function part of the SRv6 SID". The 20-bit value
// is what this returns, so it is already right-aligned in its field.
//
// ok is false for every other family, EVPN included. EVPN transposes too
// (Section 6), but its label field is not one field: Route Types 1, 2 and 5
// each hold it at a different NLRI offset, Route Type 3 carries it in the
// PMSI Tunnel Attribute rather than the NLRI at all (Section 6.3), and Route
// Type 1 per-ES carries Argument bits in the ESI Label extended community
// (Section 6.1.1). Route Type 2 holds two label fields bound to different
// Service TLVs (Section 6.2), so choosing between them needs the TLV the SID
// came from, which pool.SRv6SIDResult does not record. A caller that gets
// ok=false holds no label and MUST NOT transpose: the partial SID in the
// attribute is not the SID the peer signaled.
//
// nlri is one NLRI as returned by Split for fam. Under ADD-PATH (RFC 7911)
// its first four octets are the path identifier and are skipped here.
func TranspositionLabel(fam family.Family, nlri []byte, addPath bool) (uint32, bool) {
	if TranspositionLabelWidth(fam) != vpnTranspositionLabelWidth {
		return 0, false
	}
	off := vpnLabelStackOffset
	if addPath {
		off += 4
	}
	// The transposed bits ride the first label of the stack, the one bound to
	// the service the SRv6 SID describes.
	if off+3 > len(nlri) {
		return 0, false
	}
	// RFC 8277 Section 2.1: the 20-bit Label Value occupies the high-order
	// bits of the 3-octet field; the low 4 hold TC and the S bit.
	return uint32(nlri[off])<<12 | uint32(nlri[off+1])<<4 | uint32(nlri[off+2])>>4, true
}
