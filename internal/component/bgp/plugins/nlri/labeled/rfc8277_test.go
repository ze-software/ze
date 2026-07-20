// Design: docs/architecture/wire/nlri.md -- labeled unicast NLRI plugin
// RFC: rfc/short/rfc8277.md -- transmit-side NLRI encoding requirements
package labeled

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// ipv4Labeled builds a labeled unicast NLRI for the given prefix and stack.
func ipv4Labeled(prefix string, labels []uint32) []byte {
	return NewLabeledUnicast(
		Family{AFI: family.AFIIPv4, SAFI: SAFIMPLSLabel},
		netip.MustParsePrefix(prefix), labels, 0,
	).Bytes()
}

// TestLabeledSingleLabelSection22Encoding pins the RFC 8277 Section 2.2 wire
// layout that ze emits when a prefix carries one label: a single Length octet
// counting 24 label bits plus the prefix bits, one 3-octet label entry whose
// low nibble is Rsrv(3)+S(1), and the prefix rounded up to whole octets.
//
// VALIDATES: LabeledUnicast.WriteTo emits [Length][Label|Rsrv|S][Prefix] with
// Length = 24 + prefix bits and S set.
// PREVENTS: a peer misreading the prefix boundary because the Length octet
// counts octets, or omits the label bits.
//
// RFC requirement: RFC8277-2-2 positive -- a one-label binding uses the Section 2.2 encoding: Length = 24 + prefix bits, one 3-octet label entry, then the prefix.
// RFC requirement: RFC8277-2.2-1 positive -- the single label entry is transmitted with the S bit set to 1.
func TestLabeledSingleLabelSection22Encoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
		label  uint32
		want   []byte
	}{
		// label 100 -> 0x00 0x06 0x4|0001 ; Length = 24 + 8 = 32.
		{"10.0.0.0/8 label 100", "10.0.0.0/8", 100, []byte{32, 0x00, 0x06, 0x41, 10}},
		// label 16000 -> 0x03 0xE8 0x0|0001 ; Length = 24 + 24 = 48.
		{"192.168.1.0/24 label 16000", "192.168.1.0/24", 16000, []byte{48, 0x03, 0xE8, 0x01, 192, 168, 1}},
		// max 20-bit label -> 0xFF 0xFF 0xF|0001 ; Length = 24 + 32 = 56.
		{"10.1.2.3/32 max label", "10.1.2.3/32", 1048575, []byte{56, 0xFF, 0xFF, 0xF1, 10, 1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ipv4Labeled(tt.prefix, []uint32{tt.label})
			require.Equal(t, tt.want, got)

			prefixBits := netip.MustParsePrefix(tt.prefix).Bits()
			assert.Equal(t, byte(24+prefixBits), got[0],
				"Length octet counts 24 label bits plus the prefix bits")
			assert.Equal(t, byte(0x01), got[3]&0x01,
				"the S bit of the only label entry is 1 on transmission")
		})
	}
}

// TestLabeledLabelStackSBitPlacement pins the RFC 8277 Section 2.3 stack rule:
// every label entry except the last leaves S clear, and the last sets it, so a
// receiver reading entries until S=1 recovers exactly the stack that was sent
// and lands on the first prefix octet.
//
// VALIDATES: nlri.WriteLabelStack sets the bottom-of-stack bit only on the
// final entry, for stacks of one, two and three labels.
// PREVENTS: a mid-stack S bit truncating the stack at the receiver, or a clear
// final S bit making the receiver consume prefix octets as labels.
//
// RFC requirement: RFC8277-2.3-1 positive -- in a multi-label stack every entry except the last is transmitted with S = 0.
// RFC requirement: RFC8277-2.3-2 positive -- in a multi-label stack the last entry is transmitted with S = 1.
func TestLabeledLabelStackSBitPlacement(t *testing.T) {
	t.Parallel()

	for _, labels := range [][]uint32{{100}, {100, 200}, {100, 200, 300}} {
		got := ipv4Labeled("10.0.0.0/8", labels)

		require.Equal(t, byte(len(labels)*24+8), got[0],
			"Length octet counts 24 bits per label entry plus the prefix bits")
		require.Len(t, got, 1+len(labels)*3+1)

		for i := range labels {
			entry := got[1+i*3 : 1+i*3+3]
			value := uint32(entry[0])<<12 | uint32(entry[1])<<4 | uint32(entry[2])>>4
			assert.Equal(t, labels[i], value, "label %d value round-trips", i)

			wantS := byte(0)
			if i == len(labels)-1 {
				wantS = 1
			}
			assert.Equal(t, wantS, entry[2]&0x01,
				"stack of %d: S bit of entry %d", len(labels), i)
		}

		assert.Equal(t, byte(10), got[len(got)-1],
			"the prefix octets follow the entry whose S bit is set")
	}
}

// TestLabeledNLRILengthOctetOverflowGap documents the RFC 8277 Section 2.1
// limit as ze enforces it today. The NLRI Length field is a single octet, so a
// binding whose labels and prefix together exceed 255 bits cannot be encoded;
// the RFC forbids attempting it. ze computes the bit count in an int and
// narrows it to a byte without checking, so an over-long stack silently
// produces a Length octet that no longer describes the NLRI.
//
// VALIDATES: the exact observable behavior behind the RFC8277-2.1-10 gap.
// PREVENTS: the gap being closed silently, or being mis-recorded as closed.
func TestLabeledNLRILengthOctetOverflowGap(t *testing.T) {
	t.Parallel()

	// 6 labels (144 bits) plus a /128 prefix (128 bits) = 272 bits, which does
	// not fit the one-octet Length field. 272 mod 256 = 16.
	lu := NewLabeledUnicast(
		Family{AFI: family.AFIIPv6, SAFI: SAFIMPLSLabel},
		netip.MustParsePrefix("2001:db8::1/128"),
		[]uint32{1, 2, 3, 4, 5, 6}, 0,
	)
	got := lu.Bytes()

	assert.Equal(t, byte(16), got[0],
		"gap RFC8277-2.1-10: 272 bits are truncated to a Length octet of 16 instead of being refused")
	assert.Len(t, got, 1+6*3+16,
		"the encoder still writes the full stack and prefix behind the truncated Length")
}

// TestLabeledMultiLabelWithoutCapabilityGap documents the RFC 8277 Section 2
// rule as ze applies it today. The Multiple Labels Capability (code 8) is
// neither sent nor recognized by ze, so every session is in the single-label
// mode of Section 2, where a prefix MUST be bound to exactly one label. The
// operator-facing encoder nevertheless accepts and emits a multi-label stack.
//
// VALIDATES: the exact observable behavior behind the RFC8277-2-1, RFC8277-2-3
// and RFC8277-3.2.2-2 gaps.
// PREVENTS: the gaps being closed silently, or being mis-recorded as closed.
func TestLabeledMultiLabelWithoutCapabilityGap(t *testing.T) {
	t.Parallel()

	hex, err := EncodeNLRIHex("ipv4/mpls-label", []string{
		"prefix", "10.0.0.0/8",
		"label", "100",
		"label", "200",
	})
	require.NoError(t, err,
		"gap RFC8277-2-1: a two-label binding is accepted with no capability negotiated")
	assert.Equal(t, "38000640000C810A", hex,
		"gap RFC8277-2-3: both label entries are emitted (Length 0x38 = 24+24+8 bits)")
}
