// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS descriptor TLV encoding
// RFC: rfc/short/rfc7752.md — BGP-LS node and link descriptors
// Overview: types.go — core types, TLV constants, and helper functions
// Related: types_nlri.go — NLRI types that embed these descriptors
// Related: types_srv6.go — SRv6 SID descriptor (RFC 9514)
package ls

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"slices"

	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// NodeDescriptor contains node identification information.
// RFC 7752 Section 3.2.1.4 defines the node descriptor sub-TLVs.
type NodeDescriptor struct {
	ASN             uint32   // Autonomous System (TLV 512, RFC 7752 Section 3.2.1.4)
	BGPLSIdentifier uint32   // BGP-LS Identifier (TLV 513, RFC 7752 Section 3.2.1.4)
	OSPFAreaID      uint32   // OSPF Area-ID (TLV 514, RFC 7752 Section 3.2.1.4)
	IGPRouterID     []byte   // IGP Router-ID (TLV 515, RFC 7752 Section 3.2.1.4)
	BGPRouterID     uint32   // BGP Router-ID (TLV 516, RFC 9086 Section 4.1) — IPv4 as uint32
	ConfedMember    uint32   // BGP Confederation Member (TLV 517, RFC 9086 Section 4.2)
	SRv6SIDs        [][]byte // SRv6 SID addresses (TLV 518, RFC 9514) — 16 bytes each

	// Presence for the two sub-TLVs whose ZERO is a legal value, carried apart
	// from the value because the value cannot carry it.
	//
	// RFC 9552 Section 5.2.1.1 builds the node KEY from the OSPF Area-ID,
	// Router-ID, Protocol-ID, MT-ID and BGP-LS Instance-ID, and Section 5.2.1.4
	// calls the Area-ID "a mandatory TLV when originating information from
	// OSPF". Area 0.0.0.0 is the backbone. For TLV 513 the same section says
	// "The default value of 0 is RECOMMENDED". Eliding either on a zero value
	// therefore encodes a backbone node and an area-less node to the SAME key,
	// which is what Section 5.2.1.1 forbids: "Two different nodes MUST NOT be
	// represented by the same key."
	//
	// The other uint32 fields need no flag: AS 0 is reserved by RFC 7607, a BGP
	// Router-ID of 0.0.0.0 identifies nothing, and a confederation member of 0
	// is the same reserved AS. For those, zero and absent mean the same thing.
	HasBGPLSIdentifier bool
	HasOSPFAreaID      bool
}

// srv6SIDsOrdered returns the SRv6 SID values in the order RFC 9552 Section 5.1
// requires of repeated TLVs sharing one type: "first in ascending order based on
// the Length field followed by ascending order based on the Value field", the
// value compared "as opaque binary data and ordered lexicographically".
//
// TLV 518 is the only sub-TLV a descriptor can repeat, and a Propagator MUST
// consider an NLRI that breaks the ordering malformed, so slice order is not a
// free choice. Emitting it also gave one node two keys, which Section 5.2.1.1
// forbids: the same SIDs stored in a different order encoded differently.
//
// The caller's slice is never reordered, and fewer than two SIDs allocate
// nothing.
func (nd *NodeDescriptor) srv6SIDsOrdered() [][]byte {
	if len(nd.SRv6SIDs) < 2 {
		return nd.SRv6SIDs
	}

	ordered := slices.Clone(nd.SRv6SIDs)
	slices.SortStableFunc(ordered, func(a, b []byte) int {
		if byLength := cmp.Compare(len(a), len(b)); byLength != 0 {
			return byLength
		}
		return bytes.Compare(a, b)
	})

	return ordered
}

// Bytes encodes the node descriptor as TLVs.
// RFC 7752 Section 3.2.1.4 specifies the encoding of node descriptor sub-TLVs.
func (nd *NodeDescriptor) Bytes() []byte {
	var data []byte

	// ASN TLV (512) - RFC 7752 Section 3.2.1.4
	if nd.ASN != 0 {
		data = append(data, tlv(TLVAutonomousSystem, uint32ToBytes(nd.ASN))...)
	}

	// BGP-LS Identifier TLV (513) - RFC 7752 Section 3.2.1.4
	if nd.HasBGPLSIdentifier || nd.BGPLSIdentifier != 0 {
		data = append(data, tlv(TLVBGPLSIdentifier, uint32ToBytes(nd.BGPLSIdentifier))...)
	}

	// OSPF Area-ID TLV (514) - RFC 7752 Section 3.2.1.4
	if nd.HasOSPFAreaID || nd.OSPFAreaID != 0 {
		data = append(data, tlv(TLVOSPFAreaID, uint32ToBytes(nd.OSPFAreaID))...)
	}

	// IGP Router-ID TLV (515) - RFC 7752 Section 3.2.1.4
	if len(nd.IGPRouterID) > 0 {
		data = append(data, tlv(TLVIGPRouterID, nd.IGPRouterID)...)
	}

	// BGP Router-ID TLV (516) - RFC 9086 Section 4.1
	if nd.BGPRouterID != 0 {
		data = append(data, tlv(TLVBGPRouterID, uint32ToBytes(nd.BGPRouterID))...)
	}

	// BGP Confederation Member TLV (517) - RFC 9086 Section 4.2
	if nd.ConfedMember != 0 {
		data = append(data, tlv(TLVConfedMember, uint32ToBytes(nd.ConfedMember))...)
	}

	// SRv6 SID TLV (518) - RFC 9514
	for _, sid := range nd.srv6SIDsOrdered() {
		data = append(data, tlv(TLVSRv6SID, sid)...)
	}

	return data
}

