// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- wire values the RFCs
// state as numbers: the RSVP version, the reserved fields, the SESSION and
// FAST_REROUTE object headers, the Notify error, and the objects each RSVP
// message MUST carry. Every assertion here compares with the literal the RFC
// writes, never with the package constant that encodes it.
//
// VALIDATES: RFC 2205 Sections 3.1 and 3.1.2 header and class values, RFC 3209
// Section 4.6.1.1 SESSION reserved octets, RFC 4090 Sections 4.1 and 6.5.1.
package rsvpte

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC2205-3.1-1 positive -- a PATH and a RESV that buildPath and buildResv encode carry the literal 1 in the Vers nibble of the common header, and DecodeHeader accepts a header whose first octet is 0x10.
func TestRFC2205EmittedVersionIsOne(t *testing.T) {
	psb := rfc2205PSB()
	path := buildPath(psb, rfc2205Ingress, defaultIPTTL)
	assert.Equal(t, uint8(1), path[0]>>4, "PATH Vers nibble")

	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 16050}, Style: StyleSharedExplicit}
	resv := buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Egress)
	assert.Equal(t, uint8(1), resv[0]>>4, "RESV Vers nibble")

	hdr, err := DecodeHeader([]byte{0x10, MsgTypePath, 0, 0, 64, 0, 0, 8})
	require.NoError(t, err)
	assert.Equal(t, uint8(1), hdr.Version)
}

// RFC requirement: RFC2205-3.1-2 positive -- unused header fields are ignored on receipt: a PATH whose Reserved octet is 0xA5 and whose four Flags bits are all set decodes and brings the egress up with a RESV, exactly as the same PATH with those fields zero; and buildPath sends the Flags nibble as zero.
func TestRFC2205UnusedHeaderFieldsIgnoredOnReceipt(t *testing.T) {
	psb := rfc2205PSB()
	psb.ERO = nil
	raw := buildPath(psb, rfc2205Transit, defaultIPTTL)
	assert.Zero(t, raw[0]&0x0F, "Flags nibble sent as zero")

	raw[0] |= 0x0F
	raw[5] = 0xA5
	setMessageChecksum(raw)

	msg, err := DecodeMessage(raw)
	require.NoError(t, err, "a nonzero unused field is not a reason to refuse the message")
	assert.Equal(t, MsgTypePath, msg.Header.MsgType)

	e, ft, _ := testEngine(t, rfc2205Egress.String(), nil)
	e.handlePacket(Packet{Src: rfc2205Transit, Payload: raw})
	_, ok := e.table.Get(keyFromPSB(psb))
	assert.True(t, ok, "egress installed path state")
	_, dst, sent := ft.lastByType(MsgTypeResv)
	require.True(t, sent, "egress answers the PATH with a RESV")
	assert.Equal(t, rfc2205Transit, dst)
}

// RFC requirement: RFC3209-4.6.1-1 positive -- encodeSessionIPv4 writes the LSP_TUNNEL_IPv4 SESSION into a buffer prefilled with 0xFF and leaves the two reserved octets zero, under an object header of Length 16, Class-Num 1, C-Type 7.
func TestRFC3209SessionReservedZeroOverDirtyBuffer(t *testing.T) {
	buf := make([]byte, 32)
	for i := range buf {
		buf[i] = 0xFF
	}
	n := encodeSessionIPv4(buf, sessionIPv4{TunnelEndpoint: rfc2205Egress, TunnelID: 7, ExtTunnelID: 0x0a000001})
	require.Equal(t, 16, n)
	assert.Equal(t, []byte{0, 16, 1, 7}, buf[0:4], "object header")
	assert.Equal(t, []byte{10, 0, 0, 9}, buf[4:8], "tunnel end point")
	assert.Equal(t, []byte{0, 0}, buf[8:10], "reserved octets are zero")
	assert.Equal(t, []byte{0, 7}, buf[10:12], "Tunnel ID")
}

