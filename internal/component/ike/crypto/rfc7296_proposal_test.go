// VALIDATES: the RFC 7296 obligations IKEv2 proposal negotiation discharges. The list
// covers the proposal number returned to the peer (§3.3.1) and the transform types each
// protocol uses (§3.3.3). It covers the comparison of offered transform IDs against
// local policy (§3.3.4). It covers the Key Length attribute a transform requires
// (§3.3.5), and the attributes returned unmodified (§3.3.6). It covers the refusal of
// an incomplete or unknown proposal (§3.3.6), and of a Diffie-Hellman group of NONE
// (§2.18). Last, it covers the refusal of a transform with no specification (§3.14),
// and the PRF output floor (§5). Each test carries an `RFC requirement:` tag with its
// checklist id.
//
// PREVENTS: a return to the exact-tuple matcher. That matcher accepted or refused a
// whole proposal, and it decided no transform on its own terms. It had no way to
// refuse one transform for its own reason. It had no way to report why a proposal
// was unacceptable.
package crypto

import (
	"errors"
	"testing"
)

// ikeOffer builds one remote IKE proposal with every mandatory transform present.
func ikeOffer(num uint16) IKEProposal {
	return IKEProposal{
		Number:     num,
		Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128},
		PRF:        PRFTransform{ID: PRF_HMAC_SHA2_256},
		Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128},
		DHGroup:    DHGroupTransform{ID: DH_MODP_2048},
	}
}

// ikePolicy builds the matching local policy for ikeOffer.
func ikePolicy() []IKEProposal {
	return []IKEProposal{{
		Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128},
		PRF:        PRFTransform{ID: PRF_HMAC_SHA2_256, KeyLength: 32, OutputLength: 32},
		Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128, KeyLength: 32, TruncatedLength: 16},
		DHGroup:    DHGroupTransform{ID: DH_MODP_2048},
	}}
}

// RFC requirement: RFC7296-3.3.1-1 positive -- "When a proposal is accepted, the proposal number
// in the SA payload MUST match the number on the proposal sent that was accepted" (RFC
// 7296 Section 3.3.1). The second offer is the one local policy accepts, and the result
// carries its number rather than the position or the local policy's own number.
// RFC requirement: RFC7296-3.3.1-1 negative -- the number is copied from the accepted offer,
// never invented. An accepted offer numbered 7 returns 7. A responder cannot answer
// with a number the initiator never sent.
func TestPropAcceptedProposalNumberMatchesOffer(t *testing.T) {
	unmatched := ikeOffer(1)
	unmatched.DHGroup = DHGroupTransform{ID: DH_ECP_384}
	chosen, err := NegotiateIKE([]IKEProposal{unmatched, ikeOffer(2)}, ikePolicy())
	if err != nil {
		t.Fatalf("NegotiateIKE: %v", err)
	}
	if chosen.Number != 2 {
		t.Errorf("accepted proposal number = %d, want 2", chosen.Number)
	}

	odd, err := NegotiateIKE([]IKEProposal{ikeOffer(7)}, ikePolicy())
	if err != nil {
		t.Fatalf("NegotiateIKE(offer numbered 7): %v", err)
	}
	if odd.Number != 7 {
		t.Errorf("accepted proposal number = %d, want 7", odd.Number)
	}
}

