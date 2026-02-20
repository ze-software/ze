// Design: docs/architecture/wire/nlri.md — NLRI encoding and decoding
//
// Package nlri implements BGP Network Layer Reachability Information encoding.
//
// This file contains base types for NLRI struct embedding.
package nlri

import (
	"net/netip"
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
	family Family
	prefix netip.Prefix
	pathID uint32 // RFC 7911: 0 means no path ID
}

// Family returns the AFI/SAFI for this NLRI.
func (p *PrefixNLRI) Family() Family {
	return p.family
}

// Prefix returns the IP prefix.
func (p *PrefixNLRI) Prefix() netip.Prefix {
	return p.prefix
}

// PathID returns the ADD-PATH path identifier (0 if none).
func (p *PrefixNLRI) PathID() uint32 {
	return p.pathID
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
// ALLOCATES - use only in Bytes(), not WriteTo().
func (r *RDNLRIBase) buildData() []byte {
	if hasRD(r.rd) {
		return append(r.rd.Bytes(), r.data...)
	}
	// Return copy to avoid aliasing original slice
	result := make([]byte, len(r.data))
	copy(result, r.data)
	return result
}

// hasRD returns true if the RD is non-zero.
func hasRD(rd RouteDistinguisher) bool {
	return rd.Type != 0 || rd.Value != [6]byte{}
}
