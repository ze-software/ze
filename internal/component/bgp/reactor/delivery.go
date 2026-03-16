// Design: docs/architecture/core-design.md — BGP reactor event loop
// Overview: peer.go — per-peer delivery goroutine that calls drainDeliveryBatch

package reactor

import (
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// deliveryItem represents an event to be delivered asynchronously
// by the per-peer delivery goroutine.
//
// Only received UPDATE messages use async delivery via the per-peer
// delivery channel. Other message types (OPEN, KEEPALIVE, NOTIFICATION)
// are delivered synchronously on the read goroutine because they are
// infrequent and FSM-critical.
type deliveryItem struct {
	peerInfo plugin.PeerInfo
	msg      bgptypes.RawMessage
}

// deliveryChannelCapacity is the default per-peer delivery channel size.
// With parallel fan-out, normal plugin RTT is <1ms, so 256-deep buffer
// sustains ~256K UPDATEs/sec before backpressure engages.
const deliveryChannelCapacity = 256

// drainDeliveryBatch collects the first item plus any additional items available
// without blocking. Same pattern as process.drainBatch but for per-peer delivery.
// buf is a reusable slice from the caller — reset to [:0] and returned for reuse.
func drainDeliveryBatch(buf []deliveryItem, first *deliveryItem, ch <-chan deliveryItem) []deliveryItem {
	buf = append(buf[:0], *first)
	for {
		select {
		case item, ok := <-ch:
			if !ok {
				return buf
			}
			buf = append(buf, item)
		default: // non-blocking drain complete
			return buf
		}
	}
}
