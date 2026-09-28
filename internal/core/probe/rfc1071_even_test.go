// Design: docs/architecture/diagnostics/active-probes.md -- the ICMP echo checksum a probe sends

package probe

import "testing"

// TestRFC1071ChecksumEvenLength pins the even-count form of the RFC 1071 sum.
//
// Goal: an even-length buffer is summed as the big-endian 16-bit words [A,B],
// [C,D] and so on, with no pad octet added. Method: exact vectors computed by
// hand. [12 34 56 78] sums to 0x68ac, so the checksum is 0x9753. Summing the
// words little-endian gives 0x5397, and adding a pad of the last octet gives
// 0x1f53, so either break fails here. The six-octet vector sums to 0x10368,
// which folds to 0x0369 and gives 0xfc96.
//
// RFC requirement: RFC1071-1-4 positive -- an even-length buffer is summed as big-endian words [a,b] = a*256+b with no pad octet: icmpChecksum answers exactly 0x9753 for [12 34 56 78] and 0xfc96 for [12 34 56 78 9a bc].
func TestRFC1071ChecksumEvenLength(t *testing.T) {
	cases := []struct {
		data []byte
		want uint16
	}{
		{[]byte{0x12, 0x34, 0x56, 0x78}, 0x9753},
		{[]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc}, 0xfc96},
	}
	for _, c := range cases {
		if got := icmpChecksum(c.data); got != c.want {
			t.Errorf("icmpChecksum(% x) = %#04x, want %#04x", c.data, got, c.want)
		}
	}
}
