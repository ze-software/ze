package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// createChildRequestWithKE builds a peer CREATE_CHILD_SA request for a NEW Child SA
// with PFS: SA (ESP), Ni, KEi, TSi, TSr and no REKEY_SA notify, the RFC 7296 Section
// 1.3.1 shape "HDR, SK {SA, Ni, [KEi,] TSi, TSr}" with the optional KEi present.
func createChildRequestWithKE(t *testing.T, peerESPSPI uint32, ni []byte) []wire.PayloadEntry {
	t.Helper()
	return []wire.PayloadEntry{
		{Payload: espSAPayload(peerESPSPI)},
		{Payload: &wire.PayloadNonce{NonceData: ni}},
		{Payload: &wire.PayloadKE{DHGroup: 14, KeyExchangeData: make([]byte, 256)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, "0.0.0.0/0")},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0")},
	}
}

// VALIDATES: AC-8 and AC-9. A peer request carrying KEi beside TSi and TSr and no
// REKEY_SA asks for a new Child SA, never for an IKE SA rekey, and while the session
// holds its one Child SA the answer is NO_ADDITIONAL_SAS with nothing changed.
// PREVENTS: the KE-first classifier that sent a PFS new-child request to
// respondIKERekey, which then answered it with IKE SA keys or an IKE error.
// Method: the request goes through handleCreateChildSAOwned on an established
// responder that holds a Child SA; the test reads the answer's notifies and checks
// that no IKE SA swap was staged and the Child SA is the same one.
func TestNewChildWithKEIsNotAnIKERekey(t *testing.T) {
	log := slogutil.DiscardLogger()
	link := errLink(t)
	ps := link.ps
	ps.peerName = "new-child-ke"
	ps.espGroup = testESPGroup()
	child := ps.getChildSA()
	if child == nil {
		t.Fatal("precondition: the established responder holds no Child SA")
	}

	inner := createChildRequestWithKE(t, 0x0badcafe, testNonce(41))
	msg := &wire.Message{Header: wire.Header{MessageID: link.resp.ExpectedMsgID}}
	out := ps.handleCreateChildSAOwned(link.resp, msg, inner, false, link.myTr, nil, log)

	got := rtxRecv(t, link.peerTr)
	if got == nil {
		t.Fatal("a new Child SA request with KEi drew no answer")
	}
	types := errNotifyIn(t, link.ini, got)
	if len(types) != 1 || types[0] != wire.NotifyNoAdditionalSAs {
		t.Errorf("the answer carries notifies %v, want exactly NO_ADDITIONAL_SAS", types)
	}
	if ps.pendingIKESwap != nil {
		t.Error("a new Child SA request staged an IKE SA swap")
	}
	if out.newSA != nil {
		t.Error("a new Child SA request produced a new IKE SA")
	}
	if ps.getChildSA() != child {
		t.Error("the live Child SA changed")
	}
}

// VALIDATES: AC-8 negative. A request with SA, Ni and KEi and no TS payload is still
// an IKE SA rekey (RFC 7296 Section 1.3.2, "HDR, SK {SA, Ni, KEi}").
// Method: Ze's own IKE SA rekey request, built by initiateIKERekey on the initiator,
// is decrypted by the responder and fed to handleCreateChildSAOwned; the responder
// stages the IKE SA swap.
func TestIKERekeyWithoutTSStaysAnIKERekey(t *testing.T) {
	log := slogutil.DiscardLogger()
	link := errLink(t)
	ps := link.ps
	ps.peerName = "ike-rekey-no-ts"

	reqBytes, pending, err := initiateIKERekey(link.ini, link.ini.IKEGroup)
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()
	reqMsg := parseMsg(t, reqBytes)
	inner, err := decryptAndParse(link.resp, reqMsg, reqBytes)
	if err != nil {
		t.Fatalf("the responder could not decrypt the rekey request: %v", err)
	}
	for i := range inner {
		if _, ok := inner[i].Payload.(*wire.PayloadTS); ok {
			t.Fatal("precondition: an IKE SA rekey request carries a TS payload")
		}
	}
	ps.handleCreateChildSAOwned(link.resp, reqMsg, inner, false, link.myTr, nil, log)
	if ps.pendingIKESwap == nil {
		t.Fatal("an IKE SA rekey request staged no IKE SA swap")
	}
	ps.pendingIKESwap.forgetKeys()
}
