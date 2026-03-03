// Design: docs/architecture/pool-architecture.md — attribute and NLRI storage

package store

import (
	"context"
	"sync"
	"sync/atomic"
)

// NLRIHashable represents an NLRI that can be hashed and compared.
type NLRIHashable interface {
	// Key returns the bytes used for hashing/deduplication.
	// This is the identity of the NLRI for storage purposes.
	Key() []byte
	// FamilyKey returns AFI/SAFI as a comparable key.
	FamilyKey() uint32
}

// nlriInternRequest is sent to a family worker for interning.
type nlriInternRequest[T NLRIHashable] struct {
	value    T
	response chan T
}

// FamilyStore holds NLRIs for a single AFI/SAFI.
type FamilyStore[T NLRIHashable] struct {
	entries map[uint64][]familyEntry[T]
	mu      sync.RWMutex

	requests chan nlriInternRequest[T]

	hits   atomic.Uint64
	misses atomic.Uint64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// familyEntry holds a stored NLRI with reference count.
type familyEntry[T NLRIHashable] struct {
	value    T
	refCount int64
}

// NewFamilyStore creates a new per-family NLRI store.
func NewFamilyStore[T NLRIHashable](bufferSize int) *FamilyStore[T] {
	ctx, cancel := context.WithCancel(context.Background())
	s := &FamilyStore[T]{
		entries:  make(map[uint64][]familyEntry[T]),
		requests: make(chan nlriInternRequest[T], bufferSize),
		ctx:      ctx,
		cancel:   cancel,
	}

	s.wg.Add(1)
	go s.worker()

	return s
}

// worker processes intern requests for this family.
func (s *FamilyStore[T]) worker() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case req := <-s.requests:
			result := s.internSync(req.value)
			req.response <- result
		}
	}
}

// Intern returns a deduplicated NLRI instance.
func (s *FamilyStore[T]) Intern(value T) T {
	resp := make(chan T, 1)
	s.requests <- nlriInternRequest[T]{value: value, response: resp}
	return <-resp
}

// internSync performs the actual interning.
func (s *FamilyStore[T]) internSync(value T) T {
	hash := HashBytes(value.Key())

	s.mu.Lock()
	defer s.mu.Unlock()

	entries := s.entries[hash]
	for i := range entries {
		if nlriEqual(entries[i].value, value) {
			entries[i].refCount++
			s.hits.Add(1)
			return entries[i].value
		}
	}

	s.entries[hash] = append(entries, familyEntry[T]{value: value, refCount: 1})
	s.misses.Add(1)
	return value
}

// nlriEqual compares two NLRIs by their key bytes.
func nlriEqual[T NLRIHashable](a, b T) bool {
	aBytes := a.Key()
	bBytes := b.Key()
	if len(aBytes) != len(bBytes) {
		return false
	}
	for i := range aBytes {
		if aBytes[i] != bBytes[i] {
			return false
		}
	}
	return true
}

// InternDirect performs synchronous interning.
func (s *FamilyStore[T]) InternDirect(value T) T {
	return s.internSync(value)
}

// Release decrements the reference count for an NLRI.
func (s *FamilyStore[T]) Release(value T) bool {
	hash := HashBytes(value.Key())

	s.mu.Lock()
	defer s.mu.Unlock()

	entries := s.entries[hash]
	for i := range entries {
		if nlriEqual(entries[i].value, value) {
			entries[i].refCount--
			if entries[i].refCount <= 0 {
				s.entries[hash] = append(entries[:i], entries[i+1:]...)
				if len(s.entries[hash]) == 0 {
					delete(s.entries, hash)
				}
			}
			return true
		}
	}
	return false
}

// Len returns the number of unique entries.
func (s *FamilyStore[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, entries := range s.entries {
		count += len(entries)
	}
	return count
}

// Stats returns hit/miss statistics.
func (s *FamilyStore[T]) Stats() (hits, misses uint64) {
	return s.hits.Load(), s.misses.Load()
}

// Stop stops the worker goroutine.
func (s *FamilyStore[T]) Stop() {
	s.cancel()
	s.wg.Wait()
}

// NLRIStore manages per-family NLRI stores.
//
// This provides a registry of FamilyStore instances, one per AFI/SAFI,
// allowing efficient deduplication within each address family.
type NLRIStore[T NLRIHashable] struct {
	stores map[uint32]*FamilyStore[T]
	mu     sync.RWMutex

	bufferSize int
}

// NewNLRIStore creates a new NLRI store registry.
func NewNLRIStore[T NLRIHashable](bufferSize int) *NLRIStore[T] {
	return &NLRIStore[T]{
		stores:     make(map[uint32]*FamilyStore[T]),
		bufferSize: bufferSize,
	}
}

// GetOrCreate returns the FamilyStore for the given family key, creating if needed.
func (n *NLRIStore[T]) GetOrCreate(familyKey uint32) *FamilyStore[T] {
	n.mu.RLock()
	store, ok := n.stores[familyKey]
	n.mu.RUnlock()

	if ok {
		return store
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// Double-check after acquiring write lock
	if store, ok = n.stores[familyKey]; ok {
		return store
	}

	store = NewFamilyStore[T](n.bufferSize)
	n.stores[familyKey] = store
	return store
}

// Intern deduplicates an NLRI using its family key.
func (n *NLRIStore[T]) Intern(value T) T {
	store := n.GetOrCreate(value.FamilyKey())
	return store.Intern(value)
}

// InternDirect performs synchronous interning.
func (n *NLRIStore[T]) InternDirect(value T) T {
	store := n.GetOrCreate(value.FamilyKey())
	return store.InternDirect(value)
}

// Release decrements the reference count for an NLRI.
func (n *NLRIStore[T]) Release(value T) bool {
	n.mu.RLock()
	store, ok := n.stores[value.FamilyKey()]
	n.mu.RUnlock()

	if !ok {
		return false
	}
	return store.Release(value)
}

// TotalLen returns the total number of entries across all families.
func (n *NLRIStore[T]) TotalLen() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	count := 0
	for _, store := range n.stores {
		count += store.Len()
	}
	return count
}

// FamilyCount returns the number of active family stores.
func (n *NLRIStore[T]) FamilyCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.stores)
}

// Stop stops all family store workers.
func (n *NLRIStore[T]) Stop() {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, store := range n.stores {
		store.Stop()
	}
}
