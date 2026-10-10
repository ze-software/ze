package engine

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// crfRefusal says how crfAuthRefusingChild rewrites the Child SA half of the response.
type crfRefusal uint8

const (
	crfRefusalUnspecified crfRefusal = iota
	// crfRefusalNotify puts NO_PROPOSAL_CHOSEN where SAr2, TSi and TSr stood.
	crfRefusalNotify
	// crfRefusalSilent drops SAr2, TSi and TSr and puts nothing in their place.
	crfRefusalSilent
	// crfRefusalBadAUTH is crfRefusalNotify with one AUTH octet flipped.
	crfRefusalBadAUTH
)

// crfAuthRefusingChild runs a real PSK handshake whose IKE_AUTH response authenticates
// the responder and carries no Child SA, and hands that response to the initiator. It
// returns the initiator SA, the responder SA, and the two ends of the loopback link:
// ze sends from myTr, and peerTr receives.
//
// The refusal is the shape RFC 7296 Section 2.21.2 describes: "a responder may include
// all the payloads associated with authentication (IDr, CERT, and AUTH) while sending
// error notifications for the piggybacked exchanges (FAILED_CP_REQUIRED,
// NO_PROPOSAL_CHOSEN, and so on)". strongSwan's responder answers that way
// (child_create.c). The test takes ze's real, authenticated response and rewrites only
// the Child SA half. IDr and AUTH are untouched, and AUTH covers neither the Child SA
// payloads nor the notify (Section 2.15), so the response still verifies.
func crfAuthRefusingChild(t *testing.T, refusal crfRefusal, log *slog.Logger) (ini, resp *SA, peerTr, myTr *transport.UDPTransport) {
	t.Helper()
	quiet := slogutil.DiscardLogger()
	peerTr, myTr = rtxPeerLink(t)

	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "child-refusal-psk")
	// rtxPeerLink points remoteUDPAddr at peerTr, so anything sent has somewhere to land.
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err = newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, quiet)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, quiet)

	authMsgID := parseMsg(t, ini.LastSentMsg).Header.MessageID
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, quiet)
	if resp.State != StateEstablished {
		t.Fatalf("the responder did not establish (state %v), so there is no response to rewrite", resp.State)
	}

	accepted := lcyDecrypt(t, ini, resp.LastSentMsg)
	rewritten := make([]wire.PayloadEntry, 0, len(accepted)+1)
	for i := range accepted {
		switch p := accepted[i].Payload.(type) {
		case *wire.PayloadSA, *wire.PayloadTS:
			continue
		case *wire.PayloadAUTH:
			if refusal == crfRefusalBadAUTH {
				bad := &wire.PayloadAUTH{AuthMethod: p.AuthMethod, AuthData: bytes.Clone(p.AuthData)}
				bad.AuthData[0] ^= 0xff
				rewritten = append(rewritten, wire.PayloadEntry{Payload: bad})
				continue
			}
		}
		rewritten = append(rewritten, accepted[i])
	}
	if refusal != crfRefusalSilent {
		rewritten = append(rewritten, wire.PayloadEntry{Payload: &wire.PayloadNotify{
			NotifyMsgType: wire.NotifyNoProposalChosen,
		}})
	}
	answer, err := buildEncryptedMessageEx(resp, rewritten, authMsgID, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		t.Fatalf("building the rewritten IKE_AUTH response: %v", err)
	}

	handleAuthResponse(ini, parseMsg(t, answer), answer, table, myTr, log)
	return ini, resp, peerTr, myTr
}

// VALIDATES: AC-2. An initiator whose IKE_AUTH response authenticates the responder and
// refuses the Child SA keeps the authentication and the IKE SA: the SA establishes, it is
// marked childless for runEstablished, nothing is sent (no Delete), and the log names the
// refusal. The negative: the same response with a corrupted AUTH fails, so the outcome
// is decided by AUTH and not by the notify.
//
// RFC 7296 Section 2.21.2: "the initiator MUST NOT fail the authentication because of
// this." The f0006faaa2 delete-and-back-off this replaces is gone (owner decision Q-1).
func TestInitiatorKeepsChildlessIKESA(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ini, _, peerTr, myTr := crfAuthRefusingChild(t, crfRefusalNotify, log)

	if ini.State != StateEstablished {
		t.Fatalf("the initiator left the authenticated SA in state %v, want StateEstablished", ini.State)
	}
	if !ini.IKEAuthChildless {
		t.Error("the SA is not marked childless, so runEstablished would build a Child SA from no SAr2")
	}
	if logged := buf.String(); !strings.Contains(logged, "NO_PROPOSAL_CHOSEN") {
		t.Errorf("the initiator log does not name the peer's refusal (NO_PROPOSAL_CHOSEN):\n%s", logged)
	}
	rtxExpectSilence(t, peerTr, myTr, ini.remoteUDPAddr(), "after a childless IKE_AUTH response")

	bad, _, _, _ := crfAuthRefusingChild(t, crfRefusalBadAUTH, slogutil.DiscardLogger())
	if bad.State != StateDead {
		t.Errorf("a corrupted AUTH beside the refusal left the SA in state %v, want StateDead", bad.State)
	}
}

