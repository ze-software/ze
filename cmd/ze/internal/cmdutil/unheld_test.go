// Design: docs/architecture/api/commands.md -- the route of a path this binary does not hold

package cmdutil

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
)

// unheldCall is what one stubbed run of the route received.
type unheldCall struct {
	line   string
	format string
}

// unheldResolution resolves argv against the real `show` tree and fails the
// test unless the path is one the tree does not hold, which every test of
// this file is about.
func unheldResolution(t *testing.T, args []string) Resolution {
	t.Helper()
	res, ok := resolveForTest(t, args, "show")
	if !ok {
		t.Fatalf("ResolveCommand(%q) answered no path", args)
	}
	if res.Valid || res.dispatchable() {
		t.Fatalf("%q is held by this binary's tree: the fixture must name a path it does not hold", args)
	}
	return res
}

// reachedRoute is a route whose ze answers: run records the line and answers
// code, or calls notUnderstood with message when message is set. The stored
// config is never read: a test that reaches it fails.
func reachedRoute(t *testing.T, calls *[]unheldCall, code int, message string, stderr *bytes.Buffer) unheldRoute {
	t.Helper()
	return unheldRoute{
		reach: func() (sshclient.Reached, error) { return sshclient.Reached{Address: "127.0.0.1:1"}, nil },
		run: func(_ sshclient.Reached, line, format string, notUnderstood func(string) int) int {
			*calls = append(*calls, unheldCall{line: line, format: format})
			if message != "" {
				return notUnderstood(message)
			}
			return code
		},
		declared: func() ([]string, error) {
			t.Errorf("the stored config was read although a ze answered")
			return nil, nil
		},
		stderr: stderr,
	}
}

// unreachedRoute is a route whose walk stops at step, with the declared
// plugins and read error of the stored config. run fails the test.
func unreachedRoute(t *testing.T, step sshclient.ReachStep, plugins []string, readErr error, stderr *bytes.Buffer) unheldRoute {
	t.Helper()
	return unheldRoute{
		reach: func() (sshclient.Reached, error) {
			return sshclient.Reached{}, &sshclient.Unreached{
				Step: step, Address: "127.0.0.1:1", Cause: errors.New("connection refused"),
			}
		},
		run: func(sshclient.Reached, string, string, func(string) int) int {
			t.Errorf("a line was run although no ze answered")
			return 0
		},
		declared: func() ([]string, error) { return plugins, readErr },
		stderr:   stderr,
	}
}

// todayUnknown is the refusal `ze show <path>` gave for a path the tree does
// not hold before a running ze was asked, for a path whose first word the
// tree suggests nothing for (cmdutil.go before this route existed). The test
// that uses it fails if the tree starts suggesting a word for the fixture.
func todayUnknown(t *testing.T, res Resolution, args []string) string {
	t.Helper()
	if suggestion := SuggestFromTree(res.Relative[0], res.Tree); suggestion != "" {
		t.Fatalf("the tree suggests %q for %q: pick a word it suggests nothing for", suggestion, res.Relative[0])
	}
	return "error: unknown command: " + strings.Join(args, " ") + "\n" +
		"hint: run 'ze show help' for available commands\n"
}

// VALIDATES: AC-36: a path the tree does not hold is sent to the running ze
// as the line a compiled-in command sends, and the ze's exit code is the
// answer, with nothing written by this route.
// PREVENTS: `ze show acme-monitor target probe-b` answering "unknown command"
// while the running ze serves it.
func TestUnheldPathIsAnsweredByTheRunningZe(t *testing.T) {
	args := []string{"show", "acme-monitor", "target", "probe-b"}
	res := unheldResolution(t, args)
	var calls []unheldCall
	var stderr bytes.Buffer

	code := reachedRoute(t, &calls, 7, "", &stderr).answer(res, args, "show")

	if code != 7 {
		t.Errorf("exit = %d, want the running ze's 7", code)
	}
	if len(calls) != 1 || calls[0].line != "show acme-monitor target probe-b" || calls[0].format != "" {
		t.Errorf("the ze was asked %+v, want one line `show acme-monitor target probe-b` with no format", calls)
	}
	if stderr.Len() != 0 {
		t.Errorf("the route wrote %q: the ze's answer is the whole answer", stderr.String())
	}
}

// VALIDATES: AC-38: a trailing format keyword travels as the format, and a
// pipe typed as words travels in the line, as for a compiled-in command.
// PREVENTS: `ze show acme-monitor status json` asking the ze for a command
// named `json`.
func TestUnheldPathCarriesFormatAndPipe(t *testing.T) {
	cases := []struct {
		args   []string
		line   string
		format string
	}{
		{[]string{"show", "acme-monitor", "status", "json"}, "show acme-monitor status", "json"},
		{[]string{"show", "acme-monitor", "status", "| count"}, "show acme-monitor status | count", ""},
	}
	for _, tc := range cases {
		res := unheldResolution(t, tc.args)
		var calls []unheldCall
		var stderr bytes.Buffer
		reachedRoute(t, &calls, 0, "", &stderr).answer(res, tc.args, "show")
		if len(calls) != 1 || calls[0].line != tc.line || calls[0].format != tc.format {
			t.Errorf("%q asked %+v, want line %q format %q", tc.args, calls, tc.line, tc.format)
		}
	}
}

