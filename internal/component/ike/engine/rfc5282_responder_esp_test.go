// Design: docs/architecture/ike.md -- RFC 5282 Section 8 on the responder's Child SA selection.
// Related: rfc5282_aead_send_test.go -- the initiator's ESP offer and its re-check.

package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// aeadSAi2 returns a peer's SAi2 offering AES-GCM-256 with normal sequence numbers, plus
// the extra transforms the caller names.
func aeadSAi2(extra ...wire.Transform) *wire.PayloadSA {
	transforms := []wire.Transform{
		espEncTransform(uint16(crypto.ENCR_AES_GCM_16), 256),
		{Type: wire.TransformTypeESN, ID: espESNNotExtended},
	}
	transforms = append(transforms, extra...)
	return &wire.PayloadSA{Proposals: []wire.Proposal{{
		Number: 3, ProtocolID: wire.ProtocolESP, SPISize: 4,
		SPI: []byte{0x0a, 0x0b, 0x0c, 0x0d}, Transforms: transforms,
	}}}
}

// RFC requirement: RFC5282-8-1 positive -- the ESP SA the RESPONDER selects: a peer's SAi2
// offering AES GCM alone is accepted by selectResponderESP although the configured
// proposal names a hash, and the SAr2 proposal ze answers with (espProposalToWire over the
// selected proposal and the peer's number, as buildChildSAResponsePayloads builds it)
// carries AES GCM and no integrity transform.
//
// RFC 5282 Section 8: "This document updates [RFC4306] to require that when an
// authenticated encryption algorithm is selected as the encryption algorithm for any SA
// (IKE or ESP), an integrity algorithm MUST NOT be selected for that SA."
//
// Method: the product selection entry point over a hand-built SAi2, then the product
// encoder of the answer.
func TestRFC5282ResponderSelectsNoIntegrityForAnAEADChildSA(t *testing.T) {
	sa := &SA{ESPGroup: aeadESPGroup()}
	if err := selectResponderESP(sa, aeadSAi2()); err != nil {
		t.Fatalf("selectResponderESP refused an AES-GCM-only SAi2: %v", err)
	}
	if sa.ChildProposalNum != 3 {
		t.Fatalf("the accepted proposal number is %d, want the peer's 3", sa.ChildProposalNum)
	}
	answer := espProposalToWire(sa.ESPGroup.Proposals[0], 0x01020304, sa.ChildProposalNum, dhGroupNone)
	if got := integrityTransformCount(answer); got != 0 {
		t.Fatalf("the SAr2 answer to an AES-GCM offer carries %d integrity transforms, want 0", got)
	}
	encs := 0
	for _, tr := range answer.Transforms {
		if tr.Type == wire.TransformTypeENCR {
			encs++
			if tr.ID != uint16(crypto.ENCR_AES_GCM_16) {
				t.Fatalf("the SAr2 cipher is %d, want AES GCM (%d)", tr.ID, crypto.ENCR_AES_GCM_16)
			}
		}
	}
	if encs != 1 {
		t.Fatalf("the SAr2 answer carries %d encryption transforms, want 1", encs)
	}
}

// RFC requirement: RFC5282-8-1 negative -- the ESP SA the RESPONDER selects: a peer's SAi2
// offering AES GCM with HMAC-SHA2-256-128 as its only integrity transform is refused with
// NO_PROPOSAL_CHOSEN and no proposal is accepted, so the responder never selects an
// integrity algorithm for an AEAD Child SA.
func TestRFC5282ResponderRefusesAnAEADChildSAOfferWithIntegrity(t *testing.T) {
	sa := &SA{ESPGroup: aeadESPGroup()}
	offer := aeadSAi2(wire.Transform{
		Type: wire.TransformTypeINTG, ID: uint16(crypto.AUTH_HMAC_SHA2_256_128),
	})
	err := selectResponderESP(sa, offer)
	if !errors.Is(err, crypto.ErrNoProposalChosen) {
		t.Fatalf("selectResponderESP over AES-GCM + HMAC returned %v, want ErrNoProposalChosen", err)
	}
	if sa.ChildProposalNum != 0 {
		t.Fatalf("a proposal (number %d) was accepted, want none", sa.ChildProposalNum)
	}
}
