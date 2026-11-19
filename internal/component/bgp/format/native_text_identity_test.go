// Design: docs/architecture/api/text-parser.md -- native text identity and original event facts.
package format

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/stretchr/testify/require"
	bgpfilter "github.com/ze-software/ze/internal/component/bgp/filter"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"testing"
)

func TestNativeTextOpaqueRouteKeepsFramingAndPathZero(t *testing.T) {
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	native := make([]byte, 27)
	native[0], native[1], native[9], native[26] = 1, 25, 1, 1
	for _, addPath := range []bool{false, true} {
		encoding := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: addPath})
		reach := []byte{0, 25, 70, 4, 192, 0, 2, 1, 0}
		if addPath {
			reach = append(reach, 0, 0, 0, 0)
		}
		reach = append(reach, native...)
		result := bgpfilter.FilterResult{MPReach: []wireu.MPReachWire{wireu.MPReachWire(reach)}}
		peer := testPeer()
		rendered := string(appendFilterResultText(nil, peer, result, 5, 7, 9, rpc.DirectionReceived, encoding))
		section := "nlri " + fam.String()
		if addPath {
			section += " info 0"
		}
		require.Contains(t, rendered, section+" add raw "+hex.EncodeToString(native))
		require.Equal(t, native, reach[len(reach)-len(native):])
	}
}
func TestNativeTextAllAttributesCarriesNoLLGR(t *testing.T) {
	for _, present := range []bool{false, true} {
		attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1}
		if present {
			attrs = append(attrs, 0xc0, 8, 4, 0xff, 0xff, 0, 7)
		}
		body := make([]byte, 4+len(attrs)+4)
		binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
		copy(body[4:], attrs)
		copy(body[4+len(attrs):], []byte{24, 10, 1, 0})
		before := bytes.Clone(body)
		contextID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
		require.NoError(t, err)
		update := wireu.NewWireUpdate(body, contextID)
		wire, err := update.Attrs()
		require.NoError(t, err)
		message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: body, AttrsWire: wire, WireUpdate: update, MessageID: 5, Direction: rpc.DirectionReceived}
		rendered := string(AppendMessage(nil, testPeer(), message, bgptypes.ContentConfig{Encoding: plugin.EncodingText, Format: plugin.FormatParsed}))
		if present {
			require.Contains(t, rendered, "s-com no-llgr")
		} else {
			require.NotContains(t, rendered, "no-llgr")
		}
		require.Contains(t, rendered, "nlri ipv4/unicast add 10.1.0.0/24")
		require.Equal(t, before, body)
	}
}
func TestNativeTextOpenCarriesCapturedPhysicalIdentity(t *testing.T) {
	body := []byte{4, 0xfd, 0xe9, 0, 90, 192, 0, 2, 1, 0}
	before := bytes.Clone(body)
	peer := &plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.7"), PeerAS: rpc.RemoteASOf(65001), PeerOwner: ^uint64(0), ConnectionID: ^uint64(0) - 1}
	message := bgptypes.RawMessage{Type: msgtype.TypeOPEN, RawBytes: body, MessageID: 5, Direction: rpc.DirectionReceived}
	rendered := string(AppendMessage(nil, peer, message, bgptypes.ContentConfig{Encoding: plugin.EncodingText, Format: plugin.FormatParsed}))
	require.Contains(t, rendered, "open 5 peer-owner 18446744073709551615 connection-id 18446744073709551614 asn 65001")
	require.Equal(t, before, body)
}
