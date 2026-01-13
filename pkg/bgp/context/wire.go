// Package context provides capability-dependent encoding parameters.
//
// WireContext captures negotiated capability state that affects wire encoding.
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

// Direction indicates receive or send context.
type Direction uint8

const (
	// DirectionRecv is for parsing routes FROM peer.
	DirectionRecv Direction = iota
	// DirectionSend is for encoding routes TO peer.
	DirectionSend
)

// WireContext holds capability-dependent encoding parameters.
// References Identity and Encoding sub-components (no copy).
// Derives addPath map per direction.
//
// Created once per peer per direction at session establishment.
// Registered in global registry for ID assignment and deduplication.
type WireContext struct {
	// References to sub-components (no copy)
	identity *capability.PeerIdentity
	encoding *capability.EncodingCaps

	// Direction-specific derived data
	direction Direction
	addPath   map[nlri.Family]bool // Derived from encoding.AddPathMode + direction

	// Cached hash for registry deduplication
	hash uint64
}

// NewWireContext creates a WireContext from sub-components.
// The addPath map is derived from encoding.AddPathMode based on direction.
//
// RFC 7911 Section 4: ADD-PATH mode is asymmetric
//   - Receive: check for Receive or Both mode
//   - Send: check for Send or Both mode
func NewWireContext(identity *capability.PeerIdentity, encoding *capability.EncodingCaps, dir Direction) *WireContext {
	ctx := &WireContext{
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
func (w *WireContext) Direction() Direction {
	return w.direction
}

// ASN4 returns true if 4-byte ASN is negotiated.
// RFC 6793: Use 4-byte AS numbers when true.
func (w *WireContext) ASN4() bool {
	if w.encoding == nil {
		return false
	}
	return w.encoding.ASN4
}

// Families returns all negotiated families.
func (w *WireContext) Families() []capability.Family {
	if w.encoding == nil {
		return nil
	}
	return w.encoding.Families
}

// LocalASN returns the local AS number.
func (w *WireContext) LocalASN() uint32 {
	if w.identity == nil {
		return 0
	}
	return w.identity.LocalASN
}

// PeerASN returns the peer AS number.
func (w *WireContext) PeerASN() uint32 {
	if w.identity == nil {
		return 0
	}
	return w.identity.PeerASN
}

// IsIBGP returns true if this is an iBGP session.
func (w *WireContext) IsIBGP() bool {
	if w.identity == nil {
		return false
	}
	return w.identity.IsIBGP()
}

// AddPath returns whether ADD-PATH is enabled for the given family in this direction.
// RFC 7911: Returns true if we can receive/send path IDs for this family.
func (w *WireContext) AddPath(f nlri.Family) bool {
	if w.addPath == nil {
		return false
	}
	return w.addPath[f]
}

// ExtendedNextHopFor returns the next-hop AFI for the given family.
// RFC 8950: Returns the next-hop AFI if extended next-hop is negotiated.
// Returns 0 if not negotiated.
func (w *WireContext) ExtendedNextHopFor(f nlri.Family) nlri.AFI {
	if w.encoding == nil || w.encoding.ExtendedNextHop == nil {
		return 0
	}
	return w.encoding.ExtendedNextHop[f]
}

// ToPackContext creates an nlri.PackContext for the given family.
// Extracts relevant capability flags for NLRI encoding.
func (w *WireContext) ToPackContext(f nlri.Family) *nlri.PackContext {
	return &nlri.PackContext{
		ASN4:    w.ASN4(),
		AddPath: w.AddPath(f),
	}
}

// Hash returns a deterministic 64-bit hash for deduplication.
// Computed once at creation, cached for performance.
func (w *WireContext) Hash() uint64 {
	return w.hash
}

// computeHash calculates the hash for this context.
func (w *WireContext) computeHash() uint64 {
	h := fnv.New64a()

	// Direction (affects ADD-PATH interpretation)
	// Note: hash.Hash.Write never returns an error per interface contract.
	_, _ = h.Write([]byte{byte(w.direction)})

	// ASN4
	if w.ASN4() {
		_, _ = h.Write([]byte{1})
	} else {
		_, _ = h.Write([]byte{0})
	}

	// IsIBGP
	if w.IsIBGP() {
		_, _ = h.Write([]byte{1})
	} else {
		_, _ = h.Write([]byte{0})
	}

	// ASNs
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], w.LocalASN())
	binary.BigEndian.PutUint32(buf[4:8], w.PeerASN())
	_, _ = h.Write(buf)

	// AddPath map (sorted for determinism)
	_, _ = h.Write([]byte{0xFF}) // separator
	hashFamilyBoolMapWire(h, w.addPath)

	// ExtendedNextHop map (sorted for determinism)
	_, _ = h.Write([]byte{0xFE}) // separator
	if w.encoding != nil {
		hashFamilyAFIMapWire(h, w.encoding.ExtendedNextHop)
	}

	return h.Sum64()
}

// hashFamilyBoolMapWire writes map entries to hash in deterministic order.
func hashFamilyBoolMapWire(h hash.Hash64, m map[nlri.Family]bool) {
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

// hashFamilyAFIMapWire writes ExtendedNextHop map entries to hash in deterministic order.
func hashFamilyAFIMapWire(h hash.Hash64, m map[capability.Family]capability.AFI) {
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
func (w *WireContext) Identity() *capability.PeerIdentity {
	return w.identity
}

// Encoding returns the referenced EncodingCaps (for consumers that need it).
func (w *WireContext) Encoding() *capability.EncodingCaps {
	return w.encoding
}
