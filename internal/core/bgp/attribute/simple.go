// Design: docs/architecture/wire/attributes.md — path attribute encoding
// RFC: rfc/short/rfc4271.md — NEXT_HOP, MED, LOCAL_PREF, ATOMIC_AGGREGATE

package attribute

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// writeIPv4AddressField writes addr into the four-octet IP address field at off,
// and touches no other octet.
//
// Three attributes carry an IP address field the RFC fixes at four octets, and
// none of the three can hold more:
//
// RFC 4271 Section 5.1.7: AGGREGATOR carries "the last AS number that formed the
// aggregate route ... followed by the IP address of the BGP speaker that formed
// the aggregate route", and that address SHOULD be the speaker's BGP Identifier,
// a four-octet value.
//
// RFC 6793 Section 6: "The AS4_AGGREGATOR attribute in an UPDATE message SHALL be
// considered malformed if the attribute length is not 8." Four of those eight
// octets are the AS number, so exactly four remain for the address.
//
// RFC 4456 Section 8: ORIGINATOR_ID "is 4 bytes long".
//
// The field width decides the write, because the width is what the RFC fixes.
// Copying netip.Addr.AsSlice instead answered with the length of the VALUE, which
// is 0, 4 or 16: an IPv6 address wrote twelve octets past the region the size
// query reserved, into the octets the next attribute owns, and the caller could
// not detect it because the write still returned the promised count.
//
// An address with no IPv4 form fills the field with zeros. WriteTo has no error
// channel and none may be added (ai/rules/performance.md fixes the buffer-first
// signature), so a deterministic wire-legal value is what remains, and stale
// pooled octets are the alternative. No producer in the tree can build one of the
// three attributes with such an address; the fallback exists so that the next one
// that tries is bounded rather than corrupting its neighbor.
func writeIPv4AddressField(buf []byte, off int, addr netip.Addr) {
	field := buf[off : off+4]
	if addr.Is4() || addr.Is4In6() {
		// As4 panics for any other form, which is what the guard covers.
		v4 := addr.As4()
		copy(field, v4[:])
		return
	}
	clear(field)
}

// NextHop represents the NEXT_HOP attribute.
//
// RFC 4271 Section 5.1.3: NEXT_HOP
//   - Well-known mandatory attribute (Type Code 3)
//   - Defines the IP address of the router that SHOULD be used as the next hop
//   - Contains a 4-octet IPv4 address
//   - A BGP speaker MUST be able to support disabling third-party NEXT_HOP
//   - A route SHALL NOT be advertised using the peer's address as NEXT_HOP
//   - A BGP speaker SHALL NOT install a route with itself as the next hop
//
// Note: IPv6 next-hop addresses are carried in MP_REACH_NLRI (RFC 4760),
// not in this attribute. This implementation accepts both for flexibility.
type NextHop struct {
	Addr netip.Addr
}

func (n *NextHop) Code() AttributeCode   { return AttrNextHop }
func (n *NextHop) Flags() AttributeFlags { return FlagTransitive }

// Len returns the octet count WriteTo puts on the wire.
//
// RFC 4271 Section 5.1.3 gives NEXT_HOP a four-octet IPv4 address. Ze also emits
// the sixteen-octet form, because an IPv6 next hop reaches this attribute through
// RFC 4760 compatibility, so the count is a property of the VALUE rather than of a
// fixed field width.
//
// It is therefore measured the way WriteTo writes: nothing for an address with no
// wire form, four for an IPv4 address, sixteen otherwise. Those are the three
// lengths of netip.Addr.AsSlice, which is what WriteTo copies. A family test
// answered differently for the zero Addr -- it is not Is6, so the count was four
// while the write emitted none -- and a size query and a write that disagree
// desynchronize the attribute block for everything after it, exactly as
// (*MPReachNLRI).nextHopOctets records for MP_REACH.
//
// Branching costs no allocation. len(Addr.AsSlice()) is the same number and
// materializes a slice, on the hottest encode path in the daemon
// (ai/rules/performance.md).
//
// Zero is a length no NEXT_HOP wire form has: ValidateNextHops refuses that
// address rather than letting the plan encode an empty value.
func (n *NextHop) Len() int {
	if !n.Addr.IsValid() {
		return 0
	}
	if n.Addr.Is4() {
		return 4
	}
	return 16
}

