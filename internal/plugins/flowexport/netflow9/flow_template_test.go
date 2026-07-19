package netflow9

import (
	"encoding/binary"
	"testing"
)

// RFC requirement: RFC3954-x-9 positive -- the per-flow Template FlowSet carries Template ID 257, the compile-time constant FlowTemplateID (flow_template.go:10) that is never reassigned for the life of the process.
func TestNetflow9FlowTemplate(t *testing.T) {
	tmpl := BuildFlowTemplate()

	// FlowSet ID = 0 (template)
	if got := binary.BigEndian.Uint16(tmpl[0:]); got != 0 {
		t.Fatalf("FlowSet ID = %d, want 0", got)
	}

	// Template ID = 257
	if got := binary.BigEndian.Uint16(tmpl[4:]); got != FlowTemplateID {
		t.Fatalf("Template ID = %d, want %d", got, FlowTemplateID)
	}

	// Field count = 11
	if got := binary.BigEndian.Uint16(tmpl[6:]); got != 11 {
		t.Fatalf("Field Count = %d, want 11", got)
	}

	// Verify first field: IPV4_SRC_ADDR (type=8, length=4)
	if got := binary.BigEndian.Uint16(tmpl[8:]); got != 8 {
		t.Fatalf("field[0] type = %d, want 8 (IPV4_SRC_ADDR)", got)
	}
	if got := binary.BigEndian.Uint16(tmpl[10:]); got != 4 {
		t.Fatalf("field[0] length = %d, want 4", got)
	}

	// Verify PROTOCOL field (5th field, offset 8 + 4*4 = 24): type=4, length=1
	if got := binary.BigEndian.Uint16(tmpl[24:]); got != 4 {
		t.Fatalf("field[4] type = %d, want 4 (PROTOCOL)", got)
	}
	if got := binary.BigEndian.Uint16(tmpl[26:]); got != 1 {
		t.Fatalf("field[4] length = %d, want 1", got)
	}
}

func TestNetflow9FlowRecordSize(t *testing.T) {
	// 4+4+2+2+1+8+4+4+4+4+4 = 41
	if got := FlowRecordSize(); got != 41 {
		t.Fatalf("FlowRecordSize = %d, want 41", got)
	}
}

func TestNetflow9FlowFieldCount(t *testing.T) {
	if got := FlowFieldCount(); got != 11 {
		t.Fatalf("FlowFieldCount = %d, want 11", got)
	}
}
