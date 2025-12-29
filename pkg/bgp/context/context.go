// Package context provides capability-dependent encoding parameters.
//
// EncodingContext captures negotiated capability state that affects wire encoding.
// It is created once per peer at session establishment and registered for ID-based
// fast comparison.
package context

import (
	"encoding/binary"
	"hash"
	"hash/fnv"
	"sort"

	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

// Family represents an AFI/SAFI combination.
// Matches capability.Family but defined here to avoid circular imports.
type Family struct {
	AFI  uint16
	SAFI uint8
}

// EncodingContext holds capability-dependent encoding parameters.
// Same structure for source (receive) and destination (send) contexts.
//
// Created once per peer at session establishment.
// Registered in global registry for ID assignment and deduplication.
type EncodingContext struct {
	// RFC 6793: Use 4-byte AS numbers when true.
	ASN4 bool

	// RFC 7911: ADD-PATH enabled per family.
	// Key is AFI/SAFI, value indicates whether path ID is included.
	AddPath map[Family]bool

	// RFC 8950: Extended next-hop encoding per family.
	// Allows IPv6 next-hop for IPv4 prefixes.
	ExtendedNextHop map[Family]bool

	// Session context for path attribute handling.
	IsIBGP  bool
	LocalAS uint32
	PeerAS  uint32
}

// Hash returns a deterministic 64-bit hash for deduplication.
// Identical contexts produce identical hashes.
//
// Note: nil maps and empty maps hash identically (both write nothing).
// This is semantically correct since both mean "no families configured".
func (ctx *EncodingContext) Hash() uint64 {
	h := fnv.New64a()

	// Fixed fields (order matters for determinism)
	// Note: hash.Hash.Write never returns an error per interface contract.
	if ctx.ASN4 {
		_, _ = h.Write([]byte{1})
	} else {
		_, _ = h.Write([]byte{0})
	}
	if ctx.IsIBGP {
		_, _ = h.Write([]byte{1})
	} else {
		_, _ = h.Write([]byte{0})
	}

	// ASNs
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], ctx.LocalAS)
	binary.BigEndian.PutUint32(buf[4:8], ctx.PeerAS)
	_, _ = h.Write(buf)

	// AddPath map (sorted for determinism)
	_, _ = h.Write([]byte{0xFF}) // separator
	hashFamilyMap(h, ctx.AddPath)

	// ExtendedNextHop map (sorted for determinism)
	_, _ = h.Write([]byte{0xFE}) // separator
	hashFamilyMap(h, ctx.ExtendedNextHop)

	return h.Sum64()
}

// hashFamilyMap writes map entries to hash in deterministic order.
func hashFamilyMap(h hash.Hash64, m map[Family]bool) {
	if m == nil {
		return
	}

	// Sort keys for determinism
	keys := make([]Family, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].AFI != keys[j].AFI {
			return keys[i].AFI < keys[j].AFI
		}
		return keys[i].SAFI < keys[j].SAFI
	})

	// Write each entry
	buf := make([]byte, 4)
	for _, k := range keys {
		binary.BigEndian.PutUint16(buf[0:2], k.AFI)
		buf[2] = k.SAFI
		if m[k] {
			buf[3] = 1
		} else {
			buf[3] = 0
		}
		_, _ = h.Write(buf)
	}
}

// AddPathFor returns whether ADD-PATH is enabled for the given family.
// Returns false if the map is nil or family is not present.
func (ctx *EncodingContext) AddPathFor(f Family) bool {
	if ctx.AddPath == nil {
		return false
	}
	return ctx.AddPath[f]
}

// ExtendedNextHopFor returns whether extended next-hop is enabled for the family.
// Returns false if the map is nil or family is not present.
func (ctx *EncodingContext) ExtendedNextHopFor(f Family) bool {
	if ctx.ExtendedNextHop == nil {
		return false
	}
	return ctx.ExtendedNextHop[f]
}

// ToPackContext creates an nlri.PackContext for the given family.
// Extracts relevant capability flags for NLRI encoding.
func (ctx *EncodingContext) ToPackContext(f Family) *nlri.PackContext {
	return &nlri.PackContext{
		ASN4:    ctx.ASN4,
		AddPath: ctx.AddPathFor(f),
	}
}
