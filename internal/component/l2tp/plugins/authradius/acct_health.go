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
const acctHealthFix = "show diagnostic " + codeRADIUSAccountingUnacknowledged
const acctCountersFix = "show diagnostic " + codeRADIUSAccountingCounters

func acctHealthDeclaration() rpc.DoctorCheckDecl {
	return rpc.DoctorCheckDecl{
		Name: acctHealthCheckName, Phase: rpc.DoctorPhaseRuntime, Order: 712,
		Codes: []string{codeRADIUSAccountingUnacknowledged, codeRADIUSAccountingCounters},
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
		report.ClearWarning(Name, codeRADIUSAccountingUnacknowledged, subject)
		return
	}
	report.RaiseWarningFix(Name, codeRADIUSAccountingUnacknowledged, subject,
		"No RADIUS accounting server acknowledged this session's "+purpose+
			" record; the billing system may lack it. Restore the accounting server's reachability",
		acctHealthFix, map[string]any{findingDetailError: err.Error()})
}

// recordCountersOmitted logs why one of the session's accounting records
// leaves without its traffic counters, and raises the session's finding, from
// one place so the log line and the finding cannot disagree. The record is
// still sent: its six counter attributes are omitted, never reported as zero,
// so the billing record is incomplete and never wrong. err is nil when the
// cause carries no error of its own.
func recordCountersOmitted(sess *acctSession, why string, err error) {
	subject := acctHealthSubject(sess)
	message := "A RADIUS accounting record of this session was sent without traffic counters: " +
		why + ". The billing system lacks its octets and packets until a later record of the " +
		"session carries them. Read the cause, then inspect the session's PPP unit"
	if err == nil {
		logger().Warn("l2tp-auth-radius: "+why+"; traffic counters omitted",
			"code", codeRADIUSAccountingCounters,
			"tunnel", sess.tunnelID, "session", sess.sessionID, "acct-session-id", sess.acctSessID)
		report.RaiseWarningFix(Name, codeRADIUSAccountingCounters, subject, message, acctCountersFix)
		return
	}
	logger().Warn("l2tp-auth-radius: "+why+"; traffic counters omitted",
		"code", codeRADIUSAccountingCounters,
		"tunnel", sess.tunnelID, "session", sess.sessionID, "acct-session-id", sess.acctSessID,
		"error", err)
	report.RaiseWarningFix(Name, codeRADIUSAccountingCounters, subject, message, acctCountersFix,
		map[string]any{findingDetailError: err.Error()})
}

// clearCountersOmitted settles the session's counter finding once a record
// carries counters again. RFC 2869 Section 2.1: "Note that all information in
// an interim message is cumulative", and the Stop's counters are as well, so
// that record makes up for every record before it that omitted them.
func clearCountersOmitted(sess *acctSession) {
	report.ClearWarning(Name, codeRADIUSAccountingCounters, acctHealthSubject(sess))
}

// checkAccountingHealth answers the accounting check's findings under each of
// its codes. Each code is judged on its own, so an evicted warning of one code
// still answers unknown while the other code holds findings.
func checkAccountingHealth() []rpc.DoctorCheckDiagnostic {
	findings := retainedFindings(codeRADIUSAccountingUnacknowledged)
	return append(findings, retainedFindings(codeRADIUSAccountingCounters)...)
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
		if cause, found := issue.Detail[findingDetailError].(string); found {
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
