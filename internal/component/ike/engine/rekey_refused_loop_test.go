package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// refusedLoopQuiet is how long the peer listens after its refusal. It covers several
// one-second owner-loop ticks and the 3 s rekey retransmit timeout, so a resend on the
// next tick and a retransmission of an unconsumed request are both inside it.
const refusedLoopQuiet = rekeyRetransmitTimeout + 1500*time.Millisecond

// VALIDATES: a Child SA rekey the peer refuses with NO_PROPOSAL_CHOSEN, delivered as an
// encrypted datagram through the owner loop's inbound channel, is not sent again inside
// the owner's wait. The method runs maintainSA under a soft-expired Child SA lifetime,
// reads the rekey ze sends, answers it with an encrypted N(NO_PROPOSAL_CHOSEN) on the
// real inbound path (classifyInbound, decryptAndParse, handleCreateChildSAOwned), and
// then requires silence on the peer's socket across several ticks and a retransmit
// timeout.
// PREVENTS: the double refusal seen against strongSwan in
// child-rekey-refused-keeps-the-sa, where a second refused rekey followed the first
// inside one second. TestChildRekeyRefusalHoldsTheRetry calls the response handler
// directly, so it does not cover the inbound path this test drives.
func TestChildRekeyRefusedOnTheWireWaits(t *testing.T) {
	r := newRefusedRig(t)
	remote := r.sa.remoteUDPAddr()
	done := clfRunLoop(t, r.sa, r.ps, nil, winSoftExpired(), nil, r.myTr, &sepBus{})

	raw := rtxRecv(t, r.peerTr)
	if raw == nil {
		t.Fatal("the soft-expired Child SA sent no rekey")
	}
	hdr := parseMsg(t, raw).Header
	if hdr.ExchangeType != wire.ExchangeCreateChildSA {
		t.Fatalf("the first request is exchange %d, want CREATE_CHILD_SA", hdr.ExchangeType)
	}
	notify := &wire.PayloadNotify{NotifyMsgType: wire.NotifyNoProposalChosen}
	answer, err := buildEncryptedMessageEx(r.peer, []wire.PayloadEntry{{Payload: notify}},
		hdr.MessageID, wire.ExchangeCreateChildSA, initiatorFlag(r.peer)|wire.FlagResponse)
	if err != nil {
		t.Fatalf("build the peer's NO_PROPOSAL_CHOSEN answer: %v", err)
	}
	refusedAt := time.Now()
	r.ps.inbound <- transport.Packet{Data: answer, RemoteAddr: remote}

	select {
	case pkt := <-r.peerTr.Recv():
		resent := parseMsg(t, pkt.Data).Header
		t.Fatalf("ze sent exchange %d (message ID %d, first rekey %d) %v after the peer refused the rekey, inside the owner's wait of 13.5 to 15 s",
			resent.ExchangeType, resent.MessageID, hdr.MessageID, time.Since(refusedAt))
	case <-time.After(refusedLoopQuiet):
	}

	close(r.ps.stopCh)
	if err := clfLoopEnd(t, done, "an operator stop"); err != nil {
		t.Fatalf("an operator stop ended the loop with %v, want nil", err)
	}
	expectRefusalHold(t, r.ps.childRekeyRefusedUntil, refusedAt, time.Now(), "a refusal on the wire")
}
