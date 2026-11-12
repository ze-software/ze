// Design: docs/architecture/api/process-protocol.md -- problems a plugin reports to the daemon.
// Related: ../rpc/report.go -- the report-issue wire type and its bounds

package sdk

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Issue is one problem a plugin raises on the daemon's report bus
// (RaiseIssue). Code MUST be an event code the daemon registered
// (`ze show diagnostic <code>` describes it), so doctor renders the problem
// under that code; Fix is the command that fixes Subject, or empty when the
// plugin knows none.
type Issue struct {
	Severity rpc.ReportSeverity
	Code     string
	Subject  string
	Message  string
	Fix      string
}

// RaiseIssue raises issue on the daemon's report bus under this plugin's
// name, where show warnings, show errors and show doctor read it. A warning
// stays active until ClearIssue clears it, and a second raise of one code and
// subject refreshes it; an error is an event and cannot be cleared.
//
// It works the same internal and forked: a forked plugin's own report bus is
// a copy nothing reads, so a problem raised there reaches no operator. Like
// the state methods it is sent after the handshake, from OnStarted or a later
// callback: the engine accepts only the stage methods until then. The engine
// refuses a code it does not describe, a field past its bound, a raise past
// rpc.ReportRaisesPerWindow, and a new warning past the plugin's share of
// active warnings (rpc.ReportActiveWarningsShare), with an error naming the
// cause.
func (p *Plugin) RaiseIssue(ctx context.Context, issue Issue) error {
	in := rpc.ReportIssueInput{
		Action:   rpc.ReportRaise,
		Severity: issue.Severity,
		Code:     issue.Code,
		Subject:  issue.Subject,
		Message:  issue.Message,
		Fix:      issue.Fix,
	}
	return p.reportCall(ctx, &in)
}

// ClearIssue removes the active warning of code and subject this plugin
// raised. Clearing a warning that is not active succeeds and changes nothing.
func (p *Plugin) ClearIssue(ctx context.Context, code, subject string) error {
	in := rpc.ReportIssueInput{Action: rpc.ReportClear, Code: code, Subject: subject}
	return p.reportCall(ctx, &in)
}

// reportCall checks in against the wire bounds before it is sent, the same
// check the engine makes on receipt, so a call the bounds refuse costs no
// round trip.
func (p *Plugin) reportCall(ctx context.Context, in *rpc.ReportIssueInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := rpc.ValidateReportIssue(in); err != nil {
		return err
	}
	return p.callEngine(ctx, rpc.MethodReportIssue, in)
}
