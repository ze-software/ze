// Package store provides deduplication stores for BGP data structures.
//
// Key innovation: Uses per-attribute-type goroutines for concurrent interning,
// allowing parallel attribute processing while maintaining deduplication.
package store

import (
	"context"
	"hash/fnv"
	"sync"
	"sync/atomic"
)

// Hashable is implemented by types that can be hashed for deduplication.
type Hashable interface {
	// Hash returns a 64-bit hash of the value.
	Hash() uint64
	// Equal returns true if the value equals another.
	Equal(other any) bool
}

// internRequest is sent to a worker goroutine for interning.
type internRequest[T Hashable] struct {
	value    T
	response chan T
}

// AttributeStore provides deduplication for attributes using per-type workers.
//
// Each attribute type has its own goroutine that handles interning requests,
// allowing concurrent access without lock contention across types.
type AttributeStore[T Hashable] struct {
	entries map[uint64][]entry[T]
	mu      sync.RWMutex

	// Channel for intern requests
	requests chan internRequest[T]

	// Stats
	hits   atomic.Uint64
	misses atomic.Uint64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// entry holds a stored value with its reference count.
type entry[T Hashable] struct {
	value    T
	refCount int64
}

// NewAttributeStore creates a new attribute store with a worker goroutine.
func NewAttributeStore[T Hashable](bufferSize int) *AttributeStore[T] {
	ctx, cancel := context.WithCancel(context.Background())
	s := &AttributeStore[T]{
		entries:  make(map[uint64][]entry[T]),
		requests: make(chan internRequest[T], bufferSize),
		ctx:      ctx,
		cancel:   cancel,
	}

	s.wg.Add(1)
	go s.worker()

	return s
}

// worker processes intern requests sequentially for this store.
func (s *AttributeStore[T]) worker() {
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

// Intern returns a deduplicated instance of the value.
// If an equal value exists in the store, returns it. Otherwise stores and returns the input.
// This is thread-safe and uses the worker goroutine for serialization.
func (s *AttributeStore[T]) Intern(value T) T {
	resp := make(chan T, 1)
	s.requests <- internRequest[T]{value: value, response: resp}
	return <-resp
}

// internSync performs the actual interning (called by worker).
func (s *AttributeStore[T]) internSync(value T) T {
	hash := value.Hash()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing entry
	entries := s.entries[hash]
	for i := range entries {
		if entries[i].value.Equal(value) {
			entries[i].refCount++
			s.hits.Add(1)
			return entries[i].value
		}
	}

	// Not found, add new entry
	s.entries[hash] = append(entries, entry[T]{value: value, refCount: 1})
	s.misses.Add(1)
	return value
}

// InternDirect performs synchronous interning without using the worker.
// Use this when you're already in a serialized context.
func (s *AttributeStore[T]) InternDirect(value T) T {
	return s.internSync(value)
}

// Release decrements the reference count for a value.
// Returns true if the value was found and decremented.
func (s *AttributeStore[T]) Release(value T) bool {
	hash := value.Hash()

	s.mu.Lock()
	defer s.mu.Unlock()

	entries := s.entries[hash]
	for i := range entries {
		if entries[i].value.Equal(value) {
			entries[i].refCount--
			if entries[i].refCount <= 0 {
				// Remove entry
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

// Lookup finds a value in the store without interning.
// Returns the stored value and true if found, zero value and false otherwise.
func (s *AttributeStore[T]) Lookup(value T) (T, bool) {
	hash := value.Hash()

	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := s.entries[hash]
	for i := range entries {
		if entries[i].value.Equal(value) {
			return entries[i].value, true
		}
	}

	var zero T
	return zero, false
}

// Len returns the number of unique entries in the store.
func (s *AttributeStore[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, entries := range s.entries {
		count += len(entries)
	}
	return count
}

// Stats returns hit/miss statistics.
func (s *AttributeStore[T]) Stats() (hits, misses uint64) {
	return s.hits.Load(), s.misses.Load()
}

// Stop stops the worker goroutine.
func (s *AttributeStore[T]) Stop() {
	s.cancel()
	s.wg.Wait()
}

// Wait waits for the worker to stop.
func (s *AttributeStore[T]) Wait() {
	s.wg.Wait()
}

// HashBytes returns a 64-bit hash of a byte slice.
func HashBytes(data []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(data)
	return h.Sum64()
}

// HashUint32 returns a 64-bit hash of a uint32.
func HashUint32(v uint32) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte{
		byte(v >> 24),
		byte(v >> 16),
		byte(v >> 8),
		byte(v),
	})
	return h.Sum64()
}

// HashString returns a 64-bit hash of a string.
func HashString(s string) uint64 {
	return HashBytes([]byte(s))
}

// CombineHashes combines multiple hashes into one.
func CombineHashes(hashes ...uint64) uint64 {
	h := fnv.New64a()
	for _, hash := range hashes {
		_, _ = h.Write([]byte{
			byte(hash >> 56),
			byte(hash >> 48),
			byte(hash >> 40),
			byte(hash >> 32),
			byte(hash >> 24),
			byte(hash >> 16),
			byte(hash >> 8),
			byte(hash),
		})
	}
	return h.Sum64()
}
