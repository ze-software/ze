// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA teardown over INFORMATIONAL
// Related: rfc7296_delete_test.go -- the ESP size-3 refusal and the delFixture this file drives

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// VALIDATES: RFC 7296 Section 3.11 fixes the Delete payload's SPI Size per protocol: zero for
// IKE and four for AH and ESP. Every size other than the one its protocol fixes is refused
// with INVALID_SYNTAX and closes nothing, for all three protocols.
// PREVENTS: a check that knew only the ESP size, so an IKE Delete declaring a four-octet SPI
// or an AH Delete declaring eight octets was treated as a request naming nothing.
//
// Method: each case goes through handleInformationalOwned on an established session with an
// installed ESP pair (delFixture), and the response the peer receives is decrypted.
//
// RFC requirement: RFC7296-3.11-2 negative -- RFC 7296 Section 3.11: "It MUST be zero for IKE
// (SPI is in message header) or four for AH and ESP." An IKE Delete with SPI Size 4, an AH
// Delete with SPI Size 0 or 8, and an ESP Delete with SPI Size 0 each draw exactly one
// INVALID_SYNTAX notify, no Delete payload, and remove no installed SA.
func TestRFC7296DeleteWrongSPISizeForItsProtocolIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol uint8
		size     uint8
		spis     []byte
	}{
		{"IKE with SPI Size 4", wire.ProtocolIKE, 4, []byte{0, 0, 0, 1}},
		{"AH with SPI Size 0", wire.ProtocolAH, 0, nil},
		{"AH with SPI Size 8", wire.ProtocolAH, 8, []byte{0, 0, 0, 0, 0, 0, 0, 1}},
		{"ESP with SPI Size 0", wire.ProtocolESP, 0, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := delSession(t)
			numSPIs := uint16(0)
			if tc.size != 0 {
				numSPIs = 1
			}
			inner, out := f.inbound(t, &wire.PayloadDelete{
				ProtocolID: tc.protocol,
				SPISize:    tc.size,
				NumSPIs:    numSPIs,
				SPIs:       tc.spis,
			})

			notifies := delNotifies(inner)
			if len(notifies) != 1 {
				t.Fatalf("the Delete drew %d Notify payloads, want exactly 1 INVALID_SYNTAX", len(notifies))
			}
			if notifies[0].NotifyMsgType != wire.NotifyInvalidSyntax {
				t.Errorf("the Delete drew notify %d, want INVALID_SYNTAX (%d)",
					notifies[0].NotifyMsgType, wire.NotifyInvalidSyntax)
			}
			if dels := lcyDeletes(inner); len(dels) != 0 {
				t.Errorf("the refusal carried %d Delete payloads, want none", len(dels))
			}
			if f.dp.wasRemoved(f.child.InboundSPI) || f.dp.wasRemoved(f.child.OutboundSPI) {
				t.Error("a Delete with the wrong SPI Size removed the live Child SA")
			}
			if out.reestablish {
				t.Error("a Delete with the wrong SPI Size took the tunnel down")
			}
		})
	}
}

// RFC requirement: RFC7296-3.11-2 positive -- the size Section 3.11 fixes for each protocol is
// accepted by the same handler: an IKE Delete with SPI Size 0 and an AH Delete with SPI Size 4
// (naming an SPI ze does not hold) each draw no Notify, so the refusals above are the size
// being checked per protocol and not a handler that refuses IKE or AH Deletes outright.
func TestRFC7296DeleteSPISizeFixedForItsProtocolIsAccepted(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol uint8
		size     uint8
		numSPIs  uint16
		spis     []byte
	}{
		{"IKE with SPI Size 0", wire.ProtocolIKE, 0, 0, nil},
		{"AH with SPI Size 4", wire.ProtocolAH, 4, 1, []byte{0x0a, 0x0b, 0x0c, 0x0d}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := delSession(t)
			inner, _ := f.inbound(t, &wire.PayloadDelete{
				ProtocolID: tc.protocol,
				SPISize:    tc.size,
				NumSPIs:    tc.numSPIs,
				SPIs:       tc.spis,
			})
			if notifies := delNotifies(inner); len(notifies) != 0 {
				t.Errorf("a Delete with the size its protocol fixes drew %d Notify payloads, want none",
					len(notifies))
			}
		})
	}
}
