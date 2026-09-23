// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- a message missing a mandatory object is dropped
//
// VALIDATES: checkMandatoryObjects (mandatory.go) against the RFC 2205 Section
// 3.1 message BNFs. Each case encodes a conformant message, then the same
// message with one unbracketed object removed, so the accept and the refuse are
// driven by a one-object difference and nothing else.
package rsvpte

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// namedObject is one encodable object of a message, named so a case can drop it.
type namedObject struct {
	name   string
	encode objEncoder
}

// encodeWithout encodes msgType from objects, leaving out the one called omit.
// An empty omit encodes the conformant message.
func encodeWithout(msgType uint8, objects []namedObject, omit string) []byte {
	encoders := make([]objEncoder, 0, len(objects))
	for _, obj := range objects {
		if obj.name == omit {
			continue
		}
		encoders = append(encoders, obj.encode)
	}
	return encodeMessage(msgType, defaultIPTTL, encoders)
}

// conformantMessages returns, for each message type ze processes, the objects a
// conformant message carries, in the RFC 2205 Section 3.1 BNF order.
func conformantMessages() map[uint8][]namedObject {
	session := sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: 0x0a000001}
	hop := rsvpHop{NextHop: netip.MustParseAddr("10.0.0.1")}
	tv := timeValues{RefreshPeriod: 300000}
	sender := senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1}
	tspec := FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8}
	es := errorSpec{ErrorNode: netip.MustParseAddr("10.0.0.5"), ErrorCode: ErrCodeRoutingProblem, ErrorValue: ErrValueNoRouteAvailable}

	sessionObj := namedObject{"SESSION", func(b []byte) int { return encodeSessionIPv4(b, session) }}
	hopObj := namedObject{"RSVP_HOP", func(b []byte) int { return encodeRSVPHop(b, hop) }}
	timeObj := namedObject{"TIME_VALUES", func(b []byte) int { return encodeTimeValues(b, tv) }}
	styleObj := namedObject{"STYLE", func(b []byte) int { return encodeStyle(b, StyleSharedExplicit) }}
	errObj := namedObject{"ERROR_SPEC", func(b []byte) int { return encodeErrorSpec(b, es) }}
	senderObj := namedObject{"SENDER_TEMPLATE", func(b []byte) int { return encodeSenderTemplate(b, sender) }}
	tspecObj := namedObject{"SENDER_TSPEC", func(b []byte) int { return encodeFlowSpec(b, ClassSenderTSpec, tspec) }}
	labelReqObj := namedObject{"LABEL_REQUEST", func(b []byte) int { return encodeLabelRequest(b, labelRequest{L3PID: 0x0800}) }}
	flowObj := namedObject{"FLOWSPEC", func(b []byte) int { return encodeFlowSpec(b, ClassFlowSpec, tspec) }}
	labelObj := namedObject{"LABEL", func(b []byte) int { return encodeLabelObject(b, labelObject{Label: 16050}) }}
	filterObj := namedObject{"FILTER_SPEC", func(b []byte) int { return encodeFilterSpec(b, sender) }}
	confirmObj := namedObject{"RESV_CONFIRM", func(b []byte) int { return encodeResvConfirm(b, session.TunnelEndpoint) }}
	confirmedObj := namedObject{"ERROR_SPEC", func(b []byte) int { return encodeErrorSpec(b, errorSpec{ErrorNode: es.ErrorNode}) }}

	return map[uint8][]namedObject{
		MsgTypePath:     {sessionObj, hopObj, timeObj, labelReqObj, senderObj, tspecObj},
		MsgTypeResv:     {sessionObj, hopObj, timeObj, styleObj, flowObj, filterObj, labelObj},
		MsgTypePathTear: {sessionObj, hopObj, senderObj, tspecObj},
		MsgTypePathErr:  {sessionObj, errObj, senderObj, tspecObj},
		MsgTypeResvConf: {sessionObj, confirmedObj, confirmObj, styleObj, flowObj, filterObj},
	}
}

