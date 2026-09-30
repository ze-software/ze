// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- notifications ze sends
// Related: rfc7296_inner_error_answers_test.go -- errLink, errInnerChainRequest and errInnerAnswer

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// ikespiSAInitNotifies returns the notify payloads of the IKE_SA_INIT request ze builds
// as initiator, decoded off the wire.
func ikespiSAInitNotifies(t *testing.T) []*wire.PayloadNotify {
	t.Helper()
	iniPeer, _ := responderTestPeers(ipsec.AuthPreSharedSecret, "ikesa-notify-spi")
	sa, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	raw := buildSAInitRequest(sa, testIKEGroup())
	var msg wire.Message
	if err := msg.ReadFrom(raw); err != nil {
		t.Fatalf("the IKE_SA_INIT request does not parse: %v", err)
	}
	var out []*wire.PayloadNotify
	for i := range msg.Payloads {
		if n, ok := msg.Payloads[i].Payload.(*wire.PayloadNotify); ok {
			out = append(out, n)
		}
	}
	return out
}

// ikespiCheckEmpty fails when a decoded notify carried a non-zero SPI Size or SPI octets.
// ReadFrom keeps the SPI Size octet as received and consumes exactly that many SPI
// octets, so a zero SPISize and a nil SPI mean the wire carried neither.
func ikespiCheckEmpty(t *testing.T, where string, n *wire.PayloadNotify) {
	t.Helper()
	if n.SPISize != 0 {
		t.Errorf("%s: notify %d was sent with SPI Size %d, want 0", where, n.NotifyMsgType, n.SPISize)
	}
	if len(n.SPI) != 0 {
		t.Errorf("%s: notify %d was sent with a %d-octet SPI field, want none", where, n.NotifyMsgType, len(n.SPI))
	}
}

// VALIDATES: every notification ze sends concerning the IKE SA carries SPI Size zero and
// an empty SPI field.
// PREVENTS: an IKE-SA notification sent with an SPI, which a peer reads as a Child SA's.
//
// METHOD: the notifies of the IKE_SA_INIT request ze builds as initiator, and the
// UNSUPPORTED_CRITICAL_PAYLOAD error ze answers a malformed protected request with, are
// decoded off the wire. The error answer's data must be its one octet, so no SPI octet
// was written ahead of it.
//
// RFC requirement: RFC7296-3.10-3 positive -- each notify of ze's IKE_SA_INIT request
// (at least one, NAT detection among them) and the UNSUPPORTED_CRITICAL_PAYLOAD notify ze
// sends in answer to a protected request is on the wire with SPI Size 0 and no SPI octets.
func TestRFC7296IKESANotifiesZeSendsCarryNoSPI(t *testing.T) {
	notifies := ikespiSAInitNotifies(t)
	if len(notifies) == 0 {
		t.Fatal("the IKE_SA_INIT request carries no notify, so this case checks nothing")
	}
	for _, n := range notifies {
		ikespiCheckEmpty(t, "IKE_SA_INIT request", n)
	}

	link := errLink(t)
	req := errInnerChainRequest(t, link.ini, link.resp.ExpectedMsgID, 200, []byte{0, 0x80, 0, 8, 1, 2, 3, 4})
	answer := errInnerAnswer(t, link, req)
	if len(answer) != 1 {
		t.Fatalf("the error answer carries %d notifies, want one", len(answer))
	}
	ikespiCheckEmpty(t, "error answer", answer[0])
	if len(answer[0].NotificationData) != 1 {
		t.Errorf("error answer data is %d octets, want the one-octet payload type", len(answer[0].NotificationData))
	}
}

// RFC requirement: RFC7296-3.10-3 negative -- the encoder's input pushed toward the
// violation: a notification concerning the IKE SA whose struct names Protocol ID 1 (IKE)
// and an SPI Size of 8 but holds no SPI octets is still sent with SPI Size 0, Protocol ID
// 0 and no SPI field, the body being the 4-octet fixed part and the data alone.
func TestRFC7296IKESANotifyWithAnSPISizeButNoSPIIsSentEmpty(t *testing.T) {
	n := &wire.PayloadNotify{
		ProtocolID:       1,
		SPISize:          8,
		NotifyMsgType:    wire.NotifyInitialContact,
		NotificationData: []byte{0xab},
	}
	buf := make([]byte, 64)
	written := n.WriteTo(buf, 0)
	if written != 5 {
		t.Fatalf("the notify body is %d octets, want 5 (fixed part and one data octet, no SPI)", written)
	}
	if buf[1] != 0 {
		t.Errorf("SPI Size sent as %d, want 0", buf[1])
	}
	if buf[0] != 0 {
		t.Errorf("Protocol ID sent as %d beside an empty SPI field, want 0", buf[0])
	}
	if buf[4] != 0xab {
		t.Errorf("octet 4 is %#x, want the notification data 0xab directly after the fixed part", buf[4])
	}
}
