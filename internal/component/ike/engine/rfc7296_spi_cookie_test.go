// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the IKE_SA_INIT exchange and its cookie retry
// Related: rfc7296_cookie_test.go -- ckeCookieResponse; rfc7296_kegroup_test.go -- kegInitiator
// VALIDATES: the Responder's SPI Ze sends is zero in the first IKE_SA_INIT request and in
// its cookie repeat (RFC 7296 Section 3.1).
// PREVENTS: a cookie repeat that copies the Responder's SPI of the challenge it answers.

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC7296ResponderSPIIsZeroInTheFirstMessageAndItsCookieRepeat proves the Responder's
// SPI field is zero in the first IKE_SA_INIT request Ze sends, and again in the repeat of
// that request that carries a cookie.
//
// Goal: RFC 7296 Section 3.1 names the cookie repeat explicitly, and it is the case a
// builder can get wrong: the challenge arrives in a message that has a Responder's SPI
// field, so a builder that copied it would send a non-zero value. Method: the responder's
// COOKIE challenge carries the non-zero Responder's SPI 0909090909090909. Ze's first request
// and its cookie repeat are each read at octets 8 to 15 of the header, and the repeat is
// checked to carry the COOKIE notify, so the message measured is the repeat and not a copy
// of the first request.
//
// RFC 7296 Section 3.1: "Responder's SPI (8 octets) - A value chosen by the responder to
// identify a unique IKE Security Association. This value MUST be zero in the first message
// of an IKE initial exchange (including repeats of that message including a cookie)."
//
// RFC requirement: RFC7296-3.1-4 positive -- the first IKE_SA_INIT request and its cookie repeat each carry a zero Responder's SPI, although the COOKIE challenge carried the non-zero SPI 0909090909090909.
func TestRFC7296ResponderSPIIsZeroInTheFirstMessageAndItsCookieRepeat(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa, table := kegInitiator(t, kegIKEGroup())

	first := parseMsg(t, sa.LastSentMsg)
	if first.Header.ResponderSPI != ([8]byte{}) {
		t.Fatalf("the first IKE_SA_INIT request carries Responder's SPI %x, want zero", first.Header.ResponderSPI)
	}

	cookie := []byte{0xde, 0xad, 0xbe, 0xef}
	challenge := ckeCookieResponse(sa.InitiatorSPI, [8]byte{9, 9, 9, 9, 9, 9, 9, 9}, cookie)
	handleSAInitResponse(sa, parseMsg(t, challenge), challenge, table, nil, nil, log)
	if sa.State != StateSAInitSent {
		t.Fatalf("state = %v, want sa-init-sent: the COOKIE challenge was not answered", sa.State)
	}

	repeat := parseMsg(t, sa.LastSentMsg)
	if len(repeat.Payloads) == 0 {
		t.Fatal("the cookie repeat carries no payloads")
	}
	notify, ok := repeat.Payloads[0].Payload.(*wire.PayloadNotify)
	if !ok || notify.NotifyMsgType != wire.NotifyCookie {
		t.Fatalf("payload 0 of the repeat is %T, want the COOKIE notify", repeat.Payloads[0].Payload)
	}
	if repeat.Header.InitiatorSPI != first.Header.InitiatorSPI {
		t.Fatalf("the repeat carries Initiator's SPI %x, want the first request's %x",
			repeat.Header.InitiatorSPI, first.Header.InitiatorSPI)
	}
	if repeat.Header.ResponderSPI != ([8]byte{}) {
		t.Fatalf("the cookie repeat carries Responder's SPI %x, want zero", repeat.Header.ResponderSPI)
	}
	for i := 8; i < 16; i++ {
		if sa.LastSentMsg[i] != 0 {
			t.Fatalf("octet %d of the cookie repeat is %02x, want 00", i, sa.LastSentMsg[i])
		}
	}
}
