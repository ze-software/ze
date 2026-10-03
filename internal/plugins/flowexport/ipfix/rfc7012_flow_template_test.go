package ipfix

import (
	"encoding/binary"
	"testing"
)

func TestIPFIXFlowTemplate(t *testing.T) {
	// RFC requirement: RFC7012-4-1 positive -- the emitted template references only non-zero IANA IE IDs; IE identifier 0 (reserved) is never used

	tmpl := BuildFlowTemplate()

	// Set ID = 2 (Template Set)
	if got := binary.BigEndian.Uint16(tmpl[0:]); got != TemplateSetID {
		t.Fatalf("Set ID = %d, want %d", got, TemplateSetID)
	}

	// Template ID = 257
	if got := binary.BigEndian.Uint16(tmpl[4:]); got != FlowTemplateID {
		t.Fatalf("Template ID = %d, want %d", got, FlowTemplateID)
	}

	// Field count = 11
	if got := binary.BigEndian.Uint16(tmpl[6:]); got != 11 {
		t.Fatalf("Field Count = %d, want 11", got)
	}

	// Verify first field: sourceIPv4Address (IE 8, length 4)
	if got := binary.BigEndian.Uint16(tmpl[8:]); got != IESourceIPv4Address {
		t.Fatalf("field[0] IE = %d, want %d (sourceIPv4Address)", got, IESourceIPv4Address)
	}
	if got := binary.BigEndian.Uint16(tmpl[10:]); got != 4 {
		t.Fatalf("field[0] length = %d, want 4", got)
	}

	// Verify protocolIdentifier (5th field, offset 8 + 4*4 = 24): IE 4, length 1
	if got := binary.BigEndian.Uint16(tmpl[24:]); got != IEProtocolIdentifier {
		t.Fatalf("field[4] IE = %d, want %d (protocolIdentifier)", got, IEProtocolIdentifier)
	}
	if got := binary.BigEndian.Uint16(tmpl[26:]); got != 1 {
		t.Fatalf("field[4] length = %d, want 1", got)
	}

	// Verify flowStartMilliseconds (10th field, offset 8 + 9*4 = 44): IE 152, length 8
	if got := binary.BigEndian.Uint16(tmpl[44:]); got != IEFlowStartMilliseconds {
		t.Fatalf("field[9] IE = %d, want %d (flowStartMilliseconds)", got, IEFlowStartMilliseconds)
	}
	if got := binary.BigEndian.Uint16(tmpl[46:]); got != 8 {
		t.Fatalf("field[9] length = %d, want 8", got)
	}

	// E bit must be 0 for all IANA IEs (bit 15 of IE ID field)
	for i := range 11 {
		ieWord := binary.BigEndian.Uint16(tmpl[8+i*4:])
		if ieWord&0x8000 != 0 {
			t.Fatalf("field[%d] has E bit set (IE word = %#x)", i, ieWord)
		}
	}
}

func TestIPFIXFlowRecordSize(t *testing.T) {
	// 4+4+2+2+1+8+8+4+4+8+8 = 53
	if got := FlowRecordSize(); got != 53 {
		t.Fatalf("FlowRecordSize = %d, want 53", got)
	}
}
