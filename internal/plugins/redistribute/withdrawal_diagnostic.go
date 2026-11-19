// Design: docs/architecture/doctor-and-health-checks.md -- redistribution refusals reach the daemon.
// Related: register.go -- the original SDK instance starts and joins the report queue.
package redistribute

import (
	"context"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
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

// unconfirmedWithdrawalCode is registered from register.go's init with
// unconfirmedWithdrawalMeta.
const unconfirmedWithdrawalCode = "doctor-redistribute-withdrawal-unconfirmed"
const unconfirmedWithdrawalFix = "ze show diagnostic doctor-redistribute-withdrawal-unconfirmed"

// unconfirmedWithdrawalMeta describes unconfirmedWithdrawalCode to doctor.
var unconfirmedWithdrawalMeta = diagnostic.CodeMeta{
	Code:        unconfirmedWithdrawalCode,
	Title:       "Redistribution could not confirm a route withdrawal",
	Description: "A destination consumer answered a redistributed route's withdrawal with an error, so the orchestrator keeps the route held at the consumer instance that accepted it and withdraws it there again at the next removal of the route or the next change of the redistribute rules. Inspect the destination's own failure (for BGP, ze show errors lists doctor-bgp-redistribution with the peer and cause), restore it, then retry the source change.",
	Examples:    []string{unconfirmedWithdrawalFix, "ze show errors"},
}

// errWithdrawalSourceUnregistered answers a withdrawal that named no registered
// source: it reached no consumer, so nothing confirmed it.
// reportUnknownWithdrawal has already reported it.
var errWithdrawalSourceUnregistered = errors.New("redistribution withdrawal names no registered source protocol")

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

// reportUnconfirmedWithdrawal reports one withdrawal of key's contribution that
// its consumer answered with cause, with the original source instance and
// table, through the same original SDK queue as reportUnknownWithdrawal. The
// caller MUST NOT hold the held set's lock. A withdrawal refused for naming no
// registered source was reported when it was refused, so it is not reported
// twice.
func reportUnconfirmedWithdrawal(ctx context.Context, key heldKey, cause error) {
	if errors.Is(cause, errWithdrawalSourceUnregistered) {
		return
	}
	queue, _ := ctx.Value(redistributionIssuesKey{}).(*issuequeue.Queue)
	var text textbuf.Buffer
	text.Str("Redistribution could not confirm withdrawal of ").Prefix(key.prefix).Str(" in ").Str(key.family.String()).
		Str(" from source ").Str(key.source).Str(" instance ").Uint32(key.instance).Str(" table ").Uint32(key.table).
		Str(" at destination ").Str(key.consumer).
		Str("; the route stays held at the consumer that accepted it and is withdrawn there again at its next removal or rules reload. Cause: ").
		Err(cause)
	message := report.FitMessage(text.String())
	failed := func(err error) {
		logger().Warn("The daemon did not accept the redistribution withdrawal report",
			"code", unconfirmedWithdrawalCode, "consumer", key.consumer, "source", key.source, "prefix", key.prefix,
			"error", err, "fix", unconfirmedWithdrawalFix)
	}
	err := queue.RaiseError(unconfirmedWithdrawalCode, "route withdrawal", message, unconfirmedWithdrawalFix, failed)
	logger().Warn("Redistribution could not confirm a withdrawal; the route stays held at the consumer that accepted it for the next attempt",
		"code", unconfirmedWithdrawalCode, "consumer", key.consumer, "source", key.source, "instance", key.instance, "table", key.table,
		"family", key.family.String(), "prefix", key.prefix, "error", cause, "fix", unconfirmedWithdrawalFix, issuequeue.Outcome(err))
}
