package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// espEncTransform builds one ENCR transform, with a Key Length attribute when keyLen is
// non-zero. RFC 7296 Section 3.3.5 encodes two key lengths as two separate ENCR
// transforms, so this is the unit an offer is assembled from.
func espEncTransform(id, keyLen uint16) wire.Transform {
	t := wire.Transform{Type: wire.TransformTypeENCR, ID: id}
	if keyLen != 0 {
		t.Attrs = []wire.TransformAttr{{Type: wire.AttrTypeKeyLength, Value: keyLen}}
	}
	return t
}

// espOurs returns the configured proposal the tests below match against, plus the three
// values espProposalMatches compares.
func espOurs(t *testing.T) (encID, keyLen, integID uint16) {
	t.Helper()
	our := ipsec.ESPProposal{Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256}
	enc := lookupEncryption(our.Encryption)
	integ := lookupIntegrity(our.Hash)
	if enc.ID == 0 || integ.ID == 0 {
		t.Fatal("the configured proposal resolves to no algorithm, so no comparison below discriminates")
	}
	return uint16(enc.ID), enc.KeyLength, uint16(integ.ID)
}

// VALIDATES: a key length offered on ONE encryption transform is never paired with the
// algorithm id of ANOTHER. The two travel together, per transform.
// PREVENTS: the defect where gotEnc and gotKeyLen were separate variables that each
// survived into the next transform, so an offer of [AES-128, AES-256] was read as the
// second id beside whichever length was written last. Ze could match, and then key, a
// suite the peer never offered.
func TestEspAltKeyLengthDoesNotCrossTransforms(t *testing.T) {
	encID, keyLen, integID := espOurs(t)

	// The peer offers a DIFFERENT cipher carrying the length ze wants, then ze's cipher
	// carrying NO length at all. Neither transform is ze's suite.
	//
	// This is the exact shape the old reader got wrong. It kept one variable for the id
	// and another for the key length, and neither was reset per transform, so it ended
	// holding the id of the second transform beside the length of the first and declared
	// a match. Ze would then have keyed a combination the peer never offered.
	otherEnc := encID + 1
	if keyLen == 0 {
		t.Fatal("the configured suite carries no key length, so the crossing cannot be observed")
	}
	offer := wire.Proposal{
		ProtocolID: wire.ProtocolESP,
		Transforms: []wire.Transform{
			espEncTransform(otherEnc, keyLen),
			espEncTransform(encID, 0),
			{Type: wire.TransformTypeINTG, ID: integID},
		},
	}
	if espProposalMatches(offer, encID, keyLen, integID, false, espDHMatch{}) {
		t.Error("a key length from one ENCR transform was paired with the id of another")
	}
}

// VALIDATES: every transform of a type is read as an alternative, so a proposal whose
// FIRST encryption transform is the one ze wants still matches when others follow.
// PREVENTS: refusing a conformant peer. RFC 7296 Section 3.3 lets one proposal carry
// several transforms of a type, and reading only the last refused offers that were on
// the table. strongSwan encodes alternatives this way.
func TestEspAltFirstTransformStillMatches(t *testing.T) {
	encID, keyLen, integID := espOurs(t)

	offer := wire.Proposal{
		ProtocolID: wire.ProtocolESP,
		Transforms: []wire.Transform{
			espEncTransform(encID, keyLen),   // ze's suite, offered first
			espEncTransform(encID+1, keyLen), // an alternative ze does not want
			{Type: wire.TransformTypeINTG, ID: integID},
			{Type: wire.TransformTypeINTG, ID: integID + 1},
		},
	}
	if !espProposalMatches(offer, encID, keyLen, integID, false, espDHMatch{}) {
		t.Error("a proposal offering ze's suite as its first alternative was refused")
	}
}

// VALIDATES: an integrity alternative is read the same way as an encryption one, and the
// comparison itself is NOT relaxed: a proposal offering neither ze's cipher nor ze's
// integrity algorithm is still refused.
// PREVENTS: an expansion that turns "read every alternative" into "accept anything".
func TestEspAltComparisonIsNotRelaxed(t *testing.T) {
	encID, keyLen, integID := espOurs(t)

	// Ze's integrity algorithm is present only as the second alternative.
	matching := wire.Proposal{
		ProtocolID: wire.ProtocolESP,
		Transforms: []wire.Transform{
			espEncTransform(encID, keyLen),
			{Type: wire.TransformTypeINTG, ID: integID + 1},
			{Type: wire.TransformTypeINTG, ID: integID},
		},
	}
	if !espProposalMatches(matching, encID, keyLen, integID, false, espDHMatch{}) {
		t.Error("ze's integrity algorithm offered as a later alternative was not seen")
	}

	// Nothing ze wants is on offer.
	foreign := wire.Proposal{
		ProtocolID: wire.ProtocolESP,
		Transforms: []wire.Transform{
			espEncTransform(encID+1, keyLen),
			espEncTransform(encID+2, keyLen),
			{Type: wire.TransformTypeINTG, ID: integID + 1},
		},
	}
	if espProposalMatches(foreign, encID, keyLen, integID, false, espDHMatch{}) {
		t.Error("a proposal offering none of ze's algorithms was accepted")
	}

	// The right cipher at the wrong length is still refused: the deliberate deviation
	// documented on espProposalMatches requires an exact length.
	wrongLen := wire.Proposal{
		ProtocolID: wire.ProtocolESP,
		Transforms: []wire.Transform{
			espEncTransform(encID, keyLen/2),
			{Type: wire.TransformTypeINTG, ID: integID},
		},
	}
	if espProposalMatches(wrongLen, encID, keyLen, integID, false, espDHMatch{}) {
		t.Error("the configured cipher at a different key length was accepted")
	}
}

// VALIDATES: the alternatives reading reaches the selection entry point, not only the
// predicate, and the peer's proposal number is what the response echoes.
// PREVENTS: a fix that lives in the comparison but never runs, because the caller reads
// the offer some other way.
func TestEspAltSelectionSeesAlternatives(t *testing.T) {
	encID, keyLen, integID := espOurs(t)
	our := ipsec.ESPProposal{Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256}

	offer := &wire.PayloadSA{Proposals: []wire.Proposal{{
		Number:     7,
		ProtocolID: wire.ProtocolESP,
		Transforms: []wire.Transform{
			espEncTransform(encID, keyLen),
			espEncTransform(encID+1, keyLen),
			{Type: wire.TransformTypeINTG, ID: integID},
		},
	}}}

	rp, ok := matchOfferedESPProposal(offer, our, espDHMatch{})
	if !ok {
		t.Fatal("the selection entry point refused an offer whose first alternative is ze's suite")
	}
	if rp.Number != 7 {
		t.Errorf("the matched proposal carries number %d, want the peer's 7", rp.Number)
	}
}