// RFC requirement: RFC7296-3.3.3-1 positive -- RFC 7296 Section 3.3.3 states that a compliant
// implementation MUST understand all mandatory and optional types for each protocol it
// supports. The table names IKE as ENCR, PRF, INTEG and D-H. It names ESP as ENCR and
// ESN, with INTEG and D-H optional. It names AH as INTEG and ESN, with D-H optional.
// Every one of those types is understood for its protocol.
//
// RFC requirement: RFC7296-3.3.3-1 negative -- that table bounds what is understood. A transform
// type outside a protocol's mandatory and optional set is not understood for it. A PRF
// transform inside an ESP proposal therefore does not read as known.
func TestPropTransformTypesUnderstoodPerProtocol(t *testing.T) {
	cases := []struct {
		proto     protocolID
		mandatory []TransformType
		optional  []TransformType
		foreign   []TransformType
	}{
		{protoIKE, []TransformType{TransformTypeENCR, TransformTypePRF, TransformTypeINTEG, TransformTypeDH},
			nil, []TransformType{TransformTypeESN}},
		{protoESP, []TransformType{TransformTypeENCR, TransformTypeESN},
			[]TransformType{TransformTypeINTEG, TransformTypeDH}, []TransformType{TransformTypePRF}},
		{protoAH, []TransformType{TransformTypeINTEG, TransformTypeESN},
			[]TransformType{TransformTypeDH}, []TransformType{TransformTypePRF, TransformTypeENCR}},
	}
	for _, c := range cases {
		for _, tt := range c.mandatory {
			if !transformTypeUnderstood(c.proto, tt) {
				t.Errorf("protocol %d: mandatory transform type %d is not understood", c.proto, tt)
			}
			if !transformTypeMandatory(c.proto, tt) {
				t.Errorf("protocol %d: transform type %d is not recorded as mandatory", c.proto, tt)
			}
		}
		for _, tt := range c.optional {
			if !transformTypeUnderstood(c.proto, tt) {
				t.Errorf("protocol %d: optional transform type %d is not understood", c.proto, tt)
			}
			if transformTypeMandatory(c.proto, tt) {
				t.Errorf("protocol %d: optional transform type %d is recorded as mandatory", c.proto, tt)
			}
		}
		for _, tt := range c.foreign {
			if transformTypeUnderstood(c.proto, tt) {
				t.Errorf("protocol %d: transform type %d is outside the table but reads as understood",
					c.proto, tt)
			}
		}
	}
}

// RFC requirement: RFC7296-3.3.4-2 positive -- RFC 7296 Section 3.3.4 requires the transmitted
// Transform IDs to be compared against the locally configured ones. That comparison
// decides whether local policy permits the offered suite. Each transform type is
// compared on its own. An offer that differs only in the Diffie-Hellman group is
// refused. The same offer is accepted once local policy holds that group.
//
// RFC requirement: RFC7296-3.3.4-2 negative -- the comparison covers every transform type rather
// than the encryption alone. An offer differing only in the PRF, only in the integrity
// algorithm, or only in the encryption algorithm is refused in each case.
func TestPropTransformIDsComparedAgainstLocalPolicy(t *testing.T) {
	differsBy := map[string]func(p *IKEProposal){
		"encryption": func(p *IKEProposal) { p.Encryption.ID = ENCR_AES_GCM_16 },
		"PRF":        func(p *IKEProposal) { p.PRF.ID = PRF_HMAC_SHA2_512 },
		"integrity":  func(p *IKEProposal) { p.Integrity.ID = AUTH_HMAC_SHA2_512_256 },
		"DH group":   func(p *IKEProposal) { p.DHGroup.ID = DH_ECP_256 },
	}
	for name, mutate := range differsBy {
		offer := ikeOffer(1)
		mutate(&offer)
		if _, err := NegotiateIKE([]IKEProposal{offer}, ikePolicy()); !errors.Is(err, ErrNoProposalChosen) {
			t.Errorf("NegotiateIKE(offer differing in %s) = %v, want ErrNoProposalChosen", name, err)
		}
	}

	widened := ikePolicy()
	widened = append(widened, IKEProposal{
		Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128},
		PRF:        PRFTransform{ID: PRF_HMAC_SHA2_256},
		Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128},
		DHGroup:    DHGroupTransform{ID: DH_ECP_256},
	})
	offer := ikeOffer(1)
	offer.DHGroup = DHGroupTransform{ID: DH_ECP_256}
	if _, err := NegotiateIKE([]IKEProposal{offer}, widened); err != nil {
		t.Errorf("NegotiateIKE(offer allowed by widened policy) = %v, want acceptance", err)
	}
}

