package siteterminaldemo

import "testing"

// VALIDATES: each terminal-demo action named here carries the contract its
// answer implies. check only reads (every check target runs the renderer's
// check or validate mode), build and render change the tree, render takes a
// keyword grammar, and pty forwards its words to RunPTY while the other three
// parse their own. The area answers under its own name.
// PREVENTS: a port that claims a writing renderer as a read-only check, a
// build or render that drops its writes metadata, or a pty whose options le
// refuses.
//
// Method: membership by verb, read from the published rows. No count, no
// order and no Why prose is asserted, so a new action (as readme-carrier was
// on 2026-10-27) leaves this test green, while each named action keeps its
// contract. The removed snapshot of the same name pinned all four
// (tooling-one-off-evidence.md, "Retired action assertions").
//
// MUTATION: (actions.go, `Verb: "check", Summary:`, `Verb: "check", Writes: true, Summary:`): check claims to change the tree, so a read-only property check is filed as a writer and this test goes red.
// MUTATION: (actions.go, `AnswerWords: runPTYAction},`, `Parameters: []leaction.Parameter{{Keyword: "tape", Kind: leaction.KindLabel, Help: "tape", Value: "tape", Requirement: leaction.Optional}}, AnswerArgs: func(leaction.Arguments) (any, int) { return runPTYAction(nil) }},`): pty no longer forwards its words, so le refuses RunPTY's options and this test goes red.
func TestActionsCarryTheirContracts(t *testing.T) {
	if got := Actions().Area; got != area {
		t.Fatalf("area = %q, want %q", got, area)
	}
	writes := []struct {
		verb   string
		writes bool
	}{
		{"check", false},
		{"build", true},
		{"render", true},
	}
	for _, expected := range writes {
		row := actionRow(t, expected.verb)
		if row.Writes != expected.writes {
			t.Errorf("le %s %s: writes = %t, want %t", area, expected.verb, row.Writes, expected.writes)
		}
		if row.Forwards {
			t.Errorf("le %s %s forwards its words, so its own grammar is never checked", area, expected.verb)
		}
	}
	if !actionTable().TakesArguments("render") {
		t.Error("render declares no keyword grammar, so a demo id would sit in an untyped positional slot")
	}
	if !actionRow(t, ptyVerb).Forwards {
		t.Errorf("le %s %s does not forward its words, so RunPTY's options would be refused by le", area, ptyVerb)
	}
}
