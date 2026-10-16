// Design: docs/architecture/wire/nlri.md -- labeled withdrawal on send
// Overview: rfc7606_withdraw.go -- mpReachToUnreach

package message

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSynthesizeWithdrawLabeledOneCompatibilityField checks a labeled
// announcement turned into a withdrawal carries one three-octet Compatibility
// field per route, however many labels the announcement bound. Method: build an
// MP_REACH_NLRI with a two-label route (and a VPN route with three labels, and
// an ADD-PATH route), synthesize the withdrawal, compare the MP_UNREACH value.
//
// RFC 8277 Section 2.4: "This encoding is used whether or not the Multiple
// Labels Capability has been sent or received on the session." The static
// route withdrawal and RFC 7606 treat-as-withdraw both build here, so before
// the fix the second label reached the peer as prefix bits.
func TestSynthesizeWithdrawLabeledOneCompatibilityField(t *testing.T) {
	tests := []struct {
		name    string
		addPath bool
		reach   string // AFI SAFI NHLen NH Reserved NLRI
		want    string // AFI SAFI NLRI
	}{
		{
			name:  "ipv4/mpls-label two labels",
			reach: "000104" + "04" + "0A000001" + "00" + "38" + "000010" + "003E81" + "0A",
			want:  "000104" + "20" + "800000" + "0A",
		},
		{
			name:  "ipv4/mpls-vpn three labels",
			reach: "000180" + "0C" + "0000000000000000" + "0A000001" + "00" + "98" + "000010" + "000020" + "003E81" + "0000FDE800000001" + "0104",
			want:  "000180" + "68" + "800000" + "0000FDE800000001" + "0104",
		},
		{
			name:    "ipv4/mpls-label ADD-PATH two labels",
			addPath: true,
			reach:   "000104" + "04" + "0A000001" + "00" + "00000007" + "38" + "000010" + "003E81" + "0A",
			want:    "000104" + "00000007" + "20" + "800000" + "0A",
		},
		{
			// Unicast carries no labels: the NLRI is copied as it was.
			name:  "ipv4/unicast untouched",
			reach: "000101" + "04" + "0A000001" + "00" + "18" + "0A0101",
			want:  "000101" + "18" + "0A0101",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reach, err := hex.DecodeString(tt.reach)
			require.NoError(t, err)
			want, err := hex.DecodeString(tt.want)
			require.NoError(t, err)
			attrs := append([]byte{0x80, 0x0E, byte(len(reach))}, reach...)

			out, changed := SynthesizeWithdraw(buildBody(nil, attrs, nil), tt.addPath)
			require.True(t, changed)
			_, newAttrs, _ := splitBody(t, out)
			it := newAttrIter(newAttrs)
			code, _, value, ok := it.Next()
			require.True(t, ok)
			assert.Equal(t, uint8(15), uint8(code))
			assert.Equal(t, hex.EncodeToString(want), hex.EncodeToString(value))
		})
	}
}

// TestSynthesizeWithdrawLabeledMalformedSkipped checks a labeled MP_REACH whose
// label stack runs past its NLRI field yields no withdrawal for it, the same
// answer mpReachToUnreach gives any MP_REACH it cannot frame.
func TestSynthesizeWithdrawLabeledMalformedSkipped(t *testing.T) {
	reach, err := hex.DecodeString("000104" + "04" + "0A000001" + "00" + "38" + "000010")
	require.NoError(t, err)
	attrs := append([]byte{0x80, 0x0E, byte(len(reach))}, reach...)
	_, changed := SynthesizeWithdraw(buildBody(nil, attrs, nil), false)
	assert.False(t, changed)
}
