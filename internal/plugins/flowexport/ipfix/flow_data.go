// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- IPFIX per-flow data Set
// Related: flow_adapter.go -- WriteFlowDataSet / WriteFlowDataSet6 callers

package ipfix

import (
	"encoding/binary"
	"net/netip"
)

// FlowRecord holds a single per-flow record for IPFIX export.
// Sourced from conntrack entries with optional BGP enrichment.
type FlowRecord struct {
	SrcAddr     netip.Addr
	DstAddr     netip.Addr
	SrcPort     uint16
	DstPort     uint16
	Protocol    uint8
	Bytes       uint64
	Packets     uint64
	SrcAS       uint32
	DstAS       uint32
	StartTimeMs uint64 // milliseconds since UNIX epoch
	EndTimeMs   uint64 // milliseconds since UNIX epoch
}

// WriteFlowDataSet encodes an IPFIX Data Set containing per-flow records.
// Uses skip-and-backfill for the Set Length field.
// Returns (bytes written, data record count).
//
// RFC 7011 Section 3.4.3: Data Set's Set ID equals the Template ID.
func WriteFlowDataSet(buf []byte, off int, flows []FlowRecord, templateID uint16) (int, uint32) {
	return writeFlowDataSet(buf, off, flows, templateID, FlowRecordSize(), writeFlowRecord)
}

// writeFlowDataSet6 encodes an IPFIX Data Set containing IPv6 per-flow records.
// Mirrors WriteFlowDataSet but uses the IPv6 record size and writer.
func writeFlowDataSet6(buf []byte, off int, flows []FlowRecord, templateID uint16) (int, uint32) {
	return writeFlowDataSet(buf, off, flows, templateID, FlowRecordSize6(), writeFlowRecord6)
}

// writeFlowDataSet encodes a Data Set for the given template using the
// supplied per-record size and writer. Shared by the IPv4 and IPv6 variants.
func writeFlowDataSet(buf []byte, off int, flows []FlowRecord, templateID uint16, recSize int, writeRec func([]byte, int, *FlowRecord)) (int, uint32) {
	start := off
	if recSize == 0 || len(flows) == 0 {
		return 0, 0
	}

	// Set Header: Set ID + Length (skip for backfill)
	binary.BigEndian.PutUint16(buf[off:], templateID)
	off += 2
	lengthPos := off
	off += 2

	var count uint32
	for i := range flows {
		if off+recSize > len(buf) {
			break
		}
		writeRec(buf, off, &flows[i])
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

	// Backfill Set Length.
	binary.BigEndian.PutUint16(buf[lengthPos:], uint16(off-start))

	return off - start, count
}

// writeFlowRecord writes a single IPv4 per-flow data record at the given
// offset. Field order matches flowTemplateFields in flow_template.go.
func writeFlowRecord(buf []byte, off int, fr *FlowRecord) {
	// IE 8: sourceIPv4Address (4 bytes)
	a4 := fr.SrcAddr.As4()
	copy(buf[off:], a4[:])
	off += 4

	// IE 12: destinationIPv4Address (4 bytes)
	a4 = fr.DstAddr.As4()
	copy(buf[off:], a4[:])
	off += 4

	writeFlowTail(buf, off, fr)
}

// writeFlowRecord6 writes a single IPv6 per-flow data record. Identical to
// writeFlowRecord except the address IEs are 16 bytes each.
// Field order matches flowTemplateFields6 in flow_template.go.
func writeFlowRecord6(buf []byte, off int, fr *FlowRecord) {
	// IE 27: sourceIPv6Address (16 bytes)
	a16 := fr.SrcAddr.As16()
	copy(buf[off:], a16[:])
	off += 16

	// IE 28: destinationIPv6Address (16 bytes)
	a16 = fr.DstAddr.As16()
	copy(buf[off:], a16[:])
	off += 16

	writeFlowTail(buf, off, fr)
}

// writeFlowTail writes the non-address IEs shared by the IPv4 and IPv6 flow
// records (ports through timestamps).
func writeFlowTail(buf []byte, off int, fr *FlowRecord) {
	// IE 7: sourceTransportPort (2 bytes)
	binary.BigEndian.PutUint16(buf[off:], fr.SrcPort)
	off += 2

	// IE 11: destinationTransportPort (2 bytes)
	binary.BigEndian.PutUint16(buf[off:], fr.DstPort)
	off += 2

	// IE 4: protocolIdentifier (1 byte)
	buf[off] = fr.Protocol
	off++

	// IE 1: octetDeltaCount (8 bytes)
	binary.BigEndian.PutUint64(buf[off:], fr.Bytes)
	off += 8

	// IE 2: packetDeltaCount (8 bytes)
	binary.BigEndian.PutUint64(buf[off:], fr.Packets)
	off += 8

	// IE 16: bgpSourceAsNumber (4 bytes)
	binary.BigEndian.PutUint32(buf[off:], fr.SrcAS)
	off += 4

	// IE 17: bgpDestinationAsNumber (4 bytes)
	binary.BigEndian.PutUint32(buf[off:], fr.DstAS)
	off += 4

	// IE 152: flowStartMilliseconds (8 bytes, dateTimeMilliseconds)
	binary.BigEndian.PutUint64(buf[off:], fr.StartTimeMs)
	off += 8

	// IE 153: flowEndMilliseconds (8 bytes, dateTimeMilliseconds)
	binary.BigEndian.PutUint64(buf[off:], fr.EndTimeMs)
}
