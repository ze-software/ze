// Design: docs/architecture/wire/nlri.md -- native VPN withdrawal decoding
// RFC naming: untagged -- supplementary CLI consumer regression; it does not claim independent whole-clause coverage.

package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn" // Register the real family decoder.
)

// TestVPNWithdrawalCLIDecoderIgnoresCompatibility drives the same full UPDATE
// decoder used by ze bgp decode; there is no session and therefore no ADD-PATH.
//
// VALIDATES: adjacent VPN withdrawals keep both RDs and ignore every Compatibility
// value, including S-clear values that do not form an announcement label stack.
// PREVENTS: parseNLRIByFamily discarding the action known from MP_UNREACH_NLRI.
func TestVPNWithdrawalCLIDecoderIgnoresCompatibility(t *testing.T) {
	for _, familyCase := range []struct {
		name      string
		afi       byte
		prefixHex string
		prefix    string
		bits      byte
	}{
		{"ipv4/mpls-vpn", 1, "0a", "10.0.0.0/8", 96},
		{"ipv6/mpls-vpn", 2, "20010db8", "2001:db8::/32", 120},
	} {
		for _, compatibility := range []string{"800000", "000000", "000641", "123450", "ffffff"} {
			t.Run(familyCase.name+"/compat="+compatibility, func(t *testing.T) {
				sectionHex := fmt.Sprintf("%02x%s0000fde800000001%s%02x%s0000fde800000002%s", familyCase.bits, compatibility, familyCase.prefixHex, familyCase.bits, compatibility, familyCase.prefixHex)
				section, err := hex.DecodeString(sectionHex)
				require.NoError(t, err)
				mp := append([]byte{0, familyCase.afi, 128}, section...)
				attrs := append([]byte{0x80, 15, byte(len(mp))}, mp...)
				body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
				encoded, err := decodeHexPacket(hex.EncodeToString(body), msgTypeUpdate, "", true)
				require.NoError(t, err)
				var event struct {
					BGP struct {
						Update map[string]any `json:"update"`
					} `json:"bgp"`
				}
				require.NoError(t, json.Unmarshal([]byte(encoded), &event))
				want := []any{map[string]any{"action": "del", "nlri": []any{
					map[string]any{"rd": "0:65000:1", "prefix": familyCase.prefix},
					map[string]any{"rd": "0:65000:2", "prefix": familyCase.prefix},
				}}}
				require.Equal(t, want, event.BGP.Update[familyCase.name], "%s", encoded)
			})
		}
	}
}
