package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// refusedNotify returns the payload chain of a CREATE_CHILD_SA response that carries
// nothing but one error notify. RFC 7296 Section 3.10.1: an error notify in a response
// "indicates that the request has failed", so such a response carries no keys.
func refusedNotify(notifyType uint16, data []byte) []wire.PayloadEntry {
	return []wire.PayloadEntry{
		{Payload: &wire.PayloadNotify{NotifyMsgType: notifyType, NotificationData: data}},
	}
}

// refusedRig is one established PeerSession with a live Child SA and a UDP link to a
// stand-in peer, so a test can watch what ze sends after a refused rekey.
type refusedRig struct {
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
	_, sa, ps := establishPSK(t)
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
	return &refusedRig{sa: sa, ps: ps, old: old, dp: dp, peerTr: peerTr, myTr: myTr}
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
