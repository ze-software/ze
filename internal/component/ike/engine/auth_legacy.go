// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- AUTH payload computation
// Related: auth.go -- computeX509Auth signs here when the peer sent no SIGNATURE_HASH_ALGORITHMS notify
// Related: auth_ecdsa.go -- the RFC 4754 ECDSA methods 9, 10 and 11, signed and verified
// RFC: rfc/short/rfc8247.md -- Section 3.1, RSA Digital Signature (AUTH method 1) is MUST
// RFC: rfc/short/rfc7296.md -- Section 3.8, RSA Digital Signature with RSASSA-PKCS1-v1_5
package engine

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/fips140"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // RFC 7296 Section 3.8 makes SHA-1 the default hash of AUTH method 1
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// errLegacyRSAFIPSOnly refuses an RSA Digital Signature (method 1) while Go runs in
// FIPS 140-3 only mode, which refuses SHA-1 signatures.
var errLegacyRSAFIPSOnly = errors.New(
	"ike auth: GODEBUG=fips140=only refuses SHA-1 signatures, and RSA Digital Signature " +
		"(AUTH method 1) signs with SHA-1")

// fipsOnlyEnforced reports whether Go's FIPS 140-3 only mode (GODEBUG=fips140=only)
// is in force. The mode is fixed when the process starts, so a test replaces this
// variable to reach the refusal; nothing else writes it.
var fipsOnlyEnforced = fips140.Enforced

// legacyRSAVerifyHashes are the hashes a method 1 signature's DigestInfo may name,
// SHA-1 first. rsa.VerifyPKCS1v15 checks the DigestInfo against the hash it is
// given, so each candidate verifies only a signature made with that hash.
//
// RFC 7296 Section 3.8: "To promote interoperability, implementations that support
// this type SHOULD support signatures that use SHA-1 as the hash function and SHOULD
// use SHA-1 as the default hash function when generating signatures."
var legacyRSAVerifyHashes = [...]crypto.Hash{crypto.SHA1, crypto.SHA256, crypto.SHA384, crypto.SHA512}

// computeLegacySignatureAuth signs the AUTH payload with the legacy method that fits
// key, for a peer that sent no SIGNATURE_HASH_ALGORITHMS notify and so may not be
// sent the Digital Signature method (RFC 7427 Section 3).
//
// RFC 8247 Section 3.1 grades the methods this leaves: RSA Digital Signature (1)
// MUST, and ECDSA with SHA-256, SHA-384 and SHA-512 on P-256, P-384 and P-521 (9, 10
// and 11) SHOULD. Any other key has no method to sign with, and is refused.
func computeLegacySignatureAuth(sa *SA, certName string, key crypto.PrivateKey, signedOctets []byte) (*wire.PayloadAUTH, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return signLegacyRSAAuth(sa, k, signedOctets)
	case *ecdsa.PrivateKey:
		return signECDSAAuth(sa, certName, k, signedOctets)
	}
	return nil, fmt.Errorf(
		"%w: certificate %q holds %s, and peer %q sent no SIGNATURE_HASH_ALGORITHMS "+
			"notify, which leaves only RSA Digital Signature (method 1) and ECDSA on P-256, "+
			"P-384 or P-521 (methods 9, 10 and 11) to sign with. Reissue the certificate with "+
			"an RSA key or an ECDSA key on one of those curves, or enable RFC 7427 signature "+
			"authentication on the peer",
		errUnsupportedKey, certName, legacyKeyName(key), sa.PeerName)
}

// legacyKeyName names, for an operator, a certificate key no legacy method signs
// with. A key type Ze has no name for is described rather than guessed.
func legacyKeyName(key crypto.PrivateKey) string {
	if _, ok := key.(ed25519.PrivateKey); ok {
		return "an Ed25519 key"
	}
	return "a key of a type IKEv2 defines no legacy signature method for"
}

// signLegacyRSAAuth signs an RSA Digital Signature (method 1) AUTH payload. The
// Authentication Data is the bare RSASSA-PKCS1-v1_5 signature, as long as the key's
// modulus, with no algorithm identifier in front of it.
//
// RFC 7296 Section 3.8: "Computed as specified in Section 2.15 using an RSA private
// key with RSASSA-PKCS1-v1_5 signature scheme specified in [PKCS1]"
func signLegacyRSAAuth(sa *SA, key *rsa.PrivateKey, signedOctets []byte) (*wire.PayloadAUTH, error) {
	if fipsOnlyEnforced() {
		return nil, fmt.Errorf(
			"%w: peer %q sent no SIGNATURE_HASH_ALGORITHMS notify, so method 1 is the only "+
				"RSA signature Ze may send it. Enable RFC 7427 signature authentication on "+
				"the peer, or run ze without GODEBUG=fips140=only",
			errLegacyRSAFIPSOnly, sa.PeerName)
	}

	// RFC 7296 Section 3.8: "SHOULD use SHA-1 as the default hash function when
	// generating signatures."
	digest := sha1.Sum(signedOctets) //nolint:gosec // RFC 7296 Section 3.8 default hash of method 1
	signature, err := rsa.SignPKCS1v15(crand.Reader, key, crypto.SHA1, digest[:])
	if err != nil {
		return nil, fmt.Errorf("ike auth: RSA Digital Signature for peer %q: %w", sa.PeerName, err)
	}
	return &wire.PayloadAUTH{
		AuthMethod: wire.AuthMethodRSASig,
		AuthData:   signature,
	}, nil
}

// verifyLegacyRSAAuth verifies an RSA Digital Signature (method 1) AUTH payload.
//
// Method 1 does not say which hash the signer used; the PKCS #1 DigestInfo inside
// the signature does. Each hash of legacyRSAVerifyHashes is tried, SHA-1 first, the
// default RFC 7296 Section 3.8 names and the only one strongSwan signs method 1 with.
// Under GODEBUG=fips140=only SHA-1 is skipped, because Go refuses it there, and a
// signature that verifies under no other hash is refused naming that mode.
func verifyLegacyRSAAuth(sa *SA, authData, signedOctets []byte) error {
	cert, err := getRemoteCert(sa)
	if err != nil {
		return err
	}
	rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf(
			"%w: peer %q signed with RSA Digital Signature (method 1), and its certificate "+
				"does not hold an RSA key. Reissue the peer's certificate with an RSA key",
			errUnsupportedKey, sa.PeerName)
	}

	fipsOnly := fipsOnlyEnforced()
	for _, hashFunc := range legacyRSAVerifyHashes {
		if fipsOnly && hashFunc == crypto.SHA1 {
			continue
		}
		h := hashFunc.New()
		h.Write(signedOctets)
		if rsa.VerifyPKCS1v15(rsaPub, hashFunc, h.Sum(nil), authData) == nil {
			return nil
		}
	}
	if fipsOnly {
		return fmt.Errorf(
			"%w: peer %q sent an RSA Digital Signature (method 1) that no SHA-2 hash "+
				"verifies, and GODEBUG=fips140=only stops Ze checking SHA-1, the default of "+
				"RFC 7296 Section 3.8. Enable RFC 7427 signature authentication on the peer, "+
				"or run ze without GODEBUG=fips140=only",
			errSignatureFailed, sa.PeerName)
	}
	return errSignatureFailed
}
