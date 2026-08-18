// Design: docs/architecture/core-design.md — immutable received UPDATE snapshot
// Design: .claude/rules/design-principles.md — zero-copy, copy-on-modify (holds Incoming Peer Pool buffer read-only)
// Related: recent_cache.go — RecentUpdateCache stores ReceivedUpdate entries
// Related: reactor_notify.go — creates ReceivedUpdate on inbound UPDATE

package reactor

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// msgIDCounter generates unique message IDs.
// Atomic for concurrent access from multiple peer goroutines.
var msgIDCounter atomic.Uint64

// nextMsgID returns the next unique message ID.
func nextMsgID() uint64 {
	return msgIDCounter.Add(1)
}

// ReceivedUpdate represents an immutable snapshot of a received UPDATE.
// Each UPDATE gets a unique ID; updates to same NLRI create new IDs.
//
// Memory contract: WireUpdate slices into poolBuf.Buf; all derived slices share it.
// When cache evicts this entry, poolBuf is returned to the buffer multiplexer.
// Per-forward wire variants (dual-AS prepend, per-key local-AS override,
// ASN4->ASN2 transcode, export-filter override) hang their read-pool handle on
// fwdHandles via adoptFwdHandle and are returned on eviction as well -- the same
// single return point as poolBuf (recent_cache.go evictLocked / Delete).
// Message ID is stored in WireUpdate, accessible via WireUpdate.MessageID().
type ReceivedUpdate struct {
	// WireUpdate contains the UPDATE payload with zero-copy accessors.
	// Provides Payload(), Attrs(), NLRI(), MPReach(), MPUnreach(), SourceCtxID(), MessageID().
	// Points to wireUpdateInline so the WireUpdate lives inside the same
	// heap allocation as ReceivedUpdate (one fewer allocation per UPDATE).
	WireUpdate *wireu.WireUpdate

	// wireUpdateInline is the storage backing WireUpdate. All callers use
	// the pointer field above; this field must not be accessed directly.
	wireUpdateInline wireu.WireUpdate

	// poolBuf is the session read buffer handle that WireUpdate slices into.
	// Returned to multiplexer when cache evicts this entry.
	poolBuf BufHandle

	// SourcePeerIP is the IP address of the peer that sent this UPDATE.
	SourcePeerIP netip.Addr

	// ReceivedAt is when this UPDATE was received.
	ReceivedAt time.Time

	// Meta holds route metadata set at ingress by filters.
	// Read-only after creation. May be nil if no filter set metadata.
	// Does NOT contain "source-peer"; use SourcePeerIP or SourcePeerStr instead.
	Meta map[string]any

	// SourcePeerStr caches SourcePeerIP.String() to avoid per-forward allocations.
	// Set once at creation from the peer's cached address string.
	SourcePeerStr string

	// fwdHandleMu is a dedicated LEAF mutex guarding fwdHandles. Lock ordering:
	// adopters (the forward path) hold NO other lock when calling adoptFwdHandle;
	// the cache takes it strictly inside cache.mu when draining at eviction
	// (cache.mu -> fwdHandleMu is the only nesting). Never acquire another lock
	// while holding it. See spec-fixit-forward-readbuf-leak D-3.
	fwdHandleMu sync.Mutex

	// fwdHandles holds read-pool buffer handles borrowed on the forward path
	// (reactor_api_forward.go / forward_rs.go) for per-destination wire variants.
	// Same ownership contract as poolBuf: returned to the pool exactly once when
	// the cache evicts this entry. Appended by adoptFwdHandle immediately after
	// the wire is built on the success path; drained by returnFwdHandles.
	fwdHandles []BufHandle
}

// adoptFwdHandle takes ownership of a read-pool buffer handle borrowed on the
// forward path for a per-destination wire variant (dual-AS prepend, per-key
// local-AS override, ASN4->ASN2 transcode, or export-filter override). The handle
// backs a *wireu.WireUpdate that is aliased zero-copy into async worker writes, so
// it MUST NOT be returned at end of the forward call; it is returned exactly once
// when the cache evicts this entry -- the same point that returns poolBuf
// (spec-fixit-forward-readbuf-leak D-1/D-2). Callers adopt on the
// success path ONLY; error paths return the handle immediately (D-5). A zero
// handle (Buf == nil) is ignored. Adopters must hold no other lock (D-3).
func (u *ReceivedUpdate) adoptFwdHandle(h BufHandle) {
	if h.Buf == nil {
		return
	}
	u.fwdHandleMu.Lock()
	u.fwdHandles = append(u.fwdHandles, h)
	u.fwdHandleMu.Unlock()
}

// returnFwdHandles returns every adopted forward-path handle to the pool and
// empties the list. Called by the cache under cache.mu when the entry is evicted
// (evictLocked) or deleted (Delete) -- the single return point for entry-owned
// per-forward read-pool handles. Idempotent: a second call (e.g. a hypothetical
// double-evict) finds an empty list and returns nothing twice. ReturnReadBuffer
// runs outside fwdHandleMu so the leaf lock never nests the pool mutex.
func (u *ReceivedUpdate) returnFwdHandles() {
	u.fwdHandleMu.Lock()
	handles := u.fwdHandles
	u.fwdHandles = nil
	u.fwdHandleMu.Unlock()
	for _, h := range handles {
		ReturnReadBuffer(h)
	}
}

// getReadBuf gets a buffer handle from the appropriate multiplexer.
// Uses the same multiplexers as session reads for uniform lifecycle management.
func getReadBuf(extendedMessage bool) BufHandle {
	if extendedMessage {
		return bufMuxExt.Get()
	}
	return bufMuxStd.Get()
}
