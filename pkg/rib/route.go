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

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	bgpctx "github.com/exa-networks/zebgp/pkg/bgp/context"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
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
	// sourceCtxID identifies the encoding context (for compatibility check).
	wireBytes   []byte
	sourceCtxID bgpctx.ContextID
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

// NewRouteWithWireCache creates a route with cached wire bytes.
// Used when receiving routes - store original bytes for potential zero-copy forwarding.
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

// WireBytes returns the cached wire bytes (may be nil).
func (r *Route) WireBytes() []byte {
	return r.wireBytes
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
	// Use Pack(nil) for consistent API - returns same bytes as Bytes()
	nlriBytes := r.nlri.Pack(nil)

	// Calculate index size
	size := 3 + len(nlriBytes) // AFI(2) + SAFI(1) + NLRI
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

	// NLRI bytes
	copy(buf[offset:], nlriBytes)
	offset += len(nlriBytes)

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
