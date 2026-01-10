// Package rib implements the BGP Routing Information Base.
//
// Key innovation: AS-PATH is treated as part of route identity (like ADD-PATH
// path-id), not as a regular attribute. This enables better attribute
// deduplication when routes share all attributes except AS-PATH.
package rib

import (
	"encoding/binary"
	"hash/fnv"
	"net/netip"
	"sync/atomic"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
)

// Route represents a BGP route with AS-PATH as part of identity.
//
// Novel approach: AS-PATH is stored separately and included in the route
// index, allowing routes with identical NLRI but different AS-PATHs to
// coexist (route diversity) while sharing other attributes.
type Route struct {
	nlri       nlri.NLRI
	nextHop    netip.Addr
	attributes []attribute.Attribute
	asPath     *attribute.ASPath

	// Reference counting for memory management
	refCount atomic.Int32

	// Cached index for fast lookup
	indexCache []byte

	// Wire cache: enables zero-copy forwarding when contexts match.
	// wireBytes contains the original packed path attributes.
	// nlriWireBytes contains the original packed NLRI.
	// sourceCtxID identifies the encoding context (for compatibility check).
	wireBytes     []byte
	nlriWireBytes []byte
	sourceCtxID   bgpctx.ContextID
}

// NewRoute creates a new route without explicit AS-PATH.
// AS-PATH should be extracted from attributes if present.
func NewRoute(n nlri.NLRI, nextHop netip.Addr, attrs []attribute.Attribute) *Route {
	r := &Route{
		nlri:       n,
		nextHop:    nextHop,
		attributes: attrs,
	}
	r.refCount.Store(1)
	return r
}

// NewRouteWithASPath creates a new route with explicit AS-PATH.
// The AS-PATH is stored separately for indexing purposes.
func NewRouteWithASPath(n nlri.NLRI, nextHop netip.Addr, attrs []attribute.Attribute, asPath *attribute.ASPath) *Route {
	r := &Route{
		nlri:       n,
		nextHop:    nextHop,
		attributes: attrs,
		asPath:     asPath,
	}
	r.refCount.Store(1)
	return r
}

// NewRouteWithWireCache creates a route with cached attribute wire bytes.
// Used when receiving routes - store original bytes for potential zero-copy forwarding.
//
// Note: wireBytes is stored by reference, not copied. The caller must ensure
// the slice is not modified after passing it to this function.
func NewRouteWithWireCache(
	n nlri.NLRI,
	nextHop netip.Addr,
	attrs []attribute.Attribute,
	asPath *attribute.ASPath,
	wireBytes []byte,
	sourceCtxID bgpctx.ContextID,
) *Route {
	r := &Route{
		nlri:        n,
		nextHop:     nextHop,
		attributes:  attrs,
		asPath:      asPath,
		wireBytes:   wireBytes,
		sourceCtxID: sourceCtxID,
	}
	r.refCount.Store(1)
	return r
}

// NewRouteWithWireCacheFull creates a route with both attribute and NLRI wire caches.
// Used when receiving routes with full wire preservation for zero-copy forwarding.
//
// Note: Both wireBytes and nlriWireBytes are stored by reference, not copied.
func NewRouteWithWireCacheFull(
	n nlri.NLRI,
	nextHop netip.Addr,
	attrs []attribute.Attribute,
	asPath *attribute.ASPath,
	wireBytes []byte,
	nlriWireBytes []byte,
	sourceCtxID bgpctx.ContextID,
) *Route {
	r := &Route{
		nlri:          n,
		nextHop:       nextHop,
		attributes:    attrs,
		asPath:        asPath,
		wireBytes:     wireBytes,
		nlriWireBytes: nlriWireBytes,
		sourceCtxID:   sourceCtxID,
	}
	r.refCount.Store(1)
	return r
}

// NLRI returns the route's NLRI.
func (r *Route) NLRI() nlri.NLRI {
	return r.nlri
}

// NextHop returns the route's next-hop address.
func (r *Route) NextHop() netip.Addr {
	return r.nextHop
}

