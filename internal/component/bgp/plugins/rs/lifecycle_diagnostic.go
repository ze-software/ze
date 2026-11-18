// Design: docs/architecture/doctor-and-health-checks.md -- current RS obligations and bounded problem reporting.
// Related: retained_settlement.go -- current actors/output; server.go -- the SDK-owned report queue.
package rs

import (
	"fmt"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const lifecycleDoctorName = "bgp-rs-lifecycle"
const lifecycleDoctorCode = "doctor-bgp-rs-lifecycle"
const lifecycleDoctorFix = "ze show bgp rs status"
const lifecycleDoctorWhy = "Some route-server input, replay or cleanup was refused; inspect the original peers and output before retrying."

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{Code: lifecycleDoctorCode,
		Title:       "Route-server work needs attention",
		Description: "The route server refused input or could not complete original replay, source release or output. Its runtime check reads current obligations in the actual plugin process; bounded transient error events retain refused input and their cause.",
		Examples:    []string{"ze show doctor", lifecycleDoctorFix}}); err != nil {
		panic("BUG: route-server lifecycle diagnostic registration: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: lifecycleDoctorCode}); err != nil {
		panic("BUG: route-server lifecycle event registration: " + err.Error())
	}
}

func lifecycleDoctorDeclaration() rpc.DoctorCheckDecl {
	return rpc.DoctorCheckDecl{Name: lifecycleDoctorName, Phase: rpc.DoctorPhaseRuntime,
		Codes: []string{lifecycleDoctorCode}}
}

// lifecycleDoctorCheck reads only the current original actors and output debt.
// It neither retries work nor clears a failure, and performs no SDK I/O while
// taking the same short locks as the status reader. Safe for concurrent use.
func (rs *routeServer) lifecycleDoctorCheck(name string) ([]rpc.DoctorCheckDiagnostic, error) {
	if name != lifecycleDoctorName {
		return nil, opererr.New("The route server has no doctor check named "+strconv.Quote(name),
			"This name cannot inspect the route server's current work.", "ze show doctor")
	}
	var rows []rpc.DoctorCheckDiagnostic
	rs.mu.RLock()
	for key, actor := range rs.peers {
		scope := fmt.Sprintf("Route-server peer %s owner %d connection %d: ", key.address, key.owner, key.connectionID)
		if actor.replayFailure != nil {
			rows = append(rows, rpc.DoctorCheckDiagnostic{Code: lifecycleDoctorCode, Severity: "error",
				Message: scope + actor.replayFailure.Error(), Fix: lifecycleDoctorFix})
		}
		if actor.sourceReleaseOwed {
			rows = append(rows, rpc.DoctorCheckDiagnostic{Code: lifecycleDoctorCode, Severity: "error",
				Message: scope + "the original source lease release has not completed", Fix: lifecycleDoctorFix})
		}
	}
	rs.mu.RUnlock()
	rs.outputMu.Lock()
	for target, cause := range rs.unknownOutputs {
		scope := fmt.Sprintf("Route-server destination %s owner %d session %d: ", target.Destination, target.DestinationOwner, target.DestinationSession)
		rows = append(rows, rpc.DoctorCheckDiagnostic{Code: lifecycleDoctorCode, Severity: "error",
			Message: scope + cause.Error(), Fix: lifecycleDoctorFix})
	}
	if rs.settlementOwed != nil {
		rows = append(rows, rpc.DoctorCheckDiagnostic{Code: lifecycleDoctorCode, Severity: "error",
			Message: "Route-server settlement remains owed: " + rs.settlementOwed.Error(), Fix: lifecycleDoctorFix})
	}
	rs.outputMu.Unlock()
	return rows, nil
}

// warnLifecycle enqueues the transient event without waiting for the daemon.
// Callers supply their declared code and fix in attrs so every problem site
// remains visible to the coverage gate. One scalar deadline bounds error-event
// publication to one per report window for this plugin; each refusal is still
// logged with its exact attributes. Queue drain runs after all RS producers stop.
// Safe under RS locks: no SDK call or route-state lock is taken here.
func (rs *routeServer) warnLifecycle(message string, attrs ...any) {
	now := time.Now()
	rs.lifecycleReportMu.Lock()
	due := !now.Before(rs.lifecycleReportAt)
	if due {
		rs.lifecycleReportAt = now.Add(rpc.ReportRaiseWindow)
	}
	rs.lifecycleReportMu.Unlock()
	if due {
		detail := report.FitMessage(message + ". " + lifecycleDoctorWhy + " " + fmt.Sprint(attrs...))
		queued := rs.issues.RaiseError(lifecycleDoctorCode, "route-server", detail, lifecycleDoctorFix,
			func(err error) {
				logger().Warn("Route-server failure did not reach the daemon's report bus",
					"code", lifecycleDoctorCode, "error", err, "fix", lifecycleDoctorFix)
			})
		attrs = append(attrs, issuequeue.Outcome(queued))
	} else {
		attrs = append(attrs, "report", "rate-limited")
	}
	attrs = append(attrs, "why", lifecycleDoctorWhy)
	logger().Warn(message, attrs...)
}