// RFC requirement: RFC7296-3.3.4-3 positive -- "The implementation MUST reject SA proposals that
// are not authorized by these IKE suite controls" (RFC 7296 Section 3.3.4). An offer no
// local proposal authorizes is rejected rather than partially accepted, for the IKE SA
// and for the Child SA alike.
// RFC requirement: RFC7296-3.3.4-3 negative -- rejection is confined to the unauthorized offer.
// An offer that local policy authorizes is accepted. The control is a filter, not a
// refusal of every proposal.
func TestPropUnauthorizedProposalRejected(t *testing.T) {
	unauthorized := ikeOffer(1)
	unauthorized.Encryption = EncryptionTransform{ID: ENCR_AES_GCM_16, KeyLength: 256, IsAEAD: true}
	if _, err := NegotiateIKE([]IKEProposal{unauthorized}, ikePolicy()); !errors.Is(err, ErrNoProposalChosen) {
		t.Errorf("NegotiateIKE(unauthorized offer) = %v, want ErrNoProposalChosen", err)
	}
	if _, err := NegotiateIKE([]IKEProposal{ikeOffer(1)}, ikePolicy()); err != nil {
		t.Errorf("NegotiateIKE(authorized offer) = %v, want acceptance", err)
	}

	espLocal := []ESPProposal{{
		Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 256},
		Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128},
	}}
	espBad := []ESPProposal{{
		Number:     1,
		Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 256},
		Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_512_256},
	}}
	if _, err := NegotiateESP(espBad, espLocal); !errors.Is(err, ErrNoProposalChosen) {
		t.Errorf("NegotiateESP(unauthorized offer) = %v, want ErrNoProposalChosen", err)
	}
}

// RFC requirement: RFC7296-3.3.5-5 negative -- "Some transforms specify that the Key Length
// attribute MUST be always included (omitting the attribute is not allowed, and proposals
// not containing it MUST be rejected). For example, this includes ENCR_AES_CBC and
// ENCR_AES_CTR" (RFC 7296 Section 3.3.5). An offer of either without a Key Length
// attribute is rejected, and the reason names the missing attribute.
// RFC requirement: RFC7296-3.3.5-5 positive -- the same offer carrying the attribute is accepted,
// so the rejection is caused by the omission alone.
func TestPropKeyLengthRequiredTransformRejectedWithoutIt(t *testing.T) {
	for _, id := range []EncryptionID{ENCR_AES_CBC, encrAESCTR} {
		offer := ikeOffer(1)
		offer.Encryption = EncryptionTransform{ID: id, KeyLength: 0}
		local := ikePolicy()
		local[0].Encryption = EncryptionTransform{ID: id, KeyLength: 128}
		if _, err := NegotiateIKE([]IKEProposal{offer}, local); !errors.Is(err, ErrKeyLengthMissing) {
			t.Errorf("NegotiateIKE(%d with no key length) = %v, want ErrKeyLengthMissing", id, err)
		}
		if !errors.Is(ErrKeyLengthMissing, ErrNoProposalChosen) {
			t.Error("ErrKeyLengthMissing must report as ErrNoProposalChosen to the responder")
		}
	}

	if _, err := NegotiateIKE([]IKEProposal{ikeOffer(1)}, ikePolicy()); err != nil {
		t.Errorf("NegotiateIKE(offer carrying a key length) = %v, want acceptance", err)
	}
}

// RFC requirement: RFC7296-3.3.6-7 positive -- RFC 7296 Section 3.3.6 states that any attributes
// of a selected transform MUST be returned unmodified. Local policy asks for a 128-bit
// key or better. The peer offers the same cipher with a 256-bit key. The accepted
// transform carries the peer's 256, not the locally configured 128.
// RFC requirement: RFC7296-3.3.6-7 negative -- an unmodified return is not an unchecked one. Two
// key lengths are refused rather than returned: one below the configured minimum, and
// one this implementation cannot key.
func TestPropSelectedTransformAttributesUnmodified(t *testing.T) {
	offer := ikeOffer(1)
	offer.Encryption = EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 256}
	chosen, err := NegotiateIKE([]IKEProposal{offer}, ikePolicy())
	if err != nil {
		t.Fatalf("NegotiateIKE(256-bit offer against a 128-bit policy): %v", err)
	}
	if chosen.Encryption.KeyLength != 256 {
		t.Errorf("returned key length = %d, want 256 unmodified from the offer", chosen.Encryption.KeyLength)
	}

	local := ikePolicy()
	local[0].Encryption.KeyLength = 256
	weaker := ikeOffer(1)
	weaker.Encryption = EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128}
	if _, err := NegotiateIKE([]IKEProposal{weaker}, local); !errors.Is(err, ErrNoProposalChosen) {
		t.Errorf("NegotiateIKE(128-bit offer against a 256-bit policy) = %v, want ErrNoProposalChosen", err)
	}

	unkeyable := ikeOffer(1)
	unkeyable.Encryption = EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 512}
	if _, err := NegotiateIKE([]IKEProposal{unkeyable}, ikePolicy()); !errors.Is(err, ErrNoProposalChosen) {
		t.Errorf("NegotiateIKE(512-bit key offer) = %v, want ErrNoProposalChosen", err)
	}
}

