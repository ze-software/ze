// Design: plan/learned/733-pki-store.md -- PKI in-memory certificate store

package pki

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

var (
	errPKICertExpired     = errors.New("pki: certificate has expired")
	errPKIChainValidation = errors.New("pki: certificate chain validation failed")
)

// storeState is the immutable snapshot swapped atomically on reload.
type storeState struct {
	caCerts      map[string]*CACertEntry
	certificates map[string]*CertificateEntry
}

var (
	current atomic.Pointer[storeState]

	emptyState = &storeState{
		caCerts:      make(map[string]*CACertEntry),
		certificates: make(map[string]*CertificateEntry),
	}
)

// Validate checks chain validity and expiry without mutating the live store.
func Validate(cfg *PKIConfig) error {
	if cfg == nil {
		return nil
	}

	now := time.Now()
	for name, ca := range cfg.CACerts {
		if now.After(ca.Certificate.NotAfter) {
			return fmt.Errorf("%w: ca %q expired at %s",
				errPKICertExpired, name, ca.Certificate.NotAfter.UTC().Format(time.RFC3339))
		}
	}

	caPool := x509.NewCertPool()
	for _, ca := range cfg.CACerts {
		caPool.AddCert(ca.Certificate)
	}

	for name, entry := range cfg.Certificates {
		if now.After(entry.Certificate.NotAfter) {
			return fmt.Errorf("%w: certificate %q expired at %s",
				errPKICertExpired, name, entry.Certificate.NotAfter.UTC().Format(time.RFC3339))
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
		if err != nil {
			return fmt.Errorf("%w: certificate %q: %w", errPKIChainValidation, name, err)
		}
	}

	return nil
}

// Load validates and atomically installs a new PKIConfig into the store.
func Load(cfg *PKIConfig) error {
	if cfg == nil {
		current.Store(emptyState)
		return nil
	}

	if err := Validate(cfg); err != nil {
		return err
	}

	current.Store(&storeState{
		caCerts:      cfg.CACerts,
		certificates: cfg.Certificates,
	})
	RaiseExpiryWarnings()
	return nil
}

func get() *storeState {
	s := current.Load()
	if s == nil {
		return emptyState
	}
	return s
}

// GetCA returns the named CA certificate, or nil if not found.
func GetCA(name string) *CACertEntry {
	return get().caCerts[name]
}

// GetCertificate returns the named device certificate, or nil if not found.
func GetCertificate(name string) *CertificateEntry {
	return get().certificates[name]
}

// CertCN returns the subject common name of the named device certificate,
// or empty string if not found.
func CertCN(name string) string {
	entry := get().certificates[name]
	if entry == nil {
		return ""
	}
	return entry.Certificate.Subject.CommonName
}

// CAPool returns an x509.CertPool containing all loaded CA certificates.
func CAPool() *x509.CertPool {
	s := get()
	pool := x509.NewCertPool()
	for _, ca := range s.caCerts {
		pool.AddCert(ca.Certificate)
	}
	return pool
}

// IntermediatePool returns an x509.CertPool containing all intermediate
// certificates from loaded device certificates.
func IntermediatePool() *x509.CertPool {
	s := get()
	pool := x509.NewCertPool()
	for _, entry := range s.certificates {
		for _, inter := range entry.Intermediates {
			pool.AddCert(inter)
		}
	}
	return pool
}

// ListCACerts returns all loaded CA certificate entries.
func ListCACerts() []*CACertEntry {
	s := get()
	out := make([]*CACertEntry, 0, len(s.caCerts))
	for _, ca := range s.caCerts {
		out = append(out, ca)
	}
	return out
}

// ListCertificates returns all loaded device certificate entries.
func ListCertificates() []*CertificateEntry {
	s := get()
	out := make([]*CertificateEntry, 0, len(s.certificates))
	for _, entry := range s.certificates {
		out = append(out, entry)
	}
	return out
}

const (
	exportDir     = "/tmp/ze-ipsec"
	pemFilePerm   = 0o600
	exportDirPerm = 0o700
)

// safeName strips directory components to prevent path traversal.
func safeName(name string) string {
	return filepath.Base(name)
}

// findIssuerCA returns the CA name whose SubjectKeyIdentifier matches
// the device cert's AuthorityKeyIdentifier. Falls back to subject CN match.
func findIssuerCA(cert *x509.Certificate, caCerts map[string]*CACertEntry) (string, *CACertEntry) {
	if len(cert.AuthorityKeyId) > 0 {
		for name, ca := range caCerts {
			if bytes.Equal(ca.Certificate.SubjectKeyId, cert.AuthorityKeyId) {
				return name, ca
			}
		}
	}
	issuerCN := cert.Issuer.CommonName
	for name, ca := range caCerts {
		if ca.Certificate.Subject.CommonName == issuerCN {
			return name, ca
		}
	}
	return "", nil
}

// ExportPEM writes PEM files for the named certificate and its CA chain
// to the export directory. Returns the paths written.
func ExportPEM(certName string) (certPath, keyPath, caPath string, err error) {
	s := get()
	entry, ok := s.certificates[certName]
	if !ok {
		return "", "", "", fmt.Errorf("pki: certificate %q not found", certName)
	}

	if mkErr := os.MkdirAll(exportDir, exportDirPerm); mkErr != nil {
		return "", "", "", fmt.Errorf("pki: mkdir %s: %w", exportDir, mkErr)
	}

	safe := safeName(certName)
	certPath = filepath.Join(exportDir, "cert-"+safe+".pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: entry.Raw})
	for _, inter := range entry.RawIntermediates {
		certPEM = append(certPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: inter})...)
	}
	if wErr := os.WriteFile(certPath, certPEM, pemFilePerm); wErr != nil {
		return "", "", "", fmt.Errorf("pki: write cert: %w", wErr)
	}

	if entry.PrivateKey != nil {
		keyPath = filepath.Join(exportDir, "key-"+safe+".pem")
		keyDER, mErr := x509.MarshalPKCS8PrivateKey(entry.PrivateKey)
		if mErr != nil {
			removeWritten(certPath)
			return "", "", "", fmt.Errorf("pki: marshal key: %w", mErr)
		}
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
		if wErr := os.WriteFile(keyPath, keyPEM, pemFilePerm); wErr != nil {
			removeWritten(certPath)
			return "", "", "", fmt.Errorf("pki: write key: %w", wErr)
		}
	}

	if caName, ca := findIssuerCA(entry.Certificate, s.caCerts); ca != nil {
		caPath = filepath.Join(exportDir, "ca-"+safeName(caName)+".pem")
		caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})
		if wErr := os.WriteFile(caPath, caPEM, pemFilePerm); wErr != nil {
			removeWritten(certPath, keyPath)
			return "", "", "", fmt.Errorf("pki: write ca: %w", wErr)
		}
	}

	return certPath, keyPath, caPath, nil
}

