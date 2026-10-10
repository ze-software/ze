package engine

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// crfAuthRefusingChild runs a real PSK handshake whose IKE_AUTH response authenticates
// the responder and refuses the piggybacked Child SA, and hands that response to the
// initiator. It returns the initiator SA and session, the responder SA and session, and
// the two ends of the loopback link: ze sends from myTr, and peerTr receives.
//
// The refusal is the shape RFC 7296 Section 2.21.2 describes: "a responder may include
// all the payloads associated with authentication (IDr, CERT, and AUTH) while sending
// error notifications for the piggybacked exchanges (FAILED_CP_REQUIRED,
// NO_PROPOSAL_CHOSEN, and so on)". strongSwan's responder answers that way
// (child_create.c). Ze's own responder answers a refused Child SA differently, so the
// test takes its real, authenticated response and rewrites only the Child SA half:
// SAr2, TSi and TSr leave, NO_PROPOSAL_CHOSEN arrives. IDr and AUTH are untouched, and
// AUTH covers neither the Child SA payloads nor the notify (Section 2.15), so the
// response still verifies.
func crfAuthRefusingChild(t *testing.T) (ini *SA, iniPS *PeerSession, resp *SA, ps *PeerSession, peerTr, myTr *transport.UDPTransport) {
	t.Helper()
	quiet := slogutil.DiscardLogger()
	peerTr, myTr = rtxPeerLink(t)

	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "child-refusal-psk")
	// rtxPeerLink points remoteUDPAddr at peerTr, so a Delete has somewhere to land.
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
	ps = &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, quiet)
	if resp.State != StateEstablished {
		t.Fatalf("the responder did not establish (state %v), so there is no response to rewrite", resp.State)
	}

	accepted := lcyDecrypt(t, ini, resp.LastSentMsg)
	refused := make([]wire.PayloadEntry, 0, len(accepted)+1)
	for i := range accepted {
		switch accepted[i].Payload.(type) {
		case *wire.PayloadSA, *wire.PayloadTS:
			continue
		}
		refused = append(refused, accepted[i])
	}
	refused = append(refused, wire.PayloadEntry{Payload: &wire.PayloadNotify{
		NotifyMsgType: wire.NotifyNoProposalChosen,
	}})
	refusal, err := buildEncryptedMessageEx(resp, refused, authMsgID, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		t.Fatalf("building the refusing IKE_AUTH response: %v", err)
	}

	handleAuthResponse(ini, parseMsg(t, refusal), refusal, table, myTr, quiet)
	iniPS = &PeerSession{peerName: "ze", peerCfg: iniPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	return ini, iniPS, resp, ps, peerTr, myTr
}

// VALIDATES: an initiator whose IKE_AUTH response authenticates the responder but refuses
// the Child SA keeps the authentication (the SA establishes), then deletes the IKE SA ON
// THE WIRE, names the refusal in its log, and the peer that receives the Delete removes its
// own IKE SA.
//
// PREVENTS: the defect strongSwan interop exposed. handleAuthResponse verified AUTH and set
// StateEstablished, then initiatorFirstChildSA failed with "no peer ESP SPI recorded". The
// cycle ended with no Delete sent, so the peer kept an IKE SA nobody would use until its
// DPD found the node gone, and the log named a missing SPI instead of the peer's refusal.
//
// RFC 7296 Section 2.21.2 makes the response an authentication SUCCESS ("the initiator
// MUST NOT fail the authentication because of this") and lets the initiator delete the
// SA by policy ("The initiator MAY, of course, for reasons of policy later delete such an
// IKE SA"). Ze cannot yet create a Child SA after IKE_AUTH, so an IKE SA without one
// carries nothing, and deleting it with a Delete payload is the policy. runInitiator
// (fsm.go) calls deleteChildlessIKESA when the handshake loop ends on such an SA.
func TestAuthChildRefusalDeletesTheIKESA(t *testing.T) {
	ini, iniPS, resp, ps, peerTr, myTr := crfAuthRefusingChild(t)

	if ini.State != StateEstablished {
		t.Fatalf("the initiator left the authenticated SA in state %v, want StateEstablished; "+
			"RFC 7296 Section 2.21.2 forbids failing the authentication", ini.State)
	}
	if ini.ChildRefusal != wire.NotifyNoProposalChosen {
		t.Fatalf("the initiator recorded Child SA refusal %d, want NO_PROPOSAL_CHOSEN (%d)",
			ini.ChildRefusal, wire.NotifyNoProposalChosen)
	}

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	err := iniPS.deleteChildlessIKESA(ini, myTr, log)
	if !errors.Is(err, errChildSARefused) {
		t.Errorf("deleteChildlessIKESA returned %v, want errChildSARefused; a nil return "+
			"ends the session for good instead of reconnecting", err)
	}
	if logged := buf.String(); !strings.Contains(logged, "NO_PROPOSAL_CHOSEN") {
		t.Errorf("the initiator log does not name the peer's refusal (NO_PROPOSAL_CHOSEN):\n%s", logged)
	}

	sent := rtxRecv(t, peerTr)
	if sent == nil {
		t.Fatal("the initiator sent nothing; the peer keeps an IKE SA ze has abandoned")
	}
	hdr := parseMsg(t, sent).Header
	if hdr.ExchangeType != wire.ExchangeInformational {
		t.Errorf("the Delete went in exchange type %d, want INFORMATIONAL (%d)",
			hdr.ExchangeType, wire.ExchangeInformational)
	}
	inner := lcyDecrypt(t, resp, sent)
	dels := lcyDeletes(inner)
	if len(dels) != 1 {
		t.Fatalf("the INFORMATIONAL carries %d Delete payloads, want exactly 1", len(dels))
	}
	if dels[0].ProtocolID != wire.ProtocolIKE {
		t.Errorf("the Delete names protocol %d, want IKE (%d)", dels[0].ProtocolID, wire.ProtocolIKE)
	}

	// The peer side: its own owner-loop handler processes the Delete and ends its SA. A
	// Delete under a Message ID the peer has already answered would be replayed from its
	// cache instead, so this also proves the id is a fresh one.
	ps.handleOwnedInbound(resp, transport.Packet{Data: sent}, nil, nil, slogutil.DiscardLogger())
	if resp.State != StateDead {
		t.Errorf("the peer processed the Delete and kept its IKE SA in state %v, want StateDead",
			resp.State)
	}
}

// VALIDATES: an IKE_AUTH response that ACCEPTS the Child SA records no refusal, so
// runInitiator never deletes a working IKE SA. This is the discriminator for the test
// above: the refusal is read from the response, not assumed of every SA.
func TestAuthChildAcceptedRecordsNoRefusal(t *testing.T) {
	ini, _, _ := establishPSK(t)
	if ini.ChildRefusal != 0 {
		t.Errorf("an accepted Child SA recorded refusal %d; runInitiator would delete a "+
			"working IKE SA", ini.ChildRefusal)
	}
}
