// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- AUTH payload computation
// Overview: auth_signature_scheme.go -- the Digital Signature algorithm table this family joins
// RFC: rfc/short/rfc7427.md -- Digital Signature AUTH method 14
package engine

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"encoding/asn1"
	"fmt"
)

// The ecdsa-with-SHA2 OIDs (RFC 7427 Appendix A). The bare encodings are the match
// keys of a received AlgorithmIdentifier; what Ze emits is the wrapped form.
var (
	oidECDSASHA256 = bareOID(asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2})
	oidECDSASHA384 = bareOID(asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3})

	algIDECDSASHA256 = algorithmIdentifierEC(asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2})
	algIDECDSASHA384 = algorithmIdentifierEC(asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3})
)

var (
	ecdsaSHA256 = signatureAlgorithm{algID: algIDECDSASHA256, oid: oidECDSASHA256, hash: crypto.SHA256}
	ecdsaSHA384 = signatureAlgorithm{algID: algIDECDSASHA384, oid: oidECDSASHA384, hash: crypto.SHA384}
)

func init() {
	registerSignatureScheme(&signatureScheme{
		keyName: "an ECDSA key",
		signs: func(key crypto.PrivateKey) bool {
			_, ok := key.(*ecdsa.PrivateKey)
			return ok
		},
		verifies: func(pub crypto.PublicKey) bool {
			_, ok := pub.(*ecdsa.PublicKey)
			return ok
		},
		announce:   []uint16{hashAlgoSHA2256, hashAlgoSHA2384},
		choose:     chooseECDSASignature,
		sign:       signECDSA,
		verify:     verifyECDSA,
		algorithms: []signatureAlgorithm{ecdsaSHA256, ecdsaSHA384},
	})
}

// chooseECDSASignature answers the one algorithm the key's curve names, when the peer
// offered its hash. P-384 signs with SHA2-384 and every other curve with SHA2-256, as
// before this selection consulted the peer's list at all: what the list decides is
// whether that hash is a choice the RFC allows here. An ECDSA key is not paired with a
// hash its curve does not name, because that pairing is one Ze has never emitted and
// never tested against a peer.
func chooseECDSASignature(key crypto.PrivateKey, remoteHashAlgos []uint16) (signatureAlgorithm, error) {
	k, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return signatureAlgorithm{}, errUnsupportedKey
	}
	if k.Curve == elliptic.P384() {
		if containsHashAlgo(remoteHashAlgos, hashAlgoSHA2384) {
			return ecdsaSHA384, nil
		}
		return signatureAlgorithm{}, fmt.Errorf("%w: a P-384 key signs with SHA2-384, and the peer sent %v",
			errNoMutualHashAlgo, remoteHashAlgos)
	}
	if containsHashAlgo(remoteHashAlgos, hashAlgoSHA2256) {
		return ecdsaSHA256, nil
	}
	return signatureAlgorithm{}, fmt.Errorf("%w: a %s key signs with SHA2-256, and the peer sent %v",
		errNoMutualHashAlgo, k.Curve.Params().Name, remoteHashAlgos)
}

// signECDSA signs a digest, DER-encoding r and s.
func signECDSA(key crypto.PrivateKey, digest []byte, h crypto.Hash) ([]byte, error) {
	k, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errUnsupportedKey
	}
	if h == hashIdentity {
		return nil, fmt.Errorf("%w: an ECDSA key signs a digest, never the Identity hash", errUnsupportedKey)
	}
	// RFC 7427 Section 3: "For hash truncation, the method specified in ANSI
	// X9.62:2005 [X9.62] MUST be used." ecdsa.SignASN1 applies it to the whole
	// digest, so the digest is never cut here.
	return ecdsa.SignASN1(crand.Reader, k, digest)
}

// verifyECDSA verifies a DER-encoded ECDSA signature over a digest.
func verifyECDSA(pub crypto.PublicKey, digest, signature []byte, h crypto.Hash) error {
	k, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return errUnsupportedKey
	}
	if h == hashIdentity {
		return fmt.Errorf("%w: the AlgorithmIdentifier names the Identity hash, and the certificate holds an ECDSA key",
			errSignatureFailed)
	}
	if !ecdsa.VerifyASN1(k, digest, signature) {
		return errSignatureFailed
	}
	return nil
}
