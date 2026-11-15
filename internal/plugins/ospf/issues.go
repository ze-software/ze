// Design: docs/architecture/ospf/ospf-4-component-config.md -- the problems OSPF raises on the daemon's report bus
// Related: register.go -- runOSPFEngine starts and stops the report queue
// Related: events.go -- warnEmitRefused raises through issueQueue
// Related: bfd_client.go -- warnBFDReleaseRefused raises through issueQueue
// Related: ipsec_install.go -- warnRemoval raises through issueQueue
//
// OSPF raises a transient problem on the daemon's report bus through the SDK
// (report-issue), so the raise reaches the daemon whether OSPF runs in it or
// forked. The process-local internal/core/report would keep a forked plugin's
// raise in that process's own copy of the bus, which no doctor check reads
// (docs/architecture/doctor-and-health-checks.md, "Event codes"). Every raise
// goes through the plugin report queue (internal/component/plugin/issuequeue)
// and never waits for the daemon: the IPsec installer raises with its lock
// held, and every received Hello takes that lock.
package ospf

import (
	"log/slog"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// issueQueue is OSPF's report queue. runOSPFEngine starts it before it builds an
// engine and MUST stop it after every engine has stopped. Until it starts the
// pointer holds nil, whose queue refuses each report with issuequeue.ErrNotStarted:
// only a test that drives an engine without the plugin entry raises then.
var issueQueue atomic.Pointer[issuequeue.Queue]

// internalDefectRaiseFailed is the failed callback of an internal-defect raise: the
// daemon did not take it, so show errors does not list the defect, and only this line
// and the problem's own WARN line hold it.
func internalDefectRaiseFailed(log *slog.Logger, subject, fix string) func(error) {
	return func(err error) {
		log.Warn("ospf could not raise an internal defect on the daemon's report bus, so show errors does not list it",
			"code", diagnostic.CodeDoctorInternalDefect, "subject", subject, "error", err, "fix", fix)
	}
}

// ipsecEntryLeftRaiseFailed is the failed callback of a codeOSPFv3IPsecEntryLeft raise:
// the daemon did not take it, so ze doctor does not list the entry Ze left in the
// kernel, and only this line and the removal's own WARN line hold it.
func ipsecEntryLeftRaiseFailed(log *slog.Logger, subject, fix string) func(error) {
	return func(err error) {
		log.Warn("ospf could not raise an IPsec entry left in the kernel on the daemon's report bus, so ze doctor does not list it",
			"code", codeOSPFv3IPsecEntryLeft, "entry", subject, "error", err, "fix", fix)
	}
}
