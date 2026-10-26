package wire

import (
	"bytes"
	"errors"
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

// TestCPReadFromBounds proves the Configuration payload decoder bounds what a peer controls:
// 64 attributes decode, a 65th is refused as malformed, and a trailing remnant of one to three
// octets (too short for an attribute header) is refused as truncated rather than dropped.
// Method: build bodies of exact shape and decode each.
func TestCPReadFromBounds(t *testing.T) {
	body := func(attrs int, remnant int) []byte {
		b := []byte{CFGTypeRequest, 0, 0, 0}
		for range attrs {
			b = append(b, 0, byte(CPAttrInternalIP4DNS), 0, 0)
		}
		for range remnant {
			b = append(b, 0)
		}
		return b
	}

	var p PayloadCP
	if err := p.ReadFrom(body(CPAttrCountMax, 0)); err != nil {
		t.Fatalf("64 attributes: %v, want accepted", err)
	}
	if len(p.Attrs) != CPAttrCountMax {
		t.Fatalf("64 attributes: decoded %d", len(p.Attrs))
	}
	if err := p.ReadFrom(body(CPAttrCountMax+1, 0)); !errors.Is(err, ErrCPTooManyAttrs) {
		t.Fatalf("65 attributes: %v, want ErrCPTooManyAttrs", err)
	}
	for remnant := 1; remnant <= 3; remnant++ {
		if err := p.ReadFrom(body(1, remnant)); !errors.Is(err, ErrTruncated) {
			t.Fatalf("%d-octet remnant: %v, want ErrTruncated", remnant, err)
		}
	}
	if err := p.ReadFrom(body(0, 0)); err != nil {
		t.Fatalf("zero attributes: %v, want accepted", err)
	}
}
