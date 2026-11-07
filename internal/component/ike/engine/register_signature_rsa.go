// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- AUTH payload computation
// Overview: auth_signature_scheme.go -- the Digital Signature algorithm table this family joins
// RFC: rfc/short/rfc7427.md -- Digital Signature AUTH method 14
package engine

import (
	"crypto"
	crand "crypto/rand"
	"crypto/rsa"
	"encoding/asn1"
	"fmt"
)

// The RSASSA-PKCS1-v1_5 OIDs (RFC 7427 Appendix A). The bare encodings are the
// match keys of a received AlgorithmIdentifier; what Ze emits is the wrapped form.
var (
	oidSHA256WithRSA = bareOID(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11})
	oidSHA384WithRSA = bareOID(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12})
	oidSHA512WithRSA = bareOID(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13})

	algIDSHA256WithRSA = algorithmIdentifierRSA(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11})
	algIDSHA384WithRSA = algorithmIdentifierRSA(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12})
	algIDSHA512WithRSA = algorithmIdentifierRSA(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13})
)

var (
	rsaSHA256 = signatureAlgorithm{algID: algIDSHA256WithRSA, oid: oidSHA256WithRSA, hash: crypto.SHA256}
	rsaSHA384 = signatureAlgorithm{algID: algIDSHA384WithRSA, oid: oidSHA384WithRSA, hash: crypto.SHA384}
	rsaSHA512 = signatureAlgorithm{algID: algIDSHA512WithRSA, oid: oidSHA512WithRSA, hash: crypto.SHA512}
)

func init() {
	registerSignatureScheme(&signatureScheme{
		keyName: "an RSA key",
		signs: func(key crypto.PrivateKey) bool {
			_, ok := key.(*rsa.PrivateKey)
			return ok
		},
		verifies: func(pub crypto.PublicKey) bool {
			_, ok := pub.(*rsa.PublicKey)
			return ok
		},
		announce:   []uint16{hashAlgoSHA2256, hashAlgoSHA2384, hashAlgoSHA2512},
		choose:     chooseRSASignature,
		sign:       signRSA,
		verify:     verifyRSA,
		algorithms: []signatureAlgorithm{rsaSHA256, rsaSHA384, rsaSHA512},
	})
}

// chooseRSASignature picks the strongest SHA-2 the peer offered. An RSA key is not
// signed with SHA-1 to satisfy a SHA-1-only peer, because Ze emits no SHA-1 signature.
func chooseRSASignature(_ crypto.PrivateKey, remoteHashAlgos []uint16) (signatureAlgorithm, error) {
	if containsHashAlgo(remoteHashAlgos, hashAlgoSHA2512) {
		return rsaSHA512, nil
	}
	if containsHashAlgo(remoteHashAlgos, hashAlgoSHA2384) {
		return rsaSHA384, nil
	}
	if containsHashAlgo(remoteHashAlgos, hashAlgoSHA2256) {
		return rsaSHA256, nil
	}
	return signatureAlgorithm{}, fmt.Errorf(
		"%w: an RSA key needs SHA2-256, SHA2-384 or SHA2-512, and the peer sent %v",
		errNoMutualHashAlgo, remoteHashAlgos)
}

// signRSA signs a digest with RSASSA-PKCS1-v1_5.
func signRSA(key crypto.PrivateKey, digest []byte, h crypto.Hash) ([]byte, error) {
	k, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errUnsupportedKey
	}
	if h == hashIdentity {
		return nil, fmt.Errorf("%w: an RSA key signs a digest, never the Identity hash", errUnsupportedKey)
	}
	return rsa.SignPKCS1v15(crand.Reader, k, h, digest)
}

// verifyRSA verifies an RSASSA-PKCS1-v1_5 signature over a digest.
func verifyRSA(pub crypto.PublicKey, digest, signature []byte, h crypto.Hash) error {
	k, ok := pub.(*rsa.PublicKey)
	if !ok {
		return errUnsupportedKey
	}
	if h == hashIdentity {
		return fmt.Errorf("%w: the AlgorithmIdentifier names the Identity hash, and the certificate holds an RSA key",
			errSignatureFailed)
	}
	return rsa.VerifyPKCS1v15(k, h, digest, signature)
}