// RFC requirement: RFC7296-3.3.6-4 negative -- RFC 7296 Section 3.3.6 makes a proposal
// unacceptable in two cases. The first is a Transform Type the responder does not
// understand. The second is a mandatory Transform Type the proposal omits. An IKE
// offer with no encryption, no PRF, or no Diffie-Hellman group is refused. The reason
// names the incomplete proposal.
//
// RFC requirement: RFC7296-3.3.6-4 positive -- "however, other proposals in the same SA payload
// are processed as usual". An incomplete first proposal does not stop the second, which
// is complete, from being accepted.
func TestPropProposalMissingMandatoryTransformTypeUnacceptable(t *testing.T) {
	missing := map[string]func(p *IKEProposal){
		"encryption": func(p *IKEProposal) { p.Encryption = EncryptionTransform{} },
		"PRF":        func(p *IKEProposal) { p.PRF = PRFTransform{} },
		"DH group":   func(p *IKEProposal) { p.DHGroup = DHGroupTransform{} },
		"integrity":  func(p *IKEProposal) { p.Integrity = IntegrityTransform{} },
	}
	for name, drop := range missing {
		offer := ikeOffer(1)
		drop(&offer)
		_, err := NegotiateIKE([]IKEProposal{offer}, ikePolicy())
		if !errors.Is(err, ErrProposalIncomplete) && !errors.Is(err, ErrDHGroupNone) {
			t.Errorf("NegotiateIKE(offer with no %s) = %v, want an incomplete-proposal refusal", name, err)
		}
	}

	broken := ikeOffer(1)
	broken.PRF = PRFTransform{}
	chosen, err := NegotiateIKE([]IKEProposal{broken, ikeOffer(2)}, ikePolicy())
	if err != nil {
		t.Fatalf("NegotiateIKE(incomplete first proposal, complete second): %v", err)
	}
	if chosen.Number != 2 {
		t.Errorf("accepted proposal %d, want 2; other proposals must be processed as usual", chosen.Number)
	}
}

// RFC requirement: RFC7296-3.3.6-5 negative -- RFC 7296 Section 3.3.6 makes a transform the
// responder does not understand unacceptable. A transform identifier this
// implementation has no specification for is refused, even when both sides name the
// same unknown value. Agreement on an unknown identifier is not agreement.
// RFC requirement: RFC7296-3.3.6-5 positive -- "other transforms with the same Transform Type are
// processed as usual". The proposal carrying the unknown transform is the only one
// refused, and a later proposal built from understood transforms is accepted.
func TestPropUnknownTransformIDMakesTransformUnacceptable(t *testing.T) {
	const unknown EncryptionID = 999
	offer := ikeOffer(1)
	offer.Encryption = EncryptionTransform{ID: unknown, KeyLength: 128}
	local := ikePolicy()
	local[0].Encryption = EncryptionTransform{ID: unknown, KeyLength: 128}
	if _, err := NegotiateIKE([]IKEProposal{offer}, local); !errors.Is(err, ErrTransformUnspecified) {
		t.Errorf("NegotiateIKE(unknown encryption on both sides) = %v, want ErrTransformUnspecified", err)
	}

	chosen, err := NegotiateIKE([]IKEProposal{offer, ikeOffer(2)}, ikePolicy())
	if err != nil {
		t.Fatalf("NegotiateIKE(unknown first proposal, understood second): %v", err)
	}
	if chosen.Number != 2 {
		t.Errorf("accepted proposal %d, want 2", chosen.Number)
	}
}

