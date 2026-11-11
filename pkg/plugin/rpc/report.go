// Design: docs/architecture/api/process-protocol.md -- problems a plugin reports to the daemon.
// Related: ../sdk/report.go -- the plugin side: RaiseIssue and ClearIssue

package rpc

import (
	"errors"
	"fmt"
	"time"
)

// MethodReportIssue raises or clears one problem on the daemon's report bus,
// where show warnings, show errors and show doctor read it. A plugin that
// runs in its own process has its own copy of the bus, which nothing reads,
// so this request is the only way its problems reach the operator.
const MethodReportIssue = "ze-plugin-engine:report-issue"

// Bounds of one report-issue request, enforced by the SDK and the engine
// alike. The field bounds are the report bus's own (internal/core/report,
// validFields), so a request the engine accepts is never dropped by the bus;
// the engine's TestReportRaiseRouteBothTransports raises at every bound to
// prove it.
const (
	ReportCodeMax    = 64   // octets in a code
	ReportSubjectMax = 256  // octets in a subject
	ReportMessageMax = 1024 // octets in a message
	ReportFixMax     = 256  // octets in a fix: one command line

	// ReportRaisesPerWindow raises are accepted from one plugin in each
	// ReportRaiseWindow; a raise past it is refused. A problem is raised once
	// when it starts (a warning refreshed by a raise of the same code and
	// subject) and cleared when it ends, so a plugin within the bound reports
	// every problem it has, and one past it is flooding the bus.
	ReportRaisesPerWindow = 60
	ReportRaiseWindow     = time.Minute
)

// ReportAction is what a report-issue request does.
type ReportAction string

const (
	ReportRaise ReportAction = "raise" // add an error event, or add or refresh an active warning
	ReportClear ReportAction = "clear" // remove the active warning of a code and subject
)

// ReportSeverity is the bus a raise lands on.
type ReportSeverity string

const (
	ReportWarning ReportSeverity = "warning" // active until cleared, once per code and subject
	ReportError   ReportSeverity = "error"   // an event: kept in the error ring, never cleared
)

// ReportIssueInput is one report-issue request. The engine names the source
// of every raise and clear itself, from the plugin's authenticated name, so a
// plugin cannot raise or clear under another owner's name: the request has no
// source field.
type ReportIssueInput struct {
	Action   ReportAction   `json:"action"`
	Severity ReportSeverity `json:"severity,omitempty"` // a raise; "warning" or empty on a clear
	Code     string         `json:"code"`
	Subject  string         `json:"subject"`
	Message  string         `json:"message,omitempty"` // a raise
	Fix      string         `json:"fix,omitempty"`     // a raise: the command that fixes the subject
}

// ValidateReportIssue refuses a request with a missing or oversized field, an
// unknown action or severity, or a clear of an error. Each refusal names the
// field and its bound. Whether the code is one the daemon describes is the
// engine's judgement, because only the engine holds the registry.
func ValidateReportIssue(in *ReportIssueInput) error {
	if err := reportField("code", in.Code, ReportCodeMax); err != nil {
		return err
	}
	if err := reportField("subject", in.Subject, ReportSubjectMax); err != nil {
		return err
	}
	switch in.Action {
	case ReportRaise:
		return validateReportRaise(in)
	case ReportClear:
		return validateReportClear(in)
	default: // open set: the action arrived over the wire
		return fmt.Errorf("report-issue action %q is neither %q nor %q", in.Action, ReportRaise, ReportClear)
	}
}

func validateReportRaise(in *ReportIssueInput) error {
	switch in.Severity {
	case ReportWarning, ReportError:
	default: // open set: the severity arrived over the wire
		return fmt.Errorf("report-issue raise severity %q is neither %q nor %q", in.Severity, ReportWarning, ReportError)
	}
	if err := reportField("message", in.Message, ReportMessageMax); err != nil {
		return err
	}
	if len(in.Fix) > ReportFixMax {
		return fmt.Errorf("report-issue fix is %d octets, above the limit of %d: name one command", len(in.Fix), ReportFixMax)
	}
	return nil
}

func validateReportClear(in *ReportIssueInput) error {
	switch in.Severity {
	case "", ReportWarning:
		return nil
	case ReportError:
		return errors.New("report-issue clear names an error, and an error cannot be cleared: it is an event, kept until the error ring overwrites it; only a warning is cleared")
	default: // open set: the severity arrived over the wire
		return fmt.Errorf("report-issue clear severity %q is not %q", in.Severity, ReportWarning)
	}
}

// reportField refuses an empty value or one above its bound, naming the field.
func reportField(field, value string, limit int) error {
	if value == "" {
		return fmt.Errorf("report-issue %s is empty: the report bus needs one", field)
	}
	if len(value) > limit {
		return fmt.Errorf("report-issue %s is %d octets, above the limit of %d", field, len(value), limit)
	}
	return nil
}
