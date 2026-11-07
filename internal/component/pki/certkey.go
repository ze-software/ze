// Design: docs/architecture/pki/pki-store.md -- PKI config parser
// Related: config.go -- verifyKeyMatchesCert reads a certificate's key through here
// Related: types.go -- the certificate summaries name the key through here
// RFC: rfc/full/rfc8410.txt -- Section 4, the id-Ed448 SubjectPublicKeyInfo

package pki

import (
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/ed448"
)

// ed448KeyAlgorithm names an Ed448 certificate key in show output, where
// crypto/x509 has no PublicKeyAlgorithm for it.
const ed448KeyAlgorithm = "Ed448"

// certificatePublicKey answers cert's public key: the one crypto/x509 parsed, or
// an Ed448 key read out of the raw SubjectPublicKeyInfo. crypto/x509 parses a
// certificate whose key is id-Ed448 without error and leaves PublicKey nil, so
// the nil is never handed on as an answer.
func certificatePublicKey(cert *x509.Certificate) (crypto.PublicKey, error) {
	if cert.PublicKey != nil {
		return cert.PublicKey, nil
	}
	pub, err := ed448.ParsePKIXPublicKey(cert.RawSubjectPublicKeyInfo)
	if errors.Is(err, ed448.ErrNotEd448) {
		return nil, fmt.Errorf("%w: the certificate's public key algorithm is not one ze reads", errPKIKeyUnsupported)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPKIKeyUnsupported, err)
	}
	return pub, nil
}

// keyAlgorithmName names cert's public key algorithm for an operator.
func keyAlgorithmName(cert *x509.Certificate) string {
	if cert.PublicKeyAlgorithm != x509.UnknownPublicKeyAlgorithm {
		return cert.PublicKeyAlgorithm.String()
	}
	if _, err := ed448.ParsePKIXPublicKey(cert.RawSubjectPublicKeyInfo); err == nil {
		return ed448KeyAlgorithm
	}
	return cert.PublicKeyAlgorithm.String()
}
