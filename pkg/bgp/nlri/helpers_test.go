package nlri

import (
	"bytes"
	"testing"
)

// TestPrefixBytes verifies PrefixBytes returns correct byte count.
//
// VALIDATES: PrefixBytes(bits) == (bits+7)/8 for various bit lengths.
// PREVENTS: Off-by-one errors in prefix byte calculation.
func TestPrefixBytes(t *testing.T) {
	tests := []struct {
		bits int
		want int
	}{
		{0, 0},
		{1, 1},
		{7, 1},
		{8, 1},
		{9, 2},
		{15, 2},
		{16, 2},
		{17, 3},
		{24, 3},
		{25, 4},
		{32, 4},   // IPv4 /32
		{128, 16}, // IPv6 /128
	}

	for _, tt := range tests {
		got := PrefixBytes(tt.bits)
		if got != tt.want {
			t.Errorf("PrefixBytes(%d) = %d, want %d", tt.bits, got, tt.want)
		}
	}
}

// TestWriteLabelStack verifies WriteLabelStack encodes labels with BOS.
//
// VALIDATES: Labels encoded per RFC 3032/8277: 20-bit label + TC=0 + S bit.
// PREVENTS: Incorrect label encoding, missing BOS bit on last label.
func TestWriteLabelStack(t *testing.T) {
	tests := []struct {
		name   string
		labels []uint32
		want   []byte
	}{
		{
			name:   "single label",
			labels: []uint32{100},
			want:   []byte{0x00, 0x06, 0x41}, // 100<<4 = 0x640, S=1
		},
		{
			name:   "two labels",
			labels: []uint32{100, 200},
			want: []byte{
				0x00, 0x06, 0x40, // 100, S=0
				0x00, 0x0c, 0x81, // 200, S=1
			},
		},
		{
			name:   "label 16 (RFC 3107 special)",
			labels: []uint32{16},
			want:   []byte{0x00, 0x01, 0x01}, // S=1
		},
		{
			name:   "large label",
			labels: []uint32{0xFFFFF}, // max 20-bit value
			want:   []byte{0xFF, 0xFF, 0xF1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, len(tt.labels)*3)
			n := WriteLabelStack(buf, 0, tt.labels)
			if n != len(tt.want) {
				t.Errorf("WriteLabelStack() wrote %d bytes, want %d", n, len(tt.want))
			}
			if !bytes.Equal(buf[:n], tt.want) {
				t.Errorf("WriteLabelStack() = %x, want %x", buf[:n], tt.want)
			}
		})
	}
}

// TestWriteLabelStackOffset verifies WriteLabelStack respects offset.
//
// VALIDATES: Labels written at correct buffer offset.
// PREVENTS: Buffer overwrite bugs.
func TestWriteLabelStackOffset(t *testing.T) {
	buf := make([]byte, 10)
	buf[0] = 0xFF // Should not be overwritten

	n := WriteLabelStack(buf, 1, []uint32{100})
	if n != 3 {
		t.Errorf("wrote %d bytes, want 3", n)
	}
	if buf[0] != 0xFF {
		t.Errorf("offset not respected: buf[0] = %x, want 0xFF", buf[0]) //nolint:gosec // G602: buf[0] valid, checked on line 100
	}
	want := []byte{0x00, 0x06, 0x41}
	if !bytes.Equal(buf[1:4], want) {
		t.Errorf("buf[1:4] = %x, want %x", buf[1:4], want)
	}
}