// Len returns the TLV-encoded length in bytes.
func (nd *NodeDescriptor) Len() int {
	n := 0
	if nd.ASN != 0 {
		n += 4 + 4 // TLV header + 4-byte value
	}
	if nd.HasBGPLSIdentifier || nd.BGPLSIdentifier != 0 {
		n += 4 + 4
	}
	if nd.HasOSPFAreaID || nd.OSPFAreaID != 0 {
		n += 4 + 4
	}
	if len(nd.IGPRouterID) > 0 {
		n += 4 + len(nd.IGPRouterID)
	}
	if nd.BGPRouterID != 0 {
		n += 4 + 4 // TLV 516 (RFC 9086)
	}
	if nd.ConfedMember != 0 {
		n += 4 + 4 // TLV 517 (RFC 9086)
	}
	for _, sid := range nd.SRv6SIDs {
		n += 4 + len(sid) // TLV 518 (RFC 9514)
	}
	return n
}

// WriteTo writes the node descriptor TLVs directly to buf at offset.
// Returns bytes written.
func (nd *NodeDescriptor) WriteTo(buf []byte, off int) int {
	pos := off

	if nd.ASN != 0 {
		pos += writeTLV(buf, pos, TLVAutonomousSystem, 4)
		binary.BigEndian.PutUint32(buf[pos-4:], nd.ASN)
	}
	if nd.HasBGPLSIdentifier || nd.BGPLSIdentifier != 0 {
		pos += writeTLV(buf, pos, TLVBGPLSIdentifier, 4)
		binary.BigEndian.PutUint32(buf[pos-4:], nd.BGPLSIdentifier)
	}
	if nd.HasOSPFAreaID || nd.OSPFAreaID != 0 {
		pos += writeTLV(buf, pos, TLVOSPFAreaID, 4)
		binary.BigEndian.PutUint32(buf[pos-4:], nd.OSPFAreaID)
	}
	if len(nd.IGPRouterID) > 0 {
		pos += writeTLVBytes(buf, pos, TLVIGPRouterID, nd.IGPRouterID)
	}
	if nd.BGPRouterID != 0 {
		pos += writeTLV(buf, pos, TLVBGPRouterID, 4)
		binary.BigEndian.PutUint32(buf[pos-4:], nd.BGPRouterID)
	}
	if nd.ConfedMember != 0 {
		pos += writeTLV(buf, pos, TLVConfedMember, 4)
		binary.BigEndian.PutUint32(buf[pos-4:], nd.ConfedMember)
	}
	for _, sid := range nd.srv6SIDsOrdered() {
		pos += writeTLVBytes(buf, pos, TLVSRv6SID, sid)
	}

	return pos - off
}

// CheckedWriteTo validates capacity before writing.
func (nd *NodeDescriptor) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := nd.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return nd.WriteTo(buf, off), nil
}

// LinkDescriptor contains link identification information.
// RFC 7752 Section 3.2.2 defines the link descriptor TLVs.
type LinkDescriptor struct {
	LinkLocalID        uint32 // Link Local ID (TLV 258, RFC 7752 Section 3.2.2)
	LinkRemoteID       uint32 // Link Remote ID (TLV 258, RFC 7752 Section 3.2.2)
	LocalInterfaceAddr []byte // IPv4 (TLV 259) or IPv6 (TLV 261) Interface Address
	NeighborAddr       []byte // IPv4 (TLV 260) or IPv6 (TLV 262) Neighbor Address
	MultiTopologyID    uint16 // Multi-Topology ID (TLV 263, RFC 7752 Section 3.2.2)

	// Presence for the MT-ID, whose zero is the DEFAULT topology and so a legal
	// value. RFC 9552 Section 5.2.1.1 names the Multi-Topology Identifier as
	// part of the key, so a link in the default topology and one in no topology
	// must not encode alike.
	HasMultiTopologyID bool
}

