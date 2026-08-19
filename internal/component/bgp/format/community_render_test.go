package format

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	// The four community JSON formatters register from this plugin's init().
	// Without it GetJSONFormatter returns nil for all four codes and every
	// assertion below would pass for the wrong reason.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community"
)

// Attribute values carried by the fixture UPDATE. Each is the value half of
// the attribute, without the 3-octet header.
var (
	// COMMUNITIES (RFC 1997): 65001:100.
	fixtureCommunity = []byte{0xfd, 0xe9, 0x00, 0x64}

	// EXTENDED_COMMUNITIES (RFC 4360 Section 3.1): 2-octet AS specific
	// Route Target, AS 65000, local administrator 1.
	fixtureExtCommunity = []byte{0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01}

	// IPV6_EXTENDED_COMMUNITIES (RFC 5701 Section 2): 20 octets, type/subtype
	// then the 16-octet IPv6 global administrator 2001:db8:: then the 2-octet
	// local administrator 1.
	fixtureIPv6ExtCommunity = []byte{
		0x00, 0x02,
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x01,
	}

	// LARGE_COMMUNITIES (RFC 8092): 65001:1:2.
	fixtureLargeCommunity = []byte{
		0x00, 0x00, 0xfd, 0xe9,
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x02,
	}
)

// buildCommunityUpdateBody builds an UPDATE body carrying one of each of the
// four community attributes, plus the mandatory ORIGIN and NEXT_HOP and one
// IPv4 NLRI. The bytes are what a peer puts on the wire, so the renderers
// under test reach them through the daemon's own parse path.
func buildCommunityUpdateBody() []byte {
	appendAttr := func(dst []byte, flags byte, code attribute.AttributeCode, value []byte) []byte {
		dst = append(dst, flags, byte(code), byte(len(value)))
		return append(dst, value...)
	}

	const optionalTransitive = 0xc0
	const wellKnown = 0x40

	var attrs []byte
	attrs = appendAttr(attrs, wellKnown, attribute.AttrOrigin, []byte{0x00})
	attrs = appendAttr(attrs, wellKnown, attribute.AttrASPath, nil)
	attrs = appendAttr(attrs, wellKnown, attribute.AttrNextHop, []byte{0xc0, 0xa8, 0x01, 0x02})
	attrs = appendAttr(attrs, optionalTransitive, attribute.AttrCommunity, fixtureCommunity)
	attrs = appendAttr(attrs, optionalTransitive, attribute.AttrExtCommunity, fixtureExtCommunity)
	attrs = appendAttr(attrs, optionalTransitive, attribute.AttrIPv6ExtCommunity, fixtureIPv6ExtCommunity)
	attrs = appendAttr(attrs, optionalTransitive, attribute.AttrLargeCommunity, fixtureLargeCommunity)

	nlri := []byte{0x18, 0x0a, 0x00, 0x00} // 10.0.0.0/24

	body := make([]byte, 4, 4+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(body[0:2], 0)
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs))) //nolint:gosec // G115: fixture is 70 octets
	body = append(body, attrs...)
	return append(body, nlri...)
}

// communityUpdateMessage builds the RawMessage the daemon hands to
// AppendMessage for a received UPDATE, with the lazy wire wrappers populated
// exactly as the reactor populates them.
func communityUpdateMessage(t *testing.T) (*plugin.PeerInfo, bgptypes.RawMessage) {
	t.Helper()

	body := buildCommunityUpdateBody()
	wireUpdate := wireu.NewWireUpdate(body, testEncodingContext())
	attrs, err := wireUpdate.Attrs()
	require.NoError(t, err, "Attrs() must parse the fixture attribute block")
	require.NotNil(t, attrs, "fixture carries attributes, so AttrsWire must not be nil")

	return testPeer(), bgptypes.RawMessage{
		Type:       msgtype.TypeUPDATE,
		RawBytes:   body,
		Timestamp:  time.Now(),
		AttrsWire:  attrs,
		WireUpdate: wireUpdate,
	}
}

// attrHexKey is the "attr-N" key appendAttributeJSON and appendAttributeText
// fall back to when no formatter claims the attribute.
func attrHexKey(code attribute.AttributeCode) string {
	return "attr-" + strconv.FormatUint(uint64(code), 10)
}

