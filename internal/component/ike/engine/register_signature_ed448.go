// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- AUTH payload computation
// Overview: auth_signature_scheme.go -- the Digital Signature algorithm table this family joins
// Related: register_signature_eddsa.go -- Ed25519, the other RFC 8420 curve
// Related: ../../../core/ed448/ed448.go -- the Ed448 arithmetic and key encodings, Ze's only circl importer
// RFC: rfc/short/rfc7427.md -- Digital Signature AUTH method 14
// RFC: rfc/full/rfc8420.txt -- EdDSA in IKEv2 (no summary: no release profile lists RFC 8420)
package engine

import (
	"crypto"
	"encoding/asn1"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/ed448"
)

// RFC 8420 Appendix A.2: "id-Ed448 OBJECT IDENTIFIER ::= { 1.3.101.113 }"
// and "Parameters are absent.  Length is 7 bytes." The encoding is
// 30 05 06 03 2B 65 71, and only that encoding is accepted on receipt: oid is nil, so
// an AlgorithmIdentifier that carries parameters, or a bare OID, names no algorithm.
var (
	algIDEd448 = algorithmIdentifierEC(asn1.ObjectIdentifier{1, 3, 101, 113})

	ed448Identity = signatureAlgorithm{algID: algIDEd448, hash: hashIdentity}
)

func init() {
	registerSignatureScheme(&signatureScheme{
		keyName: "an Ed448 key",
		signs: func(key crypto.PrivateKey) bool {
			_, ok := key.(ed448.PrivateKey)
			return ok
		},
		verifies: func(pub crypto.PublicKey) bool {
			_, ok := pub.(ed448.PublicKey)
			return ok
		},
		// RFC 8420 Section 3: "For this reason, implementations MUST have the
		// "Identity" value in the SIGNATURE_HASH_ALGORITHMS notification when EdDSA
		// is supported and configured." The Ed25519 family announces it too, and
		// announcedHashAlgos sends it once.
		announce:       []uint16{hashAlgoIdentity},
		choose:         chooseEd448Signature,
		sign:           signEd448,
		verify:         verifyEd448,
		algorithms:     []signatureAlgorithm{ed448Identity},
		certificateKey: ed448CertificateKey,
	})
}

// chooseEd448Signature answers Ed448 under the Identity hash, when the peer
// announced Identity.
func chooseEd448Signature(_ crypto.PrivateKey, remoteHashAlgos []uint16) (signatureAlgorithm, error) {
	// RFC 8420 Section 2: "Ed25519 and Ed448 are only defined with the "Identity" hash and MUST NOT be sent to a receiver that has not indicated support for the "Identity" hash."
	if !containsHashAlgo(remoteHashAlgos, hashAlgoIdentity) {
		return signatureAlgorithm{}, fmt.Errorf(
			"%w: an Ed448 key signs only under the Identity hash (5) of RFC 8420, and the "+
				"peer sent %v. Enable EdDSA (RFC 8420) signature authentication on the peer, "+
				"or reissue the certificate with an ECDSA or RSA key",
			errNoMutualHashAlgo, remoteHashAlgos)
	}
	return ed448Identity, nil
}

// signEd448 signs the signed octets whole with pure Ed448 and an empty context
// (RFC 8032 Section 5.2).
func signEd448(key crypto.PrivateKey, octets []byte, h crypto.Hash) ([]byte, error) {
	k, ok := key.(ed448.PrivateKey)
	if !ok {
		return nil, errUnsupportedKey
	}
	// RFC 8420 Section 2: "The prehashed versions of Ed25519 and Ed448 (Ed25519ph and Ed448ph, respectively) MUST NOT be used in IKE."
	if h != hashIdentity {
		return nil, fmt.Errorf("%w: an Ed448 key signs only under the Identity hash, and %v was asked for",
			errUnsupportedKey, h)
	}
	signature, err := ed448.Sign(k, octets)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errUnsupportedKey, err)
	}
	return signature, nil
}

// verifyEd448 verifies a pure Ed448 signature over the signed octets whole.
func verifyEd448(pub crypto.PublicKey, octets, signature []byte, h crypto.Hash) error {
	k, ok := pub.(ed448.PublicKey)
	if !ok {
		return errUnsupportedKey
	}
	// RFC 8420 Section 2: "The prehashed versions of Ed25519 and Ed448 (Ed25519ph and Ed448ph, respectively) MUST NOT be used in IKE."
	// A hash here also means the AlgorithmIdentifier named an RSA or ECDSA
	// algorithm while the certificate holds an Ed448 key.
	if h != hashIdentity {
		return fmt.Errorf("%w: the certificate holds an Ed448 key, and the AlgorithmIdentifier names a %v signature",
			errSignatureFailed, h)
	}
	if len(signature) != ed448.SignatureSize {
		return fmt.Errorf("%w: an Ed448 signature is %d octets, and the AUTH carries %d",
			errSignatureFailed, ed448.SignatureSize, len(signature))
	}
	if !ed448.Verify(k, octets, signature) {
		return errSignatureFailed
	}
	return nil
}

// ed448CertificateKey reads an Ed448 key out of a certificate's
// SubjectPublicKeyInfo, which crypto/x509 parses without error and leaves
// Certificate.PublicKey nil for.
func ed448CertificateKey(spki []byte) (crypto.PublicKey, error) {
	pub, err := ed448.ParsePKIXPublicKey(spki)
	if errors.Is(err, ed448.ErrNotEd448) {
		return nil, errNotThisFamily
	}
	if err != nil {
		return nil, err
	}
	return pub, nil
}
