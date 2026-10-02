// Design: docs/architecture/wire/qualifiers.md -- Which Writer to Call, WriteLabelValues
// RFC: rfc/short/rfc8277.md -- Sections 2.2 and 2.3, label entry layout on transmission
// Overview: update_build_labeled.go -- BuildLabeledUnicastNLRIBytes
// Related: update_build_vpn.go -- buildVPNNLRIBytes
// Related: multi_label_test.go -- the builders' multi-label round trips

package message

import (
	"encoding/hex"
	"net/netip"
	"testing"
)

// TestRFC8277BuilderLabelLayout pins the octets the UPDATE builders write for a
// label stack Ze originates, from operator config or a CLI argument. These two
// builders write labels with nlri.WriteLabelValues, the second producer beside
// LabeledUnicast.WriteTo, which the labeled plugin's tests cover.
//
// VALIDATES: RFC 8277 Section 2.2 for one label ([Length][Label|Rsrv|S][Prefix],
// Length = 24 + prefix bits, S = 1) and Section 2.3 for a stack (S = 0 on every
// entry but the last, S = 1 on the last), with Rsrv zero on every entry, through
// BuildLabeledUnicastNLRIBytes (with and without ADD-PATH) and buildVPNNLRIBytes.
// Each case compares the whole NLRI against literal octets.
// PREVENTS: a builder that drops the bottom-of-stack bit, sets it mid-stack,
// writes traffic-class bits Ze never chose, or miscounts the Length octet.
//
// RFC requirement: RFC8277-2-2 positive -- BuildLabeledUnicastNLRIBytes and buildVPNNLRIBytes encode a one-label binding in the Section 2.2 layout: 10.0.0.0/8 with label 100 is 20 000641 0a, with Path Identifier 7 first under ADD-PATH, and 60 000641 <RD> 0a for VPN.
// RFC requirement: RFC8277-2.2-1 positive -- the one label entry both builders write ends in 0x1: S is set on transmission.
// RFC requirement: RFC8277-2.3-1 positive -- in the stacks 100,200,300 and 100,200 both builders write S = 0 on every entry but the last (000640, 000c80).
// RFC requirement: RFC8277-2.3-2 positive -- in the same stacks the last entry both builders write has S = 1 (0012c1, 000c81).
// RFC requirement: RFC8277-2.2-3 positive -- every label entry the builders write for an originated stack carries Rsrv 000 on transmission.
func TestRFC8277BuilderLabelLayout(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	rd := [8]byte{0x00, 0x00, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x64}

	cases := []struct {
		name string
		got  func() []byte
		want string
	}{
		{"labeled one label", func() []byte {
			ub := NewUpdateBuilder(65001, false, true, false)
			return ub.BuildLabeledUnicastNLRIBytes(&LabeledUnicastParams{Prefix: prefix, Labels: []uint32{100}})
		}, "20" + "000641" + "0a"},
		{"labeled one label add-path", func() []byte {
			ub := NewUpdateBuilder(65001, false, true, true)
			return ub.BuildLabeledUnicastNLRIBytes(&LabeledUnicastParams{Prefix: prefix, PathID: 7, Labels: []uint32{100}})
		}, "00000007" + "20" + "000641" + "0a"},
		{"labeled three labels", func() []byte {
			ub := NewUpdateBuilder(65001, false, true, false)
			return ub.BuildLabeledUnicastNLRIBytes(&LabeledUnicastParams{Prefix: prefix, Labels: []uint32{100, 200, 300}})
		}, "50" + "000640" + "000c80" + "0012c1" + "0a"},
		{"vpn one label", func() []byte {
			ub := NewUpdateBuilder(65001, false, true, false)
			return ub.buildVPNNLRIBytes(&VPNParams{Prefix: prefix, Labels: []uint32{100}, RDBytes: rd})
		}, "60" + "000641" + "0000fde900000064" + "0a"},
		{"vpn two labels", func() []byte {
			ub := NewUpdateBuilder(65001, false, true, false)
			return ub.buildVPNNLRIBytes(&VPNParams{Prefix: prefix, Labels: []uint32{100, 200}, RDBytes: rd})
		}, "78" + "000640" + "000c81" + "0000fde900000064" + "0a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hex.EncodeToString(tc.got()); got != tc.want {
				t.Errorf("NLRI = %s, want %s", got, tc.want)
			}
		})
	}
}
