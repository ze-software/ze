// Design: docs/architecture/mrt.md -- offline path-attribute decoding
// RFC: rfc/short/rfc6396.md -- MP_REACH_NLRI truncation in RIB entries (Section 4.3.4)
// Related: bgp.go -- BGP message parsing and the raw attribute walker
// Related: format.go -- attribute string rendering for display and matching

package mrt

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
)

// BGP path attribute type codes (IANA "BGP Path Attributes" registry).
const (
	AttrOrigin          uint8 = 1
	AttrASPath          uint8 = 2
	AttrNextHop         uint8 = 3
	AttrMED             uint8 = 4
	AttrLocalPref       uint8 = 5
	AttrAtomicAggregate uint8 = 6
	AttrAggregator      uint8 = 7
	AttrCommunity       uint8 = 8
	AttrOriginatorID    uint8 = 9
	AttrClusterList     uint8 = 10
	AttrMPReachNLRI     uint8 = 14
	AttrMPUnreachNLRI   uint8 = 15
	AttrExtCommunity    uint8 = 16
	AttrAS4Path         uint8 = 17
	AttrAS4Aggregator   uint8 = 18
	AttrLargeCommunity  uint8 = 32
	AttrOTC             uint8 = 35 // RFC 9234 Only to Customer
)

var (
	errMPReachShort   = errors.New("mrt: MP_REACH_NLRI too short")
	errMPUnreachShort = errors.New("mrt: MP_UNREACH_NLRI too short")
	errNextHopLen     = errors.New("mrt: unsupported next-hop length")
)

// MPReach is a decoded MP_REACH_NLRI attribute (RFC 4760 Section 3), the full
// on-the-wire form carried in a BGP UPDATE message.
//
// RIB entries inside a TABLE_DUMP_V2 record carry an abbreviated form instead;
// decode those with ParseMPReachRIBEntry, never with this function.
type MPReach struct {
	AFI       uint16
	SAFI      uint8
	NextHop   netip.Addr
	LinkLocal netip.Addr // set when the next hop is the 32-byte RFC 2545 form
	Prefixes  []netip.Prefix
}

// MPUnreach is a decoded MP_UNREACH_NLRI attribute (RFC 4760 Section 4).
type MPUnreach struct {
	AFI      uint16
	SAFI     uint8
	Prefixes []netip.Prefix
}

// Aggregator is a decoded AGGREGATOR attribute (RFC 4271 Section 5.1.7).
type Aggregator struct {
	ASN     uint32
	Address netip.Addr
}

// ExtendedCommunity is one 8-octet extended community (RFC 4360 Section 2).
type ExtendedCommunity struct {
	Type    uint8
	Subtype uint8
	Value   [6]byte
}

// ASPathIsFourByte reports whether AS_PATH inside the given MRT record type and
// subtype uses 4-byte AS numbers.
//
// RFC 6396 fixes the width per record type; it is never inferable from the
// attribute bytes, because a 2-byte path and a 4-byte path can share a byte
// count. Callers MUST derive the width from the record, which is what this
// function is for.
//
//	TABLE_DUMP (12)          2-byte  (Section 4.2)
//	TABLE_DUMP_V2 (13)       4-byte  (Section 4.3.4)
//	BGP4MP_MESSAGE           2-byte  (Section 4.4.2)
//	BGP4MP_MESSAGE_AS4       4-byte  (Section 4.4.3)
//
// Types that carry no BGP AS_PATH report false (the pre-V2 legacy width).
func ASPathIsFourByte(mrtType, subtype uint16) bool {
	switch mrtType {
	case TypeTableDumpV2:
		return true
	case TypeBGP4MP, TypeBGP4MPET:
		return IsAS4Subtype(subtype)
	}
	return false
}

// ParseMPReachRIBEntry decodes the abbreviated MP_REACH_NLRI carried inside a
// TABLE_DUMP_V2 RIB entry and returns its next hop.
//
// RFC 6396 Section 4.3.4: "only the Next Hop Address Length and Next Hop Address
// fields are included. The Address Family Identifier, Subsequent AFI, NLRI and
// Reserved fields are omitted." The AFI/SAFI and prefix already live in the
// enclosing RIB record header, so the value is exactly:
//
//	+---------------------+------------------------+
//	| Next Hop Length (1) | Next Hop Address (var) |
//	+---------------------+------------------------+
//
// Decoding this with the full-form parser reads the length from the wrong
// offset and yields a garbage or empty next hop, which is why this is a
// separate entry point.
func ParseMPReachRIBEntry(value []byte) (netip.Addr, error) {
	if len(value) < 1 {
		return netip.Addr{}, fmt.Errorf("%w: RIB-entry form needs at least 1 octet, have %d", errMPReachShort, len(value))
	}
	nhLen := int(value[0])
	if 1+nhLen > len(value) {
		return netip.Addr{}, fmt.Errorf("%w: next-hop length %d exceeds %d remaining octets", errMPReachShort, nhLen, len(value)-1)
	}
	addr, _, err := nextHopFromBytes(value[1 : 1+nhLen])
	return addr, err
}

