// Design: plan/learned/733-pki-store.md -- PKI config parser

package pki

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
)

const maxNameLen = 255

var (
	errPKICertEmpty      = errors.New("pki: certificate leaf is empty")
	errPKICertDecode     = errors.New("pki: certificate base64 decode failed")
	errPKIKeyEmpty       = errors.New("pki: private key leaf is empty")
	errPKIKeyDecode      = errors.New("pki: private key base64 decode failed")
	errPKIKeyMismatch    = errors.New("pki: private key does not match certificate public key")
	errPKIKeyUnsupported = errors.New("pki: unsupported private key type")
	errPKINameInvalid    = errors.New("pki: name contains invalid characters (allowed: alphanumeric, dash, underscore, dot)")
	errPKINameTooLong    = errors.New("pki: name exceeds 255 characters")
)

func validateName(name string) error {
	if name == "" {
		return errPKINameInvalid
	}
	if len(name) > maxNameLen {
		return errPKINameTooLong
	}
	for _, c := range name {
		if !isNameChar(c) {
			return errPKINameInvalid
		}
	}
	return nil
}

func isNameChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.'
}

// ParseConfig extracts PKI certificates and keys from the config tree.
// Returns a zero-value PKIConfig if no pki {} block is present.
func ParseConfig(tree *config.Tree) (*PKIConfig, error) {
	if tree == nil {
		return &PKIConfig{
			CACerts:      make(map[string]*CACertEntry),
			Certificates: make(map[string]*CertificateEntry),
		}, nil
	}

	pkiRoot := tree.GetContainer("pki")
	if pkiRoot == nil {
		return &PKIConfig{
			CACerts:      make(map[string]*CACertEntry),
			Certificates: make(map[string]*CertificateEntry),
		}, nil
	}

	cfg := &PKIConfig{
		CACerts:      make(map[string]*CACertEntry),
		Certificates: make(map[string]*CertificateEntry),
	}

	caList := pkiRoot.GetList("ca")
	for name, caTree := range caList {
		if err := validateName(name); err != nil {
			return nil, fmt.Errorf("pki ca %q: %w", name, err)
		}
		entry, err := parseCACert(name, caTree)
		if err != nil {
			return nil, fmt.Errorf("pki ca %q: %w", name, err)
		}
		cfg.CACerts[name] = entry
	}

	certList := pkiRoot.GetList("certificate")
	for name, certTree := range certList {
		if err := validateName(name); err != nil {
			return nil, fmt.Errorf("pki certificate %q: %w", name, err)
		}
		entry, err := parseDeviceCert(name, certTree)
		if err != nil {
			return nil, fmt.Errorf("pki certificate %q: %w", name, err)
		}
		cfg.Certificates[name] = entry
	}

	return cfg, nil
}

func parseCACert(name string, tree *config.Tree) (*CACertEntry, error) {
	certB64, ok := tree.Get("certificate")
	if !ok || certB64 == "" {
		return nil, errPKICertEmpty
	}

	der, err := base64.StdEncoding.DecodeString(certB64)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPKICertDecode, err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("pki: x509 parse: %w", err)
	}

	return &CACertEntry{
		Name:        name,
		Certificate: cert,
		Raw:         der,
	}, nil
}

func parseDeviceCert(name string, tree *config.Tree) (*CertificateEntry, error) {
	certB64, ok := tree.Get("certificate")
	if !ok || certB64 == "" {
		return nil, errPKICertEmpty
	}

	der, err := base64.StdEncoding.DecodeString(certB64)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPKICertDecode, err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("pki: x509 parse: %w", err)
	}

	entry := &CertificateEntry{
		Name:        name,
		Certificate: cert,
		Raw:         der,
	}

	// intermediate is a leaf-list, so an operator can name every certificate on the path
	// from this one toward the trust anchor. RFC 7296 Section 3.6 requires ze be capable
	// of being configured to send up to four X.509 certificates, and the leaf plus three
	// intermediates is that maximum. GetSlice reads a single value and a bracket list
	// alike, so a config naming one intermediate parses exactly as it did before.
	for _, interB64 := range tree.GetSlice("intermediate") {
		if interB64 == "" {
			continue
		}
		interDER, iErr := base64.StdEncoding.DecodeString(interB64)
		if iErr != nil {
			return nil, fmt.Errorf("pki: intermediate base64 decode: %w", iErr)
		}
		interCert, iErr := x509.ParseCertificate(interDER)
		if iErr != nil {
			return nil, fmt.Errorf("pki: intermediate x509 parse: %w", iErr)
		}
		entry.Intermediates = append(entry.Intermediates, interCert)
		entry.RawIntermediates = append(entry.RawIntermediates, interDER)
	}

	privContainer := tree.GetContainer("private")
	if privContainer != nil {
		keyB64, kOk := privContainer.Get("key")
		if kOk && keyB64 != "" {
			privKey, kErr := parsePrivateKey(keyB64)
			if kErr != nil {
				return nil, kErr
			}
			if err := verifyKeyMatchesCert(privKey, cert); err != nil {
				return nil, err
			}
			entry.PrivateKey = privKey
		}
	}

	return entry, nil
}

func parsePrivateKey(b64 string) (any, error) {
	if b64 == "" {
		return nil, errPKIKeyEmpty
	}

	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPKIKeyDecode, err)
	}

	if key, pErr := x509.ParsePKCS8PrivateKey(der); pErr == nil {
		return key, nil
	}

	if key, pErr := x509.ParseECPrivateKey(der); pErr == nil {
		return key, nil
	}

	if key, pErr := x509.ParsePKCS1PrivateKey(der); pErr == nil {
		return key, nil
	}

	return nil, errPKIKeyUnsupported
}

func verifyKeyMatchesCert(privKey any, cert *x509.Certificate) error {
	switch k := privKey.(type) {
	case *ecdsa.PrivateKey:
		pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
		if !ok || !k.PublicKey.Equal(pub) {
			return errPKIKeyMismatch
		}
	case *rsa.PrivateKey:
		pub, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok || !k.PublicKey.Equal(pub) {
			return errPKIKeyMismatch
		}
	case ed25519.PrivateKey:
		pub, ok := cert.PublicKey.(ed25519.PublicKey)
		if !ok || !pub.Equal(ed25519.PublicKey(k[32:])) {
			return errPKIKeyMismatch
		}
	default:
		return errPKIKeyUnsupported
	}
	return nil
}
