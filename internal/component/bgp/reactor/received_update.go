// Design: docs/architecture/core-design.md — immutable received UPDATE snapshot
// Design: .claude/rules/design-principles.md — zero-copy, copy-on-modify (holds Incoming Peer Pool buffer read-only)
// Related: recent_cache.go — RecentUpdateCache stores ReceivedUpdate entries
// Related: reactor_notify.go — creates ReceivedUpdate on inbound UPDATE

package reactor

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// ebgpWireSlot bundles a generated EBGP wire variant with its backing pool
// buffer handle. Published atomically via atomic.Pointer so cache-hit readers
// see the complete pair in a single load without taking ebgpMu.
// Immutable after publication: fields are never modified once stored.
type ebgpWireSlot struct {
	wire   *wireu.WireUpdate
	handle BufHandle
}

var errEbgpWireBufferExhaustedPoolAt = errors.New("EBGP wire buffer exhausted: pool at maximum allocation")

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
// EBGP variant buffers (in ebgpSlotASN4/ebgpSlotASN2) are returned on eviction too.
// Per-forward wire variants that are NOT one of the two atomic ebgpSlot caches
// (dual-AS prepend, per-key local-AS override, ASN4->ASN2 transcode, export-filter
// override) hang their read-pool handle on fwdHandles via adoptFwdHandle and are
// returned on eviction as well -- the same single return point as poolBuf and the
// slots (recent_cache.go evictLocked / Delete).
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

	// ebgpMu protects EBGP wire generation (miss path only).
	// Cache-hit reads use atomic loads and never acquire this mutex.
	ebgpMu sync.Mutex

	// ebgpSlotASN4 holds the lazily-generated EBGP wire for 4-byte ASN peers.
	// nil until the first EBGPWire(_, _, true) call generates and publishes it.
	// Once non-nil, immutable: readers load atomically without ebgpMu.
	ebgpSlotASN4 atomic.Pointer[ebgpWireSlot]

	// ebgpSlotASN2 holds the lazily-generated EBGP wire for 2-byte ASN peers.
	// nil until the first EBGPWire(_, _, false) call generates and publishes it.
	ebgpSlotASN2 atomic.Pointer[ebgpWireSlot]

	// fwdHandleMu is a dedicated LEAF mutex guarding fwdHandles. Lock ordering:
	// adopters (the forward path) hold NO other lock when calling adoptFwdHandle;
	// the cache takes it strictly inside cache.mu when draining at eviction
	// (cache.mu -> fwdHandleMu is the only nesting). Never acquire another lock
	// while holding it; do NOT reuse ebgpMu (that would couple unrelated
	// lifecycles). See spec-fixit-forward-readbuf-leak D-3.
	fwdHandleMu sync.Mutex

	// fwdHandles holds read-pool buffer handles borrowed on the forward path
	// (reactor_api_forward.go / forward_rs.go) for per-destination wire variants
	// that are NOT one of the two atomic ebgpSlot caches. Same ownership contract
	// as poolBuf and the ebgpSlot handles: returned to the pool exactly once when
	// the cache evicts this entry. Appended by adoptFwdHandle immediately after
	// the wire is built on the success path; drained by returnFwdHandles.
	fwdHandles []BufHandle
}

// adoptFwdHandle takes ownership of a read-pool buffer handle borrowed on the
// forward path for a per-destination wire variant (dual-AS prepend, per-key
// local-AS override, ASN4->ASN2 transcode, or export-filter override). The handle
// backs a *wireu.WireUpdate that is aliased zero-copy into async worker writes, so
// it MUST NOT be returned at end of the forward call; it is returned exactly once
// when the cache evicts this entry -- the same point that returns poolBuf and the
// ebgpSlot handles (spec-fixit-forward-readbuf-leak D-1/D-2). Callers adopt on the
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

// EBGPWire returns a WireUpdate with the local ASN prepended to AS_PATH.
// RFC 4271 Section 9.1.2: EBGP speakers MUST prepend their own AS number.
//
// Lazy: first call per dstASN4 variant generates and caches the result.
// Subsequent calls return the cached pointer.
// Cache-hit path is lock-free (single atomic pointer load).
// Miss path uses double-checked locking under ebgpMu.
//
// Parameters:
//   - localASN: the local AS number to prepend
//   - srcASN4: whether the source UPDATE uses 4-byte ASN encoding
//   - dstASN4: whether the destination peer expects 4-byte ASN encoding
//
// The returned WireUpdate carries fwdContextIDWithASN4(SourceCtxID, dstASN4):
// the source NLRI framing with the AS width rewritten to what the destination
// wants. It equals the original SourceCtxID only when the source context
// already encodes at that width.
//
// No production caller reaches this method today. The AS-path fold (e2037e598)
// moved eBGP prepending onto the edit-set path, so both slots stay nil in a
// running daemon and only tests populate them. Deleting the cache is the work
// of plan/spec-wire-edit-3-deferred-ac9-dead-code.md.
func (u *ReceivedUpdate) EBGPWire(localASN uint32, srcASN4, dstASN4 bool) (*wireu.WireUpdate, error) {
	slot := u.ebgpSlot(dstASN4)

	// Fast path: atomic load, no lock.
	if s := slot.Load(); s != nil {
		return s.wire, nil
	}

	// Miss: generate under ebgpMu with double-checked locking.
	u.ebgpMu.Lock()
	defer u.ebgpMu.Unlock()

	if s := slot.Load(); s != nil {
		return s.wire, nil
	}

	payload := u.WireUpdate.Payload()
	extendedMessage := len(payload) > message.MaxMsgLen-message.HeaderLen
	dst := getReadBuf(extendedMessage)
	if dst.Buf == nil {
		return nil, errEbgpWireBufferExhaustedPoolAt
	}

	n, err := wireu.RewriteASPath(dst.Buf, payload, localASN, srcASN4, dstASN4)
	if err != nil {
		ReturnReadBuffer(dst)
		return nil, fmt.Errorf("EBGP wire rewrite: %w", err)
	}

	wu := wireu.NewWireUpdate(dst.Buf[:n], fwdContextIDWithASN4(u.WireUpdate.SourceCtxID(), dstASN4))
	wu.SetMessageID(u.WireUpdate.MessageID())
	wu.SetSourceID(u.WireUpdate.SourceID())

	slot.Store(&ebgpWireSlot{wire: wu, handle: dst})

	return wu, nil
}

// ebgpSlot returns the atomic slot pointer for the given ASN-width variant.
func (u *ReceivedUpdate) ebgpSlot(dstASN4 bool) *atomic.Pointer[ebgpWireSlot] {
	if dstASN4 {
		return &u.ebgpSlotASN4
	}
	return &u.ebgpSlotASN2
}
