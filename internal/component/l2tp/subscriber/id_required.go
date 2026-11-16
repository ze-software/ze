// Design: docs/architecture/l2tp/subscriber-session-model.md -- subscriber identity at authorization and allocation boundaries

package subscriber

import (
	"log/slog"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

// RequireID refuses a request whose transport supplied no subscriber identity.
// Callers MUST check it before reading or changing subscriber-owned state.
// The numeric PPP driver pair is transport-local and MUST NOT replace the ID.
func RequireID(logger *slog.Logger, id string) error {
	if id != "" {
		return nil
	}
	const problem = "A subscriber request has no subscriber identity"
	const why = "Ze cannot isolate its authorization or address ownership from another transport"
	const fix = "ze show errors"
	err := opererr.New(problem, why, fix)
	logger.Error(err.Error(), "code", report.CodeInternalDefect)
	report.RaiseErrorFix("subscriber", report.CodeInternalDefect, "missing identity", problem+": "+why, fix)
	return err
}
