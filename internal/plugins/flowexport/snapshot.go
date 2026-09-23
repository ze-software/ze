// Design: docs/architecture/flowexport/flow-export-0-umbrella.md -- Counter snapshot value types

package flowexport

import "time"

// CounterSnapshot is a point-in-time capture of all interface counters.
// Value type: no pointers, safe to pass across component boundaries.
// The rateTracker.collect() notification delivers this to flowexport.
// Raw kernel counters (pre-baseline), not baseline-adjusted.
type CounterSnapshot struct {
	Time       time.Time
	Interfaces []InterfaceCounters
}

// InterfaceCounters holds raw packet totals and the 19 sFlow if_counters
// fields. The registration boundary converts iface.InterfaceInfo into it.
type InterfaceCounters struct {
	Name string

	// CounterGeneration identifies the raw source's counter continuity.
	// Zero means the source cannot supply generation metadata.
	CounterGeneration uint64

	// Raw packet totals retain all 64 bits for IPFIX and include multicast
	// and broadcast packets. They do not carry sFlow availability sentinels.
	InPackets  uint64
	OutPackets uint64

	// sFlow v5 if_counters (enterprise 0, format 1), XDR field order.
	IfIndex            uint32
	IfType             uint32
	IfSpeed            uint64 // bits per second
	IfDirection        uint32
	IfStatus           uint32 // bit 0: ifAdminStatus, bit 1: ifOperStatus
	IfInOctets         uint64
	IfInUcastPkts      uint32
	IfInMulticastPkts  uint32
	IfInBroadcastPkts  uint32
	IfInDiscards       uint32
	IfInErrors         uint32
	IfInUnknownProtos  uint32
	IfOutOctets        uint64
	IfOutUcastPkts     uint32
	IfOutMulticastPkts uint32
	IfOutBroadcastPkts uint32
	IfOutDiscards      uint32
	IfOutErrors        uint32
	IfPromiscuousMode  uint32 // 0=false, 1=true
}

// CounterUnavailable marks a 32-bit counter the source cannot observe. sFlow
// v5, "Unknown counter": "Use the maximum counter value to indicate that the
// counter is not available. Within any given sFlow session a particular
// counter must be always available, or always unavailable." The source
// writes it from what the kernel exposes, so availability is fixed for the
// session. An available counter can also reach this value before wrapping.
const CounterUnavailable = ^uint32(0)

// IfCountersSize is the sFlow v5 if_counters XDR record size.
// 16 x unsigned int (4 bytes) = 64, plus 3 x unsigned hyper (8 bytes) = 24.
const IfCountersSize = 88

const (
	IfDirectionUnknown    = 0
	IfDirectionFullDuplex = 1
	IfDirectionHalfDuplex = 2
	IfDirectionIn         = 3
	IfDirectionOut        = 4
)

const (
	IfStatusAdminUp = 1 << 0
	IfStatusOperUp  = 1 << 1
)
