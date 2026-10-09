package engine

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// refusedNotify returns the payload chain of a CREATE_CHILD_SA response that carries
// nothing but one error notify. RFC 7296 Section 3.10.1: an implementation receiving an
// unrecognized error type in a response "MUST assume that the corresponding request has
// failed entirely", and a recognized error fails it too, so such a response carries no
// keys.
func refusedNotify(notifyType uint16, data []byte) []wire.PayloadEntry {
	return []wire.PayloadEntry{
		{Payload: &wire.PayloadNotify{NotifyMsgType: notifyType, NotificationData: data}},
	}
}

// refusedRig is one established PeerSession with a live Child SA and a UDP link to a
// stand-in peer, so a test can watch what ze sends after a refused rekey.
type refusedRig struct {
	peer   *SA // The stand-in peer's end of the same IKE SA.
	sa     *SA
	ps     *PeerSession
	old    *ChildSA
	dp     *rkyDP
	peerTr *transport.UDPTransport
	myTr   *transport.UDPTransport
}

func newRefusedRig(t *testing.T) *refusedRig {
	t.Helper()
	log := slogutil.DiscardLogger()
	peer, sa, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t, sa)
	sa.PeerCfg.RemoteAddress = "127.0.0.1"
	if sa.remoteUDPAddr() == nil {
		t.Fatal("the SA has no resolvable peer address")
	}
	dp := &rkyDP{}
	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	ps.setChildSA(old)
	return &refusedRig{peer: peer, sa: sa, ps: ps, old: old, dp: dp, peerTr: peerTr, myTr: myTr}
}

// answerChildRekey starts one Child SA rekey, checks it reached the peer, and feeds the
// owner the given response chain.
func (r *refusedRig) answerChildRekey(t *testing.T, inner []wire.PayloadEntry) ownedOutcome {
	t.Helper()
	log := slogutil.DiscardLogger()
	r.ps.startChildRekey(r.sa, r.myTr, log)
	if rtxRecv(t, r.peerTr) == nil {
		t.Fatal("the Child SA rekey request never reached the peer")
	}
	if r.ps.pendingRekey == nil || r.ps.pendingRekey.kind != rekeyChild {
		t.Fatal("the Child SA rekey left no outstanding Child rekey")
	}
	respMsg := &wire.Message{Header: wire.Header{MessageID: r.ps.pendingRekey.messageID}}
	out := r.ps.handleCreateChildSAOwned(r.sa, respMsg, inner, true, r.myTr, r.dp, log)
	r.sa.releaseRequestWindow()
	return out
}

// answerIKERekey is answerChildRekey for an IKE SA rekey.
func (r *refusedRig) answerIKERekey(t *testing.T, inner []wire.PayloadEntry) ownedOutcome {
	t.Helper()
	log := slogutil.DiscardLogger()
	r.ps.startIKERekey(r.sa, r.sa.IKEGroup, r.myTr, log)
	if rtxRecv(t, r.peerTr) == nil {
		t.Fatal("the IKE SA rekey request never reached the peer")
	}
	if r.ps.pendingRekey == nil || r.ps.pendingRekey.kind != rekeyIKE {
		t.Fatal("the IKE SA rekey left no outstanding IKE rekey")
	}
	respMsg := &wire.Message{Header: wire.Header{MessageID: r.ps.pendingRekey.messageID}}
	out := r.ps.handleCreateChildSAOwned(r.sa, respMsg, inner, true, r.myTr, r.dp, log)
	r.sa.releaseRequestWindow()
	return out
}