// ValidateNextHops reports whether this NEXT_HOP has a wire form, and is the
// refusal half of the rule Len states.
//
// RFC 4271 Section 5.1.3 defines NEXT_HOP as "the IP address of the router that
// SHOULD be used as the next hop to the destinations listed in the ... UPDATE
// message". The zero netip.Addr names no such router, and the attribute has no
// zero-length form: a receiver reads an UPDATE without a usable NEXT_HOP as a
// Missing Well-known Attribute (RFC 4271 Section 5.1.3, RFC 7606 Section 3(d)).
//
// Agreement between Len and WriteTo is not encodability. Both answer zero for this
// address, so the announce plan's count check passes it, which is why the refusal
// has to be stated separately.
//
// The name and the signature are the interface announceAttrs.add
// (component/bgp/reactor/announce_build.go) already asks MPReachNLRI through. This
// attribute joins that assertion; no call site changes.
func (n *NextHop) ValidateNextHops() error {
	if n.Addr.IsValid() {
		return nil
	}
	return fmt.Errorf("%w: NEXT_HOP (RFC 4271 Section 5.1.3)", ErrUnencodableNextHop)
}

// WriteTo writes the NEXT_HOP value into buf at offset.
func (n *NextHop) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], n.Addr.AsSlice())
}

// WriteToWithContext writes the NEXT_HOP value - context-independent.
func (n *NextHop) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return n.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (n *NextHop) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := n.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return n.WriteTo(buf, off), nil
}

// ParseNextHop parses a NEXT_HOP attribute.
// RFC 4271 Section 5.1.3 specifies 4-octet length for IPv4.
// 16-octet length is accepted for IPv6 compatibility (RFC 4760).
func ParseNextHop(data []byte) (*NextHop, error) {
	if len(data) != 4 && len(data) != 16 {
		return nil, ErrInvalidLength
	}
	addr, ok := netip.AddrFromSlice(data)
	if !ok {
		return nil, ErrMalformedValue
	}
	return &NextHop{Addr: addr}, nil
}

// MED represents the MULTI_EXIT_DISC attribute.
//
// RFC 4271 Section 5.1.4: MULTI_EXIT_DISC
//   - Optional non-transitive attribute (Type Code 4)
//   - Used on external (inter-AS) links to discriminate among multiple
//     exit or entry points to the same neighboring AS
//   - Value is a 4-octet unsigned integer (metric)
//   - Lower metric SHOULD be preferred (all other factors being equal)
//   - MAY be propagated over IBGP within the same AS
//   - MUST NOT be propagated to other neighboring ASes
//   - Implementation MUST support removal of this attribute from a route
type MED uint32

func (m MED) Code() AttributeCode   { return AttrMED }
func (m MED) Flags() AttributeFlags { return FlagOptional }
func (m MED) Len() int              { return 4 }

// WriteTo writes the MED value into buf at offset.
func (m MED) WriteTo(buf []byte, off int) int {
	binary.BigEndian.PutUint32(buf[off:], uint32(m))
	return 4
}

// WriteToWithContext writes the MED value - context-independent.
func (m MED) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return m.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (m MED) CheckedWriteTo(buf []byte, off int) (int, error) {
	if len(buf) < off+4 {
		return 0, wire.ErrBufferTooSmall
	}
	return m.WriteTo(buf, off), nil
}

