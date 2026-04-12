// Design: docs/architecture/core-design.md -- BMP sender ribout dedup
//
// Related: sender.go -- outbound TCP to collectors
// Related: bmp.go -- event handler dispatches to ribout

package bmp

import (
	"hash/fnv"
	"sync"
)

// ribout tracks per-NLRI state to prevent resending identical routes.
// Key: NLRI string (e.g., "10.0.0.0/24"), Value: last sent path hash.
// A withdraw for an unknown NLRI is suppressed.
type ribout struct {
	mu    sync.Mutex
	paths map[string]uint64 // NLRI -> hash of last sent path
}

func newRibout() *ribout {
	return &ribout{
		paths: make(map[string]uint64),
	}
}

// shouldSend returns true if this NLRI+hash differs from what was last sent.
// For withdrawals (hash=0), returns true only if the NLRI was previously announced.
func (r *ribout) shouldSend(nlri string, hash uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if hash == 0 {
		// Withdrawal: only send if we previously announced this NLRI.
		if _, exists := r.paths[nlri]; exists {
			delete(r.paths, nlri)
			return true
		}
		return false
	}

	// Announcement: skip if identical to last sent.
	prev, exists := r.paths[nlri]
	if exists && prev == hash {
		return false
	}
	r.paths[nlri] = hash
	return true
}

// clear removes all tracked paths (e.g., on reconnect).
func (r *ribout) clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	clear(r.paths)
}

// fnvHash returns a 64-bit FNV-1a hash of the data.
func fnvHash(data []byte) uint64 {
	h := fnv.New64a()
	h.Write(data) // fnv.Write never returns an error
	return h.Sum64()
}