// VALIDATES: a rekey response carrying an error notify is reported as that error, on
// both rekey paths. The method calls the two apply functions directly with a response
// that carries the notify alone, and reads the error they return.
// PREVENTS: the misread seen against strongSwan in delete-while-window-held. A Child SA
// rekey answered N(NO_PROPOSAL_CHOSEN) was logged as "missing Nr(0) or ESP SPI(0)",
// because only TEMPORARY_FAILURE and NO_ADDITIONAL_SAS were read before the payload
// walk. The operator saw a malformed response where the peer had refused the proposal.
func TestRekeyErrorNotifyIsReadAsThatError(t *testing.T) {
	log := slogutil.DiscardLogger()
	cases := []struct {
		notify uint16
		data   []byte
	}{
		{wire.NotifyNoProposalChosen, nil},
		{wire.NotifyInvalidKEPayload, []byte{0, 19}},
		{wire.NotifyTSUnacceptable, nil},
		{wire.NotifyChildSANotFound, nil},
	}
	for _, c := range cases {
		name := wire.NotifyTypeName(c.notify)
		t.Run("child "+name, func(t *testing.T) {
			r := newRefusedRig(t)
			_, pending, err := initiateChildRekey(r.sa, r.old)
			if err != nil {
				t.Fatalf("initiateChildRekey: %v", err)
			}
			defer pending.clear()
			_, err = applyChildRekeyResponse(r.sa, pending, refusedNotify(c.notify, c.data), r.dp, log)
			if err == nil {
				t.Fatal("a refused Child SA rekey was accepted")
			}
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("the refusal was reported as %q, which does not name %s", err, name)
			}
			if strings.Contains(err.Error(), "missing") {
				t.Fatalf("the refusal was reported as a malformed response: %q", err)
			}
		})
		t.Run("ike "+name, func(t *testing.T) {
			r := newRefusedRig(t)
			_, pending, err := initiateIKERekey(r.sa, testIKEGroup())
			if err != nil {
				t.Fatalf("initiateIKERekey: %v", err)
			}
			defer pending.clear()
			_, err = applyIKERekeyResponse(r.sa, pending, refusedNotify(c.notify, c.data), log)
			if err == nil {
				t.Fatal("a refused IKE SA rekey was accepted")
			}
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("the refusal was reported as %q, which does not name %s", err, name)
			}
			if strings.Contains(err.Error(), "missing") {
				t.Fatalf("the refusal was reported as a malformed response: %q", err)
			}
		})
	}
}

// VALIDATES: a Child SA rekey the peer refuses keeps the current Child SA and IKE SA,
// and is not resent until the refusal hold ends. The method answers a rekey with one
// error notify, then asks for the rekey again directly and from one owner-loop tick
// under a soft-expired lifetime, and expects silence on the peer's socket both times.
// Once the hold has passed, the same rekey goes out again.
// PREVENTS: the one-second resend loop seen against strongSwan. The soft lifetime is
// a level trigger, and a refusal used to clear the exchange without arming anything,
// so every tick resent the same refused rekey until the hard lifetime ended the SA.
func TestChildRekeyRefusalHoldsTheRetry(t *testing.T) {
	refusals := []struct {
		name  string
		inner []wire.PayloadEntry
	}{
		{"NO_PROPOSAL_CHOSEN", refusedNotify(wire.NotifyNoProposalChosen, nil)},
		{"INVALID_KE_PAYLOAD", refusedNotify(wire.NotifyInvalidKEPayload, []byte{0, 19})},
		{"TS_UNACCEPTABLE", refusedNotify(wire.NotifyTSUnacceptable, nil)},
		// RFC 7296 Section 3.10.1 makes an unrecognized error type in a response a
		// failed request, so it is held the same way as a recognized refusal.
		{"an unrecognized error notify", refusedNotify(unrecognizedErrorType(t), nil)},
	}
	for _, c := range refusals {
		t.Run(c.name, func(t *testing.T) {
			log := slogutil.DiscardLogger()
			r := newRefusedRig(t)
			remote := r.sa.remoteUDPAddr()

			out := r.answerChildRekey(t, c.inner)
			if out.newChild != nil {
				t.Fatal("a refused rekey installed a replacement Child SA")
			}
			if out.reestablish {
				t.Fatal("a refused rekey tore the IKE SA down; RFC 7296 Section 1.3.1 says it SHOULD NOT")
			}
			if r.ps.pendingRekey != nil {
				t.Fatal("the refused exchange is still outstanding")
			}
			if r.ps.getChildSA() != r.old {
				t.Fatal("a refused rekey replaced the live Child SA")
			}

			r.ps.startChildRekey(r.sa, r.myTr, log)
			rtxExpectSilence(t, r.peerTr, r.myTr, remote, "a rekey resent right after the peer refused it")

			r.ps.stopCh = make(chan struct{})
			r.ps.supersede = make(chan struct{}, 1)
			looped := make(chan struct{})
			go func() {
				_ = r.ps.maintainSA(r.sa, nil, winSoftExpired(), nil,
					testIKEGroup(), NewSATable(), r.dp, r.myTr, nil, log)
				close(looped)
			}()
			time.Sleep(1500 * time.Millisecond) // one full ticker period, so a tick has run
			close(r.ps.stopCh)
			<-looped
			rtxExpectSilence(t, r.peerTr, r.myTr, remote, "an owner-loop tick after the peer refused the rekey")
			r.ps.setChildSA(r.old)

			// A refusal is not a busy peer, so it does not stop the path probe that
			// the TEMPORARY_FAILURE hold stops (probe.go).
			if r.ps.rekeyHoldInForce(time.Now()) {
				t.Fatal("a refused rekey armed the TEMPORARY_FAILURE hold the path probe reads")
			}

			r.ps.childRekeyRefusedUntil = time.Now().Add(-time.Second)
			r.ps.startChildRekey(r.sa, r.myTr, log)
			if rtxRecv(t, r.peerTr) == nil {
				t.Fatal("the rekey never went out after the hold elapsed; the wait must be a delay, not a stop")
			}
			r.ps.pendingRekey.clear()
		})
	}
}

