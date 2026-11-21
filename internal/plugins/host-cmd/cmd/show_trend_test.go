package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/host/trend"
	"github.com/ze-software/ze/internal/component/plugin"
)

// VALIDATES: AC-19, the show host trend half -- with no trend monitor
// running, show host trend answers an operator error that names the monitor,
// says why it matters and ends on the restart command, and never answers an
// empty list.
// Method: the handler is called while no monitor is published (no test of
// this package leaves one running), and the answer's status, data and error
// lines are read.
func TestShowHostTrendRefusesWhenMonitorNotRunning(t *testing.T) {
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
	if lines[2] != "fix: ze signal restart" {
		t.Errorf("fix line = %q, want %q", lines[2], "fix: ze signal restart")
	}
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
	if strings.Contains(refusal.Error(), trendNotRunningFix) {
		t.Errorf("refusal %q names the restart for a monitor that may be running", refusal)
	}
	if got := trendReadRefusal(trend.ErrNotRunning).Error(); !strings.HasSuffix(got, "fix: "+trendNotRunningFix) {
		t.Errorf("not-running refusal %q does not end on the restart", got)
	}
}
