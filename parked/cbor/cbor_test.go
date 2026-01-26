package cbor

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// TestEncodeUnsigned verifies unsigned integer encoding.
//
// VALIDATES: Correct CBOR major type 0 encoding for various sizes.
// PREVENTS: Wrong byte counts or endianness in encoded integers.
func TestEncodeUnsigned(t *testing.T) {
	tests := []struct {
		name string
		val  uint64
		want string // hex
	}{
		{"zero", 0, "00"},
		{"one", 1, "01"},
		{"max_tiny", 23, "17"},
		{"one_byte", 24, "1818"},
		{"one_byte_max", 255, "18ff"},
		{"two_byte", 256, "190100"},
		{"two_byte_max", 65535, "19ffff"},
		{"four_byte", 65536, "1a00010000"},
		{"four_byte_max", 4294967295, "1affffffff"},
		{"eight_byte", 4294967296, "1b0000000100000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n := EncodeUint(buf, 0, tt.val)
			got := hex.EncodeToString(buf[:n])
			if got != tt.want {
				t.Errorf("EncodeUint(%d) = %s, want %s", tt.val, got, tt.want)
			}
		})
	}
}

// TestEncodeNegative verifies negative integer encoding.
//
// VALIDATES: Correct CBOR major type 1 encoding.
// PREVENTS: Off-by-one errors in negative encoding (-1 encodes as 0).
func TestEncodeNegative(t *testing.T) {
	tests := []struct {
		name string
		val  int64
		want string
	}{
		{"minus_one", -1, "20"},
		{"minus_ten", -10, "29"},
		{"minus_24", -24, "37"},
		{"minus_25", -25, "3818"},
		{"minus_256", -256, "38ff"},
		{"minus_257", -257, "390100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n := EncodeInt(buf, 0, tt.val)
			got := hex.EncodeToString(buf[:n])
			if got != tt.want {
				t.Errorf("EncodeInt(%d) = %s, want %s", tt.val, got, tt.want)
			}
		})
	}
}

// TestEncodeBytes verifies byte string encoding.
//
// VALIDATES: Correct CBOR major type 2 with length prefix.
// PREVENTS: Wrong length encoding or missing data copy.
func TestEncodeBytes(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", []byte{}, "40"},
		{"one", []byte{0x01}, "4101"},
		{"three", []byte{0x01, 0x02, 0x03}, "43010203"},
		{"24_bytes", bytes.Repeat([]byte{0xAB}, 24), "5818" + "abababababababababababababababababababababababab"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 128)
			n := EncodeBytes(buf, 0, tt.data)
			got := hex.EncodeToString(buf[:n])
			if got != tt.want {
				t.Errorf("EncodeBytes(%v) = %s, want %s", tt.data, got, tt.want)
			}
		})
	}
}

// TestEncodeString verifies text string encoding.
//
// VALIDATES: Correct CBOR major type 3 for UTF-8 text.
// PREVENTS: Using byte type for text or wrong length.
func TestEncodeString(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"empty", "", "60"},
		{"hello", "hello", "6568656c6c6f"},
		{"unicode", "日本", "66e697a5e69cac"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n := EncodeString(buf, 0, tt.text)
			got := hex.EncodeToString(buf[:n])
			if got != tt.want {
				t.Errorf("EncodeString(%q) = %s, want %s", tt.text, got, tt.want)
			}
		})
	}
}

// TestEncodeArray verifies array header encoding.
//
// VALIDATES: Correct CBOR major type 4 with element count.
// PREVENTS: Wrong array type marker.
func TestEncodeArray(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  string
	}{
		{"empty", 0, "80"},
		{"three", 3, "83"},
		{"24", 24, "9818"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n := EncodeArrayHeader(buf, 0, tt.count)
			got := hex.EncodeToString(buf[:n])
			if got != tt.want {
				t.Errorf("EncodeArrayHeader(%d) = %s, want %s", tt.count, got, tt.want)
			}
		})
	}
}

// TestEncodeMap verifies map header encoding.
//
// VALIDATES: Correct CBOR major type 5 with pair count.
// PREVENTS: Wrong map type marker.
func TestEncodeMap(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  string
	}{
		{"empty", 0, "a0"},
		{"three_pairs", 3, "a3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n := EncodeMapHeader(buf, 0, tt.count)
			got := hex.EncodeToString(buf[:n])
			if got != tt.want {
				t.Errorf("EncodeMapHeader(%d) = %s, want %s", tt.count, got, tt.want)
			}
		})
	}
}