func removeWritten(paths ...string) {
	for _, p := range paths {
		if p != "" {
			os.Remove(p) //nolint:errcheck // best-effort cleanup on export failure
		}
	}
}

// CleanupPEM removes exported PEM files for the named certificate and
// the CA file that was exported for it. Safe to call after Load(nil):
// scans the export directory for any ca-*.pem if the store no longer
// holds the certificate (cannot determine issuer).
func CleanupPEM(certName string) error {
	safe := safeName(certName)
	var errs []error

	if err := os.Remove(filepath.Join(exportDir, "cert-"+safe+".pem")); err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, err)
	}
	if err := os.Remove(filepath.Join(exportDir, "key-"+safe+".pem")); err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, err)
	}

	s := get()
	if entry, ok := s.certificates[certName]; ok {
		if caName, _ := findIssuerCA(entry.Certificate, s.caCerts); caName != "" {
			if err := os.Remove(filepath.Join(exportDir, "ca-"+safeName(caName)+".pem")); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = append(errs, err)
			}
			return errors.Join(errs...)
		}
	}

	entries, err := os.ReadDir(exportDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.Join(errs...)
		}
		return errors.Join(append(errs, err)...)
	}
	for _, e := range entries {
		if matched, mErr := filepath.Match("ca-*.pem", e.Name()); mErr == nil && matched {
			if rErr := os.Remove(filepath.Join(exportDir, e.Name())); rErr != nil && !errors.Is(rErr, os.ErrNotExist) {
				errs = append(errs, rErr)
			}
		}
	}

	return errors.Join(errs...)
}
