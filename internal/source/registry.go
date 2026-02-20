// Design: docs/architecture/core-design.md — source registry

package source

import (
	"net/netip"
	"sync"
)

// Registry manages source registration and lookup.
// Thread-safe for concurrent access.
// ID ranges: 0=config, 1-99999=peers, 100000=reserved, 100001+=apis.
type Registry struct {
	mu sync.RWMutex

	// Sources indexed by ID. Sparse map since IDs have gaps between ranges.
	sources map[SourceID]*Source

	// Next ID counters
	nextPeerID SourceID
	nextAPIID  SourceID

	// Reverse indexes for O(1) lookup
	peerIdx map[netip.Addr]SourceID
	apiIdx  map[string]SourceID
}

// NewRegistry creates an empty Registry with config source pre-registered.
func NewRegistry() *Registry {
	r := &Registry{
		sources:    make(map[SourceID]*Source),
		nextPeerID: SourceIDPeerMin,
		nextAPIID:  SourceIDAPIMin,
		peerIdx:    make(map[netip.Addr]SourceID),
		apiIdx:     make(map[string]SourceID),
	}
	// Pre-register singleton config source
	r.sources[SourceIDConfig] = &Source{
		ID:     SourceIDConfig,
		Active: true,
	}
	return r
}

// RegisterPeer registers a BGP peer and returns its SourceID.
// If the peer IP is already registered, reactivates and updates AS.
// Returns InvalidSourceID if peer ID space exhausted.
func (r *Registry) RegisterPeer(ip netip.Addr, as uint32) SourceID {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already registered
	if id, ok := r.peerIdx[ip]; ok {
		src := r.sources[id]
		src.Active = true
		src.PeerAS = as
		return id
	}

	// Check ID space
	if r.nextPeerID > SourceIDPeerMax {
		return InvalidSourceID
	}

	// Assign new ID
	id := r.nextPeerID
	r.nextPeerID++

	r.sources[id] = &Source{
		ID:     id,
		Active: true,
		PeerIP: ip,
		PeerAS: as,
	}
	r.peerIdx[ip] = id
	return id
}

// RegisterAPI registers an API process and returns its SourceID.
// If the API name is already registered, reactivates it.
// Returns InvalidSourceID if name is empty or ID space exhausted.
func (r *Registry) RegisterAPI(name string) SourceID {
	if name == "" {
		return InvalidSourceID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already registered
	if id, ok := r.apiIdx[name]; ok {
		r.sources[id].Active = true
		return id
	}

	// Check ID space (must not reach InvalidSourceID)
	if r.nextAPIID >= InvalidSourceID {
		return InvalidSourceID
	}

	// Assign new ID
	id := r.nextAPIID
	r.nextAPIID++

	r.sources[id] = &Source{
		ID:     id,
		Active: true,
		Name:   name,
	}
	r.apiIdx[name] = id
	return id
}

// ConfigID returns the singleton config source ID.
func (r *Registry) ConfigID() SourceID {
	return SourceIDConfig
}

// Get retrieves the source by ID.
// Returns copy to prevent mutation without lock.
// Returns (Source{}, false) if the ID is invalid or not registered.
func (r *Registry) Get(id SourceID) (Source, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if id == InvalidSourceID {
		return Source{}, false
	}
	src, ok := r.sources[id]
	if !ok {
		return Source{}, false
	}
	return *src, true
}

// GetByPeerIP looks up a peer by IP address.
// Returns (InvalidSourceID, false) if not found.
func (r *Registry) GetByPeerIP(ip netip.Addr) (SourceID, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.peerIdx[ip]
	return id, ok
}

// GetByAPIName looks up an API by name.
// Returns (InvalidSourceID, false) if not found.
func (r *Registry) GetByAPIName(name string) (SourceID, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.apiIdx[name]
	return id, ok
}

// Deactivate marks a source as inactive.
// The source remains in the registry for historical lookup.
func (r *Registry) Deactivate(id SourceID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if src, ok := r.sources[id]; ok {
		src.Active = false
	}
}

// IsActive returns whether the source is currently active.
func (r *Registry) IsActive(id SourceID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if src, ok := r.sources[id]; ok {
		return src.Active
	}
	return false
}

// String returns the formatted string for a source ID.
func (r *Registry) String(id SourceID) string {
	src, ok := r.Get(id)
	if !ok {
		return unknownStr
	}
	return src.String()
}

// Count returns the number of registered sources.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sources)
}

// DefaultRegistry is the global registry instance.
var DefaultRegistry = NewRegistry()