// TestDecodeUint verifies unsigned integer decoding.
//
// VALIDATES: Correct parsing of CBOR major type 0.
// PREVENTS: Wrong byte count reads or endianness bugs.
func TestDecodeUint(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantLen int
	}{
		{"zero", "00", 0, 1},
		{"one", "01", 1, 1},
		{"max_tiny", "17", 23, 1},
		{"one_byte", "1818", 24, 2},
		{"two_byte", "190100", 256, 3},
		{"four_byte", "1a00010000", 65536, 5},
		{"eight_byte", "1b0000000100000000", 4294967296, 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := hex.DecodeString(tt.input)
			val, n, err := DecodeUint(data, 0)
			if err != nil {
				t.Fatalf("DecodeUint error: %v", err)
			}
			if val != tt.want {
				t.Errorf("DecodeUint = %d, want %d", val, tt.want)
			}
			if n != tt.wantLen {
				t.Errorf("DecodeUint len = %d, want %d", n, tt.wantLen)
			}
		})
	}
}

// TestDecodeInt verifies signed integer decoding.
//
// VALIDATES: Correct parsing of both positive (type 0) and negative (type 1).
// PREVENTS: Wrong sign handling or off-by-one in negative values.
func TestDecodeInt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantLen int
	}{
		{"zero", "00", 0, 1},
		{"one", "01", 1, 1},
		{"minus_one", "20", -1, 1},
		{"minus_ten", "29", -10, 1},
		{"minus_25", "3818", -25, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := hex.DecodeString(tt.input)
			val, n, err := DecodeInt(data, 0)
			if err != nil {
				t.Fatalf("DecodeInt error: %v", err)
			}
			if val != tt.want {
				t.Errorf("DecodeInt = %d, want %d", val, tt.want)
			}
			if n != tt.wantLen {
				t.Errorf("DecodeInt len = %d, want %d", n, tt.wantLen)
			}
		})
	}
}

// TestDecodeBytes verifies byte string decoding.
//
// VALIDATES: Correct length parsing and data extraction.
// PREVENTS: Wrong slice bounds or length misreads.
func TestDecodeBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []byte
		wantLen int
	}{
		{"empty", "40", []byte{}, 1},
		{"one", "4101", []byte{0x01}, 2},
		{"three", "43010203", []byte{0x01, 0x02, 0x03}, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := hex.DecodeString(tt.input)
			val, n, err := DecodeBytes(data, 0)
			if err != nil {
				t.Fatalf("DecodeBytes error: %v", err)
			}
			if !bytes.Equal(val, tt.want) {
				t.Errorf("DecodeBytes = %v, want %v", val, tt.want)
			}
			if n != tt.wantLen {
				t.Errorf("DecodeBytes len = %d, want %d", n, tt.wantLen)
			}
		})
	}
}

// TestDecodeString verifies text string decoding.
//
// VALIDATES: Correct UTF-8 text extraction.
// PREVENTS: Type confusion between bytes and text.
func TestDecodeString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantLen int
	}{
		{"empty", "60", "", 1},
		{"hello", "6568656c6c6f", "hello", 6},
		{"unicode", "66e697a5e69cac", "日本", 7}, // 1 byte header + 6 byte UTF-8
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := hex.DecodeString(tt.input)
			val, n, err := DecodeString(data, 0)
			if err != nil {
				t.Fatalf("DecodeString error: %v", err)
			}
			if val != tt.want {
				t.Errorf("DecodeString = %q, want %q", val, tt.want)
			}
			if n != tt.wantLen {
				t.Errorf("DecodeString len = %d, want %d", n, tt.wantLen)
			}
		})
	}
}

// TestDecodeArrayHeader verifies array header decoding.
//
// VALIDATES: Correct element count extraction.
// PREVENTS: Type confusion with maps.
func TestDecodeArrayHeader(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantLen int
	}{
		{"empty", "80", 0, 1},
		{"three", "83", 3, 1},
		{"24", "9818", 24, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := hex.DecodeString(tt.input)
			count, n, err := DecodeArrayHeader(data, 0)
			if err != nil {
				t.Fatalf("DecodeArrayHeader error: %v", err)
			}
			if count != tt.want {
				t.Errorf("DecodeArrayHeader = %d, want %d", count, tt.want)
			}
			if n != tt.wantLen {
				t.Errorf("DecodeArrayHeader len = %d, want %d", n, tt.wantLen)
			}
		})
	}
}

