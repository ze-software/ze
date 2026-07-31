// Design: plan/learned/739-ipsec-6-ikev2-crypto.md -- IKE/ESP proposal negotiation
// RFC: rfc/short/rfc7296.md -- proposal and transform negotiation (Sections 3.3, 3.14, 5)

package crypto

import "fmt"

// ErrNoProposalChosen is the refusal the responder reports to the peer. It becomes
// the NO_PROPOSAL_CHOSEN notification. Every reason below wraps it. A caller that
// asks only whether negotiation failed keeps working. A caller that wants the
// reason reads it with errors.Is.
var ErrNoProposalChosen = errNoProposalChosen{}

type errNoProposalChosen struct{}

func (errNoProposalChosen) Error() string { return "NO_PROPOSAL_CHOSEN" }

var (
	// ErrProposalIncomplete reports a proposal that omits a mandatory transform
	// type (RFC 7296 Section 3.3.6).
	ErrProposalIncomplete = fmt.Errorf("%w: proposal omits a mandatory transform type", ErrNoProposalChosen)
	// ErrTransformUnspecified reports a transform identifier this implementation
	// has no specification for (RFC 7296 Sections 3.3.6 and 3.14).
	ErrTransformUnspecified = fmt.Errorf("%w: transform has no specification", ErrNoProposalChosen)
	// ErrTransformTypeNotUnderstood reports a proposal that carries a Transform Type the
	// protocol does not use (RFC 7296 Sections 3.3.3 and 3.3.6).
	ErrTransformTypeNotUnderstood = fmt.Errorf("%w: proposal carries a transform type this protocol does not use", ErrNoProposalChosen)
	// ErrKeyLengthMissing reports a transform that requires a Key Length attribute
	// and carries none (RFC 7296 Section 3.3.5).
	ErrKeyLengthMissing = fmt.Errorf("%w: transform requires a key length attribute", ErrNoProposalChosen)
	// ErrDHGroupNone reports a Diffie-Hellman group of NONE offered for an IKE SA
	// (RFC 7296 Section 2.18).
	ErrDHGroupNone = fmt.Errorf("%w: Diffie-Hellman group NONE is not allowed for an IKE SA", ErrNoProposalChosen)
	// ErrPRFTooWeak reports a PRF whose output is below the floor RFC 7296
	// Section 5 sets.
	ErrPRFTooWeak = fmt.Errorf("%w: PRF output is below 128 bits", ErrNoProposalChosen)
)

// minPRFOutputBits is the PRF output floor. RFC 7296 Section 5 states that a PRF
// with an output of less than 128 bits MUST NOT be used with this protocol.
const minPRFOutputBits = 128

// dhGroupNone is the Diffie-Hellman Transform ID that names no group (RFC 7296
// Section 3.3.2).
const dhGroupNone DHGroupID = 0

// encrAESCTR is one of the two encryption transforms RFC 7296 Section 3.3.5 names
// as always requiring a Key Length attribute.
const encrAESCTR EncryptionID = 13

// protocolID names an IPsec protocol inside a proposal (RFC 7296 Section 3.3.1).
type protocolID uint8

const (
	protoIKE protocolID = 1
	protoAH  protocolID = 2
	protoESP protocolID = 3
)

// transformTypeUse records how one protocol uses one transform type.
type transformTypeUse uint8

const (
	transformTypeAbsent transformTypeUse = iota
	transformTypeOptional
	transformTypeRequired
)

// protocolTransformTypes is the table of RFC 7296 Section 3.3.3, "Valid Transform
// Types by Protocol". IKE takes ENCR, PRF, INTEG and D-H. ESP takes ENCR and ESN,
// with INTEG and D-H optional. AH takes INTEG and ESN, with D-H optional.
var protocolTransformTypes = map[protocolID]map[TransformType]transformTypeUse{
	protoIKE: {
		TransformTypeENCR:  transformTypeRequired,
		TransformTypePRF:   transformTypeRequired,
		TransformTypeINTEG: transformTypeRequired,
		TransformTypeDH:    transformTypeRequired,
	},
	protoESP: {
		TransformTypeENCR:  transformTypeRequired,
		TransformTypeESN:   transformTypeRequired,
		TransformTypeINTEG: transformTypeOptional,
		TransformTypeDH:    transformTypeOptional,
	},
	protoAH: {
		TransformTypeINTEG: transformTypeRequired,
		TransformTypeESN:   transformTypeRequired,
		TransformTypeDH:    transformTypeOptional,
	},
}

