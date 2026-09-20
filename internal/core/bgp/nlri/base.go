// Design: docs/architecture/wire/nlri.md — NLRI encoding and decoding
// RFC: rfc/short/rfc4271.md — IPv4 unicast NLRI (Section 4.3)
// RFC: rfc/short/rfc4760.md — multiprotocol NLRI extensions
//
// Package nlri implements BGP Network Layer Reachability Information encoding.
//
// This file contains base types for NLRI struct embedding.
package nlri

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
)

// PrefixNLRI provides common fields for prefix-based NLRI types.
//
// Embedded by INET and LabeledUnicast to share:
//   - family: AFI/SAFI address family
//   - prefix: IP prefix (IPv4 or IPv6)
//   - pathID: RFC 7911 ADD-PATH identifier (0 if none)
//
// Note: IPVPN has different field order (RD before prefix) so stays separate.
type PrefixNLRI struct {
	fam     family.Family
	prefix  netip.Prefix
	pathID  uint32 // RFC 7911: the Path Identifier, which has no absent value
	addPath bool   // RFC 7911: true when the wire carried a Path Identifier
}

// Family returns the AFI/SAFI for this NLRI.
func (p *PrefixNLRI) Family() family.Family {
	return p.fam
}

// Prefix returns the IP prefix.
func (p *PrefixNLRI) Prefix() netip.Prefix {
	return p.prefix
}

// PathID returns the ADD-PATH path identifier. Zero is an identifier like any
// other, so a caller that needs to know whether one was carried asks
// HasAddPath rather than comparing this value against zero.
func (p *PrefixNLRI) PathID() uint32 {
	return p.pathID
}

// HasAddPath reports whether the wire this NLRI was parsed from carried a Path
// Identifier. It implements AddPathAware.
//
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
//
// The field has no reserved or absent value, so an identifier of zero is a real
// identifier and the prefix alone cannot say whether one arrived. The parser is
// the only place that knows, because the negotiation told it how to read the
// octets, so the parser records the answer here.
//
// A locally built NLRI reports false. Nothing has encoded it yet, and the
// encoder takes the layout from the session's EncodingContext through
// WriteNLRI rather than from the NLRI itself.
func (p *PrefixNLRI) HasAddPath() bool {
	return p.addPath
}

// SupportsAddPath returns true - prefix NLRIs support ADD-PATH per RFC 7911.
func (p *PrefixNLRI) SupportsAddPath() bool {
	return true
}

// RDNLRIBase provides common fields for RD-based NLRI types.
//
// Shared by VPN, MVPN, and MUP plugin types:
//   - rd: Route Distinguisher (8 bytes, RFC 4364)
//   - data: Route-type specific data after RD
type RDNLRIBase struct {
	rd   RouteDistinguisher
	data []byte
}

// RD returns the Route Distinguisher per RFC 4364 Section 4.1.
func (r *RDNLRIBase) RD() RouteDistinguisher {
	return r.rd
}

// buildData returns rd+data or a copy of data.
// ALLOCATES - use only in Bytes(), not WriteTo(). Hot-path callers write
// directly into a pool buffer; this function backs format/test/JSON-fallback
// callers that need a standalone result slice.
func (r *RDNLRIBase) buildData() []byte {
	if hasRD(r.rd) {
		// RFC 4364 §4.2: RD is always 8 bytes. Pre-size and write in place
		// instead of append(rd.Bytes(), data...) — one alloc, no hidden copies.
		result := make([]byte, 8+len(r.data)) // pool-fallback: result owned by caller
		r.rd.WriteTo(result, 0)
		copy(result[8:], r.data)
		return result
	}
	// Return copy to avoid aliasing original slice.
	result := make([]byte, len(r.data)) // pool-fallback: result owned by caller
	copy(result, r.data)
	return result
}

// hasRD returns true if the RD is non-zero.
func hasRD(rd RouteDistinguisher) bool {
	return rd.Type != 0 || rd.Value != [6]byte{}
}
