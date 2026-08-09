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

// InterfaceCounters holds all 19 sFlow if_counters fields plus interface
// identity. Fields map to SNMP ifTable/ifXTable (RFC 2863) as required
// by sFlow v5. Callers convert from iface.InterfaceInfo into this type
// at the boundary; flowexport does not import the iface package.
type InterfaceCounters struct {
	Name string

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