// VALIDATES: an IKE SA rekey the peer refuses is held the same way, and the hold is the
// IKE SA's own. The Child SA rekey still goes out while it stands.
// PREVENTS: the same resend loop on the IKE SA rekey path, which read the same response
// as "missing Nr(0)/KEr(0)/SPI(false)".
func TestIKERekeyRefusalHoldsTheRetry(t *testing.T) {
	log := slogutil.DiscardLogger()
	r := newRefusedRig(t)
	remote := r.sa.remoteUDPAddr()

	out := r.answerIKERekey(t, refusedNotify(wire.NotifyNoProposalChosen, nil))
	if out.newSA != nil {
		t.Fatal("a refused IKE SA rekey produced a replacement IKE SA")
	}
	if out.reestablish {
		t.Fatal("a refused IKE SA rekey tore the IKE SA down")
	}
	if r.ps.pendingRekey != nil {
		t.Fatal("the refused IKE rekey is still outstanding")
	}

	r.ps.startIKERekey(r.sa, testIKEGroup(), r.myTr, log)
	rtxExpectSilence(t, r.peerTr, r.myTr, remote, "an IKE SA rekey resent right after the peer refused it")

	r.ps.startChildRekey(r.sa, r.myTr, log)
	if rtxRecv(t, r.peerTr) == nil {
		t.Fatal("a refused IKE SA rekey also stopped the Child SA rekey; the two holds must be separate")
	}
	r.ps.pendingRekey.clear()
}

// VALIDATES: a Child SA rekey answered CHILD_SA_NOT_FOUND asks the owner loop to
// re-establish, which deletes the Child SA and builds a new one from scratch. RFC 7296
// Section 2.25: "A peer that receives a CHILD_SA_NOT_FOUND notification SHOULD silently
// delete the Child SA (if it still exists) and send a request to create a new Child SA
// from scratch (if the Child SA does not yet exist)."
// PREVENTS: resending a rekey for a Child SA the peer says does not exist. No retry can
// succeed, and the peer holds no SA for the traffic this node still sends into it.
func TestChildRekeyChildSANotFoundReestablishes(t *testing.T) {
	r := newRefusedRig(t)
	out := r.answerChildRekey(t, refusedNotify(wire.NotifyChildSANotFound, nil))
	if !out.reestablish {
		t.Fatal("a CHILD_SA_NOT_FOUND answer did not ask for a new Child SA")
	}
	if out.newChild != nil {
		t.Fatal("a CHILD_SA_NOT_FOUND answer installed a replacement Child SA")
	}
}

