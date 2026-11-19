// Design: docs/architecture/l2tp/subscriber-session-model.md -- accounting answers a session-down without waiting for its server
// Related: acct.go -- sendAcctPacket, the only sender of accounting records
// Related: coa_health.go -- the other runtime check this provider declares

package l2tpauthradius

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const acctHealthCheckName = "l2tp-radius-accounting"
const acctHealthFix = "show diagnostic " + diagnostic.CodeDoctorRADIUSAccountingUnacknowledged

func acctHealthDeclaration() rpc.DoctorCheckDecl {
	return rpc.DoctorCheckDecl{
		Name: acctHealthCheckName, Phase: rpc.DoctorPhaseRuntime, Order: 712,
		Codes: []string{diagnostic.CodeDoctorRADIUSAccountingUnacknowledged},
	}
}

// acctHealthSubject names one accounting record stream. The Acct-Session-Id
// is what the operator's billing system holds, and it is unique per session,
// so two sessions sharing reused wire labels never share a finding.
func acctHealthSubject(sess *acctSession) string {
	var subject textbuf.Buffer
	return subject.Str("l2tp:").Uint16(sess.tunnelID).Byte('-').Uint16(sess.sessionID).
		Byte(' ').Str(sess.acctSessID).String()
}

// recordAccountingOutcome keeps the session's finding current: a record no
// server acknowledged raises it, and a later acknowledged record of the same
// session clears it. A Stop no server acknowledged stays raised, because no
// later record of that session can settle it.
func recordAccountingOutcome(sess *acctSession, purpose string, err error) {
	subject := acctHealthSubject(sess)
	if err == nil {
		report.ClearWarning(Name, diagnostic.CodeDoctorRADIUSAccountingUnacknowledged, subject)
		return
	}
	report.RaiseWarningFix(Name, diagnostic.CodeDoctorRADIUSAccountingUnacknowledged, subject,
		"No RADIUS accounting server acknowledged this session's "+purpose+
			" record; the billing system may lack it. Restore the accounting server's reachability",
		acctHealthFix, map[string]any{"error": err.Error()})
}

func checkAccountingHealth() []rpc.DoctorCheckDiagnostic {
	return retainedFindings(diagnostic.CodeDoctorRADIUSAccountingUnacknowledged)
}

// retainedFindings answers this provider's retained warnings under one doctor
// code as findings, each with its subject, message, cause and fix. With none
// retained, an evicted warning still answers as unknown, never as recovered.
func retainedFindings(code string) []rpc.DoctorCheckDiagnostic {
	var findings []rpc.DoctorCheckDiagnostic
	for _, issue := range report.Warnings() {
		if issue.Source != Name {
			continue
		}
		if issue.Code != code {
			continue
		}
		message := issue.Subject + ": " + issue.Message
		if cause, found := issue.Detail["error"].(string); found {
			message += ": " + cause
		}
		findings = append(findings, rpc.DoctorCheckDiagnostic{
			Code: issue.Code, Severity: string(diagnostic.SeverityWarning), Message: message, Fix: issue.Fix,
		})
	}
	if len(findings) != 0 {
		return findings
	}
	status, reason := report.HealthProbeDegraded(code)()
	if status != health.StatusHealthy {
		return []rpc.DoctorCheckDiagnostic{{Code: code,
			Severity: string(diagnostic.SeverityWarning), Message: reason, Fix: "show warnings"}}
	}
	return findings
}

// runtimeChecks maps each runtime doctor check this provider declares to its
// answer. Each check declares itself here beside its own declaration, so the
// callback names no check of its own.
var runtimeChecks = map[string]func() []rpc.DoctorCheckDiagnostic{
	coaHealthCheckName:  checkCoAHealth,
	acctHealthCheckName: checkAccountingHealth,
}

// runtimeCheckDeclarations answers the declarations of the runtime checks
// runtimeChecks serves, in a fixed order.
func runtimeCheckDeclarations() []rpc.DoctorCheckDecl {
	return []rpc.DoctorCheckDecl{coaHealthDeclaration(), acctHealthDeclaration()}
}

// checkRuntimeHealth answers the provider's runtime doctor callback.
func checkRuntimeHealth(name string) ([]rpc.DoctorCheckDiagnostic, error) {
	check, found := runtimeChecks[name]
	if !found {
		return nil, opererr.New("Unknown RADIUS runtime check "+name,
			"The provider declares only its dynamic authorization and accounting checks", "show doctor")
	}
	return check(), nil
}
