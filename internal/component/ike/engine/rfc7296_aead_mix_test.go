package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// aeadOurs returns the three values espProposalMatches compares for an AEAD proposal.
// RFC 7296 Section 3.3: an AEAD proposal carries no integrity transform, so the
// integrity id ze compares against is zero.
func aeadOurs(t *testing.T) (encID, keyLen uint16) {
	t.Helper()
	our := ipsec.ESPProposal{Number: 1, Encryption: ipsec.EncryptionAES256GCM}
	if !our.Encryption.IsAEAD() {
		t.Fatal("the fixture cipher is not AEAD, so this test is about the wrong class")
	}
	enc := lookupEncryption(our.Encryption)
	if enc.ID == 0 {
		t.Fatal("the AEAD cipher resolves to no algorithm")
	}
	return uint16(enc.ID), enc.KeyLength
}

// VALIDATES: a peer proposal that puts an AEAD cipher and a real integrity transform in
// ONE proposal is refused, and the same AEAD cipher offered on its own is accepted.
// PREVENTS: keying an ESP SA from a proposal that mixes the two classes. An AEAD cipher
// already authenticates, so a separate integrity transform beside it describes a suite
// neither end can agree on, and accepting it would key the SA from a combination the
// negotiation never really settled.
// RFC requirement: RFC7296-3.3-1 negative -- RFC 7296 Section 3.3: AEAD and non-AEAD
// ciphers cannot share one proposal, and each class goes in a proposal of its own. This is
// the RECEIVE side of that rule, which is the side a peer controls: ze refuses the mixed
// proposal rather than selecting from it.
//
// The row carried {single-polarity: positive} because ze's own config type cannot express
// the mix, so nothing on the SEND side could be rejected. That reading was right about the
// sender and silent about the receiver. The annotation is removed rather than reclassified:
// the obligation gains proof, it does not lose scope.
func TestAeadMixInOneProposalIsRefused(t *testing.T) {
	encID, keyLen := aeadOurs(t)
	integ := uint16(lookupIntegrity(ipsec.HashSHA256).ID)
	if integ == 0 {
		t.Fatal("the non-AEAD integrity algorithm resolves to zero, so the mix is not expressible")
	}

	mixed := wire.Proposal{
		ProtocolID: wire.ProtocolESP,
		Transforms: []wire.Transform{
			espEncTransform(encID, keyLen),
			{Type: wire.TransformTypeINTG, ID: integ},
		},
	}
	if espProposalMatches(mixed, encID, keyLen, 0, true) {
		t.Error("a proposal mixing an AEAD cipher with a real integrity transform was accepted")
	}
}

// RFC requirement: RFC7296-3.3-1 positive -- the same AEAD cipher in a proposal of its own
// IS accepted, whether the integrity transform is absent altogether or present as the
// explicit NONE that says the same thing. Without this half the refusal above would also
// hold against a comparison that rejected every AEAD proposal, and the rule would read as
// "ze does not do AEAD" rather than "each class goes in its own proposal".
func TestAeadAloneInItsOwnProposalIsAccepted(t *testing.T) {
	encID, keyLen := aeadOurs(t)

	absent := wire.Proposal{
		ProtocolID: wire.ProtocolESP,
		Transforms: []wire.Transform{espEncTransform(encID, keyLen)},
	}
	if !espProposalMatches(absent, encID, keyLen, 0, true) {
		t.Error("an AEAD proposal carrying no integrity transform was refused")
	}

	explicitNone := wire.Proposal{
		ProtocolID: wire.ProtocolESP,
		Transforms: []wire.Transform{
			espEncTransform(encID, keyLen),
			{Type: wire.TransformTypeINTG, ID: 0},
		},
	}
	if !espProposalMatches(explicitNone, encID, keyLen, 0, true) {
		t.Error("an AEAD proposal carrying INTEG NONE was refused")
	}
}
