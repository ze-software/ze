// Design: docs/architecture/isis/isis-5-adjacency.md -- a failed Hello send warns once per failure run
// Related: circuits.go -- helloSendWarn raises and clears the Hello send warning through issueQueue
// Related: events.go -- warnSessionEventRefused and noteLSPChangeRefused raise through issueQueue
// Related: register.go -- runISISPlugin starts and stops the report queue
//
// IS-IS raises its operating problems on the daemon's report bus through the
// SDK, never through internal/core/report: IS-IS may run forked, and a forked
// process's report bus is a copy the daemon never reads
// (docs/architecture/doctor-and-health-checks.md, "Event codes"). Every raise
// and clear goes through the plugin report queue
// (internal/component/plugin/issuequeue), so a circuit's Hello goroutine and
// the code that emits an event never wait for the daemon.
package isis

import (
	"log/slog"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// issueQueue is IS-IS's report queue. runISISPlugin starts it before the engine
// starts and MUST stop it after the engine has shut down. Until it starts the
// pointer holds nil, whose queue refuses each report with
// issuequeue.ErrNotStarted: only a test that drives the engine without the SDK
// raises then.
var issueQueue atomic.Pointer[issuequeue.Queue]

// internalDefectRaiseFailed is the failed callback of an internal-defect
// raise: the daemon did not take it, so show errors does not list the defect,
// and only this line and the problem's own WARN line hold it.
func internalDefectRaiseFailed(log *slog.Logger, subject, fix string) func(error) {
	return func(err error) {
		log.Warn("isis could not raise an internal defect on the daemon's report bus, so show errors does not list it",
			"code", diagnostic.CodeDoctorInternalDefect, "subject", subject, "error", err, "fix", fix)
	}
}
