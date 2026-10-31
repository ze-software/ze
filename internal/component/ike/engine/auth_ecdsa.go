// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- AUTH payload computation
// Related: auth.go -- verifyRemoteAuth dispatches AUTH methods 9, 10 and 11 here
// Related: auth_legacy.go -- computeLegacySignatureAuth signs with them when no SIGNATURE_HASH_ALGORITHMS notify arrived
// RFC: rfc/short/rfc4754.md -- ECDSA-256, ECDSA-384 and ECDSA-521 (AUTH methods 9, 10 and 11)
package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
	"math/big"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// ecdsaMethod is one RFC 4754 signature algorithm: its IKEv2 AUTH method number,
// the curve the signer's key is on, the hash it signs with, and the fixed octet
// length of each of r and s.
//
// RFC 4754 Section 4: "Each of these algorithms represents an instantiation of the
// ECDSA algorithm using a particular elliptic curve group and hash function."
type ecdsaMethod struct {
	name         string
	authMethod   uint8
	curve        elliptic.Curve
	newHash      func() hash.Hash
	scalarOctets int
}

// errECDSASignatureLength refuses an RFC 4754 AUTH whose length is not that of
// r || s for its method.
var errECDSASignatureLength = errors.New("ike auth: ECDSA AUTH is not r || s of the method's length")

// The three algorithms of RFC 4754 Section 4, with the r and s lengths of Section 7
// (256, 384 and 528 bits). ECDSA-521 pads to 528 bits, a whole number of octets.
var (
	ecdsa256 = ecdsaMethod{name: "ECDSA-256", authMethod: wire.AuthMethodECDSA256, curve: elliptic.P256(), newHash: sha256.New, scalarOctets: 32}
	ecdsa384 = ecdsaMethod{name: "ECDSA-384", authMethod: wire.AuthMethodECDSA384, curve: elliptic.P384(), newHash: sha512.New384, scalarOctets: 48}
	ecdsa521 = ecdsaMethod{name: "ECDSA-521", authMethod: wire.AuthMethodECDSA521, curve: elliptic.P521(), newHash: sha512.New, scalarOctets: 66}
)

// ecdsaMethods holds the three methods in RFC 4754 Section 4 order. The signer
// finds a key's method here by its curve, so the curve-to-method binding is
// declared once, on the method values the verifier also reads.
var ecdsaMethods = [...]ecdsaMethod{ecdsa256, ecdsa384, ecdsa521}

// ecdsaMethodForCurve answers the RFC 4754 method bound to curve, and false for a
// curve RFC 4754 defines no method for.
func ecdsaMethodForCurve(curve elliptic.Curve) (ecdsaMethod, bool) {
	for _, method := range ecdsaMethods {
		if method.curve == curve {
			return method, true
		}
	}
	return ecdsaMethod{}, false
}

// signECDSAAuth signs an AUTH payload of method 9, 10 or 11 with key, the method
// chosen by key's curve. It is the inverse of verifyECDSAAuth, with the same wire
// format.
//
// RFC 4754 Section 4: "the elliptic curve group must be specified since the choice
// of hash function depends on it as well." A key on any other curve has no method,
// and is refused.
func signECDSAAuth(sa *SA, certName string, key *ecdsa.PrivateKey, signedOctets []byte) (*wire.PayloadAUTH, error) {
	method, ok := ecdsaMethodForCurve(key.Curve)
	if !ok {
		return nil, fmt.Errorf(
			"%w: certificate %q holds an ECDSA key on %s, peer %q sent no "+
				"SIGNATURE_HASH_ALGORITHMS notify, and RFC 4754 defines ECDSA methods for "+
				"P-256, P-384 and P-521 only. Reissue the certificate with a key on one of "+
				"those curves or an RSA key, or enable RFC 7427 signature authentication on "+
				"the peer",
			errUnsupportedKey, certName, key.Curve.Params().Name, sa.PeerName)
	}

	h := method.newHash()
	h.Write(signedOctets)
	digest := h.Sum(nil)

	r, s, err := ecdsa.Sign(crand.Reader, key, digest)
	if err != nil {
		return nil, fmt.Errorf("ike auth: %s signature for peer %q: %w", method.name, sa.PeerName, err)
	}

	// RFC 4754 Section 7: "the signature payload SHALL contain an encoding of the
	// computed signature consisting of the concatenation of a pair of integers r and
	// s." and "The bit lengths of r and s are enforced, if necessary, by pre-pending
	// the value with zeros." FillBytes writes each big-endian and left-pads it.
	authData := make([]byte, 2*method.scalarOctets)
	r.FillBytes(authData[:method.scalarOctets])
	s.FillBytes(authData[method.scalarOctets:])

	return &wire.PayloadAUTH{
		AuthMethod: method.authMethod,
		AuthData:   authData,
	}, nil
}