// TestDecodeMandatoryObjects drives DecodeMessage with each message type ze
// processes, first conformant and then with one mandatory object removed.
// MUTATION: omit STYLE from checkMandatoryObjects and allow reservation
// descriptors before STYLE in checkObjectPlacement; no-STYLE must still reject.
func TestDecodeMandatoryObjects(t *testing.T) {
	cases := []struct {
		name      string
		msgType   uint8
		mandatory []string
	}{
		{"PATH", MsgTypePath, []string{"SESSION", "RSVP_HOP", "TIME_VALUES"}},
		{"RESV", MsgTypeResv, []string{"SESSION", "RSVP_HOP", "TIME_VALUES", "STYLE"}},
		{"PathTear", MsgTypePathTear, []string{"SESSION", "RSVP_HOP"}},
		{"PathErr", MsgTypePathErr, []string{"SESSION", "ERROR_SPEC"}},
		{"ResvConf", MsgTypeResvConf, []string{"SESSION", "ERROR_SPEC", "RESV_CONFIRM", "STYLE"}},
	}

	objects := conformantMessages()
	for _, tc := range cases {
		t.Run(tc.name+"/complete", func(t *testing.T) {
			// RFC requirement: RFC2205-3.1.3-1 positive -- a Path, Resv, PathTear, PathErr or ResvConf carrying every object its RFC 2205 Section 3.1 BNF writes unbracketed is accepted by DecodeMessage (checkMandatoryObjects, mandatory.go).
			msg, err := DecodeMessage(encodeWithout(tc.msgType, objects[tc.msgType], ""))
			require.NoError(t, err, "a conformant %s decodes", tc.name)
			assert.Equal(t, tc.msgType, msg.Header.MsgType)
		})

		for _, omit := range tc.mandatory {
			t.Run(tc.name+"/no-"+omit, func(t *testing.T) {
				// RFC requirement: RFC2205-3.1.3-1 negative -- the same message with one required object removed is refused by DecodeMessage; no-STYLE rejection does not depend on which construction check runs first.
				_, err := DecodeMessage(encodeWithout(tc.msgType, objects[tc.msgType], omit))
				require.Error(t, err, "a %s without %s is malformed", tc.name, omit)
				if omit != "STYLE" {
					assert.True(t, errors.Is(err, errObjectAbsent), "error is errObjectAbsent, got %v", err)
				}
			})
		}
	}
}

// TestEnginePathWithoutTimeValuesDropped is the reason the check exists. A PATH
// that omits TIME_VALUES used to be accepted, and receivedRefreshPeriod then gave
// it the 30-second default while its sender refreshed every 300 seconds, so ze
// expired the state between two of that sender's refreshes.
func TestEnginePathWithoutTimeValuesDropped(t *testing.T) {
	e, ft, fib := testEngine(t, "10.0.0.9", nil)

	ingress := netip.MustParseAddr("10.0.0.1")
	objects := conformantMessages()
	e.handlePacket(Packet{Src: ingress, Payload: encodeWithout(MsgTypePath, objects[MsgTypePath], "TIME_VALUES")})

	// RFC requirement: RFC2205-3.1.3-1 negative -- a PATH with no TIME_VALUES reaching handlePacket (engine.go) builds no LSP state, so the local default period never sets the lifetime of state a sender refreshes on its own period.
	assert.Zero(t, e.table.Len(), "no LSP state is created from a malformed PATH")
	assert.Empty(t, fib.popped, "no label is programmed")

	// RFC requirement: RFC2205-3.1.3-1 negative -- nothing answers the malformed PATH on the wire: RFC 2205 Appendix B logs the error locally instead of reporting it in an ERROR_SPEC, so handlePacket sends neither a RESV nor a PathErr.
	_, _, sentResv := ft.lastByType(MsgTypeResv)
	assert.False(t, sentResv, "no RESV answers a malformed PATH")
	_, _, sentErr := ft.lastByType(MsgTypePathErr)
	assert.False(t, sentErr, "no PathErr answers a malformed PATH")
}

// TestEnginePathWithTimeValuesAccepted is the same engine path with the object
// present, so the drop above is attributable to TIME_VALUES and nothing else.
func TestEnginePathWithTimeValuesAccepted(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.9", nil)
	ingress := netip.MustParseAddr("10.0.0.1")
	objects := conformantMessages()
	e.handlePacket(Packet{Src: ingress, Payload: encodeWithout(MsgTypePath, objects[MsgTypePath], "")})

	// RFC requirement: RFC2205-3.1.3-1 positive -- the same PATH carrying TIME_VALUES is processed: the egress builds LSP state and answers with a RESV (handlePathEgress, engine.go).
	assert.Equal(t, 1, e.table.Len(), "the conformant PATH creates LSP state")
	_, _, sentResv := ft.lastByType(MsgTypeResv)
	assert.True(t, sentResv, "the conformant PATH is answered with a RESV")
}
