package rib

import (
	"sync"

	"github.com/exa-networks/zebgp/internal/store"
	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
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
	// Hash the packed bytes
	return store.HashBytes(h.attr.Pack())
}

func (h hashableAttr) Equal(other any) bool {
	o, ok := other.(hashableAttr)
	if !ok {
		return false
	}
	// Compare by code and packed bytes
	if h.attr.Code() != o.attr.Code() {
		return false
	}
	hBytes := h.attr.Pack()
	oBytes := o.attr.Pack()
	if len(hBytes) != len(oBytes) {
		return false
	}
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

func (h hashableNLRI) Bytes() []byte {
	// Use Pack(nil) for consistent API - returns same bytes as Bytes()
	return h.n.Pack(nil)
}

func (h hashableNLRI) FamilyKey() uint32 {
	f := h.n.Family()
	return uint32(f.AFI)<<16 | uint32(f.SAFI)
}

// NewRouteStore creates a new route store with the given buffer size.
func NewRouteStore(bufferSize int) *RouteStore {
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

// InternAttribute deduplicates an attribute.
func (rs *RouteStore) InternAttribute(attr attribute.Attribute) attribute.Attribute {
	s := rs.getOrCreateAttrStore(attr.Code())
	result := s.store.Intern(hashableAttr{attr: attr})
	return result.attr
}

// InternAttributes deduplicates a slice of attributes.
func (rs *RouteStore) InternAttributes(attrs []attribute.Attribute) []attribute.Attribute {
	result := make([]attribute.Attribute, len(attrs))
	for i, attr := range attrs {
		result[i] = rs.InternAttribute(attr)
	}
	return result
}

// InternNLRI deduplicates an NLRI.
func (rs *RouteStore) InternNLRI(n nlri.NLRI) nlri.NLRI {
	result := rs.nlriStore.store.Intern(hashableNLRI{n: n})
	return result.n
}

// InternRoute deduplicates a route and its components.
// Returns a potentially shared route instance.
func (rs *RouteStore) InternRoute(route *Route) *Route {
	// Intern the NLRI
	internedNLRI := rs.InternNLRI(route.nlri)

	// Intern attributes
	internedAttrs := rs.InternAttributes(route.attributes)

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

// ReleaseRoute decrements the reference count and removes if zero.
func (rs *RouteStore) ReleaseRoute(route *Route) {
	if route.Release() {
		// Remove from store
		idx := string(route.Index())

		rs.routesMu.Lock()
		delete(rs.routes, idx)
		rs.routesMu.Unlock()

		// Release interned attributes
		for _, attr := range route.attributes {
			s := rs.getOrCreateAttrStore(attr.Code())
			s.store.Release(hashableAttr{attr: attr})
		}

		// Release interned NLRI
		rs.nlriStore.store.Release(hashableNLRI{n: route.nlri})
	}
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
