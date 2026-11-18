// Design: docs/architecture/doctor-and-health-checks.md -- live plugin replay health.
// Related: rr.go -- optional replay warning; replay_retry.go -- current obligations.
package rr

import (
	"context"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const replayDoctorName = "bgp-rr-replay"
const replayDoctorCode = "doctor-bgp-rr-replay"
const replayDoctorFix = "ze show doctor"

func init() {
	err := diagnostic.Register(diagnostic.CodeMeta{Code: replayDoctorCode, Title: "Reflector replay needs attention",
		Description: "The reflector could not read optional received routes, or a required original replay/output remains unsettled. Mandatory FlowSpec replay is independent of an optional page read. The runtime check names the current scope and cause.", Examples: []string{replayDoctorFix, "ze show rr status"}})
	if err != nil {
		panic("BUG: reflector replay diagnostic registration: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: replayDoctorCode}); err != nil {
		panic("BUG: reflector replay event registration: " + err.Error())
	}
}

func replayDoctorDeclaration() rpc.DoctorCheckDecl {
	return rpc.DoctorCheckDecl{Name: replayDoctorName, Phase: rpc.DoctorPhaseRuntime, Codes: []string{replayDoctorCode}}
}

func (rr *routeReflector) replayDoctorCheck(name string) ([]rpc.DoctorCheckDiagnostic, error) {
	if name != replayDoctorName {
		return nil, opererr.New("The reflector has no doctor check named "+name, "Its current replay state cannot be read through that name.", replayDoctorFix)
	}
	var rows []rpc.DoctorCheckDiagnostic
	rr.mu.RLock()
	for source, state := range rr.peers {
		scope := "Reflector peer " + source.address.String() + " owner " + strconv.FormatUint(source.owner, 10) + " connection " + strconv.FormatUint(source.connection, 10) + ": "
		if state.optionalReplayErr != nil {
			rows = append(rows, rpc.DoctorCheckDiagnostic{Code: replayDoctorCode, Severity: "warning", Message: scope + state.optionalReplayErr.Error(), Fix: replayDoctorFix})
		}
		causes := []error{state.sourceOutputDebt, state.targetOutputDebt}
		if state.replayDebt != nil {
			causes = append(causes, state.replayDebt.cause)
		}
		for _, cause := range causes {
			if cause != nil {
				rows = append(rows, rpc.DoctorCheckDiagnostic{Code: replayDoctorCode, Severity: "error", Message: scope + cause.Error(), Fix: replayDoctorFix})
			}
		}
	}
	rr.mu.RUnlock()
	rr.workMu.Lock()
	workErr := rr.workErr
	rr.workMu.Unlock()
	if workErr != nil {
		rows = append(rows, rpc.DoctorCheckDiagnostic{Code: replayDoctorCode, Severity: "error", Message: workErr.Error(), Fix: replayDoctorFix})
	}
	if q := rr.validation; q != nil {
		q.mu.Lock()
		failed := q.failed
		q.mu.Unlock()
		if failed != nil {
			rows = append(rows, rpc.DoctorCheckDiagnostic{Code: replayDoctorCode, Severity: "error", Message: failed.Error(), Fix: replayDoctorFix})
		}
	}
	return rows, nil
}

// reportOptionalReplay performs no I/O under a role lock. Its warning is
// scoped by the exact target tuple through the same immutable identity label;
// the event bus retains the bounded transient event. A report refusal is also logged
// under the declared code and never changes required replay admission.
func (rr *routeReflector) reportOptionalReplay(parent context.Context, target peerTarget, cause error) {
	rr.mu.Lock()
	state := rr.peers[target.identity]
	if state == nil {
		rr.mu.Unlock()
		return
	}
	if !state.Up {
		rr.mu.Unlock()
		return
	}
	if state.DestinationSession != target.session {
		rr.mu.Unlock()
		return
	}
	if state.ReplayGen != target.generation {
		rr.mu.Unlock()
		return
	}
	state.optionalReplayErr = cause
	rr.mu.Unlock()
	if cause == nil {
		return
	}
	subject := target.identity.address.String() + " owner " + strconv.FormatUint(target.identity.owner, 10) + " connection " + strconv.FormatUint(target.identity.connection, 10) + " session " + strconv.FormatUint(target.session, 10)
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	message := "Optional received-route replay is unavailable; mandatory FlowSpec replay continues: " + cause.Error()
	logger().Warn("Optional received-route replay is unavailable; mandatory FlowSpec replay continues", "peer", subject, "error", cause, "code", replayDoctorCode, "fix", replayDoctorFix)
	failed := rr.plugin.RaiseIssue(ctx, sdk.Issue{Severity: rpc.ReportError, Code: replayDoctorCode, Subject: subject, Message: report.FitMessage(message), Fix: replayDoctorFix})
	if failed != nil {
		logger().Warn("Reflector replay warning was not acknowledged by the daemon", "peer", subject, "error", failed, "code", replayDoctorCode, "fix", replayDoctorFix)
	}
}