// TestDecodeMapHeader verifies map header decoding.
//
// VALIDATES: Correct pair count extraction.
// PREVENTS: Type confusion with arrays.
func TestDecodeMapHeader(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantLen int
	}{
		{"empty", "a0", 0, 1},
		{"three", "a3", 3, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := hex.DecodeString(tt.input)
			count, n, err := DecodeMapHeader(data, 0)
			if err != nil {
				t.Fatalf("DecodeMapHeader error: %v", err)
			}
			if count != tt.want {
				t.Errorf("DecodeMapHeader = %d, want %d", count, tt.want)
			}
			if n != tt.wantLen {
				t.Errorf("DecodeMapHeader len = %d, want %d", n, tt.wantLen)
			}
		})
	}
}

// TestBlobWriteTo verifies Blob implements BufWriter interface.
//
// VALIDATES: Blob correctly writes raw CBOR to buffer.
// PREVENTS: Interface compliance issues.
func TestBlobWriteTo(t *testing.T) {
	raw, _ := hex.DecodeString("83010203") // array [1,2,3]
	blob := Blob(raw)

	buf := make([]byte, 64)
	n := blob.WriteTo(buf, 0)

	if n != 4 {
		t.Errorf("WriteTo length = %d, want 4", n)
	}
	if !bytes.Equal(buf[:n], raw) {
		t.Errorf("WriteTo content = %v, want %v", buf[:n], raw)
	}
}

// TestBlobWriteToWithOffset verifies WriteTo respects offset.
//
// VALIDATES: Blob writes at correct position in buffer.
// PREVENTS: Ignoring offset parameter.
func TestBlobWriteToWithOffset(t *testing.T) {
	raw, _ := hex.DecodeString("83010203")
	blob := Blob(raw)

	buf := make([]byte, 64)
	buf[0] = 0xFF // marker before
	n := blob.WriteTo(buf, 10)

	if buf[0] != 0xFF {
		t.Error("WriteTo overwrote data before offset")
	}
	if n != 4 {
		t.Errorf("WriteTo length = %d, want 4", n)
	}
	if !bytes.Equal(buf[10:14], raw) {
		t.Errorf("WriteTo at offset = %v, want %v", buf[10:14], raw)
	}
}

// TestEncoderBuilder verifies fluent encoder API.
//
// VALIDATES: Builder pattern produces correct CBOR.
// PREVENTS: Incorrect chaining or state corruption.
func TestEncoderBuilder(t *testing.T) {
	buf := make([]byte, 64)
	enc := NewEncoder(buf, 0)

	// Build: {"a": 1, "b": [2, 3]}
	n := enc.
		MapHeader(2).
		String("a").Uint(1).
		String("b").ArrayHeader(2).Uint(2).Uint(3).
		Len()

	want := "a2" + // map(2)
		"6161" + "01" + // "a": 1
		"6162" + "82" + "02" + "03" // "b": [2, 3]

	got := hex.EncodeToString(buf[:n])
	if got != want {
		t.Errorf("Encoder built = %s, want %s", got, want)
	}
}

// TestRoundTrip verifies encode-decode cycle.
//
// VALIDATES: Data survives full round-trip.
// PREVENTS: Information loss in encoding/decoding.
func TestRoundTrip(t *testing.T) {
	// Encode
	buf := make([]byte, 64)
	n := EncodeString(buf, 0, "hello")
	n += EncodeUint(buf, n, 42)
	n += EncodeBytes(buf, n, []byte{0xDE, 0xAD})

	// Decode
	off := 0
	str, strLen, _ := DecodeString(buf, off)
	off += strLen
	num, numLen, _ := DecodeUint(buf, off)
	off += numLen
	data, dataLen, _ := DecodeBytes(buf, off)
	off += dataLen

	if str != "hello" {
		t.Errorf("round-trip string = %q, want hello", str)
	}
	if num != 42 {
		t.Errorf("round-trip uint = %d, want 42", num)
	}
	if !bytes.Equal(data, []byte{0xDE, 0xAD}) {
		t.Errorf("round-trip bytes = %v, want [DE AD]", data)
	}
	if off != n {
		t.Errorf("round-trip consumed %d bytes, wrote %d", off, n)
	}
}