// RFC requirement: RFC4090-4.1-1 positive -- encodeFastReroute writes the object header Length 24, Class-Num 205, C-Type 1 into a buffer prefilled with 0xFF, and DecodeMessage reads a PATH carrying an object with Class-Num 205 and C-Type 1 as FAST_REROUTE.
func TestRFC4090FastRerouteClassAndCTypeLiterals(t *testing.T) {
	buf := make([]byte, 32)
	for i := range buf {
		buf[i] = 0xFF
	}
	n := encodeFastReroute(buf, fastReroute{SetupPrio: 7, HoldPrio: 7, HopLimit: 16, Flags: 0x02})
	require.Equal(t, 24, n)
	assert.Equal(t, []byte{0, 24, 205, 1}, buf[0:4])

	msg, err := DecodeMessage(pathWithRawObject([]byte{0, 24, 205, 1, 7, 7, 16, 0x02}))
	require.NoError(t, err)
	assert.False(t, msg.HasUnknownObject, "Class-Num 205 C-Type 1 is a known object")
	require.True(t, msg.HasFastReroute, "Class-Num 205 C-Type 1 decodes as FAST_REROUTE")
	assert.Equal(t, uint8(16), msg.FastReroute.HopLimit)
}

// RFC requirement: RFC4090-4.1-1 negative -- an object with Class-Num 205 and a C-Type other than 1 is not read as FAST_REROUTE: DecodeMessage flags it as an unknown C-Type.
func TestRFC4090FastRerouteOtherCTypeNotRecognized(t *testing.T) {
	msg, err := DecodeMessage(pathWithRawObject([]byte{0, 24, 205, 2, 7, 7, 16, 0x02}))
	require.NoError(t, err)
	assert.False(t, msg.HasFastReroute, "C-Type 2 is not the FAST_REROUTE object")
	require.True(t, msg.HasUnknownObject)
	assert.True(t, msg.UnknownCType)
	assert.Equal(t, uint8(205), msg.UnknownObject.ClassNum)
}

// pathWithRawObject returns a conformant PATH that also carries one object:
// head is its first octets, and the object is zero-padded to Length head[1].
func pathWithRawObject(head []byte) []byte {
	encoders := []objEncoder{}
	for _, obj := range conformantMessages()[MsgTypePath] {
		encoders = append(encoders, obj.encode)
	}
	encoders = append(encoders, func(b []byte) int {
		n := int(head[1])
		clear(b[:n])
		copy(b, head)
		return n
	})
	return encodeMessage(MsgTypePath, defaultIPTTL, encoders)
}

// RFC requirement: RFC4090-6.5-1 positive -- a PLR that repairs a protected LSP locally sends the head-end a PathErr whose ERROR_SPEC carries Error Code 25 and the 16-bit Error Value 0x0003: ss=00 and sub-code 3.
func TestRFC4090LocalRepairNotifyLiterals(t *testing.T) {
	e, ft, _ := plrEngine(t)
	armAndUpProtected(t, e)
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))

	e.handleLinkDown("eth0")

	perr, dst, ok := ft.lastByType(MsgTypePathErr)
	require.True(t, ok, "PLR sends a PathErr on local repair")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), dst, "toward the head-end")
	require.True(t, perr.HasErrorSpec)
	assert.Equal(t, uint8(25), perr.ErrorSpec.ErrorCode, "Error Code 25 (Notify)")
	assert.Equal(t, uint16(0x0003), perr.ErrorSpec.ErrorValue, "ss=00, sub-code 3 (Tunnel locally repaired)")
}

// RFC requirement: RFC2205-3-1 positive -- ERROR_SPEC (Class-Num 6) is a recognized class: a PathErr carrying it decodes with no unknown object and with the error node, code and value it was sent with.
func TestRFC2205ErrorSpecClassRecognized(t *testing.T) {
	msg, err := DecodeMessage(encodeWithout(MsgTypePathErr, conformantMessages()[MsgTypePathErr], ""))
	require.NoError(t, err)
	assert.False(t, msg.HasUnknownObject, "ERROR_SPEC is not an unknown class")
	require.True(t, msg.HasErrorSpec)
	assert.Equal(t, netip.MustParseAddr("10.0.0.5"), msg.ErrorSpec.ErrorNode)
	assert.Equal(t, ErrCodeRoutingProblem, msg.ErrorSpec.ErrorCode)
	assert.Equal(t, ErrValueNoRouteAvailable, msg.ErrorSpec.ErrorValue)
}

