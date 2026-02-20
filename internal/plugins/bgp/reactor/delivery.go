package reactor

import (
	"codeberg.org/thomas-mangin/ze/internal/plugin"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
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
