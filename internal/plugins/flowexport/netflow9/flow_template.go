// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- NetFlow v9 per-flow template
// Related: flow_adapter.go -- builds export packets from these templates

package netflow9

import "encoding/binary"

// RFC 3954 Section 5.2: template IDs in range 256-65535.
// FlowTemplateID is distinct from CounterTemplateID (256).
const FlowTemplateID uint16 = 257

// FlowTemplateID6 is the per-flow template for IPv6 flows. Distinct template
// ID because IPV6_SRC_ADDR / IPV6_DST_ADDR (16 bytes) differ from the IPv4
// address fields; a separate template means IPv6 records ship in their own
// datagram.
const FlowTemplateID6 uint16 = 258

// flowFields defines the per-flow template for 5-tuple + counters + timestamps.
// Each entry is (field type ID, field length in bytes).
var flowFields = [][2]uint16{
	{8, 4},  // IPV4_SRC_ADDR
	{12, 4}, // IPV4_DST_ADDR
	{7, 2},  // L4_SRC_PORT
	{11, 2}, // L4_DST_PORT
	{4, 1},  // PROTOCOL
	{1, 8},  // IN_BYTES
	{2, 4},  // IN_PKTS
	{16, 4}, // SRC_AS
	{17, 4}, // DST_AS
	{22, 4}, // FIRST_SWITCHED (sysUpTime ms)
	{21, 4}, // LAST_SWITCHED (sysUpTime ms)
}

// flowFields6 mirrors flowFields but uses IPv6 address fields (16 bytes each).
// All non-address fields are identical to the IPv4 template.
var flowFields6 = [][2]uint16{
	{27, 16}, // IPV6_SRC_ADDR
	{28, 16}, // IPV6_DST_ADDR
	{7, 2},   // L4_SRC_PORT
	{11, 2},  // L4_DST_PORT
	{4, 1},   // PROTOCOL
	{1, 8},   // IN_BYTES
	{2, 4},   // IN_PKTS
	{16, 4},  // SRC_AS
	{17, 4},  // DST_AS
	{22, 4},  // FIRST_SWITCHED (sysUpTime ms)
	{21, 4},  // LAST_SWITCHED (sysUpTime ms)
}

// FlowRecordSize returns the per-record byte size for per-flow data records.
func FlowRecordSize() int {
	size := 0
	for _, f := range flowFields {
		size += int(f[1])
	}
	return size
}

// FlowRecordSize6 returns the per-record byte size for IPv6 per-flow records.
func FlowRecordSize6() int {
	size := 0
	for _, f := range flowFields6 {
		size += int(f[1])
	}
	return size
}

// FlowFieldCount returns the number of fields in the flow template.
func FlowFieldCount() int {
	return len(flowFields)
}

// FlowFieldCount6 returns the number of fields in the IPv6 flow template.
func FlowFieldCount6() int {
	return len(flowFields6)
}

// BuildFlowTemplate pre-encodes the template FlowSet for per-flow records.
// Called once at config time; the returned bytes are copy()'d into each
// datagram when template refresh fires.
//
// RFC 3954: Template FlowSet has FlowSet ID = 0.
func BuildFlowTemplate() []byte {
	fieldCount := len(flowFields)
	recordLen := 4 + 4 + fieldCount*4
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
	binary.BigEndian.PutUint16(buf[off:], FlowTemplateID)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(fieldCount))
	off += 4

	// Field specifiers
	for _, f := range flowFields {
		binary.BigEndian.PutUint16(buf[off:], f[0])
		binary.BigEndian.PutUint16(buf[off+2:], f[1])
		off += 4
	}

	return buf
}

// BuildFlowTemplate6 pre-encodes the IPv6 per-flow template FlowSet. Mirrors
// BuildFlowTemplate but uses FlowTemplateID6 and flowFields6.
//
// RFC 3954: Template FlowSet has FlowSet ID = 0.
func BuildFlowTemplate6() []byte {
	fieldCount := len(flowFields6)
	recordLen := 4 + 4 + fieldCount*4
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
	binary.BigEndian.PutUint16(buf[off:], FlowTemplateID6)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(fieldCount))
	off += 4

	// Field specifiers
	for _, f := range flowFields6 {
		binary.BigEndian.PutUint16(buf[off:], f[0])
		binary.BigEndian.PutUint16(buf[off+2:], f[1])
		off += 4
	}

	return buf
}
