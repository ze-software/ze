// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- AUTH payload computation
// Overview: auth.go -- computeX509Auth and verifyX509Auth sign and verify through this table
// Detail: register_signature_rsa.go -- RSASSA-PKCS1-v1_5 with SHA2-256, SHA2-384 and SHA2-512
// Detail: register_signature_ecdsa.go -- ECDSA with the hash its curve names
// Detail: register_signature_eddsa.go -- Ed25519 under the Identity hash
// Detail: register_signature_ed448.go -- Ed448 under the Identity hash
// RFC: rfc/short/rfc7427.md -- Digital Signature AUTH method 14
// RFC: rfc/full/rfc8420.txt -- the Identity hash (no summary: no release profile lists RFC 8420)
package engine

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"hash"
	"slices"
)

// signatureScheme is one family of RFC 7427 Digital Signature algorithms, keyed by
// the key type it signs and verifies with.
//
// Each family registers itself from init() in its own register_signature_*.go
// file, and every question the Digital Signature method asks of a key type is
// answered from the registered families: which algorithm to sign with for a peer's
// list, how to sign, which received AlgorithmIdentifier names the family, how to
// verify, which hash identifiers Ze announces, and how the key is named to an
// operator. A new family adds its own file and edits nothing here.
//
// The table is written only by init(), before any goroutine reads it, so it is safe
// for concurrent use afterwards.
type signatureScheme struct {
	// keyName names the family's key for an operator, as in "an Ed25519 key".
	keyName string
	// signs and verifies say whether the family owns a private or a public key.
	signs    func(key crypto.PrivateKey) bool
	verifies func(pub crypto.PublicKey) bool
	// announce lists the IKEv2 hash identifiers the family needs in the
	// SIGNATURE_HASH_ALGORITHMS notify Ze sends.
	announce []uint16
	// choose picks the algorithm to sign with out of the hash identifiers the peer
	// sent, or refuses with errNoMutualHashAlgo and the reason.
	choose func(key crypto.PrivateKey, remoteHashAlgos []uint16) (signatureAlgorithm, error)
	// sign signs input: the digest of the signed octets, or the signed octets
	// themselves when h is hashIdentity.
	sign func(key crypto.PrivateKey, input []byte, h crypto.Hash) ([]byte, error)
	// verify checks signature over input, which h produced from the signed octets.
	// It MUST refuse an h its family does not sign with: the received
	// AlgorithmIdentifier chose h and the certificate chose this family, and the two
	// can disagree (an id-Ed25519 AUTH from an ECDSA certificate).
	verify func(pub crypto.PublicKey, input, signature []byte, h crypto.Hash) error
	// algorithms are every algorithm the family recognizes in a received AUTH.
	algorithms []signatureAlgorithm
	// certificateKey reads the family's public key out of a certificate's DER
	// SubjectPublicKeyInfo, for a key type crypto/x509 does not parse (it leaves
	// Certificate.PublicKey nil). It answers errNotThisFamily for a key of another
	// family. Nil means crypto/x509 parses the family's keys itself.
	certificateKey func(spki []byte) (crypto.PublicKey, error)
}

// errNotThisFamily is a certificateKey's answer for a SubjectPublicKeyInfo whose
// algorithm belongs to another family.
var errNotThisFamily = errors.New("ike auth: the certificate key belongs to another signature family")

// signatureAlgorithm is one algorithm of a family.
type signatureAlgorithm struct {
	// algID is the AlgorithmIdentifier Ze emits, SEQUENCE { OID [, parameters] }.
	algID []byte
	// oid is the bare OID a received AlgorithmIdentifier is matched on, whatever its
	// parameters, as RSA and ECDSA have always been. Nil means only algID itself is
	// accepted, byte for byte, because the RFC fixes the parameters.
	oid []byte
	// hash is the hash run over the signed octets before signing; hashIdentity runs
	// none.
	hash crypto.Hash
}

// hashIdentity is the hash of an algorithm that signs the signed octets whole, the
// "Identity" of RFC 8420. It is crypto.Hash(0) because that is the value the
// standard library's crypto.SignerOpts gives a message that was not hashed:
// ed25519.PrivateKey.Sign signs pure Ed25519 for it, and Ed25519ph only for SHA-512.
// A family whose algorithms need a hash refuses hashIdentity in sign and verify, so
// an algorithm entry written without a hash fails closed.
const hashIdentity crypto.Hash = 0