// Attributes returns the route's path attributes (excluding AS-PATH which
// is stored separately).
func (r *Route) Attributes() []attribute.Attribute {
	return r.attributes
}

// ASPath returns the route's AS-PATH (may be nil).
func (r *Route) ASPath() *attribute.ASPath {
	return r.asPath
}

// WireBytes returns the cached attribute wire bytes (may be nil).
func (r *Route) WireBytes() []byte {
	return r.wireBytes
}

// NLRIWireBytes returns the cached NLRI wire bytes (may be nil).
func (r *Route) NLRIWireBytes() []byte {
	return r.nlriWireBytes
}

// SourceCtxID returns the source context ID.
func (r *Route) SourceCtxID() bgpctx.ContextID {
	return r.sourceCtxID
}

// CanForwardDirect returns true if wireBytes can be used directly.
// This is the fast path for route reflection when source and destination
// peers have identical encoding contexts (same ASN4, ADD-PATH, etc.).
func (r *Route) CanForwardDirect(destCtxID bgpctx.ContextID) bool {
	return len(r.wireBytes) > 0 && r.sourceCtxID == destCtxID
}

// PackAttributesFor returns packed path attributes for the destination context.
// Uses cached wire bytes if contexts match (zero-copy), otherwise re-encodes.
//
// This is the main entry point for route forwarding:
//   - Fast path: return wireBytes when CanForwardDirect(destCtxID) is true
//   - Slow path: re-encode attributes using destination context
//
// Note: Callers must use registered ContextIDs (via Registry.Register).
// Unregistered IDs (0) may cause incorrect zero-copy decisions.
func (r *Route) PackAttributesFor(destCtxID bgpctx.ContextID) []byte {
	// Fast path: use cached bytes if compatible
	if r.CanForwardDirect(destCtxID) {
		return r.wireBytes
	}

	// Slow path: re-encode with destination context
	destCtx := bgpctx.Registry.Get(destCtxID)
	return packAttributesWithContext(r.attributes, r.asPath, destCtx)
}

// PackNLRIFor returns packed NLRI for the destination context.
// Uses cached nlriWireBytes if contexts match (zero-copy), otherwise re-encodes.
//
// Note: Callers must use registered ContextIDs (via Registry.Register).
func (r *Route) PackNLRIFor(destCtxID bgpctx.ContextID) []byte {
	// Fast path: use cached bytes if compatible
	if len(r.nlriWireBytes) > 0 && r.sourceCtxID == destCtxID {
		return r.nlriWireBytes
	}

	// Slow path: re-encode with destination context
	destCtx := bgpctx.Registry.Get(destCtxID)
	if destCtx == nil {
		buf := make([]byte, r.nlri.Len())
		r.nlri.WriteTo(buf, 0, nil)
		return buf
	}
	packCtx := destCtx.ToPackContext(r.nlri.Family())
	nlriLen := nlri.LenWithContext(r.nlri, packCtx)
	buf := make([]byte, nlriLen)
	nlri.WriteNLRI(r.nlri, buf, 0, packCtx)
	return buf
}

// packAttributesWithContext packs attributes using the given encoding context.
// Handles context-dependent encoding for AS_PATH (ASN4) and other attributes.
//
// Optimization: Pre-calculates total size to minimize allocations.
func packAttributesWithContext(attrs []attribute.Attribute, asPath *attribute.ASPath, ctx *bgpctx.EncodingContext) []byte {
	// Fast path: no attributes
	if len(attrs) == 0 && asPath == nil {
		return nil
	}

	// Collect all attributes including AS_PATH
	allAttrs := make([]attribute.Attribute, 0, len(attrs)+1)
	allAttrs = append(allAttrs, attrs...)
	if asPath != nil {
		allAttrs = append(allAttrs, asPath)
	}

	// Order by type code per RFC 4271 Appendix F.3
	ordered := attribute.OrderAttributes(allAttrs)

	// Pre-calculate total size with context
	totalSize := attribute.AttributesSizeWithContext(ordered, ctx)

	// Pre-allocate result buffer and write
	result := make([]byte, totalSize)
	off := 0
	for _, attr := range ordered {
		off += attribute.WriteAttrToWithContext(attr, result, off, nil, ctx)
	}

	return result
}

