// Design: docs/architecture/l2tp/subscriber-session-model.md -- immutable birth before admission

package l2tp

import (
	"log/slog"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

const sessionBirthReportSubject = "session reference"

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
		map[string]any{"cause": err.Error()})
	return subscriberid.L2TPBirth{}, refusal
}
