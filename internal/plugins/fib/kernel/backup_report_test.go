// Design: docs/guide/ospf.md -- fast reroute: the kernel carries the primary path only
// Related: backup_report.go -- reportBackupsLocked, the warning under test

package fibkernel

import (
	"net/netip"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/report"
)

// backupWarning returns the active backup-not-programmed warning, if any.
func backupWarning() (report.Issue, bool) {
	for _, w := range report.Warnings() {
		if w.Source == reportSourceFIB && w.Code == reportCodeFIBBackupNotProgrammed {
			return w, true
		}
	}
	return report.Issue{}, false
}

// Goal: the fast-reroute backups the kernel does not carry are never silent.
// While any installed route holds a backup, one warning says how many; it
// follows installs, replaces and withdrawals, and clears when none is left.
// Method: drive processEvent over the rich mock backend through a sequence of
// changes and read the report bus after each batch.
func TestFIBBackupNotProgrammedWarningFollowsInstalledRoutes(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	protected := netip.MustParsePrefix("10.20.0.0/24")
	plain := netip.MustParsePrefix("10.21.0.0/24")
	backup := []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.9"), Labels: []uint32{16010, 24003}}}
	f := newFIBKernel(newRichMockBackend())
	apply := func(changes ...incomingChange) { f.processEvent(makeSysribPayload(changes)) }
	wantRoutes := func(step string, want int) {
		t.Helper()
		w, raised := backupWarning()
		if want == 0 {
			if raised {
				t.Fatalf("%s: warning still raised: %+v", step, w)
			}
			return
		}
		if !raised {
			t.Fatalf("%s: no %s warning while %d routes hold a backup", step, reportCodeFIBBackupNotProgrammed, want)
		}
		if got := w.Detail[keyBackupRoutes]; got != want {
			t.Fatalf("%s: warning counts %v routes, want %d", step, got, want)
		}
	}

	apply(incomingChange{Action: routeaction.Add, Prefix: protected, NextHop: netip.MustParseAddr("10.0.0.2"), Backup: backup})
	wantRoutes("protected route installed", 1)

	apply(incomingChange{Action: routeaction.Add, Prefix: plain, NextHop: netip.MustParseAddr("10.0.0.3"), Metric: 5})
	wantRoutes("unprotected route installed", 1)

	apply(incomingChange{Action: routeaction.Update, Prefix: protected, NextHop: netip.MustParseAddr("10.0.0.2"), Metric: 5})
	wantRoutes("backup replaced away", 0)

	apply(incomingChange{Action: routeaction.Update, Prefix: plain, NextHop: netip.MustParseAddr("10.0.0.3"), Backup: backup})
	wantRoutes("backup replaced in", 1)

	apply(incomingChange{Action: routeaction.Withdraw, Prefix: plain})
	wantRoutes("protected route withdrawn", 0)
}
