// Design: rfc/short/rfc7011.md -- IPFIX Data Set encoding

package ipfix

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// WriteDataSet encodes an IPFIX Data Set containing interface counter
// records. Uses skip-and-backfill for the Set Length field.
// Returns (bytes written, data record count).
//
// RFC 7011 Section 3.4.3: Data Set's Set ID equals the Template ID.
// RFC 7011 Section 3.3.1: padding MUST be zeros; padding length MUST
// be shorter than any allowable record in this Set.
func WriteDataSet(buf []byte, off int, templateID uint16, ifaces []flowexport.InterfaceCounters, startTime, endTime uint32) (int, uint32) {
	start := off
	recSize := CounterRecordSize()
	if recSize == 0 || len(ifaces) == 0 {
		return 0, 0
	}

	// Set Header: Set ID + Length (skip for backfill)
	binary.BigEndian.PutUint16(buf[off:], templateID)
	off += 2
	lengthPos := off
	off += 2 // skip length

	var count uint32
	for i := range ifaces {
		// Bounds guard: callers pre-chunk to fit, but match the defensive
		// stop used by netflow9/data.go and ipfix/flow_data.go so an
		// oversized batch truncates rather than writing past the buffer.
		if off+recSize > len(buf) {
			break
		}
		ifc := &ifaces[i]
		writeCounterRecord(buf, off, ifc, startTime, endTime)
		off += recSize
		count++
	}

	// Pad to 4-byte alignment.
	padLen := (4 - (off-start)%4) % 4
	if padLen > 0 && padLen < recSize {
		for p := range padLen {
			buf[off+p] = 0
		}
		off += padLen
	}

	// Backfill Set Length (includes header and padding).
	binary.BigEndian.PutUint16(buf[lengthPos:], uint16(off-start))

	return off - start, count
}

// writeCounterRecord writes a single counter data record at the given
// offset. Field order matches the counter template defined in
// template.go: ingressInterface, octetTotalCount, packetTotalCount,
// egressInterface, flowStartSeconds, flowEndSeconds.
func writeCounterRecord(buf []byte, off int, ifc *flowexport.InterfaceCounters, startTime, endTime uint32) {
	// IE 10: ingressInterface (4 bytes)
	binary.BigEndian.PutUint32(buf[off:], ifc.IfIndex)
	off += 4

	// IE 85: octetTotalCount (8 bytes) -- rx + tx combined, cumulative
	binary.BigEndian.PutUint64(buf[off:], ifc.IfInOctets+ifc.IfOutOctets)
	off += 8

	// IE 86: packetTotalCount (8 bytes) -- rx + tx combined, cumulative
	totalPkts := uint64(ifc.IfInUcastPkts) + uint64(ifc.IfInMulticastPkts) + uint64(ifc.IfInBroadcastPkts) +
		uint64(ifc.IfOutUcastPkts) + uint64(ifc.IfOutMulticastPkts) + uint64(ifc.IfOutBroadcastPkts)
	binary.BigEndian.PutUint64(buf[off:], totalPkts)
	off += 8

	// IE 14: egressInterface (4 bytes) -- same as ingress for per-interface counters
	binary.BigEndian.PutUint32(buf[off:], ifc.IfIndex)
	off += 4

	// IE 150: flowStartSeconds (4 bytes, dateTimeSeconds)
	binary.BigEndian.PutUint32(buf[off:], startTime)
	off += 4

	// IE 151: flowEndSeconds (4 bytes, dateTimeSeconds)
	binary.BigEndian.PutUint32(buf[off:], endTime)
}
