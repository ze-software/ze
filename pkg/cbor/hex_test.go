package cbor

import (
	"bytes"
	"testing"
)

// TestHexEncode verifies hex encoding with WriteTo pattern.
//
// VALIDATES: Correct lowercase hex output.
// PREVENTS: Wrong encoding table or byte order.
func TestHexEncode(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", []byte{}, ""},
		{"one", []byte{0xAB}, "ab"},
		{"multi", []byte{0xDE, 0xAD, 0xBE, 0xEF}, "deadbeef"},
		{"zeros", []byte{0x00, 0x00}, "0000"},
		{"ff", []byte{0xFF, 0xFF}, "ffff"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n := HexEncode(buf, 0, tt.data)
			got := string(buf[:n])
			if got != tt.want {
				t.Errorf("HexEncode(%v) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

// TestHexEncodeUpper verifies uppercase hex encoding.
//
// VALIDATES: Correct uppercase hex output.
// PREVENTS: Mixed case issues.
func TestHexEncodeUpper(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", []byte{}, ""},
		{"one", []byte{0xAB}, "AB"},
		{"multi", []byte{0xDE, 0xAD, 0xBE, 0xEF}, "DEADBEEF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n := HexEncodeUpper(buf, 0, tt.data)
			got := string(buf[:n])
			if got != tt.want {
				t.Errorf("HexEncodeUpper(%v) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

// TestHexDecode verifies hex decoding.
//
// VALIDATES: Correct binary output from hex string.
// PREVENTS: Wrong nibble parsing or byte assembly.
func TestHexDecode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{"empty", "", []byte{}, false},
		{"lower", "deadbeef", []byte{0xDE, 0xAD, 0xBE, 0xEF}, false},
		{"upper", "DEADBEEF", []byte{0xDE, 0xAD, 0xBE, 0xEF}, false},
		{"mixed", "DeAdBeEf", []byte{0xDE, 0xAD, 0xBE, 0xEF}, false},
		{"zeros", "0000", []byte{0x00, 0x00}, false},
		{"odd_length", "abc", nil, true},
		{"invalid_char", "zzzz", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n, err := HexDecode(buf, 0, tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("HexDecode error: %v", err)
			}
			got := buf[:n]
			if !bytes.Equal(got, tt.want) {
				t.Errorf("HexDecode(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestHexDecodedLen verifies decoded length calculation.
//
// VALIDATES: Correct pre-calculation for buffer allocation.
// PREVENTS: Buffer overflow or underflow.
func TestHexDecodedLen(t *testing.T) {
	tests := []struct {
		hexLen  int
		wantLen int
	}{
		{0, 0},
		{2, 1},
		{8, 4},
		{16, 8},
	}

	for _, tt := range tests {
		got := HexDecodedLen(tt.hexLen)
		if got != tt.wantLen {
			t.Errorf("HexDecodedLen(%d) = %d, want %d", tt.hexLen, got, tt.wantLen)
		}
	}
}

// TestHexEncodedLen verifies encoded length calculation.
//
// VALIDATES: Correct pre-calculation for buffer allocation.
// PREVENTS: Buffer overflow.
func TestHexEncodedLen(t *testing.T) {
	tests := []struct {
		dataLen int
		wantLen int
	}{
		{0, 0},
		{1, 2},
		{4, 8},
		{8, 16},
	}

	for _, tt := range tests {
		got := HexEncodedLen(tt.dataLen)
		if got != tt.wantLen {
			t.Errorf("HexEncodedLen(%d) = %d, want %d", tt.dataLen, got, tt.wantLen)
		}
	}
}

// TestHexWriteToOffset verifies hex encoding respects offset.
//
// VALIDATES: Correct placement at arbitrary buffer position.
// PREVENTS: Ignoring offset parameter.
func TestHexWriteToOffset(t *testing.T) {
	buf := make([]byte, 64)
	buf[0] = 'X'
	n := HexEncode(buf, 10, []byte{0xAB, 0xCD})

	if buf[0] != 'X' {
		t.Error("HexEncode overwrote data before offset")
	}
	if n != 4 {
		t.Errorf("HexEncode length = %d, want 4", n)
	}
	if string(buf[10:14]) != "abcd" {
		t.Errorf("HexEncode at offset = %q, want abcd", string(buf[10:14]))
	}
}

// TestHexRoundTrip verifies encode-decode cycle.
//
// VALIDATES: Data survives full round-trip.
// PREVENTS: Information loss.
func TestHexRoundTrip(t *testing.T) {
	original := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF}

	hexBuf := make([]byte, 64)
	hexLen := HexEncode(hexBuf, 0, original)

	binBuf := make([]byte, 64)
	binLen, err := HexDecode(binBuf, 0, string(hexBuf[:hexLen]))
	if err != nil {
		t.Fatalf("HexDecode error: %v", err)
	}

	if !bytes.Equal(binBuf[:binLen], original) {
		t.Errorf("round-trip = %v, want %v", binBuf[:binLen], original)
	}
}

// TestHexBlobLenMatchesWriteTo verifies Len() == WriteTo() bytes.
//
// VALIDATES: Len() accurately predicts WriteTo output size.
//
// PREVENTS: Buffer overflow from undersized allocation.
func TestHexBlobLenMatchesWriteTo(t *testing.T) {
	tests := []HexBlob{
		{},
		{0x00},
		{0xDE, 0xAD, 0xBE, 0xEF},
		make([]byte, 100),
		make([]byte, 1000),
	}

	for i, blob := range tests {
		expectedLen := blob.Len()

		buf := make([]byte, 2000)
		n := blob.WriteTo(buf, 0)

		if expectedLen != n {
			t.Errorf("test %d: Len()=%d but WriteTo()=%d", i, expectedLen, n)
		}
		if expectedLen != len(blob) {
			t.Errorf("test %d: Len()=%d but len(blob)=%d", i, expectedLen, len(blob))
		}
	}
}
