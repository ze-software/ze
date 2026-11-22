package sdk

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/privilege"
)

// restartMembershipCases are the three answers privilege.DaemonServiceMembership
// gives, with what the restart advice MUST say on its route line for each, and
// what it MUST NOT say anywhere.
var restartMembershipCases = []struct {
	name       string
	membership privilege.ServiceMembership
	err        error
	route      []string
	absent     []string
}{
	{
		name:       "under ze.service",
		membership: privilege.MembershipZeService,
		route:      []string{"sudo systemctl restart ze"},
		absent:     []string{"no single command", "could not tell"},
	},
	{
		name:       "started another way",
		membership: privilege.MembershipOther,
		route:      []string{"ze signal stop", "(ze start,", "ze start <config-file>"},
		absent:     []string{"systemctl", "could not tell"},
	},
	{
		name:       "membership unknown",
		membership: privilege.MembershipUnknown,
		err:        errors.New("no cgroup v2 line"),
		route: []string{
			"Ze could not tell whether it runs under ze.service", "no cgroup v2 line",
			"ze signal stop", "(ze start,", "ze start <config-file>",
		},
		absent: []string{"systemctl"},
	},
}

// assertRestartRoutes checks that text carries the route for the membership of
// case tc on an indented line of its own, the appliance route on the line
// after it, nothing the case's absent list names, no `ze signal restart`, and
// no bare `reboot`.
func assertRestartRoutes(t *testing.T, text string, tc int) {
	t.Helper()
	want := restartMembershipCases[tc]
	lines := strings.Split(text, "\n")

	routeLine := -1
	for i, line := range lines {
		if strings.Contains(line, want.route[0]) {
			routeLine = i
			break
		}
	}
	if routeLine < 0 {
		t.Fatalf("no line names %q:\n%s", want.route[0], text)
	}
	if !strings.HasPrefix(lines[routeLine], "  ") {
		t.Errorf("the route line %q is not indented as a route", lines[routeLine])
	}
	for _, route := range want.route {
		if !strings.Contains(lines[routeLine], route) {
			t.Errorf("the route line %q does not name %q", lines[routeLine], route)
		}
	}
	if routeLine+1 >= len(lines) {
		t.Fatalf("no appliance route follows the route line:\n%s", text)
	}
	appliance := lines[routeLine+1]
	for _, route := range []string{"  on the ze appliance:", "request reboot in the ze CLI", "ze signal reboot from a shell"} {
		if !strings.Contains(appliance, route) {
			t.Errorf("the appliance line %q does not name %q", appliance, route)
		}
	}
	for _, absent := range want.absent {
		if strings.Contains(text, absent) {
			t.Errorf("the advice names %q, which this membership must not get:\n%s", absent, text)
		}
	}

	words := strings.Fields(text)
	for i, word := range words {
		if i == 0 {
			continue
		}
		previous := words[i-1]
		if previous == "signal" && strings.HasPrefix(word, "restart") {
			t.Errorf("the advice names ze signal restart, which leaves an unsupervised ze stopped:\n%s", text)
		}
		if word != "reboot" {
			continue
		}
		if previous != "request" && previous != "signal" {
			t.Errorf("the advice names a bare reboot after %q:\n%s", previous, text)
		}
	}
}

// TestRestartStepsNamesEveryRouteAndNeverSignalRestart pins the restart advice
// per service membership. Method: fix the membership through
// privilege.SetServiceMembershipForTest, then check RestartSteps is three
// lines (the header, the route for this process, the appliance route) with no
// trailing newline, that the route line names the systemd command only under
// ze.service, and walk the words: no `signal restart` and no `reboot` that is
// not `request reboot` or `signal reboot` (a bare reboot is not a ze CLI
// command). Not parallel: the seam is process-wide.
//
// VALIDATES: Thomas's rule that a systemd repair is offered only when Ze can
// establish that the process belongs to the Ze service: under ze.service the
// route is sudo systemctl restart ze; started another way it is the stop and
// start steps with no systemctl; when the membership cannot be read it says Ze
// could not tell, with the reason, and names no systemctl either.
// PREVENTS: an unconditional "systemctl restart ze" told to a ze no unit
// started, which changes nothing for it, and advice to run `ze signal restart`.
func TestRestartStepsNamesEveryRouteAndNeverSignalRestart(t *testing.T) {
	for i, tc := range restartMembershipCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(tc.membership, tc.err))

			text := RestartSteps()
			if strings.HasSuffix(text, "\n") {
				t.Fatalf("RestartSteps ends with a newline: %q", text)
			}
			lines := strings.Split(text, "\n")
			if len(lines) != 3 {
				t.Fatalf("RestartSteps has %d lines, want 3:\n%s", len(lines), text)
			}
			if !strings.Contains(lines[0], "stop the ze daemon and start it again") {
				t.Errorf("line 1 %q does not say to stop the ze daemon and start it again", lines[0])
			}
			if !strings.Contains(lines[1], tc.route[0]) {
				t.Errorf("line 2 %q is not the route for this process, %q", lines[1], tc.route[0])
			}
			assertRestartRoutes(t, text, i)
		})
	}
}

// TestRunningConfigLostRestartNamesTheDaemonRestart pins the restart every
// running-config-loss fix ends with, per service membership. Method: fix the
// membership through privilege.SetServiceMembershipForTest, then check the
// text says the restart takes the whole daemon down, carries the route for
// that membership and the appliance route on lines of their own, and never
// names `ze signal restart`. Not parallel: the seam is process-wide.
//
// VALIDATES: review round 4 ISSUE-2. No command restarts one plugin, so the
// fix is a daemon restart, and the operator is told what it costs; the route
// follows the service membership as RestartSteps does.
// PREVENTS: advice to run `ze signal restart`, which leaves an unsupervised ze
// stopped, a systemd command told to a ze no unit started, and no word that
// every protocol and session goes down with the restart.
func TestRunningConfigLostRestartNamesTheDaemonRestart(t *testing.T) {
	for i, tc := range restartMembershipCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(tc.membership, tc.err))

			text := RunningConfigLostRestart()
			if strings.Contains(text, "signal restart") {
				t.Errorf("RunningConfigLostRestart names ze signal restart, which leaves an unsupervised ze stopped:\n%s", text)
			}
			for _, want := range []string{"No command restarts one plugin", "every protocol", "every session"} {
				if !strings.Contains(text, want) {
					t.Errorf("RunningConfigLostRestart does not say %q:\n%s", want, text)
				}
			}
			assertRestartRoutes(t, text, i)
			// Each route is a whole line, exactly as RestartSteps writes it:
			// one copy of the advice, never a route line carrying extra text.
			lines := strings.Split(text, "\n")
			for _, route := range []string{"  " + privilege.RestartRemedy(), applianceRestartRoute} {
				if !slices.Contains(lines, route) {
					t.Errorf("RunningConfigLostRestart does not carry the route %q on a line of its own:\n%s", route, text)
				}
			}
		})
	}
}
