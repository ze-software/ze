package attribute

import (
	"encoding/binary"
)

// Builder accumulates path attributes and produces wire-format bytes.
// Zero-copy friendly: builds directly into wire format without intermediate structs.
//
// Example usage:
//
//	b := NewBuilder()
//	b.SetOrigin(OriginIGP)
//	b.SetLocalPref(100)
//	b.AddCommunity(65000, 100)
//	wireBytes := b.Build()
type Builder struct {
	origin           *uint8
	localPref        *uint32
	med              *uint32
	asPath           []uint32
	communities      []uint32
	largeCommunities []LargeCommunity
	extCommunities   []ExtendedCommunity
	atomicAggregate  bool

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
	b.origin = &origin
	return b
}

// SetLocalPref sets the LOCAL_PREF attribute.
func (b *Builder) SetLocalPref(pref uint32) *Builder {
	b.localPref = &pref
	return b
}

// SetMED sets the MULTI_EXIT_DISC attribute.
func (b *Builder) SetMED(med uint32) *Builder {
	b.med = &med
	return b
}

// SetASPath sets the AS_PATH as a sequence of ASNs.
func (b *Builder) SetASPath(asns []uint32) *Builder {
	b.asPath = asns
	return b
}

// AddCommunity adds a standard community (RFC 1997).
func (b *Builder) AddCommunity(high, low uint16) *Builder {
	b.communities = append(b.communities, uint32(high)<<16|uint32(low))
	return b
}

