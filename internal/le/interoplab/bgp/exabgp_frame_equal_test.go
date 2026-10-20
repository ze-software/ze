//go:build ze_bgp

package bgp

import (
	"encoding/hex"
	"testing"
)

// TestBGPFrameEqualReadsCommunitiesAsASet proves the comparator treats the
// members of COMMUNITIES and EXTENDED COMMUNITIES as a set, and nothing more.
// Method: the frames are the ones the old-4.2 conf-new-v4 and conf-vpn cases
// expect next to the ones ze sent; each reordered pair must compare equal, and a
// changed member, a changed attribute elsewhere, or a reordered AS_PATH must not.
func TestBGPFrameEqualReadsCommunitiesAsASet(t *testing.T) {
	const marker = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
	cases := []struct {
		name   string
		wanted string
		actual string
		equal  bool
	}{
		{
			name:   "communities in another order",
			wanted: marker + "003B0200000020400101004002004003040A00FFFE40050400000064C008087814781478140000180A0003",
			actual: marker + "003B0200000020400101004002004003040A00FFFE40050400000064C008087814000078147814180A0003",
			equal:  true,
		},
		{
			name:   "a different community",
			wanted: marker + "003B0200000020400101004002004003040A00FFFE40050400000064C008087814781478140000180A0003",
			actual: marker + "003B0200000020400101004002004003040A00FFFE40050400000064C008087814000178147814180A0003",
			equal:  false,
		},
		{
			name:   "extended communities in another order",
			wanted: marker + "003302000000214001010040020040050400000064C010100002FDE8000000010002271000000001",
			actual: marker + "003302000000214001010040020040050400000064C010100002271000000001" + "0002FDE800000001",
			equal:  true,
		},
		{
			name:   "an extended community changed",
			wanted: marker + "003302000000214001010040020040050400000064C010100002FDE8000000010002271000000001",
			actual: marker + "003302000000214001010040020040050400000064C010100002271000000002" + "0002FDE800000001",
			equal:  false,
		},
		{
			name:   "an AS_PATH whose members moved",
			wanted: marker + "002B0200000014400101004002060201000100024005040000006418" + "0A0001",
			actual: marker + "002B0200000014400101004002060201000200014005040000006418" + "0A0001",
			equal:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wanted, err := hex.DecodeString(tc.wanted)
			if err != nil {
				t.Fatalf("wanted: %v", err)
			}
			actual, err := hex.DecodeString(tc.actual)
			if err != nil {
				t.Fatalf("actual: %v", err)
			}
			if got := bgpFrameEqual(actual, wanted); got != tc.equal {
				t.Fatalf("bgpFrameEqual = %v, want %v", got, tc.equal)
			}
		})
	}
}
