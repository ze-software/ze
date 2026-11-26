// Design: docs/architecture/api/json-format.md -- the raw member of a `full` event
// Related: json_message.go -- the formatter under test

package format

import (
	"encoding/json"
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestJSONEncoderMessageFull proves a `full` subscriber's OPEN, NOTIFICATION,
// KEEPALIVE and ROUTE-REFRESH events carry the message body under
// raw.message, and a `parsed` subscriber's carry no raw member.
//
// VALIDATES: the body is the hex of the octets after the header, a KEEPALIVE's
// empty body is an empty member, and the parsed object is still written.
// PREVENTS: the ExaBGP bridge writing no `packets` or `consolidate` line for
// any message but an UPDATE, which was the case until 2026-10-22.
func TestJSONEncoderMessageFull(t *testing.T) {
	enc := NewJSONEncoder("")
	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.2"),
		LocalAddress: netip.MustParseAddr("10.0.0.17"),
		LocalAS:      64512,
		PeerAS:       rpc.RemoteASOf(64512),
	}
	cases := []struct {
		kind msgtype.MessageType
		body []byte
		name string
		hex  string
	}{
		{msgtype.TypeNOTIFICATION, []byte{0x06, 0x02}, "notification", "0602"},
		{msgtype.TypeKEEPALIVE, nil, "keepalive", ""},
		{msgtype.TypeROUTEREFRESH, []byte{0x00, 0x01, 0x00, 0x01}, "refresh", "00010001"},
	}
	for _, tc := range cases {
		raw := bgptypes.RawMessage{Type: tc.kind, RawBytes: tc.body, Direction: rpc.DirectionReceived}
		bgp := decodeBGP(t, enc.Message(&peer, raw, plugin.FormatFull))
		member, ok := bgp["raw"].(map[string]any)
		if !ok {
			t.Fatalf("%s full: no raw member in %v", tc.name, bgp)
		}
		if got := member["message"]; got != tc.hex {
			t.Errorf("%s full: raw.message = %v, want %q", tc.name, got, tc.hex)
		}
		if _, parsed := bgp[tc.name]; !parsed {
			t.Errorf("%s full: the parsed %s object is missing from %v", tc.name, tc.name, bgp)
		}
		if _, has := decodeBGP(t, enc.Message(&peer, raw, plugin.FormatParsed))["raw"]; has {
			t.Errorf("%s parsed: a parsed subscriber is written the raw member", tc.name)
		}
	}
	update := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: []byte{0, 0, 0, 0}}
	if got := enc.Message(&peer, update, plugin.FormatFull); got != "" {
		t.Errorf("an UPDATE has its own formatter, Message wrote %q", got)
	}
}

// TestJSONEncoderMessage_RawB64AttachesRaw proves that OPEN, NOTIFICATION,
// KEEPALIVE, and ROUTE-REFRESH events under plugin.FormatBase64 carry the exact
// same "raw":{"message":"<hex>"} member as plugin.FormatFull, while
// FormatParsed, FormatRaw, and FormatHex omit it.
//
// VALIDATES: AC-2 -- non-UPDATE JSON events attach raw.message for FormatBase64
// identically to FormatFull and omit it for parsed/raw/hex.
// PREVENTS: ExaBGP --form packets|consolidate losing OPEN, NOTIFICATION,
// KEEPALIVE, or ROUTE-REFRESH wire bytes after opting into FormatBase64.
func TestJSONEncoderMessage_RawB64AttachesRaw(t *testing.T) {
	t.Parallel()

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.2"),
		LocalAddress: netip.MustParseAddr("10.0.0.17"),
		LocalAS:      64512,
		PeerAS:       rpc.RemoteASOf(64512),
	}
	// Minimal RFC 4271 OPEN body: version=4, asn=65001, hold=90, router-id=10.0.0.2, opt-len=0.
	openBody := []byte{0x04, 0xfd, 0xe9, 0x00, 0x5a, 0x0a, 0x00, 0x00, 0x02, 0x00}
	cases := []struct {
		kind msgtype.MessageType
		body []byte
		name string
		hex  string
	}{
		{msgtype.TypeOPEN, openBody, "open", "04fde9005a0a00000200"},
		{msgtype.TypeNOTIFICATION, []byte{0x06, 0x02}, "notification", "0602"},
		{msgtype.TypeKEEPALIVE, nil, "keepalive", ""},
		{msgtype.TypeROUTEREFRESH, []byte{0x00, 0x01, 0x00, 0x01}, "refresh", "00010001"},
	}

	omittedFormats := []string{plugin.FormatParsed, plugin.FormatRaw, plugin.FormatHex}

	for _, tc := range cases {
		raw := bgptypes.RawMessage{Type: tc.kind, RawBytes: tc.body, Direction: rpc.DirectionReceived}

		fullEnc := NewJSONEncoder("")
		rawB64Enc := NewJSONEncoder("")
		fullDoc := fullEnc.Message(&peer, raw, plugin.FormatFull)
		rawB64Doc := rawB64Enc.Message(&peer, raw, plugin.FormatBase64)
		if rawB64Doc != fullDoc {
			t.Errorf("%s base64 differs from full:\ngot:  %s\nwant: %s", tc.name, rawB64Doc, fullDoc)
		}

		bgp := decodeBGP(t, rawB64Doc)
		member, ok := bgp[jsonKeyRaw].(map[string]any)
		if !ok {
			t.Fatalf("%s base64: no raw member in %v", tc.name, bgp)
		}
		if got := member[jsonKeyMessage]; got != tc.hex {
			t.Errorf("%s base64: raw.message = %v, want %q", tc.name, got, tc.hex)
		}

		for _, fmtMode := range omittedFormats {
			enc := NewJSONEncoder("")
			if _, has := decodeBGP(t, enc.Message(&peer, raw, fmtMode))[jsonKeyRaw]; has {
				t.Errorf("%s %s: unexpected raw member attached", tc.name, fmtMode)
			}
		}
	}

	enc := NewJSONEncoder("")
	update := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: []byte{0, 0, 0, 0}}
	if got := enc.Message(&peer, update, plugin.FormatBase64); got != "" {
		t.Errorf("an UPDATE has its own formatter, Message(FormatBase64) wrote %q", got)
	}
}

func decodeBGP(t *testing.T, document string) map[string]any {
	t.Helper()
	var outer map[string]any
	if err := json.Unmarshal([]byte(document), &outer); err != nil {
		t.Fatalf("invalid JSON %q: %v", document, err)
	}
	bgp, ok := outer["bgp"].(map[string]any)
	if !ok {
		t.Fatalf("no bgp object in %q", document)
	}
	return bgp
}
