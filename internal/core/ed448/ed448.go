// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- AUTH payload computation, the Ed448 signature family
// Related: ../../component/ike/engine/register_signature_ed448.go -- the IKE signature family this package serves
// RFC: rfc/full/rfc8032.txt -- Section 5.2, Ed448
// RFC: rfc/full/rfc8410.txt -- Sections 3, 4 and 7, the id-Ed448 key encodings
// RFC: rfc/full/rfc8420.txt -- EdDSA in IKEv2 (no summary: no release profile lists RFC 8420)

// Package ed448 is Ze's only importer of github.com/cloudflare/circl, which Thomas
// approved for Ed448 in IKE and for nothing else. Every call into circl is in this
// file, and no circl type leaves it: callers hold PrivateKey and PublicKey, which
// are Ze types. The package signs and verifies pure Ed448 with an empty context,
// and reads the two key encodings Go's crypto/x509 does not: a PKCS #8 private key
// and a SubjectPublicKeyInfo whose algorithm is id-Ed448.
//
// It is safe for concurrent use: every function is pure, and a key is immutable
// once constructed.
package ed448

import (
	"bytes"
	"crypto"
	"crypto/subtle"
	"encoding/asn1"
	"errors"
	"fmt"

	circled448 "github.com/cloudflare/circl/sign/ed448"
)

// Sizes of RFC 8032 Section 5.2: a 57-octet private key (the seed), a 57-octet
// public key and a 114-octet signature.
const (
	PublicKeySize = circled448.PublicKeySize
	SeedSize      = circled448.SeedSize
	SignatureSize = circled448.SignatureSize
)

// ErrNotEd448 says a key encoding names another algorithm than id-Ed448. A caller
// that tries several key families reads it as "not this family", and any other
// error as a malformed Ed448 key.
var ErrNotEd448 = errors.New("ed448: the key is not an id-Ed448 key")

// errMalformed says an id-Ed448 key encoding does not decode.
var errMalformed = errors.New("ed448: malformed id-Ed448 key")

// errNoKey says a zero PrivateKey was asked to sign.
var errNoKey = errors.New("ed448: the private key holds no key")

// algorithmIdentifier is the DER of the id-Ed448 AlgorithmIdentifier,
// SEQUENCE { OID 1.3.101.113 }, 30 05 06 03 2B 65 71.
//
// RFC 8410 Section 3: "For all of the OIDs, the parameters MUST be absent."
// Only this encoding names Ed448: an AlgorithmIdentifier with parameters is
// refused, not read as Ed448.
var algorithmIdentifier = []byte{0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x71}

// PublicKey is an Ed448 public key, the 57-octet encoding of RFC 8032 Section
// 5.2.5. Its zero value holds no key, and Verify refuses it.
type PublicKey struct {
	raw []byte
}

// PrivateKey is an Ed448 private key. Its zero value holds no key, and Sign
// refuses it.
type PrivateKey struct {
	key circled448.PrivateKey
}

// NewPublicKey answers the public key whose 57-octet encoding is raw. It checks
// the length only: a point that does not decode fails Verify.
func NewPublicKey(raw []byte) (PublicKey, error) {
	if len(raw) != PublicKeySize {
		return PublicKey{}, fmt.Errorf("%w: an Ed448 public key is %d octets, and this one is %d",
			errMalformed, PublicKeySize, len(raw))
	}
	return PublicKey{raw: bytes.Clone(raw)}, nil
}

// NewPrivateKeyFromSeed answers the private key whose 57-octet secret (the seed
// of RFC 8032 Section 5.2.5) is seed.
func NewPrivateKeyFromSeed(seed []byte) (PrivateKey, error) {
	// circled448.NewKeyFromSeed panics on a seed of another length, and the seed
	// comes from an operator's PEM: refuse it here.
	if len(seed) != SeedSize {
		return PrivateKey{}, fmt.Errorf("%w: an Ed448 private key is %d octets, and this one is %d",
			errMalformed, SeedSize, len(seed))
	}
	return PrivateKey{key: circled448.NewKeyFromSeed(seed)}, nil
}

