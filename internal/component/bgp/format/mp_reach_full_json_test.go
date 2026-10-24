package format

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFullJSONStatesMPReachOnce checks that a `full` UPDATE event states an
// MP_REACH_NLRI once, decoded in the `nlri` section, and never again as an
// `attr-14` hex member of `attr`.
//
// Method: one IPv6 unicast UPDATE (ORIGIN, empty AS_PATH, MP_REACH for
// 2001:db8:1::/48 via 2001:db8::1), rendered under FormatParsed and
// FormatFull. Both must decode the prefix, and neither may carry attr-14.
//
// VALIDATES: appendAttributesJSON skips the MP attributes the NLRI section
// states.
// PREVENTS: the ExaBGP bridge writing `attribute-0x0E-0x80`, a member ExaBGP
// never writes, on every live MP line.
func TestFullJSONStatesMPReachOnce(t *testing.T) {
	body := []byte{
		0x00, 0x00, // withdrawn routes length
		0x00, 0x26, // total path attribute length: 4 + 3 + 31
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x02, 0x00, // AS_PATH, empty
		0x80, 0x0E, 0x1C, // MP_REACH_NLRI, optional, 28 octets
		0x00, 0x02, 0x01, // AFI IPv6, SAFI unicast
		0x10, // next hop length 16
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01,
		0x00,                                     // reserved
		0x30, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, // 2001:db8:1::/48
	}
	peer := plugin.PeerInfo{Address: netip.MustParseAddr("2001:db8::1"), PeerAS: rpc.RemoteASOf(65001)}
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)

	for _, format := range []string{plugin.FormatParsed, plugin.FormatFull} {
		t.Run(format, func(t *testing.T) {
			wireUpdate := wireu.NewWireUpdate(body, ctxID)
			attrsWire, err := wireUpdate.Attrs()
			require.NoError(t, err)
			msg := bgptypes.RawMessage{
				Type: msgtype.TypeUPDATE, RawBytes: body, AttrsWire: attrsWire,
				WireUpdate: wireUpdate, Direction: rpc.DirectionReceived,
			}
			content := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: format}
			output := string(AppendMessage(nil, &peer, msg, content))

			var document struct {
				BGP struct {
					Update struct {
						Attr map[string]any `json:"attr"`
						NLRI map[string]any `json:"nlri"`
					} `json:"update"`
				} `json:"bgp"`
			}
			require.NoError(t, json.Unmarshal([]byte(output), &document), output)
			update := document.BGP.Update
			if _, has := update.Attr["attr-14"]; has {
				t.Errorf("attr states MP_REACH again as attr-14: %s", output)
			}
			if _, has := update.NLRI["ipv6/unicast"]; !has {
				t.Errorf("the nlri section lost the decoded MP_REACH: %s", output)
			}
			if update.Attr["origin"] != "igp" {
				t.Errorf("the other attributes are gone: %s", output)
			}
		})
	}
}