// VALIDATES: a running ze that does not know the path either gives the
// unknown-command refusal of this binary, byte for byte, exit 1; a ze text
// that says more is shown beside it.
// PREVENTS: a typo answered with the bare "unknown command" of the daemon and
// no help line, or the daemon's explanation lost.
func TestUnheldPathTheRunningZeDoesNotKnow(t *testing.T) {
	args := []string{"show", "zzqqxxwv", "status"}
	res := unheldResolution(t, args)
	var calls []unheldCall
	var stderr bytes.Buffer

	code := reachedRoute(t, &calls, 0, "unknown command", &stderr).answer(res, args, "show")

	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if got, want := stderr.String(), todayUnknown(t, res, args); got != want {
		t.Errorf("refusal = %q, want %q", got, want)
	}

	stderr.Reset()
	calls = nil
	reachedRoute(t, &calls, 0, "plugin acme-monitor is restarting", &stderr).answer(res, args, "show")
	if !strings.Contains(stderr.String(), "hint: the running ze answered: plugin acme-monitor is restarting\n") {
		t.Errorf("the ze's own text is missing from %q", stderr.String())
	}
}

// VALIDATES: with no ze answering and a stored config that declares no
// external plugin, the refusal is byte for byte the one an unknown path had.
// PREVENTS: every typo of `ze <verb>` gaining a daemon message.
func TestUnheldPathNoZeNoExternalPluginIsUnchanged(t *testing.T) {
	args := []string{"show", "zzqqxxwv", "status"}
	res := unheldResolution(t, args)
	var stderr bytes.Buffer

	code := unreachedRoute(t, sshclient.ReachStepListener, nil, nil, &stderr).answer(res, args, "show")

	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if got, want := stderr.String(), todayUnknown(t, res, args); got != want {
		t.Errorf("refusal = %q, want %q", got, want)
	}
}

// VALIDATES: AC-37: with no ze answering and a stored config that declares an
// external plugin, the path is not available, may be the plugin's, and has one
// fix per step the walk stopped at, alone on the last line.
// PREVENTS: "unknown command" for a command the plugin serves once ze runs,
// and a fix that cannot work for the step that failed.
func TestUnheldPathNoZeExternalPluginDeclared(t *testing.T) {
	args := []string{"show", "acme-monitor", "status"}
	res := unheldResolution(t, args)
	cases := []struct {
		step  sshclient.ReachStep
		fix   string
		again bool
		why   string
	}{
		{sshclient.ReachStepListener, "fix: ze start", true, "no ze listens at 127.0.0.1:1 (connection refused)"},
		{sshclient.ReachStepAddress, "fix: ze status", true, "the address of ze's SSH listener could not be read"},
		{sshclient.ReachStepCredentials, `fix: ze cli --user <name> -c "show acme-monitor status"`, false,
			"a ze listens at 127.0.0.1:1, but this account has no credentials"},
	}
	for _, tc := range cases {
		var stderr bytes.Buffer
		code := unreachedRoute(t, tc.step, []string{"acme-monitor"}, nil, &stderr).answer(res, args, "show")
		out := stderr.String()
		if code != 1 {
			t.Errorf("step %d: exit = %d, want 1", tc.step, code)
		}
		for _, want := range []string{
			"error: show acme-monitor status is not available",
			"it may belong to an external plugin, which only a running ze holds",
			`the config declares external plugin "acme-monitor"`,
			tc.why,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("step %d: %q missing from %q", tc.step, want, out)
			}
		}
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if last := lines[len(lines)-1]; last != tc.fix {
			t.Errorf("step %d: last line = %q, want %q", tc.step, last, tc.fix)
		}
		if got := strings.Contains(out, "then `ze show acme-monitor status` again"); got != tc.again {
			t.Errorf("step %d: rerun sentence present = %v, want %v in %q", tc.step, got, tc.again, out)
		}
		if strings.Contains(out, "unknown command") {
			t.Errorf("step %d: a path that may be the plugin's is not refused as unknown: %q", tc.step, out)
		}
	}
}

// VALIDATES: a stored config that cannot be read leaves the unknown-command
// refusal in place and says what was not checked.
// PREVENTS: a failed read taken as "declares no external plugin" in silence.
func TestUnheldPathStoredConfigUnreadable(t *testing.T) {
	args := []string{"show", "zzqqxxwv", "status"}
	res := unheldResolution(t, args)
	var stderr bytes.Buffer

	code := unreachedRoute(t, sshclient.ReachStepListener, nil, errors.New("crc mismatch"), &stderr).answer(res, args, "show")

	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	out := stderr.String()
	if !strings.HasPrefix(out, todayUnknown(t, res, args)) {
		t.Errorf("the unknown-command refusal must come first: %q", out)
	}
	if !strings.HasSuffix(out, "hint: the stored config could not be read to learn whether it declares an external plugin (crc mismatch)\n") {
		t.Errorf("the unread config is not named: %q", out)
	}
}

// VALIDATES: RunCommand sends a path the tree does not hold to this route.
// PREVENTS: the route existing beside a RunCommand that still refuses the path
// itself.
func TestRunCommandSendsAnUnheldPathToTheRunningZe(t *testing.T) {
	saved := unheldPaths
	t.Cleanup(func() { unheldPaths = saved })
	var calls []unheldCall
	var stderr bytes.Buffer
	unheldPaths = reachedRoute(t, &calls, 5, "", &stderr)

	if code := RunCommand([]string{"show", "acme-monitor", "status"}, "show"); code != 5 {
		t.Errorf("RunCommand(`ze show acme-monitor status`) = %d, want the route's 5", code)
	}
	if len(calls) != 1 || calls[0].line != "show acme-monitor status" {
		t.Errorf("the route ran %+v, want `show acme-monitor status`", calls)
	}
}
