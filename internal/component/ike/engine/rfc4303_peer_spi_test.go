// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the ESP SPI the peer allocates
// Related: rfc4303_spi_test.go -- the ESP SPI Ze allocates for itself

package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The ESP SPI Ze SENDS on the wire is never one Ze chose. RFC 4303 Section 2.1 lets
// the receiver pick it, so it is the peer's number: SAr2 when Ze initiates IKE_AUTH,
// SAi2 when Ze responds, and the SA payload of a CREATE_CHILD_SA in either role.
// These tests drive each of the four producers with a peer SPI of 0, the reserved
// value, and with a valid one. The messages are the real encrypted exchange: a test
// decrypts one side's message, edits the SPI, and encrypts it again with the sender's
// keys, so the receiver's full handler parses it.
//
// VALIDATES: every producer of the SPI Ze sends on refuses a peer SPI of 0 and uses a
// valid peer SPI unchanged.
// PREVENTS: an outbound ESP state keyed on a random SPI the peer never allocated.

// peerSPIPreAuth runs IKE_SA_INIT between two Ze SAs and stops before IKE_AUTH, so
// a test can edit the IKE_AUTH request or response before the peer handles it.
func peerSPIPreAuth(t *testing.T) (ini, resp *SA, ps *PeerSession) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "peer-spi-psk")

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
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
	ps = &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	return ini, resp, ps
}

// setESPSPI rewrites the SPI of every ESP proposal in the SA payload of inner. It fails
// the test when inner holds no ESP proposal, so an edit never silently misses.
func setESPSPI(t *testing.T, inner []wire.PayloadEntry, spi uint32) {
	t.Helper()
	edited := 0
	for _, pe := range inner {
		p, ok := pe.Payload.(*wire.PayloadSA)
		if !ok {
			continue
		}
		for i := range p.Proposals {
			if p.Proposals[i].ProtocolID != wire.ProtocolESP {
				continue
			}
			p.Proposals[i].SPI = []byte{byte(spi >> 24), byte(spi >> 16), byte(spi >> 8), byte(spi)}
			p.Proposals[i].SPISize = 4
			edited++
		}
	}
	if edited == 0 {
		t.Fatal("the message carries no ESP proposal to edit")
	}
}

// reencrypt decrypts raw at receiver, sets the ESP SPI of its SA payload to spi, and
// encrypts the result again with sender's keys, keeping the header's exchange, flags
// and Message ID.
func reencrypt(t *testing.T, sender, receiver *SA, raw []byte, spi uint32) []byte {
	t.Helper()
	hdr := parseMsg(t, raw).Header
	inner := mbDecrypt(t, receiver, raw)
	setESPSPI(t, inner, spi)
	out, err := buildEncryptedMessageEx(sender, inner, hdr.MessageID, hdr.ExchangeType, hdr.Flags)
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}
	return out
}

// outboundSPIs returns the SPI of every outbound ESP state the dataplane installed.
func outboundSPIs(dp *mockDP) []uint32 {
	var out []uint32
	for i := range dp.sas {
		if dp.sas[i].Dir == dataplane.SADirOut {
			out = append(out, dp.sas[i].SPI)
		}
	}
	return out
}

