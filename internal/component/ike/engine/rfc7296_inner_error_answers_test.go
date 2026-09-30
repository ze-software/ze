// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- error notifications for a malformed request
// Related: rfc7296_notify_error_test.go -- the truncated-chain case and the outer-parse silence
// Related: notify_error_fixtures_test.go -- errLink, errNotifyIn and the SK request builders
//
// VALIDATES: RFC 7296 Sections 2.21.2 and 3.10.1. A protected request whose inner chain
// carries an unsupported critical payload draws exactly UNSUPPORTED_CRITICAL_PAYLOAD, and
// every other malformation of the chain draws exactly INVALID_SYNTAX.
// PREVENTS: a malformed request being half-processed, or answered with the wrong status.

package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// errInnerChainRequest seals inner as the plaintext of an SK payload whose first inner
// payload type is firstType, in an INFORMATIONAL request the initiator sends. The SK
// payload decrypts and passes its integrity check; only the chain inside may be wrong.
func errInnerChainRequest(t *testing.T, ini *SA, msgID uint32, firstType uint8, inner []byte) []byte {
	t.Helper()
	var raw []byte
	var err error
	if ini.Proposal.Encryption.IsAEAD {
		raw, err = buildSKMessageAEADWithMsgID(ini, inner, firstType, msgID,
			wire.ExchangeInformational, initiatorFlag(ini))
	} else {
		raw, err = buildSKMessageCBCWithMsgID(ini, inner, firstType, msgID,
			wire.ExchangeInformational, initiatorFlag(ini))
	}
	if err != nil {
		t.Fatalf("build the encrypted request: %v", err)
	}
	return raw
}

// errInnerAnswer delivers req to the responder and returns the notifies of its answer,
// failing when the request reached the exchange handlers or drew no answer.
func errInnerAnswer(t *testing.T, link errPair, req []byte) []*wire.PayloadNotify {
	t.Helper()
	out := link.ps.handleOwnedInbound(link.resp, transport.Packet{Data: req}, link.myTr, nil, slogutil.DiscardLogger())
	if out.peerAlive {
		t.Error("a malformed request reached the exchange handlers instead of being rejected whole")
	}
	got := rtxRecv(t, link.peerTr)
	if got == nil {
		t.Fatal("the malformed request drew no answer")
	}
	inner, err := decryptAndParse(link.ini, parseMsg(t, got), got)
	if err != nil {
		t.Fatalf("the answer does not authenticate under the IKE SA: %v", err)
	}
	var notifies []*wire.PayloadNotify
	for i := range inner {
		n, ok := inner[i].Payload.(*wire.PayloadNotify)
		if !ok {
			t.Errorf("the answer carries a %T beside its notify", inner[i].Payload)
			continue
		}
		notifies = append(notifies, n)
	}
	return notifies
}

// TestRFC7296UnsupportedCriticalPayloadDrawsOnlyThatNotify proves the critical-payload
// half of Section 2.21.2.
//
// RFC 7296 Section 2.21.2: "Note, however, that request messages that contain an
// unsupported critical payload, or where the whole message is malformed (rather than just
// bad payload contents), MUST be rejected in their entirety, and MUST only lead to an
// UNSUPPORTED_CRITICAL_PAYLOAD or INVALID_SYNTAX Notification sent as a response."
//
// Method: a protected INFORMATIONAL request whose only inner payload is private-use type
// 200 with the critical bit set. The request does not reach the exchange handlers, and
// the answer carries exactly one notify: UNSUPPORTED_CRITICAL_PAYLOAD whose data is the
// one-octet type (Section 2.5). The negative (bad payload contents are not a malformed
// message) is TestCritChainReportsTruncationButNotBadContents in ike/wire.
func TestRFC7296UnsupportedCriticalPayloadDrawsOnlyThatNotify(t *testing.T) {
	link := errLink(t)
	const unknownType = 200
	// Generic header: Next Payload 0, Critical bit set, Payload Length 8, then 4 octets.
	inner := []byte{0, 0x80, 0, 8, 0xde, 0xad, 0xbe, 0xef}
	req := errInnerChainRequest(t, link.ini, link.resp.ExpectedMsgID, unknownType, inner)

	// RFC requirement: RFC7296-2.21.2-1 positive -- a request carrying an unsupported
	// critical payload is rejected in its entirety and answered with exactly one
	// UNSUPPORTED_CRITICAL_PAYLOAD notify naming the payload type, and nothing else.
	notifies := errInnerAnswer(t, link, req)
	if len(notifies) != 1 {
		t.Fatalf("the answer carries %d notifies, want exactly one", len(notifies))
	}
	if notifies[0].NotifyMsgType != wire.NotifyUnsupportedCriticalPayload {
		t.Errorf("the answer carries notify %d, want UNSUPPORTED_CRITICAL_PAYLOAD (%d)",
			notifies[0].NotifyMsgType, wire.NotifyUnsupportedCriticalPayload)
	}
	if !bytes.Equal(notifies[0].NotificationData, []byte{unknownType}) {
		t.Errorf("notification data = %x, want the one-octet payload type %x",
			notifies[0].NotificationData, unknownType)
	}
}

// TestRFC7296UncoveredInnerErrorsDrawInvalidSyntax widens Section 3.10.1 beyond the
// truncated chain.
//
// RFC 7296 Section 3.10.1: "To avoid leaking information to someone probing a node, this
// status MUST be sent in response to any error not covered by one of the other status
// types."
//
// Method: two further chain errors no other status type covers, each in a protected
// request whose integrity check passes: a Payload Length of zero and a Payload Length of
// three, both below the four-octet generic header. Each draws exactly one notify,
// INVALID_SYNTAX, and the request does not reach the exchange handlers.
func TestRFC7296UncoveredInnerErrorsDrawInvalidSyntax(t *testing.T) {
	cases := []struct {
		name  string
		inner []byte
	}{
		{"payload length zero", []byte{0, 0, 0, 0, 1, 2, 3, 4}},
		{"payload length below the generic header", []byte{0, 0, 0, 3, 1, 2, 3, 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			link := errLink(t)
			req := errInnerChainRequest(t, link.ini, link.resp.ExpectedMsgID, wire.PayloadTypeNonce, tc.inner)

			// RFC requirement: RFC7296-3.10.1-3 positive -- a Payload Length of zero or
			// three in a protected request, an error no other status type covers, draws
			// exactly INVALID_SYNTAX.
			notifies := errInnerAnswer(t, link, req)
			if len(notifies) != 1 {
				t.Fatalf("the answer carries %d notifies, want exactly one", len(notifies))
			}
			if notifies[0].NotifyMsgType != wire.NotifyInvalidSyntax {
				t.Errorf("the answer carries notify %d, want INVALID_SYNTAX (%d)",
					notifies[0].NotifyMsgType, wire.NotifyInvalidSyntax)
			}
		})
	}
}
