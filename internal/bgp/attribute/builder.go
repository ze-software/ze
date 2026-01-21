package attribute

import (
	"encoding/binary"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/bgp/wire"
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
	nextHop          *[4]byte // IPv4 next-hop (type 3)
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

// SetNextHop sets the NEXT_HOP attribute from raw bytes (IPv4 only, type code 3).
// RFC 4271 Section 5.1.3 - well-known mandatory for IPv4 unicast.
// For IPv6, use MP_REACH_NLRI instead.
// The bytes are copied as-is (network byte order).
func (b *Builder) SetNextHop(ip [4]byte) *Builder {
	b.nextHop = &ip
	return b
}

// SetNextHopAddr sets the NEXT_HOP attribute from netip.Addr.
// Only IPv4 addresses are valid; IPv6 returns the builder unchanged.
func (b *Builder) SetNextHopAddr(addr netip.Addr) *Builder {
	if !addr.Is4() {
		return b
	}
	ip := addr.As4()
	b.nextHop = &ip
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

// Len returns the wire-format length in bytes.
// Use this to pre-allocate buffers before calling WriteTo.
func (b *Builder) Len() int {
	if len(b.wire) > 0 {
		return len(b.wire)
	}

	size := 0
	if b.origin != nil {
		size += 4 // ORIGIN
	}

	if len(b.asPath) > 0 {
		// RFC 4271: Max 255 ASNs per segment, split if needed
		var asPathLen int
		remaining := len(b.asPath)
		for remaining > 0 {
			chunk := remaining
			if chunk > MaxASPathSegmentLength {
				chunk = MaxASPathSegmentLength
			}
			asPathLen += 2 + chunk*4 // type(1) + count(1) + asns
			remaining -= chunk
		}
		if asPathLen > 255 {
			size += 4 + asPathLen
		} else {
			size += 3 + asPathLen
		}
	}

	if b.nextHop != nil {
		size += 7
	}
	if b.med != nil {
		size += 7
	}
	if b.localPref != nil {
		size += 7
	}
	if b.atomicAggregate {
		size += 3
	}

	if len(b.communities) > 0 {
		commLen := len(b.communities) * 4
		if commLen > 255 {
			size += 4 + commLen
		} else {
			size += 3 + commLen
		}
	}

	if len(b.extCommunities) > 0 {
		extLen := len(b.extCommunities) * 8
		if extLen > 255 {
			size += 4 + extLen
		} else {
			size += 3 + extLen
		}
	}

	if len(b.largeCommunities) > 0 {
		largeLen := len(b.largeCommunities) * 12
		if largeLen > 255 {
			size += 4 + largeLen
		} else {
			size += 3 + largeLen
		}
	}

	return size
}

// WriteTo writes wire-format bytes to buf, returning bytes written.
// The buffer must be at least Len() bytes. Use this for zero-allocation encoding.
// Returns the number of bytes written.
func (b *Builder) WriteTo(buf []byte) int {
	if len(b.wire) > 0 {
		return copy(buf, b.wire)
	}

	off := 0

	// ORIGIN (type 1) - only if set
	if b.origin != nil {
		buf[off] = 0x40
		buf[off+1] = 1
		buf[off+2] = 1
		buf[off+3] = *b.origin
		off += 4
	}

	// AS_PATH (type 2)
	// RFC 4271: Max 255 ASNs per segment, split if needed
	if len(b.asPath) > 0 {
		// Calculate value length with segment splitting
		var asPathLen int
		remaining := len(b.asPath)
		for remaining > 0 {
			chunk := remaining
			if chunk > MaxASPathSegmentLength {
				chunk = MaxASPathSegmentLength
			}
			asPathLen += 2 + chunk*4 // type(1) + count(1) + asns
			remaining -= chunk
		}

		// Write header with extended length if needed
		if asPathLen > 255 {
			buf[off] = 0x50                                            // Transitive + Extended Length
			buf[off+1] = 2                                             // AS_PATH
			binary.BigEndian.PutUint16(buf[off+2:], uint16(asPathLen)) //nolint:gosec // bounded
			off += 4
		} else {
			buf[off] = 0x40 // Transitive
			buf[off+1] = 2  // AS_PATH
			buf[off+2] = byte(asPathLen)
			off += 3
		}

		// Write segments, splitting at 255 ASNs
		remaining = len(b.asPath)
		idx := 0
		for remaining > 0 {
			chunk := remaining
			if chunk > MaxASPathSegmentLength {
				chunk = MaxASPathSegmentLength
			}
			buf[off] = byte(ASSequence)
			buf[off+1] = byte(chunk)
			off += 2
			for i := 0; i < chunk; i++ {
				binary.BigEndian.PutUint32(buf[off:], b.asPath[idx+i])
				off += 4
			}
			idx += chunk
			remaining -= chunk
		}
	}

	// NEXT_HOP (type 3)
	if b.nextHop != nil {
		buf[off] = 0x40
		buf[off+1] = 3
		buf[off+2] = 4
		copy(buf[off+3:], b.nextHop[:])
		off += 7
	}

	// MED (type 4)
	if b.med != nil {
		buf[off] = 0x80
		buf[off+1] = 4
		buf[off+2] = 4
		binary.BigEndian.PutUint32(buf[off+3:], *b.med)
		off += 7
	}

	// LOCAL_PREF (type 5)
	if b.localPref != nil {
		buf[off] = 0x40
		buf[off+1] = 5
		buf[off+2] = 4
		binary.BigEndian.PutUint32(buf[off+3:], *b.localPref)
		off += 7
	}

	// ATOMIC_AGGREGATE (type 6)
	if b.atomicAggregate {
		buf[off] = 0x40
		buf[off+1] = 6
		buf[off+2] = 0
		off += 3
	}

	// COMMUNITY (type 8)
	if len(b.communities) > 0 {
		commLen := len(b.communities) * 4
		if commLen > 255 {
			buf[off] = 0xD0
			buf[off+1] = 8
			binary.BigEndian.PutUint16(buf[off+2:], uint16(commLen)) //nolint:gosec // bounded
			off += 4
		} else {
			buf[off] = 0xC0
			buf[off+1] = 8
			buf[off+2] = byte(commLen)
			off += 3
		}
		for _, c := range b.communities {
			binary.BigEndian.PutUint32(buf[off:], c)
			off += 4
		}
	}

	// EXTENDED_COMMUNITIES (type 16)
	if len(b.extCommunities) > 0 {
		extLen := len(b.extCommunities) * 8
		if extLen > 255 {
			buf[off] = 0xD0
			buf[off+1] = 16
			binary.BigEndian.PutUint16(buf[off+2:], uint16(extLen)) //nolint:gosec // bounded
			off += 4
		} else {
			buf[off] = 0xC0
			buf[off+1] = 16
			buf[off+2] = byte(extLen)
			off += 3
		}
		for _, ec := range b.extCommunities {
			copy(buf[off:], ec[:])
			off += 8
		}
	}

	// LARGE_COMMUNITY (type 32)
	if len(b.largeCommunities) > 0 {
		largeLen := len(b.largeCommunities) * 12
		if largeLen > 255 {
			buf[off] = 0xD0
			buf[off+1] = 32
			binary.BigEndian.PutUint16(buf[off+2:], uint16(largeLen)) //nolint:gosec // bounded
			off += 4
		} else {
			buf[off] = 0xC0
			buf[off+1] = 32
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

	return off
}

// CheckedWriteTo validates capacity before writing.
func (b *Builder) CheckedWriteTo(buf []byte) (int, error) {
	needed := b.Len()
	if len(buf) < needed {
		return 0, wire.ErrBufferTooSmall
	}
	return b.WriteTo(buf), nil
}

// Build produces the wire-format bytes for all attributes.
// Attributes are ordered per RFC 4271 (by type code).
// For zero-allocation encoding, use Len() + WriteTo() instead.
func (b *Builder) Build() []byte {
	if len(b.wire) > 0 {
		return b.wire
	}

	buf := make([]byte, b.Len())
	b.WriteTo(buf)
	return buf
}

// IsEmpty returns true if no attributes have been set.
func (b *Builder) IsEmpty() bool {
	return b.origin == nil &&
		b.nextHop == nil &&
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
	b.nextHop = nil
	b.localPref = nil
	b.med = nil
	b.asPath = nil
	b.communities = nil
	b.largeCommunities = nil
	b.extCommunities = nil
	b.atomicAggregate = false
	b.wire = nil
}

// ToAttributes converts Builder state to []Attribute slice.
// This is a transition method for compatibility with code that expects parsed
// attributes. For true wire-first encoding, use Build() to get wire bytes directly.
// Note: Does not include NEXT_HOP or AS_PATH (handled separately by reactor).
func (b *Builder) ToAttributes() []Attribute {
	var result []Attribute

	// ORIGIN (always present, default IGP)
	if b.origin != nil {
		result = append(result, Origin(*b.origin))
	} else {
		result = append(result, OriginIGP)
	}

	// MED
	if b.med != nil {
		result = append(result, MED(*b.med))
	}

	// LOCAL_PREF (filtered at send time for eBGP)
	if b.localPref != nil {
		result = append(result, LocalPref(*b.localPref))
	}

	// ATOMIC_AGGREGATE
	if b.atomicAggregate {
		result = append(result, AtomicAggregate{})
	}

	// COMMUNITY
	if len(b.communities) > 0 {
		comms := make(Communities, len(b.communities))
		for i, c := range b.communities {
			comms[i] = Community(c)
		}
		result = append(result, comms)
	}

	// LARGE_COMMUNITY
	if len(b.largeCommunities) > 0 {
		result = append(result, LargeCommunities(b.largeCommunities))
	}

	// EXTENDED_COMMUNITIES
	if len(b.extCommunities) > 0 {
		result = append(result, ExtendedCommunities(b.extCommunities))
	}

	return result
}

// ToASPath returns the AS_PATH as an ASPath attribute.
// Returns nil if no AS_PATH was set.
func (b *Builder) ToASPath() *ASPath {
	if len(b.asPath) == 0 {
		return nil
	}
	return &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: b.asPath},
		},
	}
}

// ASPathSlice returns a copy of the raw AS_PATH slice.
// Returns nil if no AS_PATH was set.
func (b *Builder) ASPathSlice() []uint32 {
	if len(b.asPath) == 0 {
		return nil
	}
	result := make([]uint32, len(b.asPath))
	copy(result, b.asPath)
	return result
}
