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
	t.Parallel()
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

// TestWriteLabelValues verifies WriteLabelValues encodes bare labels with BOS.
//
// VALIDATES: Labels a speaker ORIGINATES are encoded per RFC 3032 Section 2.1:
// 20-bit label + TC=0 + S on the last entry.
// PREVENTS: Incorrect label encoding, missing BOS bit on last label.
func TestWriteLabelValues(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			buf := make([]byte, len(tt.labels)*3)
			n := WriteLabelValues(buf, 0, tt.labels)
			if n != len(tt.want) {
				t.Errorf("WriteLabelValues() wrote %d bytes, want %d", n, len(tt.want))
			}
			if !bytes.Equal(buf[:n], tt.want) {
				t.Errorf("WriteLabelValues() = %x, want %x", buf[:n], tt.want)
			}
		})
	}
}

// TestWriteLabelValuesOffset verifies WriteLabelValues respects offset.
//
// VALIDATES: Labels written at correct buffer offset.
// PREVENTS: Buffer overwrite bugs.
func TestWriteLabelValuesOffset(t *testing.T) {
	t.Parallel()
	buf := make([]byte, 10)
	buf[0] = 0xFF // Should not be overwritten

	n := WriteLabelValues(buf, 1, []uint32{100})
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

// TestWriteLabelStackKeepsTheTrafficClass pins the half WriteLabelValues cannot
// state: an entry a peer SENT, relayed back out unchanged.
//
// VALIDATES: WriteLabelStack writes the whole 3-octet entry, so the traffic
// class survives a parse and a re-encode, and the bottom-of-stack bit is set on
// the last entry and cleared on the rest whatever the caller passed.
// PREVENTS: the relay this repository shipped until 2026-09-20, which read the
// 20-bit label, dropped RFC 3032 Section 2.1's TC field, and published a
// traffic class of zero for one the peer had set.
func TestWriteLabelStackKeepsTheTrafficClass(t *testing.T) {
	t.Parallel()
	// Two entries as they sit on the wire: label 100 with TC 5 and no S bit,
	// then label 200 with TC 0 and S set.
	wire := []byte{0x00, 0x06, 0x4A, 0x00, 0x0C, 0x81}

	entries, remaining, err := ParseLabelStack(wire)
	if err != nil {
		t.Fatalf("ParseLabelStack() error = %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("remaining = %x, want empty", remaining)
	}
	if len(entries) != 2 {
		t.Fatalf("parsed %d entries, want 2", len(entries))
	}
	if got := LabelValue(entries[0]); got != 100 {
		t.Errorf("LabelValue(entries[0]) = %d, want 100", got)
	}

	buf := make([]byte, len(entries)*3)
	WriteLabelStack(buf, 0, entries)
	if !bytes.Equal(buf, wire) {
		t.Errorf("relayed stack = %x, want %x: the traffic class did not survive", buf, wire)
	}
}
