// Package storage provides pool-based RIB storage types.
package storage

import (
	"bytes"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/pkg/pool"
)

// NLRISet stores NLRIs for one attribute set.
// Interface allows family-specific optimization.
type NLRISet interface {
	// Add appends NLRI to the set.
	Add(nlri []byte)

	// Remove removes NLRI from the set, returns true if found.
	Remove(nlri []byte) bool

	// Contains checks if NLRI exists.
	Contains(nlri []byte) bool

	// Iterate calls fn for each NLRI (wire bytes).
	// Stops if fn returns false.
	Iterate(fn func(nlri []byte) bool)

	// Len returns number of NLRIs.
	Len() int

	// Release frees any pool handles (no-op for direct).
	Release()
}

// DirectNLRISet stores small NLRIs as concatenated wire bytes.
// Used for IPv4 where NLRI (1-5 bytes) < handle overhead (4 bytes).
type DirectNLRISet struct {
	data    []byte      // Concatenated wire-format NLRIs
	count   int         // Number of NLRIs (avoid re-parsing for Len())
	family  nlri.Family // AFI + SAFI
	addPath bool        // If true, NLRIs have 4-byte path-id prefix
}

// NewDirectNLRISet creates a DirectNLRISet for the given family.
func NewDirectNLRISet(family nlri.Family, addPath bool) *DirectNLRISet {
	return &DirectNLRISet{
		family:  family,
		addPath: addPath,
	}
}

// nlriLen returns the wire length of an NLRI at offset.
// RFC 4271: prefix encoding is [prefix-len:1][prefix-bytes:(prefixLen+7)/8]
// RFC 7911: ADD-PATH prepends [path-id:4].
// Returns 0 if data is malformed or truncated.
func (s *DirectNLRISet) nlriLen(offset int) int {
	if offset >= len(s.data) {
		return 0
	}
	var length int
	if s.addPath {
		// ADD-PATH: [path-id:4][prefix-len:1][prefix-bytes]
		if offset+5 > len(s.data) {
			return 0
		}
		prefixLen := s.data[offset+4]
		length = 4 + 1 + (int(prefixLen)+7)/8
	} else {
		// Standard: [prefix-len:1][prefix-bytes]
		prefixLen := s.data[offset]
		length = 1 + (int(prefixLen)+7)/8
	}
	// Validate calculated length doesn't exceed buffer
	if offset+length > len(s.data) {
		return 0
	}
	return length
}

// Add appends NLRI to the set if not already present.
// Defensive duplicate check - caller (FamilyRIB) should prevent duplicates via prefixIndex.
func (s *DirectNLRISet) Add(nlri []byte) {
	if s.Contains(nlri) {
		return
	}
	s.data = append(s.data, nlri...)
	s.count++
}

// Remove removes NLRI from the set, returns true if found.
func (s *DirectNLRISet) Remove(nlri []byte) bool {
	offset := 0
	for offset < len(s.data) {
		length := s.nlriLen(offset)
		if length == 0 {
			break
		}
		if bytes.Equal(s.data[offset:offset+length], nlri) {
			// Found - remove by shifting remaining data
			copy(s.data[offset:], s.data[offset+length:])
			s.data = s.data[:len(s.data)-length]
			s.count--
			return true
		}
		offset += length
	}
	return false
}

// Contains checks if NLRI exists.
func (s *DirectNLRISet) Contains(nlri []byte) bool {
	offset := 0
	for offset < len(s.data) {
		length := s.nlriLen(offset)
		if length == 0 {
			break
		}
		if bytes.Equal(s.data[offset:offset+length], nlri) {
			return true
		}
		offset += length
	}
	return false
}

// Iterate calls fn for each NLRI (wire bytes).
func (s *DirectNLRISet) Iterate(fn func(nlri []byte) bool) {
	offset := 0
	for offset < len(s.data) {
		length := s.nlriLen(offset)
		if length == 0 {
			break
		}
		if !fn(s.data[offset : offset+length]) {
			return
		}
		offset += length
	}
}

// Len returns number of NLRIs.
func (s *DirectNLRISet) Len() int {
	return s.count
}

// Release is no-op for direct storage (GC handles data).
func (s *DirectNLRISet) Release() {}

// PooledNLRISet stores large NLRIs as pool handles.
// Used for IPv6+, VPN, EVPN where NLRI > handle size (4 bytes).
type PooledNLRISet struct {
	handles []pool.Handle  // Pool handles for each NLRI
	index   map[string]int // NLRI bytes → index in handles (for O(1) lookup)
	family  nlri.Family    // AFI + SAFI
	addPath bool           // If true, NLRIs have 4-byte path-id prefix
}

// NewPooledNLRISet creates a PooledNLRISet for the given family.
func NewPooledNLRISet(family nlri.Family, addPath bool) *PooledNLRISet {
	return &PooledNLRISet{
		index:   make(map[string]int),
		family:  family,
		addPath: addPath,
	}
}

// Add appends NLRI to the set.
// If NLRI already exists, this is a no-op (avoids duplicate handles).
func (s *PooledNLRISet) Add(nlri []byte) {
	key := string(nlri)
	if _, exists := s.index[key]; exists {
		return // Already present
	}
	h := pool.NLRI.Intern(nlri)
	idx := len(s.handles)
	s.handles = append(s.handles, h)
	s.index[key] = idx
}

// Remove removes NLRI from the set, returns true if found.
func (s *PooledNLRISet) Remove(nlri []byte) bool {
	key := string(nlri)
	idx, exists := s.index[key]
	if !exists {
		return false
	}

	// Release the pool handle
	pool.NLRI.Release(s.handles[idx])

	// Swap with last and remove (O(1) removal)
	lastIdx := len(s.handles) - 1
	if idx != lastIdx {
		// Get the NLRI bytes for the last element (need to update index)
		lastData := pool.NLRI.Get(s.handles[lastIdx])
		lastKey := string(lastData)

		s.handles[idx] = s.handles[lastIdx]
		s.index[lastKey] = idx
	}

	s.handles = s.handles[:lastIdx]
	delete(s.index, key)
	return true
}

// Contains checks if NLRI exists.
func (s *PooledNLRISet) Contains(nlri []byte) bool {
	_, exists := s.index[string(nlri)]
	return exists
}

// Iterate calls fn for each NLRI (wire bytes).
func (s *PooledNLRISet) Iterate(fn func(nlri []byte) bool) {
	for _, h := range s.handles {
		if !fn(pool.NLRI.Get(h)) {
			return
		}
	}
}

// Len returns number of NLRIs.
func (s *PooledNLRISet) Len() int {
	return len(s.handles)
}

// Release frees all pool handles.
func (s *PooledNLRISet) Release() {
	for _, h := range s.handles {
		pool.NLRI.Release(h)
	}
	s.handles = nil
	s.index = nil
}

// shouldPoolNLRI determines if family should use pooled storage.
// IPv4 unicast/multicast use direct (1-5 bytes < 4 byte handle).
// All others benefit from pooling.
func shouldPoolNLRI(family nlri.Family) bool {
	switch {
	case family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast:
		return false
	case family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIMulticast:
		return false
	default:
		return true
	}
}

// NewNLRISet creates appropriate NLRISet implementation for family.
func NewNLRISet(family nlri.Family, addPath bool) NLRISet {
	if shouldPoolNLRI(family) {
		return NewPooledNLRISet(family, addPath)
	}
	return NewDirectNLRISet(family, addPath)
}
