// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- sFlow v5 counter sample encoding

package sflow

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

const (
	// DataFormatCountersSample is the sFlow v5 counters_sample data_format (enterprise 0, format 2).
	DataFormatCountersSample = 0x00000002

	// DataFormatIfCounters is the sFlow v5 if_counters record data_format (enterprise 0, format 1).
	DataFormatIfCounters = 0x00000001

	// counterSampleHeaderSize is the fixed overhead of a counters_sample record (20 bytes).
	counterSampleHeaderSize = 20

	// ifCountersRecordHeaderSize is the overhead of the if_counters record wrapper (8 bytes).
	ifCountersRecordHeaderSize = 8
)

// counterSampleSize returns the total encoded size of one counters_sample
// containing a single if_counters record.
func counterSampleSize() int {
	return counterSampleHeaderSize + ifCountersRecordHeaderSize + flowexport.IfCountersSize
}

// writeCounterSample writes a counters_sample record into buf at off.
// The sample contains a single if_counters record for the given interface.
// Returns the new offset after the sample.
func writeCounterSample(buf []byte, off int, ifIndex, seqNum uint32, c *flowexport.InterfaceCounters) int {
	// sFlow v5: counters_sample data_format = enterprise 0, format 2
	binary.BigEndian.PutUint32(buf[off:], DataFormatCountersSample)
	off += 4

	// Skip-and-backfill: sample_length (bytes following this field)
	sampleLengthOff := off
	off += 4

	// sFlow v5: per-source sequence number
	binary.BigEndian.PutUint32(buf[off:], seqNum)
	off += 4

	// sFlow v5: source_id = type (high 8 bits) | index (low 24 bits)
	// type 0 = ifIndex
	sourceID := ifIndex & 0x00FFFFFF
	binary.BigEndian.PutUint32(buf[off:], sourceID)
	off += 4

	// sFlow v5: num_records = 1 (single if_counters record)
	binary.BigEndian.PutUint32(buf[off:], 1)
	off += 4

	off = writeIfCounters(buf, off, c)

	// sFlow v5: backfill sample_length
	sampleLength := uint32(off - sampleLengthOff - 4)
	binary.BigEndian.PutUint32(buf[sampleLengthOff:], sampleLength)

	return off
}

// writeIfCounters writes the if_counters record (enterprise 0, format 1)
// into buf at off. The record contains 19 fields matching the SNMP
// ifTable/ifXTable MIB objects. Returns the new offset after the record.
func writeIfCounters(buf []byte, off int, c *flowexport.InterfaceCounters) int {
	// sFlow v5: if_counters record_data_format = enterprise 0, format 1
	binary.BigEndian.PutUint32(buf[off:], DataFormatIfCounters)
	off += 4

	// sFlow v5: if_counters record_length = 88 bytes (fixed)
	binary.BigEndian.PutUint32(buf[off:], flowexport.IfCountersSize)
	off += 4

	// sFlow v5 Section "Generic Interface Counters": 19 fields in order

	// 1. ifIndex (uint32)
	binary.BigEndian.PutUint32(buf[off:], c.IfIndex)
	off += 4

	// 2. ifType (uint32) - IANA ifType
	binary.BigEndian.PutUint32(buf[off:], c.IfType)
	off += 4

	// 3. ifSpeed (uint64) - bits/sec
	binary.BigEndian.PutUint64(buf[off:], c.IfSpeed)
	off += 8

	// 4. ifDirection (uint32) - 0=unknown, 1=full-duplex, 2=half-duplex
	binary.BigEndian.PutUint32(buf[off:], c.IfDirection)
	off += 4

	// 5. ifStatus (uint32) - bit 0: ifAdminStatus, bit 1: ifOperStatus
	binary.BigEndian.PutUint32(buf[off:], c.IfStatus)
	off += 4

	// 6. ifInOctets (uint64) - ifHCInOctets
	binary.BigEndian.PutUint64(buf[off:], c.IfInOctets)
	off += 8

	// 7. ifInUcastPkts (uint32)
	binary.BigEndian.PutUint32(buf[off:], c.IfInUcastPkts)
	off += 4

	// 8. ifInMulticastPkts (uint32)
	binary.BigEndian.PutUint32(buf[off:], c.IfInMulticastPkts)
	off += 4

	// 9. ifInBroadcastPkts (uint32)
	binary.BigEndian.PutUint32(buf[off:], c.IfInBroadcastPkts)
	off += 4

	// 10. ifInDiscards (uint32)
	binary.BigEndian.PutUint32(buf[off:], c.IfInDiscards)
	off += 4

	// 11. ifInErrors (uint32)
	binary.BigEndian.PutUint32(buf[off:], c.IfInErrors)
	off += 4

	// 12. ifInUnknownProtos (uint32)
	binary.BigEndian.PutUint32(buf[off:], c.IfInUnknownProtos)
	off += 4

	// 13. ifOutOctets (uint64) - ifHCOutOctets
	binary.BigEndian.PutUint64(buf[off:], c.IfOutOctets)
	off += 8

	// 14. ifOutUcastPkts (uint32)
	binary.BigEndian.PutUint32(buf[off:], c.IfOutUcastPkts)
	off += 4

	// 15. ifOutMulticastPkts (uint32)
	binary.BigEndian.PutUint32(buf[off:], c.IfOutMulticastPkts)
	off += 4

	// 16. ifOutBroadcastPkts (uint32)
	binary.BigEndian.PutUint32(buf[off:], c.IfOutBroadcastPkts)
	off += 4

	// 17. ifOutDiscards (uint32)
	binary.BigEndian.PutUint32(buf[off:], c.IfOutDiscards)
	off += 4

	// 18. ifOutErrors (uint32)
	binary.BigEndian.PutUint32(buf[off:], c.IfOutErrors)
	off += 4

	// 19. ifPromiscuousMode (uint32) - 0=false, 1=true
	binary.BigEndian.PutUint32(buf[off:], c.IfPromiscuousMode)
	off += 4

	return off
}
