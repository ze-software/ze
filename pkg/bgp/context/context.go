// Package context provides capability-dependent encoding parameters.
//
// EncodingContext captures negotiated capability state that affects wire encoding.
// It references sub-components from capability negotiation for zero duplication.
package context

import (
	"encoding/binary"
	"hash"
	"hash/fnv"
	"sort"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/capability"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
)

// Family is an alias for nlri.Family. Use nlri.Family directly in new code.
type Family = nlri.Family

// Direction indicates receive or send context.
type Direction uint8

const (
	// DirectionRecv is for parsing routes FROM peer.
	DirectionRecv Direction = iota
	// DirectionSend is for encoding routes TO peer.
	DirectionSend
)

// EncodingContext holds capability-dependent encoding parameters.
// References Identity and Encoding sub-components (no copy).
// Derives addPath map per direction.
//
// Created once per peer per direction at session establishment.
// Registered in global registry for ID assignment and deduplication.
type EncodingContext struct {
	// References to sub-components (no copy)
	identity *capability.PeerIdentity
	encoding *capability.EncodingCaps

	// Direction-specific derived data
	direction Direction
	addPath   map[nlri.Family]bool // Derived from encoding.AddPathMode + direction

	// Cached hash for registry deduplication
	hash uint64
}

// NewEncodingContext creates an EncodingContext from sub-components.
// The addPath map is derived from encoding.AddPathMode based on direction.
//
// RFC 7911 Section 4: ADD-PATH mode is asymmetric
//   - Receive: check for Receive or Both mode
//   - Send: check for Send or Both mode
func NewEncodingContext(identity *capability.PeerIdentity, encoding *capability.EncodingCaps, dir Direction) *EncodingContext {
	ctx := &EncodingContext{
		identity:  identity,
		encoding:  encoding,
		direction: dir,
		addPath:   make(map[nlri.Family]bool),
	}

	// Derive addPath map based on direction
	if encoding != nil && encoding.AddPathMode != nil {
		for f, mode := range encoding.AddPathMode {
			var enabled bool
			switch dir {
			case DirectionRecv:
				// RFC 7911: Can receive if mode is Receive or Both
				enabled = mode == capability.AddPathReceive || mode == capability.AddPathBoth
			case DirectionSend:
				// RFC 7911: Can send if mode is Send or Both
				enabled = mode == capability.AddPathSend || mode == capability.AddPathBoth
			}
			if enabled {
				ctx.addPath[f] = true
			}
		}
	}

	// Compute and cache hash
	ctx.hash = ctx.computeHash()

	return ctx
}

// Direction returns the context direction (Recv or Send).
func (c *EncodingContext) Direction() Direction {
	return c.direction
}

// ASN4 returns true if 4-byte ASN is negotiated.
// RFC 6793: Use 4-byte AS numbers when true.
func (c *EncodingContext) ASN4() bool {
	if c.encoding == nil {
		return false
	}
	return c.encoding.ASN4
}

// Families returns all negotiated families.
func (c *EncodingContext) Families() []capability.Family {
	if c.encoding == nil {
		return nil
	}
	return c.encoding.Families
}

// LocalASN returns the local AS number.
func (c *EncodingContext) LocalASN() uint32 {
	if c.identity == nil {
		return 0
	}
	return c.identity.LocalASN
}

// PeerASN returns the peer AS number.
func (c *EncodingContext) PeerASN() uint32 {
	if c.identity == nil {
		return 0
	}
	return c.identity.PeerASN
}

// IsIBGP returns true if this is an iBGP session.
func (c *EncodingContext) IsIBGP() bool {
	if c.identity == nil {
		return false
	}
	return c.identity.IsIBGP()
}

// AddPath returns whether ADD-PATH is enabled for the given family in this direction.
// RFC 7911: Returns true if we can receive/send path IDs for this family.
func (c *EncodingContext) AddPath(f nlri.Family) bool {
	if c.addPath == nil {
		return false
	}
	return c.addPath[f]
}

// AddPathFor is an alias for AddPath for API compatibility.
func (c *EncodingContext) AddPathFor(f nlri.Family) bool {
	return c.AddPath(f)
}

// ExtendedNextHopFor returns the next-hop AFI for the given family.
// RFC 8950: Returns the next-hop AFI if extended next-hop is negotiated.
// Returns 0 if not negotiated.
func (c *EncodingContext) ExtendedNextHopFor(f nlri.Family) nlri.AFI {
	if c.encoding == nil || c.encoding.ExtendedNextHop == nil {
		return 0
	}
	return c.encoding.ExtendedNextHop[f]
}

