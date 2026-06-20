// Design: rfc/short/rfc7011.md -- IPFIX Template Set encoding
// Related: ie.go -- Information Element ID constants

package ipfix

import "encoding/binary"

// CounterTemplateID is the template ID for interface counter records.
// RFC 7011 Section 3.4.1: Template IDs MUST be greater than 255.
const CounterTemplateID = 256

// TemplateSetID identifies a Template Set.
// RFC 7011 Section 3.3.2: Set ID 2 = Template Set.
const TemplateSetID = 2

// counterTemplateFields defines the fields in the counter template,
// ordered as (IE ID, field length in bytes).
// Interface counters are raw cumulative kernel values, so the template uses
// the Total counter IEs (85/86), not the Delta IEs (1/2). See ie.go.
var counterTemplateFields = [][2]uint16{
	{IEIngressInterface, 4},
	{IEOctetTotalCount, 8},
	{IEPacketTotalCount, 8},
	{IEEgressInterface, 4},
	{IEFlowStartSeconds, 4},
	{IEFlowEndSeconds, 4},
}

// CounterRecordSize returns the per-record byte size for counter data
// records as defined by the counter template fields.
func CounterRecordSize() int {
	size := 0
	for _, f := range counterTemplateFields {
		size += int(f[1])
	}
	return size
}

// BuildCounterTemplate pre-encodes the IPFIX Template Set for interface
// counter records. Called at config time; the returned bytes are copied
// into each datagram when the template refresh timer fires.
//
// Wire layout (RFC 7011 Section 3.4.1):
//
//	Set Header: Set ID (2) + Length (2)
//	Template Record: Template ID (2) + Field Count (2)
//	Field Specifiers: (IE ID with E bit (2) + Field Length (2)) x N
func BuildCounterTemplate() []byte {
	fieldCount := len(counterTemplateFields)
	// Set header (4) + template record header (4) + field specifiers (4 each)
	setLength := 4 + 4 + fieldCount*4
	buf := make([]byte, setLength)
	off := 0

	// Set Header
	binary.BigEndian.PutUint16(buf[off:], TemplateSetID)
	off += 2
	binary.BigEndian.PutUint16(buf[off:], uint16(setLength))
	off += 2

	// Template Record Header
	binary.BigEndian.PutUint16(buf[off:], CounterTemplateID)
	off += 2
	binary.BigEndian.PutUint16(buf[off:], uint16(fieldCount))
	off += 2

	// Field Specifiers: E=0 (bit 15 clear) for all IANA IEs.
	// RFC 7011 Section 3.2: when E=0, Enterprise Number MUST NOT be present.
	for _, f := range counterTemplateFields {
		binary.BigEndian.PutUint16(buf[off:], f[0]) // E=0, IE ID in bits 14-0
		off += 2
		binary.BigEndian.PutUint16(buf[off:], f[1])
		off += 2
	}

	return buf
}