// TestInitiatorRefusesPeerSPIZero drives the initiator's IKE_AUTH response handler
// with an SAr2 whose ESP SPI is 0, and then asks for the first Child SA.
//
// Goal: the responder's SAr2 SPI is the SPI Ze puts on every ESP packet it sends.
// A 0 there is a value the peer MUST NOT send, and Ze MUST NOT replace it with a
// number the peer never allocated. Method: the responder's real IKE_AUTH response is
// decrypted, its SAr2 SPI set to 0, and encrypted again with the responder's keys.
// handleAuthResponse MUST NOT establish the IKE SA, and initiatorFirstChildSA MUST
// refuse and install no ESP state.
//
// MUTATION: delete the SPI 0 refusal in handleAuthResponse (fsm.go) and this test
// goes red: the SA reaches StateEstablished.
//
// RFC requirement: RFC4303-2.1-1 negative -- an IKE_AUTH response whose SAr2 ESP SPI is 0 leaves the initiator's IKE SA not established, initiatorFirstChildSA then returns an error, and the dataplane holds no ESP state.
func TestInitiatorRefusesPeerSPIZero(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := peerSPIPreAuth(t)
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
	answer := reencrypt(t, resp, ini, resp.LastSentMsg, 0)

	handleAuthResponse(ini, parseMsg(t, answer), answer, nil, nil, log)
	if ini.State == StateEstablished {
		t.Fatal("the initiator established an IKE SA whose SAr2 carries the reserved ESP SPI 0")
	}

	dp := &mockDP{}
	child, err := initiatorFirstChildSA(ini, ini.PeerCfg, 7, dp, log)
	if err == nil {
		t.Fatalf("initiatorFirstChildSA accepted a missing peer SPI and sends on SPI %#08x", child.OutboundSPI)
	}
	if len(dp.sas) != 0 {
		t.Fatalf("the dataplane holds %d ESP states, want 0", len(dp.sas))
	}
}

// TestInitiatorUsesPeerSPI drives the same exchange with a valid SAr2 SPI.
//
// Goal: the negative test above passes for code that refuses every SAr2, so this
// pins the accepted case. Method: the responder's IKE_AUTH response carries SPI
// 0x0a0b0c0d in SAr2. The initiator MUST establish, and the one outbound ESP state
// of its first Child SA MUST carry exactly 0x0a0b0c0d.
//
// RFC requirement: RFC4303-2.1-1 positive -- an IKE_AUTH response whose SAr2 ESP SPI is 0x0a0b0c0d establishes the initiator's IKE SA, and the first Child SA installs exactly one outbound ESP state, with SPI 0x0a0b0c0d.
func TestInitiatorUsesPeerSPI(t *testing.T) {
	const peerSPI = 0x0a0b0c0d
	log := slogutil.DiscardLogger()
	ini, resp, ps := peerSPIPreAuth(t)
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
	answer := reencrypt(t, resp, ini, resp.LastSentMsg, peerSPI)

	handleAuthResponse(ini, parseMsg(t, answer), answer, nil, nil, log)
	if ini.State != StateEstablished {
		t.Fatalf("state = %v, want established for SAr2 SPI %#08x", ini.State, uint32(peerSPI))
	}

	dp := &mockDP{}
	child, err := initiatorFirstChildSA(ini, ini.PeerCfg, 7, dp, log)
	if err != nil {
		t.Fatalf("initiatorFirstChildSA: %v", err)
	}
	if child.OutboundSPI != peerSPI {
		t.Errorf("Child SA outbound SPI = %#08x, want the peer's %#08x", child.OutboundSPI, uint32(peerSPI))
	}
	got := outboundSPIs(dp)
	if len(got) != 1 || got[0] != peerSPI {
		t.Errorf("outbound ESP states = %#08x, want exactly [%#08x]", got, uint32(peerSPI))
	}
}

