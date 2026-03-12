package zefs

import (
	"bytes"
	"testing"
)

// VALIDATES: netstring encoding produces fixed-width header + data + zero padding
// PREVENTS: variable-width headers that shift offsets on size changes

func TestNetstringEncode(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		capacity int
		want     string // expected output as string for readability
	}{
		{
			name:     "simple data with padding",
			data:     []byte("hello"),
			capacity: 16,
			want:     "0000005:0000016:hello\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00",
		},
		{
			name:     "empty data",
			data:     []byte{},
			capacity: 8,
			want:     "0000000:0000008:\x00\x00\x00\x00\x00\x00\x00\x00",
		},
		{
			name:     "data fills capacity exactly",
			data:     []byte("abcd"),
			capacity: 4,
			want:     "0000004:0000004:abcd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encodeNetstring(tt.data, tt.capacity)
			if err != nil {
				t.Fatalf("encodeNetstring: %v", err)
			}
			if !bytes.Equal(got, []byte(tt.want)) {
				t.Errorf("encodeNetstring(%q, %d)\ngot:  %q\nwant: %q", tt.data, tt.capacity, got, tt.want)
			}
		})
	}
}

// VALIDATES: netstring decoding reads fixed-width header and extracts used data
// PREVENTS: reading beyond capacity or misaligned offsets

func TestNetstringDecode(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantData []byte
		wantCap  int
		wantNext int // offset after this netstring
		wantErr  bool
	}{
		{
			name:     "simple",
			input:    []byte("0000005:0000016:hello\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"),
			wantData: []byte("hello"),
			wantCap:  16,
			wantNext: headerLen + 16,
		},
		{
			name:     "empty data",
			input:    []byte("0000000:0000008:\x00\x00\x00\x00\x00\x00\x00\x00"),
			wantData: []byte{},
			wantCap:  8,
			wantNext: headerLen + 8,
		},
		{
			name:     "exact fill",
			input:    []byte("0000004:0000004:abcd"),
			wantData: []byte("abcd"),
			wantCap:  4,
			wantNext: headerLen + 4,
		},
		{
			name:    "truncated header",
			input:   []byte("00000"),
			wantErr: true,
		},
		{
			name:    "invalid used number",
			input:   []byte("000abcd:0000016:hello\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"),
			wantErr: true,
		},
		{
			name:    "negative capacity",
			input:   []byte("0000001:-000001:x"),
			wantErr: true,
		},
		{
			name:    "negative used",
			input:   []byte("-000001:0000016:hello\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"),
			wantErr: true,
		},
		{
			name:    "truncated data",
			input:   []byte("0000005:0000016:hel"),
			wantErr: true,
		},
		{
			name:    "used exceeds capacity",
			input:   []byte("0000010:0000004:abcdefghij"),
			wantErr: true,
		},
		{
			name:    "malformed colon position 7",
			input:   []byte("0000005X0000016:hello\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"),
			wantErr: true,
		},
		{
			name:    "malformed colon position 15",
			input:   []byte("0000005:0000016Xhello\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, cap_, next, err := decodeNetstring(tt.input, 0)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(data, tt.wantData) {
				t.Errorf("data: got %q, want %q", data, tt.wantData)
			}
			if cap_ != tt.wantCap {
				t.Errorf("capacity: got %d, want %d", cap_, tt.wantCap)
			}
			if next != tt.wantNext {
				t.Errorf("next offset: got %d, want %d", next, tt.wantNext)
			}
		})
	}
}

// VALIDATES: encode then decode round-trips correctly
// PREVENTS: encode/decode mismatch

func TestNetstringRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		capacity int
	}{
		{"short text", []byte("hello"), 32},
		{"empty", []byte{}, 16},
		{"with newlines", []byte("line1\nline2\n"), 64},
		{"with colons", []byte("key:value:extra"), 32},
		{"with nulls", []byte("a\x00b\x00c"), 16},
		{"exact capacity", []byte("12345678"), 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, encErr := encodeNetstring(tt.data, tt.capacity)
			if encErr != nil {
				t.Fatalf("encode error: %v", encErr)
			}
			data, cap_, _, err := decodeNetstring(encoded, 0)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if !bytes.Equal(data, tt.data) {
				t.Errorf("round-trip data: got %q, want %q", data, tt.data)
			}
			if cap_ != tt.capacity {
				t.Errorf("round-trip capacity: got %d, want %d", cap_, tt.capacity)
			}
		})
	}
}

