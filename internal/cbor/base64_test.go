package cbor

import (
	"bytes"
	"testing"
)

// TestBase64Encode verifies standard base64 encoding with WriteTo pattern.
//
// VALIDATES: Correct RFC 4648 standard base64 output with padding.
// PREVENTS: Wrong alphabet or padding issues.
func TestBase64Encode(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", []byte{}, ""},
		{"one", []byte{0x00}, "AA=="},
		{"two", []byte{0x00, 0x00}, "AAA="},
		{"three", []byte{0x00, 0x00, 0x00}, "AAAA"},
		{"hello", []byte("hello"), "aGVsbG8="},
		{"binary", []byte{0xDE, 0xAD, 0xBE, 0xEF}, "3q2+7w=="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n := Base64Encode(buf, 0, tt.data)
			got := string(buf[:n])
			if got != tt.want {
				t.Errorf("Base64Encode(%v) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

// TestBase64EncodeURL verifies URL-safe base64 encoding.
//
// VALIDATES: Correct RFC 4648 URL-safe alphabet (- and _ instead of + and /).
// PREVENTS: Wrong alphabet for URL contexts.
func TestBase64EncodeURL(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", []byte{}, ""},
		{"needs_minus", []byte{0xFB}, "-w=="}, // Standard would be "+w=="
		{"needs_under", []byte{0xFF}, "_w=="}, // Standard would be "/w=="
		{"complex", []byte{0xFB, 0xFF, 0xFE}, "-__-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n := Base64EncodeURL(buf, 0, tt.data)
			got := string(buf[:n])
			if got != tt.want {
				t.Errorf("Base64EncodeURL(%v) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

// TestBase64EncodeNoPadding verifies base64 without padding.
//
// VALIDATES: Correct output without trailing '=' characters.
// PREVENTS: Unwanted padding in contexts that don't need it.
func TestBase64EncodeNoPadding(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", []byte{}, ""},
		{"one", []byte{0x00}, "AA"},
		{"two", []byte{0x00, 0x00}, "AAA"},
		{"three", []byte{0x00, 0x00, 0x00}, "AAAA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n := Base64EncodeNoPadding(buf, 0, tt.data)
			got := string(buf[:n])
			if got != tt.want {
				t.Errorf("Base64EncodeNoPadding(%v) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

// TestBase64Decode verifies standard base64 decoding.
//
// VALIDATES: Correct binary output from base64 string.
// PREVENTS: Wrong alphabet handling or padding issues.
func TestBase64Decode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{"empty", "", []byte{}, false},
		{"padded_1", "AA==", []byte{0x00}, false},
		{"padded_2", "AAA=", []byte{0x00, 0x00}, false},
		{"no_pad", "AAAA", []byte{0x00, 0x00, 0x00}, false},
		{"hello", "aGVsbG8=", []byte("hello"), false},
		{"binary", "3q2+7w==", []byte{0xDE, 0xAD, 0xBE, 0xEF}, false},
		{"invalid_char", "!!!!", nil, true},
		{"wrong_padding", "A===", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n, err := Base64Decode(buf, 0, tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Base64Decode error: %v", err)
			}
			got := buf[:n]
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Base64Decode(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestBase64DecodeURL verifies URL-safe base64 decoding.
//
// VALIDATES: Correct handling of URL-safe alphabet.
// PREVENTS: Failing on - and _ characters.
func TestBase64DecodeURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{"empty", "", []byte{}, false},
		{"url_chars", "-w==", []byte{0xFB}, false},
		{"url_chars2", "_w==", []byte{0xFF}, false},
		{"complex", "-__-", []byte{0xFB, 0xFF, 0xFE}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n, err := Base64DecodeURL(buf, 0, tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Base64DecodeURL error: %v", err)
			}
			got := buf[:n]
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Base64DecodeURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestBase64DecodeNoPadding verifies decoding without padding.
//
// VALIDATES: Correct handling of unpadded input.
// PREVENTS: Failing when padding is absent.
func TestBase64DecodeNoPadding(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []byte
	}{
		{"empty", "", []byte{}},
		{"one", "AA", []byte{0x00}},
		{"two", "AAA", []byte{0x00, 0x00}},
		{"three", "AAAA", []byte{0x00, 0x00, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n, err := Base64DecodeNoPadding(buf, 0, tt.input)
			if err != nil {
				t.Fatalf("Base64DecodeNoPadding error: %v", err)
			}
			got := buf[:n]
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Base64DecodeNoPadding(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestBase64EncodedLen verifies encoded length calculation.
//
// VALIDATES: Correct pre-calculation for buffer allocation.
// PREVENTS: Buffer overflow.
func TestBase64EncodedLen(t *testing.T) {
	tests := []struct {
		dataLen int
		wantLen int
	}{
		{0, 0},
		{1, 4},  // 1 byte -> 4 chars (2 data + 2 padding)
		{2, 4},  // 2 bytes -> 4 chars (3 data + 1 padding)
		{3, 4},  // 3 bytes -> 4 chars (no padding)
		{4, 8},  // 4 bytes -> 8 chars
		{6, 8},  // 6 bytes -> 8 chars
		{7, 12}, // 7 bytes -> 12 chars
	}

	for _, tt := range tests {
		got := Base64EncodedLen(tt.dataLen)
		if got != tt.wantLen {
			t.Errorf("Base64EncodedLen(%d) = %d, want %d", tt.dataLen, got, tt.wantLen)
		}
	}
}

// TestBase64DecodedLen verifies decoded length calculation.
//
// VALIDATES: Correct pre-calculation for buffer allocation.
// PREVENTS: Buffer overflow or underflow.
func TestBase64DecodedLen(t *testing.T) {
	tests := []struct {
		input   string
		wantLen int
	}{
		{"", 0},
		{"AA==", 1},
		{"AAA=", 2},
		{"AAAA", 3},
		{"AAAAAAAA", 6},
	}

	for _, tt := range tests {
		got := Base64DecodedLen(tt.input)
		if got != tt.wantLen {
			t.Errorf("Base64DecodedLen(%q) = %d, want %d", tt.input, got, tt.wantLen)
		}
	}
}

// TestBase64WriteToOffset verifies base64 encoding respects offset.
//
// VALIDATES: Correct placement at arbitrary buffer position.
// PREVENTS: Ignoring offset parameter.
func TestBase64WriteToOffset(t *testing.T) {
	buf := make([]byte, 64)
	buf[0] = 'X'
	n := Base64Encode(buf, 10, []byte{0x00, 0x00, 0x00})

	if buf[0] != 'X' {
		t.Error("Base64Encode overwrote data before offset")
	}
	if n != 4 {
		t.Errorf("Base64Encode length = %d, want 4", n)
	}
	if string(buf[10:14]) != "AAAA" {
		t.Errorf("Base64Encode at offset = %q, want AAAA", string(buf[10:14]))
	}
}

// TestBase64RoundTrip verifies encode-decode cycle.
//
// VALIDATES: Data survives full round-trip.
// PREVENTS: Information loss.
func TestBase64RoundTrip(t *testing.T) {
	original := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF}

	b64Buf := make([]byte, 64)
	b64Len := Base64Encode(b64Buf, 0, original)

	binBuf := make([]byte, 64)
	binLen, err := Base64Decode(binBuf, 0, string(b64Buf[:b64Len]))
	if err != nil {
		t.Fatalf("Base64Decode error: %v", err)
	}

	if !bytes.Equal(binBuf[:binLen], original) {
		t.Errorf("round-trip = %v, want %v", binBuf[:binLen], original)
	}
}

// TestBase64URLRoundTrip verifies URL-safe encode-decode cycle.
//
// VALIDATES: URL-safe data survives full round-trip.
// PREVENTS: Alphabet confusion.
func TestBase64URLRoundTrip(t *testing.T) {
	// Contains bytes that produce + and / in standard base64
	original := []byte{0xFB, 0xFF, 0xFE, 0x00, 0x01}

	b64Buf := make([]byte, 64)
	b64Len := Base64EncodeURL(b64Buf, 0, original)

	binBuf := make([]byte, 64)
	binLen, err := Base64DecodeURL(binBuf, 0, string(b64Buf[:b64Len]))
	if err != nil {
		t.Fatalf("Base64DecodeURL error: %v", err)
	}

	if !bytes.Equal(binBuf[:binLen], original) {
		t.Errorf("round-trip = %v, want %v", binBuf[:binLen], original)
	}
}

// TestBase64BlobLenMatchesWriteTo verifies Len() == WriteTo() bytes.
//
// VALIDATES: Len() accurately predicts WriteTo output size.
//
// PREVENTS: Buffer overflow from undersized allocation.
func TestBase64BlobLenMatchesWriteTo(t *testing.T) {
	tests := []Base64Blob{
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
