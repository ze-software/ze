package leaction

import (
	"errors"
	"strings"
	"testing"
)

// wholeTreeArea builds an area holding one WholeTree action of each shape, an
// argument-aware one and a bare one, and counts what their bodies ran.
func wholeTreeArea(t *testing.T) (Area, *int) {
	t.Helper()
	ran := 0
	area := New("verify",
		Action{Verb: "worktree", Why: "w", WholeTree: true,
			Parameters: []Parameter{{Keyword: "keep"}},
			AnswerArgs: func(Arguments) (any, int) { ran++; return nil, 0 }},
		Action{Verb: "bare", Why: "b", WholeTree: true,
			Answer: func() (any, int) { ran++; return nil, 0 }},
		Action{Verb: "list", Why: "l",
			Answer: func() (any, int) { ran++; return nil, 0 }},
	)
	return area, &ran
}

// VALIDATES: AC-8 -- a WholeTree action is refused with exit 2 before its body
// runs, and the gate is handed the exact command line the caller typed.
// PREVENTS: an agent's tool call starting a whole-tree run.
func TestWholeTreeActionRefusedBeforeItsBody(t *testing.T) {
	area, ran := wholeTreeArea(t)
	var asked []string
	restore := SetWholeTreeAdmission(func(line string) error {
		asked = append(asked, line)
		return errors.New("refused for the test")
	})
	defer restore()
	for _, line := range [][]string{{"worktree", "keep"}, {"bare"}} {
		var code int
		stderr := captureStderr(t, func() { _, code = area.Answer(line) })
		if code != 2 || !strings.Contains(stderr, "refused for the test") {
			t.Fatalf("%v answered %d, stderr %q, want 2 and the refusal", line, code, stderr)
		}
	}
	if *ran != 0 {
		t.Fatalf("a refused whole-tree body ran %d time(s)", *ran)
	}
	want := []string{"./le verify worktree keep", "./le verify bare"}
	if strings.Join(asked, "|") != strings.Join(want, "|") {
		t.Fatalf("the gate was asked %q, want %q", asked, want)
	}
}

// VALIDATES: AC-8 over the real question -- `go test` has no terminal on
// stdin, so the unreplaced gate refuses with its release-check message.
// PREVENTS: the default gate admitting a process no person started.
func TestWholeTreeRealGateRefusesWithNoTerminal(t *testing.T) {
	area, ran := wholeTreeArea(t)
	var code int
	stderr := captureStderr(t, func() { _, code = area.Answer([]string{"bare"}) })
	if code != 2 || *ran != 0 || !strings.Contains(stderr, "a release check only Thomas starts") {
		t.Fatalf("answered %d, ran %d, stderr %q, want a terminal refusal", code, *ran, stderr)
	}
}

// VALIDATES: AC-10 -- an admitted line runs, and an action that is not
// WholeTree never asks the gate.
func TestWholeTreeAdmittedRunsAndOthersNeverAsk(t *testing.T) {
	area, ran := wholeTreeArea(t)
	asked := 0
	restore := SetWholeTreeAdmission(func(string) error { asked++; return nil })
	defer restore()
	if _, code := area.Answer([]string{"worktree"}); code != 0 || *ran != 1 || asked != 1 {
		t.Fatalf("admitted line answered %d, ran %d, asked %d", code, *ran, asked)
	}
	if _, code := area.Answer([]string{"list"}); code != 0 || *ran != 2 || asked != 1 {
		t.Fatalf("plain action answered %d, ran %d, asked the gate %d time(s)", code, *ran, asked)
	}
}

// VALIDATES: the listing publishes the declaration, so a test over Actions()
// can find every whole-tree verb without a list beside the registry.
func TestWholeTreeIsPublishedInTheListing(t *testing.T) {
	area, _ := wholeTreeArea(t)
	got := map[string]bool{}
	for _, row := range area.Actions().Actions {
		got[row.Verb] = row.WholeTree
	}
	if !got["worktree"] || !got["bare"] || got["list"] {
		t.Fatalf("listing published %v", got)
	}
}
