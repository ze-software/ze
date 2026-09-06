// Design: docs/architecture/wire/attributes.md — path attribute encoding
// RFC: rfc/short/rfc4271.md — attribute header size class and ascending emission order (Sections 4.3, 5)
// Related: attribute.go — WriteAttrTo and AttrWireLen, the per-attribute primitives this file emits through

package attribute

import (
	"net/netip"
)

// Builder accumulates path attributes and hands them out as attribute VALUES in
// ascending type-code order.
//
// It is an intent collector, not an encoder. Until 2026-08-01 it carried a
// second, independent attribute encoder: a WriteTo whose emission order was
// hard-coded in the function body, and a Len that re-derived the header size
// class the same body decided again. That made three writers for one wire
// format, so ordering was an agreement between call sites rather than a property
// of one writer. AppendAttributes is now the single ordering statement, and both
// consumers read it: Build materializes bytes through WriteAttrTo, and the
// announce rails plan the same list as edit-set slots for the shared one-pass
// writer (internal/component/bgp/reactor/announce_build.go).
//
// Example usage:
//
//	b := NewBuilder()
//	b.SetOrigin(OriginIGP)
//	b.SetLocalPref(100)
//	b.AddCommunity(65000, 100)
//	wireBytes := b.Build()
type Builder struct {
	origin           *Origin
	nextHop          *NextHop // IPv4 next-hop (type 3)
	localPref        *LocalPref
	med              *MED
	asPath           *ASPath
	asPathASNs       []uint32
	communities      Communities
	largeCommunities LargeCommunities
	extCommunities   ExtendedCommunities
	atomicAggregate  bool
	aggregator       *Aggregator
	originatorID     *OriginatorID
	clusterList      ClusterList
	aigp             *AIGP
	prefixSID        []byte // BGP Prefix-SID TLVs (attribute 40), already encoded

	// Pre-built wire bytes (for forwarding received attributes)
	wire []byte
}

// NewBuilder creates a new attribute builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// SetOrigin sets the ORIGIN attribute.
// 0=IGP, 1=EGP, 2=INCOMPLETE.
func (b *Builder) SetOrigin(origin uint8) *Builder {
	o := Origin(origin)
	b.origin = &o
	return b
}

// SetLocalPref sets the LOCAL_PREF attribute.
func (b *Builder) SetLocalPref(pref uint32) *Builder {
	lp := LocalPref(pref)
	b.localPref = &lp
	return b
}

// SetMED sets the MULTI_EXIT_DISC attribute.
func (b *Builder) SetMED(med uint32) *Builder {
	m := MED(med)
	b.med = &m
	return b
}

// SetNextHop sets the NEXT_HOP attribute from raw bytes (IPv4 only, type code 3).
// RFC 4271 Section 5.1.3 - well-known mandatory for IPv4 unicast.
// For IPv6, use MP_REACH_NLRI instead.
// The bytes are copied as-is (network byte order).
func (b *Builder) SetNextHop(ip [4]byte) *Builder {
	return b.SetNextHopAddr(netip.AddrFrom4(ip))
}

// SetNextHopAddr sets the NEXT_HOP attribute from netip.Addr.
// Only IPv4 addresses are valid; IPv6 returns the builder unchanged.
func (b *Builder) SetNextHopAddr(addr netip.Addr) *Builder {
	if !addr.Is4() {
		return b
	}
	b.nextHop = &NextHop{Addr: addr}
	return b
}

// SetASPath sets the AS_PATH as a sequence of ASNs.
//
// The builder models an AS_PATH as ONE AS_SEQUENCE. ASPath.WriteToWithASN4
// splits a segment above MaxASPathSegmentLength exactly as the retired
// Builder.WriteTo did (RFC 4271 Section 4.3: a segment carries at most 255 AS
// numbers), so the emitted bytes are unchanged.
func (b *Builder) SetASPath(asns []uint32) *Builder {
	b.asPathASNs = asns
	if len(asns) == 0 {
		b.asPath = nil
		return b
	}
	b.asPath = &ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: asns}}}
	return b
}

// AddCommunity adds a standard community (RFC 1997).
func (b *Builder) AddCommunity(high, low uint16) *Builder {
	b.communities = append(b.communities, Community(uint32(high)<<16|uint32(low)))
	return b
}

