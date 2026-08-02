// RFC 7296 Section 1.3.3, the REKEY_SA notification of a Child SA rekey.
//
// Section 1.3.3 is "Rekeying Child SAs". It makes the notification conditional: a
// CREATE_CHILD_SA carries REKEY_SA when the purpose of the exchange is to replace an
// existing ESP or AH SA, and the notification names that SA by SPI. The sibling exchange
// of Section 1.3.2 rekeys the IKE SA over the same exchange type and carries none, which
// is what makes the condition observable.
//
// Helpers here start with `rksa`, so they cannot collide with the sibling
// `rfc7296_rekey_test.go` (`rky`) that covers Sections 1.3, 2.8 and 2.8.1.

package engine

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// rksaRekeySANotify returns the REKEY_SA notification of a payload chain, or nil when the
// chain carries none. It walks the DECRYPTED payloads of a message that was built and
// encrypted, so what it reports is what a peer reads off the wire rather than a field the
// test set on a struct.
func rksaRekeySANotify(inner []wire.PayloadEntry) *wire.PayloadNotify {
	for i := range inner {
		if n, ok := inner[i].Payload.(*wire.PayloadNotify); ok && n.NotifyMsgType == wire.NotifyRekeySA {
			return n
		}
	}
	return nil
}

// VALIDATES: a CREATE_CHILD_SA whose purpose is to replace an ESP SA carries the REKEY_SA
// notification, and the notification names the SA being replaced. A CREATE_CHILD_SA with
// any other purpose carries none.
// PREVENTS: a Child rekey a peer reads as a request for an ADDITIONAL Child SA, which
// leaves the retired pair installed and doubles the Child SAs on both ends.
// RFC requirement: RFC7296-1.3.3-2 positive -- initiateChildRekey (rekey.go) puts a
// PayloadNotify of type NotifyRekeySA first in the request it builds. The assertion reads
// the notification back out of the ENCRYPTED message the peer decrypts, and it checks the
// Protocol ID and the SPI, so the notification identifies the ESP SA the exchange
// replaces. hasRekeySANotify (inbound.go) is the recognizer the receive path uses to
// classify the exchange, and it agrees.
// RFC requirement: RFC7296-1.3.3-2 negative -- initiateIKERekey (rekey.go) builds a
// CREATE_CHILD_SA of the SAME exchange type whose purpose is to replace the IKE SA, not an
// ESP or AH SA. It carries no REKEY_SA notification. The notification therefore follows the
// purpose of the exchange, and is not a payload the builder writes into every
// CREATE_CHILD_SA whatever it is for.
func TestRksaChildRekeyCarriesTheRekeySANotify(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)

	old, err := createFirstChildSA(ini, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, nil, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	defer old.Clear()

	reqBytes, pending, err := initiateChildRekey(ini, old)
	if err != nil {
		t.Fatalf("initiateChildRekey: %v", err)
	}
	defer pending.clear()

	req := parseMsg(t, reqBytes)
	if req.Header.ExchangeType != wire.ExchangeCreateChildSA {
		t.Fatalf("child rekey exchange = %d, want CREATE_CHILD_SA", req.Header.ExchangeType)
	}
	reqInner, err := decryptAndParse(resp, req, reqBytes)
	if err != nil {
		t.Fatalf("the peer could not decrypt the child rekey request: %v", err)
	}

	notify := rksaRekeySANotify(reqInner)
	if notify == nil {
		t.Fatal("the child rekey request carries no REKEY_SA notification")
	}
	if notify.ProtocolID != wire.ProtocolESP {
		t.Errorf("REKEY_SA protocol id = %d, want %d (ESP, the protocol of the SA being replaced)",
			notify.ProtocolID, wire.ProtocolESP)
	}
	if notify.SPISize != 4 || len(notify.SPI) != 4 {
		t.Fatalf("REKEY_SA SPI size = %d, SPI length = %d, want 4 and 4", notify.SPISize, len(notify.SPI))
	}
	if got := binary.BigEndian.Uint32(notify.SPI); got != old.InboundSPI {
		t.Errorf("REKEY_SA names SPI %d, want %d (the inbound SPI of the SA being replaced)", got, old.InboundSPI)
	}
	if !hasRekeySANotify(reqInner) {
		t.Error("the receive path does not classify the request as a rekey")
	}

	// Negative. Rekeying the IKE SA is the same exchange type with a different purpose. It
	// replaces no ESP or AH SA, so the condition the obligation names is not met and the
	// request carries no REKEY_SA.
	ikeBytes, ikePending, err := initiateIKERekey(ini, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer ikePending.clear()

	ikeReq := parseMsg(t, ikeBytes)
	if ikeReq.Header.ExchangeType != wire.ExchangeCreateChildSA {
		t.Fatalf("ike rekey exchange = %d, want CREATE_CHILD_SA", ikeReq.Header.ExchangeType)
	}
	ikeInner, err := decryptAndParse(resp, ikeReq, ikeBytes)
	if err != nil {
		t.Fatalf("the peer could not decrypt the ike rekey request: %v", err)
	}
	if n := rksaRekeySANotify(ikeInner); n != nil {
		t.Errorf("an IKE SA rekey carries a REKEY_SA notification for protocol %d", n.ProtocolID)
	}
	if hasRekeySANotify(ikeInner) {
		t.Error("the receive path classifies an IKE SA rekey as the replacement of an ESP SA")
	}
}
