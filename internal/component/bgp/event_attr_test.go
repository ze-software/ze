package bgp_test

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
)

// ParseEvent read path attributes from an "attributes" object while every
// writer in internal/component/bgp/format opens the update body with `"attr":{`.
// Nothing compared the two, so a subscriber on format="parsed" received an
// event with no origin, no AS path, no MED, no local preference and no
// communities, and every one of those fields read as absent rather than as
// unparsed. The tests here drive the writer rather than a hand-written string,
// which is the only way that disagreement is visible.

// parsedUpdateEvent builds an UPDATE carrying the well-known attributes and one
// IPv4 unicast route, and returns the JSON line the daemon writes for it.
func parsedUpdateEvent(t *testing.T) []byte {
	t.Helper()

	appendAttr := func(dst []byte, flags byte, code attribute.AttributeCode, value []byte) []byte {
		dst = append(dst, flags, byte(code), byte(len(value)))
		return append(dst, value...)
	}

	const wellKnown = 0x40
	const optional = 0x80

	var attrs []byte
	attrs = appendAttr(attrs, wellKnown, attribute.AttrOrigin, []byte{0x00}) // igp
	// One AS_SEQUENCE segment holding AS 65001, four octets wide because the
	// encoding context below negotiated four-octet ASNs (RFC 6793).
	attrs = appendAttr(attrs, wellKnown, attribute.AttrASPath, []byte{0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9})
	attrs = appendAttr(attrs, wellKnown, attribute.AttrNextHop, []byte{0x0a, 0x00, 0x00, 0x02})
	attrs = appendAttr(attrs, optional, attribute.AttrMED, []byte{0x00, 0x00, 0x00, 0x64})
	attrs = appendAttr(attrs, wellKnown, attribute.AttrLocalPref, []byte{0x00, 0x00, 0x00, 0xc8})

	nlri := []byte{0x18, 0x0a, 0x00, 0x00} // 10.0.0.0/24

	body := make([]byte, 4, 4+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(body[0:2], 0)
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	body = append(body, attrs...)
	body = append(body, nlri...)

	ctx := bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65002, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true},
		bgpctx.DirectionRecv,
	)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err, "the fixture attribute block must parse")

	peer := &plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAddress: netip.MustParseAddr("10.0.0.2"),
		Name:         "peer1",
		LocalAS:      65002,
		PeerAS:       65001,
	}
	msg := bgptypes.RawMessage{
		Type:       msgtype.TypeUPDATE,
		RawBytes:   body,
		Timestamp:  time.Now(),
		MessageID:  7,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
	}
	content := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatParsed}
	return format.AppendMessage(nil, peer, msg, content)
}

// TestParseEventReadsTheAttrKeyTheWriterProduces closes the producer-to-parser
// gap on path attributes.
//
// VALIDATES: every attribute the daemon writes under `update.attr` reaches the
// parsed Event.
// PREVENTS: ParseEvent reading `update.attributes`, a key no writer produces.
// A subscriber then saw an UPDATE with no origin, no AS path and no
// communities, and read each absence as "the peer sent none" -- so a policy
// plugin keyed on a community matched nothing and said nothing.
func TestParseEventReadsTheAttrKeyTheWriterProduces(t *testing.T) {
	data := parsedUpdateEvent(t)
	require.Contains(t, string(data), `"attr":{`,
		"the writer opens the update body with attr; if it stops, this test is pinning the wrong key")

	event, err := bgp.ParseEvent(data)
	require.NoError(t, err, "the daemon's own event must parse: %s", data)

	assert.Equal(t, "igp", event.Origin, "origin must survive the parser: %s", data)
	assert.Equal(t, []uint32{65001}, event.ASPath)
	require.NotNil(t, event.MED)
	assert.Equal(t, uint32(100), *event.MED)
	require.NotNil(t, event.LocalPreference)
	assert.Equal(t, uint32(200), *event.LocalPreference)

	require.Contains(t, event.FamilyOps, family.IPv4Unicast)
}

// TestParseEventStillReadsTheAttributesKey pins the compatibility half of the
// same change.
//
// VALIDATES: an event whose attributes sit under "attributes" parses as before.
// PREVENTS: the attr fix being a swap rather than an addition, which would
// retire whatever producer still writes the older spelling without anybody
// finding out until a field read as absent.
func TestParseEventStillReadsTheAttributesKey(t *testing.T) {
	input := []byte(`{"type":"bgp","bgp":{
		"message":{"type":"update","id":7,"direction":"received"},
		"peer":{"local":{"address":"10.0.0.2","as":65002},"name":"peer1","remote":{"address":"10.0.0.1","as":65001}},
		"update":{
			"attributes":{"origin":"egp","as-path":[65010],"med":5,"local-preference":50},
			"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.9.0.0/24"]}]}
		}
	}}`)

	event, err := bgp.ParseEvent(input)
	require.NoError(t, err)

	assert.Equal(t, "egp", event.Origin)
	assert.Equal(t, []uint32{65010}, event.ASPath)
	require.NotNil(t, event.MED)
	assert.Equal(t, uint32(5), *event.MED)
	require.NotNil(t, event.LocalPreference)
	assert.Equal(t, uint32(50), *event.LocalPreference)
}
