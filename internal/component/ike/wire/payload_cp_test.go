package wire

import (
	"bytes"
	"testing"
)

func TestPayloadCPRoundtrip(t *testing.T) {
	p := PayloadCP{
		CFGType: CFGTypeRequest,
		Attrs: []ConfigAttr{
			{Type: CPAttrInternalIP4Address, Value: []byte{10, 0, 0, 1}},
			{Type: CPAttrInternalIP4DNS, Value: []byte{8, 8, 8, 8}},
		},
	}

	buf := make([]byte, 256)
	n := p.WriteTo(buf, 0)

	var got PayloadCP
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if got.CFGType != CFGTypeRequest {
		t.Errorf("CFGType = %d, want %d", got.CFGType, CFGTypeRequest)
	}
	if len(got.Attrs) != 2 {
		t.Fatalf("got %d attrs, want 2", len(got.Attrs))
	}
	if got.Attrs[0].Type != CPAttrInternalIP4Address {
		t.Errorf("attr[0].Type = %d, want %d", got.Attrs[0].Type, CPAttrInternalIP4Address)
	}
	if !bytes.Equal(got.Attrs[0].Value, []byte{10, 0, 0, 1}) {
		t.Errorf("attr[0].Value = %v, want 10.0.0.1", got.Attrs[0].Value)
	}
	if got.Attrs[1].Type != CPAttrInternalIP4DNS {
		t.Errorf("attr[1].Type = %d, want %d", got.Attrs[1].Type, CPAttrInternalIP4DNS)
	}
	if !bytes.Equal(got.Attrs[1].Value, []byte{8, 8, 8, 8}) {
		t.Errorf("attr[1].Value = %v, want 8.8.8.8", got.Attrs[1].Value)
	}
}