// TestResponderRefusesPeerSPIZero drives the responder's IKE_AUTH request handler with
// an SAi2 whose ESP SPI is 0.
//
// Goal: SAi2's SPI is what the responder sends on. Method: the initiator's real
// IKE_AUTH request is decrypted, its SAi2 SPI set to 0, and encrypted again with the
// initiator's keys. handleAuthRequest MUST NOT install a first Child SA, and its
// IKE_AUTH response MUST refuse the Child SA with NO_PROPOSAL_CHOSEN and carry no SA
// payload. The IKE SA itself is not judged here: RFC 4303 and RFC 3948 constrain the
// ESP SPI, and RFC 7296 Section 2.21.2 lets the IKE SA stay up without a Child SA.
//
// MUTATION: record `outSPI ^ 1` in place of the SAi2 SPI in selectAuthChildSA
// (responder.go), so a peer 0 becomes 1 and passes both refusals, and this test goes
// red. Deleting one refusal alone leaves it green: selectAuthChildSA and
// createFirstChildSA each refuse the 0.
//
// RFC requirement: RFC4303-2.1-1 negative -- an IKE_AUTH request whose SAi2 ESP SPI is 0 leaves the responder's session with no first Child SA, and the IKE_AUTH response carries NO_PROPOSAL_CHOSEN and no SA payload.
// RFC requirement: RFC3948-2.1-1 negative -- the responder refuses an SAi2 ESP SPI of 0, the SPI its outbound ESP header would carry: no first Child SA exists, and the IKE_AUTH response carries NO_PROPOSAL_CHOSEN and no SA payload.
func TestResponderRefusesPeerSPIZero(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := peerSPIPreAuth(t)
	request := reencrypt(t, ini, resp, ini.LastSentMsg, 0)

	ps.handleAuthRequest(resp, parseMsg(t, request), request, nil, nil, log)
	if child := ps.getChildSA(); child != nil {
		t.Fatalf("the responder installed a Child SA with outbound SPI %#08x", child.OutboundSPI)
	}
	if len(resp.LastSentMsg) == 0 {
		t.Fatal("the responder sent no IKE_AUTH response, so the Child SA refusal never reached the peer")
	}
	refused := false
	for _, pe := range mbDecrypt(t, ini, resp.LastSentMsg) {
		switch p := pe.Payload.(type) {
		case *wire.PayloadSA:
			t.Error("the IKE_AUTH response carries an SA payload for a Child SA on SPI 0")
		case *wire.PayloadNotify:
			if p.NotifyMsgType == wire.NotifyNoProposalChosen {
				refused = true
			}
		}
	}
	if !refused {
		t.Error("the IKE_AUTH response carries no NO_PROPOSAL_CHOSEN refusing the Child SA")
	}
}

// TestResponderUsesPeerSPI drives the same request with a valid SAi2 SPI.
//
// Goal: pin the accepted case the negative test cannot. Method: SAi2 carries
// 0x0a0b0c0d. The responder MUST establish, and its first Child SA MUST send on
// exactly 0x0a0b0c0d.
//
// RFC requirement: RFC4303-2.1-1 positive -- an IKE_AUTH request whose SAi2 ESP SPI is 0x0a0b0c0d establishes the responder's IKE SA, and its first Child SA's outbound SPI is 0x0a0b0c0d.
// RFC requirement: RFC3948-2.1-1 positive -- the responder's first Child SA carries the non-zero SAi2 SPI 0x0a0b0c0d as its outbound ESP SPI.
func TestResponderUsesPeerSPI(t *testing.T) {
	const peerSPI = 0x0a0b0c0d
	log := slogutil.DiscardLogger()
	ini, resp, ps := peerSPIPreAuth(t)
	request := reencrypt(t, ini, resp, ini.LastSentMsg, peerSPI)

	ps.handleAuthRequest(resp, parseMsg(t, request), request, nil, nil, log)
	if resp.State != StateEstablished {
		t.Fatalf("state = %v, want established for SAi2 SPI %#08x", resp.State, uint32(peerSPI))
	}
	child := ps.getChildSA()
	if child == nil {
		t.Fatal("the responder installed no first Child SA")
	}
	if child.OutboundSPI != peerSPI {
		t.Errorf("Child SA outbound SPI = %#08x, want the peer's %#08x", child.OutboundSPI, uint32(peerSPI))
	}
}

// peerSPIRekeyExchange is a Child SA rekey in flight between two Ze SAs.
type peerSPIRekeyExchange struct {
	ini, resp *SA
	pending   *pendingRekey       // the initiator's side of the rekey
	respOld   *ChildSA            // the responder's Child SA being rekeyed
	request   []wire.PayloadEntry // the rekey request, as the responder decrypted it
	msgID     uint32              // the request's Message ID
}