// transformTypeUnderstood reports whether this protocol uses this transform type.
// RFC 7296 Section 3.3.3: "A compliant implementation MUST understand all mandatory
// and optional types for each protocol it supports".
func transformTypeUnderstood(proto protocolID, tt TransformType) bool {
	return protocolTransformTypes[proto][tt] != transformTypeAbsent
}

// TransformTypeUnderstoodIKE reports whether an IKE SA proposal uses this transform type.
// The reader of a wire proposal calls it for every transform it meets. A type that reads
// false makes the proposal unacceptable under RFC 7296 Section 3.3.6, and the reader
// records it in UnknownTransformType.
func TransformTypeUnderstoodIKE(tt TransformType) bool {
	return transformTypeUnderstood(protoIKE, tt)
}

// TransformTypeUnderstoodESP reports whether an ESP proposal uses this transform type, on
// the same terms as TransformTypeUnderstoodIKE.
func TransformTypeUnderstoodESP(tt TransformType) bool {
	return transformTypeUnderstood(protoESP, tt)
}

// transformTypeMandatory reports whether this protocol requires this transform
// type in every proposal (RFC 7296 Section 3.3.3).
func transformTypeMandatory(proto protocolID, tt TransformType) bool {
	return protocolTransformTypes[proto][tt] == transformTypeRequired
}

// prfOutputBits is the output size of each PRF this implementation specifies. It
// is the single source both for the specification check of RFC 7296 Section 3.14
// and for the output floor of RFC 7296 Section 5.
var prfOutputBits = map[PRFID]int{
	PRF_HMAC_SHA2_256: 256,
	PRF_HMAC_SHA2_384: 384,
	PRF_HMAC_SHA2_512: 512,
}

// encryptionKeyBits is the set of encryption key lengths this implementation can
// key. AES takes a 128, 192 or 256 bit key.
var encryptionKeyBits = map[uint16]bool{128: true, 192: true, 256: true}

// specifiedEncryption lists the encryption transforms this implementation
// specifies (RFC 7296 Section 3.14).
var specifiedEncryption = map[EncryptionID]bool{
	ENCR_AES_CBC:    true,
	encrAESCTR:      true,
	ENCR_AES_GCM_16: true,
}

// specifiedIntegrity lists the integrity transforms this implementation specifies.
// AUTH_NONE is specified: it is the value an AEAD proposal carries.
var specifiedIntegrity = map[IntegrityID]bool{
	AUTH_NONE:              true,
	AUTH_HMAC_SHA2_256_128: true,
	AUTH_HMAC_SHA2_384_192: true,
	AUTH_HMAC_SHA2_512_256: true,
}

// specifiedDHGroup lists the Diffie-Hellman groups this implementation specifies.
var specifiedDHGroup = map[DHGroupID]bool{
	DH_MODP_2048: true,
	DH_ECP_256:   true,
	DH_ECP_384:   true,
}

// keyLengthRequired reports whether this encryption transform must carry a Key
// Length attribute. RFC 7296 Section 3.3.5: "Some transforms specify that the Key
// Length attribute MUST be always included ... For example, this includes
// ENCR_AES_CBC and ENCR_AES_CTR".
func keyLengthRequired(id EncryptionID) bool {
	return id == ENCR_AES_CBC || id == encrAESCTR
}

// keyLengthRule names how an offered Key Length attribute is compared with the key
// length local policy holds. RFC 7296 Section 3.3.6 gives the two sides of a negotiation
// different obligations, so the two sides read the attribute under different rules. The
// zero value is the strict rule, which makes the safe rule the default one.
type keyLengthRule uint8

const (
	// keyLengthExact requires the offered key length to equal a configured one. RFC 7296
	// Section 3.3.6 obliges the initiator to check that the accepted offer agrees with one
	// of its proposals. Section 3.3.5 states that the responder returns the attribute
	// unchanged, and that an initiator which accepts several key lengths sends one
	// transform for each. A length this side never sent therefore names an unsent suite.
	keyLengthExact keyLengthRule = iota
	// keyLengthAtLeast accepts a key longer than the configured one when this
	// implementation can key it. RFC 7296 Section 3.3.5 states that implementers SHOULD
	// accept values that they deem to supply greater security. It applies to the responder,
	// which selects one offer from the offers the peer sends.
	keyLengthAtLeast
)

