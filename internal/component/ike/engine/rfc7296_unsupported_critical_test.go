// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- error notifications for a malformed request
// Related: rfc7296_inner_error_answers_test.go -- errLink, errInnerChainRequest and errInnerAnswer

package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ucritPayload is an unrecognized payload of four body octets with the critical bit set
// or clear, as the only payload of a protected request's inner chain.
func ucritPayload(critical bool) []byte {
	flags := byte(0)
	if critical {
		flags = 0x80
	}
	return []byte{0, flags, 0, 8, 0xde, 0xad, 0xbe, 0xef}
}

// VALIDATES: the response to a request carrying an unsupported critical payload includes
// an UNSUPPORTED_CRITICAL_PAYLOAD notify whose Notification Data is that payload's
// one-octet type.
// PREVENTS: a response that names a fixed or wrong type, or that carries the notify when
// the payload was not critical.
//
// METHOD: protected INFORMATIONAL requests from the initiator, each with one unrecognized
// inner payload. Two different unrecognized types (200 and 49) with the critical bit set
// must each draw the notify naming that type, so the data follows the payload. The same
// payload with the critical bit clear must draw a response with no such notify.
//
// RFC requirement: RFC7296-2.5-18 positive -- the response to a request whose inner chain
// carries an unrecognized payload of type 200, and one of type 49, with the critical bit
// set includes exactly one notify, UNSUPPORTED_CRITICAL_PAYLOAD, whose Notification Data is
// the one octet 200, respectively 49.
func TestRFC7296UnsupportedCriticalResponseNamesThePayloadType(t *testing.T) {
	for _, unknownType := range []uint8{200, 49} {
		link := errLink(t)
		req := errInnerChainRequest(t, link.ini, link.resp.ExpectedMsgID, unknownType, ucritPayload(true))
		notifies := errInnerAnswer(t, link, req)
		if len(notifies) != 1 {
			t.Fatalf("type %d: the response carries %d notifies, want exactly one", unknownType, len(notifies))
		}
		if notifies[0].NotifyMsgType != wire.NotifyUnsupportedCriticalPayload {
			t.Errorf("type %d: the response carries notify %d, want UNSUPPORTED_CRITICAL_PAYLOAD (%d)",
				unknownType, notifies[0].NotifyMsgType, wire.NotifyUnsupportedCriticalPayload)
		}
		if !bytes.Equal(notifies[0].NotificationData, []byte{unknownType}) {
			t.Errorf("type %d: notification data = %x, want the one octet %02x",
				unknownType, notifies[0].NotificationData, unknownType)
		}
	}
}

// RFC requirement: RFC7296-2.5-18 negative -- the same unrecognized type-200 payload with
// the critical bit CLEAR is not an unsupported critical payload: the request is answered,
// and the response carries no UNSUPPORTED_CRITICAL_PAYLOAD notify.
func TestRFC7296NonCriticalUnknownPayloadDrawsNoUnsupportedCritical(t *testing.T) {
	link := errLink(t)
	req := errInnerChainRequest(t, link.ini, link.resp.ExpectedMsgID, 200, ucritPayload(false))
	link.ps.handleOwnedInbound(link.resp, transport.Packet{Data: req}, link.myTr, nil, slogutil.DiscardLogger())
	got := rtxRecv(t, link.peerTr)
	if got == nil {
		t.Fatal("the request with a non-critical unknown payload drew no response")
	}
	inner, err := decryptAndParse(link.ini, parseMsg(t, got), got)
	if err != nil {
		t.Fatalf("the response does not authenticate under the IKE SA: %v", err)
	}
	for i := range inner {
		n, ok := inner[i].Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		if n.NotifyMsgType == wire.NotifyUnsupportedCriticalPayload {
			t.Errorf("a non-critical unknown payload drew UNSUPPORTED_CRITICAL_PAYLOAD (data %x)",
				n.NotificationData)
		}
	}
}