// peerSPIRekey establishes an IKE SA and a first Child SA on both sides, then has the
// initiator start a Child SA rekey.
func peerSPIRekey(t *testing.T) peerSPIRekeyExchange {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	respOld := ps.getChildSA()
	if respOld == nil {
		t.Fatal("the responder holds no first Child SA to rekey")
	}
	iniOld, err := initiatorFirstChildSA(ini, ini.PeerCfg, 7, &mockDP{}, log)
	if err != nil {
		t.Fatalf("initiatorFirstChildSA: %v", err)
	}
	raw, pending, err := initiateChildRekey(ini, iniOld)
	if err != nil {
		t.Fatalf("initiateChildRekey: %v", err)
	}
	return peerSPIRekeyExchange{
		ini: ini, resp: resp, pending: pending, respOld: respOld,
		request: mbDecrypt(t, resp, raw), msgID: parseMsg(t, raw).Header.MessageID,
	}
}

// TestChildRekeyRequestRefusesPeerSPIZero drives the responder of a Child SA rekey
// with a CREATE_CHILD_SA request whose ESP SPI is 0.
//
// Goal: the SPI in a rekey request is what the new Child SA sends on. Method: the
// initiator's real rekey request is decrypted and its SPI set to 0. respondChildRekey
// MUST refuse it as a malformed request, return no Child SA, and install no ESP state.
//
// MUTATION: drop `|| peerSPI == 0` from the missing-payload refusal in
// respondChildRekey (rekey.go) and this test goes red.
//
// RFC requirement: RFC4303-2.1-1 negative -- a CREATE_CHILD_SA rekey request whose ESP SPI is 0 makes respondChildRekey return an error wrapping errMalformedRequest and no Child SA, and the dataplane holds no ESP state.
// RFC requirement: RFC3948-2.1-1 negative -- respondChildRekey refuses a rekey request ESP SPI of 0, the SPI the new outbound ESP header would carry: no Child SA and no ESP state.
func TestChildRekeyRequestRefusesPeerSPIZero(t *testing.T) {
	log := slogutil.DiscardLogger()
	x := peerSPIRekey(t)
	resp, respOld, request, msgID := x.resp, x.respOld, x.request, x.msgID
	setESPSPI(t, request, 0)

	dp := &mockDP{}
	_, child, err := respondChildRekey(resp, request, respOld, msgID, dp, log)
	if !errors.Is(err, errMalformedRequest) {
		t.Fatalf("respondChildRekey error = %v, want errMalformedRequest", err)
	}
	if child != nil {
		t.Fatalf("respondChildRekey returned a Child SA with outbound SPI %#08x", child.OutboundSPI)
	}
	if len(dp.sas) != 0 {
		t.Fatalf("the dataplane holds %d ESP states, want 0", len(dp.sas))
	}
}

// TestChildRekeyRequestUsesPeerSPI drives the same rekey request with a valid SPI.
//
// Goal: pin the accepted case the negative test cannot. Method: the request carries
// 0x0a0b0c0d, and the rekeyed Child SA MUST send on exactly that SPI.
//
// RFC requirement: RFC4303-2.1-1 positive -- a CREATE_CHILD_SA rekey request whose ESP SPI is 0x0a0b0c0d makes respondChildRekey return a Child SA whose outbound SPI is 0x0a0b0c0d.
// RFC requirement: RFC3948-2.1-1 positive -- the Child SA respondChildRekey returns for a rekey request SPI of 0x0a0b0c0d carries that non-zero SPI as its outbound ESP SPI.
func TestChildRekeyRequestUsesPeerSPI(t *testing.T) {
	const peerSPI = 0x0a0b0c0d
	log := slogutil.DiscardLogger()
	x := peerSPIRekey(t)
	resp, respOld, request, msgID := x.resp, x.respOld, x.request, x.msgID
	setESPSPI(t, request, peerSPI)

	_, child, err := respondChildRekey(resp, request, respOld, msgID, &mockDP{}, log)
	if err != nil {
		t.Fatalf("respondChildRekey: %v", err)
	}
	if child.OutboundSPI != peerSPI {
		t.Errorf("rekeyed Child SA outbound SPI = %#08x, want the peer's %#08x", child.OutboundSPI, uint32(peerSPI))
	}
}

