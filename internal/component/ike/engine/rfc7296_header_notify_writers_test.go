// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- unprotected notify emitters
// Related: rfc7296_header_test.go -- the X and V bit obligations on the engine's other messages
//
// VALIDATES: the two header writers that engineBuiltMessages does not sample clear the
// V bit and the X bits in the flags octet of every message they send. sendSAInitNotifyRaw
// writes the COOKIE challenge and the IKE_SA_INIT error notifies (NO_PROPOSAL_CHOSEN,
// INVALID_KE_PAYLOAD). sendInvalidIKESPI writes the unprotected INVALID_IKE_SPI answer to
// a datagram that names no known SA.
// PREVENTS: one of those writers setting wire.FlagVersion or a reserved bit, or copying
// the flags of the request it answers, while the sampled writers stay clean.
//
// Method: each writer is driven through the function its production caller invokes, over
// a loopback socket, and the datagram the peer reads is parsed. The notify type proves
// which writer produced it, the Response bit proves the flags octet was read at the right
// offset, and the V and X masks are then asserted clear.
package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// hnwRequireCleanFlags parses one emitted datagram, requires its single Notify to be of
// the given type and its Response bit set, and requires the V and X bits clear.
func hnwRequireCleanFlags(t *testing.T, label string, raw []byte, notifyType uint16) {
	t.Helper()
	if raw == nil {
		t.Fatalf("%s: the writer sent nothing, so its flags cannot be judged", label)
	}
	var msg wire.Message
	if err := msg.ReadFrom(raw); err != nil {
		t.Fatalf("%s: the emitted datagram does not parse: %v", label, err)
	}
	if len(msg.Payloads) != 1 {
		t.Fatalf("%s: %d payloads, want one Notify", label, len(msg.Payloads))
	}
	notify, isNotify := msg.Payloads[0].Payload.(*wire.PayloadNotify)
	if !isNotify {
		t.Fatalf("%s: payload is %T, want a Notify", label, msg.Payloads[0].Payload)
	}
	if notify.NotifyMsgType != notifyType {
		t.Fatalf("%s: notify type %d, want %d", label, notify.NotifyMsgType, notifyType)
	}
	flags := raw[19]
	if flags != msg.Header.Flags {
		t.Fatalf("%s: octet 19 is %08b but the parsed flags are %08b", label, flags, msg.Header.Flags)
	}
	if flags&wire.FlagResponse == 0 {
		t.Fatalf("%s: flags = %08b, want the Response bit, or the octet read is not the flags", label, flags)
	}
	if flags&wire.FlagVersion != 0 {
		t.Errorf("%s: flags = %08b, the V bit must be cleared when sending", label, flags)
	}
	if flags&xBitMask != 0 {
		t.Errorf("%s: flags = %08b, X bits (mask %08b) must be cleared when sending", label, flags, xBitMask)
	}
}

// RFC requirement: RFC7296-3.1-11 positive -- sendSAInitNotifyRaw clears the V bit in
// every IKE_SA_INIT notify it sends: the COOKIE challenge sent by sendCookieChallenge, and
// the NO_PROPOSAL_CHOSEN and INVALID_KE_PAYLOAD responses sent by sendSAInitNotify.
func TestSAInitNotifiesClearXAndVBits(t *testing.T) {
	log := slogutil.DiscardLogger()
	peerTr, myTr := rtxPeerLink(t)
	remote, ok := peerTr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer transport local address is not *net.UDPAddr")
	}

	spiI := ntfSPI(0x10)
	cookie := make([]byte, cookieDataLen)
	for i := range cookie {
		cookie[i] = byte(i + 1)
	}
	sendCookieChallenge(myTr, nil, remote, spiI, cookie, "ze", log)
	hnwRequireCleanFlags(t, "COOKIE", rtxRecv(t, peerTr), wire.NotifyCookie)

	sa := &SA{PeerName: "ze", InitiatorSPI: ntfSPI(0x20), ResponderSPI: ntfSPI(0x30)}
	sendSAInitNotify(sa, myTr, remote, wire.NotifyNoProposalChosen, nil, log)
	hnwRequireCleanFlags(t, "NO_PROPOSAL_CHOSEN", rtxRecv(t, peerTr), wire.NotifyNoProposalChosen)

	sendSAInitNotify(sa, myTr, remote, wire.NotifyInvalidKEPayload, []byte{0, 19}, log)
	hnwRequireCleanFlags(t, "INVALID_KE_PAYLOAD", rtxRecv(t, peerTr), wire.NotifyInvalidKEPayload)
}

// RFC requirement: RFC7296-3.1-11 positive -- sendInvalidIKESPI clears the V bit in the
// INVALID_IKE_SPI it sends, including when the request it answers carries the V bit and
// every X bit, so the answer does not copy the request's flags.
func TestInvalidIKESPIClearsXAndVBits(t *testing.T) {
	plain := ntfRequest(ntfSPI(0x40), ntfSPI(0x50), wire.ExchangeInformational, 7, false)
	hnwRequireCleanFlags(t, "INVALID_IKE_SPI", ntfEmit(t, plain, false), wire.NotifyInvalidIKESPI)

	marked := ntfRequest(ntfSPI(0x60), ntfSPI(0x70), wire.ExchangeInformational, 9, false)
	marked[19] |= wire.FlagVersion | xBitMask
	hnwRequireCleanFlags(t, "INVALID_IKE_SPI to a V and X marked request", ntfEmit(t, marked, false),
		wire.NotifyInvalidIKESPI)
}
