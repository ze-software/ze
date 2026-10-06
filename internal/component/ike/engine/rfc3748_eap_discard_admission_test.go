// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP discard wiring.
package engine

import (
	"bytes"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestEngineDiscardedEAPCodesPreserveExchange drives both IKE receive entry
// points with encrypted wrong-role and undefined Codes before every EAP round.
// The real held-back packet must still complete that round at the same Message
// ID, and the conversation must finish with matching nonzero MSKs. This is an
// engine boundary test, not interoperability with an independent implementation.
// RFC 3748 Section 4: "Since EAP only defines Codes 1-4, EAP packets with other codes
// MUST be silently discarded by both authenticators and peers."
// RFC requirement: RFC3748-4-5 positive -- encrypted undefined Codes 0, 5 and 255
// reach both IKE EAP roles without changing SA state, retransmission deadline/count,
// outbound message ID or last sent message, and neither role sends a UDP response.
// RFC requirement: RFC3748-4-5 negative -- held-back valid EAP packets at the same
// IKE message IDs advance both roles and send over UDP after more than twenty
// ignored packets per round; the exchange derives matching nonzero MSKs and
// produces an initiator AUTH that the responder verifies.
// MUTATION: remove the result.Discarded return in fsm.go::handleEAPResponse;
// the unchanged deadline/retry assertions must fail before any valid control.
// MUTATION: increment ps.rounds in the undefined-Code arm of core/eap/peer.go;
// the held-back valid packet must then fail instead of completing the exchange.
func TestEngineDiscardedEAPCodesPreserveExchange(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, table, ps, authReq := eaplenExchange(t)
	peerTr, myTr := rtxPeerLink(t, resp, ini)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	peerAddr := eaprtxPeerAddr(t, peerTr)
	ps.handleResponderInbound(resp, parseMsg(t, authReq), transport.Packet{Data: authReq, RemoteAddr: peerAddr}, myTr, log)
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("initial IKE_AUTH produced no EAP Request")
	}
	// RFC 7296 Section 2.16: the first authenticated EAP Request starts the peer.
	handleInbound(ini, transport.Packet{Data: first}, table, myTr, log)
	if sent := rtxRecv(t, peerTr); !bytes.Equal(sent, ini.LastSentMsg) {
		t.Fatal("initial EAP Response was not sent over UDP")
	}
	if ini.State != StateEAPInProgress {
		t.Fatalf("initiator did not start EAP: %v", ini.State)
	}

	// Twenty-one ignored packets exceed the EAP peer's twenty-round budget.
	for range 10 {
		request := ini.LastSentMsg
		lastReply := resp.LastSentMsg
		expectedID := resp.ExpectedMsgID
		resp.RetransmitTime = time.Now().Add(time.Minute)
		resp.RetransmitCount = 2
		respTimer, respRetries, respID := resp.RetransmitTime, resp.RetransmitCount, resp.NextMsgID
		for range 21 {
			for _, code := range []uint8{0, 5, 255, eap.CodeRequest, eap.CodeSuccess, eap.CodeFailure} {
				// RFC 3748 Sections 2.2, 2.3 and 4: wrong-role and undefined Codes.
				raw := eapAdmissionMessage(t, ini, code, eaplenMessageID(request), initiatorFlag(ini))
				ps.handleResponderInbound(resp, parseMsg(t, raw), transport.Packet{Data: raw, RemoteAddr: peerAddr}, myTr, log)
				if resp.State != StateEAPInProgress || resp.ExpectedMsgID != expectedID {
					t.Fatalf("Code %d advanced or killed responder: state=%v messageID=%d", code, resp.State, resp.ExpectedMsgID)
				}
				if !bytes.Equal(resp.LastSentMsg, lastReply) {
					t.Fatalf("responder answered discarded Code %d", code)
				}
				if resp.RetransmitTime != respTimer || resp.RetransmitCount != respRetries || resp.NextMsgID != respID {
					t.Fatalf("Code %d changed responder retransmission or outbound message state", code)
				}
			}
		}
		rtxExpectSilence(t, peerTr, myTr, peerAddr, "discarded EAP Codes")
		// RFC 3748 Section 4: the legitimate Response still reaches the method.
		ps.handleResponderInbound(resp, parseMsg(t, request), transport.Packet{Data: request, RemoteAddr: peerAddr}, myTr, log)
		reply := rtxRecv(t, peerTr)
		if reply == nil {
			t.Fatal("legitimate EAP response produced no IKE_AUTH reply")
		}
		if resp.ExpectedMsgID != expectedID+1 || bytes.Equal(resp.LastSentMsg, lastReply) {
			t.Fatal("valid EAP Response did not advance the responder exchange")
		}
		ini.RetransmitTime = time.Now().Add(time.Minute)
		ini.RetransmitCount = 2
		id := ini.NextMsgID
		timer, retries := ini.RetransmitTime, ini.RetransmitCount
		for range 21 {
			for _, code := range []uint8{0, 5, 255, eap.CodeResponse} {
				// RFC 3748 Sections 2.2, 2.3 and 4: admission precedes the round cap.
				raw := eapAdmissionMessage(t, resp, code, eaplenMessageID(reply), wire.FlagResponse)
				handleInbound(ini, transport.Packet{Data: raw}, table, myTr, log)
				if ini.State != StateEAPInProgress || ini.NextMsgID != id {
					t.Fatalf("Code %d advanced or killed initiator: state=%v messageID=%d", code, ini.State, ini.NextMsgID)
				}
				if !bytes.Equal(ini.LastSentMsg, request) || ini.RetransmitTime != timer || ini.RetransmitCount != retries {
					t.Fatalf("Code %d changed initiator output or retransmission state", code)
				}
			}
		}
		rtxExpectSilence(t, peerTr, myTr, peerAddr, "peer discarded EAP Codes")
		// RFC 3748 Section 4: a legitimate Request or Success still proceeds.
		handleInbound(ini, transport.Packet{Data: reply}, table, myTr, log)
		if sent := rtxRecv(t, peerTr); !bytes.Equal(sent, ini.LastSentMsg) {
			t.Fatal("valid EAP control did not send its next IKE_AUTH over UDP")
		}
		if ini.NextMsgID != id+1 || bytes.Equal(ini.LastSentMsg, request) {
			t.Fatal("valid EAP control did not advance the initiator exchange")
		}
		if ini.RetransmitTime.Equal(timer) || ini.RetransmitCount != 0 {
			t.Fatal("valid EAP control did not renew the retransmission budget")
		}
		if ini.State == StateAuthSent {
			if ini.EAPMSK == ([64]byte{}) || ini.EAPMSK != resp.EAPMSK {
				t.Fatal("EAP did not finish with matching nonzero MSKs")
			}
			inner, err := decryptAndParse(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range inner {
				if auth, ok := entry.Payload.(*wire.PayloadAUTH); ok {
					// RFC 7296 Section 2.16: prove the completed EAP key authenticates IKE.
					if err := verifyRemoteAuth(resp, auth); err != nil {
						t.Fatalf("final AUTH after discarded packets: %v", err)
					}
					return
				}
			}
			t.Fatal("completed EAP exchange did not produce final IKE AUTH")
		}
		if ini.State != StateEAPInProgress || resp.State != StateEAPInProgress {
			t.Fatalf("legitimate EAP round failed: initiator=%v responder=%v", ini.State, resp.State)
		}
	}
	t.Fatal("EAP failed to finish within ten valid rounds")
}

// eapAdmissionMessage seals an EAP packet without making its invalid Code valid.
// RFC 3748 Section 4: "The Code field is one octet and identifies the Type of EAP packet."
// Wire offsets within EAP: Code(0), Identifier(1), Length(2:4), Type(4).
func eapAdmissionMessage(t *testing.T, sender *SA, code uint8, id uint32, flags uint8) []byte {
	t.Helper()
	packet := &eap.Packet{Code: code, Identifier: 1, Type: eap.TypeIdentity}
	// RFC 7296 Section 2.16: EAP is carried inside the protected IKE_AUTH payload.
	raw, err := buildEncryptedMessageEx(sender, []wire.PayloadEntry{{Payload: eapToWire(packet)}}, id, wire.ExchangeIKEAuth, flags)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