// IKEProposal represents a single IKE SA crypto proposal.
type IKEProposal struct {
	Number     uint16
	Encryption EncryptionTransform
	PRF        PRFTransform
	Integrity  IntegrityTransform
	DHGroup    DHGroupTransform
	// PolicyKeyLength holds the encryption key length local policy configured, when the
	// responder accepted a longer key under RFC 7296 Section 3.3.5. It is zero when the
	// accepted key length is the configured one. The caller reports every non-zero value,
	// because the running key then differs from the configured key.
	PolicyKeyLength uint16
	// UnknownTransformType holds the first Transform Type the peer offered that IKE does
	// not use. The reader of a wire proposal sets it from TransformTypeUnderstoodIKE. RFC
	// 7296 Section 3.3.6 makes such a proposal unacceptable, and negotiation refuses it.
	// Zero means every type the peer offered is one IKE uses, because RFC 7296
	// Section 3.3.2 reserves the type zero.
	UnknownTransformType TransformType
}

// ESPProposal represents a single ESP/Child SA crypto proposal.
type ESPProposal struct {
	Number     uint16
	Encryption EncryptionTransform
	Integrity  IntegrityTransform
	// UnknownTransformType holds the first Transform Type the peer offered that ESP
	// does not use. The IKE field of the same name records it on the same terms.
	UnknownTransformType TransformType
}

// acceptEncryption decides one encryption transform against local policy. A selected
// transform returns its offered Key Length attribute unmodified (RFC 7296 Section 3.3.6).
// The rule decides what a key length other than the configured one means. Under
// keyLengthAtLeast a longer key is accepted when this implementation can key it (RFC 7296
// Section 3.3.5). Under keyLengthExact only the configured length is accepted.
func acceptEncryption(remote, local EncryptionTransform, rule keyLengthRule) (EncryptionTransform, error) {
	if !specifiedEncryption[remote.ID] {
		return EncryptionTransform{}, ErrTransformUnspecified
	}
	if remote.ID != local.ID {
		return EncryptionTransform{}, ErrNoProposalChosen
	}
	if keyLengthRequired(remote.ID) && remote.KeyLength == 0 {
		return EncryptionTransform{}, ErrKeyLengthMissing
	}
	if remote.KeyLength != local.KeyLength {
		if rule != keyLengthAtLeast {
			return EncryptionTransform{}, ErrNoProposalChosen
		}
		if remote.KeyLength < local.KeyLength || !encryptionKeyBits[remote.KeyLength] {
			return EncryptionTransform{}, ErrNoProposalChosen
		}
	}
	chosen := local
	chosen.KeyLength = remote.KeyLength
	return chosen, nil
}

// acceptPRF decides one PRF transform against local policy, and enforces the
// output floor of RFC 7296 Section 5.
func acceptPRF(remote, local PRFTransform) error {
	bits, specified := prfOutputBits[remote.ID]
	if err := prfOutputAcceptable(bits, specified); err != nil {
		return err
	}
	if remote.ID != local.ID {
		return ErrNoProposalChosen
	}
	return nil
}

// prfOutputAcceptable decides one PRF output size against the floor of RFC 7296 Section 5:
// "a PRF with an output of less than 128 bits MUST NOT be used with this protocol". It
// takes the size rather than reads prfOutputBits, so a test reaches the floor with a
// number instead of a write to a package-level map.
//
// Every PRF prfOutputBits holds today is at or above the floor, so the allowlist is what
// refuses a weak PRF in a running daemon. The floor is the guard on the allowlist itself.
// It fires the moment an entry below 128 bits is added, which keeps that addition from
// silently negotiating (ai/rules/fail-closed-guards.md).
func prfOutputAcceptable(bits int, specified bool) error {
	if !specified {
		return ErrTransformUnspecified
	}
	if bits < minPRFOutputBits {
		return ErrPRFTooWeak
	}
	return nil
}

// acceptIntegrity decides one integrity transform against local policy.
func acceptIntegrity(remote, local IntegrityTransform) error {
	if !specifiedIntegrity[remote.ID] {
		return ErrTransformUnspecified
	}
	if remote.ID != local.ID {
		return ErrNoProposalChosen
	}
	return nil
}

