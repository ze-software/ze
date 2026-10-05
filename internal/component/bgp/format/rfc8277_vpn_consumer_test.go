// Design: docs/architecture/wire/nlri.md -- native VPN withdrawal decoding

package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgpfilter "github.com/ze-software/ze/internal/component/bgp/filter"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn" // Register the real family decoder.
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
)

// TestVPNWithdrawalJSONConsumersIgnoreCompatibility exercises the received-event
// formatter with real registered VPN decoders, not a substitute family callback.
//
// VALIDATES: every Compatibility value produces the same RD/prefix identities,
// explicit ADD-PATH zero survives, and native bytes are not rewritten by rendering.
// PREVENTS: S-clear Compatibility producing unparsed output, S-set Compatibility
// becoming a label, or a following route being consumed as part of a label stack.
//
// RFC requirement: RFC8277-2.4-1 positive -- direct and filtered VPNv4/VPNv6 withdrawal events ignore Compatibility 0x800000 while preserving each RD, prefix and negotiated Path Identifier without labels.
// RFC requirement: RFC8277-2.4-1 negative -- zero, S-set and other Compatibility values produce the same withdrawal identities rather than rejection, label output or consumption of adjacent routes.
func TestVPNWithdrawalJSONConsumersIgnoreCompatibility(t *testing.T) {
	for _, afi := range []family.AFI{family.AFIIPv4, family.AFIIPv6} {
		fam := family.Family{AFI: afi, SAFI: family.SAFIVPN}
		for _, addPath := range []bool{false, true} {
			for _, compatibility := range [][3]byte{{0x80, 0, 0}, {}, {0, 6, 0x41}, {0x12, 0x34, 0x50}, {0xff, 0xff, 0xff}} {
				t.Run(fmt.Sprintf("%s/addpath=%t/compat=%x", fam, addPath, compatibility), func(t *testing.T) {
					section, want := vpnConsumerSection(afi, addPath, compatibility)
					value := append([]byte{byte(afi >> 8), byte(afi), byte(family.SAFIVPN)}, section...)
					attrs := append([]byte{0x80, 15, byte(len(value))}, value...)
					body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
					original := bytes.Clone(body)
					ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: addPath})
					ctxID, err := bgpctx.Registry.Register(ctx)
					require.NoError(t, err)
					update := wireu.NewWireUpdate(body, ctxID)
					attrWire, err := update.Attrs()
					require.NoError(t, err)
					message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: body, AttrsWire: attrWire, WireUpdate: update}
					peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001}
					content := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatParsed}
					encoded := AppendMessage(nil, &peer, message, content)
					var event map[string]any
					require.NoError(t, json.Unmarshal(encoded, &event))
					operations := getNLRI(t, getEventPayload(t, event))[fam.String()]
					require.Equal(t, []any{map[string]any{"action": "del", "nlri": want}}, operations, "%s", encoded)
					withdrawn, err := wireu.MPUnreachWire(value).NLRIs(addPath)
					require.NoError(t, err)
					filtered := appendFamiliesJSON([]byte{'{'}, nil, []bgpfilter.FamilyNLRI{{Family: fam, NLRIs: withdrawn}})
					filtered = append(filtered, '}')
					var families map[string]any
					require.NoError(t, json.Unmarshal(filtered, &families))
					require.Equal(t, []any{map[string]any{"action": "del", "nlri": want}}, families[fam.String()], "%s", filtered)
					require.Equal(t, original, body, "JSON decoding must preserve native wire bytes")
				})
			}
		}
	}
}

// vpnConsumerSection keeps adjacent same-prefix/different-RD and same-RD/different-
// prefix routes in the section. The latter also carries a nonzero Path Identifier.
func vpnConsumerSection(afi family.AFI, addPath bool, compatibility [3]byte) ([]byte, []any) {
	var section []byte
	var want []any
	for i := range 3 {
		rd := byte(1)
		if i == 1 {
			rd = 2
		}
		pathID := byte(0)
		if i == 2 {
			pathID = 17
		}
		if addPath {
			section = append(section, 0, 0, 0, pathID)
		}
		prefix := []byte{10}
		prefixText := "10.0.0.0/8"
		if i == 2 {
			prefix[0] = 11
			prefixText = "11.0.0.0/8"
		}
		if afi == family.AFIIPv6 {
			prefix = []byte{0x20, 1, 0x0d, 0xb8}
			prefixText = "2001:db8::/32"
			if i == 2 {
				prefix[3] = 0xb9
				prefixText = "2001:db9::/32"
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