// ParseMPReach decodes the full MP_REACH_NLRI attribute (RFC 4760 Section 3):
// AFI(2) + SAFI(1) + Next Hop Length(1) + Next Hop(var) + Reserved(1) + NLRI(var).
//
// Use ParseMPReachRIBEntry for TABLE_DUMP_V2 RIB entries, which omit every
// field except the next hop.
//
// A damaged NLRI section returns BOTH the attribute decoded so far (AFI, SAFI,
// next hop, and the prefixes read before the damage) AND an error, mirroring
// ParsePrefixesAFI. A caller that wants to render what survived checks the
// value; a caller that wants correctness checks the error. Returning nil with
// the error would throw away good prefixes and leave a caller that counts them
// unable to tell "3 prefixes" from "3 prefixes and the rest is unreadable".
// A failure BEFORE the NLRI section (a truncated fixed header or an unusable
// next hop) yields nil, because nothing was decoded.
func ParseMPReach(value []byte) (*MPReach, error) {
	const fixedHeader = 4 // AFI(2) + SAFI(1) + Next Hop Length(1)
	if len(value) < fixedHeader {
		return nil, fmt.Errorf("%w: need at least %d octets for AFI/SAFI/next-hop-length, have %d", errMPReachShort, fixedHeader, len(value))
	}

	mp := &MPReach{
		AFI:  binary.BigEndian.Uint16(value[0:2]),
		SAFI: value[2],
	}

	nhLen := int(value[3])
	if fixedHeader+nhLen > len(value) {
		return nil, fmt.Errorf("%w: next-hop length %d exceeds %d remaining octets", errMPReachShort, nhLen, len(value)-fixedHeader)
	}
	nextHop, linkLocal, err := nextHopFromBytes(value[fixedHeader : fixedHeader+nhLen])
	if err != nil {
		return nil, err
	}
	mp.NextHop = nextHop
	mp.LinkLocal = linkLocal

	// Reserved(1) then NLRI. A value that stops at the reserved octet carries no
	// NLRI, which is legal.
	off := fixedHeader + nhLen
	if off >= len(value) {
		return mp, nil
	}
	off++ // Reserved
	if off < len(value) {
		// Salvage: keep the prefixes decoded before the damage and report it.
		prefixes, perr := ParsePrefixesAFI(value[off:], mp.AFI, false)
		mp.Prefixes = prefixes
		if perr != nil {
			return mp, fmt.Errorf("MP_REACH_NLRI (afi %d, safi %d): %w", mp.AFI, mp.SAFI, perr)
		}
	}
	return mp, nil
}

// ParseMPUnreach decodes the MP_UNREACH_NLRI attribute (RFC 4760 Section 4):
// AFI(2) + SAFI(1) + Withdrawn Routes(var).
//
// Like ParseMPReach, a damaged withdrawn-routes section returns both the
// prefixes decoded so far and an error.
func ParseMPUnreach(value []byte) (*MPUnreach, error) {
	const fixedHeader = 3 // AFI(2) + SAFI(1)
	if len(value) < fixedHeader {
		return nil, fmt.Errorf("%w: need at least %d octets for AFI/SAFI, have %d", errMPUnreachShort, fixedHeader, len(value))
	}
	mp := &MPUnreach{
		AFI:  binary.BigEndian.Uint16(value[0:2]),
		SAFI: value[2],
	}
	if len(value) > fixedHeader {
		// Salvage: keep the prefixes decoded before the damage and report it.
		prefixes, perr := ParsePrefixesAFI(value[fixedHeader:], mp.AFI, false)
		mp.Prefixes = prefixes
		if perr != nil {
			return mp, fmt.Errorf("MP_UNREACH_NLRI (afi %d, safi %d): %w", mp.AFI, mp.SAFI, perr)
		}
	}
	return mp, nil
}

