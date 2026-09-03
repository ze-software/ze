// Design: docs/architecture/pki/tls-listeners.md -- server TLS material from the PKI store
// Related: show.go -- certBundlePEM produces the same leaf+intermediate shape for display
// Related: store.go -- Validate applies the same chain verification at commit time

package pki

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"sort"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Doctor codes this package emits for a configured certificate reference.
// CodeCertExpired matches the code the file-based DoT/DoH check already uses
// (internal/core/dnsserver/certcheck.go): an expired certificate has the same
// operator fix whether it came from a file or from the store.
const (
	CodeCertReference = "doctor-tls-reference"
	CodeCertExpired   = "doctor-tls-expired"
)

// certExpiryWarnWindow is how far ahead of NotAfter a certificate is reported as
// a warning. Mirrors the file-based window in dnsserver/certcheck.go and
// component/doctor/checks_tls.go so every TLS surface warns at the same point.
const certExpiryWarnWindow = 30 * 24 * time.Hour

// errServerTLSNoName is the only fixed error here; the rest name the
// certificate the operator wrote, so they are built per call.
var errServerTLSNoName = errors.New("pki: server TLS material requires a certificate name")

// ServerTLSMaterial returns the PEM material a TLS server needs to serve the
// named store certificate with its full chain: the leaf CERTIFICATE block
// followed by one block per stored intermediate, and the PKCS#8 private key.
//
// The block order is load-bearing. tls.X509KeyPair parses every CERTIFICATE
// block into tls.Certificate.Certificate and TLS requires the sender's own
// certificate at index 0, so the leaf MUST come first
// (selfcert.TestNewTLSConfigServesChain proves the stdlib behavior this relies
// on).
//
// It fails rather than substituting anything when the name does not resolve or
// the entry holds no private key. A caller that asked for a named certificate
// must never end up serving a different one: an operator who configured their
// own chain would see a working HTTPS listener and never learn that ze fell back
// to a self-signed certificate.
func ServerTLSMaterial(name string) (certPEM, keyPEM []byte, err error) {
	if name == "" {
		return nil, nil, errServerTLSNoName
	}

	s := get()
	entry, ok := s.certificates[name]
	if !ok {
		var tb textbuf.Buffer
		tb.Str("pki: certificate ").Str(name).Str(" not found")
		if names := certificateNames(s); len(names) > 0 {
			tb.Str(" (available: ").Join(names, ", ").Byte(')')
		}
		return nil, nil, errors.New(tb.String())
	}
	if entry.PrivateKey == nil {
		var tb textbuf.Buffer
		return nil, nil, errors.New(tb.Str("pki: certificate ").Str(name).Str(" has no private key").String())
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(entry.PrivateKey)
	if err != nil {
		var tb textbuf.Buffer
		return nil, nil, errors.New(tb.Str("pki: certificate ").Str(name).Str(": marshal private key: ").Err(err).String())
	}

	return chainPEM(entry), pem.EncodeToMemory(&pem.Block{Type: pemBlockPrivateKey, Bytes: keyDER}), nil
}

// chainPEM concatenates the leaf certificate and every stored intermediate into
// one PEM document, leaf first. Same shape as certBundlePEM's certificate half
// (show.go), which is what `show pki certificate name <n> bundle pem` prints.
func chainPEM(entry *CertificateEntry) []byte {
	out := pem.EncodeToMemory(&pem.Block{Type: pemBlockCertificate, Bytes: entry.Raw})
	for _, inter := range entry.RawIntermediates {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: pemBlockCertificate, Bytes: inter})...)
	}
	return out
}

