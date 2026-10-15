package sdk

import (
	"slices"
	"strings"
	"testing"
)

// TestRestartStepsNamesEveryRouteAndNeverSignalRestart pins the restart advice.
// Method: split RestartSteps into lines and check each supervisor's route is on
// its own line, then walk the words: no `signal restart` (it leaves ze stopped)
// and no `reboot` that is not `request reboot` or `signal reboot` (a bare
// reboot is not a ze CLI command).
func TestRestartStepsNamesEveryRouteAndNeverSignalRestart(t *testing.T) {
	text := RestartSteps()
	if strings.HasSuffix(text, "\n") {
		t.Fatalf("RestartSteps ends with a newline: %q", text)
	}
	lines := strings.Split(text, "\n")
	if len(lines) != 4 {
		t.Fatalf("RestartSteps has %d lines, want 4:\n%s", len(lines), text)
	}
	wantByLine := [][]string{
		{"stop the ze daemon and start it again"},
		{"under systemd:", "systemctl restart ze"},
		{"without systemd:", "ze signal stop", "(ze start,", "ze start <config-file>"},
		{"on the ze appliance:", "request reboot in the ze CLI", "ze signal reboot from a shell"},
	}
	for i, wants := range wantByLine {
		for _, want := range wants {
			if !strings.Contains(lines[i], want) {
				t.Errorf("line %d %q does not name %q", i+1, lines[i], want)
			}
		}
	}

	words := strings.Fields(text)
	for i, word := range words {
		if i == 0 {
			continue
		}
		previous := words[i-1]
		if previous == "signal" && strings.HasPrefix(word, "restart") {
			t.Errorf("RestartSteps names ze signal restart, which leaves ze stopped:\n%s", text)
		}
		if word != "reboot" {
			continue
		}
		if previous != "request" && previous != "signal" {
			t.Errorf("RestartSteps names a bare reboot after %q:\n%s", previous, text)
		}
	}
}

// TestRunningConfigLostNextNamesTheDaemonRestart pins the fix a lost-listener
// error gives. Method: check the text says the restart takes the whole daemon
// down, carries each restart route of RestartSteps on a line of its own, and
// never names `ze signal restart`.
//
// VALIDATES: review round 4 ISSUE-2. No command restarts one plugin, so the
// fix is a daemon restart, and the operator is told what it costs.
// PREVENTS: advice to run `ze signal restart`, which leaves ze stopped under
// systemd (it exits cleanly and the unit restarts only on failure), with no
// word that every protocol and session goes down with it.
func TestRunningConfigLostNextNamesTheDaemonRestart(t *testing.T) {
	text := RunningConfigLostNext
	if strings.Contains(text, "signal restart") {
		t.Errorf("RunningConfigLostNext names ze signal restart, which leaves ze stopped:\n%s", text)
	}
	for _, want := range []string{"No command restarts one plugin", "every protocol", "every session"} {
		if !strings.Contains(text, want) {
			t.Errorf("RunningConfigLostNext does not say %q:\n%s", want, text)
		}
	}
	lines := strings.Split(text, "\n")
	for _, route := range restartRouteLines {
		if !slices.Contains(lines, route) {
			t.Errorf("RunningConfigLostNext does not carry the route %q on its own line:\n%s", route, text)
		}
	}
}
