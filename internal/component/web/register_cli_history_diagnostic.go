// Design: docs/architecture/doctor-and-health-checks.md -- browser recall boundary failures.
// Related: cli_history.go -- request admission and JSON delivery own separate classes.
package web

import (
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	browserHistoryEventSource          = "web-history"
	browserHistoryEventSubject         = "command-recall"
	browserHistoryResponseEventSubject = "command-recall-response"
	browserHistoryEventWindow          = time.Minute
	browserHistoryEventFix             = "Open /cli and reconnect to restore command recall"
)

func init() {
	registerDoctorCodes()
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorWebHistoryUnavailable}); err != nil {
		panic("BUG: browser recall admission event registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorWebHistoryResponse}); err != nil {
		panic("BUG: browser recall response event registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorWebEditorEvictDiscardFailed}); err != nil {
		panic("BUG: web editor eviction discard event registration refused: " + err.Error())
	}
}

// reportBrowserHistoryUnavailable records the request boundary, including an
// invalid receipt. Storage owners separately report their failed reads/writes.
// Fixed keys bound user-driven failures without retaining any request or error
// content. Safe for concurrent use.
func reportBrowserHistoryUnavailable() {
	const message = "Command recall was unavailable for a browser submission. Command execution remains available. Open /cli and reconnect to restore command recall."
	report.RaiseErrorFixOnce(browserHistoryEventSource, codeDoctorWebHistoryUnavailable, browserHistoryEventSubject,
		message, browserHistoryEventFix, browserHistoryEventWindow)
	serverLogger.Warn(message, "code", codeDoctorWebHistoryUnavailable, "fix", browserHistoryEventFix)
}

// reportBrowserHistoryResponseFailure records only actual JSON delivery
// failures. A successful storage read is not classified as a failed read when
// its response cannot reach the browser. Safe for concurrent use.
func reportBrowserHistoryResponseFailure() {
	const message = "Command recall could not be sent to the browser. Saved recall was not changed. Open /cli and reconnect to restore command recall."
	report.RaiseErrorFixOnce(browserHistoryEventSource, codeDoctorWebHistoryResponse, browserHistoryResponseEventSubject,
		message, browserHistoryEventFix, browserHistoryEventWindow)
	serverLogger.Warn(message, "code", codeDoctorWebHistoryResponse, "fix", browserHistoryEventFix)
}
