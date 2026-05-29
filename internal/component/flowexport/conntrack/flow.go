// Design: plan/spec-flow-export-2-flow-records.md -- Conntrack flow entry types

package conntrack

import (
	"net/netip"
	"time"
)

// FlowEntry represents a single conntrack flow with 5-tuple, counters,
// and timestamps. Value type: safe to copy and pass across boundaries.
// Bytes and Packets are the sum of both directions (original + reply).
type FlowEntry struct {
	SrcAddr   netip.Addr
	DstAddr   netip.Addr
	SrcPort   uint16
	DstPort   uint16
	Protocol  uint8
	Bytes     uint64
	Packets   uint64
	StartTime time.Time
	LastSeen  time.Time
	Mark      uint32
	ID        uint32
}