// ParseMED parses a MULTI_EXIT_DISC attribute.
// RFC 4271 Section 5.1.4 specifies 4-octet length.
func ParseMED(data []byte) (MED, error) {
	if len(data) != 4 {
		return 0, ErrInvalidLength
	}
	return MED(binary.BigEndian.Uint32(data)), nil
}

// LocalPref represents the LOCAL_PREF attribute.
//
// RFC 4271 Section 5.1.5: LOCAL_PREF
//   - Well-known attribute (Type Code 5)
//   - SHALL be included in all UPDATE messages to internal peers
//   - Higher degree of preference MUST be preferred
//   - Value is a 4-octet unsigned integer
//   - MUST NOT be included in UPDATE messages to external peers
//     (except for BGP Confederations per RFC 3065)
//   - If received from an external peer, MUST be ignored
//     (except for BGP Confederations per RFC 3065)
type LocalPref uint32

func (l LocalPref) Code() AttributeCode   { return AttrLocalPref }
func (l LocalPref) Flags() AttributeFlags { return FlagTransitive }
func (l LocalPref) Len() int              { return 4 }

// WriteTo writes the LOCAL_PREF value into buf at offset.
func (l LocalPref) WriteTo(buf []byte, off int) int {
	binary.BigEndian.PutUint32(buf[off:], uint32(l))
	return 4
}

// WriteToWithContext writes the LOCAL_PREF value - context-independent.
func (l LocalPref) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return l.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (l LocalPref) CheckedWriteTo(buf []byte, off int) (int, error) {
	if len(buf) < off+4 {
		return 0, wire.ErrBufferTooSmall
	}
	return l.WriteTo(buf, off), nil
}

// ParseLocalPref parses a LOCAL_PREF attribute.
// RFC 4271 Section 5.1.5 specifies 4-octet length.
func ParseLocalPref(data []byte) (LocalPref, error) {
	if len(data) != 4 {
		return 0, ErrInvalidLength
	}
	return LocalPref(binary.BigEndian.Uint32(data)), nil
}

// AtomicAggregate represents the ATOMIC_AGGREGATE attribute.
//
// RFC 4271 Section 5.1.6: ATOMIC_AGGREGATE
//   - Well-known discretionary attribute (Type Code 6)
//   - Zero length (presence alone is meaningful)
//   - SHOULD be included when an aggregate excludes AS numbers from the
//     AS_PATH of aggregated routes (due to dropping AS_SET)
//   - Receivers SHOULD NOT remove this attribute when propagating
//   - Receivers MUST NOT make any NLRI more specific when this is present
//   - Indicates the actual path may differ from AS_PATH (but is loop-free)
type AtomicAggregate struct{}

func (AtomicAggregate) Code() AttributeCode   { return AttrAtomicAggregate }
func (AtomicAggregate) Flags() AttributeFlags { return FlagTransitive }
func (AtomicAggregate) Len() int              { return 0 }

// WriteTo writes nothing (ATOMIC_AGGREGATE has zero length).
func (AtomicAggregate) WriteTo(_ []byte, _ int) int { return 0 }

// WriteToWithContext writes nothing - context-independent.
func (AtomicAggregate) WriteToWithContext(_ []byte, _ int, _, _ *bgpctx.EncodingContext) int {
	return 0
}

// CheckedWriteTo validates capacity before writing (always succeeds, zero length).
func (AtomicAggregate) CheckedWriteTo(_ []byte, _ int) (int, error) {
	return 0, nil
}

// parseAtomicAggregate validates and returns an AtomicAggregate.
// RFC 4271: ATOMIC_AGGREGATE has length 0.
func parseAtomicAggregate(data []byte) (Attribute, error) {
	if len(data) != 0 {
		return nil, fmt.Errorf("ATOMIC_AGGREGATE must be empty, got %d bytes", len(data))
	}
	return AtomicAggregate{}, nil
}