// unrecognizedErrorType returns an error notify type the registry does not name.
func unrecognizedErrorType(t *testing.T) uint16 {
	t.Helper()
	for v := uint16(8191); v > 0; v-- {
		if !wire.NotifyTypeRecognized(v) {
			return v
		}
	}
	t.Fatal("every error notify type is registered")
	return 0
}

// refusedRigTwoGroups is newRefusedRig with a second IKE proposal configured in group
// 19, so the operator's proposals for this peer name two Diffie-Hellman groups: 14
// (the first, which every rekey starts with) and 19, and enables PFS for the Child SA.
func refusedRigTwoGroups(t *testing.T) *refusedRig {
	t.Helper()
	r := newRefusedRig(t)
	extra := r.sa.IKEGroup.Proposals[0]
	extra.Number++
	extra.DHGroup = 19
	r.sa.IKEGroup.Proposals = append(r.sa.IKEGroup.Proposals, extra)
	// testESPGroup disables PFS, and a Child SA rekey without a KEi has no group to
	// change. The operator enables it here, so the Child SA rekey carries KEi.
	r.old.ESPGroup.PFS = ipsec.PFSEnable
	return r
}

// VALIDATES: a Child SA rekey answered INVALID_KE_PAYLOAD naming a group the operator
// configured is retried at once in that group, and a group already refused for this SA
// is never offered again. When every configured group has been refused, the rekey
// waits, then the record is cleared and the cycle starts over from the first group.
// The method reads the group of the KEi each request carries (pendingRekey.dh).
// PREVENTS: a fixed wait where a usable proposal remains, and a resend of a refused
// proposal, which is the loop the first fix stopped with a delay.
func TestChildRekeyInvalidKERetriesInConfiguredGroup(t *testing.T) {
	log := slogutil.DiscardLogger()
	r := refusedRigTwoGroups(t)
	remote := r.sa.remoteUDPAddr()

	r.answerChildRekey(t, refusedNotify(wire.NotifyInvalidKEPayload, []byte{0, 19}))
	r.ps.startChildRekey(r.sa, r.myTr, log)
	if rtxRecv(t, r.peerTr) == nil {
		t.Fatal("the rekey was not retried at once in the group the peer named")
	}
	if got := r.ps.pendingRekey.dh.GroupID; got != 19 {
		t.Fatalf("the retry carried KEi in group %d, want the named group 19", got)
	}
	respMsg := &wire.Message{Header: wire.Header{MessageID: r.ps.pendingRekey.messageID}}
	// The peer now names group 14, which it already refused for this SA.
	r.ps.handleCreateChildSAOwned(r.sa, respMsg, refusedNotify(wire.NotifyInvalidKEPayload, []byte{0, 14}),
		true, r.myTr, r.dp, log)
	r.sa.releaseRequestWindow()

	r.ps.startChildRekey(r.sa, r.myTr, log)
	rtxExpectSilence(t, r.peerTr, r.myTr, remote, "a rekey resent in a group the peer already refused")

	r.ps.childRekeyRefusedUntil = time.Now().Add(-time.Second)
	r.ps.startChildRekey(r.sa, r.myTr, log)
	if rtxRecv(t, r.peerTr) == nil {
		t.Fatal("the rekey never went out after the wait")
	}
	if got := r.ps.pendingRekey.dh.GroupID; got != 14 {
		t.Fatalf("after the wait the rekey carried group %d, want the first configured group 14", got)
	}
	r.ps.pendingRekey.clear()
}

