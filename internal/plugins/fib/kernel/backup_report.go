// Design: docs/guide/ospf.md -- fast reroute: the kernel carries the primary path only
// Related: nexthop_linux.go -- buildRichRoute, which sends the primary path(s) alone
// Related: fibkernel.go -- processEvent, sweepStale and flushRoutes keep the set below
// Related: register.go -- the init() that registers the code below
//
// A routing protocol may hand this plugin a fast-reroute backup next hop beside
// a route's primary path (RFC 5286 LFA, TI-LFA). The kernel holds no standby
// member: Linux refuses RTNH_F_LINKDOWN from userspace (net/ipv4/fib_semantics.c,
// fib_get_nhs), and an unflagged member would load-share in steady state. So the
// backup stays in Ze's own state and is not programmed, and one warning on the
// report bus says how many installed routes hold a backup the kernel does not
// carry, rather than one log line per route on the install path.

package fibkernel

import (
	"strconv"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// reportCodeFIBBackupNotProgrammed is the report-bus code of the warning, and
// backupReportSubject the one subject it is raised under.
const (
	reportCodeFIBBackupNotProgrammed = "doctor-fib-backup-not-programmed"
	backupReportSubject              = "fast-reroute"
	keyBackupRoutes                  = "routes"
)

// fibBackupDiagnosticCode describes the code for `ze show diagnostic`. The
// event-code registration in register.go requires it to be registered first.
var fibBackupDiagnosticCode = diagnostic.CodeMeta{
	Code:  reportCodeFIBBackupNotProgrammed,
	Title: "Fast-reroute backups are not programmed in the kernel",
	Description: "A routing protocol computed a fast-reroute backup next hop (LFA or TI-LFA) for one or more " +
		"installed routes. The kernel carries only the primary path of these routes. The backup is not used " +
		"when the primary link fails: traffic recovers only when the routing protocol converges again. " +
		"The backups are shown by ze show ospf route fast-reroute.",
	Examples: []string{"ze show warnings source fib", "ze show diagnostic " + reportCodeFIBBackupNotProgrammed},
}

// trackBackupLocked records whether the route just programmed for pfx holds a
// backup the kernel does not carry. The caller holds f.mu and calls it only
// after the kernel accepted the route.
func (f *fibKernel) trackBackupLocked(pfx string, c *incomingChange) {
	if len(c.Backup) > 0 {
		f.backups[pfx] = struct{}{}
		return
	}
	delete(f.backups, pfx)
}

// reportBackupsLocked raises the warning while any installed route holds a
// backup the kernel does not carry, and clears it when none does. It reaches
// the report bus only when the count changed, so a batch that changes no
// backup costs one comparison. The caller holds f.mu.
func (f *fibKernel) reportBackupsLocked() {
	held := len(f.backups)
	if held == f.backupsReported {
		return
	}
	f.backupsReported = held
	if held == 0 {
		report.ClearWarning(reportSourceFIB, reportCodeFIBBackupNotProgrammed, backupReportSubject)
		return
	}
	report.RaiseWarning(reportSourceFIB, reportCodeFIBBackupNotProgrammed, backupReportSubject,
		strconv.Itoa(held)+" installed routes have a fast-reroute backup that the kernel does not carry. "+
			"Only the primary path is programmed, so a primary link failure is not repaired until the routing protocol converges again.",
		map[string]any{keyBackupRoutes: held})
}
