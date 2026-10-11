package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// VALIDATES: a half-open initiator SA that is waiting for an IKE_AUTH response,
// either the first one or one EAP round, neither answers nor changes state when an
// unprotected INVALID_IKE_SPI Notify arrives at its outstanding Message ID.
// PREVENTS: one forged datagram carrying the two SPIs killing an IKE SA before it
// establishes. handleAuthResponse and handleEAPResponse used to set StateDead on an
// IKE_AUTH response with no SK payload, while the established-SA path already
// dropped the same message.
//
// Method: the input is the bytes sendInvalidIKESPI writes when the initiator's
// IKE_AUTH request reaches a node that does not know these SPIs, so the exchange
// type, both SPIs and the Message ID are the ones the SA is waiting on. It enters
// through routeInbound, the dispatch entry point. A sentinel proves nothing was
// sent, and the SA's state, Message IDs, retransmission schedule and EAP session
// are compared with their values before delivery.
// RFC requirement: RFC7296-2.21.4-6 positive -- an unprotected INVALID_IKE_SPI
// Notify delivered through routeInbound to an initiator SA in StateAuthSent or
// StateEAPInProgress draws no datagram and leaves the SA's state, Message IDs,
// retransmission schedule and EAP session unchanged.
func TestErrUnprotectedNotifyLeavesHalfOpenSA(t *testing.T) {
	cases := []struct {
		name  string
		state SAState
	}{
		{name: "awaiting the IKE_AUTH response", state: StateAuthSent},
		{name: "awaiting an EAP round", state: StateEAPInProgress},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sa, peerTr, myTr, _, log := eapwSession(t)
			sa.State = tc.state
			remote, ok := peerTr.LocalAddr().(*net.UDPAddr)
			if !ok {
				t.Fatal("peer transport local address is not *net.UDPAddr")
			}

			msgID := sa.NextMsgID
			forged := ntfEmit(t, ntfRequest(sa.InitiatorSPI, sa.ResponderSPI, wire.ExchangeIKEAuth, msgID, false), false)
			if forged == nil {
				t.Fatal("the out-of-SA emitter wrote no INVALID_IKE_SPI to use as input")
			}
			var forgedMsg wire.Message
			if err := forgedMsg.ReadFrom(forged); err != nil {
				t.Fatalf("the INVALID_IKE_SPI input does not parse: %v", err)
			}
			if forgedMsg.Header.ExchangeType != wire.ExchangeIKEAuth {
				t.Fatalf("the input is exchange %d, want IKE_AUTH", forgedMsg.Header.ExchangeType)
			}
			if forgedMsg.Header.Flags&wire.FlagResponse == 0 {
				t.Fatalf("the input is flags %#x, want a response", forgedMsg.Header.Flags)
			}
			if forgedMsg.Header.MessageID != msgID {
				t.Fatalf("the input is at Message ID %d, want the outstanding %d", forgedMsg.Header.MessageID, msgID)
			}
			if len(forgedMsg.Payloads) != 1 {
				t.Fatalf("the input carries %d payloads, want one unprotected Notify", len(forgedMsg.Payloads))
			}
			if n, isNotify := forgedMsg.Payloads[0].Payload.(*wire.PayloadNotify); !isNotify || n.NotifyMsgType != wire.NotifyInvalidIKESPI {
				t.Fatalf("the input payload is %T, want an unprotected INVALID_IKE_SPI Notify", forgedMsg.Payloads[0].Payload)
			}

			beforeNext, beforeExpected := sa.NextMsgID, sa.ExpectedMsgID
			beforeRetransmitTime, beforeRetransmitCount := sa.RetransmitTime, sa.RetransmitCount
			beforeEAP := sa.EAPSession

			routeInbound(sa, transport.Packet{Data: forged, RemoteAddr: remote}, nil, myTr, log)

			rtxExpectSilence(t, peerTr, myTr, remote, "the unprotected INVALID_IKE_SPI Notify")
			if sa.State != tc.state {
				t.Errorf("state = %v, want the unchanged %v", sa.State, tc.state)
			}
			if sa.NextMsgID != beforeNext {
				t.Errorf("NextMsgID = %d, want the unchanged %d", sa.NextMsgID, beforeNext)
			}
			if sa.ExpectedMsgID != beforeExpected {
				t.Errorf("ExpectedMsgID = %d, want the unchanged %d", sa.ExpectedMsgID, beforeExpected)
			}
			if !sa.RetransmitTime.Equal(beforeRetransmitTime) {
				t.Errorf("RetransmitTime = %v, want the unchanged %v", sa.RetransmitTime, beforeRetransmitTime)
			}
			if sa.RetransmitCount != beforeRetransmitCount {
				t.Errorf("RetransmitCount = %d, want the unchanged %d", sa.RetransmitCount, beforeRetransmitCount)
			}
			if sa.EAPSession != beforeEAP {
				t.Error("the EAP session was replaced")
			}
		})
	}
}