// acceptDHGroup decides one Diffie-Hellman transform against local policy. RFC
// 7296 Section 2.18 forbids the value NONE for an IKE SA, so an IKE negotiation
// refuses it before any comparison with local policy.
func acceptDHGroup(remote, local DHGroupTransform, forIKESA bool) error {
	if forIKESA && remote.ID == dhGroupNone {
		return ErrDHGroupNone
	}
	if !specifiedDHGroup[remote.ID] {
		return ErrTransformUnspecified
	}
	if remote.ID != local.ID {
		return ErrNoProposalChosen
	}
	return nil
}

// ikeProposalComplete checks that an offered IKE proposal carries every mandatory
// transform type. RFC 7296 Section 3.3.6 makes a proposal that is "missing a
// mandatory Transform Type" unacceptable. A transform type is present when it names
// a value. The Transform ID zero is reserved for ENCR and PRF, and it is NONE for
// INTEG and D-H.
//
// An AEAD encryption transform makes the integrity type NONE by design. The
// integrity check therefore applies to the non-AEAD case only.
func ikeProposalComplete(p *IKEProposal) error {
	if p.UnknownTransformType != 0 {
		return ErrTransformTypeNotUnderstood
	}
	if transformTypeMandatory(protoIKE, TransformTypeENCR) && p.Encryption.ID == 0 {
		return ErrProposalIncomplete
	}
	if transformTypeMandatory(protoIKE, TransformTypePRF) && p.PRF.ID == 0 {
		return ErrProposalIncomplete
	}
	if transformTypeMandatory(protoIKE, TransformTypeDH) && p.DHGroup.ID == dhGroupNone {
		return ErrDHGroupNone
	}
	if transformTypeMandatory(protoIKE, TransformTypeINTEG) &&
		p.Integrity.ID == AUTH_NONE && !p.Encryption.ID.IsAEAD() {
		return ErrProposalIncomplete
	}
	return nil
}

// matchIKE decides one offered IKE proposal against one local proposal, one
// transform type at a time. RFC 7296 Section 3.3.4 requires the offered Transform
// IDs to be compared against local configuration, and Section 3.3.6 requires a
// single complete set to be selected.
func matchIKE(remote, local *IKEProposal, rule keyLengthRule) (IKEProposal, error) {
	enc, err := acceptEncryption(remote.Encryption, local.Encryption, rule)
	if err != nil {
		return IKEProposal{}, err
	}
	if err := acceptPRF(remote.PRF, local.PRF); err != nil {
		return IKEProposal{}, err
	}
	if err := acceptIntegrity(remote.Integrity, local.Integrity); err != nil {
		return IKEProposal{}, err
	}
	if err := acceptDHGroup(remote.DHGroup, local.DHGroup, true); err != nil {
		return IKEProposal{}, err
	}
	chosen := *local
	chosen.Encryption = enc
	chosen.Number = remote.Number
	// A key length other than the configured one reaches here only under
	// keyLengthAtLeast. The caller reports the difference, so the running key never
	// differs from the configured key without a record.
	chosen.PolicyKeyLength = 0
	if enc.KeyLength != local.Encryption.KeyLength {
		chosen.PolicyKeyLength = local.Encryption.KeyLength
	}
	return chosen, nil
}

// NegotiateIKE selects one complete set of IKE SA parameters from the offers a peer
// sends. It is the RESPONDER path. RFC 7296 Section 3.3.6 requires the responder to
// select a single complete set of parameters, or to reject every offer.
//
// The responder reads the Key Length attribute under keyLengthAtLeast, which RFC 7296
// Section 3.3.5 allows: implementers SHOULD accept values that they deem to supply
// greater security. An accepted key longer than the configured one sets PolicyKeyLength
// on the result, and the caller reports it.
//
// The initiator MUST NOT use this function. Its own obligation is stricter, and
// VerifyAcceptedIKE holds it.
func NegotiateIKE(remote, local []IKEProposal) (IKEProposal, error) {
	return negotiateIKE(remote, local, keyLengthAtLeast)
}

// VerifyAcceptedIKE checks the IKE offer a responder accepted against the proposals this
// side sent. It is the INITIATOR path. RFC 7296 Section 3.3.6: "The initiator of an
// exchange MUST check that the accepted offer is consistent with one of its proposals,
// and if not MUST terminate the exchange".
//
// The check reads the Key Length attribute under keyLengthExact. RFC 7296 Section 3.3.5
// states that the attribute returns unchanged, and that an initiator which accepts
// several key lengths sends one transform for each. A key length this side never sent is
// therefore an inconsistent answer, above the configured length as much as below it.
func VerifyAcceptedIKE(accepted, sent []IKEProposal) (IKEProposal, error) {
	return negotiateIKE(accepted, sent, keyLengthExact)
}

