package zefs

import (
	"bytes"
	"testing"
)

// VALIDATES: netcapstring encoding produces self-describing header + data + zero padding
// PREVENTS: malformed headers that break parsing

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
			// number=2 (digitCount(16)=2), header=:2:16:05:
			want: ":2:16:05:hello\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00",
		},
		{
			name:     "empty data",
			data:     []byte{},
			capacity: 8,
			// number=1, header=:1:8:0:
			want: ":1:8:0:\x00\x00\x00\x00\x00\x00\x00\x00",
		},
		{
			name:     "data fills capacity exactly",
			data:     []byte("abcd"),
			capacity: 4,
			// number=1, header=:1:4:4:
			want: ":1:4:4:abcd",
		},
		{
			name:     "large capacity three digits",
			data:     []byte("x"),
			capacity: 100,
			// number=3 (digitCount(100)=3), header=:3:100:001:
			want: ":3:100:001:x" + string(make([]byte, 99)),
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

// VALIDATES: netcapstring decoding reads self-describing header and extracts used data
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
			input:    []byte(":2:16:05:hello\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"),
			wantData: []byte("hello"),
			wantCap:  16,
			wantNext: netcapstringHeaderLen(16) + 16,
		},
		{
			name:     "empty data",
			input:    []byte(":1:8:0:\x00\x00\x00\x00\x00\x00\x00\x00"),
			wantData: []byte{},
			wantCap:  8,
			wantNext: netcapstringHeaderLen(8) + 8,
		},
		{
			name:     "exact fill",
			input:    []byte(":1:4:4:abcd"),
			wantData: []byte("abcd"),
			wantCap:  4,
			wantNext: netcapstringHeaderLen(4) + 4,
		},
		{
			name:    "truncated header",
			input:   []byte(":2"),
			wantErr: true,
		},
		{
			name:    "missing leading colon",
			input:   []byte("2:16:05:hello"),
			wantErr: true,
		},
		{
			name:    "invalid number field",
			input:   []byte(":abc:16:05:hello"),
			wantErr: true,
		},
		{
			name:    "zero number field",
			input:   []byte(":0:16:05:hello"),
			wantErr: true,
		},
		{
			name:    "truncated data",
			input:   []byte(":2:16:05:hel"),
			wantErr: true,
		},
		{
			name:    "used exceeds capacity",
			input:   []byte(":2:04:10:abcdefghij"),
			wantErr: true,
		},
		{
			name:    "truncated capacity field",
			input:   []byte(":2:1"),
			wantErr: true,
		},
		{
			name:    "missing colon after capacity",
			input:   []byte(":2:16X05:hello"),
			wantErr: true,
		},
		{
			name:    "missing colon after used",
			input:   []byte(":2:16:05Xhello"),
			wantErr: true,
		},
		{
			name:    "invalid capacity digits",
			input:   []byte(":2:ab:05:hello"),
			wantErr: true,
		},
		{
			name:    "invalid used digits",
			input:   []byte(":2:16:xy:hello"),
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
		{"large capacity", []byte("test"), 10000},
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
	dataOffset := netcapstringHeaderLen(16)
	encoded[dataOffset] = 'H'
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
		{"very large", 1_000_000, 0, 1_100_001, 2_097_152},
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
		{"truncated at colon", []byte(":"), 0},
		{"missing leading colon", []byte("2:16:05:hello"), 0},
		{"truncated number", []byte(":2"), 0},
		{"truncated data", []byte(":2:16:05:hel"), 0},
		{"offset past end", []byte(":1:8:5:hello\x00\x00\x00"), 100},
		{"used exceeds capacity", []byte(":2:04:10:abcdefghij"), 0},
		{"number too large", []byte(":99:"), 0},
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

// VALIDATES: encodeNetcapstring rejects capacity less than data length
// PREVENTS: producing headers where used > capacity

func TestNetcapstringEncodeCapacityLessThanData(t *testing.T) {
	data := []byte("hello world") // 11 bytes
	capacity := 5                 // less than data length

	_, err := encodeNetcapstring(data, capacity)
	if err == nil {
		t.Error("encode should fail: data length exceeds capacity")
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
	wantLen := netcapstringHeaderLen(capacity) + capacity
	if len(encoded) != wantLen {
		t.Errorf("encoded length: got %d, want %d", len(encoded), wantLen)
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
	// Build a valid netcapstring header with cap=16, used=5, but provide only 8 bytes of data
	hdr := []byte(":2:16:05:")
	var buf []byte
	buf = append(buf, make([]byte, 10)...)
	buf = append(buf, hdr...)
	buf = append(buf, []byte("12345678")...) // only 8, need 16

	_, _, _, err := decodeNetcapstring(buf, 10)
	if err == nil {
		t.Error("expected error for truncated data at non-zero offset")
	}
}

// VALIDATES: digitCount returns correct digit counts
// PREVENTS: wrong header widths from miscounted digits

func TestDigitCount(t *testing.T) {
	tests := []struct {
		n    int
		want int
	}{
		{0, 1},
		{1, 1},
		{9, 1},
		{10, 2},
		{99, 2},
		{100, 3},
		{999, 3},
		{1000, 4},
		{9999, 4},
		{10000, 5},
		{1000000, 7},
	}
	for _, tt := range tests {
		got := digitCount(tt.n)
		if got != tt.want {
			t.Errorf("digitCount(%d) = %d, want %d", tt.n, got, tt.want)
		}
	}
}

// VALIDATES: netcapstringHeaderLen matches actual encoded header length
// PREVENTS: header length calculation divergence from encoding

func TestNetcapstringHeaderLen(t *testing.T) {
	caps := []int{0, 1, 9, 10, 99, 100, 999, 1000, 9999, 10000, 100000}
	for _, cap_ := range caps {
		encoded, err := encodeNetcapstring([]byte{}, cap_)
		if err != nil {
			t.Fatalf("encode cap=%d: %v", cap_, err)
		}
		wantLen := netcapstringHeaderLen(cap_) + cap_
		if len(encoded) != wantLen {
			t.Errorf("cap=%d: len(encoded)=%d, headerLen+cap=%d", cap_, len(encoded), wantLen)
		}
	}
}

// VALIDATES: growCapacity always returns >= dataLen
// PREVENTS: allocated capacity smaller than needed

func TestGrowCapacityAlwaysFits(t *testing.T) {
	for _, dataLen := range []int{0, 1, 63, 64, 65, 100, 1000, 100000, 1000000} {
		cap_ := growCapacity(dataLen, 0)
		if cap_ < dataLen {
			t.Errorf("growCapacity(%d, 0) = %d, must be >= dataLen", dataLen, cap_)
		}
	}
}

// VALIDATES: writeNetcapstring writes into caller-provided buffer at offset
// PREVENTS: allocation per entry during store serialization

func TestWriteNetcapstring(t *testing.T) {
	data := []byte("hello")
	capacity := 16

	// Allocate buffer with room for the netcapstring
	total := netcapstringTotalLen(capacity)
	buf := make([]byte, total)
	n := writeNetcapstring(buf, 0, data, capacity)

	if n != total {
		t.Errorf("writeNetcapstring returned %d, want %d", n, total)
	}

	// Verify round-trip: decode what was written
	decoded, cap_, next, err := decodeNetcapstringRef(buf, 0)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(decoded) != "hello" {
		t.Errorf("data: got %q, want %q", decoded, "hello")
	}
	if cap_ != capacity {
		t.Errorf("capacity: got %d, want %d", cap_, capacity)
	}
	if next != total {
		t.Errorf("next: got %d, want %d", next, total)
	}
}

// VALIDATES: writeNetcapstring at non-zero offset chains correctly
// PREVENTS: offset arithmetic bugs in WriteTo pattern

func TestWriteNetcapstringAtOffset(t *testing.T) {
	cap1, cap2 := 8, 16
	total := netcapstringTotalLen(cap1) + netcapstringTotalLen(cap2)
	buf := make([]byte, total)

	// Write two entries back-to-back
	off := writeNetcapstring(buf, 0, []byte("aaa"), cap1)
	off += writeNetcapstring(buf, off, []byte("bbb"), cap2)

	if off != total {
		t.Errorf("total written: %d, want %d", off, total)
	}

	// Decode both
	d1, _, next, err := decodeNetcapstringRef(buf, 0)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if string(d1) != "aaa" {
		t.Errorf("first: got %q", d1)
	}

	d2, _, _, err := decodeNetcapstringRef(buf, next)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if string(d2) != "bbb" {
		t.Errorf("second: got %q", d2)
	}
}

// VALIDATES: writeNetcapstring produces identical output to encodeNetcapstring
// PREVENTS: divergence between allocating and WriteTo paths

func TestWriteNetcapstringMatchesEncode(t *testing.T) {
	tests := []struct {
		data     []byte
		capacity int
	}{
		{[]byte("hello"), 16},
		{[]byte{}, 8},
		{[]byte("exact"), 5},
		{[]byte("large"), 10000},
	}
	for _, tt := range tests {
		encoded, err := encodeNetcapstring(tt.data, tt.capacity)
		if err != nil {
			t.Fatal(err)
		}

		buf := make([]byte, netcapstringTotalLen(tt.capacity))
		writeNetcapstring(buf, 0, tt.data, tt.capacity)

		if !bytes.Equal(buf, encoded) {
			t.Errorf("data=%q cap=%d: WriteTo and encode differ", tt.data, tt.capacity)
		}
	}
}

// VALIDATES: writeNetcapstring zeros padding on overwrite
// PREVENTS: stale data leaking through padding on in-place writes

func TestWriteNetcapstringZerosPadding(t *testing.T) {
	capacity := 16
	total := netcapstringTotalLen(capacity)

	// Fill buffer with 0xFF to simulate non-zeroed memory
	buf := make([]byte, total)
	for i := range buf {
		buf[i] = 0xFF
	}

	// Write short data into the buffer
	writeNetcapstring(buf, 0, []byte("hi"), capacity)

	// Decode and verify data
	data, _, _, err := decodeNetcapstringRef(buf, 0)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(data) != "hi" {
		t.Errorf("data: got %q, want %q", data, "hi")
	}

	// Verify padding bytes are zero (not 0xFF)
	hdrLen := netcapstringHeaderLen(capacity)
	dataEnd := hdrLen + 2 // "hi" = 2 bytes
	for i := dataEnd; i < total; i++ {
		if buf[i] != 0 {
			t.Errorf("padding byte %d is 0x%02X, want 0x00", i, buf[i])
			break
		}
	}
}

// VALIDATES: writeNetcapstringHeader writes only the header portion
// PREVENTS: header-only writes corrupting data region

func TestWriteNetcapstringHeader(t *testing.T) {
	capacity := 100
	hdrLen := netcapstringHeaderLen(capacity)
	buf := make([]byte, hdrLen)

	n := writeNetcapstringHeader(buf, 0, capacity, 42)
	if n != hdrLen {
		t.Errorf("header wrote %d bytes, want %d", n, hdrLen)
	}

	// Parse the header by decoding (with fake data region)
	full := make([]byte, hdrLen+capacity)
	copy(full, buf)
	_, cap_, _, err := decodeNetcapstringRef(full, 0)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cap_ != capacity {
		t.Errorf("capacity: got %d, want %d", cap_, capacity)
	}
}

// VALIDATES: netcapSlot methods provide correct derived values and in-place writes
// PREVENTS: offset arithmetic errors in slot-based access

func TestNetcapSlotDerivedValues(t *testing.T) {
	capacity := 64
	data := []byte("hello world")
	total := netcapstringTotalLen(capacity)
	buf := make([]byte, total)
	writeNetcapstring(buf, 0, data, capacity)

	slot := netcapSlot{offset: 0, capacity: capacity, used: len(data)}

	if slot.headerLen() != netcapstringHeaderLen(capacity) {
		t.Errorf("headerLen: got %d, want %d", slot.headerLen(), netcapstringHeaderLen(capacity))
	}
	if slot.totalLen() != total {
		t.Errorf("totalLen: got %d, want %d", slot.totalLen(), total)
	}
	if slot.dataOffset() != slot.headerLen() {
		t.Errorf("dataOffset: got %d, want %d", slot.dataOffset(), slot.headerLen())
	}

	// Zero-copy data access
	got := slot.data(buf)
	if string(got) != "hello world" {
		t.Errorf("data: got %q, want %q", got, "hello world")
	}
}

func TestNetcapSlotWriteData(t *testing.T) {
	capacity := 32
	total := netcapstringTotalLen(capacity)
	buf := make([]byte, total)

	slot := netcapSlot{offset: 0, capacity: capacity, used: 0}
	if err := slot.writeData(buf, []byte("first")); err != nil {
		t.Fatalf("writeData: %v", err)
	}

	if slot.used != 5 {
		t.Errorf("used after writeData: got %d, want 5", slot.used)
	}

	// Verify round-trip
	decoded, _, _, err := decodeNetcapstringRef(buf, 0)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(decoded) != "first" {
		t.Errorf("decoded: got %q, want %q", decoded, "first")
	}

	// Overwrite with shorter data
	if err := slot.writeData(buf, []byte("hi")); err != nil {
		t.Fatalf("writeData shorter: %v", err)
	}
	if slot.used != 2 {
		t.Errorf("used after shorter write: got %d, want 2", slot.used)
	}
	decoded, _, _, err = decodeNetcapstringRef(buf, 0)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(decoded) != "hi" {
		t.Errorf("decoded: got %q, want %q", decoded, "hi")
	}
}

func TestNetcapSlotWriteAt(t *testing.T) {
	capacity := 32
	total := netcapstringTotalLen(capacity)
	buf := make([]byte, total)

	slot := netcapSlot{offset: 0, capacity: capacity, used: 0}

	// Write at offset 0 -- used should become 5
	if err := slot.writeAt(buf, 0, []byte("hello")); err != nil {
		t.Fatalf("writeAt(0): %v", err)
	}
	if slot.used != 5 {
		t.Errorf("used after writeAt(0): got %d, want 5", slot.used)
	}

	// Write at offset 10 -- used should extend to 13
	if err := slot.writeAt(buf, 10, []byte("end")); err != nil {
		t.Fatalf("writeAt(10): %v", err)
	}
	if slot.used != 13 {
		t.Errorf("used after writeAt(10): got %d, want 13", slot.used)
	}

	// Write within existing used range -- used should not change
	if err := slot.writeAt(buf, 2, []byte("ll")); err != nil {
		t.Fatalf("writeAt(2): %v", err)
	}
	if slot.used != 13 {
		t.Errorf("used after writeAt(2): got %d, want 13 (unchanged)", slot.used)
	}

	// Verify the data region content
	got := slot.data(buf)
	if len(got) != 13 {
		t.Fatalf("data length: got %d, want 13", len(got))
	}
	if string(got[:5]) != "hello" {
		t.Errorf("data[:5]: got %q, want %q", got[:5], "hello")
	}
	if string(got[10:13]) != "end" {
		t.Errorf("data[10:13]: got %q, want %q", got[10:13], "end")
	}
}

// VALIDATES: writeData rejects data exceeding capacity
// PREVENTS: buffer overwrite from oversized data

func TestNetcapSlotWriteDataExceedsCapacity(t *testing.T) {
	capacity := 8
	buf := make([]byte, netcapstringTotalLen(capacity))
	slot := netcapSlot{offset: 0, capacity: capacity, used: 0}

	err := slot.writeData(buf, []byte("too long for slot"))
	if err == nil {
		t.Error("expected error for data exceeding capacity")
	}
	if slot.used != 0 {
		t.Errorf("used should be unchanged after error, got %d", slot.used)
	}
}

// VALIDATES: writeData with data exactly at capacity boundary
// PREVENTS: off-by-one on boundary

func TestNetcapSlotWriteDataExactCapacity(t *testing.T) {
	capacity := 5
	buf := make([]byte, netcapstringTotalLen(capacity))
	slot := netcapSlot{offset: 0, capacity: capacity, used: 0}

	err := slot.writeData(buf, []byte("exact"))
	if err != nil {
		t.Fatalf("writeData at exact capacity: %v", err)
	}
	if slot.used != 5 {
		t.Errorf("used: got %d, want 5", slot.used)
	}
}

// VALIDATES: writeData on zero-capacity slot with empty data
// PREVENTS: edge case with minimal slot

func TestNetcapSlotWriteDataZeroCapacity(t *testing.T) {
	capacity := 0
	buf := make([]byte, netcapstringTotalLen(capacity))
	slot := netcapSlot{offset: 0, capacity: capacity, used: 0}

	err := slot.writeData(buf, []byte{})
	if err != nil {
		t.Fatalf("writeData empty on zero-capacity: %v", err)
	}
	if slot.used != 0 {
		t.Errorf("used: got %d, want 0", slot.used)
	}

	// Any non-empty data should fail
	err = slot.writeData(buf, []byte("x"))
	if err == nil {
		t.Error("expected error for data on zero-capacity slot")
	}
}

// VALIDATES: writeAt rejects negative offset
// PREVENTS: writing before the data region

func TestNetcapSlotWriteAtNegativeOffset(t *testing.T) {
	capacity := 32
	buf := make([]byte, netcapstringTotalLen(capacity))
	slot := netcapSlot{offset: 0, capacity: capacity, used: 0}

	err := slot.writeAt(buf, -1, []byte("data"))
	if err == nil {
		t.Error("expected error for negative offset")
	}
	if slot.used != 0 {
		t.Errorf("used should be unchanged after error, got %d", slot.used)
	}
}

// VALIDATES: writeAt rejects empty data
// PREVENTS: used counter bumped without writing anything

func TestNetcapSlotWriteAtEmptyData(t *testing.T) {
	capacity := 32
	buf := make([]byte, netcapstringTotalLen(capacity))
	slot := netcapSlot{offset: 0, capacity: capacity, used: 5}

	err := slot.writeAt(buf, 10, []byte{})
	if err == nil {
		t.Error("expected error for empty data")
	}
	if slot.used != 5 {
		t.Errorf("used should be unchanged after error, got %d", slot.used)
	}
}

// VALIDATES: writeAt rejects write past capacity
// PREVENTS: overwriting adjacent slot's data

func TestNetcapSlotWriteAtExceedsCapacity(t *testing.T) {
	capacity := 16
	buf := make([]byte, netcapstringTotalLen(capacity))
	slot := netcapSlot{offset: 0, capacity: capacity, used: 0}

	// Write starting at offset 10, 10 bytes long = ends at 20, exceeds capacity 16
	err := slot.writeAt(buf, 10, []byte("1234567890"))
	if err == nil {
		t.Error("expected error for write past capacity")
	}
	if slot.used != 0 {
		t.Errorf("used should be unchanged after error, got %d", slot.used)
	}
}

// VALIDATES: writeAt at exact capacity boundary
// PREVENTS: off-by-one at end of slot

func TestNetcapSlotWriteAtExactEnd(t *testing.T) {
	capacity := 16
	buf := make([]byte, netcapstringTotalLen(capacity))
	slot := netcapSlot{offset: 0, capacity: capacity, used: 0}

	// Write 6 bytes starting at offset 10 = ends at 16 = exactly at capacity
	err := slot.writeAt(buf, 10, []byte("ending"))
	if err != nil {
		t.Fatalf("writeAt at exact end: %v", err)
	}
	if slot.used != 16 {
		t.Errorf("used: got %d, want 16", slot.used)
	}

	// One more byte would exceed
	err = slot.writeAt(buf, 16, []byte("x"))
	if err == nil {
		t.Error("expected error for write past capacity")
	}
}

// VALIDATES: writeAt on zero-capacity slot rejects all writes
// PREVENTS: any write on empty slot

func TestNetcapSlotWriteAtZeroCapacity(t *testing.T) {
	capacity := 0
	buf := make([]byte, netcapstringTotalLen(capacity))
	slot := netcapSlot{offset: 0, capacity: capacity, used: 0}

	err := slot.writeAt(buf, 0, []byte("x"))
	if err == nil {
		t.Error("expected error for write on zero-capacity slot")
	}
}

// VALIDATES: slot at non-zero offset computes correct data position
// PREVENTS: data written at wrong position when slot is not at buffer start

func TestNetcapSlotNonZeroOffset(t *testing.T) {
	capacity := 16
	slotSize := netcapstringTotalLen(capacity)
	// Two slots back-to-back
	buf := make([]byte, slotSize*2)

	slot1 := netcapSlot{offset: 0, capacity: capacity, used: 0}
	slot2 := netcapSlot{offset: slotSize, capacity: capacity, used: 0}

	if err := slot1.writeData(buf, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := slot2.writeData(buf, []byte("second")); err != nil {
		t.Fatal(err)
	}

	// Decode both and verify they didn't overlap
	d1, _, next, err := decodeNetcapstringRef(buf, 0)
	if err != nil {
		t.Fatalf("decode slot1: %v", err)
	}
	if string(d1) != "first" {
		t.Errorf("slot1: got %q", d1)
	}

	d2, _, _, err := decodeNetcapstringRef(buf, next)
	if err != nil {
		t.Fatalf("decode slot2: %v", err)
	}
	if string(d2) != "second" {
		t.Errorf("slot2: got %q", d2)
	}
}

// VALIDATES: encodeNetcapstring rejects zero capacity with non-empty data
// PREVENTS: data silently truncated

func TestEncodeNetcapstringZeroCapNonEmptyData(t *testing.T) {
	_, err := encodeNetcapstring([]byte("data"), 0)
	if err == nil {
		t.Error("expected error for non-empty data with zero capacity")
	}
}

// VALIDATES: decodeNetcapstringRef handles empty buffer
// PREVENTS: index out of range on empty input

func TestDecodeNetcapstringRefEmptyBuffer(t *testing.T) {
	_, _, _, err := decodeNetcapstringRef([]byte{}, 0)
	if err == nil {
		t.Error("expected error for empty buffer")
	}
}

// VALIDATES: decodeNetcapstringRef handles offset at end of buffer
// PREVENTS: index out of range when offset equals length

func TestDecodeNetcapstringRefOffsetAtEnd(t *testing.T) {
	buf := []byte(":1:8:3:abc\x00\x00\x00\x00\x00")
	_, _, _, err := decodeNetcapstringRef(buf, len(buf))
	if err == nil {
		t.Error("expected error for offset at end of buffer")
	}
}

// VALIDATES: digitCount handles edge cases
// PREVENTS: wrong header size computation

func TestDigitCountEdgeCases(t *testing.T) {
	tests := []struct {
		n    int
		want int
	}{
		{0, 1},
		{1, 1},
		{9, 1},
		{10, 2},
		{99, 2},
		{100, 3},
		{999999999, 9},
		{1000000000, 10},
	}
	for _, tt := range tests {
		if got := digitCount(tt.n); got != tt.want {
			t.Errorf("digitCount(%d) = %d, want %d", tt.n, got, tt.want)
		}
	}
}

// VALIDATES: netcapstringTotalLen matches actual encoded size
// PREVENTS: size calculation divergence

func TestNetcapstringTotalLen(t *testing.T) {
	for _, cap_ := range []int{0, 1, 9, 10, 99, 100, 999, 1000, 10000} {
		encoded, err := encodeNetcapstring([]byte{}, cap_)
		if err != nil {
			t.Fatal(err)
		}
		if netcapstringTotalLen(cap_) != len(encoded) {
			t.Errorf("cap=%d: totalLen=%d, encoded=%d", cap_, netcapstringTotalLen(cap_), len(encoded))
		}
	}
}

// VALIDATES: encodeNetcapstring rejects negative capacity
// PREVENTS: nonsensical header from negative capacity

func TestNetcapstringEncodeNegativeCapacity(t *testing.T) {
	_, err := encodeNetcapstring([]byte("test"), -1)
	if err == nil {
		t.Error("expected error for negative capacity")
	}
}

// VALIDATES: format with zero capacity produces valid minimal entry
// PREVENTS: edge case with zero-sized slot

// VALIDATES: decodeNetcapstringRef rejects crafted capacity that would overflow off+cap
// PREVENTS: integer overflow bypassing truncation check (CVE-class)

func TestDecodeNetcapstringRefOverflowCapacity(t *testing.T) {
	// Craft a header with number=19 and capacity = max 19-digit value.
	// This exercises the overflow-safe check: cap_ > len(buf) - off
	// On 64-bit, strconv.Atoi("9999999999999999999") = 9999999999999999999 (valid int64).
	// The subtraction check catches it without overflow.
	crafted := ":19:9999999999999999999:0000000000000000000:"
	_, _, _, err := decodeNetcapstringRef([]byte(crafted), 0)
	if err == nil {
		t.Error("expected error for capacity exceeding buffer size")
	}
}

// VALIDATES: decodeNetcapstringRef rejects used > cap even with valid header format
// PREVENTS: reading past allocated region via crafted used field

func TestDecodeNetcapstringRefCraftedUsedExceedsCap(t *testing.T) {
	// Header says used=9, cap=4. Format is valid but used > cap.
	crafted := ":1:4:9:abcdefghi"
	_, _, _, err := decodeNetcapstringRef([]byte(crafted), 0)
	if err == nil {
		t.Error("expected error for used exceeding capacity")
	}
}

func TestNetcapstringZeroCapacity(t *testing.T) {
	encoded, err := encodeNetcapstring([]byte{}, 0)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	data, cap_, _, err := decodeNetcapstring(encoded, 0)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("data should be empty, got %q", data)
	}
	if cap_ != 0 {
		t.Errorf("capacity: got %d, want 0", cap_)
	}
}
