package siteterminaldemo

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: consecutive load confirmations remain observable when the terminal
// renderer emits only the changed filename cells and a delete-character command.
func TestPTYWaitMatchesDifferentialStatus(t *testing.T) {
	const prefix = "Configuration merged from /src/demos/terminal/topics/bfd/"
	program := fmt.Sprintf(`stty -echo; printf 'READY\n'; read first; printf '\033[5;1H%s1-profile.conf'; read second; printf '\033[5;%dH2-peer.\033[3P'; read third; printf '\r\ngot:%%s\r\n' "$third"`, prefix, len(prefix)+1)
	var stdout, stderr bytes.Buffer
	code := RunPTY([]string{
		"--ready", "READY", "--timeout", "2",
		"--command", "first",
		"--command", `@wait merged from /src/demos/terminal/topics/bfd/1-profile\.conf`,
		"--command", "second",
		"--command", `@wait merged from /src/demos/terminal/topics/bfd/2-peer\.conf`,
		"--command", "done",
		"--", "sh", "-c", program,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunPTY code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "got:done") {
		t.Fatalf("the command after the second confirmation was not processed: %q", stdout.String())
	}
}

// PREVENTS: a dashboard check that waits, after the `q` that leaves
// `monitor bgp`, for a row that comes back with the text it had before the
// dashboard opened. The showcase waited for `operational]` there and timed
// out with only `► dashboard stopped` among the changed rows.
// Method: the rows a command session counts as changed when the CLI returns
// from a peer table that never painted the mode line or the prompt, matched
// against the wait each dashboard session places after its `@type q`. The
// frames are written by hand from what Model.View renders (message line, mode
// line, prompt); the repository holds no recording of them.
// MUTATION: `@wait operational\]` in place of dashboardReturnWait in
// showcaseDashboardSession or cliDashboardSession turns its subtest red.
func TestDashboardSessionsWaitForWhatTheReturnPaints(t *testing.T) {
	const modeLine, prompt = "\x1b[39;1HZe Editor [operational]", "\x1b[40;1Hze> "
	screen := newTerminalScreen(40, 120)
	screen.settle(modeLine + prompt)
	screen.settle("\x1b[2;3HPeer       ASN ^  State\x1b[3;3H127.0.0.2  65001  established")
	window := len(screen.history)
	screen.settle("\x1b[38;1H► dashboard stopped" + modeLine + prompt)
	changed := strings.Join(screen.history[window:], "\n")
	if strings.Contains(changed, "operational]") {
		t.Fatalf("the unchanged mode line counts as changed, so this test judges nothing: %q", changed)
	}
	sessions := map[string][]string{
		"showcase":      showcaseDashboardSession(),
		"cli-dashboard": cliDashboardSession(),
	}
	for name, arguments := range sessions {
		t.Run(name, func(t *testing.T) {
			var commands []string
			for index := 0; index+1 < len(arguments); index++ {
				if arguments[index] == "--command" {
					commands = append(commands, arguments[index+1])
				}
			}
			quit := slices.Index(commands, "@type q")
			if quit < 0 || quit+1 >= len(commands) {
				t.Fatalf("the session does not quit the dashboard and then wait: %q", commands)
			}
			directive, argument := splitPTYDirective(commands[quit+1])
			if directive != tapeWaitDirective {
				t.Fatalf("the command after the quit is %q, not a wait", commands[quit+1])
			}
			if !regexp.MustCompile(argument).MatchString(changed) {
				t.Fatalf("the wait %q after the quit matches no row the return changes: %q", argument, changed)
			}
		})
	}
}

// VALIDATES: fresh output elsewhere on the screen cannot reuse an old success
// status as proof that the new command completed.
func TestPTYWaitRefusesUnchangedPriorStatus(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunPTY([]string{
		"--ready", "READY", "--timeout", "1",
		"--command", "work",
		"--command", "@wait COMMITTED",
		"--command", "done",
		"--", "sh", "-c", `stty -echo; printf '\033[5;1HCOMMITTED\033[1;1HREADY'; read first; printf '\033[10;1Hworking'; read second; printf 'incorrect stale match\n'`,
	}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), `timed out waiting for "COMMITTED"`) {
		t.Fatalf("an unchanged prior status satisfied the new wait: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
