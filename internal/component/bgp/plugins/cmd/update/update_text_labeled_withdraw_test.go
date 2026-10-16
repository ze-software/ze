package update

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// TestTextWithdrawalOneCompatibilityField checks a labeled `del` naming a label
// stack builds a withdrawn NLRI with one three-octet field where the stack was,
// so the receiver frames the prefix at the right offset. Method: parse a
// two-label withdrawal for SAFI 4 and SAFI 128, write the withdrawn NLRI, and
// compare it octet for octet with the expected wire.
//
// RFC 8277 Section 2.4 gives a withdrawn NLRI one Compatibility field, "used
// whether or not the Multiple Labels Capability has been sent or received on
// the session"; before the fix each label was written, and the second reached
// the peer as 24 bits of prefix. The session writer then sets the field to
// 0x800000 (reactor setWithdrawalCompatibility).
func TestTextWithdrawalOneCompatibilityField(t *testing.T) {
	tests := []struct {
		name string
		cmd  []string
		want string
	}{
		{
			name: "ipv4/mpls-label",
			cmd:  []string{"nlri", "ipv4/mpls-label", "label", "[", "100", "200", "]", "del", "10.0.0.0/24"},
			want: "30" + "000641" + "0a0000",
		},
		{
			name: "ipv4/mpls-vpn",
			cmd:  []string{"nlri", "ipv4/mpls-vpn", "rd", "65000:1", "label", "[", "100", "200", "]", "del", "1.4.0.0/16"},
			want: "68" + "000641" + "0000fde800000001" + "0104",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseUpdateText(tt.cmd)
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Withdraw, 1)
			n := result.Groups[0].Withdraw[0]
			buf := make([]byte, nlri.LenWithContext(n, false))
			nlri.WriteNLRI(n, buf, 0, false)
			assert.Equal(t, tt.want, hex.EncodeToString(buf), "withdrawn NLRI")
		})
	}
}