// AddCommunityValue adds a community by its 32-bit value.
func (b *Builder) AddCommunityValue(community uint32) *Builder {
	b.communities = append(b.communities, Community(community))
	return b
}

// AddLargeCommunity adds a large community (RFC 8092).
func (b *Builder) AddLargeCommunity(global, local1, local2 uint32) *Builder {
	b.largeCommunities = append(b.largeCommunities, LargeCommunity{
		GlobalAdmin: global,
		LocalData1:  local1,
		LocalData2:  local2,
	})
	return b
}

// AddExtendedCommunity adds an extended community (RFC 4360).
func (b *Builder) AddExtendedCommunity(ec ExtendedCommunity) *Builder {
	b.extCommunities = append(b.extCommunities, ec)
	return b
}

// SetAtomicAggregate marks the ATOMIC_AGGREGATE attribute present.
//
// RFC 4271 Section 4.3 f): "ATOMIC_AGGREGATE is a well-known discretionary
// attribute of length 0." Presence alone carries the meaning, so the setter
// takes no value. Reset clears it.
func (b *Builder) SetAtomicAggregate() *Builder {
	b.atomicAggregate = true
	return b
}

// SetAggregator sets the AGGREGATOR attribute (RFC 4271 Section 5.1.7).
//
// The caller MUST pass an IPv4 address and a non-zero ASN. The address field is
// four octets wide, so an IPv6 value has nowhere to go, and RFC 7607 Section 2
// forbids originating AS 0. Both are refused where operator text is read
// (parseAggregatorText in the bgp-cmd-update plugin), because that is the only
// boundary an invalid value can enter through.
func (b *Builder) SetAggregator(asn uint32, addr netip.Addr) *Builder {
	b.aggregator = &Aggregator{ASN: asn, Address: addr}
	return b
}

// SetOriginatorID sets the ORIGINATOR_ID attribute (RFC 4456 Section 8).
//
// The caller MUST pass an IPv4 address: RFC 4456 Section 8 states the attribute
// "is 4 bytes long", so a wider value cannot be encoded.
func (b *Builder) SetOriginatorID(addr netip.Addr) *Builder {
	o := OriginatorID(addr)
	b.originatorID = &o
	return b
}

// SetClusterList sets the CLUSTER_LIST attribute (RFC 4456 Section 8).
//
// The order of ids is the reflection path and is preserved: RFC 4456 Section 8
// says a route reflector "MUST prepend the local CLUSTER_ID to the
// CLUSTER_LIST", so the first id is the most recent reflector. The slice is
// referenced, not copied; the caller MUST NOT mutate it afterwards.
func (b *Builder) SetClusterList(ids []uint32) *Builder {
	b.clusterList = ClusterList(ids)
	return b
}

// SetAIGP sets the AIGP attribute with the given metric value (RFC 7311).
func (b *Builder) SetAIGP(metric uint64) *Builder { //nolint:unparam // fluent-builder contract: every Builder setter returns *Builder so calls chain (SetOrigin ... setWire). builder_parse.go ends its chain here, but a setter that returns nothing cannot sit in the middle of one
	b.aigp = NewAIGPMetric(metric)
	return b
}

// SetPrefixSID sets the BGP Prefix-SID attribute (code 40) from its already
// encoded TLV bytes, as ParsePrefixSID and ParsePrefixSIDSRv6 produce them.
//
// RFC 8669 Section 3: "The BGP Prefix-SID attribute is an optional, transitive
// BGP path attribute." The flags are written 0xC0 for that reason, and the TLVs
// are passed through unread: their layout is RFC 8669 Section 3.1 and RFC 9252
// Section 3, and the encoder that produced them owns it (prefixsid.go).
//
// The slice is referenced, not copied; the caller MUST NOT mutate it afterwards.
// An empty slice clears the attribute rather than emitting a zero-length one,
// because attribute 40 with no TLV names no SID.
func (b *Builder) SetPrefixSID(tlvs []byte) *Builder { //nolint:unparam // fluent-builder contract: every Builder setter returns *Builder so calls chain
	b.prefixSID = tlvs
	return b
}

