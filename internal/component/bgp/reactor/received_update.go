// Design: docs/architecture/core-design.md — immutable received UPDATE snapshot
// Related: recent_cache.go — RecentUpdateCache stores ReceivedUpdate entries
// Related: reactor_notify.go — creates ReceivedUpdate on inbound UPDATE

package reactor

import (
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
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
// EBGP pool buffers (ebgpPoolBuf4, ebgpPoolBuf2) are returned on cache eviction too.
// Message ID is stored in WireUpdate, accessible via WireUpdate.MessageID().
type ReceivedUpdate struct {
	// WireUpdate contains the UPDATE payload with zero-copy accessors.
	// Provides Payload(), Attrs(), NLRI(), MPReach(), MPUnreach(), SourceCtxID(), MessageID().
	WireUpdate *wireu.WireUpdate

	// poolBuf is the session read buffer handle that WireUpdate slices into.
	// Returned to multiplexer when cache evicts this entry.
	poolBuf BufHandle

	// SourcePeerIP is the IP address of the peer that sent this UPDATE.
	SourcePeerIP netip.Addr

	// ReceivedAt is when this UPDATE was received.
	ReceivedAt time.Time

	// Meta holds route metadata set at ingress by filters.
	// Read-only after creation. May be nil if no filter set metadata.
	Meta map[string]any

	// ebgpMu protects lazy EBGP wire generation.
	ebgpMu sync.Mutex

	// ebgpWireASN4 is the lazily-generated EBGP wire version with 4-byte ASN encoding.
	// Cached after first call to EBGPWire(_, _, true).
	ebgpWireASN4 *wireu.WireUpdate

	// ebgpWireASN2 is the lazily-generated EBGP wire version with 2-byte ASN encoding.
	// Cached after first call to EBGPWire(_, _, false).
	ebgpWireASN2 *wireu.WireUpdate

	// ebgpPoolBuf4 is the pool buffer handle backing ebgpWireASN4.
	// Returned to multiplexer on cache eviction.
	ebgpPoolBuf4 BufHandle

	// ebgpPoolBuf2 is the pool buffer handle backing ebgpWireASN2.
	// Returned to multiplexer on cache eviction.
	ebgpPoolBuf2 BufHandle
}

// getReadBuf gets a buffer handle from the appropriate multiplexer.
// Uses the same multiplexers as session reads for uniform lifecycle management.
func getReadBuf(extendedMessage bool) BufHandle {
	if extendedMessage {
		return bufMux64K.Get()
	}
	return bufMux4K.Get()
}

// EBGPWire returns a WireUpdate with the local ASN prepended to AS_PATH.
// RFC 4271 Section 9.1.2: EBGP speakers MUST prepend their own AS number.
//
// Lazy: first call per dstASN4 variant generates and caches the result.
// Subsequent calls return the cached pointer. Thread-safe via ebgpMu.
//
// Parameters:
//   - localASN: the local AS number to prepend
//   - srcASN4: whether the source UPDATE uses 4-byte ASN encoding
//   - dstASN4: whether the destination peer expects 4-byte ASN encoding
//
// The returned WireUpdate shares the original SourceCtxID for zero-copy
// compatibility checks with other peers using the same encoding context.
func (u *ReceivedUpdate) EBGPWire(localASN uint32, srcASN4, dstASN4 bool) (*wireu.WireUpdate, error) {
	u.ebgpMu.Lock()
	defer u.ebgpMu.Unlock()

	// Check cache
	if dstASN4 {
		if u.ebgpWireASN4 != nil {
			return u.ebgpWireASN4, nil
		}
	} else {
		if u.ebgpWireASN2 != nil {
			return u.ebgpWireASN2, nil
		}
	}

	// Generate patched payload
	payload := u.WireUpdate.Payload()

	// Use extended multiplexer if original payload is large
	extendedMessage := len(payload) > message.MaxMsgLen-message.HeaderLen
	dst := getReadBuf(extendedMessage)
	if dst.Buf == nil {
		return nil, fmt.Errorf("EBGP wire buffer exhausted: pool at maximum allocation")
	}

	n, err := wireu.RewriteASPath(dst.Buf, payload, localASN, srcASN4, dstASN4)
	if err != nil {
		ReturnReadBuffer(dst) // Return handle on error
		return nil, fmt.Errorf("EBGP wire rewrite: %w", err)
	}

	// Wrap in WireUpdate with same context ID as original
	wu := wireu.NewWireUpdate(dst.Buf[:n], u.WireUpdate.SourceCtxID())
	wu.SetMessageID(u.WireUpdate.MessageID())
	wu.SetSourceID(u.WireUpdate.SourceID())

	// Cache result
	if dstASN4 {
		u.ebgpWireASN4 = wu
		u.ebgpPoolBuf4 = dst
	} else {
		u.ebgpWireASN2 = wu
		u.ebgpPoolBuf2 = dst
	}

	return wu, nil
}
