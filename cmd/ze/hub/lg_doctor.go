// Design: docs/architecture/pki/tls-listeners.md -- each looking-glass entry chooses HTTPS or HTTP, one rule for start, reload and doctor
//
// Always on, although the looking glass itself is tagged (ze_lg): the reload
// path (listener_migrate.go) logs under these codes in every build, and the
// untagged tests judge the report. The doctor check that declares the codes
// registers only under ze_lg (register_lg_doctor.go), so `ze doctor` never
// names a looking glass the binary does not carry.
//
// Related: lg_transport.go -- lgListenerConfigFrom and planLGTransports, the rule this reports
// Related: register_lg_doctor.go -- the doctor check and the code explanations

package hub

import (
	"errors"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The doctor codes of the looking-glass listeners. Each names one answer
// lgListenerConfigFrom or planLGTransports can give that an operator has to
// act on.
const (
	// codeDoctorLGListenUnreadable: ze.looking-glass.listen does not parse.
	codeDoctorLGListenUnreadable = "doctor-lg-listen-unreadable"
	// codeDoctorLGTLSUnservable: an entry says `tls true` and there is no
	// certificate to present, so ze refuses to start or reload.
	codeDoctorLGTLSUnservable = "doctor-lg-tls-unservable"
	// codeDoctorLGPlainHTTP: an entry left tls unset and serves plain HTTP
	// for want of a certificate.
	codeDoctorLGPlainHTTP = "doctor-lg-plain-http"
)

// lgListenerDiagnostics reports what the looking-glass listeners of tree
// serve on a daemon that has a database store when hasStore is set. It asks
// the functions the start path and the reload path ask (lgListenerConfigFrom,
// planLGTransports), so `ze doctor` cannot disagree with what ze does with the
// same config. A config that starts no looking glass reports nothing.
func lgListenerDiagnostics(tree *zeconfig.Tree, hasStore bool) []diagnostic.Diagnostic {
	cfg, err := lgListenerConfigFrom(tree)
	if err != nil {
		return []diagnostic.Diagnostic{lgListenerDiagnostic(codeDoctorLGListenUnreadable, diagnostic.SeverityError, err)}
	}
	plan, err := planLGTransports(cfg.listeners, cfg.certName, hasStore)
	if err != nil {
		return []diagnostic.Diagnostic{lgListenerDiagnostic(codeDoctorLGTLSUnservable, diagnostic.SeverityError, err)}
	}
	diags := make([]diagnostic.Diagnostic, 0, len(plan.downgraded))
	for _, addr := range plan.downgraded {
		diags = append(diags, lgListenerDiagnostic(codeDoctorLGPlainHTTP, diagnostic.SeverityWarning, lgDowngradeWarning(addr)))
	}
	return diags
}

// lgListenerDiagnostic lowers one operator error of the looking-glass
// listeners to a doctor finding: the problem and why as the message, the fix
// on its own line, and the cause, when there is one, as what was found.
//
// Every error lgListenerConfigFrom and planLGTransports answer is an operator
// error (lgListenUnreadable, lgTLSNeedsCertificate), so any other error is a
// Ze defect.
func lgListenerDiagnostic(code string, severity diagnostic.Severity, err error) diagnostic.Diagnostic {
	failure, isOperatorError := errors.AsType[*opererr.Error](err)
	if !isOperatorError {
		panic("BUG: hub: a looking-glass listener refusal is not an operator error: " + err.Error())
	}
	var message textbuf.Buffer
	diag := diagnostic.Diagnostic{
		Code:     code,
		Severity: severity,
		Message:  message.Str(failure.Problem()).Byte(' ').Str(failure.Why()).String(),
		Fix:      failure.Fix(),
	}
	if cause := failure.Unwrap(); cause != nil {
		diag.Actual = cause.Error()
	}
	return diag
}
