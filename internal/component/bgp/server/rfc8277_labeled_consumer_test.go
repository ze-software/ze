// Design: docs/architecture/wire/nlri.md -- labeled withdrawal consumer semantics

package server

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/labeled" // Register the real family decoder.
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestLabeledWithdrawalCodecConsumersIgnoreCompatibility exercises registered
// engine consumers, not a callback that merely echoes their metadata.
//
// VALIDATES: both labeled-unicast AFIs ignore all Compatibility bits, keep every
// adjacent prefix and negotiated identifier (including zero), and publish no labels.
// PREVENTS: S-clear values failing decode, S-set values becoming labels, or the
// section decoder silently losing every withdrawal after the first one.
//
// RFC requirement: RFC8277-2.4-1 positive -- registered labeled-unicast withdrawal codecs ignore Compatibility 0x800000 for both AFIs and retain each prefix and negotiated Path Identifier without labels.
// RFC requirement: RFC8277-2.4-1 negative -- zero, S-set and other Compatibility values are ignored, not rejected or published as labels, for single and adjacent labeled withdrawals.
func TestLabeledWithdrawalCodecConsumersIgnoreCompatibility(t *testing.T) {
	for _, afi := range []family.AFI{family.AFIIPv4, family.AFIIPv6} {
		fam := family.Family{AFI: afi, SAFI: family.SAFIMPLSLabel}
		for _, addPath := range []bool{false, true} {
			for _, count := range []int{1, 2} {
				for _, compatibility := range [][3]byte{{0x80, 0, 0}, {}, {0, 6, 0x41}, {0x12, 0x34, 0x50}, {0xff, 0xff, 0xff}} {
					var section []byte
					var want []any
					for i := range count {
						pathID := byte(i * 17)
						if addPath {
							section = append(section, 0, 0, 0, pathID)
						}
						prefix := []byte{byte(10 + i)}
						prefixText := fmt.Sprintf("%d.0.0.0/8", 10+i)
						if afi == family.AFIIPv6 {
							prefix = []byte{0x20, 1, 0x0d, byte(0xb8 + i)}
							prefixText = fmt.Sprintf("2001:db%x::/32", 8+i)
						}
						section = append(section, byte(24+8*len(prefix)), compatibility[0], compatibility[1], compatibility[2])
						section = append(section, prefix...)
						route := map[string]any{"prefix": prefixText}
						if addPath {
							route["path-id"] = float64(pathID)
						}
						want = append(want, route)
					}
					for _, method := range []string{"decode-mp-unreach", "decode-nlri"} {
						t.Run(fmt.Sprintf("%s/%s/addpath=%t/count=%d/compat=%x", method, fam, addPath, count, compatibility), func(t *testing.T) {
							input := map[string]any{"add-path": addPath}
							if method == "decode-mp-unreach" {
								mp := append([]byte{byte(afi >> 8), byte(afi), byte(family.SAFIMPLSLabel)}, section...)
								input["hex"] = hex.EncodeToString(mp)
							} else {
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
							default:
								t.Fatalf("unexpected output type %T", result)
							}
							var got any
							require.NoError(t, json.Unmarshal(raw, &got))
							if count == 1 {
								require.Equal(t, want[0], got)
							} else {
								require.Equal(t, want, got)
							}
						})
					}
				}
			}
		}
	}
}
