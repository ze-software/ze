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
	"strconv"
	"sync"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// jsonEscapeOctetsMax is the most octets JSON spends on one octet of a string
// value: a \u00XX escape. json.Marshal, which the SDK encodes every request
// with, writes one for a control character, for <, > and & (< and the
// rest), and � for each octet that is not valid UTF-8.
const jsonEscapeOctetsMax = 6

// reportIssueFraming is a report-issue request whose values are all empty:
// the keys, quotes and punctuation json.Marshal writes around the values.
const reportIssueFraming = `{"action":"","severity":"","code":"","subject":"","message":"","fix":""}`

// reportIssueEncodedMax bounds one encoded report-issue request before it is
// decoded. It is derived from the field bounds: the framing, the longest
// action and severity (plain ASCII, which JSON never escapes), and every
// octet of every string field at its bound written as an escape. So any
// request rpc.ValidateReportIssue accepts, encoded by the SDK, fits under it,
// and the SDK and the engine agree; a sender that pads the JSON with
// whitespace between tokens may not, and is refused here.
const reportIssueEncodedMax = len(reportIssueFraming) + len(rpc.ReportRaise) + len(rpc.ReportWarning) +
	jsonEscapeOctetsMax*(rpc.ReportCodeMax+rpc.ReportSubjectMax+rpc.ReportMessageMax+rpc.ReportFixMax)

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
	// The report bus bounds a source as it bounds every field; a raise takes
	// its source from the plugin's name, so a name past the bound would be
	// dropped by the bus, and the raise is refused instead.
	source := proc.Name()
	if len(source) > report.SourceMax {
		return nil, fmt.Errorf("report-issue from plugin %s: the name is %d octets and the report bus holds %d; configure the plugin under a shorter name", source, len(source), report.SourceMax)
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
// the bus under source with its fix: an error as an event, a warning within
// the plugin's share of the bus's active warnings.
func (s *Server) raiseReported(source string, in *rpc.ReportIssueInput) (any, error) {
	admitted, firstRefusal := s.pluginReports.admit(source)
	if !admitted {
		// The log line is bounded by the plugin's own rate window, not by
		// refusePluginRequest: that bound is shared by every kind of refused
		// request, so an earlier refusal of another kind would otherwise leave
		// this one with no line and no event for the rest of its window.
		if firstRefusal {
			logger().Warn("plugin report raises refused past the rate bound",
				"code", diagnostic.CodeDoctorPluginRequestRefused, "plugin", source,
				"raises", rpc.ReportRaisesPerWindow, "window", rpc.ReportRaiseWindow)
		}
		refusePluginRequest(source, "plugin "+source+" raised more than "+strconv.Itoa(rpc.ReportRaisesPerWindow)+" problems in "+
			rpc.ReportRaiseWindow.String()+"; its later raises in that window were refused and never reached the report bus")
		return nil, fmt.Errorf("report-issue from plugin %s refused: it raised %d problems in the last %s, the limit; raise a problem once when it starts and clear it when it ends", source, rpc.ReportRaisesPerWindow, rpc.ReportRaiseWindow)
	}
	switch in.Severity {
	case rpc.ReportWarning:
		if err := s.pluginReports.raiseWarningWithin(source, in); err != nil {
			return nil, err
		}
	case rpc.ReportError:
		report.RaiseErrorFix(source, in.Code, in.Subject, in.Message, in.Fix)
	default: // open set: ValidateReportIssue refused every other severity
		return nil, fmt.Errorf("report-issue raise severity %q is neither %q nor %q", in.Severity, rpc.ReportWarning, rpc.ReportError)
	}
	return nil, nil //nolint:nilnil // no result payload; (nil,nil) is success-with-no-content
}

// activeWarningsMax is the most active warnings one plugin keeps on a bus
// whose warning cap is warningCap: its rpc.ReportActiveWarningsShare share,
// rounded down, and at least one.
func activeWarningsMax(warningCap int) int {
	return max(1, warningCap/rpc.ReportActiveWarningsShare)
}

// pluginReportRate counts each plugin's raises in fixed windows of
// rpc.ReportRaiseWindow and admits at most rpc.ReportRaisesPerWindow in each,
// and serializes the warning raises the route makes, so the per-plugin share
// of active warnings is counted and raised under one lock. It holds one entry
// per plugin whose process is running or ran since its last cleanup:
// cleanupProcess forgets the entry when a process ends, an ad-hoc session's
// included, so the map is bounded by the processes alive at once. Safe for
// concurrent use; the zero value is ready and reads the wall clock.
type pluginReportRate struct {
	mu      sync.Mutex
	now     func() time.Time // nil reads time.Now; a test moves its own clock
	windows map[string]*reportWindow
}

// reportWindow is one plugin's current window. The two refused bits bound
// the route's own log lines to one per window for each kind of refusal; the
// bus event is bounded apart, by refusePluginRequest.
type reportWindow struct {
	start        time.Time
	raises       int
	rateRefused  bool // a raise past the rate was refused in this window, and logged
	shareRefused bool // a warning past the plugin's share was refused in this window, and logged
}

// admit counts one raise from owner. It answers whether the raise is within
// the bound, and whether it is the first refused in the window, which the
// caller logs once.
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
	firstRefusal = !window.rateRefused
	window.rateRefused = true
	return false, firstRefusal
}