// VALIDATES: the same rule on the IKE SA rekey path: the named configured group is
// used at once.
// PREVENTS: the IKE SA rekey waiting, or resending in the refused group.
func TestIKERekeyInvalidKERetriesInConfiguredGroup(t *testing.T) {
	log := slogutil.DiscardLogger()
	r := refusedRigTwoGroups(t)

	r.answerIKERekey(t, refusedNotify(wire.NotifyInvalidKEPayload, []byte{0, 19}))
	r.ps.startIKERekey(r.sa, r.sa.IKEGroup, r.myTr, log)
	if rtxRecv(t, r.peerTr) == nil {
		t.Fatal("the IKE SA rekey was not retried at once in the group the peer named")
	}
	if got := r.ps.pendingRekey.dh.GroupID; got != 19 {
		t.Fatalf("the retry carried KEi in group %d, want the named group 19", got)
	}
	r.ps.pendingRekey.clear()
}

// relayToPeer carries the rekey request ze just sent to the stand-in peer, lets respond
// build the peer's answer from the decrypted request, and feeds that answer back to the
// owner. It returns the request ze sent, so a test can read the KE payload on the wire.
func (r *refusedRig) relayToPeer(t *testing.T, respond func(req []wire.PayloadEntry, msgID uint32) []byte) ([]wire.PayloadEntry, ownedOutcome) {
	t.Helper()
	log := slogutil.DiscardLogger()
	raw := rtxRecv(t, r.peerTr)
	if raw == nil {
		t.Fatal("the rekey request never reached the peer")
	}
	if r.ps.pendingRekey == nil {
		t.Fatal("the rekey left no outstanding exchange")
	}
	msgID := r.ps.pendingRekey.messageID
	req, err := decryptAndParse(r.peer, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("the peer could not decrypt the rekey request: %v", err)
	}
	answer := respond(req, msgID)
	inner, err := decryptAndParse(r.sa, parseMsg(t, answer), answer)
	if err != nil {
		t.Fatalf("ze could not decrypt the peer's answer: %v", err)
	}
	respMsg := &wire.Message{Header: wire.Header{MessageID: msgID}}
	out := r.ps.handleCreateChildSAOwned(r.sa, respMsg, inner, true, r.myTr, r.dp, log)
	r.sa.releaseRequestWindow()
	return req, out
}

