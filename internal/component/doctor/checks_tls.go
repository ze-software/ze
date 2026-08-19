// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Related: doctor.go — readiness check runner and output contract
// Related: checks_config.go — other config-driven validity checks

// Certificate material checks: TLS cert/key pairs referenced from config,
// web TLS material in blob storage, embedded PKI certificates, and the SSH
// host key/certificate files (presence, parse, validity window, expiry).

package doctor

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

func checkTLS(tree *config.Tree, configDir string) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic

	if mcpCfg, ok := config.ExtractMCPConfig(tree); ok {
		diags = append(diags, checkCertPair("mcp", mcpCfg.TLS.Cert, mcpCfg.TLS.Key, configDir)...)
	}

	if apiCfg, ok := config.ExtractAPIConfig(tree); ok {
		diags = append(diags, checkCertPair("api-grpc", apiCfg.GRPCTLSCert, apiCfg.GRPCTLSKey, configDir)...)
	}

	return diags
}

func checkWebTLS(tree *config.Tree, store storage.Storage) []diagnostic.Diagnostic {
	if _, ok := config.ExtractWebConfig(tree); !ok {
		return nil
	}

	certData, certErr := store.ReadFile(zefs.KeyWebCert.Pattern)
	keyExists := store.Exists(zefs.KeyWebKey.Pattern)

	if certErr != nil && !keyExists {
		return nil
	}

	var diags []diagnostic.Diagnostic

	if certErr == nil && len(certData) > 0 {
		diags = append(diags, checkCertExpiry("web", zefs.KeyWebCert.Pattern, certData)...)
	}

	if certErr == nil && !keyExists {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-tls-missing",
			Severity: diagnostic.SeverityError,
			Message:  "web: certificate present in storage but key missing",
		})
	}

	if certErr != nil && keyExists {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-tls-missing",
			Severity: diagnostic.SeverityError,
			Message:  "web: key present in storage but certificate missing",
		})
	}

	if certErr == nil && keyExists {
		diags = append(diags, checkWebTLSPair(certData, store)...)
	}

	return diags
}

// checkWebTLSPair reports whether the stored web certificate and key load as a
// TLS pair. The caller has read the certificate and has seen the key exist, so
// the only three states left are the three this answers: the key reads and the
// pair loads, the key reads and the pair does not load, the key does not read.
// A read failure here is never a missing key, and MUST NOT be reported as one.
//
// The message names the outcome and never the material. Both files are private
// key storage, ze doctor prints this line, and a support bundle keeps it.
func checkWebTLSPair(certData []byte, store storage.Storage) []diagnostic.Diagnostic {
	var tb textbuf.Buffer

	keyData, keyErr := store.ReadFile(zefs.KeyWebKey.Pattern)
	if keyErr != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-tls-invalid",
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("web: key present in storage but cannot be read: ").Err(keyErr).String(),
			Path:     zefs.KeyWebKey.Pattern,
		}}
	}

	// The error is dropped, not passed through: tls.X509KeyPair reports the PEM
	// block types it skipped, and those come from the key file.
	if _, err := tls.X509KeyPair(certData, keyData); err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-tls-invalid",
			Severity: diagnostic.SeverityError,
			Message:  "web: certificate and key in storage are not a usable pair",
			Path:     zefs.KeyWebCert.Pattern,
			Expected: "the stored certificate and key load as a TLS pair",
			Actual:   "the stored pair does not load",
		}}
	}

	return nil
}

func checkPKICerts(tree *config.Tree) []diagnostic.Diagnostic {
	pki := tree.GetContainer("pki")
	if pki == nil {
		return nil
	}

	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for _, ca := range pki.GetListOrdered("ca") {
		path := tb.Reset().Str("pki/ca/").Str(ca.Key).Str("/certificate").String()
		certData, ok := ca.Value.Get("certificate")
		if !ok || certData == "" {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-pki-cert",
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("PKI CA ").Str(ca.Key).Str(": certificate missing").String(),
				Path:     path,
			})
			continue
		}
		diags = append(diags, checkBase64DERCert(tb.Reset().Str("PKI CA ").Str(ca.Key).String(), path, certData)...)
	}

	for _, cert := range pki.GetListOrdered("certificate") {
		path := tb.Reset().Str("pki/certificate/").Str(cert.Key).Str("/certificate").String()
		certData, ok := cert.Value.Get("certificate")
		if !ok || certData == "" {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-pki-cert",
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("PKI certificate ").Str(cert.Key).Str(": certificate missing").String(),
				Path:     path,
			})
			continue
		}
		diags = append(diags, checkBase64DERCert(tb.Reset().Str("PKI certificate ").Str(cert.Key).String(), path, certData)...)
	}

	return diags
}

