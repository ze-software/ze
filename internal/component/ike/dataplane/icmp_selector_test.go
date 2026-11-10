// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: icmp_selector.go -- MaskedBlocks; icmp_selector_vpp_test.go -- the VPP refusal
// RFC: rfc/short/rfc4301.md -- the ICMP type and code selector (Section 4.4.1.1)
package dataplane

import "testing"

// VALIDATES: MaskedBlocks covers every 8-bit range exactly, with disjoint aligned
// blocks, and the two 16-bit extremes.
// PREVENTS: a block that reaches past the range end (widening) or a gap inside it.
func TestMaskedBlocksAreExact(t *testing.T) {
	for start := range 256 {
		for end := start; end <= 255; end++ {
			blocks := MaskedBlocks(uint16(start), uint16(end))
			for v := range 512 {
				hits := 0
				for _, b := range blocks {
					if (uint16(v)^b.Port)&b.Mask == 0 {
						hits++
					}
				}
				want := 0
				if v >= start && v <= end {
					want = 1
				}
				if hits != want {
					t.Fatalf("range %d-%d: value %d matched %d blocks, want %d (%+v)", start, end, v, hits, want, blocks)
				}
			}
		}
	}
	if got := MaskedBlocks(0, 0xffff); len(got) != 1 || !got[0].IsAny() {
		t.Fatalf("whole 16-bit range = %+v, want one any match", got)
	}
	if got := MaskedBlocks(0, 0); len(got) != 1 || got[0] != ExactPortMatch(0) {
		t.Fatalf("range 0-0 = %+v, want exactly 0", got)
	}
	if got := MaskedBlocks(5, 4); len(got) != 0 {
		t.Fatalf("empty range = %+v, want no block", got)
	}
}