// Aggregator represents the AGGREGATOR attribute.
//
// RFC 4271 Section 5.1.7: AGGREGATOR
//   - Optional transitive attribute (Type Code 7)
//   - MAY be included in updates formed by aggregation
//   - Contains the AS number and IP address of the BGP speaker that
//     performed the aggregation
//   - The IP address SHOULD be the same as the BGP Identifier
//   - Original format: 2-octet AS number + 4-octet IP address (6 octets)
//
// RFC 6793 (BGP Support for Four-Octet AS Number Space):
//   - Extended format: 4-octet AS number + 4-octet IP address (8 octets)
//   - Used when both peers support 4-byte AS numbers
type Aggregator struct {
	ASN     uint32
	Address netip.Addr
}

func (a *Aggregator) Code() AttributeCode   { return AttrAggregator }
func (a *Aggregator) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns the packed length. Always returns 8 (4-byte AS format).
// Note: RFC 4271 specifies 6 bytes (2-byte AS), but this implementation
// uses RFC 6793 4-byte AS format by default.
func (a *Aggregator) Len() int { return 8 }

// WriteTo writes the AGGREGATOR using 4-byte AS format (RFC 6793).
//
// The address occupies the four octets writeIPv4AddressField owns, whatever form
// the Address holds, so this write can never disagree with the 8 Len returns.
func (a *Aggregator) WriteTo(buf []byte, off int) int {
	binary.BigEndian.PutUint32(buf[off:], a.ASN)
	writeIPv4AddressField(buf, off+4, a.Address)
	return 8
}

// WriteToWithContext writes AGGREGATOR with context-dependent format.
//
// Both branches write the address through writeIPv4AddressField, so each returns
// exactly the count LenWithContext promised for the same context.
func (a *Aggregator) WriteToWithContext(buf []byte, off int, _, dstCtx *bgpctx.EncodingContext) int {
	if dstCtx == nil || dstCtx.ASN4() {
		// 8-byte format: 4-byte ASN + 4-byte IP
		binary.BigEndian.PutUint32(buf[off:], a.ASN)
		writeIPv4AddressField(buf, off+4, a.Address)
		return 8
	}

	// 6-byte format: 2-byte ASN + 4-byte IP
	asn := a.ASN
	if asn > 65535 {
		asn = 23456 // AS_TRANS per RFC 6793 Section 9
	}
	binary.BigEndian.PutUint16(buf[off:], uint16(asn)) //nolint:gosec // bounds checked above
	writeIPv4AddressField(buf, off+2, a.Address)
	return 6
}

// CheckedWriteTo validates capacity before writing.
func (a *Aggregator) CheckedWriteTo(buf []byte, off int) (int, error) {
	if len(buf) < off+8 {
		return 0, wire.ErrBufferTooSmall
	}
	return a.WriteTo(buf, off), nil
}

// LenWithContext returns length based on encoding context.
// RFC 6793: 8 bytes for 4-byte ASN, 6 bytes for 2-byte ASN.
func (a *Aggregator) LenWithContext(_, dstCtx *bgpctx.EncodingContext) int {
	if dstCtx == nil || dstCtx.ASN4() {
		return 8
	}
	return 6
}

// ParseAggregator parses an AGGREGATOR attribute.
//
// RFC 4271 Section 5.1.7: Original 2-byte AS format (6 octets total).
// RFC 6793: Extended 4-byte AS format (8 octets total).
//
// The fourByteAS parameter indicates whether the peer supports 4-byte
// AS numbers (negotiated via BGP capabilities).
func ParseAggregator(data []byte, fourByteAS bool) (*Aggregator, error) {
	if fourByteAS {
		if len(data) != 8 {
			return nil, ErrInvalidLength
		}
		addr, _ := netip.AddrFromSlice(data[4:8])
		return &Aggregator{
			ASN:     binary.BigEndian.Uint32(data[0:4]),
			Address: addr,
		}, nil
	}
	// 2-byte AS format (RFC 4271)
	if len(data) != 6 {
		return nil, ErrInvalidLength
	}
	addr, _ := netip.AddrFromSlice(data[2:6])
	return &Aggregator{
		ASN:     uint32(binary.BigEndian.Uint16(data[0:2])),
		Address: addr,
	}, nil
}

