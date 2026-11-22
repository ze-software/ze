package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/host/trend"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/privilege"
)

// VALIDATES: AC-19, the show host trend half -- with no trend monitor
// running, show host trend answers an operator error that names the monitor,
// says why it matters and ends on the restart command, and never answers an
// empty list.
// Method: the handler is called while no monitor is published (no test of
// this package leaves one running), with ze fixed under ze.service, and the
// answer's status, data and error lines are read.
func TestShowHostTrendRefusesWhenMonitorNotRunning(t *testing.T) {
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
	if _, err := trend.Rules(); !errors.Is(err, trend.ErrNotRunning) {
		t.Fatalf("precondition: a trend monitor is published in this test binary (Rules error %v)", err)
	}
	resp, err := handleShowTrend(nil, command.ValidatedArgs{})
	if err != nil {
		t.Fatalf("handleShowTrend returned a Go error, want an operator error answer: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status = %q, want %q", resp.Status, plugin.StatusError)
	}
	if resp.Data != nil {
		t.Errorf("a refusal carries data %#v, want none", resp.Data)
	}
	lines := strings.Split(resp.Error, "\n")
	if len(lines) != 3 {
		t.Fatalf("error has %d lines, want problem, why and fix:\n%s", len(lines), resp.Error)
	}
	if lines[0] != trendNotRunningProblem {
		t.Errorf("problem line = %q, want %q", lines[0], trendNotRunningProblem)
	}
	if !strings.Contains(lines[1], "kernel memory") {
		t.Errorf("why line = %q, want it to say what is not watched", lines[1])
	}
	if lines[2] != "fix: sudo systemctl restart ze" {
		t.Errorf("fix line = %q, want %q", lines[2], "fix: sudo systemctl restart ze")
	}
}

// VALIDATES: the show host trend refusal names the restart for this process:
// `sudo systemctl restart ze` when ze runs under ze.service, and the
// stop-and-start steps (ze signal stop, then ze start) for a ze started another
// way, with no systemctl. Neither names `ze signal restart`, which exits
// without starting a new process.
// Method: fix the membership through privilege.SetServiceMembershipForTest and
// read the fix of the refusal over ErrNotRunning.
// PREVENTS: an operator whose ze runs outside a supervisor told to run a
// command that stops ze and leaves the trend monitor stopped with it.
func TestShowHostTrendRefusalNamesTheRestartForThisProcess(t *testing.T) {
	t.Run("ze.service", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
		if got := trendReadRefusal(trend.ErrNotRunning).Fix(); got != "sudo systemctl restart ze" {
			t.Fatalf("under ze.service the fix is %q, want %q", got, "sudo systemctl restart ze")
		}
	})
	t.Run("other", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
		got := trendReadRefusal(trend.ErrNotRunning).Fix()
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

// VALIDATES: a trend read failing for a reason other than a stopped monitor is
// never reported as one: it keeps its cause and does not name the restart.
// Method: trendReadRefusal over a foreign error and over ErrNotRunning.
func TestShowHostTrendRefusalKeepsForeignCause(t *testing.T) {
	foreign := errors.New("snapshot torn")
	refusal := trendReadRefusal(foreign)
	if !errors.Is(refusal, foreign) {
		t.Errorf("refusal %q does not keep its cause", refusal)
	}
	if strings.Contains(refusal.Error(), trend.NotRunningFix()) {
		t.Errorf("refusal %q names the restart for a monitor that may be running", refusal)
	}
	if got := trendReadRefusal(trend.ErrNotRunning).Error(); !strings.HasSuffix(got, "fix: "+trend.NotRunningFix()) {
		t.Errorf("not-running refusal %q does not end on the restart", got)
	}
}
