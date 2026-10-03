// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- IPFIX counter export
// Related: rfc7011_export_test.go -- the per-flow record's integral encoding
//
// VALIDATES: the counter Data Record integrals are big-endian, the three
// Template IDs one Transport Session carries are distinct and in range, and
// every emitted Information Element identifier lies in 1-32767.
// PREVENTS: a little-endian regression in writeCounterRecord, two templates
// sharing one ID, and a template that references IE identifier 0.

package ipfix

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// counterIntegrals is one interface whose every integral field holds distinct
// octets, so the byte order of each field is visible in the encoded record.
func counterIntegrals() []flowexport.InterfaceCounters {
	return []flowexport.InterfaceCounters{{
		IfIndex:     0x21222324,
		IfInOctets:  0x0102030405060700,
		IfOutOctets: 0x08,
		InPackets:   0x1112131415161700,
		OutPackets:  0x18,
	}}
}

// RFC requirement: RFC7011-6.1.1-1 positive -- the counter Data Record writes its
// unsigned32 ingressInterface, unsigned64 octetTotalCount and packetTotalCount,
// unsigned32 egressInterface and the two unsigned32 dateTimeSeconds most
// significant octet first: the record is the big-endian concatenation of them.
func TestRFC7011CounterRecordIntegralsNetworkByteOrder(t *testing.T) {
	var buf [128]byte
	n, count := WriteDataSet(buf[:], 0, CounterTemplateID, counterIntegrals(), 0x31323334, 0x41424344)
	if count != 1 {
		t.Fatalf("records = %d, want 1", count)
	}
	want := []byte{
		0x21, 0x22, 0x23, 0x24,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x21, 0x22, 0x23, 0x24,
		0x31, 0x32, 0x33, 0x34,
		0x41, 0x42, 0x43, 0x44,
	}
	if got := buf[4:n]; !bytes.Equal(got, want) {
		t.Fatalf("counter record = %x, want %x", got, want)
	}
}

// RFC requirement: RFC7011-6.1.1-1 negative -- no integral field of the counter
// Data Record appears least significant octet first: the byte-reversed interface
// index, octet total, packet total and start time are all absent.
func TestRFC7011CounterRecordIntegralsNeverLittleEndian(t *testing.T) {
	var buf [128]byte
	n, _ := WriteDataSet(buf[:], 0, CounterTemplateID, counterIntegrals(), 0x31323334, 0x41424344)
	rec := buf[4:n]
	for _, reversed := range [][]byte{
		{0x24, 0x23, 0x22, 0x21},
		{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01},
		{0x18, 0x17, 0x16, 0x15, 0x14, 0x13, 0x12, 0x11},
		{0x34, 0x33, 0x32, 0x31},
	} {
		if bytes.Contains(rec, reversed) {
			t.Fatalf("little-endian field %x found in counter record %x", reversed, rec)
		}
	}
}

// emittedTemplates returns every Template Set the exporter can put on one
// Transport Session, by name.
func emittedTemplates() map[string][]byte {
	return map[string][]byte{
		"counter": BuildCounterTemplate(),
		"flow4":   BuildFlowTemplate(),
		"flow6":   BuildFlowTemplate6(),
	}
}

// RFC requirement: RFC7011-3.4.1-1 positive -- the counter, IPv4 flow and IPv6
// flow Template Records that share one Transport Session and Observation Domain
// carry three distinct Template IDs, each in the range 256 to 65535.
func TestRFC7011TemplateIDsUniqueInSession(t *testing.T) {
	owner := make(map[uint16]string)
	for name, tmpl := range emittedTemplates() {
		id := binary.BigEndian.Uint16(tmpl[4:])
		if id < 256 {
			t.Errorf("%s: Template ID %d is below 256", name, id)
		}
		if prev, dup := owner[id]; dup {
			t.Errorf("%s and %s share Template ID %d", prev, name, id)
		}
		owner[id] = name
	}
	if len(owner) != 3 {
		t.Fatalf("distinct Template IDs = %d, want 3", len(owner))
	}
}

// RFC requirement: RFC7012-4-1 positive -- every field specifier of every emitted
// template (counter, IPv4 flow, IPv6 flow) carries an IANA Information Element
// identifier in 1-32767: never the reserved 0, and the E bit is clear.
func TestRFC7012EmittedIEIdentifiersInRange(t *testing.T) {
	for name, tmpl := range emittedTemplates() {
		fields := int(binary.BigEndian.Uint16(tmpl[6:]))
		if fields == 0 {
			t.Fatalf("%s: template has no field specifiers", name)
		}
		for i := range fields {
			id := binary.BigEndian.Uint16(tmpl[8+i*4:])
			if id == 0 {
				t.Errorf("%s field[%d]: IE identifier 0 is reserved", name, i)
			}
			if id > 32767 {
				t.Errorf("%s field[%d]: IE word %#x is outside 1-32767", name, i, id)
			}
		}
	}
}
