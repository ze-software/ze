// Design: docs/guide/l2tp.md -- dynamic authorization backend failures
// Related: coa_transaction.go -- failed application and successful retry

package l2tpauthradius

import (
	"github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

const coaHealthCheckName = "l2tp-radius-coa-apply"
const coaHealthFix = "show l2tp shaper"

func coaHealthDeclaration() rpc.DoctorCheckDecl {
	return rpc.DoctorCheckDecl{
		Name: coaHealthCheckName, Phase: rpc.DoctorPhaseRuntime, Order: 711,
		Codes: []string{diagnostic.CodeDoctorRADIUSCoAApply},
	}
}

// coaHealthSubject names one session independently of the lookup route. It is
// the same value a session-down event can produce without retaining a second
// map of failures. The report bus owns the bounded warning population.
func coaHealthSubject(session *subscriber.Session) string {
	if session.AccessType == subscriber.AccessPPPoE {
		return "pppoe:" + session.ID
	}
	var subject textbuf.Buffer
	return subject.Str("l2tp:").Uint16(session.TunnelID).Byte('-').Uint16(session.SessionID).String()
}

func recordCoAHealth(session *subscriber.Session, err error) {
	subject := coaHealthSubject(session)
	if err == nil {
		report.ClearWarning(Name, diagnostic.CodeDoctorRADIUSCoAApply, subject)
		return
	}
	report.RaiseWarningFix(Name, diagnostic.CodeDoctorRADIUSCoAApply, subject,
		"Subscriber authorization was not confirmed; inspect shaping and repair the backend before retrying the request",
		coaHealthFix, map[string]any{"error": err.Error()})
}

func checkCoAHealth(name string) ([]rpc.DoctorCheckDiagnostic, error) {
	if name != coaHealthCheckName {
		return nil, opererr.New("Unknown RADIUS runtime check "+name,
			"The provider only declares its dynamic authorization backend check", "show doctor")
	}
	var findings []rpc.DoctorCheckDiagnostic
	for _, issue := range report.Warnings() {
		if issue.Source != Name {
			continue
		}
		if issue.Code != diagnostic.CodeDoctorRADIUSCoAApply {
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
		return findings, nil
	}
	// An evicted warning is unknown, never evidence that the backend recovered.
	status, reason := report.HealthProbeDegraded(diagnostic.CodeDoctorRADIUSCoAApply)()
	if status != health.StatusHealthy {
		return []rpc.DoctorCheckDiagnostic{{Code: diagnostic.CodeDoctorRADIUSCoAApply,
			Severity: string(diagnostic.SeverityWarning), Message: reason, Fix: "show warnings"}}, nil
	}
	return findings, nil
}

func subscribeCoAHealth(bus ze.EventBus) func() {
	if bus == nil {
		return func() {}
	}
	unsubL2TP := events.SessionDown.Subscribe(bus, func(event *events.SessionDownPayload) {
		recordCoAHealth(&subscriber.Session{TunnelID: event.TunnelID, SessionID: event.SessionID}, nil)
	})
	unsubSubscriber := subevents.SessionDown.Subscribe(bus, func(event *subevents.SessionDownPayload) {
		recordCoAHealth(&event.Session, nil)
	})
	return func() {
		unsubL2TP()
		unsubSubscriber()
	}
}
