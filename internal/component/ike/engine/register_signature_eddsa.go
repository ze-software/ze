// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- AUTH payload computation
// Overview: auth_signature_scheme.go -- the Digital Signature algorithm table this family joins
// RFC: rfc/short/rfc7427.md -- Digital Signature AUTH method 14
// RFC: rfc/full/rfc8420.txt -- EdDSA in IKEv2 (no summary: no release profile lists RFC 8420)
package engine

import (
	"crypto"
	"crypto/ed25519"
	"encoding/asn1"
	"fmt"
)

// RFC 8420 Appendix A.1: "id-Ed25519 OBJECT IDENTIFIER ::= { 1.3.101.112 }"
// and "Parameters are absent.  Length is 7 bytes." The encoding is
// 30 05 06 03 2B 65 70, and only that encoding is accepted on receipt: oid is nil, so
// an AlgorithmIdentifier that carries parameters, or a bare OID, names no algorithm.
var (
	algIDEd25519 = algorithmIdentifierEC(asn1.ObjectIdentifier{1, 3, 101, 112})

	ed25519Identity = signatureAlgorithm{algID: algIDEd25519, hash: hashIdentity}
)

func init() {
	registerSignatureScheme(&signatureScheme{
		keyName: "an Ed25519 key",
		signs: func(key crypto.PrivateKey) bool {
			_, ok := key.(ed25519.PrivateKey)
			return ok
		},
		verifies: func(pub crypto.PublicKey) bool {
			_, ok := pub.(ed25519.PublicKey)
			return ok
		},
		// RFC 8420 Section 3: "For this reason, implementations MUST have the
		// "Identity" value in the SIGNATURE_HASH_ALGORITHMS notification when EdDSA
		// is supported and configured." Ze verifies Ed25519 from any peer, and the
		// notify goes out before the peer's certificate is known, so Identity is
		// always announced. The SHA-2 values stay beside it because RSA and ECDSA
		// need a hash, which is the case Section 3's SHOULD NOT leaves open.
		announce:   []uint16{hashAlgoIdentity},
		choose:     chooseEd25519Signature,
		sign:       signEd25519,
		verify:     verifyEd25519,
		algorithms: []signatureAlgorithm{ed25519Identity},
	})
}

// chooseEd25519Signature answers Ed25519 under the Identity hash, when the peer
// announced Identity.
func chooseEd25519Signature(_ crypto.PrivateKey, remoteHashAlgos []uint16) (signatureAlgorithm, error) {
	// RFC 8420 Section 2: "Ed25519 and Ed448 are only defined with the "Identity" hash and MUST NOT be sent to a receiver that has not indicated support for the "Identity" hash."
	if !containsHashAlgo(remoteHashAlgos, hashAlgoIdentity) {
		return signatureAlgorithm{}, fmt.Errorf(
			"%w: an Ed25519 key signs only under the Identity hash (5) of RFC 8420, and the "+
				"peer sent %v. Enable EdDSA (RFC 8420) signature authentication on the peer, "+
				"or reissue the certificate with an ECDSA or RSA key",
			errNoMutualHashAlgo, remoteHashAlgos)
	}
	return ed25519Identity, nil
}

// signEd25519 signs the signed octets whole with pure Ed25519 (RFC 8032 Section 5.1).
func signEd25519(key crypto.PrivateKey, octets []byte, h crypto.Hash) ([]byte, error) {
	k, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errUnsupportedKey
	}
	// RFC 8420 Section 2: "The prehashed versions of Ed25519 and Ed448 (Ed25519ph and Ed448ph, respectively) MUST NOT be used in IKE."
	if h != hashIdentity {
		return nil, fmt.Errorf("%w: an Ed25519 key signs only under the Identity hash, and %v was asked for",
			errUnsupportedKey, h)
	}
	// ed25519.Sign panics on a key of the wrong length, and the PKI store is the
	// only producer of this key: refuse rather than trust that.
	if len(k) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%w: the Ed25519 private key is %d octets, not %d",
			errUnsupportedKey, len(k), ed25519.PrivateKeySize)
	}
	return ed25519.Sign(k, octets), nil
}

// verifyEd25519 verifies a pure Ed25519 signature over the signed octets whole.
func verifyEd25519(pub crypto.PublicKey, octets, signature []byte, h crypto.Hash) error {
	k, ok := pub.(ed25519.PublicKey)
	if !ok {
		return errUnsupportedKey
	}
	// RFC 8420 Section 2: "The prehashed versions of Ed25519 and Ed448 (Ed25519ph and Ed448ph, respectively) MUST NOT be used in IKE."
	// A hash here also means the AlgorithmIdentifier named an RSA or ECDSA
	// algorithm while the certificate holds an Ed25519 key.
	if h != hashIdentity {
		return fmt.Errorf("%w: the certificate holds an Ed25519 key, and the AlgorithmIdentifier names a %v signature",
			errSignatureFailed, h)
	}
	// ed25519.Verify panics on a public key of the wrong length. crypto/x509 checks
	// the length when it parses the certificate, and this check pairs it.
	if len(k) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: the certificate's Ed25519 key is %d octets, not %d",
			errSignatureFailed, len(k), ed25519.PublicKeySize)
	}
	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("%w: an Ed25519 signature is %d octets, and the AUTH carries %d",
			errSignatureFailed, ed25519.SignatureSize, len(signature))
	}
	if !ed25519.Verify(k, octets, signature) {
		return errSignatureFailed
	}
	return nil
}