// VALIDATES: a Child SA rekey refused with INVALID_KE_PAYLOAD naming group 19 completes
// in group 19, end to end. The method runs both ends of one IKE SA. The peer answers the
// first request, which carries KEi in group 14, with INVALID_KE_PAYLOAD naming 19, and
// its real respondChildRekey accepts the retry. The first answer is scripted: ze offers
// one group per Child SA rekey, so a responder that runs only group 19 answers that
// offer NO_PROPOSAL_CHOSEN, and the INVALID_KE_PAYLOAD naming 19 is the peer's choice
// this test stands in for. The test checks the retry's KEi on the wire, that the old
// Child SA stayed live until the replacement was installed, and that both ends derived
// the same keys, which they can only do from one shared secret in group 19.
// PREVENTS: a retry that goes out in the named group but is refused on the way back by
// ze's own checks of the answer (verifyAcceptedOffer, childRekeyKeys), which would loop
// the refusal against a peer that has already said yes.
func TestChildRekeyCompletesInTheGroupThePeerNamed(t *testing.T) {
	log := slogutil.DiscardLogger()
	r := refusedRigTwoGroups(t)
	// The peer runs PFS in group 19. A Ze responder demands the group of its IKE SA, so
	// setting that group on the peer's end makes it accept KEi in 19 and nothing else.
	r.peer.Proposal.DHGroup.ID = 19
	peerESP := testESPGroup()
	peerESP.PFS = ipsec.PFSEnable
	peerDP := &rkyDP{}
	peerOld, err := createFirstChildSA(r.peer, peerESP, "10.0.0.2", "10.0.0.1", 1, peerDP, log)
	if err != nil {
		t.Fatalf("createFirstChildSA for the peer: %v", err)
	}
	var peerNew *ChildSA
	respond := func(req []wire.PayloadEntry, msgID uint32) []byte {
		answer, child, err := respondChildRekey(r.peer, req, peerOld, msgID, peerDP, log)
		if err != nil {
			t.Fatalf("the peer refused the request outright: %v", err)
		}
		peerNew = child
		return answer
	}

	// RFC 7296 Section 1.3: "There are two octets of data associated with this
	// notification: the accepted Diffie-Hellman group number in big endian order."
	refuse := func(_ []wire.PayloadEntry, msgID uint32) []byte {
		notify := &wire.PayloadNotify{NotifyMsgType: wire.NotifyInvalidKEPayload, NotificationData: []byte{0, 19}}
		answer, err := buildEncryptedMessageEx(r.peer, []wire.PayloadEntry{{Payload: notify}},
			msgID, wire.ExchangeCreateChildSA, initiatorFlag(r.peer)|wire.FlagResponse)
		if err != nil {
			t.Fatalf("build the peer's INVALID_KE_PAYLOAD answer: %v", err)
		}
		return answer
	}

	r.ps.startChildRekey(r.sa, r.myTr, log)
	req, out := r.relayToPeer(t, refuse)
	if got := rkyFindKE(t, req).DHGroup; got != 14 {
		t.Fatalf("the first request carried KEi in group %d, want the first configured group 14", got)
	}
	if out.newChild != nil {
		t.Fatal("an INVALID_KE_PAYLOAD answer installed a replacement Child SA")
	}
	if r.ps.getChildSA() != r.old {
		t.Fatal("the refusal replaced the live Child SA")
	}
	if r.dp.wasRemoved(r.old.InboundSPI) {
		t.Fatal("the refusal removed the live Child SA from the dataplane")
	}

	r.ps.startChildRekey(r.sa, r.myTr, log)
	req, out = r.relayToPeer(t, respond)
	if got := rkyFindKE(t, req).DHGroup; got != 19 {
		t.Fatalf("the retry carried KEi in group %d on the wire, want the named group 19", got)
	}
	if peerNew == nil {
		t.Fatal("the peer did not accept the retry in group 19")
	}
	if out.newChild == nil {
		t.Fatal("ze refused the peer's acceptance of the retry in group 19")
	}
	if r.ps.getChildSA() != out.newChild {
		t.Fatal("the replacement Child SA is not the live one")
	}
	if r.dp.installedSA(out.newChild.InboundSPI) == nil {
		t.Fatal("the replacement Child SA was not installed")
	}
	if out.newChild.OutboundSPI != peerNew.InboundSPI {
		t.Fatalf("ze sends on SPI %#x, the peer receives on %#x", out.newChild.OutboundSPI, peerNew.InboundSPI)
	}
	if !bytes.Equal(out.newChild.Keys.EncryptKeyI, peerNew.Keys.EncryptKeyI) ||
		!bytes.Equal(out.newChild.Keys.EncryptKeyR, peerNew.Keys.EncryptKeyR) {
		t.Fatal("the two ends derived different keys for the replacement Child SA")
	}
	if r.ps.pendingRekey != nil {
		t.Fatal("the completed rekey is still outstanding")
	}
}