// Hash algorithm identifiers of IANA's IKEv2 "Hash Algorithms" registry, which is what
// a SIGNATURE_HASH_ALGORITHMS notify carries (RFC 7427 Section 4). Identifier 0 is
// RESERVED, so no valid identifier is the zero value. SHA-1 (1) has no constant here
// because Ze emits no SHA-1 signature: a peer that offers it alone offers nothing Ze
// can sign with.
const (
	hashAlgoSHA2256 uint16 = 2
	hashAlgoSHA2384 uint16 = 3
	hashAlgoSHA2512 uint16 = 4
	// RFC 8420 Section 2: "This document defines a new value called "Identity" (5) in
	// the "IKEv2 Hash Algorithms" registry for use in the SIGNATURE_HASH_ALGORITHMS
	// notification."
	hashAlgoIdentity uint16 = 5
)

// signatureSchemes holds the registered families, in registration order.
var signatureSchemes []*signatureScheme

// registerSignatureScheme adds a family to the table. It MUST be called from init().
func registerSignatureScheme(s *signatureScheme) {
	signatureSchemes = append(signatureSchemes, s)
}

// signingScheme answers the family that signs with key, and false when none does.
func signingScheme(key crypto.PrivateKey) (*signatureScheme, bool) {
	for _, s := range signatureSchemes {
		if s.signs(key) {
			return s, true
		}
	}
	return nil, false
}

// verifyingScheme answers the family that verifies with pub, and false when none does.
func verifyingScheme(pub crypto.PublicKey) (*signatureScheme, bool) {
	for _, s := range signatureSchemes {
		if s.verifies(pub) {
			return s, true
		}
	}
	return nil, false
}

// certificatePublicKey answers the public key of cert: the one crypto/x509 parsed,
// or, for a key type it leaves unparsed, the one a registered family reads out of
// the raw SubjectPublicKeyInfo. A key no family reads is errUnsupportedKey, never
// a nil key handed to a verifier.
func certificatePublicKey(cert *x509.Certificate) (crypto.PublicKey, error) {
	if cert.PublicKey != nil {
		return cert.PublicKey, nil
	}
	for _, s := range signatureSchemes {
		if s.certificateKey == nil {
			continue
		}
		pub, err := s.certificateKey(cert.RawSubjectPublicKeyInfo)
		if errors.Is(err, errNotThisFamily) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%w: the certificate's %s does not decode: %w", errUnsupportedKey, s.keyName, err)
		}
		return pub, nil
	}
	return nil, fmt.Errorf("%w: crypto/x509 and no signature family read the certificate's public key", errUnsupportedKey)
}

// announcedHashAlgos answers the hash identifiers Ze's SIGNATURE_HASH_ALGORITHMS
// notify carries: every identifier a registered family needs, once each, ascending.
func announcedHashAlgos() []uint16 {
	var algos []uint16
	for _, s := range signatureSchemes {
		for _, id := range s.announce {
			if !slices.Contains(algos, id) {
				algos = append(algos, id)
			}
		}
	}
	slices.Sort(algos)
	return algos
}

// selectSignatureAlgorithm picks the algorithm for one Digital Signature AUTH payload
// out of the hash algorithms the peer sent in its SIGNATURE_HASH_ALGORITHMS notify.
//
// RFC 7427 Section 4 MUST: "Both ends send their list of supported hash algorithms.
// When calculating the digital signature, a peer MUST pick one algorithm sent by the
// other peer." The offered list is therefore the whole population to choose from, and
// a local capability that is not in it is not a candidate. The key's family decides
// which of the offered identifiers it can sign with; a key no family signs with is
// errUnsupportedKey.
func selectSignatureAlgorithm(key crypto.PrivateKey, remoteHashAlgos []uint16) (algID []byte, h crypto.Hash, err error) {
	scheme, ok := signingScheme(key)
	if !ok {
		return nil, 0, errUnsupportedKey
	}
	alg, err := scheme.choose(key, remoteHashAlgos)
	if err != nil {
		return nil, 0, err
	}
	return alg.algID, alg.hash, nil
}

// containsHashAlgo reports whether the peer sent target in its
// SIGNATURE_HASH_ALGORITHMS notify.
//
// An EMPTY list is answered false for every target, because a peer that sent no
// notify offered no algorithm. This function used to answer true for every target
// instead, which made "no notify arrived" indistinguishable from "every algorithm is
// acceptable" and let Ze sign with an algorithm no peer had ever named. RFC 7427
// Section 4 gives the list the opposite force: a signer MUST pick one algorithm the
// other peer sent, and nothing sent leaves nothing to pick.
func containsHashAlgo(algos []uint16, target uint16) bool {
	return slices.Contains(algos, target)
}

