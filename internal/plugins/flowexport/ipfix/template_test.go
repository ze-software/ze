package ipfix

import (
	"encoding/binary"
	"testing"
)

func TestIPFIXTemplateSet(t *testing.T) {
	// RFC requirement: RFC7012-x-1 positive -- every emitted field specifier has the E bit clear and is exactly 4 octets, carrying no Enterprise Number

	tmpl := BuildCounterTemplate()

	// Set Header: Set ID (2) + Length (2) = 4
	// Template Record: Template ID (2) + Field Count (2) = 4
	// Field Specifiers: 6 fields x 4 bytes = 24
	// Total = 32
	expectedLen := 4 + 4 + 6*4
	if len(tmpl) != expectedLen {
		t.Fatalf("template length = %d, want %d", len(tmpl), expectedLen)
	}

	off := 0

	// RFC 7011 Section 3.3.2: Set ID 2 = Template Set.
	setID := binary.BigEndian.Uint16(tmpl[off:])
	if setID != TemplateSetID {
		t.Errorf("Set ID = %d, want %d (Template Set)", setID, TemplateSetID)
	}
	off += 2

	setLength := binary.BigEndian.Uint16(tmpl[off:])
	if int(setLength) != expectedLen {
		t.Errorf("Set Length = %d, want %d", setLength, expectedLen)
	}
	off += 2

	// RFC 7011 Section 3.4.1: Template ID MUST be >= 256.
	templateID := binary.BigEndian.Uint16(tmpl[off:])
	if templateID != CounterTemplateID {
		t.Errorf("Template ID = %d, want %d", templateID, CounterTemplateID)
	}
	if templateID < 256 {
		t.Errorf("Template ID = %d, MUST be >= 256 per RFC 7011", templateID)
	}
	off += 2

	fieldCount := binary.BigEndian.Uint16(tmpl[off:])
	if fieldCount != 6 {
		t.Errorf("Field Count = %d, want 6", fieldCount)
	}
	off += 2

	// Verify each field specifier.
	expectedFields := [][2]uint16{
		{IEIngressInterface, 4},
		{IEOctetTotalCount, 8},
		{IEPacketTotalCount, 8},
		{IEEgressInterface, 4},
		{IEFlowStartSeconds, 4},
		{IEFlowEndSeconds, 4},
	}

	for i, expect := range expectedFields {
		ieID := binary.BigEndian.Uint16(tmpl[off:])
		off += 2
		fieldLen := binary.BigEndian.Uint16(tmpl[off:])
		off += 2

		// E bit (bit 15) must be 0 for IANA IEs.
		if ieID&0x8000 != 0 {
			t.Errorf("field %d: E bit set, want clear for IANA IE", i)
		}

		if ieID != expect[0] {
			t.Errorf("field %d: IE ID = %d, want %d", i, ieID, expect[0])
		}
		if fieldLen != expect[1] {
			t.Errorf("field %d: field length = %d, want %d", i, fieldLen, expect[1])
		}
	}
}

func TestIPFIXCounterRecordSize(t *testing.T) {
	// 4 + 8 + 8 + 4 + 4 + 4 = 32
	if CounterRecordSize() != 32 {
		t.Errorf("CounterRecordSize() = %d, want 32", CounterRecordSize())
	}
}
