// Related: actions.go -- the verify verbs that declare leaction.Action.WholeTree

package verify

import (
	"os"
	"slices"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/le/le/human"
)

// noTerminal answers that no stream is a terminal, which is what a tool call
// through pipes gives the gate. The tests install it through the Go-only seam
// rather than reading the test binary's own streams: `go test` started from
// Thomas's shell can hand the binary his terminal, and the real gate would then
// admit a whole-tree run from inside a unit test.
func noTerminal(*os.File) (bool, error) { return false, nil }

// refuseAsAToolCall installs the real refusal over noTerminal, and records
// each command line the gate was asked about.
func refuseAsAToolCall(t *testing.T) *[]string {
	t.Helper()
	var asked []string
	restore := leaction.SetWholeTreeAdmission(func(line string) error {
		asked = append(asked, line)
		return human.Check(noTerminal, os.Stdin, os.Stdout, line)
	})
	t.Cleanup(restore)
	return &asked
}

// assertRefusedAsAToolCall drives one `le verify` line and proves the gate
// refused it with exit 2 before the body ran: the only text on stderr is the
// refusal, which names the release check, the exact line to hand Thomas and
// the scoped run. A body that ran would have written its own refusal, its
// admission line or a stage line beside it.
func assertRefusedAsAToolCall(t *testing.T, line []string) {
	t.Helper()
	asked := refuseAsAToolCall(t)
	var code int
	stderr := captureStderr(t, func() { _, code = Answer(line) })
	given := "./le " + area + " " + strings.Join(line, " ")
	if code != 2 {
		t.Fatalf("%s answered %d, want 2; stderr:\n%s", given, code, stderr)
	}
	if len(*asked) != 1 || (*asked)[0] != given {
		t.Fatalf("%s asked the gate about %q, want exactly %q", given, *asked, given)
	}
	for _, want := range []string{"a release check only Thomas starts", "stdin", "stdout", "\n    " + given + "\n", human.ScopedCommand} {
		if !strings.Contains(stderr, want) {
			t.Errorf("%s refusal does not carry %q:\n%s", given, want, stderr)
		}
	}
	for _, body := range []string{"worktree add", "admitted", "slot", "stage"} {
		if strings.Contains(stderr, body) {
			t.Errorf("%s refusal carries %q, so the body ran:\n%s", given, body, stderr)
		}
	}
}

// VALIDATES: AC-8 through the verb -- `le verify worktree`, alone and with
// commit, keep and first-red, is refused before it creates a worktree or asks
// for a job slot, when no stream is a terminal.
// PREVENTS: an agent's tool call starting the whole-tree run (D-3).
func TestVerifyWorktreeRefusesWithoutTerminal(t *testing.T) {
	for _, line := range [][]string{
		{actionName},
		{actionName, commitKeyword, "HEAD"},
		{actionName, "keep"},
		{actionName, keywordFirstRed},
	} {
		t.Run(strings.Join(line, "_"), func(t *testing.T) {
			assertRefusedAsAToolCall(t, line)
		})
	}
}

// VALIDATES: AC-8 for `le verify current`, bare, full and fast: fast is a full
// verification for the gate too (decision Q-5).
func TestVerifyCurrentRefusesWithoutTerminal(t *testing.T) {
	for _, line := range [][]string{
		{"current"},
		{"current", keywordFull},
		{"current", keywordFast},
		{"current", keywordFast, keywordFirstRed},
	} {
		t.Run(strings.Join(line, "_"), func(t *testing.T) {
			assertRefusedAsAToolCall(t, line)
		})
	}
}

// readOnlyVerbs are the verify verbs that start no verification: each reads
// what a run already wrote. TestEveryWholeTreeVerbIsGated drives each one under
// the refusing gate and proves it answers without the gate being asked, so a
// verb lands here only with that proof, never by being listed.
var readOnlyVerbs = map[string][]string{
	"reds": {"reds", "file", "internal/le/verify/actions.go"},
	"list": {"list"},
}

// wholeTreeGrammar is the whole keyword grammar each gated verb accepts. A
// keyword added to a gated verb fails this test until somebody reads it and
// adds it here, which is where an override keyword would be caught (AC-11).
var wholeTreeGrammar = map[string][]string{
	actionName: {commitKeyword, "keep", keywordFirstRed, keywordPart, keywordOf, keywordAll},
	"current":  {keywordFull, keywordFast, keywordFirstRed},
}

// VALIDATES: AC-8 and AC-11 over the registered table. Every verify verb either
// declares WholeTree and is refused as a tool call, or is a read-only verb
// proven never to ask the gate; and no gated verb carries a keyword outside
// its known grammar, so no override exists to type.
// PREVENTS: a new whole-tree verb registering without the declaration.
func TestEveryWholeTreeVerbIsGated(t *testing.T) {
	rows := Actions().Actions
	for _, row := range rows {
		if !row.WholeTree {
			line, known := readOnlyVerbs[row.Verb]
			if !known {
				t.Errorf("verify %s neither declares WholeTree nor is a proven read-only verb: "+
					"declare leaction.Action.WholeTree if it starts a verification", row.Verb)
				continue
			}
			asked := refuseAsAToolCall(t)
			captureStderr(t, func() { Answer(line) })
			if len(*asked) != 0 {
				t.Errorf("read-only verify %s asked the whole-tree gate %q", row.Verb, *asked)
			}
			continue
		}
		grammar, known := wholeTreeGrammar[row.Verb]
		if !known {
			t.Errorf("gated verify %s has no recorded grammar to check for an override", row.Verb)
			continue
		}
		for _, parameter := range row.Parameters {
			if !slices.Contains(grammar, parameter.Keyword) {
				t.Errorf("gated verify %s accepts keyword %q outside its grammar %q", row.Verb, parameter.Keyword, grammar)
			}
		}
		assertRefusedAsAToolCall(t, []string{row.Verb})
	}
	for verb := range wholeTreeGrammar {
		if !slices.ContainsFunc(rows, func(row leaction.Row) bool { return row.Verb == verb && row.WholeTree }) {
			t.Errorf("verify %s is no longer registered as WholeTree", verb)
		}
	}
}