// signDigest signs input with key's family. Input is the digest h produced, or the
// signed octets whole when h is hashIdentity.
func signDigest(key crypto.PrivateKey, input []byte, h crypto.Hash) ([]byte, error) {
	scheme, ok := signingScheme(key)
	if !ok {
		return nil, errUnsupportedKey
	}
	return scheme.sign(key, input, h)
}

// receivedSignatureAlgorithm finds the registered algorithm a received
// AlgorithmIdentifier names, and false when none does.
//
// RFC 7427 Section 3 defines the field as an ASN.1 AlgorithmIdentifier, SEQUENCE {
// OID [, params] }, which is what strongSwan sends. Ze accepts a bare OID too for the
// families that match on the OID alone. A family that leaves oid nil is matched on its
// whole encoding, so its parameters are exactly the ones the RFC fixes.
func receivedSignatureAlgorithm(algID []byte) (signatureAlgorithm, bool) {
	for _, s := range signatureSchemes {
		if alg, ok := s.recognizes(algID); ok {
			return alg, true
		}
	}
	return signatureAlgorithm{}, false
}

// recognizes answers the family's algorithm a received AlgorithmIdentifier names,
// and false when it names none of the family's.
func (s *signatureScheme) recognizes(algID []byte) (signatureAlgorithm, bool) {
	oid := receivedOID(algID)
	for _, alg := range s.algorithms {
		if bytes.Equal(algID, alg.algID) {
			return alg, true
		}
		if alg.oid != nil && bytes.Equal(oid, alg.oid) {
			return alg, true
		}
	}
	return signatureAlgorithm{}, false
}

// receivedOID answers the OID of a received AlgorithmIdentifier SEQUENCE, or algID
// itself when it is not one (a bare OID).
func receivedOID(algID []byte) []byte {
	if len(algID) <= 2 {
		return algID
	}
	if algID[0] != 0x30 {
		return algID
	}
	var inner asn1.RawValue
	if _, err := asn1.Unmarshal(algID, &inner); err != nil {
		return algID
	}
	var oidVal asn1.RawValue
	if _, err := asn1.Unmarshal(inner.Bytes, &oidVal); err != nil {
		return algID
	}
	if oidVal.Tag != asn1.TagOID {
		return algID
	}
	return oidVal.FullBytes
}

// requireKeyFamilyNamesAlgorithm refuses a received AlgorithmIdentifier that the
// family of the certificate's key does not own.
//
// RFC 7427 Section 3: "Computed as specified in Section 2.15 of [RFC7296] using a private
// key associated with the public key sent in the Certificate payload". The
// AlgorithmIdentifier names the signature algorithm, so it has to be one of the
// certificate key's own family. The hash does not decide this: Ed25519 and Ed448
// both sign under the Identity hash, so an Ed448 signature labeled id-Ed25519 from
// an Ed448 certificate would otherwise verify under a name it does not carry.
func requireKeyFamilyNamesAlgorithm(pubKey crypto.PublicKey, algID []byte) error {
	scheme, ok := verifyingScheme(pubKey)
	if !ok {
		return errUnsupportedKey
	}
	if _, ok := scheme.recognizes(algID); !ok {
		return fmt.Errorf("%w: the certificate holds %s, and the AlgorithmIdentifier % x names another family's algorithm",
			errSignatureFailed, scheme.keyName, algID)
	}
	return nil
}

// hashFromAlgID answers the hash a received AlgorithmIdentifier names, as the
// constructor verifyX509Auth runs over the signed octets. For the Identity hash it
// answers newIdentityHash, whose sum is the signed octets themselves.
func hashFromAlgID(algID []byte) func() hash.Hash {
	alg, ok := receivedSignatureAlgorithm(algID)
	// refuse-by-default: guards verifyX509Auth -- nil is its "unsupported algorithm identifier" refusal
	if !ok {
		return nil
	}
	if alg.hash == hashIdentity {
		return newIdentityHash
	}
	return alg.hash.New
}

// verifySignature checks signature over digest with pubKey's family. The family
// refuses a hash it does not sign with. verifyX509Auth first refuses an
// AlgorithmIdentifier the key's family does not own (requireKeyFamilyNamesAlgorithm),
// which the hash alone cannot decide when two families share the Identity hash.
func verifySignature(pubKey crypto.PublicKey, digest, signature []byte, hashFunc func() hash.Hash) error {
	scheme, ok := verifyingScheme(pubKey)
	if !ok {
		return errUnsupportedKey
	}
	h, err := cryptoHashFromFunc(hashFunc)
	if err != nil {
		return err
	}
	return scheme.verify(pubKey, digest, signature, h)
}

