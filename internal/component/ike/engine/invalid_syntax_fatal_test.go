package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: sending INVALID_SYNTAX also ENDS ZE'S OWN IKE SA.
//
// RFC 7296 Section 2.21.3 (rfc/full/rfc7296.txt:3341-3345): when a node
// "returns an INVALID_SYNTAX notification, then this error notification is considered fatal in both peers, meaning that the IKE SA is deleted without needing an explicit Delete payload."
//
// PREVENTS: the half-open SA left behind by the fix that added the emission. respondError
// (notify_error.go) built the notification, cached it and sent it, and marked nothing. The
// peer discarded its half on receipt, exactly as the sentence requires, while ze went on
// encrypting to it until dead-peer detection noticed minutes later.
//
// The assertion is on the STATE the owner loop reads on its tick (established.go), because
// that is what actually ends the session. A log line would not.
func TestIsfInvalidSyntaxDeletesZesOwnIKESA(t *testing.T) {
	f := delSession(t)

	if f.local.State != StateEstablished {
		t.Fatalf("the fixture SA is in state %v, so its death below proves nothing", f.local.State)
	}
	// A Delete whose ESP SPI Size is not the four octets RFC 7296 Section 3.11 fixes. The
	// pre-scan in handleInformationalOwned answers it with INVALID_SYNTAX.
	bad := &wire.PayloadDelete{
		ProtocolID: wire.ProtocolESP,
		SPISize:    3,
		NumSPIs:    1,
		SPIs:       []byte{0xde, 0xad, 0xbe},
	}
	inner, _ := f.inbound(t, bad)

	// The precondition: this really is the INVALID_SYNTAX path, and not some other refusal.
	notifies := delNotifies(inner)
	if len(notifies) != 1 || notifies[0].NotifyMsgType != wire.NotifyInvalidSyntax {
		t.Fatalf("the malformed Delete drew %d notifies (first type %v), want exactly one "+
			"INVALID_SYNTAX; the fatality below is untested without it",
			len(notifies), isfFirstType(notifies))
	}

	if f.local.State != StateDead {
		t.Errorf("ze sent INVALID_SYNTAX and left its own SA in state %v, want StateDead (%v); "+
			"the peer has already discarded its half, so ze is encrypting to nobody",
			f.local.State, StateDead)
	}
}

// VALIDATES: the fatality above is chosen by the NOTIFY TYPE, not attached to every refusal.
//
// This is the discriminator, and it is what stops the fix from being "kill the SA whenever
// an error goes out". RFC 7296 Section 2.21.3 makes INVALID_SYNTAX fatal and says nothing
// of the kind about the rest. Section 2.7 has NO_PROPOSAL_CHOSEN answer a Child SA rekey ze
// will not accept, and closing the IKE SA there would take a working tunnel down over one
// drifted algorithm -- the outcome the rekey refusal path exists to avoid (inbound.go).
//
// It drives respondError directly, because that emitter is where the rule now lives and it
// is shared by every authenticated-path refusal: respondInnerParseError, the malformed
// Delete pre-scan, and the two rekey refusals notifyForRefusal maps.
func TestIsfOtherErrorNotifiesLeaveTheIKESAUp(t *testing.T) {
	f := delSession(t)
	log := slogutil.DiscardLogger()

	f.ps.respondError(f.local, f.local.ExpectedMsgID, wire.ExchangeCreateChildSA,
		wire.NotifyNoProposalChosen, nil, f.myTr, log)

	if raw := rtxRecv(t, f.peerTr); raw == nil {
		t.Fatal("the refusal sent nothing, so this test measures no emission at all")
	}
	if f.local.State != StateEstablished {
		t.Errorf("a NO_PROPOSAL_CHOSEN refusal left the SA in state %v, want StateEstablished "+
			"(%v); RFC 7296 Section 2.21.3 makes INVALID_SYNTAX fatal, not every error",
			f.local.State, StateEstablished)
	}
}

// isfFirstType names the first notify type in a chain, for a failure message that has to
// survive an empty chain.
func isfFirstType(notifies []*wire.PayloadNotify) string {
	if len(notifies) == 0 {
		return "none"
	}
	return wire.NotifyTypeName(notifies[0].NotifyMsgType)
}