// VALIDATES: AC-3. An authenticated IKE_AUTH response with neither SAr2 nor an error
// notify establishes a childless IKE SA, as a refusal does, instead of failing later on
// "no peer ESP SPI recorded" with no Delete sent (owner decision Q-3).
func TestInitiatorNoSAr2NoNotifyIsChildless(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ini, _, _, _ := crfAuthRefusingChild(t, crfRefusalSilent, log)
	if ini.State != StateEstablished {
		t.Fatalf("state %v, want StateEstablished", ini.State)
	}
	if !ini.IKEAuthChildless {
		t.Error("a response with no SAr2 and no notify is not marked childless")
	}
	if logged := buf.String(); !strings.Contains(logged, "no Child SA") {
		t.Errorf("the log does not say the response carried no Child SA:\n%s", logged)
	}
}

// VALIDATES: an IKE_AUTH response that ACCEPTS the Child SA is not childless, so
// runEstablished builds the Child SA. The discriminator for the two tests above.
func TestAuthChildAcceptedIsNotChildless(t *testing.T) {
	ini, _, _ := establishPSK(t)
	if ini.IKEAuthChildless {
		t.Error("an accepted Child SA marked the SA childless")
	}
}

// VALIDATES: AC-1 on the PSK path. A responder whose initiator authenticates but whose
// ESP proposal it refuses answers IDr, AUTH and NO_PROPOSAL_CHOSEN, in that order, with
// no SAr2, TSi or TSr; establishes the IKE SA; installs no Child SA. The initiator then
// establishes a childless IKE SA from that answer.
//
// RFC 7296 Section 2.21.2: "a responder may include all the payloads associated with
// authentication (IDr, CERT, and AUTH) while sending error notifications for the
// piggybacked exchanges". Before, ze answered the notify ALONE and killed the SA.
func TestResponderKeepsIKESAWhenChildRefused(t *testing.T) {
	quiet := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	iniESP := testESPGroup()
	respESP := testESPGroup()
	respESP.Proposals[0].Encryption = ipsec.EncryptionAES128
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "child-refused-psk")

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, iniESP)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent
	resp, err := newResponderSA("ze", respPeer, ikeGroup, respESP, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, quiet)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, quiet)

	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: respESP}
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, quiet)
	if resp.State != StateEstablished {
		t.Fatalf("the responder left the authenticated SA in state %v, want StateEstablished", resp.State)
	}
	if ps.getChildSA() != nil {
		t.Error("the responder installed a Child SA it refused")
	}

	inner := lcyDecrypt(t, ini, resp.LastSentMsg)
	var order []uint8
	for i := range inner {
		switch p := inner[i].Payload.(type) {
		case *wire.PayloadSA, *wire.PayloadTS:
			t.Errorf("the childless response carries payload type %d", p.Type())
		case *wire.PayloadNotify:
			if p.NotifyMsgType != wire.NotifyNoProposalChosen {
				t.Errorf("the response carries notify %s, want NO_PROPOSAL_CHOSEN",
					wire.NotifyTypeName(p.NotifyMsgType))
			}
		}
		order = append(order, inner[i].Payload.Type())
	}
	want := []uint8{wire.PayloadTypeIDr, wire.PayloadTypeAUTH, wire.PayloadTypeNotify}
	if !bytes.Equal(order, want) {
		t.Errorf("payload order %v, want IDr, AUTH, N %v", order, want)
	}

	handleAuthResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, quiet)
	if ini.State != StateEstablished || !ini.IKEAuthChildless {
		t.Errorf("the initiator reached state %v childless=%v, want an established childless SA",
			ini.State, ini.IKEAuthChildless)
	}
}