// descriptorTLV is one encoded sub-TLV. A zero value carries no TLV.
type descriptorTLV struct {
	kind  uint16
	value []byte
}

// addressTLV types one address by its length: 4 octets selects the IPv4
// sub-TLV, 16 the IPv6 one. Any other length carries no address and encodes
// nothing.
func addressTLV(addr []byte, v4Kind, v6Kind uint16) descriptorTLV {
	switch len(addr) {
	case 4:
		return descriptorTLV{kind: v4Kind, value: addr}
	case 16:
		return descriptorTLV{kind: v6Kind, value: addr}
	}

	return descriptorTLV{}
}

// addressTLVs returns the interface-address and neighbor-address sub-TLVs in
// ascending type order, an absent one last.
//
// Each address is typed by its own family: an interface address is TLV 259
// (IPv4) or 261 (IPv6), a neighbor address is 260 or 262. Field order and type
// order are therefore not the same order, and an IPv6 interface address beside
// an IPv4 neighbor address emits 261 before 260. RFC 9552 Section 5.1 requires
// that "all TLVs within the NLRI MUST be ordered in ascending order by TLV
// Type", and an NLRI that breaks the rule "MUST be considered as malformed by a
// BGP-LS Propagator".
//
// Bytes, Len and WriteTo all read the order from here, so the three cannot
// drift apart.
func (ld *LinkDescriptor) addressTLVs() (first, second descriptorTLV) {
	iface := addressTLV(ld.LocalInterfaceAddr, TLVIPv4InterfaceAddr, TLVIPv6InterfaceAddr)
	neighbor := addressTLV(ld.NeighborAddr, TLVIPv4NeighborAddr, TLVIPv6NeighborAddr)

	if neighbor.value != nil && (iface.value == nil || neighbor.kind < iface.kind) {
		return neighbor, iface
	}

	return iface, neighbor
}

// Bytes encodes the link descriptor as TLVs.
// RFC 7752 Section 3.2.2 specifies the encoding of link descriptor TLVs.
func (ld *LinkDescriptor) Bytes() []byte {
	var data []byte

	// Link Local/Remote Identifiers (TLV 258) - RFC 7752 Section 3.2.2
	// Format: 4-byte Local ID + 4-byte Remote ID = 8 bytes total
	if ld.LinkLocalID != 0 || ld.LinkRemoteID != 0 {
		val := make([]byte, 8)
		binary.BigEndian.PutUint32(val[0:4], ld.LinkLocalID)
		binary.BigEndian.PutUint32(val[4:8], ld.LinkRemoteID)
		data = append(data, tlv(TLVLinkLocalRemoteID, val)...)
	}

	// Interface and neighbor addresses (TLVs 259 to 262), in the ascending type
	// order addressTLVs settles - RFC 9552 Section 5.1
	first, second := ld.addressTLVs()
	if first.value != nil {
		data = append(data, tlv(first.kind, first.value)...)
	}
	if second.value != nil {
		data = append(data, tlv(second.kind, second.value)...)
	}

	// Multi-Topology Identifier (TLV 263) - RFC 9552 Section 5.2.2.1. Last,
	// because 263 is the highest type here and Section 5.1 requires TLVs "be
	// ordered in ascending order by TLV Type".
	if ld.HasMultiTopologyID || ld.MultiTopologyID != 0 {
		data = append(data, tlv(TLVMultiTopologyID, uint16ToBytes(ld.MultiTopologyID))...)
	}

	return data
}

// Len returns the TLV-encoded length in bytes.
func (ld *LinkDescriptor) Len() int {
	n := 0
	if ld.LinkLocalID != 0 || ld.LinkRemoteID != 0 {
		n += 4 + 8 // TLV header + 8-byte value
	}
	// Counted through addressTLVs so Len agrees with what Bytes and WriteTo
	// actually emit: an address of any length other than 4 or 16 carries no
	// sub-TLV and must not be counted.
	first, second := ld.addressTLVs()
	if first.value != nil {
		n += 4 + len(first.value)
	}
	if second.value != nil {
		n += 4 + len(second.value)
	}
	if ld.HasMultiTopologyID || ld.MultiTopologyID != 0 {
		n += 4 + 2 // TLV 263 carries a 2-octet MT-ID
	}
	return n
}

