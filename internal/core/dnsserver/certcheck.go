// Design: docs/architecture/dns/secure-transports.md -- shared DoT/DoH certificate
// validity check, reused by the as112 and geodns doctor checks so both report
// missing / malformed / expired cert material identically
// (ai/rules/repo-maintenance.md: "New service with TLS -> Certificate validity +
// expiry check"). Mirrors internal/core/diagnostic/doctor_cert.go semantics.

package dnsserver

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// certExpiryWarnWindow is how far ahead of NotAfter a certificate is flagged as
// a warning (mirrors the 30-day window of diagnostic.DoctorCertExpiry).
const certExpiryWarnWindow = 30 * 24 * time.Hour

// The two severities a CertProblem carries. The codes it carries are
// declared by internal/core/diagnostic, which `ze explain <code>` reads.
const (
	severityError   = "error"
	severityWarning = "warning"
)

// CertProblem is a single doctor finding about the DoT/DoH certificate material.
// Code is one of the registered doctor-tls-* codes; Severity is "error" or
// "warning".
type CertProblem struct {
	Code     string
	Severity string
	Message  string
}

// CheckCertMaterial validates the operator-supplied DoT/DoH certificate. An
// empty pair is the self-signed fallback and is not a problem. now is injected
// so callers/tests control the expiry evaluation point.
func CheckCertMaterial(certFile, keyFile string, now time.Time) []CertProblem {
	if certFile == "" && keyFile == "" {
		return nil // self-signed fallback: nothing operator-supplied to validate
	}
	if certFile == "" || keyFile == "" {
		return []CertProblem{{
			Code:     diagnostic.CodeDoctorTLSInvalid,
			Severity: severityError,
			Message:  "tls needs both cert-file and key-file, only one is set",
		}}
	}

	certPEM, err := os.ReadFile(certFile) //nolint:gosec // cert path from parsed operator config
	if err != nil {
		var tb textbuf.Buffer
		return []CertProblem{{Code: diagnostic.CodeDoctorTLSMissing, Severity: severityError,
			Message: tb.Str("cannot read cert-file ").Str(certFile).Str(": ").Err(err).String()}}
	}
	if _, err := os.Stat(keyFile); err != nil {
		var tb textbuf.Buffer
		return []CertProblem{{Code: diagnostic.CodeDoctorTLSMissing, Severity: severityError,
			Message: tb.Str("cannot read key-file ").Str(keyFile).Str(": ").Err(err).String()}}
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		var tb textbuf.Buffer
		return []CertProblem{{Code: diagnostic.CodeDoctorTLSInvalid, Severity: severityError,
			Message: tb.Str("cert-file ").Str(certFile).Str(" is not valid PEM").String()}}
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		var tb textbuf.Buffer
		return []CertProblem{{Code: diagnostic.CodeDoctorTLSInvalid, Severity: severityError,
			Message: tb.Str("cert-file ").Str(certFile).Str(": ").Err(err).String()}}
	}

	if now.After(cert.NotAfter) || now.Before(cert.NotBefore) {
		var tb textbuf.Buffer
		return []CertProblem{{Code: diagnostic.CodeDoctorTLSExpired, Severity: severityError,
			Message: tb.Str("certificate ").Str(certFile).Str(" is outside its validity window (not-before ").
				Str(cert.NotBefore.Format(time.RFC3339)).Str(", not-after ").Str(cert.NotAfter.Format(time.RFC3339)).Byte(')').String()}}
	}
	if cert.NotAfter.Sub(now) < certExpiryWarnWindow {
		daysLeft := int(cert.NotAfter.Sub(now).Hours() / 24)
		var tb textbuf.Buffer
		return []CertProblem{{Code: diagnostic.CodeDoctorTLSExpired, Severity: severityWarning,
			Message: tb.Str("certificate ").Str(certFile).Str(" expires in ").Int(int64(daysLeft)).Str(" day(s)").String()}}
	}
	return nil
}
