// Design: docs/architecture/pool-architecture.md — RIB wire storage

package rib

import (
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/store"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// RouteStore provides global deduplication for routes and their components.
//
// Novel design: Uses per-attribute-type goroutines for concurrent interning,
// allowing parallel attribute processing while maintaining deduplication.
// AS-PATH is treated as part of route identity (not a regular attribute)
// to enable better deduplication when routes share attributes except AS-PATH.
type RouteStore struct {
	// Per-attribute-type stores (keyed by attribute code)
	attrStores map[attribute.AttributeCode]*attrStore

	// NLRI store (per-family)
	nlriStore *nlriStoreWrapper

	// Reference counting for routes
	routes   map[string]*Route
	routesMu sync.RWMutex

	bufferSize int
	mu         sync.RWMutex
}

// attrStore wraps a generic attribute store.
type attrStore struct {
	store *store.AttributeStore[hashableAttr]
}

// hashableAttr wraps an attribute.Attribute with Hash/Equal methods.
type hashableAttr struct {
	attr attribute.Attribute
}

func (h hashableAttr) Hash() uint64 {
	buf := make([]byte, h.attr.Len()) // pool-fallback: escapes via HashBytes
	h.attr.WriteTo(buf, 0)
	return store.HashBytes(buf)
}

func (h hashableAttr) Equal(other any) bool {
	o, ok := other.(hashableAttr)
	if !ok {
		return false
	}
	if h.attr.Code() != o.attr.Code() {
		return false
	}
	hLen := h.attr.Len()
	oLen := o.attr.Len()
	if hLen != oLen {
		return false
	}
	hBytes := make([]byte, hLen) // pool-fallback: escapes via WriteTo
	h.attr.WriteTo(hBytes, 0)
	oBytes := make([]byte, oLen) // pool-fallback: escapes via WriteTo
	o.attr.WriteTo(oBytes, 0)
	for i := range hBytes {
		if hBytes[i] != oBytes[i] {
			return false
		}
	}
	return true
}

// nlriStoreWrapper wraps the NLRI store.
type nlriStoreWrapper struct {
	store *store.NLRIStore[hashableNLRI]
}

// hashableNLRI wraps an nlri.NLRI with required methods.
type hashableNLRI struct {
	n nlri.NLRI
}

func (h hashableNLRI) Key() []byte {
	payloadLen := h.n.Len()
	pathID := h.n.PathID()
	if pathID == 0 {
		payload := make([]byte, payloadLen) // pool-fallback: returned slice
		h.n.WriteTo(payload, 0)
		return payload
	}
	key := make([]byte, 4+payloadLen) // pool-fallback: returned slice
	key[0] = byte(pathID >> 24)
	key[1] = byte(pathID >> 16)
	key[2] = byte(pathID >> 8)
	key[3] = byte(pathID)
	h.n.WriteTo(key, 4)
	return key
}

func (h hashableNLRI) FamilyKey() uint32 {
	f := h.n.Family()
	return uint32(f.AFI)<<16 | uint32(f.SAFI)
}

// newRouteStore creates a new route store with the given buffer size.
func newRouteStore(bufferSize int) *RouteStore {
	return &RouteStore{
		attrStores: make(map[attribute.AttributeCode]*attrStore),
		nlriStore: &nlriStoreWrapper{
			store: store.NewNLRIStore[hashableNLRI](bufferSize),
		},
		routes:     make(map[string]*Route),
		bufferSize: bufferSize,
	}
}

// getOrCreateAttrStore returns the store for an attribute code, creating if needed.
func (rs *RouteStore) getOrCreateAttrStore(code attribute.AttributeCode) *attrStore {
	rs.mu.RLock()
	s, ok := rs.attrStores[code]
	rs.mu.RUnlock()

	if ok {
		return s
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Double-check after acquiring write lock
	if s, ok = rs.attrStores[code]; ok {
		return s
	}

	s = &attrStore{
		store: store.NewAttributeStore[hashableAttr](rs.bufferSize),
	}
	rs.attrStores[code] = s
	return s
}

// internAttribute deduplicates an attribute.
func (rs *RouteStore) internAttribute(attr attribute.Attribute) attribute.Attribute {
	s := rs.getOrCreateAttrStore(attr.Code())
	result := s.store.Intern(hashableAttr{attr: attr})
	return result.attr
}

// internAttributes deduplicates a slice of attributes.
func (rs *RouteStore) internAttributes(attrs []attribute.Attribute) []attribute.Attribute {
	result := make([]attribute.Attribute, len(attrs))
	for i, attr := range attrs {
		result[i] = rs.internAttribute(attr)
	}
	return result
}

// internNLRI deduplicates an NLRI.
func (rs *RouteStore) internNLRI(n nlri.NLRI) nlri.NLRI {
	result := rs.nlriStore.store.Intern(hashableNLRI{n: n})
	return result.n
}

// internRoute deduplicates a route and its components.
// Returns a potentially shared route instance.
func (rs *RouteStore) internRoute(route *Route) *Route {
	// Intern the NLRI
	internedNLRI := rs.internNLRI(route.nlri)

	// Intern attributes
	internedAttrs := rs.internAttributes(route.attributes)

	// Check if route already exists
	idx := string(route.Index())

	rs.routesMu.Lock()
	defer rs.routesMu.Unlock()

	if existing, ok := rs.routes[idx]; ok {
		existing.Acquire()
		return existing
	}

	// Create new route with interned components
	newRoute := &Route{
		nlri:       internedNLRI,
		nextHop:    route.nextHop,
		attributes: internedAttrs,
		asPath:     route.asPath,
	}
	newRoute.refCount.Store(1)

	rs.routes[idx] = newRoute
	return newRoute
}

// releaseRoute decrements the reference count and removes if zero.
//
// The decrement runs under routesMu because internRoute finds an entry and
// acquires it under that same lock. Decrementing outside it lets an intern
// take the count back to 1 on a route this call has already decided to free,
// and the interned components then return to their stores under a route the
// other caller still holds.
//
// The interned components are released after the unlock, never under routesMu.
// getOrCreateAttrStore takes rs.mu, and internRoute takes rs.mu before
// routesMu, so holding both the other way round inverts the lock ordering.
// Releasing after the unlock is safe: AttributeStore.Release carries its own
// lock and refcount.
func (rs *RouteStore) releaseRoute(route *Route) {
	rs.routesMu.Lock()
	if !route.Release() {
		rs.routesMu.Unlock()
		return
	}
	delete(rs.routes, string(route.Index()))
	rs.routesMu.Unlock()

	// Release interned attributes
	for _, attr := range route.attributes {
		s := rs.getOrCreateAttrStore(attr.Code())
		s.store.Release(hashableAttr{attr: attr})
	}

	// Release interned NLRI
	rs.nlriStore.store.Release(hashableNLRI{n: route.nlri})
}

// Stats returns store statistics.
func (rs *RouteStore) Stats() RouteStoreStats {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	stats := RouteStoreStats{
		AttributeTypes: len(rs.attrStores),
		NLRIFamilies:   rs.nlriStore.store.FamilyCount(),
	}

	rs.routesMu.RLock()
	stats.Routes = len(rs.routes)
	rs.routesMu.RUnlock()

	for _, s := range rs.attrStores {
		stats.Attributes += s.store.Len()
	}

	stats.NLRIs = rs.nlriStore.store.TotalLen()

	return stats
}

// Stop stops all worker goroutines.
func (rs *RouteStore) Stop() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for _, s := range rs.attrStores {
		s.store.Stop()
	}
	rs.nlriStore.store.Stop()
}

// RouteStoreStats holds statistics about the route store.
type RouteStoreStats struct {
	Routes         int
	Attributes     int
	AttributeTypes int
	NLRIs          int
	NLRIFamilies   int
}