// setWire sets pre-built wire bytes (for forwarding).
// When wire is set, Build() returns it directly.
func (b *Builder) setWire(wire []byte) *Builder { //nolint:unparam // fluent-builder contract: every Builder setter returns *Builder so calls chain. The caller that ends its chain here is not the one the signature serves; a setter returning nothing cannot sit in the middle of a chain
	b.wire = wire
	return b
}

// builderInlineAttrs is the number of attributes AppendAttributes can produce.
// It is the setter count, so a caller's scratch array never spills: ORIGIN,
// AS_PATH, NEXT_HOP, MED, LOCAL_PREF, ATOMIC_AGGREGATE, AGGREGATOR, COMMUNITY,
// ORIGINATOR_ID, CLUSTER_LIST, EXTENDED_COMMUNITY, AIGP, LARGE_COMMUNITY,
// PREFIX_SID.
const builderInlineAttrs = 14

// AppendAttributes appends the builder's attributes to dst in ASCENDING type-code
// order and returns the extended slice.
//
// This is the builder's ONLY ordering statement. RFC 4271 Section 5 orders path
// attributes by ascending type code on emission, and every consumer of a Builder
// now inherits that order from this one function rather than restating it: Build
// materializes the list through WriteAttrTo, and the announce rails plan the same
// list as edit-set slots for the shared one-pass writer.
//
// It returns dst unchanged when the raw-wire escape hatch is set: those bytes are
// an already-encoded attribute section that the builder does not interpret, so
// they are a base rather than a list of attributes. Callers that must handle both
// read RawWire first.
//
// Pass a stack array of builderInlineAttrs to stay allocation-free:
//
//	var scratch [attribute.BuilderInlineAttrs]attribute.Attribute
//	attrs := b.AppendAttributes(scratch[:0])
func (b *Builder) AppendAttributes(dst []Attribute) []Attribute {
	if len(b.wire) > 0 {
		return dst
	}
	if b.origin != nil {
		dst = append(dst, *b.origin)
	}
	if b.asPath != nil {
		dst = append(dst, b.asPath)
	}
	if b.nextHop != nil {
		dst = append(dst, b.nextHop)
	}
	if b.med != nil {
		dst = append(dst, *b.med)
	}
	if b.localPref != nil {
		dst = append(dst, *b.localPref)
	}
	if b.atomicAggregate {
		dst = append(dst, AtomicAggregate{})
	}
	if b.aggregator != nil {
		dst = append(dst, b.aggregator)
	}
	if len(b.communities) > 0 {
		dst = append(dst, b.communities)
	}
	if b.originatorID != nil {
		dst = append(dst, *b.originatorID)
	}
	if len(b.clusterList) > 0 {
		dst = append(dst, b.clusterList)
	}
	if len(b.extCommunities) > 0 {
		dst = append(dst, b.extCommunities)
	}
	if b.aigp != nil {
		dst = append(dst, b.aigp)
	}
	if len(b.largeCommunities) > 0 {
		dst = append(dst, b.largeCommunities)
	}
	if len(b.prefixSID) > 0 {
		dst = append(dst, NewOpaqueAttribute(FlagOptional|FlagTransitive, AttrPrefixSID, b.prefixSID))
	}
	return dst
}

// BuilderInlineAttrs is builderInlineAttrs, exported so a caller can size its own
// scratch array against the same constant instead of guessing.
const BuilderInlineAttrs = builderInlineAttrs

// RawWire returns the pre-encoded attribute section set by SetWire, or nil.
//
// The escape hatch is how flowspec and other pre-encoded attributes pass through
// untouched, so a consumer that can take a base section takes these bytes and
// plans nothing (AppendAttributes returns no attributes for such a builder).
func (b *Builder) RawWire() []byte { return b.wire }

// Len returns the wire-format length in bytes.
// Use this to pre-allocate buffers before calling Build.
func (b *Builder) Len() int {
	if len(b.wire) > 0 {
		return len(b.wire)
	}
	var scratch [builderInlineAttrs]Attribute
	size := 0
	for _, attr := range b.AppendAttributes(scratch[:0]) {
		size += AttrWireLen(attr)
	}
	return size
}

