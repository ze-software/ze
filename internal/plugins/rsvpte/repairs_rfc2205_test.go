// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- three RFC 2205 and RFC
// 3209 obligations the codec and the engine broke and now meet: the RESV names
// its sender as FILTER_SPEC, a NULL object is recognized, and a PATH without a
// SENDER_TSPEC is refused.
//
// VALIDATES: buildResv writes Class-Num 10 for the sender and binds LABEL and
// RECORD_ROUTE to it; classKnownUnprocessed recognizes Class-Num 0; handlePath
// drops a PATH with no SENDER_TSPEC and logs it locally.
// PREVENTS: a RESV a peer rejects for naming its sender as SENDER_TEMPLATE, a
// PATH padded with NULL objects answered with Error Code 13, and a reservation
// admitted at a token rate of zero the sender never asked for.
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rfc2205ResvObjects encodes the RESV buildResv writes for a reservation that
// carries an RRO, and returns its object headers in wire order.
func rfc2205ResvObjects(t *testing.T) []objectHeader {
	t.Helper()
	rsb := &resvStateBlock{
		Session:  sessionIPv4{TunnelEndpoint: rfc2205Egress, TunnelID: 7, ExtTunnelID: 0x0a000001},
		FlowSpec: FlowSpec{TokenRate: 4e8, TokenBucket: 4e8, PeakRate: 4e8},
		Label:    labelObject{Label: 16050},
		Style:    StyleSharedExplicit,
		RRO:      []rroEntry{{Type: RROSubIPv4, Address: rfc2205Egress}},
	}
	filter := senderTemplateIPv4{SenderAddr: rfc2205Ingress, LSPID: 1}
	return objectBodies(t, buildResv(rsb, filter, DefaultRefreshPeriod, rfc2205Transit))
}

// classIndex returns the position of the first object of classNum, or -1.
func classIndex(objs []objectHeader, classNum uint8) int {
	for i, obj := range objs {
		if obj.ClassNum == classNum {
			return i
		}
	}
	return -1
}

// RFC requirement: RFC3209-3-1 positive — the RESV buildResv encodes names its sender as a FILTER_SPEC object (Class-Num 10, C-Type 7 LSP_TUNNEL_IPv4) and writes the LABEL and the RECORD_ROUTE directly after it.
func TestRFC3209ResvLabelFollowsFilterSpec(t *testing.T) {
	objs := rfc2205ResvObjects(t)
	filter := classIndex(objs, ClassFilterSpec)
	require.NotEqual(t, -1, filter, "the RESV carries a FILTER_SPEC")
	assert.Equal(t, CTypeLSPTunnelIPv4, objs[filter].CType, "RFC 3209 Section 4.6.3.1: LSP_TUNNEL_IPv4 C-Type = 7")
	require.Less(t, filter+2, len(objs), "the LABEL and the RECORD_ROUTE follow the FILTER_SPEC")
	assert.Equal(t, ClassLabel, objs[filter+1].ClassNum, "the LABEL is bound to the FILTER_SPEC before it")
	assert.Equal(t, ClassRecordRoute, objs[filter+2].ClassNum, "the RECORD_ROUTE is bound to the same FILTER_SPEC")
}

// RFC requirement: RFC3209-3-1 negative — the RESV buildResv encodes never names its sender as a SENDER_TEMPLATE object (Class-Num 11), and no LABEL or RECORD_ROUTE appears before the FILTER_SPEC they are bound to.
func TestRFC3209ResvNeverCarriesSenderTemplate(t *testing.T) {
	objs := rfc2205ResvObjects(t)
	assert.Equal(t, -1, classIndex(objs, ClassSenderTemplate), "a RESV carries no SENDER_TEMPLATE; that class belongs to the Path family")
	filter := classIndex(objs, ClassFilterSpec)
	require.NotEqual(t, -1, filter, "the RESV carries a FILTER_SPEC")
	for _, obj := range objs[:filter] {
		assert.NotEqual(t, ClassLabel, obj.ClassNum, "no LABEL precedes the FILTER_SPEC it is bound to")
		assert.NotEqual(t, ClassRecordRoute, obj.ClassNum, "no RECORD_ROUTE precedes the FILTER_SPEC it is bound to")
	}
}