// Index returns a unique identifier for this route.
// Includes: Family + NLRI wire format + AS-PATH hash (if present).
//
// This enables the novel approach where AS-PATH is part of route identity,
// allowing multiple routes for the same prefix with different AS-PATHs.
func (r *Route) Index() []byte {
	if r.indexCache != nil {
		return r.indexCache
	}

	family := r.nlri.Family()
	// Phase 3: WriteTo(nil) returns payload only, need to include path ID separately
	nlriLen := r.nlri.Len()
	pathID := r.nlri.PathID()
	hasPathID := pathID != 0

	// Calculate index size
	size := 3 + nlriLen // AFI(2) + SAFI(1) + NLRI
	if hasPathID {
		size += 4 // Path ID
	}
	if r.asPath != nil {
		size += 8 // AS-PATH hash
	}

	buf := make([]byte, size)
	offset := 0

	// Family (AFI + SAFI)
	binary.BigEndian.PutUint16(buf[offset:], uint16(family.AFI))
	offset += 2
	buf[offset] = byte(family.SAFI)
	offset++

	// Path ID (if present)
	if hasPathID {
		binary.BigEndian.PutUint32(buf[offset:], pathID)
		offset += 4
	}

	// NLRI bytes - write directly into buffer
	r.nlri.WriteTo(buf, offset, nil)
	offset += nlriLen

	// AS-PATH hash (if present)
	if r.asPath != nil {
		h := hashASPath(r.asPath)
		binary.BigEndian.PutUint64(buf[offset:], h)
	}

	r.indexCache = buf
	return buf
}

// hashASPath computes a hash of the AS-PATH for indexing.
func hashASPath(asPath *attribute.ASPath) uint64 {
	h := fnv.New64a()
	for _, seg := range asPath.Segments {
		_, _ = h.Write([]byte{byte(seg.Type)})
		for _, asn := range seg.ASNs {
			var buf [4]byte
			binary.BigEndian.PutUint32(buf[:], asn)
			_, _ = h.Write(buf[:])
		}
	}
	return h.Sum64()
}

// RefCount returns the current reference count.
func (r *Route) RefCount() int32 {
	return r.refCount.Load()
}

// Acquire increments the reference count.
func (r *Route) Acquire() {
	r.refCount.Add(1)
}

// Release decrements the reference count.
// Returns true if the route can be freed (refCount reached 0).
func (r *Route) Release() bool {
	newCount := r.refCount.Add(-1)
	return newCount <= 0
}

// AttrIterator returns an iterator over the cached attribute wire bytes.
// Returns nil if route has no wire cache (wireBytes is empty).
//
// The iterator provides zero-copy access to path attributes stored in wireBytes.
// Use this instead of Attributes() when you only need to iterate without
// building a slice of parsed Attribute objects.
func (r *Route) AttrIterator() *attribute.AttrIterator {
	if len(r.wireBytes) == 0 {
		return nil
	}
	return attribute.NewAttrIterator(r.wireBytes)
}

// ASPathIterator returns an iterator over the AS-PATH attribute in wireBytes.
// Returns nil if route has no wire cache or no AS_PATH attribute.
// Set asn4=true for 4-byte ASN encoding, false for 2-byte.
//
// The iterator provides zero-copy access to AS-PATH segments.
// Use this instead of ASPath() when you only need to iterate without
// building parsed ASPathSegment slices.
func (r *Route) ASPathIterator(asn4 bool) *attribute.ASPathIterator {
	if len(r.wireBytes) == 0 {
		return nil
	}

	// Find AS_PATH attribute in wireBytes
	iter := attribute.NewAttrIterator(r.wireBytes)
	for typeCode, _, value, ok := iter.Next(); ok; typeCode, _, value, ok = iter.Next() {
		if typeCode == attribute.AttrASPath {
			asPathBytes := value.Slice(r.wireBytes)
			return attribute.NewASPathIterator(asPathBytes, asn4)
		}
	}
	return nil
}