// Bytes answers a copy of the key's 57-octet encoding.
func (pub PublicKey) Bytes() []byte {
	return bytes.Clone(pub.raw)
}

// Equal reports whether x is an Ed448 public key with the same encoding. The
// zero PublicKey equals nothing.
func (pub PublicKey) Equal(x crypto.PublicKey) bool {
	other, ok := x.(PublicKey)
	if !ok {
		return false
	}
	if len(pub.raw) != PublicKeySize {
		return false
	}
	return subtle.ConstantTimeCompare(pub.raw, other.raw) == 1
}

// Public answers the public key of priv, and the zero PublicKey for the zero
// PrivateKey.
func (priv PrivateKey) Public() PublicKey {
	if len(priv.key) != circled448.PrivateKeySize {
		return PublicKey{}
	}
	pub, ok := priv.key.Public().(circled448.PublicKey)
	if !ok {
		panic("BUG: circl's Ed448 private key answered a public key of another type")
	}
	return PublicKey{raw: bytes.Clone(pub)}
}

// Sign signs message whole with pure Ed448 and an empty context.
//
// RFC 8420 Section 2: "The prehashed versions of Ed25519 and Ed448 (Ed25519ph and Ed448ph, respectively) MUST NOT be used in IKE."
func Sign(priv PrivateKey, message []byte) ([]byte, error) {
	// circled448.Sign panics on a key of another length.
	if len(priv.key) != circled448.PrivateKeySize {
		return nil, errNoKey
	}
	// RFC 8032 Section 5.2: "Value of context is set by signer and verifier (maximum of 255
	// octets; the default is empty string) and has to match octet by octet
	// for verification to be successful."
	return circled448.Sign(priv.key, message, ""), nil
}

// Verify reports whether signature is a pure Ed448 signature by pub over message,
// with an empty context. A key or signature of the wrong length is false.
func Verify(pub PublicKey, message, signature []byte) bool {
	if len(pub.raw) != PublicKeySize {
		return false
	}
	// RFC 8032 Section 5.2.7: "If any of the decodings fail (including S being out of range), the signature is
	// invalid." circled448.Verify refuses a signature that is not 114 octets, and
	// an S at or above L, before it decodes anything.
	return circled448.Verify(circled448.PublicKey(pub.raw), message, signature, "")
}

// subjectPublicKeyInfo is RFC 8410 Section 4's SubjectPublicKeyInfo.
type subjectPublicKeyInfo struct {
	Algorithm asn1.RawValue
	PublicKey asn1.BitString
}

// ParsePKIXPublicKey reads the public key out of a DER SubjectPublicKeyInfo, as a
// certificate's RawSubjectPublicKeyInfo carries it. It answers ErrNotEd448 when
// the algorithm is another one.
//
//	SubjectPublicKeyInfo  ::=  SEQUENCE  {
//	    algorithm         AlgorithmIdentifier,   30 05 06 03 2B 65 71
//	    subjectPublicKey  BIT STRING             03 3A 00 <57 octets>
//	}
func ParsePKIXPublicKey(der []byte) (PublicKey, error) {
	var spki subjectPublicKeyInfo
	rest, err := asn1.Unmarshal(der, &spki)
	if err != nil {
		return PublicKey{}, fmt.Errorf("%w: SubjectPublicKeyInfo: %w", errMalformed, err)
	}
	if len(rest) != 0 {
		return PublicKey{}, fmt.Errorf("%w: %d octets follow the SubjectPublicKeyInfo", errMalformed, len(rest))
	}
	// RFC 8410 Section 3: "For all of the OIDs, the parameters MUST be absent."
	if !bytes.Equal(spki.Algorithm.FullBytes, algorithmIdentifier) {
		return PublicKey{}, ErrNotEd448
	}
	// RFC 8410 Section 4: "The algorithms defined in this document always encode the public key
	// as an exact multiple of 8 bits."
	if spki.PublicKey.BitLength != 8*PublicKeySize {
		return PublicKey{}, fmt.Errorf("%w: the subjectPublicKey is %d bits, not %d",
			errMalformed, spki.PublicKey.BitLength, 8*PublicKeySize)
	}
	return NewPublicKey(spki.PublicKey.Bytes)
}

