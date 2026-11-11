// Design: docs/architecture/api/process-protocol.md -- problems a plugin reports to the daemon
// Related: dispatch_registry.go -- the engineOps entry for report-issue
// Related: ../../../../pkg/plugin/sdk/report.go -- the plugin side: RaiseIssue, ClearIssue

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// reportIssueEncodedMax bounds one encoded report-issue request before it is
// decoded: every field at its bound, with room for the JSON around them.
const reportIssueEncodedMax = 4 << 10

// reportSourceMax is the report bus's bound on a source, which a raise takes
// from the plugin's name. A name past it would be dropped by the bus, so the
// raise is refused instead.
const reportSourceMax = 64

// reportSourceRoute is the source of the refusal this route raises about a
// plugin that floods it: the route itself, never the plugin, which did not
// raise it.
const reportSourceRoute = "plugin-report-route"

// opReportIssue raises or clears one problem on the daemon's report bus for a
// plugin, which is the only bus show doctor reads: a forked plugin's own bus
// is a copy in its process. The source is the plugin's authenticated name, so
// no plugin raises or clears under another owner. Every refusal is the answer
// to the plugin, naming the field and the bound, because the bus itself drops
// a bad raise with no word to anyone.
func (s *Server) opReportIssue(_ context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	if len(params) > reportIssueEncodedMax {
		return nil, fmt.Errorf("report-issue request is %d octets, above the limit of %d: shorten the message", len(params), reportIssueEncodedMax)
	}
	var in rpc.ReportIssueInput
	if err := json.Unmarshal(params, &in); err != nil {
		return nil, fmt.Errorf("decode report-issue: %w", err)
	}
	if err := rpc.ValidateReportIssue(&in); err != nil {
		return nil, err
	}
	if proc == nil {
		return nil, errors.New("report-issue has no plugin owner")
	}
	source := proc.Name()
	if len(source) > reportSourceMax {
		return nil, fmt.Errorf("report-issue from plugin %s: the name is %d octets and the report bus holds %d; configure the plugin under a shorter name", source, len(source), reportSourceMax)
	}
	// The startup coordinator answers only stage methods until the handshake
	// ends; this refuses the same request on a path that reaches the entry
	// earlier, so neither transport raises for a plugin that has not started.
	if stage := proc.Stage(); stage != plugin.StageRunning {
		return nil, fmt.Errorf("report-issue from plugin %s arrived at startup stage %s, before its startup handshake ended: raise from OnStarted or a later callback", source, stage)
	}
	// The registered event codes are the ones the recent-events check
	// declares, so doctor renders the raise under its own code, `ze show
	// diagnostic` describes it, and no plugin raises a code another check
	// owns. A code that is only described (diagnostic.Lookup) is refused:
	// recent-events would render it under a code no check of its own reports.
	if !slices.Contains(diagnostic.EventCodes(), in.Code) {
		return nil, fmt.Errorf("report-issue code %s is not a registered event code: the daemon reports only the event codes compiled into it, so doctor could not describe it; raise one of those codes (ze show diagnostic <code> describes each)", in.Code)
	}
	switch in.Action {
	case rpc.ReportClear:
		report.ClearWarning(source, in.Code, in.Subject)
		return nil, nil //nolint:nilnil // no result payload; (nil,nil) is success-with-no-content
	case rpc.ReportRaise:
		return s.raiseReported(source, &in)
	default: // open set: ValidateReportIssue refused every other action
		return nil, fmt.Errorf("report-issue action %q is neither %q nor %q", in.Action, rpc.ReportRaise, rpc.ReportClear)
	}
}

// raiseReported applies the per-plugin raise bound (R-2), then raises in on
// the bus under source with its fix.
func (s *Server) raiseReported(source string, in *rpc.ReportIssueInput) (any, error) {
	admitted, firstRefusal := s.pluginReports.admit(source)
	if !admitted {
		if firstRefusal {
			logger().Warn("plugin report raises refused past the rate bound",
				"code", diagnostic.CodeDoctorPluginRequestRefused, "plugin", source,
				"raises", rpc.ReportRaisesPerWindow, "window", rpc.ReportRaiseWindow)
			report.RaiseError(reportSourceRoute, diagnostic.CodeDoctorPluginRequestRefused, source,
				"plugin "+source+" raised more than "+fmt.Sprint(rpc.ReportRaisesPerWindow)+" problems in "+rpc.ReportRaiseWindow.String()+
					"; its later raises in that window were refused and never reached the report bus")
		}
		return nil, fmt.Errorf("report-issue from plugin %s refused: it raised %d problems in the last %s, the limit; raise a problem once when it starts and clear it when it ends", source, rpc.ReportRaisesPerWindow, rpc.ReportRaiseWindow)
	}
	switch in.Severity {
	case rpc.ReportWarning:
		report.RaiseWarningFix(source, in.Code, in.Subject, in.Message, in.Fix)
	case rpc.ReportError:
		report.RaiseErrorFix(source, in.Code, in.Subject, in.Message, in.Fix)
	default: // open set: ValidateReportIssue refused every other severity
		return nil, fmt.Errorf("report-issue raise severity %q is neither %q nor %q", in.Severity, rpc.ReportWarning, rpc.ReportError)
	}
	return nil, nil //nolint:nilnil // no result payload; (nil,nil) is success-with-no-content
}

// pluginReportRate counts each plugin's raises in fixed windows of
// rpc.ReportRaiseWindow and admits at most rpc.ReportRaisesPerWindow in each.
// It holds one entry per plugin name, and names come from the configured
// plugins, so it is bounded by the config. Safe for concurrent use; the zero
// value is ready and reads the wall clock.
type pluginReportRate struct {
	mu      sync.Mutex
	now     func() time.Time // nil reads time.Now; a test moves its own clock
	windows map[string]*reportWindow
}

// reportWindow is one plugin's current window.
type reportWindow struct {
	start   time.Time
	raises  int
	refused bool // a raise was refused in this window, and the refusal reported
}

// admit counts one raise from owner. It answers whether the raise is within
// the bound, and whether it is the first refused in the window, which the
// caller reports once.
func (r *pluginReportRate) admit(owner string) (admitted, firstRefusal bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	if r.windows == nil {
		r.windows = make(map[string]*reportWindow)
	}
	window := r.windows[owner]
	if window == nil {
		window = &reportWindow{start: now}
		r.windows[owner] = window
	}
	if now.Sub(window.start) >= rpc.ReportRaiseWindow {
		*window = reportWindow{start: now}
	}
	if window.raises < rpc.ReportRaisesPerWindow {
		window.raises++
		return true, false
	}
	firstRefusal = !window.refused
	window.refused = true
	return false, firstRefusal
}
