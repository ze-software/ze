package format

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// TestReceivedAggregatorRendersDecoded drives AppendMessage with an UPDATE that
// carries an ATOMIC_AGGREGATE and an AGGREGATOR, and asserts the JSON event
// states them as `true` and "<asn>:<address>".
//
// VALIDATES: both attributes a peer sends reach the plugin's JSON event under
// the keys ExaBGP also writes, "atomic-aggregate" and "aggregator", in the form
// ExaBGP writes, on the fast path and on the full path.
//
// PREVENTS: the attributes falling through to "attr-6" and "attr-7" with their
// octets in hex, which every event carried while no formatter was registered.
func TestReceivedAggregatorRendersDecoded(t *testing.T) {
	// RFC 6793 Section 3: the four-octet form an ASN4 session carries. AS
	// 4200000000 does not fit two octets, so a two-octet decode is caught too.
	value := []byte{0xfa, 0x56, 0xea, 0x00, 0xdb, 0x76, 0xe1, 0xbd}
	wants := []string{`"aggregator":"4200000000:219.118.225.189"`, `"atomic-aggregate":true`}

	tests := []struct {
		name    string
		content bgptypes.ContentConfig
	}{
		{name: "json parsed fast path", content: bgptypes.ContentConfig{Encoding: "json", Format: "parsed"}},
		{name: "json full generic path", content: bgptypes.ContentConfig{Encoding: "json", Format: "full"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := aggregatorUpdateMessage(t, value)
			got := string(AppendMessage(nil, testPeer(), msg, tt.content))
			for _, want := range wants {
				require.Contains(t, got, want, "the attribute must be decoded\n%s", got)
			}
			for _, code := range []attribute.AttributeCode{attribute.AttrAtomicAggregate, attribute.AttrAggregator} {
				require.NotContains(t, got, attrHexKey(code), "%s must not fall back to hex\n%s", code.String(), got)
			}
		})
	}
}

// aggregatorUpdateMessage builds the RawMessage of a received UPDATE carrying
// ORIGIN, an empty AS_PATH, NEXT_HOP, ATOMIC_AGGREGATE, the AGGREGATOR value
// given, and one IPv4 NLRI, with the lazy wire wrappers the reactor populates.
func aggregatorUpdateMessage(t *testing.T, aggregator []byte) bgptypes.RawMessage {
	t.Helper()

	appendAttr := func(dst []byte, flags byte, code attribute.AttributeCode, value []byte) []byte {
		dst = append(dst, flags, byte(code), byte(len(value)))
		return append(dst, value...)
	}
	var attrs []byte
	attrs = appendAttr(attrs, 0x40, attribute.AttrOrigin, []byte{0x00})
	attrs = appendAttr(attrs, 0x40, attribute.AttrASPath, nil)
	attrs = appendAttr(attrs, 0x40, attribute.AttrNextHop, []byte{0xc0, 0xa8, 0x01, 0x02})
	attrs = appendAttr(attrs, 0x40, attribute.AttrAtomicAggregate, nil)
	attrs = appendAttr(attrs, 0xc0, attribute.AttrAggregator, aggregator)
	nlri := []byte{0x18, 0x0a, 0x00, 0x00} // 10.0.0.0/24

	body := make([]byte, 4, 4+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs))) //nolint:gosec // G115: fixture is 28 octets
	body = append(body, attrs...)
	body = append(body, nlri...)

	wireUpdate := wireu.NewWireUpdate(body, testEncodingContext())
	wireAttrs, err := wireUpdate.Attrs()
	require.NoError(t, err, "Attrs() must parse the fixture attribute block")
	return bgptypes.RawMessage{
		Type:       msgtype.TypeUPDATE,
		RawBytes:   body,
		Timestamp:  time.Now(),
		AttrsWire:  wireAttrs,
		WireUpdate: wireUpdate,
	}
}