// oneAsymmetricKey is RFC 8410 Section 7's OneAsymmetricKey (RFC 5958), the
// PKCS #8 shape of an Ed448 private key.
type oneAsymmetricKey struct {
	Version    int
	Algorithm  asn1.RawValue
	PrivateKey []byte
	Attributes asn1.RawValue  `asn1:"optional,tag:0"`
	PublicKey  asn1.BitString `asn1:"optional,tag:1"`
}

// ParsePKCS8PrivateKey reads an Ed448 private key out of a DER PKCS #8
// OneAsymmetricKey. It answers ErrNotEd448 when the algorithm is another one. A
// publicKey field, when present, has to be the key's own: a key whose public half
// disagrees with its private half is refused rather than trusted.
//
//	OneAsymmetricKey ::= SEQUENCE {
//	   version             INTEGER (0 or 1),
//	   privateKeyAlgorithm 30 05 06 03 2B 65 71,
//	   privateKey          OCTET STRING { CurvePrivateKey OCTET STRING, 57 octets },
//	   attributes          [0] IMPLICIT OPTIONAL,
//	   publicKey           [1] IMPLICIT BIT STRING OPTIONAL
//	}
func ParsePKCS8PrivateKey(der []byte) (PrivateKey, error) {
	var oak oneAsymmetricKey
	rest, err := asn1.Unmarshal(der, &oak)
	if err != nil {
		return PrivateKey{}, fmt.Errorf("%w: OneAsymmetricKey: %w", errMalformed, err)
	}
	if len(rest) != 0 {
		return PrivateKey{}, fmt.Errorf("%w: %d octets follow the OneAsymmetricKey", errMalformed, len(rest))
	}
	// RFC 8410 Section 3: "For all of the OIDs, the parameters MUST be absent."
	if !bytes.Equal(oak.Algorithm.FullBytes, algorithmIdentifier) {
		return PrivateKey{}, ErrNotEd448
	}
	if oak.Version < 0 {
		return PrivateKey{}, fmt.Errorf("%w: OneAsymmetricKey version %d", errMalformed, oak.Version)
	}
	if oak.Version > 1 {
		return PrivateKey{}, fmt.Errorf("%w: OneAsymmetricKey version %d", errMalformed, oak.Version)
	}
	// RFC 8410 Section 7: "Thus, when encoding a
	// OneAsymmetricKey object, the private key is wrapped in a
	// CurvePrivateKey object and wrapped by the OCTET STRING of the
	// "privateKey" field."
	var seed []byte
	inner, err := asn1.Unmarshal(oak.PrivateKey, &seed)
	if err != nil {
		return PrivateKey{}, fmt.Errorf("%w: CurvePrivateKey: %w", errMalformed, err)
	}
	if len(inner) != 0 {
		return PrivateKey{}, fmt.Errorf("%w: %d octets follow the CurvePrivateKey", errMalformed, len(inner))
	}
	priv, err := NewPrivateKeyFromSeed(seed)
	if err != nil {
		return PrivateKey{}, err
	}
	// The publicKey field is optional (RFC 5958); absent, the key is the seed's.
	if oak.PublicKey.BitLength == 0 {
		return priv, nil
	}
	carried, err := NewPublicKey(oak.PublicKey.Bytes)
	if err != nil {
		return PrivateKey{}, err
	}
	if !priv.Public().Equal(carried) {
		return PrivateKey{}, fmt.Errorf("%w: the publicKey field is not the private key's public key", errMalformed)
	}
	return priv, nil
}
