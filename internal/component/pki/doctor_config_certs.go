// Design: docs/architecture/pki/pki-store.md -- the certificates an operator declares in config
// Overview: doctor.go -- the sibling check over the generated local CA root
// Related: register.go -- registers this check via diagnostic.RegisterDoctorCheck
//
// The certificate material an operator pastes into the `pki` config block: one
// base64 DER string for each CA and each certificate. Every Ze listener that
// names one of these entries fails to serve TLS when the string does not
// decode, does not parse, or names a validity window that has closed, and the
// failure otherwise waits for the first connection.
//
// The pki component owns the block, so it owns the check that reads it
// (ai/rules/repo-maintenance.md, ownership). It moved here from
// internal/component/doctor, which called it by hand.
package pki

import (
	"crypto/x509"
	"encoding/base64"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// configCertExpiryWarnDays is how far ahead of not-after a configured
// certificate is reported. It is the window every served certificate gets;
// caRootExpiryWarnWindow in doctor.go is longer, and says why.
const configCertExpiryWarnDays = 30

// configCertDoctorCheck describes the check. register.go registers it.
//
// Order 180 reproduces the sequence the doctor runner printed before the
// check moved onto the registry: it ran before the runner's own phase
// dispatch, right after the stored web pair at 170
// (internal/component/web/doctor_material.go). Every check the runner called
// after its dispatch, the SSH host key among them, sorts above 2000.
var configCertDoctorCheck = diagnostic.DoctorCheck{
	Name:         "pki-configured-certificates",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        180,
	Component:    "pki",
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorPKICert},
	Check:        checkConfiguredCertificates,
}

// checkConfiguredCertificates reports every CA and certificate in the pki
// config block whose material is missing, undecodable, or outside its validity
// window.
func checkConfiguredCertificates(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	pki := tree.GetContainer("pki")
	if pki == nil {
		return nil
	}

	var diags []diagnostic.Diagnostic
	diags = append(diags, configCertEntries(pki, "ca", "PKI CA")...)
	return append(diags, configCertEntries(pki, "certificate", "PKI certificate")...)
}

// configCertEntries reports on one list of the pki block. list is the YANG list
// name, and label opens the message the operator reads.
func configCertEntries(pki *config.Tree, list, label string) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for _, entry := range pki.GetListOrdered(list) {
		path := tb.Reset().Str("pki/").Str(list).Byte('/').Str(entry.Key).Str("/certificate").String()
		material, ok := entry.Value.Get("certificate")
		if !ok || material == "" {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     diagnostic.CodeDoctorPKICert,
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str(label).Byte(' ').Str(entry.Key).Str(": certificate missing").String(),
				Path:     path,
			})
			continue
		}
		service := tb.Reset().Str(label).Byte(' ').Str(entry.Key).String()
		diags = append(diags, checkBase64DERCert(service, path, material)...)
	}
	return diags
}

// checkBase64DERCert reports on one base64 DER certificate string: it decodes,
// it parses, and it is inside its validity window.
//
// It is the config-block twin of diagnostic.DoctorCertExpiry, which answers the
// same three questions for PEM material read from a file. The two differ in the
// encoding they read and in the code they report, so the operator can tell a
// pasted config value from a file on disk.
func checkBase64DERCert(service, path, value string) []diagnostic.Diagnostic {
	var tb textbuf.Buffer
	der, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorPKICert,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str(service).Str(": certificate is not base64 DER: ").Err(err).String(),
			Path:     path,
		}}
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorPKICert,
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str(service).Str(": cannot parse certificate: ").Err(err).String(),
			Path:     path,
		}}
	}

	now := time.Now()
	notAfter := cert.NotAfter.Format(time.RFC3339)
	if now.After(cert.NotAfter) {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorPKICert,
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
			Code:     diagnostic.CodeDoctorPKICert,
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str(service).Str(": certificate not yet valid (starts ").Str(notBefore).Byte(')').String(),
			Path:     path,
		}}
	}

	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
	if daysLeft < configCertExpiryWarnDays {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorPKICert,
			Severity: diagnostic.SeverityWarning,
			Message: tb.Reset().Str(service).Str(": certificate expires in ").Int(int64(daysLeft)).
				Str(" days (").Str(notAfter).Byte(')').String(),
			Path: path,
		}}
	}
	return nil
}