// forget drops owner's window. The process behind it ended, so no raise of
// its can arrive; a process started later under the name starts a window.
func (r *pluginReportRate) forget(owner string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.windows, owner)
}

// raiseWarningWithin raises in as an active warning of source, unless it is
// a new warning that would take source past its share of the bus
// (activeWarningsMax). The bus is the ledger, read in one call: a clear, a
// ClearSource or an eviction frees a slot with nothing here to update, and a
// raise that refreshes an active warning adds none. r.mu is held across the
// count and the raise, so two raises from one plugin cannot both pass it.
func (r *pluginReportRate) raiseWarningWithin(source string, in *rpc.ReportIssueInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	active, present, warningCap := report.SourceWarnings(source, in.Code, in.Subject)
	limit := activeWarningsMax(warningCap)
	if !present && active >= limit {
		// Logged once per rate window of the plugin, apart from the bus
		// event's bound, as the rate refusal is (raiseReported). admit ran
		// first, so the window exists unless the process ended meanwhile;
		// then the line is written.
		if window := r.windows[source]; window == nil || !window.shareRefused {
			if window != nil {
				window.shareRefused = true
			}
			logger().Warn("plugin report warning refused past its share of the bus",
				"code", diagnostic.CodeDoctorPluginRequestRefused, "plugin", source,
				"active", active, "limit", limit, "cap", warningCap)
		}
		refusePluginRequest(source, "plugin "+source+" keeps "+strconv.Itoa(active)+" warnings active, its share of the report bus ("+
			strconv.Itoa(limit)+"); its new warnings are refused until it clears one")
		return fmt.Errorf("report-issue from plugin %s refused: it keeps %d warnings active, the most one plugin may keep (the report bus holds %d, and one plugin gets 1/%d of it); clear a warning that no longer holds with ClearIssue, then raise again",
			source, active, warningCap, rpc.ReportActiveWarningsShare)
	}
	report.RaiseWarningFix(source, in.Code, in.Subject, in.Message, in.Fix)
	return nil
}

// refusePluginRequest raises the refusal of a request from plugin as a
// doctor-plugin-request-refused error event whose subject is the plugin, at
// most once per plugin in each rpc.ReportRaiseWindow. A plugin's own requests
// drive these events, so without the bound one plugin looping on a bad
// request (a malformed emit, a failing command, a malformed destination, a
// flood of report-issue raises) overwrites the whole error ring and hides
// every other owner's events. The bound is one for every kind of request: the
// ring is the ledger (report.RaiseErrorFixOnce), so it holds no state that
// could grow or outlive the plugin. Because the bound is shared, it answers
// nothing a caller could gate its log line on: an earlier refusal of another
// kind would silence that line. A caller logs at every occurrence, or once
// per window by a bound of its own, as the report-issue route does
// (reportWindow).
func refusePluginRequest(plugin, message string) {
	report.RaiseErrorFixOnce(serverEventSource, diagnostic.CodeDoctorPluginRequestRefused, plugin,
		report.FitMessage(message+"; ze raises one such event per plugin in "+rpc.ReportRaiseWindow.String()),
		fixShowErrors, rpc.ReportRaiseWindow)
}

// forgetPluginReports drops the report-issue rate window of name, whose
// process ended, so the windows do not grow with the plugins and the ad-hoc
// sessions that come and go.
func (s *Server) forgetPluginReports(name string) {
	s.pluginReports.forget(name)
}
