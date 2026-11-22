// Design: docs/architecture/doctor-and-health-checks.md -- transient recall failures.
// Related: history.go -- Load and publish own the failed storage operations.
package cli

import (
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	historyEventSource  = "cli-history"
	historyEventSubject = "command-recall"
	historyEventWindow  = time.Minute
	historyEventFix     = "ze show doctor"
)

func init() {
	registerDoctorCodes()
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorCLIHistoryRead}); err != nil {
		panic("BUG: command-history read event registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: diagnostic.CodeDoctorCLIHistoryWrite}); err != nil {
		panic("BUG: command-history write event registration refused: " + err.Error())
	}
}

// reportHistoryReadFailure records the operation without its raw storage error:
// a backend can put command content or credentials in that error. mode MUST be
// one of the two validated storage mode names. Fixed keys and the ring's own
// ledger bound all user-driven failures to one class event per minute.
// Safe for concurrent use.
func reportHistoryReadFailure(mode string) {
	report.RaiseErrorFixOnce(historyEventSource, codeDoctorCLIHistoryRead, historyEventSubject,
		"Command history read unavailable for "+mode+" mode; saved recall could not be read and will not be overwritten. Restore configuration-store access, then reconnect.",
		historyEventFix, historyEventWindow)
}

// reportHistoryWriteFailure records a failed publication when it completes,
// including one that recovers before a caller polls LastWriteError. mode MUST
// be one of the two validated storage mode names. Never pass command snapshots,
// usernames or raw errors to the bus. Safe for concurrent use.
func reportHistoryWriteFailure(mode string) {
	report.RaiseErrorFixOnce(historyEventSource, diagnostic.CodeDoctorCLIHistoryWrite, historyEventSubject,
		"Command history write unavailable for "+mode+" mode; recall may be lost when this session ends. Restore configuration-store access, then reconnect.",
		historyEventFix, historyEventWindow)
}
