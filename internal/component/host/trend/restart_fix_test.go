// Design: docs/architecture/doctor-and-health-checks.md -- host trend checks
// Related: register.go -- NotRunningFix, the fix of the host-trend-monitor check

package trend

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
)

// VALIDATES: the doctor-host-trend-not-running finding names the restart for this
// process: `sudo systemctl restart ze` when ze runs under ze.service, and the
// stop-and-start steps (ze signal stop, then ze start) for a ze started another way,
// with no systemctl. Neither names `ze signal restart`, which exits without starting
// a new process.
// METHOD: fix the membership through privilege.SetServiceMembershipForTest, then run
// the registered host-trend-monitor check with no monitor running and read its fix.
// PREVENTS: an operator whose ze runs outside a supervisor told to run a command that
// stops ze and leaves the host unwatched, or told to run systemctl for a ze systemd
// did not start.
func TestTrendNotRunningFixNamesTheRestartForThisProcess(t *testing.T) {
	isolateTrend(t, nil)
	monitorCheck := registeredRuntimeCheck(t, monitorCheckName)
	fix := func(t *testing.T) string {
		t.Helper()
		rows := monitorCheck.Check(diagnostic.DoctorCheckContext{})
		if len(rows) != 1 || rows[0].Code != CodeNotRunning {
			t.Fatalf("monitor check rows = %+v, want one %s row", rows, CodeNotRunning)
		}
		return rows[0].Fix
	}

	t.Run("ze.service", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
		if got := fix(t); got != "sudo systemctl restart ze" {
			t.Fatalf("under ze.service the fix is %q, want %q", got, "sudo systemctl restart ze")
		}
	})
	t.Run("other", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
		got := fix(t)
		if strings.Contains(got, "systemctl") {
			t.Errorf("outside ze.service the fix names systemctl: %q", got)
		}
		if strings.Contains(got, "ze signal restart") {
			t.Errorf("the fix names ze signal restart, which leaves ze stopped: %q", got)
		}
		for _, want := range []string{"ze signal stop", "ze start"} {
			if !strings.Contains(got, want) {
				t.Errorf("outside ze.service the fix does not name %q: %q", want, got)
			}
		}
	})
}