// RFC requirement: RFC7296-2.18-3 negative -- RFC 7296 Section 2.18 states that an initiator MUST
// NOT propose the value NONE for the Diffie-Hellman transform. A responder MUST NOT
// accept such a proposal. An IKE offer with a Diffie-Hellman group of NONE is therefore
// refused, even when every other transform matches local policy.
// RFC requirement: RFC7296-2.18-3 positive -- a real group is accepted, so the refusal is caused
// by NONE rather than by the Diffie-Hellman comparison as such.
func TestPropDHNoneRefusedForIKESA(t *testing.T) {
	offer := ikeOffer(1)
	offer.DHGroup = DHGroupTransform{ID: dhGroupNone}
	local := ikePolicy()
	local[0].DHGroup = DHGroupTransform{ID: dhGroupNone}
	if _, err := NegotiateIKE([]IKEProposal{offer}, local); !errors.Is(err, ErrDHGroupNone) {
		t.Errorf("NegotiateIKE(DH group NONE on both sides) = %v, want ErrDHGroupNone", err)
	}
	if !errors.Is(ErrDHGroupNone, ErrNoProposalChosen) {
		t.Error("ErrDHGroupNone must report as ErrNoProposalChosen to the responder")
	}

	if _, err := NegotiateIKE([]IKEProposal{ikeOffer(1)}, ikePolicy()); err != nil {
		t.Errorf("NegotiateIKE(offer with a real DH group) = %v, want acceptance", err)
	}
}

// RFC requirement: RFC7296-3.14-7 negative -- RFC 7296 Section 3.14 states that peers MUST NOT
// negotiate transforms for which no specification exists. A PRF, an integrity algorithm
// and a Diffie-Hellman group with no specification here are each refused. A matching
// pair of unknown identifiers never becomes a negotiated transform.
// RFC requirement: RFC7296-3.14-7 positive -- every transform this implementation does specify
// negotiates, so the check names an absent specification rather than blocking the
// specified set.
func TestPropUnspecifiedTransformRefused(t *testing.T) {
	unspecified := map[string]func(p *IKEProposal){
		"PRF":        func(p *IKEProposal) { p.PRF = PRFTransform{ID: PRFID(998)} },
		"integrity":  func(p *IKEProposal) { p.Integrity = IntegrityTransform{ID: IntegrityID(997)} },
		"DH group":   func(p *IKEProposal) { p.DHGroup = DHGroupTransform{ID: DHGroupID(996)} },
		"encryption": func(p *IKEProposal) { p.Encryption = EncryptionTransform{ID: EncryptionID(995), KeyLength: 128} },
	}
	for name, mutate := range unspecified {
		offer := ikeOffer(1)
		mutate(&offer)
		local := ikePolicy()
		mutate(&local[0])
		if _, err := NegotiateIKE([]IKEProposal{offer}, local); !errors.Is(err, ErrTransformUnspecified) {
			t.Errorf("NegotiateIKE(unspecified %s) = %v, want ErrTransformUnspecified", name, err)
		}
	}

	if _, err := NegotiateIKE([]IKEProposal{ikeOffer(1)}, ikePolicy()); err != nil {
		t.Errorf("NegotiateIKE(specified transforms) = %v, want acceptance", err)
	}
}

// RFC requirement: RFC7296-5-3 negative -- RFC 7296 Section 5 states that a PRF with an output of
// less than 128 bits MUST NOT be used with this protocol. Negotiation reads one table of
// PRF output sizes, and it refuses any entry below the floor. RFC 7296 lists no PRF with
// an output under 128 bits. The test therefore registers one for its own length, to
// reach the branch.
// RFC requirement: RFC7296-5-3 positive -- every PRF this implementation offers is at or above
// the floor, and each of them negotiates.
func TestPropPRFOutputBelow128BitsRefused(t *testing.T) {
	const weak PRFID = 900
	prfOutputBits[weak] = 64
	t.Cleanup(func() { delete(prfOutputBits, weak) })

	offer := ikeOffer(1)
	offer.PRF = PRFTransform{ID: weak}
	local := ikePolicy()
	local[0].PRF = PRFTransform{ID: weak}
	if _, err := NegotiateIKE([]IKEProposal{offer}, local); !errors.Is(err, ErrPRFTooWeak) {
		t.Errorf("NegotiateIKE(64-bit PRF) = %v, want ErrPRFTooWeak", err)
	}

	for id, bits := range prfOutputBits {
		if id == weak {
			continue
		}
		if bits < minPRFOutputBits {
			t.Errorf("PRF %d has a %d-bit output, below the %d-bit floor", id, bits, minPRFOutputBits)
		}
		offered := ikeOffer(1)
		offered.PRF = PRFTransform{ID: id}
		policy := ikePolicy()
		policy[0].PRF = PRFTransform{ID: id}
		if _, err := NegotiateIKE([]IKEProposal{offered}, policy); err != nil {
			t.Errorf("NegotiateIKE(PRF %d) = %v, want acceptance", id, err)
		}
	}
}
