package bmp

import (
	"bytes"
	"testing"
)

func TestBMPTLVDecode(t *testing.T) {
	// VALIDATES: AC-7 -- TLV type + length + value extraction
	buf := []byte{
		0, 2, // type = 2 (sysName)
		0, 5, // length = 5
		'h', 'e', 'l', 'l', 'o', // value
	}
	tlv, n, err := DecodeTLV(buf, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 9 {
		t.Errorf("consumed %d, want 9", n)
	}
	if tlv.Type != InitTLVSysName {
		t.Errorf("type = %d, want %d", tlv.Type, InitTLVSysName)
	}
	if tlv.Length != 5 {
		t.Errorf("length = %d, want 5", tlv.Length)
	}
	if string(tlv.Value) != "hello" {
		t.Errorf("value = %q, want %q", string(tlv.Value), "hello")
	}
}

func TestBMPTLVEncode(t *testing.T) {
	// VALIDATES: AC-7 -- TLV serialization round-trip
	original := TLV{Type: InitTLVSysDescr, Length: 3, Value: []byte("abc")}
	buf := make([]byte, TLVHeaderSize+3)
	n := WriteTLV(buf, 0, original)
	if n != 7 {
		t.Errorf("wrote %d, want 7", n)
	}

	decoded, _, err := DecodeTLV(buf, 0)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded.Type != original.Type {
		t.Errorf("type = %d, want %d", decoded.Type, original.Type)
	}
	if !bytes.Equal(decoded.Value, original.Value) {
		t.Errorf("value = %q, want %q", decoded.Value, original.Value)
	}
}

func TestBMPTLVDecodeTooShort(t *testing.T) {
	// VALIDATES: AC-6 -- short TLV returns error
	tests := []struct {
		name string
		buf  []byte
	}{
		{"header too short", []byte{0, 1}},
		{"value truncated", []byte{0, 0, 0, 5, 'a', 'b'}},
		{"empty", []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := DecodeTLV(tt.buf, 0)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestBMPTLVsMultiple(t *testing.T) {
	// VALIDATES: AC-7 -- multiple TLVs decoded in sequence
	buf := make([]byte, 128)
	off := 0
	off += WriteTLV(buf, off, MakeStringTLV(InitTLVSysName, "router1"))
	off += WriteTLV(buf, off, MakeStringTLV(InitTLVSysDescr, "ze v1"))

	tlvs, err := DecodeTLVs(buf, 0, off)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tlvs) != 2 {
		t.Fatalf("got %d TLVs, want 2", len(tlvs))
	}
	if string(tlvs[0].Value) != "router1" {
		t.Errorf("tlv[0] value = %q, want %q", string(tlvs[0].Value), "router1")
	}
	if string(tlvs[1].Value) != "ze v1" {
		t.Errorf("tlv[1] value = %q, want %q", string(tlvs[1].Value), "ze v1")
	}
}

func TestMakeStringTLV(t *testing.T) {
	tlv := MakeStringTLV(InitTLVString, "test message")
	if tlv.Type != InitTLVString {
		t.Errorf("type = %d, want %d", tlv.Type, InitTLVString)
	}
	if int(tlv.Length) != len("test message") {
		t.Errorf("length = %d, want %d", tlv.Length, len("test message"))
	}
	if string(tlv.Value) != "test message" {
		t.Errorf("value = %q, want %q", string(tlv.Value), "test message")
	}
}