// TestReceivedCommunitiesRenderDecoded drives AppendMessage with an UPDATE
// carrying all four community attributes on the wire, and asserts each one
// reaches the reader decoded rather than as raw hex.
//
// VALIDATES: a COMMUNITIES, EXTENDED_COMMUNITIES, IPV6_EXTENDED_COMMUNITIES or
// LARGE_COMMUNITIES attribute parsed off the wire renders under its own key on
// the JSON fast path, the JSON generic path and the human text path.
//
// PREVENTS: the formatters asserting a POINTER while every parser in
// internal/core/bgp/attribute/community.go returns a VALUE. That assertion
// never matched, so appendCommunitiesJSON and its three siblings returned nil,
// appendAttributeJSON read the nil as "no formatter", and every community
// attribute a peer sent rendered as "attr-8", "attr-16", "attr-25" or
// "attr-32" with a hex payload.
func TestReceivedCommunitiesRenderDecoded(t *testing.T) {
	// The JSON surface renders EXTENDED_COMMUNITIES by name, through
	// attribute.ExtendedCommunity.AppendDecoded. The fixture is a two-octet AS
	// Route Target for AS 65000, local administrator 1.
	const extName = "target:65000:1"

	// The text surface still writes the raw hex, because "x-com <hex>" is the
	// filter-text contract every filter plugin parses
	// (docs/architecture/api/text-format.md).
	extHex := hex.EncodeToString(fixtureExtCommunity)

	// IPV6_EXTENDED_COMMUNITIES stays hex on every surface: RFC 5701 Section 2
	// puts a 16-octet IPv6 global administrator where RFC 4360 Section 3.1 puts
	// a 2-octet AS, so the named vocabulary reads nothing in it.
	ipv6ExtHex := hex.EncodeToString(fixtureIPv6ExtCommunity)

	tests := []struct {
		name     string
		content  bgptypes.ContentConfig
		wants    []string
		unwanted []attribute.AttributeCode
	}{
		{
			// JSON + parsed + no filters + AttrsWire set takes the fast path
			// through appendParsedUpdateJSONDirect.
			name:    "json parsed fast path",
			content: bgptypes.ContentConfig{Encoding: "json", Format: "parsed"},
			wants: []string{
				`"communities":["65001:100"]`,
				`"extended-communities":["` + extName + `"]`,
				`"ipv6-extended-communities":["` + ipv6ExtHex + `"]`,
				`"large-communities":["65001:1:2"]`,
			},
			unwanted: []attribute.AttributeCode{
				attribute.AttrCommunity,
				attribute.AttrExtCommunity,
				attribute.AttrIPv6ExtCommunity,
				attribute.AttrLargeCommunity,
			},
		},
		{
			// JSON + full goes through the filter machinery and
			// appendFilterResultJSON instead.
			name:    "json full generic path",
			content: bgptypes.ContentConfig{Encoding: "json", Format: "full"},
			wants: []string{
				`"communities":["65001:100"]`,
				`"extended-communities":["` + extName + `"]`,
				`"ipv6-extended-communities":["` + ipv6ExtHex + `"]`,
				`"large-communities":["65001:1:2"]`,
			},
			unwanted: []attribute.AttributeCode{
				attribute.AttrCommunity,
				attribute.AttrExtCommunity,
				attribute.AttrIPv6ExtCommunity,
				attribute.AttrLargeCommunity,
			},
		},
		{
			// Text goes through appendFilterResultText and its own
			// per-attribute switch in text_human.go. That switch has a case
			// for each of the three community codes, so a failed assertion
			// there emits NOTHING and never reaches the attr-N fallback: an
			// "attr-8 absent" assertion could not fail on this surface and is
			// deliberately omitted. attr-25 is the correct output here,
			// because the text surface has no short form for
			// IPV6_EXTENDED_COMMUNITIES.
			name:    "text parsed",
			content: bgptypes.ContentConfig{Encoding: "text", Format: "parsed"},
			wants: []string{
				"s-com 65001:100",
				"x-com " + extHex,
				"l-com 65001:1:2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peer, msg := communityUpdateMessage(t)
			got := string(AppendMessage(nil, peer, msg, tt.content))

			for _, want := range tt.wants {
				require.Contains(t, got, want, "output must carry the decoded attribute\n%s", got)
			}
			for _, code := range tt.unwanted {
				require.NotContains(t, got, attrHexKey(code),
					"%s must not fall back to the hex form\n%s", code.String(), got)
			}
		})
	}
}
