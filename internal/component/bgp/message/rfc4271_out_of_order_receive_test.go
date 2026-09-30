package message_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestRFC4271OutOfOrderAttributesReachTheReceivePathIntact proves the receive
// path reads every path attribute of an UPDATE whatever order the sender used.
//
// VALIDATES: the same five attributes (ORIGIN, AS_PATH, NEXT_HOP, MED,
// LOCAL_PREF) and one NLRI, sent ascending, interleaved and fully descending,
// each pass ValidateUpdateRFC7606 with no action, and wireu.WireUpdate (the
// reactor's lazy view) returns for every attribute the value the ascending
// UPDATE carries, with the same NLRI.
// PREVENTS: a receiver whose lookup stops at the first attribute whose type
// code is lower than the previous one, or that reads a value at the offset an
// ascending layout would put it.
//
// RFC requirement: RFC4271-5-6 positive -- an UPDATE whose attributes arrive interleaved
// (MED, ORIGIN, LOCAL_PREF, NEXT_HOP, AS_PATH) is accepted and every attribute value and the
// NLRI read on the receive path equal the ascending UPDATE's.
// RFC requirement: RFC4271-5-6 negative -- an UPDATE whose attributes arrive in fully
// descending type-code order is not refused and loses no attribute: every value and the NLRI
// read on the receive path equal the ascending UPDATE's.
func TestRFC4271OutOfOrderAttributesReachTheReceivePathIntact(t *testing.T) {
	origin := []byte{0x40, 0x01, 0x01, 0x02}                               // ORIGIN INCOMPLETE
	asPath := []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xea} // AS_SEQUENCE 65002
	nextHop := []byte{0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01}            // 192.0.2.1
	med := []byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x0a}                // 10
	localPref := []byte{0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0xc8}          // 200
	nlri := []byte{24, 198, 51, 100}

	ascending := updateBody([][]byte{origin, asPath, nextHop, med, localPref}, nlri)
	interleaved := updateBody([][]byte{med, origin, localPref, nextHop, asPath}, nlri)
	descending := updateBody([][]byte{localPref, med, nextHop, asPath, origin}, nlri)

	codes := []attribute.AttributeCode{
		attribute.AttrOrigin, attribute.AttrASPath, attribute.AttrNextHop,
		attribute.AttrMED, attribute.AttrLocalPref,
	}
	want := readAttributes(t, ascending, codes)
	for name, body := range map[string][]byte{"interleaved": interleaved, "descending": descending} {
		attrLen := int(body[2])<<8 | int(body[3])
		// An iBGP ASN4 session: LOCAL_PREF is kept and AS_PATH has 4-octet ASNs.
		res := message.ValidateUpdateRFC7606(body[4:4+attrLen], true, true, true)
		require.Equal(t, message.RFC7606ActionNone, res.Action, "%s: %s", name, res.Description)

		got := readAttributes(t, body, codes)
		for i, code := range codes {
			require.Equal(t, want[i], got[i], "%s: attribute %d", name, code)
		}
		gotNLRI, err := wireu.NewWireUpdate(body, 0).NLRI()
		require.NoError(t, err, name)
		require.Equal(t, nlri, gotNLRI, "%s: NLRI", name)
	}
}

// updateBody frames an UPDATE body: no withdrawn routes, attrs in the given
// order, then nlri.
func updateBody(attrs [][]byte, nlri []byte) []byte {
	var packed []byte
	for _, a := range attrs {
		packed = append(packed, a...)
	}
	body := []byte{0, 0, byte(len(packed) >> 8), byte(len(packed))}
	body = append(body, packed...)
	return append(body, nlri...)
}

// readAttributes reads each code through the receive path's lazy view and
// fails when one is missing. The source context is a registered ASN4 session,
// as the reactor gives every received UPDATE.
func readAttributes(t *testing.T, body []byte, codes []attribute.AttributeCode) []attribute.Attribute {
	t.Helper()
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	attrs, err := wireu.NewWireUpdate(body, ctxID).Attrs()
	require.NoError(t, err)
	out := make([]attribute.Attribute, len(codes))
	for i, code := range codes {
		a, getErr := attrs.Get(code)
		require.NoError(t, getErr, "attribute %d", code)
		require.NotNil(t, a, "attribute %d", code)
		out[i] = a
	}
	return out
}