// reservationMessages returns a conformant ResvTear and ResvErr, the two
// message types conformantMessages leaves out, in RFC 2205 Section 3.1 order.
func reservationMessages() map[uint8][]namedObject {
	session := sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: 0x0a000001}
	hop := rsvpHop{NextHop: netip.MustParseAddr("10.0.0.9")}
	sender := senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1}
	flow := FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8}
	es := errorSpec{ErrorNode: netip.MustParseAddr("10.0.0.5"), ErrorCode: 2}

	sessionObj := namedObject{"SESSION", func(b []byte) int { return encodeSessionIPv4(b, session) }}
	hopObj := namedObject{"RSVP_HOP", func(b []byte) int { return encodeRSVPHop(b, hop) }}
	styleObj := namedObject{"STYLE", func(b []byte) int { return encodeStyle(b, StyleSharedExplicit) }}
	errObj := namedObject{"ERROR_SPEC", func(b []byte) int { return encodeErrorSpec(b, es) }}
	flowObj := namedObject{"FLOWSPEC", func(b []byte) int { return encodeFlowSpec(b, ClassFlowSpec, flow) }}
	filterObj := namedObject{"FILTER_SPEC", func(b []byte) int { return encodeFilterSpec(b, sender) }}
	return map[uint8][]namedObject{
		MsgTypeResvTear: {sessionObj, hopObj, styleObj, flowObj, filterObj},
		MsgTypeResvErr:  {sessionObj, hopObj, errObj, styleObj, flowObj, filterObj},
	}
}

// constructionCases names, for each RSVP message type other than PATH and
// RESV, the objects RFC 2205 Section 3.1 writes unbracketed in its BNF.
func constructionCases() []struct {
	name     string
	msgType  uint8
	objects  []namedObject
	required []string
} {
	conformant, reservation := conformantMessages(), reservationMessages()
	return []struct {
		name     string
		msgType  uint8
		objects  []namedObject
		required []string
	}{
		{"PathTear", MsgTypePathTear, conformant[MsgTypePathTear], []string{"SESSION", "RSVP_HOP"}},
		{"ResvTear", MsgTypeResvTear, reservation[MsgTypeResvTear], []string{"SESSION", "RSVP_HOP", "STYLE"}},
		{"PathErr", MsgTypePathErr, conformant[MsgTypePathErr], []string{"SESSION", "ERROR_SPEC"}},
		{"ResvErr", MsgTypeResvErr, reservation[MsgTypeResvErr], []string{"SESSION", "RSVP_HOP", "ERROR_SPEC", "STYLE"}},
		{"ResvConf", MsgTypeResvConf, conformant[MsgTypeResvConf], []string{"SESSION", "ERROR_SPEC", "RESV_CONFIRM", "STYLE"}},
	}
}

// constructionTransit returns a transit holding path state for the session and
// sender that conformantMessages and reservationMessages carry, so a message
// that passed the construction check would act on that state.
func constructionTransit(t *testing.T) (*engine, *fakeTransport) {
	t.Helper()
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)
	psb := rfc2205PSB()
	psb.Session.TunnelID = 1
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(psb, rfc2205Ingress, defaultIPTTL)})
	require.Len(t, e.table.All(), 1, "transit installed path state")
	return e, ft
}

// RFC requirement: RFC2205-4-4 positive -- a conformant PathTear, ResvTear, PathErr, ResvErr and ResvConf each pass DecodeMessage's construction check and decode as their own message type.
func TestRFC2205EachMessageTypeConstructionAccepted(t *testing.T) {
	for _, tc := range constructionCases() {
		msg, err := DecodeMessage(encodeWithout(tc.msgType, tc.objects, ""))
		require.NoError(t, err, "a conformant %s decodes", tc.name)
		assert.Equal(t, tc.msgType, msg.Header.MsgType, tc.name)
	}
}

// RFC requirement: RFC2205-4-4 negative -- a PathTear, ResvTear, PathErr, ResvErr or ResvConf missing any one object its BNF writes unbracketed is refused by DecodeMessage (with errObjectAbsent for every object but STYLE), and a transit holding path state for that message's session and sender sends nothing in answer and keeps its state.
func TestRFC2205EachMessageTypeMalformedRefused(t *testing.T) {
	for _, tc := range constructionCases() {
		for _, omit := range tc.required {
			raw := encodeWithout(tc.msgType, tc.objects, omit)
			_, err := DecodeMessage(raw)
			require.Error(t, err, "%s without %s", tc.name, omit)
			// Without STYLE the flow descriptor is misplaced, and the placement
			// check refuses it before the presence check runs.
			if omit != "STYLE" {
				assert.True(t, errors.Is(err, errObjectAbsent), "%s without %s: got %v", tc.name, omit, err)
			}

			e, ft := constructionTransit(t)
			before := sentCount(ft)
			e.handlePacket(Packet{Src: rfc2205Egress, Payload: raw})
			assert.Equal(t, before, sentCount(ft), "%s without %s: nothing sent", tc.name, omit)
			assert.Len(t, e.table.All(), 1, "%s without %s: path state untouched", tc.name, omit)
		}
	}
}
