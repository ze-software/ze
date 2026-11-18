// Design: docs/architecture/doctor-and-health-checks.md -- redistribution refusals reach the daemon.
// Related: register.go -- the original SDK instance starts and joins the report queue.
package redistribute

import (
	"context"
	"net/netip"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/report"
)

const unknownWithdrawalCode = "doctor-redistribute-source-unregistered"
const unknownWithdrawalFix = "ze show diagnostic doctor-redistribute-source-unregistered"

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        unknownWithdrawalCode,
		Title:       "Redistribution refused an unknown withdrawal source",
		Description: "A route withdrawal named no registered source protocol, so the destination consumer was left unchanged. Inspect the source name in the event, correct its registration or the producer, then restart the corrected build and confirm its current routes.",
		Examples:    []string{unknownWithdrawalFix, "ze request restart"},
	}); err != nil {
		panic("BUG: redistribution source diagnostic registration: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: unknownWithdrawalCode}); err != nil {
		panic("BUG: redistribution source event registration: " + err.Error())
	}
}

// redistributionIssuesKey is private runtime context data, never a route fact
// or authority. Its immutable queue belongs to the original SDK instance.
type redistributionIssuesKey struct{}

func withRedistributionIssues(ctx context.Context, queue *issuequeue.Queue) context.Context {
	return context.WithValue(ctx, redistributionIssuesKey{}, queue)
}

// reportUnknownWithdrawal refuses the consumer mutation and reports one actual
// failed operation. Enqueue is bounded and nonblocking, including under a
// caller's lock. The runtime MUST join its run goroutine before stopping this
// original queue. Stop seals admission and drains accepted reports; it does
// not prove every copied event callback joined. A late callback keeps this
// queue, receives ErrStopped and logs report-error. No current process lookup
// supplies a reporter.
func reportUnknownWithdrawal(ctx context.Context, source string, fam family.Family, prefix netip.Prefix) {
	queue, _ := ctx.Value(redistributionIssuesKey{}).(*issuequeue.Queue)
	message := report.FitMessage("Redistribution refused withdrawal of " + prefix.String() + " in " + fam.String() + " from unregistered source " + source + "; the destination consumer was left unchanged.")
	failed := func(err error) {
		logger().Warn("The daemon did not accept the redistribution withdrawal report",
			"code", unknownWithdrawalCode, "source", source, "prefix", prefix,
			"error", err, "fix", unknownWithdrawalFix)
	}
	err := queue.RaiseError(unknownWithdrawalCode, "route withdrawal", message, unknownWithdrawalFix, failed)
	logger().Warn("Redistribution refused a withdrawal with no registered source; the destination consumer was left unchanged",
		"code", unknownWithdrawalCode, "source", source, "family", fam.String(), "prefix", prefix, "fix", unknownWithdrawalFix, issuequeue.Outcome(err))
}