// VALIDATES: decoding works at non-zero offsets and chains via next
// PREVENTS: offset arithmetic errors in multi-entry parsing

func TestNetstringDecodeAtOffset(t *testing.T) {
	// Encode two netstrings back-to-back
	ns1, err := encodeNetstring([]byte("first"), 8)
	if err != nil {
		t.Fatal(err)
	}
	ns2, err := encodeNetstring([]byte("second"), 8)
	if err != nil {
		t.Fatal(err)
	}
	var buf []byte
	buf = append(buf, ns1...)
	buf = append(buf, ns2...)

	// Decode first at offset 0
	data1, cap1, next1, err := decodeNetstring(buf, 0)
	if err != nil {
		t.Fatalf("first decode: %v", err)
	}
	if string(data1) != "first" {
		t.Errorf("first: got %q", data1)
	}
	if cap1 != 8 {
		t.Errorf("first cap: got %d, want 8", cap1)
	}

	// Decode second at the offset returned by first
	data2, cap2, next2, err := decodeNetstring(buf, next1)
	if err != nil {
		t.Fatalf("second decode: %v", err)
	}
	if string(data2) != "second" {
		t.Errorf("second: got %q", data2)
	}
	if cap2 != 8 {
		t.Errorf("second cap: got %d, want 8", cap2)
	}
	if next2 != len(buf) {
		t.Errorf("next2: got %d, want %d", next2, len(buf))
	}
}

// VALIDATES: decodeNetstringRef returns sub-slice (zero-copy)
// PREVENTS: unnecessary allocation on the hot read path

func TestNetstringDecodeRefZeroCopy(t *testing.T) {
	encoded, err := encodeNetstring([]byte("hello"), 16)
	if err != nil {
		t.Fatal(err)
	}

	data, _, _, err := decodeNetstringRef(encoded, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", data, "hello")
	}

	// Verify zero-copy: data should share backing array with encoded
	// Modifying encoded's data region should be visible through data
	encoded[headerLen] = 'H'
	if data[0] != 'H' {
		t.Error("decodeNetstringRef should return sub-slice sharing backing array")
	}

	// Verify capacity is capped to prevent access to padding
	if cap(data) != len(data) {
		t.Errorf("cap should be capped to len: cap=%d, len=%d", cap(data), len(data))
	}
}

// VALIDATES: growCapacity produces correct sizes for edge cases
// PREVENTS: incorrect spare capacity or unbounded growth

func TestGrowCapacity(t *testing.T) {
	tests := []struct {
		name       string
		dataLen    int
		currentCap int
		wantMin    int // result must be >= this
		wantMax    int // result must be <= this
	}{
		{"zero data", 0, 0, 64, 64},
		{"small data below minimum", 10, 0, 64, 64},
		{"data at minimum", 64, 64, 71, 128},
		{"data requiring doubling", 200, 64, 221, 256},
		{"large data", 8192, 256, 9012, 16384},
		{"near max", maxHeaderVal - 100, 0, maxHeaderVal - 100, maxHeaderVal},
		{"at max", maxHeaderVal, 0, maxHeaderVal, maxHeaderVal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := growCapacity(tt.dataLen, tt.currentCap)
			if got < tt.wantMin {
				t.Errorf("growCapacity(%d, %d) = %d, want >= %d", tt.dataLen, tt.currentCap, got, tt.wantMin)
			}
			if got > tt.wantMax {
				t.Errorf("growCapacity(%d, %d) = %d, want <= %d", tt.dataLen, tt.currentCap, got, tt.wantMax)
			}
			if got < tt.dataLen {
				t.Errorf("growCapacity(%d, %d) = %d, must be >= dataLen", tt.dataLen, tt.currentCap, got)
			}
		})
	}
}

// VALIDATES: encodeNetstring rejects values exceeding 7-digit header limit
// PREVENTS: silent framing corruption on large entries

func TestNetstringHeaderOverflow(t *testing.T) {
	tests := []struct {
		name     string
		dataLen  int
		capacity int
	}{
		{"data exceeds max", maxHeaderVal + 1, maxHeaderVal + 1},
		{"capacity exceeds max", 0, maxHeaderVal + 1},
		{"both at max are ok", 0, maxHeaderVal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.dataLen)
			_, err := encodeNetstring(data, tt.capacity)
			if tt.capacity <= maxHeaderVal && tt.dataLen <= maxHeaderVal {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("expected error for overflow, got nil")
				}
			}
		})
	}
}
