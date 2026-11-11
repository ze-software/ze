// Design: docs/architecture/core-design.md -- the answer to an unknown command
// Related: nearest.go -- the nearest-name measure these tests pin

package leroot

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// nearestProbeNames are the command names the unknown-command tests are
// answered from: one two-word command and so its namespace (`nrqalpha`), four
// members of one namespace for the listing cap, and a command named in the
// runs-nothing test. Every word carries the `nrq` prefix, which no other probe
// in the package uses.
var nearestProbeNames = []string{
	"nrqalpha nrqbeta",
	"nrqmany nrqone",
	"nrqmany nrqtwo",
	"nrqmany nrqthree",
	"nrqmany nrqfour",
	"nrqrun nrqgo",
}

// registerNearestProbes answers nearestProbeNames as the registry's visible
// command names for this one test, and restores the registry read when it
// ends. Nothing is registered, so no probe reaches another test of the binary
// (the startup check of a later test would refuse a namespace with no summary).
func registerNearestProbes(t *testing.T) {
	t.Helper()
	saved := visibleCommandNames
	visibleCommandNames = func() []string { return nearestProbeNames }
	t.Cleanup(func() { visibleCommandNames = saved })
}

// dispatchCaptured runs Dispatch and answers its code, stdout and stderr.
func dispatchCaptured(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	code := 0
	stderr := ""
	stdout := captureStdout(t, func() {
		stderr = captureStderr(t, func() { code = Dispatch("le", args) })
	})
	return code, stdout, stderr
}

// VALIDATES: AC-8. An unknown command answers exit 2 and two lines on stderr,
// the first naming the nearest registered command, one row per rank of the
// measure: reordered or hyphenated words, the trailing words of a command,
// and a spelling within edit distance.
// PREVENTS: a mistyped command answering with the whole usage page, which the
// caller reads in full to find the one name it meant.
func TestUnknownCommandNamesTheNearestRegisteredOne(t *testing.T) {
	registerNearestProbes(t)
	const help = "List every command: le help\n"
	for _, tc := range []struct {
		rank string
		args []string
		want string
	}{
		{"1: the words reordered", []string{"nrqbeta", "nrqalpha"},
			"error: unknown command: nrqbeta. Nearest: le nrqalpha nrqbeta\n" + help},
		{"1: the words hyphenated and reordered", []string{"nrqbeta-nrqalpha"},
			"error: unknown command: nrqbeta-nrqalpha. Nearest: le nrqalpha nrqbeta\n" + help},
		{"2: the command typed by its last word", []string{"nrqbeta"},
			"error: unknown command: nrqbeta. Nearest: le nrqalpha nrqbeta\n" + help},
		{"3: two neighbors swapped, the namespace and its member", []string{"nrqalpah"},
			"error: unknown command: nrqalpah. Nearest: le nrqalpha, le nrqalpha nrqbeta\n" + help},
		{"3: words after the mistyped one", []string{"nrqrunn", "nrqgo", "x"},
			"error: unknown command: nrqrunn. Nearest: le nrqrun, le nrqrun nrqgo\n" + help},
	} {
		t.Run(tc.rank, func(t *testing.T) {
			code, stdout, stderr := dispatchCaptured(t, tc.args)
			if code != 2 {
				t.Errorf("%v answered %d, want 2", tc.args, code)
			}
			if stdout != "" {
				t.Errorf("%v wrote stdout: %q", tc.args, stdout)
			}
			if stderr != tc.want {
				t.Errorf("%v answered\n%q\nwant\n%q", tc.args, stderr, tc.want)
			}
		})
	}
}

// VALIDATES: AC-8 survivor and the boundary "a fourth is counted, not
// listed". A word near five names lists the first three in name order and
// counts the rest.
// PREVENTS: a short typo answering a list as long as the usage it replaced.
func TestUnknownCommandListsAtMostThreeCandidates(t *testing.T) {
	registerNearestProbes(t)
	code, _, stderr := dispatchCaptured(t, []string{"nrqmny"})
	want := "error: unknown command: nrqmny. Nearest: le nrqmany, le nrqmany nrqfour, le nrqmany nrqone (and 2 more)\n" +
		"List every command: le help\n"
	if code != 2 {
		t.Errorf("a word near five names answered %d, want 2", code)
	}
	if stderr != want {
		t.Errorf("a word near five names answered\n%q\nwant\n%q", stderr, want)
	}

	// Exactly three near names are listed with no count.
	listed := nearestNames([]string{"nrqmny"}, []string{"nrqmany a", "nrqmany b", "nrqmany c"})
	if len(listed) != nearestListedMax {
		t.Fatalf("three near names answered %v", listed)
	}
	if line := unknownLine("le", "nrqmny", listed); line != "error: unknown command: nrqmny. Nearest: le nrqmany a, le nrqmany b, le nrqmany c" {
		t.Errorf("three near names answered %q", line)
	}
}

