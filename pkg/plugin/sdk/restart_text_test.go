package sdk

import (
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