// VALIDATES: the same, for an IKE SA rekey. The peer is configured for group 19 only,
// so its real respondIKERekey refuses KEi in group 14 with INVALID_KE_PAYLOAD naming
// 19, and accepts the retry. The new IKE SA runs group 19, and both ends hold the same
// SK_d.
// PREVENTS: a retry whose DH value is computed in the named group while its KE payload
// still names the first configured group, which the peer refuses again.
func TestIKERekeyCompletesInTheGroupThePeerNamed(t *testing.T) {
	log := slogutil.DiscardLogger()
	r := refusedRigTwoGroups(t)
	r.peer.IKEGroup.Proposals = []ipsec.IKEProposal{r.sa.IKEGroup.Proposals[1]}
	if r.peer.IKEGroup.Proposals[0].DHGroup != 19 {
		t.Fatal("the rig's second IKE proposal is not group 19")
	}
	var peerNew *SA
	respond := func(req []wire.PayloadEntry, msgID uint32) []byte {
		answer, sa, err := respondIKERekey(r.peer, req, msgID, log)
		if err != nil {
			t.Fatalf("the peer refused the request outright: %v", err)
		}
		peerNew = sa
		return answer
	}

	r.ps.startIKERekey(r.sa, r.sa.IKEGroup, r.myTr, log)
	req, out := r.relayToPeer(t, respond)
	if got := rkyFindKE(t, req).DHGroup; got != 14 {
		t.Fatalf("the first request carried KEi in group %d, want the first configured group 14", got)
	}
	if peerNew != nil || out.newSA != nil {
		t.Fatal("the peer was expected to refuse KEi in group 14 with INVALID_KE_PAYLOAD")
	}
	if out.reestablish {
		t.Fatal("the refusal tore the current IKE SA down")
	}

	r.ps.startIKERekey(r.sa, r.sa.IKEGroup, r.myTr, log)
	req, out = r.relayToPeer(t, respond)
	if got := rkyFindKE(t, req).DHGroup; got != 19 {
		t.Fatalf("the retry carried KEi in group %d on the wire, want the named group 19", got)
	}
	if peerNew == nil {
		t.Fatal("the peer did not accept the retry in group 19")
	}
	defer peerNew.SKKeys.Clear()
	if out.newSA == nil {
		t.Fatal("ze refused the peer's acceptance of the retry in group 19")
	}
	defer out.newSA.SKKeys.Clear()
	if got := out.newSA.Proposal.DHGroup.ID; got != 19 {
		t.Fatalf("the new IKE SA runs group %d, want 19", got)
	}
	if !bytes.Equal(out.newSA.SKKeys.SK_d, peerNew.SKKeys.SK_d) {
		t.Fatal("the two ends derived different SK_d for the new IKE SA")
	}
}

// The owner's wait after every configured proposal has been refused: 15 seconds, less
// up to 10% jitter, so peers refused together do not retry together.
const (
	refusalWaitMax = 15 * time.Second
	refusalWaitMin = refusalWaitMax - refusalWaitMax/10
)

// expectRefusalHold fails the test unless until, the end of a refusal hold armed between
// before and after, lies in the owner's window of 15 seconds less up to 10%.
func expectRefusalHold(t *testing.T, until, before, after time.Time, what string) {
	t.Helper()
	if until.IsZero() {
		t.Fatalf("%s: no refusal hold was armed", what)
	}
	if until.Before(before.Add(refusalWaitMin)) {
		t.Fatalf("%s: the hold ends %v after the refusal, shorter than %v", what, until.Sub(before), refusalWaitMin)
	}
	if until.After(after.Add(refusalWaitMax)) {
		t.Fatalf("%s: the hold ends %v after the refusal, longer than %v", what, until.Sub(after), refusalWaitMax)
	}
}

// VALIDATES: a rekey refused for every configured proposal waits 15 seconds less up to
// 10% jitter before it starts over, on both rekey paths. The method answers one rekey
// NO_PROPOSAL_CHOSEN end to end and reads the hold it armed, then refuses 64 more
// directly through refuseRekey, so the random jitter is sampled across its range.
// PREVENTS: a wait the owner did not set. A fixed one-second wait is the resend loop
// this fix exists to stop, and a long one leaves an SA past its soft lifetime with no
// rekey until the hard lifetime ends it.
func TestRekeyRefusedForEveryProposalWaitsFifteenSeconds(t *testing.T) {
	paths := []struct {
		name   string
		kind   rekeyKind
		answer func(r *refusedRig, t *testing.T, inner []wire.PayloadEntry) ownedOutcome
		hold   func(ps *PeerSession) time.Time
	}{
		{"child", rekeyChild, (*refusedRig).answerChildRekey,
			func(ps *PeerSession) time.Time { return ps.childRekeyRefusedUntil }},
		{"ike", rekeyIKE, (*refusedRig).answerIKERekey,
			func(ps *PeerSession) time.Time { return ps.ikeRekeyRefusedUntil }},
	}
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			log := slogutil.DiscardLogger()
			r := newRefusedRig(t)
			before := time.Now()
			path.answer(r, t, refusedNotify(wire.NotifyNoProposalChosen, nil))
			expectRefusalHold(t, path.hold(r.ps), before, time.Now(), "after NO_PROPOSAL_CHOSEN")

			for range 64 {
				before = time.Now()
				r.ps.refuseRekey(r.sa, &pendingRekey{kind: path.kind}, dhGroupNone, log)
				expectRefusalHold(t, path.hold(r.ps), before, time.Now(), "a direct refusal")
			}
		})
	}
}