// cryptoHashFromFunc answers the crypto.Hash a hashFromAlgID constructor builds. A
// constructor of an unknown size is refused rather than read as SHA-256.
func cryptoHashFromFunc(hashFunc func() hash.Hash) (crypto.Hash, error) {
	h := hashFunc()
	if _, ok := h.(*identityHash); ok {
		return hashIdentity, nil
	}
	switch h.Size() {
	case 32:
		return crypto.SHA256, nil
	case 48:
		return crypto.SHA384, nil
	case 64:
		return crypto.SHA512, nil
	}
	return 0, fmt.Errorf("ike auth: no signature hash has a %d-octet digest", h.Size())
}

// identityHash is RFC 8420's "Identity" hash as a hash.Hash: Sum answers the bytes
// written, so the hash-then-verify pipeline of verifyX509Auth hands an EdDSA verifier
// the signed octets whole. It is not safe for concurrent use.
type identityHash struct {
	written []byte
}

// newIdentityHash answers an empty identityHash.
func newIdentityHash() hash.Hash {
	return &identityHash{}
}

// Write keeps p. It never fails.
func (h *identityHash) Write(p []byte) (int, error) {
	h.written = append(h.written, p...)
	return len(p), nil
}

// Sum appends the bytes written to b.
func (h *identityHash) Sum(b []byte) []byte {
	return append(b, h.written...)
}

// Reset forgets the bytes written.
func (h *identityHash) Reset() {
	h.written = h.written[:0]
}

// Size answers how many octets Sum appends, which is how many were written.
func (h *identityHash) Size() int {
	return len(h.written)
}

// BlockSize answers 1: the identity consumes its input an octet at a time.
func (h *identityHash) BlockSize() int {
	return 1
}

// rsaAlgorithmIdentifier and ecAlgorithmIdentifier are the two DER shapes of
// RFC 5280 Section 4.1.1.2 AlgorithmIdentifier that IKEv2 signature auth uses.
// They differ only in the parameters field, and the difference is required
// rather than cosmetic: RFC 4055 Section 5 says the parameters of an
// RSASSA-PKCS1-v1_5 algorithm MUST be present and MUST be NULL, RFC 5758
// Section 3.2 says the parameters of ecdsa-with-SHA256 and ecdsa-with-SHA384
// MUST be absent, and RFC 8420 Appendix A says those of id-Ed25519 are absent.
type rsaAlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}

type ecAlgorithmIdentifier struct {
	Algorithm asn1.ObjectIdentifier
}

// algorithmIdentifierRSA encodes SEQUENCE { OID, NULL }.
//
// RFC 7427 Section 3 defines the Authentication Data as an ASN.1 Length octet,
// then an "ASN.1 AlgorithmIdentifier object", then the signature. An
// AlgorithmIdentifier is a SEQUENCE that wraps the OID, so the bare OID Ze used
// to emit was not one: strongSwan parses that field as ASN.1 and refuses the
// payload with "digital signature authentication payload invalid", which failed
// every certificate-authenticated exchange against it.
func algorithmIdentifierRSA(oid asn1.ObjectIdentifier) []byte {
	der, err := asn1.Marshal(rsaAlgorithmIdentifier{Algorithm: oid, Parameters: asn1.NullRawValue})
	if err != nil {
		panic("BUG: cannot DER-encode a constant RSA signature AlgorithmIdentifier")
	}
	return der
}

// algorithmIdentifierEC encodes SEQUENCE { OID }, the shape of an AlgorithmIdentifier
// whose parameters are absent (ECDSA and EdDSA).
func algorithmIdentifierEC(oid asn1.ObjectIdentifier) []byte {
	der, err := asn1.Marshal(ecAlgorithmIdentifier{Algorithm: oid})
	if err != nil {
		panic("BUG: cannot DER-encode a constant parameterless signature AlgorithmIdentifier")
	}
	return der
}

// bareOID DER-encodes oid alone, the match key of a family that accepts any
// parameters.
func bareOID(oid asn1.ObjectIdentifier) []byte {
	der, err := asn1.Marshal(oid)
	if err != nil {
		panic("BUG: cannot DER-encode a constant signature algorithm OID")
	}
	return der
}
