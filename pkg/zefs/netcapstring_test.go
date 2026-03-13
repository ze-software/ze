package zefs

import (
	"bytes"
	"testing"
)

// VALIDATES: netcapstring encoding produces fixed-width header + data + zero padding
// PREVENTS: variable-width headers that shift offsets on size changes

func TestNetcapstringEncode(t *testing.T) {
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
			got, err := encodeNetcapstring(tt.data, tt.capacity)
			if err != nil {
				t.Fatalf("encodeNetcapstring: %v", err)
			}
			if !bytes.Equal(got, []byte(tt.want)) {
				t.Errorf("encodeNetcapstring(%q, %d)\ngot:  %q\nwant: %q", tt.data, tt.capacity, got, tt.want)
			}
		})
	}
}

// VALIDATES: netcapstring decoding reads fixed-width header and extracts used data
// PREVENTS: reading beyond capacity or misaligned offsets

func TestNetcapstringDecode(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantData []byte
		wantCap  int
		wantNext int // offset after this netcapstring
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
		{
			name:    "invalid capacity number",
			input:   []byte("0000005:000abcd:hello\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, cap_, next, err := decodeNetcapstring(tt.input, 0)
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

func TestNetcapstringRoundTrip(t *testing.T) {
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
			encoded, encErr := encodeNetcapstring(tt.data, tt.capacity)
			if encErr != nil {
				t.Fatalf("encode error: %v", encErr)
			}
			data, cap_, _, err := decodeNetcapstring(encoded, 0)
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

func TestNetcapstringDecodeAtOffset(t *testing.T) {
	// Encode two netcapstrings back-to-back
	ns1, err := encodeNetcapstring([]byte("first"), 8)
	if err != nil {
		t.Fatal(err)
	}
	ns2, err := encodeNetcapstring([]byte("second"), 8)
	if err != nil {
		t.Fatal(err)
	}
	var buf []byte
	buf = append(buf, ns1...)
	buf = append(buf, ns2...)

	// Decode first at offset 0
	data1, cap1, next1, err := decodeNetcapstring(buf, 0)
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
	data2, cap2, next2, err := decodeNetcapstring(buf, next1)
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

// VALIDATES: decodeNetcapstringRef returns sub-slice (zero-copy)
// PREVENTS: unnecessary allocation on the hot read path

func TestNetcapstringDecodeRefZeroCopy(t *testing.T) {
	encoded, err := encodeNetcapstring([]byte("hello"), 16)
	if err != nil {
		t.Fatal(err)
	}

	data, _, _, err := decodeNetcapstringRef(encoded, 0)
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
		t.Error("decodeNetcapstringRef should return sub-slice sharing backing array")
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

// VALIDATES: decodeNetcapstringRef returns errors for malformed input
// PREVENTS: panic or silent corruption on bad data in zero-copy path

func TestNetcapstringDecodeRefErrors(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		off   int
	}{
		{"truncated header", []byte("00000"), 0},
		{"malformed colon", []byte("0000005X0000016:hello\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"), 0},
		{"truncated data", []byte("0000005:0000016:hel"), 0},
		{"offset past end", []byte("0000005:0000016:hello\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"), 100},
		{"used exceeds capacity", []byte("0000010:0000004:abcdefghij"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := decodeNetcapstringRef(tt.input, tt.off)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// VALIDATES: decodeNetcapstringRef at non-zero offset chains correctly
// PREVENTS: offset arithmetic bugs in zero-copy path

func TestNetcapstringDecodeRefAtOffset(t *testing.T) {
	ns1, err := encodeNetcapstring([]byte("aaa"), 8)
	if err != nil {
		t.Fatal(err)
	}
	ns2, err := encodeNetcapstring([]byte("bbb"), 8)
	if err != nil {
		t.Fatal(err)
	}
	var buf []byte
	buf = append(buf, ns1...)
	buf = append(buf, ns2...)

	// Decode first (zero-copy)
	data1, _, next1, err := decodeNetcapstringRef(buf, 0)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if string(data1) != "aaa" {
		t.Errorf("first: got %q", data1)
	}

	// Decode second at returned offset (zero-copy)
	data2, _, next2, err := decodeNetcapstringRef(buf, next1)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if string(data2) != "bbb" {
		t.Errorf("second: got %q", data2)
	}
	if next2 != len(buf) {
		t.Errorf("next2: got %d, want %d", next2, len(buf))
	}
}

// VALIDATES: decodeNetcapstring and decodeNetcapstringRef agree on output
// PREVENTS: divergence between copy and zero-copy decode paths

func TestNetcapstringDecodeCopyVsRef(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		cap_ int
	}{
		{"short", []byte("hi"), 8},
		{"empty", []byte{}, 4},
		{"exact", []byte("1234"), 4},
		{"with nulls", []byte("a\x00b"), 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := encodeNetcapstring(tt.data, tt.cap_)
			if err != nil {
				t.Fatal(err)
			}

			copyData, copyCap, copyNext, err := decodeNetcapstring(encoded, 0)
			if err != nil {
				t.Fatalf("copy decode: %v", err)
			}

			refData, refCap, refNext, err := decodeNetcapstringRef(encoded, 0)
			if err != nil {
				t.Fatalf("ref decode: %v", err)
			}

			if !bytes.Equal(copyData, refData) {
				t.Errorf("data mismatch: copy=%q ref=%q", copyData, refData)
			}
			if copyCap != refCap {
				t.Errorf("cap mismatch: copy=%d ref=%d", copyCap, refCap)
			}
			if copyNext != refNext {
				t.Errorf("next mismatch: copy=%d ref=%d", copyNext, refNext)
			}
		})
	}
}

// VALIDATES: parseHeader rejects header shorter than 16 bytes
// PREVENTS: out-of-bounds access on short input

func TestParseHeaderTooShort(t *testing.T) {
	_, _, err := parseHeader([]byte("0000005:000001"))
	if err == nil {
		t.Error("expected error for short header")
	}
}

// VALIDATES: encodeNetcapstring rejects values exceeding 7-digit header limit
// PREVENTS: silent framing corruption on large entries

// VALIDATES: encodeNetcapstring silently truncates when capacity < len(data)
// PREVENTS: assumption that encode rejects data larger than capacity

func TestNetcapstringEncodeCapacityLessThanData(t *testing.T) {
	data := []byte("hello world") // 11 bytes
	capacity := 5                 // less than data length

	encoded, err := encodeNetcapstring(data, capacity)
	if err != nil {
		t.Fatalf("encodeNetcapstring: %v", err)
	}

	// Decode and verify: only first 'capacity' bytes of data are in the buffer,
	// but header says used=11. Decode should fail because used > capacity.
	// Actually: header says used=11, capacity=5. parseHeader rejects used > capacity.
	_, _, _, decErr := decodeNetcapstring(encoded, 0)
	if decErr == nil {
		t.Error("decode should fail: used exceeds capacity in header")
	}
}

// VALIDATES: encodeNetcapstring with capacity exactly equal to data length
// PREVENTS: off-by-one in zero-padding (no padding needed)

func TestNetcapstringEncodeExactCapacity(t *testing.T) {
	data := []byte("exact")
	capacity := len(data)

	encoded, err := encodeNetcapstring(data, capacity)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != headerLen+capacity {
		t.Errorf("encoded length: got %d, want %d", len(encoded), headerLen+capacity)
	}

	decoded, cap_, _, err := decodeNetcapstring(encoded, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, data) {
		t.Errorf("round-trip: got %q, want %q", decoded, data)
	}
	if cap_ != capacity {
		t.Errorf("capacity: got %d, want %d", cap_, capacity)
	}
}

// VALIDATES: decodeNetcapstring at non-zero offset with truncated data region
// PREVENTS: silent corruption when offset calculation leaves insufficient bytes

func TestNetcapstringDecodeOffsetTruncatedData(t *testing.T) {
	// Build a valid netcapstring then place it partway into a buffer with
	// enough room for the header but not the data.
	hdr := []byte("0000005:0000016:") // used=5, capacity=16
	// Put 10 bytes of prefix, then the header, but only 8 bytes of data (need 16)
	var buf []byte
	buf = append(buf, make([]byte, 10)...)
	buf = append(buf, hdr...)
	buf = append(buf, []byte("12345678")...) // only 8, need 16

	_, _, _, err := decodeNetcapstring(buf, 10)
	if err == nil {
		t.Error("expected error for truncated data at non-zero offset")
	}
}

// VALIDATES: writeHeader at maxHeaderVal boundary produces correct 7-digit format
// PREVENTS: overflow or misformat in fixed-width header at maximum value

func TestWriteHeaderMaxValues(t *testing.T) {
	buf := make([]byte, headerLen)
	writeHeader(buf, maxHeaderVal, maxHeaderVal)
	want := "9999999:9999999:"
	if string(buf) != want {
		t.Errorf("writeHeader(max, max): got %q, want %q", buf, want)
	}
}

// VALIDATES: writeHeader with zero values
// PREVENTS: leading-zero formatting issues

func TestWriteHeaderZeroValues(t *testing.T) {
	buf := make([]byte, headerLen)
	writeHeader(buf, 0, 0)
	want := "0000000:0000000:"
	if string(buf) != want {
		t.Errorf("writeHeader(0, 0): got %q, want %q", buf, want)
	}
}

// VALIDATES: growCapacity always returns >= dataLen
// PREVENTS: allocated capacity smaller than needed

func TestGrowCapacityAlwaysFits(t *testing.T) {
	for _, dataLen := range []int{0, 1, 63, 64, 65, 100, 1000, maxHeaderVal - 1, maxHeaderVal} {
		cap_ := growCapacity(dataLen, 0)
		if cap_ < dataLen {
			t.Errorf("growCapacity(%d, 0) = %d, must be >= dataLen", dataLen, cap_)
		}
		if cap_ > maxHeaderVal {
			t.Errorf("growCapacity(%d, 0) = %d, must be <= maxHeaderVal", dataLen, cap_)
		}
	}
}

func TestNetcapstringHeaderOverflow(t *testing.T) {
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
			_, err := encodeNetcapstring(data, tt.capacity)
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