// OriginatorID represents the ORIGINATOR_ID attribute (RFC 4456).
type OriginatorID netip.Addr

func (o OriginatorID) Code() AttributeCode   { return AttrOriginatorID }
func (o OriginatorID) Flags() AttributeFlags { return FlagOptional }
func (o OriginatorID) Len() int              { return 4 }

// WriteTo writes the ORIGINATOR_ID value into buf at offset.
//
// RFC 4456 Section 8: "This attribute is 4 bytes long", so the write is four
// octets for every address form and always returns the 4 Len promised. Copying
// netip.Addr.AsSlice returned that slice's own length instead, which put sixteen
// octets into a four-octet attribute for an IPv6 value.
func (o OriginatorID) WriteTo(buf []byte, off int) int {
	writeIPv4AddressField(buf, off, netip.Addr(o))
	return 4
}

// WriteToWithContext writes the ORIGINATOR_ID value - context-independent.
func (o OriginatorID) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return o.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (o OriginatorID) CheckedWriteTo(buf []byte, off int) (int, error) {
	if len(buf) < off+4 {
		return 0, wire.ErrBufferTooSmall
	}
	return o.WriteTo(buf, off), nil
}

// ParseOriginatorID parses an ORIGINATOR_ID attribute (RFC 4456).
//
// RFC 4456 Section 8: "ORIGINATOR_ID is a new optional, non-transitive
// BGP attribute of Type code 9. This attribute is 4 bytes long and it
// will be created by an RR in reflecting a route."
//
// RFC 4456 Section 8: "A router that recognizes the ORIGINATOR_ID attribute
// SHOULD ignore a route received with its BGP Identifier as the ORIGINATOR_ID."
// (Loop prevention for reflected routes.)
func ParseOriginatorID(data []byte) (OriginatorID, error) {
	if len(data) != 4 {
		return OriginatorID{}, ErrInvalidLength
	}
	addr, ok := netip.AddrFromSlice(data)
	if !ok {
		return OriginatorID{}, ErrMalformedValue
	}
	return OriginatorID(addr), nil
}

// ClusterList represents the CLUSTER_LIST attribute (RFC 4456).
type ClusterList []uint32

func (c ClusterList) Code() AttributeCode   { return AttrClusterList }
func (c ClusterList) Flags() AttributeFlags { return FlagOptional }
func (c ClusterList) Len() int              { return len(c) * 4 }

// WriteTo writes the CLUSTER_LIST value into buf at offset.
func (c ClusterList) WriteTo(buf []byte, off int) int {
	for i, id := range c {
		binary.BigEndian.PutUint32(buf[off+i*4:], id)
	}
	return len(c) * 4
}

// WriteToWithContext writes the CLUSTER_LIST value - context-independent.
func (c ClusterList) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return c.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (c ClusterList) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := c.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return c.WriteTo(buf, off), nil
}

// ParseClusterList parses a CLUSTER_LIST attribute.
//
// RFC 4456 Section 8: "CLUSTER_LIST is a new, optional, non-transitive
// BGP attribute of Type code 10. It is a sequence of CLUSTER_ID values
// representing the reflection path that the route has passed."
//
// RFC 4456 Section 8: "When an RR reflects a route, it MUST prepend the
// local CLUSTER_ID to the CLUSTER_LIST. If the CLUSTER_LIST is empty,
// it MUST create a new one.".
func ParseClusterList(data []byte) (ClusterList, error) {
	if len(data)%4 != 0 {
		return nil, ErrInvalidLength
	}
	list := make(ClusterList, len(data)/4)
	for i := range list {
		list[i] = binary.BigEndian.Uint32(data[i*4:])
	}
	return list, nil
}