// VALIDATES: a Child SA rekey sent without KEi, because the operator left PFS off, and
// answered INVALID_KE_PAYLOAD is a refusal of the whole offer: ze waits, and does not
// resend. The method configures groups 14 and 19 and has the peer name 19, a group the
// operator configured, so only the absent KEi keeps ze from retrying.
// PREVENTS: the one-second loop again. A request without KEi has no group to change, so
// a "retry" would resend the same KEi-less rekey, and the group it sent is never
// recorded as refused, so every owner-loop tick would send it once more.
func TestChildRekeyInvalidKEWithoutPFSIsARefusal(t *testing.T) {
	log := slogutil.DiscardLogger()
	r := refusedRigTwoGroups(t)
	r.old.ESPGroup.PFS = ipsec.PFSDisable
	remote := r.sa.remoteUDPAddr()

	before := time.Now()
	r.answerChildRekey(t, refusedNotify(wire.NotifyInvalidKEPayload, []byte{0, 19}))
	expectRefusalHold(t, r.ps.childRekeyRefusedUntil, before, time.Now(), "INVALID_KE_PAYLOAD without PFS")

	r.ps.startChildRekey(r.sa, r.myTr, log)
	rtxExpectSilence(t, r.peerTr, r.myTr, remote, "a KEi-less rekey resent after INVALID_KE_PAYLOAD")
}

// VALIDATES: INVALID_KE_PAYLOAD naming a group the operator did not configure for this
// peer is a refusal on both rekey paths: ze waits, and sends nothing in that group. The
// method configures groups 14 and 19 and has the peer name 20, which ze supports.
// PREVENTS: a peer choosing the Diffie-Hellman group of an SA against the operator's
// configuration. The owner's rule is that ze retries only in a group configured here.
func TestRekeyInvalidKENamingAnUnconfiguredGroupIsARefusal(t *testing.T) {
	named := []byte{0, 20}
	t.Run("child", func(t *testing.T) {
		log := slogutil.DiscardLogger()
		r := refusedRigTwoGroups(t)
		remote := r.sa.remoteUDPAddr()

		before := time.Now()
		r.answerChildRekey(t, refusedNotify(wire.NotifyInvalidKEPayload, named))
		expectRefusalHold(t, r.ps.childRekeyRefusedUntil, before, time.Now(), "Child SA rekey, group 20 named")

		r.ps.startChildRekey(r.sa, r.myTr, log)
		rtxExpectSilence(t, r.peerTr, r.myTr, remote, "a Child SA rekey sent in a group the operator did not configure")
	})
	t.Run("ike", func(t *testing.T) {
		log := slogutil.DiscardLogger()
		r := refusedRigTwoGroups(t)
		remote := r.sa.remoteUDPAddr()

		before := time.Now()
		r.answerIKERekey(t, refusedNotify(wire.NotifyInvalidKEPayload, named))
		expectRefusalHold(t, r.ps.ikeRekeyRefusedUntil, before, time.Now(), "IKE SA rekey, group 20 named")

		r.ps.startIKERekey(r.sa, r.sa.IKEGroup, r.myTr, log)
		rtxExpectSilence(t, r.peerTr, r.myTr, remote, "an IKE SA rekey sent in a group the operator did not configure")
	})
}