// Build produces the wire-format bytes for all attributes, in ascending type-code
// order (RFC 4271 Section 5).
//
// It is the byte-producing convenience for callers that hold no output buffer.
// The bytes come from WriteAttrTo -- the same per-attribute primitive every other
// writer in the tree uses -- so this function decides nothing about the header
// size class and nothing about order beyond what AppendAttributes already says.
// A caller on a per-route path plans the AppendAttributes list instead
// (ai/rules/performance.md: Build is not for the hot path).
func (b *Builder) Build() []byte {
	if len(b.wire) > 0 {
		return b.wire
	}

	var scratch [builderInlineAttrs]Attribute
	attrs := b.AppendAttributes(scratch[:0])

	size := 0
	for _, attr := range attrs {
		size += AttrWireLen(attr)
	}

	buf := make([]byte, size) // result copy for a caller that owns no buffer
	off := 0
	for _, attr := range attrs {
		off += WriteAttrTo(attr, buf, off)
	}
	return buf[:off]
}

// IsEmpty returns true if no attributes have been set.
func (b *Builder) IsEmpty() bool {
	return b.origin == nil &&
		b.nextHop == nil &&
		b.localPref == nil &&
		b.med == nil &&
		b.asPath == nil &&
		len(b.communities) == 0 &&
		len(b.largeCommunities) == 0 &&
		len(b.extCommunities) == 0 &&
		!b.atomicAggregate &&
		b.aggregator == nil &&
		b.originatorID == nil &&
		len(b.clusterList) == 0 &&
		b.aigp == nil &&
		len(b.prefixSID) == 0 &&
		len(b.wire) == 0
}

// Reset clears all attributes.
func (b *Builder) Reset() {
	b.origin = nil
	b.nextHop = nil
	b.localPref = nil
	b.med = nil
	b.asPath = nil
	b.asPathASNs = nil
	b.communities = nil
	b.largeCommunities = nil
	b.extCommunities = nil
	b.atomicAggregate = false
	b.aggregator = nil
	b.originatorID = nil
	b.clusterList = nil
	b.aigp = nil
	b.prefixSID = nil
	b.wire = nil
}

// ToAttributes converts Builder state to []Attribute slice.
// This is a transition method for compatibility with code that expects parsed
// attributes. For true wire-first encoding, use AppendAttributes.
// Note: Does not include NEXT_HOP or AS_PATH (handled separately by reactor),
// and it substitutes ORIGIN=IGP when none was set, so it is NOT the emission
// list. AppendAttributes is.
func (b *Builder) ToAttributes() []Attribute {
	var result []Attribute

	// ORIGIN (always present, default IGP)
	if b.origin != nil {
		result = append(result, *b.origin)
	} else {
		result = append(result, OriginIGP)
	}

	// MED
	if b.med != nil {
		result = append(result, *b.med)
	}

	// LOCAL_PREF (filtered at send time for eBGP)
	if b.localPref != nil {
		result = append(result, *b.localPref)
	}

	// ATOMIC_AGGREGATE
	if b.atomicAggregate {
		result = append(result, AtomicAggregate{})
	}

	// AGGREGATOR
	if b.aggregator != nil {
		result = append(result, b.aggregator)
	}

	// COMMUNITY
	if len(b.communities) > 0 {
		result = append(result, b.communities)
	}

	// ORIGINATOR_ID
	if b.originatorID != nil {
		result = append(result, *b.originatorID)
	}

	// CLUSTER_LIST
	if len(b.clusterList) > 0 {
		result = append(result, b.clusterList)
	}

	// LARGE_COMMUNITY
	if len(b.largeCommunities) > 0 {
		result = append(result, b.largeCommunities)
	}

	// EXTENDED_COMMUNITIES
	if len(b.extCommunities) > 0 {
		result = append(result, b.extCommunities)
	}

	// AIGP
	if b.aigp != nil {
		result = append(result, b.aigp)
	}

	return result
}

// ToASPath returns the AS_PATH as an ASPath attribute.
// Returns nil if no AS_PATH was set.
func (b *Builder) ToASPath() *ASPath { return b.asPath }

// ASPathSlice returns a copy of the raw AS_PATH slice.
// Returns nil if no AS_PATH was set.
func (b *Builder) ASPathSlice() []uint32 {
	if len(b.asPathASNs) == 0 {
		return nil
	}
	result := make([]uint32, len(b.asPathASNs)) // result copy for the caller
	copy(result, b.asPathASNs)
	return result
}
