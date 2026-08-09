// Design: docs/architecture/rib/forward-handle.md -- producer side of locrib.ForwardHandle
// Related: rib_structured.go -- creates one handle per received UPDATE
// Related: rib_bestchange.go -- threads the handle to locrib.InsertForward

package rib

import (
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/memguard"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// ribForwardHandle implements locrib.ForwardHandle and locrib.ForwardBytes
// on top of the UPDATE wire bytes carried by bgptypes.RawMessage.
//
// Lifetime model: the source slice is valid only while the producing
// handler (handleReceivedStructured) has not returned --
// types.RawMessage.IsAsyncSafe documents that received-UPDATE RawBytes
// are reused after the callback. A subscriber that wants to retain the
// bytes past the handler invocation MUST call AddRef from inside the
// ChangeHandler. The first AddRef triggers a sync.Once copy of source
// into an owned buf; subsequent AddRefs are pure atomic increments.
// Bytes returns the owned copy (nil if AddRef was never called).
//
// Cost profile: a subscriber-free UPDATE pays one handle struct alloc
// (the refcount + Once fields) and zero byte copies. An UPDATE observed
// by at least one retaining subscriber pays the struct alloc plus one
// copy of RawBytes (bounded by the 4K / 64K UPDATE max).
type ribForwardHandle struct {
	source []byte // valid only until the producing handler returns
	once   sync.Once
	buf    []byte // owned copy materialized by the first AddRef
	refs   atomic.Int32
}

// newForwardHandle wraps the UPDATE wire bytes in a ForwardHandle.
// Returns nil when source is empty so the dispatched Change carries
// Forward == nil (matches the "no wire payload" contract).
func newForwardHandle(source []byte) locrib.ForwardHandle {
	if len(source) == 0 {
		return nil
	}
	return &ribForwardHandle{source: source}
}

// AddRef copies source into an owned buf on first call (via sync.Once),
// then increments the reference count. Safe for concurrent use.
//
// The subscriber calls AddRef from inside its ChangeHandler while the
// producing handler is still running, so source is still valid at the
// moment once.Do fires. Subsequent AddRefs skip the copy.
func (h *ribForwardHandle) AddRef() {
	h.once.Do(func() {
		h.buf = make([]byte, len(h.source))
		copy(h.buf, h.source)
		h.source = nil // drop the unsafe reference; buf is now authoritative
	})
	h.refs.Add(1)
}

// Release decrements the reference count. Typically called off-lock
// from the subscriber's worker after it has finished with Bytes.
//
// On the final Release (count reaches zero) debug builds poison the owned copy
// so a subscriber that reads Bytes() after its matching Release reads the poison
// pattern instead of live wire bytes (the contract-C canary; see
// docs/architecture/memory/lifetime-contracts.md). buf is nil when AddRef was
// never called, and Poison no-ops on nil. In release builds memguard.Enabled is
// a false constant, so this folds back to the bare decrement — no added cost.
func (h *ribForwardHandle) Release() {
	n := h.refs.Add(-1)
	if memguard.Enabled && n <= 0 {
		memguard.Poison(h.buf)
	}
}

// Bytes returns the retained copy of the UPDATE wire bytes. Returns
// nil if AddRef has not been called. Implementation of
// locrib.ForwardBytes.
//
// Safe to call from any goroutine between the subscriber's AddRef
// and matching Release. After all AddRefs have been matched by Release
// and no one retains the interface value, the handle (and buf) become
// GC-eligible. In debug builds buf is poisoned on the final Release, so a
// contract-violating read after Release returns the poison pattern.
func (h *ribForwardHandle) Bytes() []byte {
	return h.buf
}
