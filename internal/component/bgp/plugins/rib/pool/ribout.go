// Design: docs/architecture/pool-architecture.md — Adj-RIB-Out wire attribute pool

package pool

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"
)

// RibOut pools wire attribute bytes for Adj-RIB-Out deduplication.
// The same UPDATE sent to N peers stores one pool copy and N handles.
var RibOut *attrpool.Pool

func init() {
	RibOut = mustPool(16, 1<<18, shardsHot) // full wire attribute blobs, maximally diverse
}
