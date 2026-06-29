// Design: docs/architecture/encoding-context.md — encoding context

package context

import (
	"errors"
	"sync"
)

// ErrContextIDExhausted is returned when all 65535 context IDs are in use.
var ErrContextIDExhausted = errors.New("context ID space exhausted")

// ContextID is a compact identifier for an EncodingContext.
// Enables fast compatibility checks via integer comparison.
//
// Note: uint16 limits to 65535 unique contexts. This is sufficient for
// typical BGP deployments (each peer needs 1-2 contexts, supporting 30K+ peers).
type ContextID uint16

// ContextRegistry manages EncodingContext registration and lookup.
// Deduplicates identical contexts to save memory.
// Thread-safe for concurrent access.
type ContextRegistry struct {
	mu       sync.RWMutex
	contexts map[ContextID]*EncodingContext
	byHash   map[uint64]ContextID
	nextID   ContextID
}

// NewRegistry creates an empty ContextRegistry.
func NewRegistry() *ContextRegistry {
	return &ContextRegistry{
		contexts: make(map[ContextID]*EncodingContext),
		byHash:   make(map[uint64]ContextID),
		nextID:   1, // Start from 1, leaving 0 as invalid
	}
}

// Register returns an ID for the context, deduplicating identical ones.
// If a context with the same hash already exists, returns its existing ID.
// Otherwise, assigns a new ID and stores the context.
// Returns ErrContextIDExhausted if all 65535 IDs are in use.
func (r *ContextRegistry) Register(ctx *EncodingContext) (ContextID, error) {
	hash := ctx.Hash()

	// Fast path: check if already registered
	r.mu.RLock()
	if id, ok := r.byHash[hash]; ok {
		r.mu.RUnlock()
		return id, nil
	}
	r.mu.RUnlock()

	// Slow path: need to register
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if id, ok := r.byHash[hash]; ok {
		return id, nil
	}

	if len(r.contexts) >= 65535 {
		return 0, ErrContextIDExhausted
	}

	id := r.nextID
	r.nextID++
	if r.nextID == 0 {
		r.nextID = 1
	}

	r.contexts[id] = ctx
	r.byHash[hash] = id

	return id, nil
}

// Get retrieves the context by ID.
// Returns nil if the ID is not registered.
func (r *ContextRegistry) Get(id ContextID) *EncodingContext {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.contexts[id]
}

// Count returns the number of registered contexts.
func (r *ContextRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.contexts)
}

// Global registry instance for application-wide context management.
// Use this for registering peer contexts at session establishment.
var Registry = NewRegistry()