// TestChildRekeyResponseRefusesPeerSPIZero drives the initiator of a Child SA rekey
// with a CREATE_CHILD_SA response whose ESP SPI is 0.
//
// Goal: the SPI in a rekey response is what the initiator's new Child SA sends on.
// Method: the responder answers the real rekey request, the answer is decrypted, and
// its SPI set to 0. applyChildRekeyResponse MUST refuse it and install no ESP state.
//
// MUTATION: drop `|| outSPI == 0` from the missing-payload refusal in
// applyChildRekeyResponse (rekey.go) and this test goes red.
//
// RFC requirement: RFC4303-2.1-1 negative -- a CREATE_CHILD_SA rekey response whose ESP SPI is 0 makes applyChildRekeyResponse return an error, and the dataplane holds no ESP state.
// RFC requirement: RFC3948-2.1-1 negative -- applyChildRekeyResponse refuses a rekey response ESP SPI of 0, the SPI the new outbound ESP header would carry: an error and no ESP state.
func TestChildRekeyResponseRefusesPeerSPIZero(t *testing.T) {
	log := slogutil.DiscardLogger()
	x := peerSPIRekey(t)
	ini, resp, pending, respOld, request, msgID := x.ini, x.resp, x.pending, x.respOld, x.request, x.msgID
	answer, _, err := respondChildRekey(resp, request, respOld, msgID, &mockDP{}, log)
	if err != nil {
		t.Fatalf("respondChildRekey: %v", err)
	}
	inner := mbDecrypt(t, ini, answer)
	setESPSPI(t, inner, 0)

	dp := &mockDP{}
	child, err := applyChildRekeyResponse(ini, pending, inner, dp, log)
	if err == nil {
		t.Fatalf("applyChildRekeyResponse accepted SPI 0 and sends on %#08x", child.OutboundSPI)
	}
	if len(dp.sas) != 0 {
		t.Fatalf("the dataplane holds %d ESP states, want 0", len(dp.sas))
	}
}

// TestChildRekeyResponseUsesPeerSPI drives the same rekey response with a valid SPI.
//
// Goal: pin the accepted case the negative test cannot. Method: the response carries
// 0x0a0b0c0d, and the rekeyed Child SA MUST send on exactly that SPI.
//
// RFC requirement: RFC4303-2.1-1 positive -- a CREATE_CHILD_SA rekey response whose ESP SPI is 0x0a0b0c0d makes applyChildRekeyResponse return a Child SA whose outbound SPI is 0x0a0b0c0d.
// RFC requirement: RFC3948-2.1-1 positive -- the Child SA applyChildRekeyResponse returns for a rekey response SPI of 0x0a0b0c0d carries that non-zero SPI as its outbound ESP SPI.
func TestChildRekeyResponseUsesPeerSPI(t *testing.T) {
	const peerSPI = 0x0a0b0c0d
	log := slogutil.DiscardLogger()
	x := peerSPIRekey(t)
	ini, resp, pending, respOld, request, msgID := x.ini, x.resp, x.pending, x.respOld, x.request, x.msgID
	answer, _, err := respondChildRekey(resp, request, respOld, msgID, &mockDP{}, log)
	if err != nil {
		t.Fatalf("respondChildRekey: %v", err)
	}
	inner := mbDecrypt(t, ini, answer)
	setESPSPI(t, inner, peerSPI)

	child, err := applyChildRekeyResponse(ini, pending, inner, &mockDP{}, log)
	if err != nil {
		t.Fatalf("applyChildRekeyResponse: %v", err)
	}
	if child.OutboundSPI != peerSPI {
		t.Errorf("rekeyed Child SA outbound SPI = %#08x, want the peer's %#08x", child.OutboundSPI, uint32(peerSPI))
	}
}