func checkBase64DERCert(service, path, value string) []diagnostic.Diagnostic {
	var tb textbuf.Buffer
	der, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-pki-cert",
			Severity: diagnostic.SeverityError,
			Message:  tb.Str(service).Str(": certificate is not base64 DER: ").Err(err).String(),
			Path:     path,
		}}
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-pki-cert",
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str(service).Str(": cannot parse certificate: ").Err(err).String(),
			Path:     path,
		}}
	}

	now := time.Now()
	notAfter := cert.NotAfter.Format(time.RFC3339)
	if now.After(cert.NotAfter) {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-pki-cert",
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str(service).Str(": certificate expired on ").Str(notAfter).String(),
			Path:     path,
			Expected: "not-after > now",
			Actual:   notAfter,
		}}
	}
	if now.Before(cert.NotBefore) {
		notBefore := cert.NotBefore.Format(time.RFC3339)
		return []diagnostic.Diagnostic{{
			Code:     "doctor-pki-cert",
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str(service).Str(": certificate not yet valid (starts ").Str(notBefore).Byte(')').String(),
			Path:     path,
			Expected: "not-before < now",
			Actual:   notBefore,
		}}
	}

	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
	if daysLeft < 30 {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-pki-cert",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Reset().Str(service).Str(": certificate expires in ").Int(int64(daysLeft)).Str(" days (").Str(notAfter).Byte(')').String(),
			Path:     path,
		}}
	}
	return nil
}

func checkCertPair(service, certPath, keyPath, configDir string) []diagnostic.Diagnostic {
	if certPath == "" && keyPath == "" {
		return nil
	}

	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer

	if certPath != "" {
		resolved := resolvePath(certPath, configDir)
		data, err := os.ReadFile(resolved) //nolint:gosec // cert path from parsed config
		if err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-tls-missing",
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str(service).Str(": certificate not found: ").Str(resolved).String(),
				Path:     resolved,
			})
		} else {
			diags = append(diags, checkCertExpiry(service, resolved, data)...)
		}
	}

	if keyPath != "" {
		resolved := resolvePath(keyPath, configDir)
		if _, err := os.Stat(resolved); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-tls-missing",
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str(service).Str(": key not found: ").Str(resolved).String(),
				Path:     resolved,
			})
		}
	}

	return diags
}

func checkCertExpiry(service, path string, pemData []byte) []diagnostic.Diagnostic {
	var tb textbuf.Buffer
	block, _ := pem.Decode(pemData)
	if block == nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-tls-invalid",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str(service).Str(": ").Str(path).Str(": not valid PEM").String(),
			Path:     path,
		}}
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-tls-invalid",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Reset().Str(service).Str(": ").Str(path).Str(": cannot parse certificate: ").Err(err).String(),
			Path:     path,
		}}
	}
	now := time.Now()
	ts := cert.NotAfter.Format(time.RFC3339)
	if now.After(cert.NotAfter) {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-tls-expired",
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str(service).Str(": certificate expired on ").Str(ts).String(),
			Path:     path,
			Expected: "not-after > now",
			Actual:   ts,
		}}
	}
	if now.Before(cert.NotBefore) {
		notBefore := cert.NotBefore.Format(time.RFC3339)
		return []diagnostic.Diagnostic{{
			Code:     "doctor-tls-expired",
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str(service).Str(": certificate not yet valid (starts ").Str(notBefore).Byte(')').String(),
			Path:     path,
			Expected: "not-before < now",
			Actual:   notBefore,
		}}
	}

	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
	if daysLeft < 30 {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-tls-expired",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Reset().Str(service).Str(": certificate expires in ").Int(int64(daysLeft)).Str(" days (").Str(ts).Byte(')').String(),
			Path:     path,
		}}
	}
	return nil
}

func checkSSHHostKey(tree *config.Tree, configDir string) []diagnostic.Diagnostic {
	if configDir == "" {
		return nil
	}

	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return nil
	}
	sshBlock := envBlock.GetContainer("ssh")
	if sshBlock == nil {
		return nil
	}
	enabled, _ := sshBlock.Get("enabled")
	if enabled != configTrueValue {
		return nil
	}

	var diags []diagnostic.Diagnostic

	keyPath := ""
	if v, ok := sshBlock.Get("host-key"); ok && v != "" {
		keyPath = resolvePath(v, configDir)
	}
	if keyPath == "" {
		keyPath = filepath.Join(configDir, "ssh_host_ed25519_key")
	}
	var tb textbuf.Buffer
	if _, err := os.Stat(keyPath); err != nil {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-ssh-hostkey-missing",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("SSH host key not found: ").Str(keyPath).Str(" (will be auto-generated on first start)").String(),
			Path:     keyPath,
		})
	}

	if certPath, ok := sshBlock.Get("host-certificate"); ok && certPath != "" {
		resolved := resolvePath(certPath, configDir)
		if _, err := os.Stat(resolved); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-ssh-hostkey-missing",
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("SSH host certificate not found: ").Str(resolved).String(),
				Path:     resolved,
			})
		}
	}

	return diags
}
func resolvePath(p, configDir string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if configDir != "" {
		return filepath.Join(configDir, p)
	}
	return p
}