// WriteTo writes the link descriptor TLVs directly to buf at offset.
// Returns bytes written.
func (ld *LinkDescriptor) WriteTo(buf []byte, off int) int {
	pos := off

	if ld.LinkLocalID != 0 || ld.LinkRemoteID != 0 {
		pos += writeTLV(buf, pos, TLVLinkLocalRemoteID, 8)
		binary.BigEndian.PutUint32(buf[pos-8:], ld.LinkLocalID)
		binary.BigEndian.PutUint32(buf[pos-4:], ld.LinkRemoteID)
	}

	first, second := ld.addressTLVs()
	if first.value != nil {
		pos += writeTLVBytes(buf, pos, first.kind, first.value)
	}
	if second.value != nil {
		pos += writeTLVBytes(buf, pos, second.kind, second.value)
	}

	if ld.HasMultiTopologyID || ld.MultiTopologyID != 0 {
		pos += writeTLV(buf, pos, TLVMultiTopologyID, 2)
		binary.BigEndian.PutUint16(buf[pos-2:], ld.MultiTopologyID)
	}

	return pos - off
}

// CheckedWriteTo validates capacity before writing.
func (ld *LinkDescriptor) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := ld.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return ld.WriteTo(buf, off), nil
}

// PrefixDescriptor contains prefix identification information.
// RFC 7752 Section 3.2.3 defines the prefix descriptor TLVs.
type PrefixDescriptor struct {
	MultiTopologyID    uint16 // Multi-Topology ID (TLV 263, RFC 7752 Section 3.2.3)
	OSPFRouteType      uint8  // OSPF Route Type (TLV 264, RFC 7752 Section 3.2.3)
	IPReachabilityInfo []byte // IP Reachability Information (TLV 265, RFC 7752 Section 3.2.3)

	// Presence for the MT-ID, whose zero is the DEFAULT topology and therefore a
	// legal value. RFC 9552 Section 5.2.1.1 names the Multi-Topology Identifier
	// as part of the key that distinguishes one node from another, so a prefix
	// in the default topology and one in no topology at all must not encode
	// alike.
	//
	// OSPFRouteType needs no flag: RFC 9552 Section 5.2.3 assigns 1 through 6
	// and leaves 0 unassigned, so zero and absent mean the same thing.
	HasMultiTopologyID bool
}

// Bytes encodes the prefix descriptor as TLVs.
// RFC 7752 Section 3.2.3 specifies the encoding of prefix descriptor TLVs.
func (pd *PrefixDescriptor) Bytes() []byte {
	var data []byte

	// TLVs are written in ascending type order because RFC 9552 Section 5.1
	// requires it: "To compare NLRIs with unknown TLVs, all TLVs within the NLRI
	// MUST be ordered in ascending order by TLV Type." So 263, then 264, then 265.

	// Multi-Topology Identifier (TLV 263) - RFC 9552 Section 5.2.2.1
	if pd.HasMultiTopologyID || pd.MultiTopologyID != 0 {
		data = append(data, tlv(TLVMultiTopologyID, uint16ToBytes(pd.MultiTopologyID))...)
	}

	// OSPF Route Type (TLV 264) - RFC 9552 Section 5.2.3
	if pd.OSPFRouteType != 0 {
		data = append(data, tlv(TLVOSPFRouteType, []byte{pd.OSPFRouteType})...)
	}

	// IP Reachability Information (TLV 265) - RFC 7752 Section 3.2.3
	if len(pd.IPReachabilityInfo) > 0 {
		data = append(data, tlv(TLVIPReachabilityInfo, pd.IPReachabilityInfo)...)
	}

	return data
}

// Len returns the TLV-encoded length in bytes.
func (pd *PrefixDescriptor) Len() int {
	n := 0
	if pd.HasMultiTopologyID || pd.MultiTopologyID != 0 {
		n += 4 + 2 // TLV 263 carries a 2-octet MT-ID
	}
	if pd.OSPFRouteType != 0 {
		n += 4 + 1 // TLV 264 carries a 1-octet route type
	}
	if len(pd.IPReachabilityInfo) > 0 {
		n += 4 + len(pd.IPReachabilityInfo)
	}
	return n
}

// WriteTo writes the prefix descriptor TLVs directly to buf at offset.
// Returns bytes written.
func (pd *PrefixDescriptor) WriteTo(buf []byte, off int) int {
	pos := off
	if pd.HasMultiTopologyID || pd.MultiTopologyID != 0 {
		pos += writeTLV(buf, pos, TLVMultiTopologyID, 2)
		binary.BigEndian.PutUint16(buf[pos-2:], pd.MultiTopologyID)
	}
	if pd.OSPFRouteType != 0 {
		pos += writeTLVBytes(buf, pos, TLVOSPFRouteType, []byte{pd.OSPFRouteType})
	}
	if len(pd.IPReachabilityInfo) > 0 {
		pos += writeTLVBytes(buf, pos, TLVIPReachabilityInfo, pd.IPReachabilityInfo)
	}
	return pos - off
}

// CheckedWriteTo validates capacity before writing.
func (pd *PrefixDescriptor) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := pd.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return pd.WriteTo(buf, off), nil
}
