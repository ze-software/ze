// Design: docs/architecture/core-design.md — BGP reactor event loop

package reactor

import (
	"net/netip"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wireu"
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
// Memory contract: WireUpdate slices into poolBuf; all derived slices share it.
// When cache evicts this entry, poolBuf is returned to the session buffer pool.
// Message ID is stored in WireUpdate, accessible via WireUpdate.MessageID().
type ReceivedUpdate struct {
	// WireUpdate contains the UPDATE payload with zero-copy accessors.
	// Provides Payload(), Attrs(), NLRI(), MPReach(), MPUnreach(), SourceCtxID(), MessageID().
	WireUpdate *wireu.WireUpdate

	// poolBuf is the session read buffer that WireUpdate slices into.
	// Returned to pool when cache evicts this entry.
	poolBuf []byte

	// SourcePeerIP is the IP address of the peer that sent this UPDATE.
	SourcePeerIP netip.Addr

	// ReceivedAt is when this UPDATE was received.
	ReceivedAt time.Time
}
