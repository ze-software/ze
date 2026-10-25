// Design: docs/contributing/running-commands.md -- le's exit codes
// Related: le/root/exitcode.go -- the table `le help ai exit-codes` prints

package le

import (
	"os"
	"path/filepath"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/test/cli"
)

// TestAUsageErrorAndAnExecutionFailureAnswerDifferentCodes validates AC-28 and
// the could-not-run half of AC-29 through the real dispatcher: an unknown
// command and a word a namespace does not hold are usage errors and answer 2,
// and a gate run outside a checkout could not run and answers 3, so a caller
// tells "you typed it wrong" from "the tree is not there" by the code alone.
// Method: dispatch each line as `le` would and read the code.
func TestAUsageErrorAndAnExecutionFailureAnswerDifferentCodes(t *testing.T) {
	for _, row := range []struct {
		line []string
		code int
		why  string
	}{
		{line: []string{"no-such-command-p5"}, code: 2, why: "an unknown command"},
		{line: []string{"ai", "no-such-member-p5"}, code: 2, why: "a word the ai namespace does not hold"},
		{line: []string{"cli", "grammar", "check", "stray-p5"}, code: 2, why: "a stray word after a verb"},
	} {
		if _, code := dispatchToStdout(t, row.line...); code != row.code {
			t.Errorf("%s (le %v) answered %d, want %d", row.why, row.line, code, row.code)
		}
	}

	t.Chdir(t.TempDir())
	if _, code := dispatchToStdout(t, "cli", "grammar", "check"); code != 3 {
		t.Errorf("le cli grammar check outside a checkout answered %d, want could-not-run 3", code)
	}
}

// TestEveryStrayWordRefusalComesFromTheTable validates AC-30 after
// D-P3-refuseargument: no area answers a stray word through a refusal of its
// own. For every action of every tabled area, the line `<verb> <stray word>`
// (after a positional value where the action takes one) answers the usage code
// 2, and its stderr is word for word the refusal a reference area built from
// the published rows alone writes. An area that refused the word in its own
// words, or with another code, is a refusal path outside the table.
// Method: the probes of TestEveryPublishedVerbIsDispatchedThroughItsOwnTable,
// reduced to the stray-word shape and held to the code D-Q8 names rather than
// to whatever code the reference answers.
func TestEveryStrayWordRefusalComesFromTheTable(t *testing.T) {
	probed := 0
	for _, tool := range commandsAtStart {
		list, declared := leroot.ActionsOf(tool.Name)
		if !declared {
			continue
		}
		// A pass-through area's words are its program's: TestPassThroughAreasKeepTheirProgramsCode.
		if list.PassesWords {
			continue
		}
		handler := leroot.LookupCommand(tool.Name)
		if handler == nil {
			t.Errorf("area %q publishes a table and registers no handler", tool.Name)
			continue
		}
		reference := referenceArea(t, list)
		for _, row := range list.Actions {
			probes := probesFor(row)
			// A forwarding row, or a positional argv, takes every word as data.
			if len(probes) == 0 {
				continue
			}
			stray := probes[0]
			probed++
			gotText, gotCode := probeAnswer(judged(t, tool.Name, handler), stray)
			if gotCode != 2 {
				t.Errorf("le %s %v answered %d for a stray word, want the usage code 2", tool.Name, stray, gotCode)
			}
			answerText, _ := probeAnswer(reference.Answer, stray)
			sweepText, _ := probeAnswer(func(args []string) (any, int) {
				return reference.Sweep(args, leaction.RunEveryAction)
			}, stray)
			if gotText != answerText && gotText != sweepText {
				t.Errorf("le %s %v refused a stray word outside its table:\n got   %q\n table %q\n sweep %q",
					tool.Name, stray, gotText, answerText, sweepText)
			}
		}
	}
	if probed == 0 {
		t.Fatal("no tabled action was probed: the test judged nothing")
	}
}

// TestPassThroughAreasKeepTheirProgramsCode validates AC-32 and R-14 through a
// real pass-through area: `le test json-ip-bytes` hands every word to its
// helper, and the helper's exit code is the command's, including a code le
// itself would spell differently (the helper answers 1 for input it cannot
// decode, where D-Q8 would say 3) and the helper's own 2 for a flag it does
// not know, which le's table would otherwise refuse as a dash-leading word.
// Method: run each line through the le dispatcher and through the helper
// directly, over the same stdin, and require one code from both.
func TestPassThroughAreasKeepTheirProgramsCode(t *testing.T) {
	if !leroot.Forwards("test json-ip-bytes") {
		t.Fatal("le test json-ip-bytes is not a pass-through area: the probe no longer reaches a program")
	}
	for _, row := range []struct {
		words []string
		stdin string
		code  int
		why   string
	}{
		{words: []string{"--ip", "192.0.2.1"}, stdin: "{}", code: 0, why: "a document the helper totals"},
		{words: []string{"--ip", "192.0.2.1"}, stdin: "not json", code: 1, why: "input the helper cannot decode"},
		{words: []string{"--ip", "192.0.2.1", "stray"}, stdin: "{}", code: 2, why: "a positional word the helper refuses"},
		{words: []string{"--no-such-flag"}, stdin: "{}", code: 2, why: "a flag the helper does not know"},
	} {
		direct := 0
		withStdin(t, row.stdin, func() {
			withStdoutDiscarded(t, func() { direct = cli.CmdJSONIPBytes(row.words) })
		})
		viaLe := 0
		withStdin(t, row.stdin, func() {
			_, viaLe = dispatchToStdout(t, append([]string{"test", "json-ip-bytes"}, row.words...)...)
		})
		if direct != row.code {
			t.Errorf("%s: the helper itself answered %d, want %d", row.why, direct, row.code)
		}
		if viaLe != direct {
			t.Errorf("%s: le test json-ip-bytes %v answered %d, the helper %d: le rewrote its program's code",
				row.why, row.words, viaLe, direct)
		}
	}
}

// withStdin runs fn with os.Stdin reading text from a file.
func withStdin(t *testing.T, text string, fn func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write the stdin file: %v", err)
	}
	file, err := os.Open(path) //nolint:gosec // the test's own temporary file
	if err != nil {
		t.Fatalf("open the stdin file: %v", err)
	}
	defer file.Close() //nolint:errcheck // a test's temporary file

	saved := os.Stdin
	os.Stdin = file
	fn()
	os.Stdin = saved
}

// withStdoutDiscarded runs fn with os.Stdout written to a temporary file.
func withStdoutDiscarded(t *testing.T, fn func()) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("create the stdout file: %v", err)
	}
	defer file.Close() //nolint:errcheck // a test's temporary file

	saved := os.Stdout
	os.Stdout = file
	fn()
	os.Stdout = saved
}
