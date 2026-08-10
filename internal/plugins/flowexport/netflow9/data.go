// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- NetFlow v9 data FlowSet encoding

package netflow9

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// FlowSetHeaderSize is the size of a FlowSet header (ID + Length).
const FlowSetHeaderSize = 4

// writeDataFlowSet writes a data FlowSet for interface counter records
// into buf at the given offset. Returns (bytes written, record count).
//
// RFC 3954: Data FlowSet ID = the template ID it references (256+).
// Layout: FlowSet ID (2) + Length (2) + records + padding.
// Length is backfilled after all records are written.
func writeDataFlowSet(buf []byte, off int, templateID uint16, ifaces []flowexport.InterfaceCounters) (int, uint16) {
	start := off
	recSize := CounterRecordSize()

	// FlowSet header: skip length, backfill later
	binary.BigEndian.PutUint16(buf[off:], templateID)
	lengthPos := off + 2
	off += FlowSetHeaderSize

	var count uint16
	for i := range ifaces {
		if off+recSize > len(buf) {
			break
		}
		off += writeCounterRecord(buf, off, &ifaces[i])
		count++
	}

	// Pad to 4-byte boundary
	dataLen := off - start
	pad := (4 - dataLen%4) % 4
	for range pad {
		buf[off] = 0
		off++
	}

	// Backfill FlowSet length (includes header + records + padding)
	totalLen := off - start
	binary.BigEndian.PutUint16(buf[lengthPos:], uint16(totalLen))

	return totalLen, count
}

// writeCounterRecord writes one interface counter data record at off.
// Field order matches the counter template defined in template.go:
// INPUT_SNMP(4), IN_BYTES(8), IN_PKTS(4), OUT_BYTES(8), OUT_PKTS(4), OUTPUT_SNMP(4).
func writeCounterRecord(buf []byte, off int, ic *flowexport.InterfaceCounters) int {
	start := off

	// INPUT_SNMP (4 bytes): ingress ifIndex
	binary.BigEndian.PutUint32(buf[off:], ic.IfIndex)
	off += 4

	// IN_BYTES (8 bytes): incoming byte counter
	binary.BigEndian.PutUint64(buf[off:], ic.IfInOctets)
	off += 8

	// IN_PKTS (4 bytes): incoming packet counter
	binary.BigEndian.PutUint32(buf[off:], ic.IfInUcastPkts)
	off += 4

	// OUT_BYTES (8 bytes): outgoing byte counter
	binary.BigEndian.PutUint64(buf[off:], ic.IfOutOctets)
	off += 8

	// OUT_PKTS (4 bytes): outgoing packet counter
	binary.BigEndian.PutUint32(buf[off:], ic.IfOutUcastPkts)
	off += 4

	// OUTPUT_SNMP (4 bytes): egress ifIndex (same as ingress for counter export)
	binary.BigEndian.PutUint32(buf[off:], ic.IfIndex)
	off += 4

	return off - start
}