// verifyECDSAAuth verifies an AUTH payload of method 9, 10 or 11 that the remote
// peer sent.
//
// Ze signs with these methods only when the peer sent no SIGNATURE_HASH_ALGORITHMS
// notify (signECDSAAuth), and with RFC 7427 method 14 otherwise. A peer may sign
// with them whatever Ze sent, and an EAP initiator verifies the responder's
// signature without signing one itself.
//
// Wire format of the Authentication Data (RFC 4754 Section 7), n being 32, 48 or
// 66 octets for ECDSA-256, ECDSA-384 and ECDSA-521:
//
//	offset 0     r, big-endian, left-padded with zeros to n octets
//	offset n     s, big-endian, left-padded with zeros to n octets
//	length 2n
//
// RFC 4754 Section 7: "When ECDSA-256, ECDSA-384, or ECDSA-521 is used as the
// digital signature in IKE or IKEv2, the signature payload SHALL contain an
// encoding of the computed signature consisting of the concatenation of a pair of
// integers r and s."
func verifyECDSAAuth(sa *SA, method ecdsaMethod, authData, signedOctets []byte) error {
	// RFC 4754 Section 7: "The bit lengths of r and s are enforced, if necessary,
	// by pre-pending the value with zeros." Any other length is not r || s: an RFC
	// 7427 DER signature sent under this method number lands here.
	if len(authData) != 2*method.scalarOctets {
		return fmt.Errorf(
			"%w: peer %q sent an %s AUTH of %d octets, and RFC 4754 Section 7 makes it "+
				"r and s of %d octets each (%d in all). Configure the peer to sign with %s "+
				"or with RFC 7427 Digital Signature (method 14)",
			errECDSASignatureLength, sa.PeerName, method.name, len(authData), method.scalarOctets,
			2*method.scalarOctets, method.name)
	}

	cert, err := getRemoteCert(sa)
	if err != nil {
		return err
	}
	publicKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf(
			"%w: peer %q signed with %s, and its certificate does not hold an ECDSA key. "+
				"Reissue the peer's certificate with an ECDSA key",
			errUnsupportedKey, sa.PeerName, method.name)
	}
	// RFC 4754 Section 4: "the elliptic curve group must be specified since the
	// choice of hash function depends on it as well."
	if publicKey.Curve != method.curve {
		return fmt.Errorf(
			"%w: peer %q signed with %s, which RFC 4754 Section 4 binds to %s, and its "+
				"certificate key is on %s. Configure the peer to sign with the method of its "+
				"key's curve",
			errUnsupportedKey, sa.PeerName, method.name, method.curve.Params().Name,
			publicKey.Curve.Params().Name)
	}

	h := method.newHash()
	h.Write(signedOctets)
	digest := h.Sum(nil)

	r := new(big.Int).SetBytes(authData[:method.scalarOctets])
	s := new(big.Int).SetBytes(authData[method.scalarOctets:])
	if !ecdsa.Verify(publicKey, digest, r, s) {
		return errSignatureFailed
	}
	return nil
}
