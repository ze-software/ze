//go:build ze_web

// Design: docs/architecture/doctor-and-health-checks.md -- web cleanup failures.
// Related: service_web.go -- the service owns the strict borrowed-store join.
package hub

import (
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	webCleanupEventSource         = "web-server"
	webCleanupEventSubject        = "web-shutdown"
	webNetworkCleanupEventSubject = "web-network-cleanup"
	webCleanupEventWindow         = time.Minute
	webCleanupEventFix            = "ze show log recent level err component web.server"
)

func init() {
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorWebCleanupOverdue}); err != nil {
		panic("BUG: web cleanup event registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorWebNetworkCleanup}); err != nil {
		panic("BUG: web network cleanup event registration refused: " + err.Error())
	}
}

// reportWebNetworkCleanupFailure reports the network owner's result before
// history cleanup can block. It does not classify a history or overdue failure
// as a network failure. Safe for concurrent use.
func reportWebNetworkCleanupFailure(err error) {
	const message = "Web network cleanup reported a failure. Admitted handlers have finished, and command-history cleanup still runs. Inspect recent web.server errors to locate the cause."
	report.RaiseErrorFixOnce(webCleanupEventSource, codeDoctorWebNetworkCleanup, webNetworkCleanupEventSubject,
		message, webCleanupEventFix, webCleanupEventWindow)
	slogutil.Logger("web.server").Error(message, "code", codeDoctorWebNetworkCleanup, "fix", webCleanupEventFix, "error", err)
}
