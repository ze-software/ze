// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- Conntrack flow entry types

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
	// TCPState is the conntrack TCP connection state (nf_conntrack_tcp:
	// 1=SYN_SENT, 2=SYN_RECV, 3=ESTABLISHED, ...) for TCP flows, 0 otherwise.
	// A SYN flood shows as a dominance of half-open states (SYN_SENT/SYN_RECV);
	// the DDoS characterizer reads this via the recent-flow ring. Only the
	// periodic-dump path populates it (destroy events carry no protoinfo).
	TCPState uint8
}