// negotiateIKE holds the selection loop both roles share. RFC 7296 Section 3.3.6
// requires one proposal to be chosen when the payload carries several. A proposal that
// is unacceptable does not stop the ones after it. The reason reported is the first
// proposal's reason, so a single-proposal offer reports why it was refused.
func negotiateIKE(remote, local []IKEProposal, rule keyLengthRule) (IKEProposal, error) {
	var reason error
	note := func(err error) {
		if reason == nil {
			reason = err
		}
	}
	for i := range remote {
		if err := ikeProposalComplete(&remote[i]); err != nil {
			note(err)
			continue
		}
		tried := false
		for j := range local {
			chosen, err := matchIKE(&remote[i], &local[j], rule)
			if err == nil {
				return chosen, nil
			}
			note(err)
			tried = true
		}
		if !tried {
			note(ErrNoProposalChosen)
		}
	}
	if reason == nil {
		reason = ErrNoProposalChosen
	}
	return IKEProposal{}, reason
}

// espProposalComplete checks that an offered ESP proposal carries the mandatory
// transform types this type can express. RFC 7296 Section 3.3.3 makes ENCR and ESN
// mandatory for ESP. The Extended Sequence Numbers transform is carried outside
// this type. Only the encryption transform is therefore checked here.
func espProposalComplete(p *ESPProposal) error {
	if p.UnknownTransformType != 0 {
		return ErrTransformTypeNotUnderstood
	}
	if transformTypeMandatory(protoESP, TransformTypeENCR) && p.Encryption.ID == 0 {
		return ErrProposalIncomplete
	}
	return nil
}

// matchESP decides one offered ESP proposal against one local proposal. The
// integrity transform is optional for ESP (RFC 7296 Section 3.3.3), and an AEAD
// cipher carries it as NONE.
func matchESP(remote, local *ESPProposal, rule keyLengthRule) (ESPProposal, error) {
	if _, err := acceptEncryption(remote.Encryption, local.Encryption, rule); err != nil {
		return ESPProposal{}, err
	}
	if err := acceptIntegrity(remote.Integrity, local.Integrity); err != nil {
		return ESPProposal{}, err
	}
	// The peer's own attributes go back to it unmodified (RFC 7296 Section 3.3.6),
	// so the accepted proposal is the offer rather than a rebuild of it.
	return *remote, nil
}

// NegotiateESP checks the Child SA offer a responder accepted against the proposals this
// side sent. It reads the same per-transform terms as VerifyAcceptedIKE. It is the
// INITIATOR path, and the initiator check is its only caller. RFC 7296 Section 3.3.6
// obliges the initiator to check the accepted offer against its own proposals. The Key
// Length attribute is therefore read under keyLengthExact.
//
// The responder's own Child SA selection does not come through here. It reads the wire
// proposals directly, in matchOfferedESPProposal (ike/engine/responder.go), because it
// also needs the Proposal Num that RFC 7296 Section 3.3.1 makes the response echo. That
// path compares the key length for equality too.
//
// The accepted proposal is returned with the peer's own attributes, which RFC 7296
// Section 3.3.6 requires to be returned unmodified.
func NegotiateESP(accepted, sent []ESPProposal) (ESPProposal, error) {
	return negotiateESP(accepted, sent, keyLengthExact)
}

// negotiateESP holds the ESP selection loop. It reads one rule for the Key Length
// attribute, exactly as negotiateIKE does.
func negotiateESP(remote, local []ESPProposal, rule keyLengthRule) (ESPProposal, error) {
	var reason error
	note := func(err error) {
		if reason == nil {
			reason = err
		}
	}
	for i := range remote {
		if err := espProposalComplete(&remote[i]); err != nil {
			note(err)
			continue
		}
		tried := false
		for j := range local {
			chosen, err := matchESP(&remote[i], &local[j], rule)
			if err == nil {
				return chosen, nil
			}
			note(err)
			tried = true
		}
		if !tried {
			note(ErrNoProposalChosen)
		}
	}
	if reason == nil {
		reason = ErrNoProposalChosen
	}
	return ESPProposal{}, reason
}
