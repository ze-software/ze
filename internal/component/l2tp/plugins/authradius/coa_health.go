// Design: docs/guide/l2tp.md -- dynamic authorization backend failures
// Related: coa_transaction.go -- failed application and successful retry

package l2tpauthradius

import (
	"github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/diagnostic"
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

// checkCoAHealth answers the runtime check coaHealthDeclaration declares;
// checkRuntimeHealth routes the callback to it.
func checkCoAHealth() []rpc.DoctorCheckDiagnostic {
	return retainedFindings(diagnostic.CodeDoctorRADIUSCoAApply)
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
