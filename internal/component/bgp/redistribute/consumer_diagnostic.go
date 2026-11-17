// Design: docs/guide/redistribution.md -- BGP redistribution operation failures.
// Related: register.go -- the recent-events doctor owns this transient code.
package redistribute

import (
	"context"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const codeBGPRedistributionFailure = "doctor-bgp-redistribution"
const bgpRedistributionFix = "ze show bgp peers"
const bgpRedistributionSubject = "BGP redistribution"
const bgpRedistributionReportBudget = time.Second

// consumerIssueReporter is the existing SDK connection's report capability.
// The sole production consumer factory receives the orchestrator's *sdk.Plugin;
// reduced updater-only test adapters need not manufacture a report connection.
type consumerIssueReporter interface {
	RaiseIssue(context.Context, sdk.Issue) error
}

var _ consumerIssueReporter = (*sdk.Plugin)(nil)

func (c *BGPConsumer) reportFailure(ctx context.Context, operation, peer, prefix string, fam family.Family, command string, cause error) {
	message := fmt.Sprintf("BGP redistribution could not %s %s route %s toward %s. The requested BGP output was not confirmed. Cause: %s", operation, fam, prefix, peer, cause)
	logger().Warn(message, "code", codeBGPRedistributionFailure, "fix", bgpRedistributionFix,
		"operation", operation, "peer", peer, "prefix", prefix, "family", fam.String(), "error", cause, "command", command)
	reporter, available := c.dispatcher.(consumerIssueReporter)
	if !available {
		return
	} // The actual factory supplies the SDK; this is a reduced test adapter.
	work, cancel := context.WithTimeout(ctx, bgpRedistributionReportBudget)
	defer cancel()
	// A failed operation is a bounded recent error event, not a persistent claim
	// that every route is currently broken. Later success must not erase another
	// operation's incident, and no per-route warning/history map is retained here.
	err := reporter.RaiseIssue(work, sdk.Issue{Severity: rpc.ReportError, Code: codeBGPRedistributionFailure,
		Subject: bgpRedistributionSubject, Message: report.FitMessage(message), Fix: bgpRedistributionFix})
	if err != nil {
		logger().Warn("BGP redistribution failure report was not acknowledged; inspect the original route failure in the log",
			"code", codeBGPRedistributionFailure, "fix", bgpRedistributionFix, "error", err, "prefix", prefix, "peer", peer)
	}
}
