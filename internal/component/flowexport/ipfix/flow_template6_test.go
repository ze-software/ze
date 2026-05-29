package ipfix

import (
	"encoding/binary"
	"testing"
)

func TestIPFIXFlowTemplate6(t *testing.T) {
	tmpl := BuildFlowTemplate6()

	// Set ID = 2 (Template Set)
	if got := binary.BigEndian.Uint16(tmpl[0:]); got != TemplateSetID {
		t.Fatalf("Set ID = %d, want %d", got, TemplateSetID)
	}

	// Template ID = 258
	if got := binary.BigEndian.Uint16(tmpl[4:]); got != FlowTemplateID6 {
		t.Fatalf("Template ID = %d, want %d", got, FlowTemplateID6)
	}
	if FlowTemplateID6 != 258 {
		t.Fatalf("FlowTemplateID6 = %d, want 258", FlowTemplateID6)
	}

	// Field count = 11
	if got := binary.BigEndian.Uint16(tmpl[6:]); got != 11 {
		t.Fatalf("Field Count = %d, want 11", got)
	}

	// field[0]: sourceIPv6Address (IE 27, length 16)
	if got := binary.BigEndian.Uint16(tmpl[8:]); got != IESourceIPv6Address {
		t.Fatalf("field[0] IE = %d, want %d (sourceIPv6Address)", got, IESourceIPv6Address)
	}
	if got := binary.BigEndian.Uint16(tmpl[10:]); got != 16 {
		t.Fatalf("field[0] length = %d, want 16", got)
	}

	// field[1]: destinationIPv6Address (IE 28, length 16)
	if got := binary.BigEndian.Uint16(tmpl[12:]); got != IEDestinationIPv6Address {
		t.Fatalf("field[1] IE = %d, want %d (destinationIPv6Address)", got, IEDestinationIPv6Address)
	}
	if got := binary.BigEndian.Uint16(tmpl[14:]); got != 16 {
		t.Fatalf("field[1] length = %d, want 16", got)
	}

	// field[4]: protocolIdentifier (IE 4, length 1), offset 8 + 4*4 = 24
	if got := binary.BigEndian.Uint16(tmpl[24:]); got != IEProtocolIdentifier {
		t.Fatalf("field[4] IE = %d, want %d (protocolIdentifier)", got, IEProtocolIdentifier)
	}
	if got := binary.BigEndian.Uint16(tmpl[26:]); got != 1 {
		t.Fatalf("field[4] length = %d, want 1", got)
	}

	// E bit must be 0 for all IANA IEs.
	for i := range 11 {
		ieWord := binary.BigEndian.Uint16(tmpl[8+i*4:])
		if ieWord&0x8000 != 0 {
			t.Fatalf("field[%d] has E bit set (IE word = %#x)", i, ieWord)
		}
	}
}

func TestIPFIXFlowRecordSize6(t *testing.T) {
	// 16+16+2+2+1+8+8+4+4+8+8 = 77
	if got := FlowRecordSize6(); got != 77 {
		t.Fatalf("FlowRecordSize6 = %d, want 77", got)
	}
}

func TestIPFIXFlowFieldCount6(t *testing.T) {
	if got := FlowFieldCount6(); got != 11 {
		t.Fatalf("FlowFieldCount6 = %d, want 11", got)
	}
}
