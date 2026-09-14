// Design: docs/features/ai-first.md -- the certificate material a registered doctor check reads
// Overview: doctor_probe.go -- the reachability probes the registered checks share
// Related: doctor_registry.go -- the registry the owner packages register with
// Related: codes.go -- the doctor-tls-* codes these functions report
//
// Five packages configure TLS and each one owes the operator the same three
// answers before the daemon starts: is the material there, does it parse, and
// is it inside its validity window. The answers live here because the
// doctor-tls-* codes are declared here, so one producer reports them and a
// check moved out of internal/component/doctor keeps the message the runner
// printed rather than growing a second copy of it.
//
// The warning window is 30 days. internal/component/pki holds its own 90-day
// window for the local certificate authority root, and says why: the root is
// replaced by hand on every peer that trusts it, where a served certificate is
// replaced on the router alone.

package diagnostic

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorCertExpiryWarnDays is how far ahead of not-after a certificate is
// reported as a warning.
const doctorCertExpiryWarnDays = 30

// DoctorConfigPath resolves a path a config leaf carries against the directory
// the config was loaded from. An absolute path is returned unchanged, and so is
// a relative one when the caller knows no config directory.
func DoctorConfigPath(path, configDir string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if configDir != "" {
		return filepath.Join(configDir, path)
	}
	return path
}

// DoctorCertPair reports on a certificate and key named by config leaves, as a
// path each. A pair where neither leaf is set is not configured and reports
// nothing. service names the surface in the message the operator reads.
func DoctorCertPair(service, certPath, keyPath, configDir string) []Diagnostic {
	if certPath == "" && keyPath == "" {
		return nil
	}

	var diags []Diagnostic
	var tb textbuf.Buffer

	if certPath != "" {
		resolved := DoctorConfigPath(certPath, configDir)
		data, err := os.ReadFile(resolved) //nolint:gosec // cert path from parsed config
		if err != nil {
			diags = append(diags, Diagnostic{
				Code:     CodeDoctorTLSMissing,
				Severity: SeverityError,
				Message:  tb.Reset().Str(service).Str(": certificate not found: ").Str(resolved).String(),
				Path:     resolved,
			})
		} else {
			diags = append(diags, DoctorCertExpiry(service, resolved, data)...)
		}
	}

	if keyPath != "" {
		resolved := DoctorConfigPath(keyPath, configDir)
		if _, err := os.Stat(resolved); err != nil {
			diags = append(diags, Diagnostic{
				Code:     CodeDoctorTLSMissing,
				Severity: SeverityError,
				Message:  tb.Reset().Str(service).Str(": key not found: ").Str(resolved).String(),
				Path:     resolved,
			})
		}
	}

	return diags
}

// DoctorCertExpiry reports whether PEM certificate material parses and is
// inside its validity window. It reports the first fault it finds: material
// that is not PEM, material that is not a certificate, a window that has
// closed, a window that has not opened, and a window that closes inside
// doctorCertExpiryWarnDays.
func DoctorCertExpiry(service, path string, pemData []byte) []Diagnostic {
	var tb textbuf.Buffer
	block, _ := pem.Decode(pemData)
	if block == nil {
		return []Diagnostic{{
			Code:     CodeDoctorTLSInvalid,
			Severity: SeverityWarning,
			Message:  tb.Str(service).Str(": ").Str(path).Str(": not valid PEM").String(),
			Path:     path,
		}}
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return []Diagnostic{{
			Code:     CodeDoctorTLSInvalid,
			Severity: SeverityWarning,
			Message:  tb.Reset().Str(service).Str(": ").Str(path).Str(": cannot parse certificate: ").Err(err).String(),
			Path:     path,
		}}
	}

	now := time.Now()
	notAfter := cert.NotAfter.Format(time.RFC3339)
	if now.After(cert.NotAfter) {
		return []Diagnostic{{
			Code:     CodeDoctorTLSExpired,
			Severity: SeverityError,
			Message:  tb.Reset().Str(service).Str(": certificate expired on ").Str(notAfter).String(),
			Path:     path,
			Expected: "not-after > now",
			Actual:   notAfter,
		}}
	}
	if now.Before(cert.NotBefore) {
		notBefore := cert.NotBefore.Format(time.RFC3339)
		return []Diagnostic{{
			Code:     CodeDoctorTLSExpired,
			Severity: SeverityError,
			Message:  tb.Reset().Str(service).Str(": certificate not yet valid (starts ").Str(notBefore).Byte(')').String(),
			Path:     path,
			Expected: "not-before < now",
			Actual:   notBefore,
		}}
	}

	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
	if daysLeft < doctorCertExpiryWarnDays {
		return []Diagnostic{{
			Code:     CodeDoctorTLSExpired,
			Severity: SeverityWarning,
			Message: tb.Reset().Str(service).Str(": certificate expires in ").Int(int64(daysLeft)).
				Str(" days (").Str(notAfter).Byte(')').String(),
			Path: path,
		}}
	}
	return nil
}
