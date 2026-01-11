package attribute

import (
	"encoding/binary"
	"net/netip"

	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/wire"
)

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
func (n *NextHop) Len() int {
	if n.Addr.Is6() {
		return 16
	}
	return 4
}
func (n *NextHop) Pack() []byte { return n.Addr.AsSlice() }

// PackWithContext returns Pack() - NEXT_HOP encoding is context-independent.
func (n *NextHop) PackWithContext(_, _ *bgpctx.EncodingContext) []byte { return n.Pack() }

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
func (m MED) Pack() []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(m))
	return buf
}

// PackWithContext returns Pack() - MED encoding is context-independent.
func (m MED) PackWithContext(_, _ *bgpctx.EncodingContext) []byte { return m.Pack() }

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
func (l LocalPref) Pack() []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(l))
	return buf
}

// PackWithContext returns Pack() - LOCAL_PREF encoding is context-independent.
func (l LocalPref) PackWithContext(_, _ *bgpctx.EncodingContext) []byte { return l.Pack() }

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
func (AtomicAggregate) Pack() []byte          { return nil }

// PackWithContext returns Pack() - ATOMIC_AGGREGATE encoding is context-independent.
func (AtomicAggregate) PackWithContext(_, _ *bgpctx.EncodingContext) []byte { return nil }

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

// Pack encodes the AGGREGATOR using 4-byte AS format (RFC 6793).
func (a *Aggregator) Pack() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], a.ASN)
	copy(buf[4:8], a.Address.AsSlice())
	return buf
}

// PackWithContext serializes AGGREGATOR with context-dependent format.
//
// RFC 6793: 8-byte format (4-byte ASN + 4-byte IP) when dstCtx.ASN4=true,
// 6-byte format (2-byte ASN + 4-byte IP) when dstCtx.ASN4=false.
// Large ASNs (>65535) are encoded as AS_TRANS (23456) in 2-byte format.
func (a *Aggregator) PackWithContext(_, dstCtx *bgpctx.EncodingContext) []byte {
	if dstCtx == nil || dstCtx.ASN4 {
		// 8-byte format: 4-byte ASN + 4-byte IP
		buf := make([]byte, 8)
		binary.BigEndian.PutUint32(buf[0:4], a.ASN)
		copy(buf[4:8], a.Address.AsSlice())
		return buf
	}

	// 6-byte format: 2-byte ASN + 4-byte IP
	asn := a.ASN
	if asn > 65535 {
		asn = 23456 // AS_TRANS per RFC 6793 Section 9
	}
	buf := make([]byte, 6)
	binary.BigEndian.PutUint16(buf[0:2], uint16(asn)) // #nosec G115 -- bounds checked above
	copy(buf[2:6], a.Address.AsSlice())
	return buf
}

// WriteTo writes the AGGREGATOR using 4-byte AS format (RFC 6793).
func (a *Aggregator) WriteTo(buf []byte, off int) int {
	binary.BigEndian.PutUint32(buf[off:], a.ASN)
	copy(buf[off+4:], a.Address.AsSlice())
	return 8
}

// WriteToWithContext writes AGGREGATOR with context-dependent format.
func (a *Aggregator) WriteToWithContext(buf []byte, off int, _, dstCtx *bgpctx.EncodingContext) int {
	if dstCtx == nil || dstCtx.ASN4 {
		// 8-byte format: 4-byte ASN + 4-byte IP
		binary.BigEndian.PutUint32(buf[off:], a.ASN)
		copy(buf[off+4:], a.Address.AsSlice())
		return 8
	}

	// 6-byte format: 2-byte ASN + 4-byte IP
	asn := a.ASN
	if asn > 65535 {
		asn = 23456 // AS_TRANS per RFC 6793 Section 9
	}
	binary.BigEndian.PutUint16(buf[off:], uint16(asn)) //nolint:gosec // bounds checked above
	copy(buf[off+2:], a.Address.AsSlice())
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
	if dstCtx == nil || dstCtx.ASN4 {
		return 8
	}
	return 6
}

// CheckedWriteToWithContext validates capacity before writing with context.
func (a *Aggregator) CheckedWriteToWithContext(buf []byte, off int, srcCtx, dstCtx *bgpctx.EncodingContext) (int, error) {
	needed := a.LenWithContext(srcCtx, dstCtx)
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return a.WriteToWithContext(buf, off, srcCtx, dstCtx), nil
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
func (o OriginatorID) Pack() []byte          { return netip.Addr(o).AsSlice() }

// PackWithContext returns Pack() - ORIGINATOR_ID encoding is context-independent.
func (o OriginatorID) PackWithContext(_, _ *bgpctx.EncodingContext) []byte { return o.Pack() }

// WriteTo writes the ORIGINATOR_ID value into buf at offset.
func (o OriginatorID) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], netip.Addr(o).AsSlice())
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
// ORIGINATOR_ID is the Router ID (4 bytes) of the route reflector client
// that originated the route.
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
func (c ClusterList) Pack() []byte {
	buf := make([]byte, len(c)*4)
	for i, id := range c {
		binary.BigEndian.PutUint32(buf[i*4:], id)
	}
	return buf
}

// PackWithContext returns Pack() - CLUSTER_LIST encoding is context-independent.
func (c ClusterList) PackWithContext(_, _ *bgpctx.EncodingContext) []byte { return c.Pack() }

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
