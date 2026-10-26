package siteterminaldemo

import (
	"bytes"
	"fmt"
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
