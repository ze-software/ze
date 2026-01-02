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

// Family is an alias for nlri.Family. Use nlri.Family directly in new code.
type Family = nlri.Family

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
	AddPath map[nlri.Family]bool

	// RFC 8950: Extended next-hop encoding per family.
	// Value is the next-hop AFI (e.g., AFIIPv6 for IPv4 prefix with IPv6 NH).
	// Zero value means not enabled.
	ExtendedNextHop map[nlri.Family]nlri.AFI

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
	hashFamilyBoolMap(h, ctx.AddPath)

	// ExtendedNextHop map (sorted for determinism)
	_, _ = h.Write([]byte{0xFE}) // separator
	hashFamilyAFIMap(h, ctx.ExtendedNextHop)

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
func hashFamilyAFIMap(h hash.Hash64, m map[nlri.Family]nlri.AFI) {
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

	// Write each entry (family + next-hop AFI)
	buf := make([]byte, 5)
	for _, k := range keys {
		binary.BigEndian.PutUint16(buf[0:2], uint16(k.AFI))
		buf[2] = uint8(k.SAFI)
		binary.BigEndian.PutUint16(buf[3:5], uint16(m[k]))
		_, _ = h.Write(buf)
	}
}

// AddPathFor returns whether ADD-PATH is enabled for the given family.
// Returns false if the map is nil or family is not present.
func (ctx *EncodingContext) AddPathFor(f nlri.Family) bool {
	if ctx.AddPath == nil {
		return false
	}
	return ctx.AddPath[f]
}

// ExtendedNextHopFor returns the next-hop AFI for the given family.
// Returns 0 if extended next-hop is not enabled for this family.
// Example: ExtendedNextHopFor(IPv4Unicast) returns AFIIPv6 if IPv6 NH is enabled.
func (ctx *EncodingContext) ExtendedNextHopFor(f nlri.Family) nlri.AFI {
	if ctx.ExtendedNextHop == nil {
		return 0
	}
	return ctx.ExtendedNextHop[f]
}

// ToPackContext creates an nlri.PackContext for the given family.
// Extracts relevant capability flags for NLRI encoding.
func (ctx *EncodingContext) ToPackContext(f nlri.Family) *nlri.PackContext {
	return &nlri.PackContext{
		ASN4:    ctx.ASN4,
		AddPath: ctx.AddPathFor(f),
	}
}