// nullObject returns an encoder for a NULL object (Class-Num 0) whose body is
// bodyLen octets of 0xFF: RFC 2205 Section 3.1 lets the length be any multiple
// of 4 and says the contents are ignored, so the body is deliberately not zero.
func nullObject(bodyLen int) objEncoder {
	return func(b []byte) int {
		encodeObjectHeader(b, objectHeader{Length: uint16(objHdrLen + bodyLen), ClassNum: ClassNull, CType: 1})
		for i := objHdrLen; i < objHdrLen+bodyLen; i++ {
			b[i] = 0xFF
		}
		return objHdrLen + bodyLen
	}
}

// pathWithNullObjects encodes a conformant PATH with a NULL object of eight
// octets between every pair of its objects, and one more at the end.
func pathWithNullObjects() []byte {
	objects := conformantMessages()[MsgTypePath]
	encoders := make([]objEncoder, 0, 2*len(objects))
	for _, obj := range objects {
		encoders = append(encoders, obj.encode, nullObject(8))
	}
	return encodeMessage(MsgTypePath, defaultIPTTL, encoders)
}

// RFC requirement: RFC2205-3-1 positive — a PATH carrying NULL objects (Class-Num 0) between its objects decodes as a known message: NULL is not classified as an unknown class, and every object around the NULLs decodes.
func TestRFC2205NullObjectRecognized(t *testing.T) {
	msg, err := DecodeMessage(pathWithNullObjects())
	require.NoError(t, err)
	assert.False(t, msg.HasUnknownObject, "NULL heads the Section 3.1 list of classes an implementation must recognize")
	assert.True(t, msg.HasSession)
	assert.True(t, msg.HasHop)
	assert.True(t, msg.HasTimeValues)
	assert.True(t, msg.HasSenderTemplate)
	assert.True(t, msg.HasSenderTSpec)
	assert.InDelta(t, 1e8, msg.SenderTSpec.TokenRate, 1, "the 0xFF NULL bodies leak into no decoded field")
}

// RFC requirement: RFC2205-3-1 negative — an egress that receives a PATH carrying NULL objects answers no PathErr with Error Code 13 (Unknown object class): it installs the path state and answers the RESV a conformant PATH earns.
func TestRFC2205NullObjectNotRejected(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.9", nil)
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: pathWithNullObjects()})

	pathErr, _, sent := ft.lastByType(MsgTypePathErr)
	if sent {
		assert.NotEqual(t, ErrCodeUnknownObjectClass, pathErr.ErrorSpec.ErrorCode, "a NULL object is not an unknown class")
	}
	assert.Len(t, e.table.All(), 1, "the PATH installs path state despite its NULL objects")
	_, _, resvSent := ft.lastByType(MsgTypeResv)
	assert.True(t, resvSent, "the egress answers the PATH with a RESV")
}

// RFC requirement: RFC2205-2-4 negative — an egress that receives a PATH without a SENDER_TSPEC installs no path state, answers no RESV and, as Appendix B prescribes for a malformed message, sends no PathErr.
func TestRFC2205PathWithoutSenderTSpecDropped(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.9", nil)
	raw := encodeWithout(MsgTypePath, conformantMessages()[MsgTypePath], "SENDER_TSPEC")
	msg, err := DecodeMessage(raw)
	require.NoError(t, err, "the sender descriptor is bracketed, so the construction check passes")
	require.False(t, msg.HasSenderTSpec, "the crafted PATH really lacks a SENDER_TSPEC")

	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: raw})

	assert.Empty(t, e.table.All(), "no path state for a PATH without SENDER_TSPEC")
	assert.Zero(t, sentCount(ft), "no RESV and no PathErr answer a PATH without SENDER_TSPEC")
}
