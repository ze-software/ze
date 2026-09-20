// Design: docs/architecture/wire/nlri.md — NLRI encoding and decoding
// RFC: rfc/short/rfc4271.md — NLRI base types (Section 4.3)
// RFC: rfc/short/rfc4760.md — multiprotocol NLRI family dispatch
//
// Package nlri implements BGP Network Layer Reachability Information types.
//
// RFC 4271 Section 4.3 defines the base NLRI encoding for IPv4 prefixes as
// a 2-tuple of <length, prefix> where length is the prefix length in bits
// and prefix contains the minimum number of octets to represent the prefix.
//
// RFC 4760 extends this to support multiple address families via the
// MP_REACH_NLRI (Type Code 14) and MP_UNREACH_NLRI (Type Code 15) path
// attributes. Section 5 of RFC 4760 defines the same <length, prefix>
// encoding for multiprotocol NLRI.
//
// Supports all major NLRI families:
//   - INET (IPv4/IPv6 unicast/multicast) - RFC 4271, RFC 4760
//   - IPVPN (VPNv4/VPNv6) - RFC 4364, RFC 4659
//   - EVPN (all 5 route types) - RFC 7432
//   - FlowSpec - RFC 8955
//   - BGP-LS - RFC 9552
//   - And more
package nlri

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/core/family"
)

// NLRI represents Network Layer Reachability Information.
//
// RFC 4271 Section 4.3 defines NLRI for IPv4 unicast as a variable-length
// field containing one or more 2-tuples of the form <length, prefix>:
//
//	+---------------------------+
//	|   Length (1 octet)        |  <- prefix length in bits
//	+---------------------------+
//	|   Prefix (variable)       |  <- minimum octets to contain prefix
//	+---------------------------+
//
// RFC 4760 Section 5 extends this encoding to all address families.
//
// RFC 7911 Section 3 extends the encoding with an optional Path Identifier
// for ADD-PATH support:
//
//	+--------------------------------+
//	| Path Identifier (4 octets)     |  <- only when ADD-PATH negotiated
//	+--------------------------------+
//	| Length (1 octet)               |
//	+--------------------------------+
//	| Prefix (variable)              |
//	+--------------------------------+
//
// This is the core interface for all NLRI types (prefixes, VPN routes,
// EVPN routes, FlowSpec rules, etc.).
//
// Phase 3 simplification: Len()/Bytes()/WriteTo() return payload only (no path ID).
// Use WriteNLRI() for ADD-PATH aware encoding.
type NLRI interface {
	// Family returns the AFI/SAFI for this NLRI.
	// RFC 4760 Section 3: <AFI, SAFI> identifies NLRI semantics.
	Family() family.Family

	// Bytes returns the wire-format encoding of this NLRI (payload only).
	// RFC 4271 Section 4.3: Encoded as <length, prefix> tuples.
	// The returned slice may be shared; do not modify.
	//
	// Note: Path ID is NOT included. Use WriteNLRI() for ADD-PATH encoding.
	Bytes() []byte

	// Len returns the payload length in bytes (no path ID).
	// Use LenWithContext() for ADD-PATH aware length calculation.
	Len() int

	// String returns a human-readable representation.
	String() string

	// PathID returns the ADD-PATH path identifier (0 if not present).
	// RFC 7911 Section 3: Path Identifier is a 4-octet field.
	PathID() uint32

	// WriteTo writes the NLRI payload (without path ID) into buf at offset.
	// Returns number of bytes written.
	//
	// Note: Path ID is NOT written. Use WriteNLRI() for ADD-PATH encoding.
	WriteTo(buf []byte, off int) int

	// SupportsAddPath returns true if this NLRI type supports ADD-PATH encoding.
	// RFC 7911 Section 3: ADD-PATH capability allows multiple paths per prefix.
	// Some NLRI types (FlowSpec, BGPLS, etc.) don't support ADD-PATH per their RFCs.
	SupportsAddPath() bool
}

// JSONAppender is an optional interface implemented by NLRI types that can
// emit their own JSON representation directly into a caller-provided []byte,
// bypassing the wire-encode / hex / re-parse / map-marshal round-trip used by
// the RPC decoder path.
//
// Hot-path formatters (format/text_json.go) probe for this interface and use
// it when available. Types that do not implement it fall back to the registry
// decoder path (required for external plugins that live over RPC).
type JSONAppender interface {
	AppendJSON(buf []byte) []byte
}

// AddPathAware is an optional interface implemented by NLRI types that hold raw
// wire bytes whose layout depends on the ADD-PATH negotiation.
//
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
//
// A decoder handed those octets cannot tell a Path Identifier from the first
// octets of the NLRI itself, so the flag has to travel with the data. The
// formatters probe for this interface to learn the layout of the bytes they
// hold, rather than switching on a concrete type: a new opaque NLRI carrier
// answers the question by implementing the method, and no formatter is edited.
type AddPathAware interface {
	HasAddPath() bool
}

// SplitPathID removes the 4-octet Path Identifier from the head of one NLRI and
// returns it with the octets that follow. When addPath is false the data is
// returned unchanged and the identifier is zero.
//
// RFC 7911 Section 3: "the NLRI encoding MUST be extended by prepending the
// Path Identifier field, which is of four octets."
//
// A section that is too short to hold the identifier the negotiation promised is
// malformed, so this returns an error rather than a zero identifier: a caller
// cannot tell a real path identifier of zero from four octets that were never
// there.
func SplitPathID(data []byte, addPath bool) (pathID uint32, rest []byte, err error) {
	if !addPath {
		return 0, data, nil
	}
	if len(data) < 4 {
		return 0, nil, ErrPathIDTruncated
	}
	return binary.BigEndian.Uint32(data[:4]), data[4:], nil
}

// LenWithContext returns the wire-format length adjusted for ADD-PATH.
//
// RFC 7911 Section 3 - Extended NLRI Encodings:
// When ADD-PATH is negotiated, each NLRI is prefixed with a 4-byte Path
// Identifier. This function calculates the total wire length:
//   - If addPath=false: returns Len() (payload only)
//   - If addPath=true: returns Len() + 4 (path ID + payload)
//
// Note: Some NLRI types (FlowSpec, BGPLS, etc.) don't support ADD-PATH
// per their respective RFCs and always return Len() regardless of addPath.
func LenWithContext(n NLRI, addPath bool) int {
	baseLen := n.Len()

	// Types that don't support ADD-PATH
	if !n.SupportsAddPath() {
		return baseLen
	}

	// ADD-PATH: add 4 bytes for path identifier
	if addPath {
		return baseLen + 4
	}

	return baseLen
}

// WriteNLRI writes NLRI with ADD-PATH handling into buf at offset.
//
// RFC 7911 Section 3: ADD-PATH prepends 4-byte path identifier:
//   - If addPath=true AND NLRI type supports ADD-PATH: writes path ID + payload
//   - Otherwise: writes payload only
//
// This is the recommended way to encode NLRIs for wire format.
func WriteNLRI(n NLRI, buf []byte, off int, addPath bool) int {
	pos := off

	// Handle ADD-PATH path identifier
	// RFC 7911: Path ID only included when addPath=true AND NLRI supports it
	if addPath && n.SupportsAddPath() {
		binary.BigEndian.PutUint32(buf[pos:], n.PathID())
		pos += 4
	}

	// Write payload (without path ID)
	pos += n.WriteTo(buf, pos)

	return pos - off
}
