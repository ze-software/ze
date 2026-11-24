// Design: docs/architecture/api/json-format.md -- negotiated 4-byte ASN width on UPDATE events.
// Related: text_update.go -- appendASN4JSON, appendParsedUpdateJSONDirect, appendRawFromResult.
// Related: text_json.go -- appendFilterResultJSON.

package format

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgpfilter "github.com/ze-software/ze/internal/component/bgp/filter"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestUpdateJSONStampsASN4 verifies that every JSON UPDATE event path stamps
// `"asn4":true` or `"asn4":false` inside the `"bgp"` object when an
// EncodingContext is known, and omits `"asn4"` when no context is attached.
//
// VALIDATES: Q-E4 wire invariant — consumers (such as the ExaBGP translator)
// can decode raw AS_PATH bytes accurately from `bgp:update` without needing a
// hidden `bgp:negotiated` subscription.
// PREVENTS: a missing or wrong `asn4` stamp on fast-path parsed, filtered
// parsed, full, raw, hex, or EOR JSON UPDATE events.
func TestUpdateJSONStampsASN4(t *testing.T) {
	t.Parallel()

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("192.0.2.10"),
		LocalAddress: netip.MustParseAddr("192.0.2.1"),
		LocalAS:      65000,
		PeerAS:       rpc.RemoteASOf(65001),
		ConnectionID: 99,
	}
	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParseAddr("192.0.2.10"),
		0, 0, nil,
	)

	allAttrs := bgpfilter.NewFilterAll()

	for _, tc := range []struct {
		name     string
		asn4     bool
		withCtx  bool
		wantJSON string
	}{
		{name: "asn4-true", asn4: true, withCtx: true, wantJSON: "true"},
		{name: "asn4-false", asn4: false, withCtx: true, wantJSON: "false"},
		{name: "nil-context", withCtx: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var ctxID bgpctx.ContextID
			if tc.withCtx {
				var err error
				ctxID, err = bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(tc.asn4))
				require.NoError(t, err)
			}

			wireUpdate := wireu.NewWireUpdate(body, ctxID)
			attrsWire, err := wireUpdate.Attrs()
			require.NoError(t, err)

			msg := bgptypes.RawMessage{
				Type:       msgtype.TypeUPDATE,
				Direction:  rpc.DirectionReceived,
				MessageID:  7,
				RawBytes:   body,
				WireUpdate: wireUpdate,
				AttrsWire:  attrsWire,
			}
			eor := bgptypes.RawMessage{
				Type:       msgtype.TypeUPDATE,
				Direction:  rpc.DirectionReceived,
				MessageID:  8,
				RawBytes:   []byte{0, 0, 0, 0},
				WireUpdate: wireu.NewWireUpdate([]byte{0, 0, 0, 0}, ctxID),
			}

			forms := []struct {
				name    string
				msg     bgptypes.RawMessage
				content bgptypes.ContentConfig
			}{
				{
					name:    "parsed-direct",
					msg:     msg,
					content: bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatParsed},
				},
				{
					name:    "parsed-filtered",
					msg:     msg,
					content: bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatParsed, Attributes: &allAttrs},
				},
				{
					name:    "full",
					msg:     msg,
					content: bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatFull},
				},
				{
					name:    "raw",
					msg:     msg,
					content: bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatRaw},
				},
				{
					name:    "hex",
					msg:     msg,
					content: bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatHex},
				},
				{
					name:    "eor",
					msg:     eor,
					content: bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatFull},
				},
			}

			for _, form := range forms {
				t.Run(form.name, func(t *testing.T) {
					t.Parallel()
					encoded := AppendMessage(nil, &peer, form.msg, form.content)
					var doc struct {
						BGP map[string]json.RawMessage `json:"bgp"`
					}
					require.NoError(t, json.Unmarshal(encoded, &doc), "invalid JSON: %s", encoded)
					if !tc.withCtx {
						require.NotContains(t, doc.BGP, "asn4", "nil context must omit asn4: %s", encoded)
						return
					}
					require.Equal(t, json.RawMessage(tc.wantJSON), doc.BGP["asn4"], "encoded: %s", encoded)
				})
			}
		})
	}
}
