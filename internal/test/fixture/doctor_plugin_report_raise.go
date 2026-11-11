// Design: docs/architecture/api/process-protocol.md -- problems a plugin reports to the daemon
// Overview: register_doctor_plugin_report_raise.go -- the observer registered from this scenario
// Related: plugin_fixture_06.go -- fixture06DispatchObject, the show doctor read
// Related: dns_source_operator.go -- operatorPollAttempts, operatorPollDelay

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Report-route observer values. The fix is a command recent-events never
// writes on its own, so a row carrying it carries the raise's fix.
const (
	reportRaisePlugin       = "doctor-report-raise-test" // the name the .ci configures, so the source the daemon names
	reportRaiseSubject      = "report-route-observer"
	reportRaiseMessage      = "raised by a forked plugin through report-issue"
	reportRaiseFix          = "show plugin list"
	reportRaiseUnregistered = "doctor-report-route-unregistered"
)

// doctorPluginReportRaise raises a warning through the SDK from this forked
// process, polls show doctor until the row carries the warning under its own
// code, names this plugin as the source and carries the raise's fix, checks
// that an unregistered code is refused, then clears the warning and polls
// until the row is gone.
func doctorPluginReportRaise(ctx context.Context, p *sdk.Plugin) error {
	source := reportRaisePlugin
	if err := p.RaiseIssue(ctx, sdk.Issue{
		Severity: rpc.ReportWarning,
		Code:     diagnostic.CodeDoctorInternalDefect,
		Subject:  reportRaiseSubject,
		Message:  reportRaiseMessage,
		Fix:      reportRaiseFix,
	}); err != nil {
		return fmt.Errorf("raise through report-issue: %w", err)
	}
	var last map[string]any
	raised := Poll(ctx, operatorPollAttempts, operatorPollDelay, func() bool {
		data, err := fixture06DispatchObject(ctx, p, "show doctor")
		if err != nil {
			return false
		}
		last = data
		return reportRaiseRow(data, source)
	})
	if !raised {
		return fmt.Errorf("show doctor lacks %s from %s on %s with fix %q; last answer %v",
			diagnostic.CodeDoctorInternalDefect, source, reportRaiseSubject, reportRaiseFix, last)
	}

	err := p.RaiseIssue(ctx, sdk.Issue{
		Severity: rpc.ReportWarning, Code: reportRaiseUnregistered, Subject: reportRaiseSubject, Message: reportRaiseMessage,
	})
	if err == nil {
		return errors.New("the daemon accepted a raise of an unregistered code")
	}
	if !strings.Contains(err.Error(), "not a registered event code") {
		return fmt.Errorf("an unregistered code was refused for another reason: %w", err)
	}

	if err := p.ClearIssue(ctx, diagnostic.CodeDoctorInternalDefect, reportRaiseSubject); err != nil {
		return fmt.Errorf("clear through report-issue: %w", err)
	}
	cleared := Poll(ctx, operatorPollAttempts, operatorPollDelay, func() bool {
		data, err := fixture06DispatchObject(ctx, p, "show doctor")
		if err != nil {
			return false
		}
		last = data
		return !reportRaiseRow(data, source)
	})
	if !cleared {
		return fmt.Errorf("show doctor still carries the cleared warning; last answer %v", last)
	}
	_, err = fmt.Fprintln(os.Stderr, "OK: show doctor reports a forked plugin's raise with its fix, and its clear removes it")
	return err
}

// reportRaiseRow answers whether data holds the observer's warning: the code
// it raised, a message naming its source and subject, and its own fix.
func reportRaiseRow(data map[string]any, source string) bool {
	diagnostics, ok := data["diagnostics"].([]any)
	if !ok {
		return false
	}
	for _, raw := range diagnostics {
		entry, isMap := raw.(map[string]any)
		if !isMap {
			continue
		}
		if entry["code"] != diagnostic.CodeDoctorInternalDefect {
			continue
		}
		if entry["fix"] != reportRaiseFix {
			continue
		}
		message, isString := entry["message"].(string)
		if !isString {
			continue
		}
		if strings.HasPrefix(message, source+" warning on "+reportRaiseSubject) {
			return true
		}
	}
	return false
}
