// Design: docs/architecture/route-types.md — rib.Route, the engine's route value
//
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

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
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

	// Cached index for fast lookup
	indexCache []byte
}

// NewRouteWithASPath creates a new route with explicit AS-PATH.
// The AS-PATH is stored separately for indexing purposes.
// Pass a nil AS-PATH for a route that carries none.
func NewRouteWithASPath(n nlri.NLRI, nextHop netip.Addr, attrs []attribute.Attribute, asPath *attribute.ASPath) *Route {
	return &Route{
		nlri:       n,
		nextHop:    nextHop,
		attributes: attrs,
		asPath:     asPath,
	}
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

// Index returns a unique identifier for this route.
// Includes: Family + NLRI wire format + AS-PATH hash (if present).
//
// This enables the novel approach where AS-PATH is part of route identity,
// allowing multiple routes for the same prefix with different AS-PATHs.
func (r *Route) Index() []byte {
	if r.indexCache != nil {
		return r.indexCache
	}

	fam := r.nlri.Family()
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
	binary.BigEndian.PutUint16(buf[offset:], uint16(fam.AFI))
	offset += 2
	buf[offset] = byte(fam.SAFI)
	offset++

	// Path ID (if present)
	if hasPathID {
		binary.BigEndian.PutUint32(buf[offset:], pathID)
		offset += 4
	}

	// NLRI bytes - write directly into buffer
	r.nlri.WriteTo(buf, offset)
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

// RouteJSON is a JSON-serializable view of a Route with optional peer info.
// Used for API output. Implements json.Marshaler for efficient serialization.
type RouteJSON struct {
	Route  *Route
	PeerID string
}

// MarshalJSON implements json.Marshaler.
// Output format: {"peer":"...", "prefix":"...", "next_hop":"...", "as_path":"..."}.
func (rj RouteJSON) MarshalJSON() ([]byte, error) {
	// Pre-allocate buffer for efficiency
	buf := make([]byte, 0, 128)
	buf = append(buf, '{')

	// Peer (optional)
	if rj.PeerID != "" {
		buf = append(buf, `"peer":"`...)
		buf = append(buf, rj.PeerID...)
		buf = append(buf, `",`...)
	}

	// Prefix
	buf = append(buf, `"prefix":"`...)
	if rj.Route.nlri != nil {
		buf = append(buf, rj.Route.nlri.String()...)
	}
	buf = append(buf, `",`...)

	// NextHop
	buf = append(buf, `"next_hop":"`...)
	if rj.Route.nextHop.IsValid() {
		buf = append(buf, rj.Route.nextHop.String()...)
	}
	buf = append(buf, '"')

	// ASPath (optional)
	if rj.Route.asPath != nil {
		buf = append(buf, `,"as_path":"`...)
		buf = appendASPath(buf, rj.Route.asPath)
		buf = append(buf, '"')
	}

	buf = append(buf, '}')
	return buf, nil
}

// appendASPath appends formatted AS-PATH to buffer.
func appendASPath(buf []byte, asPath *attribute.ASPath) []byte {
	buf = append(buf, '[')
	first := true
	for _, seg := range asPath.Segments {
		if seg.Type == attribute.ASSequence {
			for _, asn := range seg.ASNs {
				if !first {
					buf = append(buf, ' ')
				}
				buf = appendUint32(buf, asn)
				first = false
			}
		} else {
			// AS_SET: { asn asn }
			if !first {
				buf = append(buf, ' ')
			}
			buf = append(buf, '{')
			for i, asn := range seg.ASNs {
				if i > 0 {
					buf = append(buf, ' ')
				}
				buf = appendUint32(buf, asn)
			}
			buf = append(buf, '}')
			first = false
		}
	}
	buf = append(buf, ']')
	return buf
}

// appendUint32 appends a uint32 as decimal string.
func appendUint32(buf []byte, n uint32) []byte {
	if n == 0 {
		return append(buf, '0')
	}
	var digits [10]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return append(buf, digits[i:]...)
}
