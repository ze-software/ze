// Design: docs/architecture/pki/tls-listeners.md -- doctor check over the stored web TLS pair
// Overview: register.go -- the init() that installs the registration below
// Related: doctor.go -- the sibling check over environment.web.certificate
// Related: cmd/ze/hub/cert_store.go -- the reader and writer of the pair this check judges
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. A check over the certificate and key the web listener
// serves from blob storage belongs to the component that serves them
// (ai/patterns/registration.md, "Doctor Check Registry"), so it is owned here
// now and dropping this component drops its check with it.

package web

import (
	"crypto/tls"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// webTLSMaterialDoctorCheck describes the check. register.go registers it.
//
// Order 170 reproduces the sequence the doctor runner printed before the
// check moved onto the registry: it ran before the runner's own phase
// dispatch, after the gRPC pair at 165 (internal/component/api/grpc/doctor.go)
// and before the configured PKI certificates at 180
// (internal/component/pki/doctor_config_certs.go).
var webTLSMaterialDoctorCheck = diagnostic.DoctorCheck{
	Name:         "web-tls-material",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        170,
	Component:    segWeb,
	Dependencies: []string{"storage"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes: []string{
		diagnostic.CodeDoctorTLSMissing,
		diagnostic.CodeDoctorTLSExpired,
		diagnostic.CodeDoctorTLSInvalid,
	},
	Check: checkWebTLSMaterial,
}

// checkWebTLSMaterial reports on the certificate and key stored for an enabled
// web listener: one half without the other, material outside its validity
// window, and a pair that does not load together. A store holding neither is
// the self-signed path and is not a finding.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkWebTLSMaterial(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*zeconfig.Tree)
	if !ok || tree == nil {
		return nil
	}
	if _, ok := zeconfig.ExtractWebConfig(tree); !ok {
		return nil
	}
	return webTLSMaterialDiagnostics(ctx.Store)
}

// webTLSMaterialDiagnostics is the store half of the check, so a test can hand
// it a store without a config tree.
func webTLSMaterialDiagnostics(store storage.Storage) []diagnostic.Diagnostic {
	if store == nil {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorTLSInvalid,
			Severity: diagnostic.SeverityError,
			Message:  "web: cannot read the stored TLS pair: this doctor run resolved no storage",
			Help:     "check the storage diagnostics reported above this one",
		}}
	}

	certData, certErr := store.ReadFile(zefs.KeyWebCert.Pattern)
	keyExists := store.Exists(zefs.KeyWebKey.Pattern)

	if certErr != nil && !keyExists {
		return nil
	}

	var diags []diagnostic.Diagnostic

	if certErr == nil && len(certData) > 0 {
		diags = append(diags, diagnostic.DoctorCertExpiry(segWeb, zefs.KeyWebCert.Pattern, certData)...)
	}

	if certErr == nil && !keyExists {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorTLSMissing,
			Severity: diagnostic.SeverityError,
			Message:  "web: certificate present in storage but key missing",
		})
	}

	if certErr != nil && keyExists {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorTLSMissing,
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
			Code:     diagnostic.CodeDoctorTLSInvalid,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("web: key present in storage but cannot be read: ").Err(keyErr).String(),
			Path:     zefs.KeyWebKey.Pattern,
		}}
	}

	// The error is dropped, not passed through: tls.X509KeyPair reports the PEM
	// block types it skipped, and those come from the key file.
	if _, err := tls.X509KeyPair(certData, keyData); err != nil {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorTLSInvalid,
			Severity: diagnostic.SeverityError,
			Message:  "web: certificate and key in storage are not a usable pair",
			Path:     zefs.KeyWebCert.Pattern,
			Expected: "the stored certificate and key load as a TLS pair",
			Actual:   "the stored pair does not load",
		}}
	}

	return nil
}