// ToPackContext creates an nlri.PackContext for the given family.
// Extracts relevant capability flags for NLRI encoding.
func (c *EncodingContext) ToPackContext(f nlri.Family) *nlri.PackContext {
	return &nlri.PackContext{
		ASN4:    c.ASN4(),
		AddPath: c.AddPath(f),
	}
}

// Hash returns a deterministic 64-bit hash for deduplication.
// Computed once at creation, cached for performance.
func (c *EncodingContext) Hash() uint64 {
	return c.hash
}

// computeHash calculates the hash for this context.
func (c *EncodingContext) computeHash() uint64 {
	h := fnv.New64a()

	// Direction (affects ADD-PATH interpretation)
	// Note: hash.Hash.Write never returns an error per interface contract.
	_, _ = h.Write([]byte{byte(c.direction)})

	// ASN4
	if c.ASN4() {
		_, _ = h.Write([]byte{1})
	} else {
		_, _ = h.Write([]byte{0})
	}

	// IsIBGP
	if c.IsIBGP() {
		_, _ = h.Write([]byte{1})
	} else {
		_, _ = h.Write([]byte{0})
	}

	// ASNs
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], c.LocalASN())
	binary.BigEndian.PutUint32(buf[4:8], c.PeerASN())
	_, _ = h.Write(buf)

	// AddPath map (sorted for determinism)
	_, _ = h.Write([]byte{0xFF}) // separator
	hashFamilyBoolMap(h, c.addPath)

	// ExtendedNextHop map (sorted for determinism)
	_, _ = h.Write([]byte{0xFE}) // separator
	if c.encoding != nil {
		hashFamilyAFIMap(h, c.encoding.ExtendedNextHop)
	}

	return h.Sum64()
}

// hashFamilyBoolMap writes map entries to hash in deterministic order.
func hashFamilyBoolMap(h hash.Hash64, m map[nlri.Family]bool) {
	if m == nil {
		return
	}

	// Sort keys for determinism
	keys := make([]nlri.Family, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return nlri.FamilyLess(keys[i], keys[j])
	})

	// Write each entry
	buf := make([]byte, 4)
	for _, k := range keys {
		binary.BigEndian.PutUint16(buf[0:2], uint16(k.AFI))
		buf[2] = uint8(k.SAFI)
		if m[k] {
			buf[3] = 1
		} else {
			buf[3] = 0
		}
		_, _ = h.Write(buf)
	}
}

// hashFamilyAFIMap writes ExtendedNextHop map entries to hash in deterministic order.
func hashFamilyAFIMap(h hash.Hash64, m map[capability.Family]capability.AFI) {
	if m == nil {
		return
	}

	// Sort keys for determinism
	keys := make([]capability.Family, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return nlri.FamilyLess(keys[i], keys[j])
	})

	// Write each entry (family + next-hop AFI)
	buf := make([]byte, 5)
	for _, k := range keys {
		binary.BigEndian.PutUint16(buf[0:2], uint16(k.AFI))
		buf[2] = uint8(k.SAFI)
		binary.BigEndian.PutUint16(buf[3:5], uint16(m[k]))
		_, _ = h.Write(buf)
	}
}

// Identity returns the referenced PeerIdentity (for consumers that need it).
func (c *EncodingContext) Identity() *capability.PeerIdentity {
	return c.identity
}

// Encoding returns the referenced EncodingCaps (for consumers that need it).
func (c *EncodingContext) Encoding() *capability.EncodingCaps {
	return c.encoding
}

// EncodingContextForASN4 creates a minimal EncodingContext with just ASN4 set.
// Use this for attribute encoding where ADD-PATH and direction don't matter.
func EncodingContextForASN4(asn4 bool) *EncodingContext {
	return NewEncodingContext(nil, &capability.EncodingCaps{ASN4: asn4}, DirectionSend)
}

// EncodingContextWithAddPath creates an EncodingContext with ASN4 and ADD-PATH settings.
// Direction is set to DirectionSend since this is typically used for encoding.
func EncodingContextWithAddPath(asn4 bool, addPath map[nlri.Family]bool) *EncodingContext {
	addPathMode := make(map[capability.Family]capability.AddPathMode)
	for f, enabled := range addPath {
		if enabled {
			addPathMode[f] = capability.AddPathSend // Mode that enables Send direction
		}
	}
	return NewEncodingContext(nil, &capability.EncodingCaps{
		ASN4:        asn4,
		AddPathMode: addPathMode,
	}, DirectionSend)
}