// nextHopFromBytes converts a BGP next-hop field into an address, returning the
// link-local half when the 32-byte RFC 2545 Section 3 form is used.
func nextHopFromBytes(nh []byte) (addr, linkLocal netip.Addr, err error) {
	switch len(nh) {
	case 4:
		return netip.AddrFrom4([4]byte(nh)), netip.Addr{}, nil
	case 16:
		return netip.AddrFrom16([16]byte(nh)), netip.Addr{}, nil
	case 32:
		// RFC 2545 Section 3: global address followed by link-local address.
		return netip.AddrFrom16([16]byte(nh[:16])), netip.AddrFrom16([16]byte(nh[16:])), nil
	}
	return netip.Addr{}, netip.Addr{}, fmt.Errorf("%w: %d octets, want 4, 16 or 32", errNextHopLen, len(nh))
}

// ExtractNextHopRIB returns the next hop for a TABLE_DUMP_V2 RIB entry.
//
// It prefers the plain NEXT_HOP attribute (type 3), which is how IPv4 RIB
// entries carry the next hop, and otherwise decodes the abbreviated
// MP_REACH_NLRI defined by RFC 6396 Section 4.3.4.
//
// Use ExtractNextHop instead for attributes taken from a BGP UPDATE message.
func ExtractNextHopRIB(attrs []PathAttribute) netip.Addr {
	if nh := FindAttribute(attrs, AttrNextHop); nh != nil && len(nh.Value) == 4 {
		return netip.AddrFrom4([4]byte(nh.Value))
	}
	if mp := FindAttribute(attrs, AttrMPReachNLRI); mp != nil {
		if addr, err := ParseMPReachRIBEntry(mp.Value); err == nil {
			return addr
		}
	}
	return netip.Addr{}
}

// ExtractAggregator returns the AGGREGATOR attribute (RFC 4271 Section 5.1.7),
// falling back to AS4_AGGREGATOR (RFC 6793 Section 3) when only that is present.
//
// The AS width is taken from the attribute length, which is unambiguous here:
// RFC 6793 fixes AGGREGATOR at 6 octets for a 2-byte AS and 8 for a 4-byte AS.
// (AS_PATH has no such length tell, which is why ParseASPath takes the width as
// a parameter instead.)
func ExtractAggregator(attrs []PathAttribute) (Aggregator, bool) {
	a := FindAttribute(attrs, AttrAggregator)
	if a == nil {
		a = FindAttribute(attrs, AttrAS4Aggregator)
	}
	if a == nil {
		return Aggregator{}, false
	}
	switch len(a.Value) {
	case 6:
		return Aggregator{
			ASN:     uint32(binary.BigEndian.Uint16(a.Value[0:2])),
			Address: netip.AddrFrom4([4]byte(a.Value[2:6])),
		}, true
	case 8:
		return Aggregator{
			ASN:     binary.BigEndian.Uint32(a.Value[0:4]),
			Address: netip.AddrFrom4([4]byte(a.Value[4:8])),
		}, true
	}
	return Aggregator{}, false
}

// HasAtomicAggregate reports whether ATOMIC_AGGREGATE (RFC 4271 Section 5.1.6)
// is present. The attribute is a flag: its value is zero-length by definition.
func HasAtomicAggregate(attrs []PathAttribute) bool {
	return FindAttribute(attrs, AttrAtomicAggregate) != nil
}

// ExtractExtendedCommunities returns the extended communities (RFC 4360
// Section 2) as 8-octet records. A value whose length is not a multiple of 8 is
// malformed and yields none.
func ExtractExtendedCommunities(attrs []PathAttribute) []ExtendedCommunity {
	a := FindAttribute(attrs, AttrExtCommunity)
	if a == nil || len(a.Value) == 0 || len(a.Value)%8 != 0 {
		return nil
	}
	ecs := make([]ExtendedCommunity, 0, len(a.Value)/8)
	for off := 0; off+8 <= len(a.Value); off += 8 {
		ec := ExtendedCommunity{Type: a.Value[off], Subtype: a.Value[off+1]}
		copy(ec.Value[:], a.Value[off+2:off+8])
		ecs = append(ecs, ec)
	}
	return ecs
}

// ExtractLargeCommunities returns large communities (RFC 8092) as
// (global, local1, local2) triples.
func ExtractLargeCommunities(attrs []PathAttribute) [][3]uint32 {
	a := FindAttribute(attrs, AttrLargeCommunity)
	if a == nil || len(a.Value) == 0 || len(a.Value)%12 != 0 {
		return nil
	}
	out := make([][3]uint32, 0, len(a.Value)/12)
	for off := 0; off+12 <= len(a.Value); off += 12 {
		out = append(out, [3]uint32{
			binary.BigEndian.Uint32(a.Value[off:]),
			binary.BigEndian.Uint32(a.Value[off+4:]),
			binary.BigEndian.Uint32(a.Value[off+8:]),
		})
	}
	return out
}