// VALIDATES: AC-8 survivor. A word near no registered name answers the first
// line without a nearest clause, and the second line still names the help.
// The measure's bounds, one row each: edit distance 3 is too far, and a
// distance of half the typed word's length or more is no near name.
// PREVENTS: a guess that misleads more than it helps (R-5).
func TestUnknownCommandWithNoNearNamePointsAtHelp(t *testing.T) {
	registerNearestProbes(t)
	code, stdout, stderr := dispatchCaptured(t, []string{"nrqalphxyz"})
	want := "error: unknown command: nrqalphxyz\nList every command: le help\n"
	if code != 2 {
		t.Errorf("a word near nothing answered %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("a word near nothing wrote stdout: %q", stdout)
	}
	if stderr != want {
		t.Errorf("a word near nothing answered\n%q\nwant\n%q", stderr, want)
	}

	for _, tc := range []struct {
		why        string
		typed      string
		candidates []string
		want       int
	}{
		{"distance 1 and under half the length", "comit", []string{"commit create"}, 1},
		{"distance 2 by two swaps", "cmomti", []string{"commit create"}, 1},
		{"distance 2, the last accepted", "xbcxefgh", []string{"go abcdefgh"}, 1},
		{"distance 3, under half the length", "xbcxefxh", []string{"go abcdefgh"}, 0},
		{"distance 1 at half the length", "xo", []string{"go lint"}, 0},
		{"distance 2 at half the length", "lixx", []string{"go lint"}, 0},
		{"a swap of two neighbors is one", "lnit", []string{"go lint"}, 1},
	} {
		if got := nearestNames([]string{tc.typed}, tc.candidates); len(got) != tc.want {
			t.Errorf("%s: %q among %v answered %v, want %d name(s)", tc.why, tc.typed, tc.candidates, got, tc.want)
		}
	}
}

// VALIDATES: AC-8 at the helpAsked site. `le help <unknown>` answers the same
// two lines, the same code and no stdout as running the unknown command.
// PREVENTS: the help question keeping the full usage page after dispatch
// stopped printing it, so the two sites disagree.
func TestHelpOfAnUnknownCommandAnswersTheSameLines(t *testing.T) {
	registerNearestProbes(t)
	for _, words := range [][]string{{"nrqbeta"}, {"nrqalpah"}, {"nrqalphxyz"}} {
		runCode, _, runErr := dispatchCaptured(t, words)
		helpCode, helpOut, helpErr := dispatchCaptured(t, append([]string{"help"}, words...))
		if helpCode != 2 || runCode != 2 {
			t.Errorf("%v answered %d run and %d asked, want 2 for both", words, runCode, helpCode)
		}
		if helpOut != "" {
			t.Errorf("le help %v wrote stdout: %q", words, helpOut)
		}
		if helpErr != runErr {
			t.Errorf("le help %v answered\n%q\nwhere running it answered\n%q", words, helpErr, runErr)
		}
	}
}

// VALIDATES: AC-8 and the security row "Autocorrection". The unknown-command
// answer names the nearest command and never runs it, or anything else.
// Method: a real handler is registered under the nearest name's dispatch path
// for this test alone (command.AddLocalDataForTest), so le could reach it, and
// it counts every call.
// PREVENTS: a typo starting the command le guessed it meant.
func TestUnknownCommandRunsNothing(t *testing.T) {
	registerNearestProbes(t)
	ran := 0
	t.Cleanup(command.AddLocalDataForTest(CommandPath("nrqrun nrqgo"), toolHandler(func([]string) (any, int) {
		ran++
		return map[string]string{"probe": "ran"}, 0
	})))
	if _, handler, _ := resolve([]string{"nrqrun", "nrqgo"}); handler == nil {
		t.Fatal("the probe handler does not resolve, so this test could not see it run")
	}
	for _, args := range [][]string{{"nrqgo"}, {"nrqgo", "show"}, {"help", "nrqgo"}, {"nrqrnu", "nrqgo", "show"}} {
		code, stdout, stderr := dispatchCaptured(t, args)
		if code != 2 {
			t.Errorf("%v answered %d, want 2", args, code)
		}
		if stdout != "" {
			t.Errorf("%v wrote stdout: %q", args, stdout)
		}
		if want := "Nearest: le nrqrun"; !containsLine(stderr, want) {
			t.Errorf("%v answered no nearest name %q:\n%s", args, want, stderr)
		}
	}
	if ran != 0 {
		t.Errorf("an unknown command ran the nearest command's handler %d time(s)", ran)
	}
}

// containsLine reports whether some line of text holds want.
func containsLine(text, want string) bool {
	for line := range strings.Lines(text) {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}
