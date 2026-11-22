package nlri

import (
	"bytes"
	"testing"
)

// TestWriteLabeledNLRI verifies WriteLabeledNLRI builds the labeled unicast
// NLRI of RFC 8277 Sections 2.2 and 2.3 from a prefix and a label stack.
//
// VALIDATES: Length counts 24 bits per label plus the prefix bits, the label
// entries follow in stack order with S on the last alone, the prefix octets
// close the NLRI, and the octets before off stay the caller's.
// PREVENTS: a Length that counts the prefix alone, a stack cut to one label,
// and a write that starts at the buffer's first octet whatever off says.
func TestWriteLabeledNLRI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		off        int
		prefixNLRI []byte
		labels     []uint32
		want       []byte
	}{
		{
			name:       "one label, IPv4 /24",
			prefixNLRI: []byte{24, 203, 0, 113},
			labels:     []uint32{100},
			want:       []byte{0x30, 0x00, 0x06, 0x41, 203, 0, 113},
		},
		{
			name:       "two labels, IPv4 /24",
			prefixNLRI: []byte{24, 203, 0, 113},
			labels:     []uint32{100, 200},
			want:       []byte{0x48, 0x00, 0x06, 0x40, 0x00, 0x0c, 0x81, 203, 0, 113},
		},
		{
			name:       "one label, default route",
			prefixNLRI: []byte{0},
			labels:     []uint32{100},
			want:       []byte{0x18, 0x00, 0x06, 0x41},
		},
		{
			name:       "one label, IPv6 /48, behind a path identifier",
			off:        4,
			prefixNLRI: []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01},
			labels:     []uint32{100},
			want:       []byte{0xaa, 0xaa, 0xaa, 0xaa, 0x48, 0x00, 0x06, 0x41, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01},
		},
		{
			name:       "trailing octets past the prefix length are not copied",
			prefixNLRI: []byte{24, 203, 0, 113, 0xff},
			labels:     []uint32{100},
			want:       []byte{0x30, 0x00, 0x06, 0x41, 203, 0, 113},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := bytes.Repeat([]byte{0xaa}, tt.off+LabeledNLRIOctetsMax)
			octets, ok := WriteLabeledNLRI(buf, tt.off, tt.prefixNLRI, tt.labels)
			if !ok {
				t.Fatalf("WriteLabeledNLRI refused a labeled NLRI it can write")
			}
			if octets != len(tt.want)-tt.off {
				t.Errorf("octets written = %d, want %d", octets, len(tt.want)-tt.off)
			}
			if got := buf[:tt.off+octets]; !bytes.Equal(got, tt.want) {
				t.Errorf("NLRI = %x, want %x", got, tt.want)
			}
		})
	}
}

// TestWriteLabeledNLRIRefuses verifies WriteLabeledNLRI answers false and
// writes nothing for input that is no labeled NLRI it can write whole.
//
// VALIDATES: an empty stack, an empty or truncated prefix NLRI, a Length past
// the 255 bits one octet counts, and a short buffer are each refused with the
// buffer untouched.
// PREVENTS: a Length octet that wraps past 255, a labeled NLRI with no label,
// and a partial write a caller could send.
func TestWriteLabeledNLRIRefuses(t *testing.T) {
	t.Parallel()
	elevenLabels := make([]uint32, LabeledNLRILabelsMax+1)
	tests := []struct {
		name       string
		bufOctets  int
		prefixNLRI []byte
		labels     []uint32
	}{
		{name: "no label", bufOctets: LabeledNLRIOctetsMax, prefixNLRI: []byte{24, 203, 0, 113}},
		{name: "no prefix NLRI", bufOctets: LabeledNLRIOctetsMax, labels: []uint32{100}},
		{name: "prefix shorter than its length", bufOctets: LabeledNLRIOctetsMax, prefixNLRI: []byte{24, 203, 0}, labels: []uint32{100}},
		{name: "more labels than Length can count", bufOctets: 2 * LabeledNLRIOctetsMax, prefixNLRI: []byte{0}, labels: elevenLabels},
		{name: "Length past 255 bits", bufOctets: 2 * LabeledNLRIOctetsMax, prefixNLRI: append([]byte{128}, make([]byte, 16)...), labels: []uint32{1, 2, 3, 4, 5, 6}},
		{name: "buffer one octet short", bufOctets: 6, prefixNLRI: []byte{24, 203, 0, 113}, labels: []uint32{100}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := bytes.Repeat([]byte{0xaa}, tt.bufOctets)
			octets, ok := WriteLabeledNLRI(buf, 0, tt.prefixNLRI, tt.labels)
			if ok {
				t.Fatalf("WriteLabeledNLRI wrote %d octets, want a refusal", octets)
			}
			if octets != 0 {
				t.Errorf("octets = %d on a refusal, want 0", octets)
			}
			if !bytes.Equal(buf, bytes.Repeat([]byte{0xaa}, tt.bufOctets)) {
				t.Errorf("buffer changed on a refusal: %x", buf)
			}
		})
	}
}

// TestLabeledNLRIBounds pins the two bounds a caller sizes its buffers with.
//
// VALIDATES: the deepest stack LabeledNLRILabelsMax names is written whole
// into a buffer of LabeledNLRIOctetsMax octets, with Length 240; the refusal
// test above shows one label more does not fit.
// PREVENTS: a stack buffer sized below what a received NLRI can carry.
func TestLabeledNLRIBounds(t *testing.T) {
	t.Parallel()
	buf := make([]byte, LabeledNLRIOctetsMax)
	octets, ok := WriteLabeledNLRI(buf, 0, []byte{0}, make([]uint32, LabeledNLRILabelsMax))
	if !ok {
		t.Fatalf("WriteLabeledNLRI refused a %d-label stack, the deepest Length can count", LabeledNLRILabelsMax)
	}
	if octets != 1+3*LabeledNLRILabelsMax {
		t.Errorf("octets = %d, want %d", octets, 1+3*LabeledNLRILabelsMax)
	}
	if buf[0] != 240 {
		t.Errorf("Length = %d, want 240", buf[0])
	}
}
