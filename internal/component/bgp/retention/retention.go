// Design: docs/guide/graceful-restart.md -- in-process per-family retention ownership.
// Package retention lets BGP forwarding owners ask the active GR state machine
// whether a source family is retained, without importing the GR plugin.
package retention

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/family"
)

// Owner publishes one GR engine's lookup. The engine MUST Close it after stopping
// event delivery. The lookup MUST be safe for concurrent calls and MUST NOT call
// back into forwarding or the RIB; callers may hold their inventory lock.
type Owner struct {
	lookup func(string, family.Family) bool
}

var current atomic.Pointer[Owner]

// Publish installs the in-process owner. The caller MUST Close the returned owner
// when it stops. An older owner's Close cannot erase a replacement engine.
func Publish(lookup func(string, family.Family) bool) *Owner {
	owner := &Owner{lookup: lookup}
	current.Store(owner)
	return owner
}

// Close releases this publication. The caller MUST stop event delivery first.
// Safe for concurrent use, including replacement by another owner.
func (owner *Owner) Close() {
	current.CompareAndSwap(owner, nil)
}

// Family reports active retention, not a capability declaration. False with no
// owner means no in-process GR decision is available; it does not infer the state
// of an external GR process, which cannot publish into this address space.
func Family(peer string, fam family.Family) bool {
	owner := current.Load()
	if owner == nil {
		return false
	}
	if owner.lookup == nil {
		return false
	}
	return owner.lookup(peer, fam)
}
