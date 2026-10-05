// Design: docs/architecture/wire/nlri.md -- native VPN withdrawal decoding

package server

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn" // Register the real family decoder.
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestVPNWithdrawalCodecConsumersIgnoreCompatibility reaches all registered RPC
// consumers of native VPN withdrawal bytes with adjacent distinct route identities.
//
// VALIDATES: MP_UNREACH, full UPDATE, and explicit withdrawal NLRI decode agree on
// RD/prefix/Path Identifier without publishing Compatibility as a label.
// PREVENTS: action metadata being dropped at one of the registry decoder boundaries.
//
// RFC requirement: RFC8277-2.4-1 positive -- registered MP_UNREACH, UPDATE and explicit withdrawal NLRI codecs ignore VPN Compatibility 0x800000 and publish all RD/prefix/Path Identifier identities without labels.
// RFC requirement: RFC8277-2.4-1 negative -- zero, S-set and other Compatibility values do not reject VPN withdrawals or change their identities into label-bearing announcements.
func TestVPNWithdrawalCodecConsumersIgnoreCompatibility(t *testing.T) {
	for _, afi := range []family.AFI{family.AFIIPv4, family.AFIIPv6} {
		fam := family.Family{AFI: afi, SAFI: family.SAFIVPN}
		for _, addPath := range []bool{false, true} {
			for _, compatibility := range [][3]byte{{0x80, 0, 0}, {}, {0, 6, 0x41}, {0x12, 0x34, 0x50}, {0xff, 0xff, 0xff}} {
				section, want := vpnCodecSection(afi, addPath, compatibility)
				mp := append([]byte{byte(afi >> 8), byte(afi), byte(family.SAFIVPN)}, section...)
				attrs := append([]byte{0x80, 15, byte(len(mp))}, mp...)
				body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
				for _, method := range []string{"decode-mp-unreach", "decode-update", "decode-nlri"} {
					t.Run(fmt.Sprintf("%s/%s/addpath=%t/compat=%x", method, fam, addPath, compatibility), func(t *testing.T) {
						input := map[string]any{"add-path": addPath}
						switch method {
						case "decode-mp-unreach":
							input["hex"] = hex.EncodeToString(mp)
						case "decode-update":
							input["hex"] = hex.EncodeToString(body)
						case "decode-nlri":
							input["family"] = fam.String()
							input["hex"] = hex.EncodeToString(section)
							input["withdraw"] = true
						}
						params, err := json.Marshal(input)
						require.NoError(t, err)
						handler := registry.CollectRPCHandlers()["ze-plugin-engine:"+method]
						require.NotNil(t, handler)
						result, err := handler(params)
						require.NoError(t, err)
						var raw json.RawMessage
						switch out := result.(type) {
						case *rpc.DecodeMPUnreachOutput:
							require.Equal(t, fam.String(), out.Family)
							raw = out.NLRI
						case *rpc.DecodeNLRIOutput:
							raw = out.JSON
						case *rpc.DecodeUpdateOutput:
							var decoded struct {
								Update struct {
									NLRI map[string][]struct {
										Action string          `json:"action"`
										NLRI   json.RawMessage `json:"nlri"`
									} `json:"nlri"`
								} `json:"update"`
							}
							require.NoError(t, json.Unmarshal([]byte(out.JSON), &decoded))
							operations := decoded.Update.NLRI[fam.String()]
							require.Len(t, operations, 1, "%s", out.JSON)
							require.Equal(t, "del", operations[0].Action)
							raw = operations[0].NLRI
						default:
							t.Fatalf("unexpected output type %T", result)
						}
						var routes []any
						require.NoError(t, json.Unmarshal(raw, &routes), "%s", raw)
						require.Equal(t, want, routes)
					})
				}
			}
		}
	}
}

// TestVPNAnnouncementCodecRetainsLabels holds the announcement polarity through
// the same real decoder, including negotiated Path Identifier zero.
//
// VALIDATES: MP_REACH still publishes each route's MPLS label stack.
// PREVENTS: implementing withdrawal support by suppressing labels for all VPN NLRI.
func TestVPNAnnouncementCodecRetainsLabels(t *testing.T) {
	for _, afi := range []family.AFI{family.AFIIPv4, family.AFIIPv6} {
		for _, addPath := range []bool{false, true} {
			t.Run(fmt.Sprintf("afi=%d/addpath=%t", afi, addPath), func(t *testing.T) {
				section, want := vpnCodecSection(afi, addPath, [3]byte{0, 6, 0x41})
				for _, route := range want {
					decoded, ok := route.(map[string]any)
					require.True(t, ok)
					decoded["labels"] = []any{[]any{float64(100), float64(1601)}}
				}
				nextHop := make([]byte, 12)
				nextHop[8], nextHop[9], nextHop[10], nextHop[11] = 192, 0, 2, 1
				if afi == family.AFIIPv6 {
					nextHop = make([]byte, 24)
					nextHop[8], nextHop[9], nextHop[10], nextHop[11], nextHop[23] = 0x20, 1, 0x0d, 0xb8, 1
				}
				mp := append([]byte{byte(afi >> 8), byte(afi), byte(family.SAFIVPN), byte(len(nextHop))}, nextHop...)
				mp = append(mp, 0)
				mp = append(mp, section...)
				params, err := json.Marshal(rpc.DecodeMPReachInput{Hex: hex.EncodeToString(mp), AddPath: addPath})
				require.NoError(t, err)
				result, err := registry.CollectRPCHandlers()["ze-plugin-engine:decode-mp-reach"](params)
				require.NoError(t, err)
				out, ok := result.(*rpc.DecodeMPReachOutput)
				require.True(t, ok)
				var routes []any
				require.NoError(t, json.Unmarshal(out.NLRI, &routes))
				require.Equal(t, want, routes)
			})
		}
	}
}

func vpnCodecSection(afi family.AFI, addPath bool, compatibility [3]byte) ([]byte, []any) {
	var section []byte
	var want []any
	for i := range 3 {
		rd, pathID := byte(1), byte(0)
		if i == 1 {
			rd = 2
		}
		if i == 2 {
			pathID = 17
		}
		if addPath {
			section = append(section, 0, 0, 0, pathID)
		}
		prefix, prefixText := []byte{10}, "10.0.0.0/8"
		if i == 2 {
			prefix, prefixText = []byte{11}, "11.0.0.0/8"
		}
		if afi == family.AFIIPv6 {
			prefix, prefixText = []byte{0x20, 1, 0x0d, 0xb8}, "2001:db8::/32"
			if i == 2 {
				prefix[3], prefixText = 0xb9, "2001:db9::/32"
			}
		}
		section = append(section, byte(24+64+8*len(prefix)), compatibility[0], compatibility[1], compatibility[2],
			0, 0, 0xfd, 0xe8, 0, 0, 0, rd)
		section = append(section, prefix...)
		route := map[string]any{"rd": fmt.Sprintf("0:65000:%d", rd), "prefix": prefixText}
		if addPath {
			route["path-id"] = float64(pathID)
		}
		want = append(want, route)
	}
	return section, want
}
