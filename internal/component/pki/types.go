// Design: plan/spec-ipsec-1-pki-store.md -- PKI certificate store types

package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"time"
)

// CACertEntry holds a parsed CA certificate.
type CACertEntry struct {
	Name        string
	Certificate *x509.Certificate
	Raw         []byte
}

// CertificateEntry holds a parsed device certificate with its private key
// and optional intermediate certificate.
type CertificateEntry struct {
	Name         string
	Certificate  *x509.Certificate
	Raw          []byte
	PrivateKey   crypto.PrivateKey
	Intermediate *x509.Certificate
	RawInter     []byte
}

// CertSummary is the JSON-serializable summary for show pki certificates.
type CertSummary struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	SubjectCN string `json:"subject-cn"`
	IssuerCN  string `json:"issuer-cn"`
	NotAfter  string `json:"not-after"`
	KeyAlgo   string `json:"key-algorithm"`
	Valid     bool   `json:"valid"`
}

// PKIConfig holds all parsed certificates from a config tree.
type PKIConfig struct {
	CACerts      map[string]*CACertEntry
	Certificates map[string]*CertificateEntry
}

func certSummary(name, typ string, cert *x509.Certificate, valid bool) CertSummary {
	return CertSummary{
		Name:      name,
		Type:      typ,
		SubjectCN: cert.Subject.CommonName,
		IssuerCN:  cert.Issuer.CommonName,
		NotAfter:  cert.NotAfter.UTC().Format(time.RFC3339),
		KeyAlgo:   cert.PublicKeyAlgorithm.String(),
		Valid:     valid,
	}
}

func keyUsageStrings(ku x509.KeyUsage) []string {
	var out []string
	pairs := []struct {
		bit  x509.KeyUsage
		name string
	}{
		{x509.KeyUsageDigitalSignature, "DigitalSignature"},
		{x509.KeyUsageContentCommitment, "ContentCommitment"},
		{x509.KeyUsageKeyEncipherment, "KeyEncipherment"},
		{x509.KeyUsageDataEncipherment, "DataEncipherment"},
		{x509.KeyUsageKeyAgreement, "KeyAgreement"},
		{x509.KeyUsageCertSign, "CertSign"},
		{x509.KeyUsageCRLSign, "CRLSign"},
	}
	for _, p := range pairs {
		if ku&p.bit != 0 {
			out = append(out, p.name)
		}
	}
	return out
}

func keySize(cert *x509.Certificate) int {
	switch pub := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		return pub.Size() * 8
	case *ecdsa.PublicKey:
		return pub.Curve.Params().BitSize
	case ed25519.PublicKey:
		return len(pub) * 8
	default:
		return 0
	}
}
