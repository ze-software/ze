// Design: docs/architecture/l2tp/subscriber-session-model.md -- immutable birth before admission

package l2tp

import (
	"log/slog"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

const sessionBirthReportSubject = "session reference"

const sessionBirthRefusalReportSubject = "invalid session reference"

// newSessionBirth refuses admission when an immutable reference cannot be
// minted. The existing internal-defect doctor check retains the failure, so a
// refused call is observable even when its peer sees only No Resources.
func newSessionBirth(logger *slog.Logger) (subscriberid.L2TPBirth, error) {
	birth, err := subscriberid.NewL2TPBirth()
	if err == nil {
		report.ClearWarning("l2tp", report.CodeInternalDefect, sessionBirthReportSubject)
		return birth, nil
	}
	const problem = "L2TP could not allocate a session reference"
	const why = "the randomness source failed, so the new call was refused"
	const fix = "ze doctor"
	refusal := opererr.Wrap(err, problem, why, fix)
	logger.Warn(refusal.Error(), "code", report.CodeInternalDefect)
	report.RaiseWarningFix("l2tp", report.CodeInternalDefect, sessionBirthReportSubject, problem+": "+why, fix,
		map[string]any{reportFieldCause: err.Error()})
	return subscriberid.L2TPBirth{}, refusal
}

// reportSessionBirthRefusal retains a refused event separately from allocation
// failures, which a later successful allocation may clear. Invalid-Birth guards
// MUST call it before acquiring actor, observer or registry locks; this reporter
// MUST NOT run while those locks are held.
func reportSessionBirthRefusal(logger *slog.Logger, operation string, tunnelID, sessionID uint16) {
	const problem = "L2TP could not identify the admitted subscriber"
	why := "Ze refused " + operation + " to keep another subscriber's state unchanged"
	const fix = "ze doctor"
	refusal := opererr.New(problem, why, fix)
	logger.Warn(refusal.Error(), "code", report.CodeInternalDefect,
		"operation", operation, reportFieldTunnelID, tunnelID, reportFieldSessionID, sessionID, "fix", fix)
	report.RaiseWarningFix(SubsystemName, report.CodeInternalDefect, sessionBirthRefusalReportSubject,
		problem+": "+why, fix,
		map[string]any{"operation": operation, reportFieldTunnelID: tunnelID, reportFieldSessionID: sessionID})
}
