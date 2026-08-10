// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- NetFlow v9 per-flow data FlowSet
// Related: flow_adapter.go -- WriteFlowDataFlowSet / WriteFlowDataFlowSet6 callers

package netflow9

import (
	"encoding/binary"
	"net/netip"
)

// FlowRecord holds a single per-flow record for NetFlow v9 export.
// Sourced from conntrack entries with optional BGP enrichment.
type FlowRecord struct {
	SrcAddr       netip.Addr
	DstAddr       netip.Addr
	SrcPort       uint16
	DstPort       uint16
	Protocol      uint8
	Bytes         uint64
	Packets       uint32
	SrcAS         uint32
	DstAS         uint32
	FirstSwitched uint32 // sysUpTime in ms at first packet
	LastSwitched  uint32 // sysUpTime in ms at last packet
}

// writeFlowDataFlowSet writes a data FlowSet for per-flow records
// into buf at the given offset. Returns (bytes written, record count).
//
// RFC 3954: Data FlowSet ID = template ID (257 for flow records).
func writeFlowDataFlowSet(buf []byte, off int, flows []FlowRecord) (int, uint16) {
	start := off
	recSize := FlowRecordSize()

	// FlowSet header: ID + length (skip for backfill)
	binary.BigEndian.PutUint16(buf[off:], FlowTemplateID)
	lengthPos := off + 2
	off += FlowSetHeaderSize

	var count uint16
	for i := range flows {
		if off+recSize > len(buf) {
			break
		}
		off += writeFlowRecord(buf, off, &flows[i])
		count++
	}

	// Pad to 4-byte boundary
	dataLen := off - start
	pad := (4 - dataLen%4) % 4
	for range pad {
		buf[off] = 0
		off++
	}

	totalLen := off - start
	binary.BigEndian.PutUint16(buf[lengthPos:], uint16(totalLen))

	return totalLen, count
}

// writeFlowDataFlowSet6 writes a data FlowSet for IPv6 per-flow records into
// buf at the given offset. Returns (bytes written, record count).
//
// RFC 3954: Data FlowSet ID = template ID (258 for IPv6 flow records).
func writeFlowDataFlowSet6(buf []byte, off int, flows []FlowRecord) (int, uint16) {
	start := off
	recSize := FlowRecordSize6()

	// FlowSet header: ID + length (skip for backfill)
	binary.BigEndian.PutUint16(buf[off:], FlowTemplateID6)
	lengthPos := off + 2
	off += FlowSetHeaderSize

	var count uint16
	for i := range flows {
		if off+recSize > len(buf) {
			break
		}
		off += writeFlowRecord6(buf, off, &flows[i])
		count++
	}

	// Pad to 4-byte boundary
	dataLen := off - start
	pad := (4 - dataLen%4) % 4
	for range pad {
		buf[off] = 0
		off++
	}

	totalLen := off - start
	binary.BigEndian.PutUint16(buf[lengthPos:], uint16(totalLen))

	return totalLen, count
}

// writeFlowRecord writes one per-flow data record at off.
// Field order matches the flow template defined in flow_template.go.
func writeFlowRecord(buf []byte, off int, fr *FlowRecord) int {
	start := off

	// IPV4_SRC_ADDR (4 bytes)
	a4 := fr.SrcAddr.As4()
	copy(buf[off:], a4[:])
	off += 4

	// IPV4_DST_ADDR (4 bytes)
	a4 = fr.DstAddr.As4()
	copy(buf[off:], a4[:])
	off += 4

	off += writeFlowTail(buf, off, fr)
	return off - start
}

// writeFlowRecord6 writes one IPv6 per-flow data record at off. Identical to
// writeFlowRecord except the two address fields are 16 bytes each.
// Field order matches flowFields6 in flow_template.go.
func writeFlowRecord6(buf []byte, off int, fr *FlowRecord) int {
	start := off

	// IPV6_SRC_ADDR (16 bytes)
	a16 := fr.SrcAddr.As16()
	copy(buf[off:], a16[:])
	off += 16

	// IPV6_DST_ADDR (16 bytes)
	a16 = fr.DstAddr.As16()
	copy(buf[off:], a16[:])
	off += 16

	off += writeFlowTail(buf, off, fr)
	return off - start
}

// writeFlowTail writes the non-address fields shared by the IPv4 and IPv6
// flow records (ports through timestamps). Returns bytes written.
func writeFlowTail(buf []byte, off int, fr *FlowRecord) int {
	start := off

	// L4_SRC_PORT (2 bytes)
	binary.BigEndian.PutUint16(buf[off:], fr.SrcPort)
	off += 2

	// L4_DST_PORT (2 bytes)
	binary.BigEndian.PutUint16(buf[off:], fr.DstPort)
	off += 2

	// PROTOCOL (1 byte)
	buf[off] = fr.Protocol
	off++

	// IN_BYTES (8 bytes)
	binary.BigEndian.PutUint64(buf[off:], fr.Bytes)
	off += 8

	// IN_PKTS (4 bytes)
	binary.BigEndian.PutUint32(buf[off:], fr.Packets)
	off += 4

	// SRC_AS (4 bytes)
	binary.BigEndian.PutUint32(buf[off:], fr.SrcAS)
	off += 4

	// DST_AS (4 bytes)
	binary.BigEndian.PutUint32(buf[off:], fr.DstAS)
	off += 4

	// FIRST_SWITCHED (4 bytes, sysUpTime ms)
	binary.BigEndian.PutUint32(buf[off:], fr.FirstSwitched)
	off += 4

	// LAST_SWITCHED (4 bytes, sysUpTime ms)
	binary.BigEndian.PutUint32(buf[off:], fr.LastSwitched)
	off += 4

	return off - start
}