// certificateNames returns the sorted device-certificate names in the store.
func certificateNames(s *storeState) []string {
	names := make([]string, 0, len(s.certificates))
	for n := range s.certificates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Severity values a CertProblem carries. They spell what dnsserver.CertProblem
// uses, so a consumer maps both sources into its diagnostics the same way.
const (
	severityError   = "error"
	severityWarning = "warning"
)

// CertProblem is a single doctor finding about a configured certificate
// reference. Code is one of the registered doctor-tls-* codes; Severity is
// severityError or severityWarning. Shaped like dnsserver.CertProblem so a consumer maps
// both sources into its diagnostics the same way.
type CertProblem struct {
	Code     string
	Severity string
	Message  string
}

// CheckCertReference validates that a configured certificate name can actually
// serve TLS, against a config PARSED OFFLINE rather than the live store, so
// `ze doctor` reports a broken reference before the config is committed.
//
// An empty name means no reference is configured and is never a problem. now is
// injected so callers and tests control the expiry evaluation point.
func CheckCertReference(cfg *PKIConfig, name string, now time.Time) []CertProblem {
	if name == "" {
		return nil
	}

	var entry *CertificateEntry
	if cfg != nil {
		entry = cfg.Certificates[name]
	}
	if entry == nil {
		var tb textbuf.Buffer
		tb.Str("certificate ").Str(name).Str(" is referenced but the pki block defines no certificate with that name")
		if cfg != nil {
			if names := configCertificateNames(cfg); len(names) > 0 {
				tb.Str(" (defined: ").Join(names, ", ").Byte(')')
			}
		}
		return []CertProblem{{Code: CodeCertReference, Severity: severityError, Message: tb.String()}}
	}

	var problems []CertProblem

	if entry.PrivateKey == nil {
		var tb textbuf.Buffer
		problems = append(problems, CertProblem{
			Code:     CodeCertReference,
			Severity: severityError,
			Message: tb.Str("certificate ").Str(name).
				Str(" has no private key, so it cannot serve TLS (add private { key ... } to the pki entry)").String(),
		})
	}

	cert := entry.Certificate
	switch {
	case now.After(cert.NotAfter) || now.Before(cert.NotBefore):
		var tb textbuf.Buffer
		problems = append(problems, CertProblem{
			Code:     CodeCertExpired,
			Severity: severityError,
			Message: tb.Str("certificate ").Str(name).Str(" is outside its validity window (not-before ").
				Str(cert.NotBefore.UTC().Format(time.RFC3339)).Str(", not-after ").
				Str(cert.NotAfter.UTC().Format(time.RFC3339)).Byte(')').String(),
		})
		// An expired certificate fails chain verification too; reporting both
		// would bury the fix that matters under a derived symptom.
		return problems
	case cert.NotAfter.Sub(now) < certExpiryWarnWindow:
		daysLeft := int(cert.NotAfter.Sub(now).Hours() / 24)
		var tb textbuf.Buffer
		problems = append(problems, CertProblem{
			Code:     CodeCertExpired,
			Severity: severityWarning,
			Message:  tb.Str("certificate ").Str(name).Str(" expires in ").Int(int64(daysLeft)).Str(" day(s)").String(),
		})
	}

	if err := verifyEntryChain(cfg, entry, now); err != nil {
		var tb textbuf.Buffer
		problems = append(problems, CertProblem{
			Code:     CodeCertReference,
			Severity: severityError,
			Message: tb.Str("certificate ").Str(name).
				Str(" does not build a chain to a configured ca certificate: ").Err(err).
				Str(" (check the intermediate matches the leaf's issuer)").String(),
		})
	}

	return problems
}

// verifyEntryChain runs the same verification pki.Validate applies at commit
// time (store.go), so doctor and commit agree on what a serveable chain is. A
// leaf issued directly by a stored root and carrying no intermediate verifies
// cleanly, which is why AC-11 raises no diagnostic.
func verifyEntryChain(cfg *PKIConfig, entry *CertificateEntry, now time.Time) error {
	caPool := x509.NewCertPool()
	if cfg != nil {
		for _, ca := range cfg.CACerts {
			caPool.AddCert(ca.Certificate)
		}
	}
	intermediatePool := x509.NewCertPool()
	for _, inter := range entry.Intermediates {
		intermediatePool.AddCert(inter)
	}
	_, err := entry.Certificate.Verify(x509.VerifyOptions{
		Roots:         caPool,
		Intermediates: intermediatePool,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	return err
}

// configCertificateNames returns the sorted certificate names a parsed config
// defines.
func configCertificateNames(cfg *PKIConfig) []string {
	names := make([]string, 0, len(cfg.Certificates))
	for n := range cfg.Certificates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
