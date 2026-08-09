// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- IPFIX per-flow template
// Related: flow_adapter.go -- builds IPFIX messages from these templates

package ipfix

import "encoding/binary"

// FlowTemplateID is the template ID for per-flow records.
// Distinct from CounterTemplateID (256).
// RFC 7011 Section 3.4.1: Template IDs MUST be greater than 255.
const FlowTemplateID = 257

// FlowTemplateID6 is the per-flow template for IPv6 flows. Distinct template
// ID because sourceIPv6Address / destinationIPv6Address (16 bytes) differ from
// the IPv4 address IEs; IPv6 records ship in their own Data Set.
const FlowTemplateID6 = 258

// flowTemplateFields defines the per-flow template using IANA IEs.
// Each entry is (IE ID, field length in bytes).
var flowTemplateFields = [][2]uint16{
	{IESourceIPv4Address, 4},
	{IEDestinationIPv4Address, 4},
	{IESourceTransportPort, 2},
	{IEDestinationTransportPort, 2},
	{IEProtocolIdentifier, 1},
	{IEOctetDeltaCount, 8},
	{IEPacketDeltaCount, 8},
	{IEBgpSourceAsNumber, 4},
	{IEBgpDestinationAsNumber, 4},
	{IEFlowStartMilliseconds, 8},
	{IEFlowEndMilliseconds, 8},
}

// flowTemplateFields6 mirrors flowTemplateFields but uses IPv6 address IEs
// (16 bytes each). All non-address fields are identical to the IPv4 template.
var flowTemplateFields6 = [][2]uint16{
	{IESourceIPv6Address, 16},
	{IEDestinationIPv6Address, 16},
	{IESourceTransportPort, 2},
	{IEDestinationTransportPort, 2},
	{IEProtocolIdentifier, 1},
	{IEOctetDeltaCount, 8},
	{IEPacketDeltaCount, 8},
	{IEBgpSourceAsNumber, 4},
	{IEBgpDestinationAsNumber, 4},
	{IEFlowStartMilliseconds, 8},
	{IEFlowEndMilliseconds, 8},
}

// FlowRecordSize returns the per-record byte size for per-flow data records.
func FlowRecordSize() int {
	size := 0
	for _, f := range flowTemplateFields {
		size += int(f[1])
	}
	return size
}

// FlowRecordSize6 returns the per-record byte size for IPv6 per-flow records.
func FlowRecordSize6() int {
	size := 0
	for _, f := range flowTemplateFields6 {
		size += int(f[1])
	}
	return size
}

// FlowFieldCount returns the number of fields in the flow template.
func FlowFieldCount() int {
	return len(flowTemplateFields)
}

// FlowFieldCount6 returns the number of fields in the IPv6 flow template.
func FlowFieldCount6() int {
	return len(flowTemplateFields6)
}

// BuildFlowTemplate pre-encodes the IPFIX Template Set for per-flow records.
// Called at config time; the returned bytes are copied into each datagram
// when the template refresh timer fires.
//
// RFC 7011 Section 3.4.1: Template Set has Set ID = 2.
func BuildFlowTemplate() []byte {
	return buildFlowTemplate(FlowTemplateID, flowTemplateFields)
}

// BuildFlowTemplate6 pre-encodes the IPFIX Template Set for IPv6 per-flow
// records. Mirrors BuildFlowTemplate but uses FlowTemplateID6 and the IPv6
// field set.
func BuildFlowTemplate6() []byte {
	return buildFlowTemplate(FlowTemplateID6, flowTemplateFields6)
}

// buildFlowTemplate encodes a Template Set for the given template ID and field
// set. Shared by the IPv4 and IPv6 builders.
func buildFlowTemplate(templateID uint16, fields [][2]uint16) []byte {
	fieldCount := len(fields)
	setLength := 4 + 4 + fieldCount*4
	buf := make([]byte, setLength)
	off := 0

	// Set Header
	binary.BigEndian.PutUint16(buf[off:], TemplateSetID)
	off += 2
	binary.BigEndian.PutUint16(buf[off:], uint16(setLength))
	off += 2

	// Template Record Header
	binary.BigEndian.PutUint16(buf[off:], templateID)
	off += 2
	binary.BigEndian.PutUint16(buf[off:], uint16(fieldCount))
	off += 2

	// Field Specifiers: E=0 (bit 15 clear) for all IANA IEs.
	for _, f := range fields {
		binary.BigEndian.PutUint16(buf[off:], f[0])
		off += 2
		binary.BigEndian.PutUint16(buf[off:], f[1])
		off += 2
	}

	return buf
}