// AddCommunityValue adds a community by its 32-bit value.
func (b *Builder) AddCommunityValue(community uint32) *Builder {
	b.communities = append(b.communities, community)
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

// SetAtomicAggregate sets the ATOMIC_AGGREGATE attribute.
func (b *Builder) SetAtomicAggregate(v bool) *Builder {
	b.atomicAggregate = v
	return b
}

// SetWire sets pre-built wire bytes (for forwarding).
// When wire is set, Build() returns it directly.
func (b *Builder) SetWire(wire []byte) *Builder {
	b.wire = wire
	return b
}

// Build produces the wire-format bytes for all attributes.
// Attributes are ordered per RFC 4271 (by type code).
func (b *Builder) Build() []byte {
	// Fast path: pre-built wire bytes
	if len(b.wire) > 0 {
		return b.wire
	}

	// Calculate total size
	size := 0

	// ORIGIN (always present, default IGP)
	size += 4 // header(3) + value(1)

	// AS_PATH (max 255 ASNs per segment = 1022 bytes, fits in uint16)
	if len(b.asPath) > 0 {
		asPathLen := 2 + len(b.asPath)*4 // type(1) + count(1) + ASNs
		if asPathLen > 255 {
			size += 4 + asPathLen // extended header
		} else {
			size += 3 + asPathLen
		}
	}

	// MED
	if b.med != nil {
		size += 7 // header(3) + value(4)
	}

	// LOCAL_PREF
	if b.localPref != nil {
		size += 7 // header(3) + value(4)
	}

	// ATOMIC_AGGREGATE
	if b.atomicAggregate {
		size += 3 // header(3) + no value
	}

	// COMMUNITY
	if len(b.communities) > 0 {
		commLen := len(b.communities) * 4
		if commLen > 255 {
			size += 4 + commLen
		} else {
			size += 3 + commLen
		}
	}

	// EXTENDED_COMMUNITIES
	if len(b.extCommunities) > 0 {
		extLen := len(b.extCommunities) * 8
		if extLen > 255 {
			size += 4 + extLen
		} else {
			size += 3 + extLen
		}
	}

	// LARGE_COMMUNITY
	if len(b.largeCommunities) > 0 {
		largeLen := len(b.largeCommunities) * 12
		if largeLen > 255 {
			size += 4 + largeLen
		} else {
			size += 3 + largeLen
		}
	}

	// Build buffer
	buf := make([]byte, size)
	off := 0

	// ORIGIN (type 1) - well-known mandatory
	origin := uint8(0) // IGP default
	if b.origin != nil {
		origin = *b.origin
	}
	buf[off] = 0x40 // Transitive
	buf[off+1] = 1  // ORIGIN
	buf[off+2] = 1  // Length
	buf[off+3] = origin
	off += 4

	// AS_PATH (type 2) - well-known mandatory
	if len(b.asPath) > 0 {
		asPathLen := 2 + len(b.asPath)*4
		if asPathLen > 255 {
			buf[off] = 0x50                                            // Transitive + Extended Length
			buf[off+1] = 2                                             // AS_PATH
			binary.BigEndian.PutUint16(buf[off+2:], uint16(asPathLen)) //nolint:gosec // max 255*4+2=1022, fits uint16
			off += 4
		} else {
			buf[off] = 0x40 // Transitive
			buf[off+1] = 2  // AS_PATH
			buf[off+2] = byte(asPathLen)
			off += 3
		}
		buf[off] = byte(ASSequence)
		buf[off+1] = byte(len(b.asPath))
		off += 2
		for _, asn := range b.asPath {
			binary.BigEndian.PutUint32(buf[off:], asn)
			off += 4
		}
	}

	// MED (type 4) - optional non-transitive
	if b.med != nil {
		buf[off] = 0x80 // Optional
		buf[off+1] = 4  // MED
		buf[off+2] = 4  // Length
		binary.BigEndian.PutUint32(buf[off+3:], *b.med)
		off += 7
	}

	// LOCAL_PREF (type 5) - well-known (for iBGP)
	if b.localPref != nil {
		buf[off] = 0x40 // Transitive
		buf[off+1] = 5  // LOCAL_PREF
		buf[off+2] = 4  // Length
		binary.BigEndian.PutUint32(buf[off+3:], *b.localPref)
		off += 7
	}

	// ATOMIC_AGGREGATE (type 6) - well-known discretionary
	if b.atomicAggregate {
		buf[off] = 0x40 // Transitive
		buf[off+1] = 6  // ATOMIC_AGGREGATE
		buf[off+2] = 0  // Length (no value)
		off += 3
	}

	// COMMUNITY (type 8) - optional transitive
	if len(b.communities) > 0 {
		commLen := len(b.communities) * 4
		if commLen > 255 {
			buf[off] = 0xD0                                          // Optional + Transitive + Extended
			buf[off+1] = 8                                           // COMMUNITY
			binary.BigEndian.PutUint16(buf[off+2:], uint16(commLen)) //nolint:gosec // bounded by slice capacity
			off += 4
		} else {
			buf[off] = 0xC0 // Optional + Transitive
			buf[off+1] = 8  // COMMUNITY
			buf[off+2] = byte(commLen)
			off += 3
		}
		for _, c := range b.communities {
			binary.BigEndian.PutUint32(buf[off:], c)
			off += 4
		}
	}

	// EXTENDED_COMMUNITIES (type 16) - optional transitive
	if len(b.extCommunities) > 0 {
		extLen := len(b.extCommunities) * 8
		if extLen > 255 {
			buf[off] = 0xD0                                         // Optional + Transitive + Extended
			buf[off+1] = 16                                         // EXTENDED_COMMUNITIES
			binary.BigEndian.PutUint16(buf[off+2:], uint16(extLen)) //nolint:gosec // bounded by slice capacity
			off += 4
		} else {
			buf[off] = 0xC0 // Optional + Transitive
			buf[off+1] = 16 // EXTENDED_COMMUNITIES
			buf[off+2] = byte(extLen)
			off += 3
		}
		for _, ec := range b.extCommunities {
			copy(buf[off:], ec[:])
			off += 8
		}
	}

	// LARGE_COMMUNITY (type 32) - optional transitive
	if len(b.largeCommunities) > 0 {
		largeLen := len(b.largeCommunities) * 12
		if largeLen > 255 {
			buf[off] = 0xD0                                           // Optional + Transitive + Extended
			buf[off+1] = 32                                           // LARGE_COMMUNITY
			binary.BigEndian.PutUint16(buf[off+2:], uint16(largeLen)) //nolint:gosec // bounded by slice capacity
			off += 4
		} else {
			buf[off] = 0xC0 // Optional + Transitive
			buf[off+1] = 32 // LARGE_COMMUNITY
			buf[off+2] = byte(largeLen)
			off += 3
		}
		for _, lc := range b.largeCommunities {
			binary.BigEndian.PutUint32(buf[off:], lc.GlobalAdmin)
			binary.BigEndian.PutUint32(buf[off+4:], lc.LocalData1)
			binary.BigEndian.PutUint32(buf[off+8:], lc.LocalData2)
			off += 12
		}
	}

	return buf[:off]
}

// IsEmpty returns true if no attributes have been set.
func (b *Builder) IsEmpty() bool {
	return b.origin == nil &&
		b.localPref == nil &&
		b.med == nil &&
		len(b.asPath) == 0 &&
		len(b.communities) == 0 &&
		len(b.largeCommunities) == 0 &&
		len(b.extCommunities) == 0 &&
		!b.atomicAggregate &&
		len(b.wire) == 0
}

// Reset clears all attributes.
func (b *Builder) Reset() {
	b.origin = nil
	b.localPref = nil
	b.med = nil
	b.asPath = nil
	b.communities = nil
	b.largeCommunities = nil
	b.extCommunities = nil
	b.atomicAggregate = false
	b.wire = nil
}
