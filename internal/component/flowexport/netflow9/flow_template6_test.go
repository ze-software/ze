package netflow9

import (
	"encoding/binary"
	"testing"
)

func TestNetflow9FlowTemplate6(t *testing.T) {
	tmpl := BuildFlowTemplate6()

	// FlowSet ID = 0 (template)
	if got := binary.BigEndian.Uint16(tmpl[0:]); got != 0 {
		t.Fatalf("FlowSet ID = %d, want 0", got)
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

	// field[0]: IPV6_SRC_ADDR (type 27, length 16)
	if got := binary.BigEndian.Uint16(tmpl[8:]); got != 27 {
		t.Fatalf("field[0] type = %d, want 27 (IPV6_SRC_ADDR)", got)
	}
	if got := binary.BigEndian.Uint16(tmpl[10:]); got != 16 {
		t.Fatalf("field[0] length = %d, want 16", got)
	}

	// field[1]: IPV6_DST_ADDR (type 28, length 16)
	if got := binary.BigEndian.Uint16(tmpl[12:]); got != 28 {
		t.Fatalf("field[1] type = %d, want 28 (IPV6_DST_ADDR)", got)
	}
	if got := binary.BigEndian.Uint16(tmpl[14:]); got != 16 {
		t.Fatalf("field[1] length = %d, want 16", got)
	}

	// field[4]: PROTOCOL (type 4, length 1), offset 8 + 4*4 = 24
	if got := binary.BigEndian.Uint16(tmpl[24:]); got != 4 {
		t.Fatalf("field[4] type = %d, want 4 (PROTOCOL)", got)
	}
	if got := binary.BigEndian.Uint16(tmpl[26:]); got != 1 {
		t.Fatalf("field[4] length = %d, want 1", got)
	}
}

func TestNetflow9FlowRecordSize6(t *testing.T) {
	// 16+16+2+2+1+8+4+4+4+4+4 = 65
	if got := FlowRecordSize6(); got != 65 {
		t.Fatalf("FlowRecordSize6 = %d, want 65", got)
	}
}

func TestNetflow9FlowFieldCount6(t *testing.T) {
	if got := FlowFieldCount6(); got != 11 {
		t.Fatalf("FlowFieldCount6 = %d, want 11", got)
	}
}
