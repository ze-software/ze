package netflow9

import (
	"encoding/binary"
	"testing"
)

// RFC requirement: RFC3954-x-9 positive -- the counter Template FlowSet carries Template ID 256, the compile-time constant CounterTemplateID (template.go:8) that is never reassigned for the life of the process.
func TestNetflow9Template(t *testing.T) {
	tmpl := BuildCounterTemplate()

	// FlowSet ID = 0 (template)
	fsID := binary.BigEndian.Uint16(tmpl[0:])
	if fsID != FlowSetIDTemplate {
		t.Errorf("FlowSet ID: got %d, want %d", fsID, FlowSetIDTemplate)
	}

	// FlowSet length must equal len(tmpl)
	fsLen := binary.BigEndian.Uint16(tmpl[2:])
	if int(fsLen) != len(tmpl) {
		t.Errorf("FlowSet length: got %d, want %d", fsLen, len(tmpl))
	}

	// Template ID = 256
	tplID := binary.BigEndian.Uint16(tmpl[4:])
	if tplID != CounterTemplateID {
		t.Errorf("template ID: got %d, want %d", tplID, CounterTemplateID)
	}

	// Field count
	fc := binary.BigEndian.Uint16(tmpl[6:])
	if int(fc) != counterFieldCount() {
		t.Errorf("field count: got %d, want %d", fc, counterFieldCount())
	}

	// Verify each field specifier
	off := 8
	for i, expected := range counterFields {
		fType := binary.BigEndian.Uint16(tmpl[off:])
		fLen := binary.BigEndian.Uint16(tmpl[off+2:])
		if fType != expected[0] {
			t.Errorf("field %d type: got %d, want %d", i, fType, expected[0])
		}
		if fLen != expected[1] {
			t.Errorf("field %d length: got %d, want %d", i, fLen, expected[1])
		}
		off += 4
	}
}

func TestNetflow9TemplateRefresh(t *testing.T) {
	tmpl1 := BuildCounterTemplate()
	tmpl2 := BuildCounterTemplate()

	if len(tmpl1) != len(tmpl2) {
		t.Fatalf("template lengths differ: %d vs %d", len(tmpl1), len(tmpl2))
	}
	for i := range tmpl1 {
		if tmpl1[i] != tmpl2[i] {
			t.Fatalf("template byte %d differs: %d vs %d", i, tmpl1[i], tmpl2[i])
		}
	}

	// Template bytes can be copy()'d into a datagram buffer
	buf := make([]byte, 1400)
	n := copy(buf[20:], tmpl1)
	if n != len(tmpl1) {
		t.Errorf("copy returned %d, want %d", n, len(tmpl1))
	}
}

func TestNetflow9TemplateAlignment(t *testing.T) {
	tmpl := BuildCounterTemplate()
	if len(tmpl)%4 != 0 {
		t.Errorf("template length %d not 4-byte aligned", len(tmpl))
	}
}

func TestCounterRecordSize(t *testing.T) {
	// INPUT_SNMP(4) + IN_BYTES(8) + IN_PKTS(4) + OUT_BYTES(8) + OUT_PKTS(4) + OUTPUT_SNMP(4) = 32
	expected := 4 + 8 + 4 + 8 + 4 + 4
	got := CounterRecordSize()
	if got != expected {
		t.Errorf("CounterRecordSize: got %d, want %d", got, expected)
	}
}
