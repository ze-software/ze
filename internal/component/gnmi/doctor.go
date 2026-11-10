// Design: docs/guide/gnmi.md -- TLS: the transport serves TLS or nothing
// Overview: register.go -- the init() that installs the registration below
// Related: tls.go -- loadTLSCredentials, the load this check runs
//
// Serve refuses to start gNMI when the TLS material the config names does not
// load, and the hub logs that refusal under doctor-gnmi-tls-material. This
// check reports the same cause before the daemon starts, from the same pair
// (zeconfig.GNMITLSPaths) and through the same load (loadTLSCredentials), so
// `ze doctor` and the refusal can never disagree about what is wrong.

package gnmi

import (
	"errors"
	"os"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// gnmiDoctorService opens the messages DoctorCertExpiry writes, so the
// operator knows which listener the certificate belongs to.
const gnmiDoctorService = "gnmi"

// gnmiTLSDoctorCheck describes the check. register.go registers it.
//
// Order 168 sorts it after the gRPC pair at 165
// (internal/component/api/grpc/doctor.go) and before the web material at 170
// (internal/component/web/doctor_material.go), beside the other listeners'
// TLS checks.
var gnmiTLSDoctorCheck = diagnostic.DoctorCheck{
	Name:         "gnmi-tls-material",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        168,
	Component:    "gnmi",
	Dependencies: []string{"filesystem"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorGNMITLSMaterial},
	Check:        checkGNMITLSMaterial,
}

// checkGNMITLSMaterial reports on the TLS pair gNMI would serve: a half pair,
// a file that cannot be read, a pair that does not load, and a certificate
// outside or near the end of its validity window. It reports nothing when
// gNMI does not start or names no TLS material, because Serve then loads
// nothing.
//
// The runner asserts the tree's type before any check runs (doctorTree,
// internal/component/doctor/registry.go), so the only other value is no tree
// at all. Both zeconfig readers accept a nil tree, where the environment alone
// can start gNMI and name its pair, so that case is judged rather than skipped.
func checkGNMITLSMaterial(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, _ := ctx.Tree.(*zeconfig.Tree)
	if !zeconfig.GNMIStarts(tree) {
		return nil
	}
	certFile, keyFile := zeconfig.GNMITLSPaths(tree)
	named := Config{CertFile: certFile, KeyFile: keyFile}
	if !named.tlsNamed() {
		return nil
	}
	if _, err := loadTLSCredentials(certFile, keyFile); err != nil {
		return []diagnostic.Diagnostic{gnmiTLSRefusalFinding(err)}
	}
	return gnmiTLSExpiryFindings(certFile)
}

// gnmiTLSRefusalFinding turns the refusal Serve would return into the finding:
// its problem as the message, its reason and the underlying error as the help,
// and its fix as the fix. loadTLSCredentials answers only operator errors, so
// the second branch is reached only by a defect there, and still reports it.
func gnmiTLSRefusalFinding(err error) diagnostic.Diagnostic {
	refusal, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		return diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorGNMITLSMaterial,
			Severity: diagnostic.SeverityError,
			Message:  "gNMI not started: its TLS material does not load: " + err.Error(),
			Fix:      "point " + tlsCertSetting + " and " + tlsKeySetting + " at a readable PEM pair" + tlsRestart,
		}
	}
	var help textbuf.Buffer
	help.Str(refusal.Why())
	if cause := errors.Unwrap(refusal); cause != nil {
		help.Str(": ").Err(cause)
	}
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorGNMITLSMaterial,
		Severity: diagnostic.SeverityError,
		Message:  refusal.Problem(),
		Help:     help.String(),
		Fix:      refusal.Fix(),
	}
}

// gnmiTLSExpiryFindings reports a loaded certificate that is expired, not yet
// valid, or near expiry. The pair parsed, so DoctorCertExpiry answers only on
// the validity window; each finding is reported under the gNMI code with the
// fix that replaces the file.
func gnmiTLSExpiryFindings(certFile string) []diagnostic.Diagnostic {
	certPEM, err := os.ReadFile(certFile) //nolint:gosec // operator-configured certificate path
	if err != nil {
		// The pair loaded a moment ago, so the file went away since: the same
		// unreadable case loadTLSCredentials refuses, reported the same way.
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorGNMITLSMaterial,
			Severity: diagnostic.SeverityError,
			Message:  "gNMI TLS certificate file " + certFile + " cannot be read",
			Help:     tlsNoClearText + ": " + err.Error(),
			Path:     certFile,
			Fix:      "restore " + certFile + " or point " + tlsCertSetting + " at a readable PEM file" + tlsRestart,
		}}
	}
	diags := diagnostic.DoctorCertExpiry(gnmiDoctorService, certFile, certPEM)
	for i := range diags {
		diags[i].Code = diagnostic.CodeDoctorGNMITLSMaterial
		diags[i].Fix = "replace " + certFile + " with a certificate inside its validity window, or point " +
			tlsCertSetting + " at one" + tlsRestart
	}
	return diags
}
