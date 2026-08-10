// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- NetFlow v9 template FlowSet

package netflow9

import "encoding/binary"

// RFC 3954 Section 5.2: template IDs in range 256-65535.
const CounterTemplateID uint16 = 256

// FlowSet IDs per RFC 3954.
const (
	FlowSetIDTemplate = 0
)

// counterFields defines the interface counter template for NetFlow v9.
// Each entry is (field type ID, field length in bytes).
var counterFields = [][2]uint16{
	{10, 4}, // INPUT_SNMP: ingress ifIndex
	{1, 8},  // IN_BYTES: incoming byte counter
	{2, 4},  // IN_PKTS: incoming packet counter
	{23, 8}, // OUT_BYTES: outgoing byte counter
	{24, 4}, // OUT_PKTS: outgoing packet counter
	{14, 4}, // OUTPUT_SNMP: egress ifIndex
}

// CounterRecordSize returns the per-record byte size for the counter template.
func CounterRecordSize() int {
	size := 0
	for _, f := range counterFields {
		size += int(f[1])
	}
	return size
}

// counterFieldCount returns the number of fields in the counter template.
func counterFieldCount() int {
	return len(counterFields)
}

// BuildCounterTemplate pre-encodes the template FlowSet for interface
// counter records. Called once at config time; the returned bytes are
// copy()'d into each datagram when template refresh fires.
//
// RFC 3954 Section 5.2: Template FlowSet has FlowSet ID = 0.
// Layout: FlowSet ID (2) + Length (2) + Template ID (2) + Field Count (2)
// + N * (Field Type (2) + Field Length (2)) + padding to 4-byte boundary.
func BuildCounterTemplate() []byte {
	fieldCount := len(counterFields)
	// FlowSet header (4) + template header (4) + fields (4 each)
	recordLen := 4 + 4 + fieldCount*4
	// Pad to 4-byte boundary
	padded := recordLen
	if padded%4 != 0 {
		padded += 4 - padded%4
	}
	buf := make([]byte, padded)
	off := 0

	// FlowSet header
	binary.BigEndian.PutUint16(buf[off:], FlowSetIDTemplate)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(padded))
	off += 4

	// Template record header
	binary.BigEndian.PutUint16(buf[off:], CounterTemplateID)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(fieldCount))
	off += 4

	// Field specifiers
	for _, f := range counterFields {
		binary.BigEndian.PutUint16(buf[off:], f[0])
		binary.BigEndian.PutUint16(buf[off+2:], f[1])
		off += 4
	}

	return buf
}
