package leaction

import (
	"slices"
	"strings"
	"testing"
)

// argvArea is one action whose last keyword hands the rest of the line to a
// program, as `le job run` does.
func argvArea(got *Arguments) Area {
	return New("job", Action{
		Verb: "run", Why: "run one job.",
		Parameters: []Parameter{
			{Keyword: "label", Kind: KindLabel, Value: "label", Requirement: Required, Help: "the job's name"},
			{Keyword: "quiet", Kind: KindSwitch, Help: "log instead of printing"},
			{Keyword: "command", Kind: KindArgv, Value: "argv", Requirement: Required, Help: "the program and its arguments"},
		},
		AnswerArgs: func(args Arguments) (any, int) {
			*got = args
			return nil, 0
		},
	})
}

// TestArgvTakesTheRestOfTheLine proves an argv keyword ends the grammar: every
// word after it reaches the action as typed, an option and a word spelled like
// another keyword of the same action included.
//
// VALIDATES: `le job run label <label> [quiet] command <argv...>` is declared in
// the table (AC-7) and parses as the hand-rolled parser did.
// PREVENTS: a child's `-v` refused as a dash-leading value, and a child's
// `quiet` read as this command's switch.
func TestArgvTakesTheRestOfTheLine(t *testing.T) {
	var got Arguments
	area := argvArea(&got)
	_, code := area.Answer([]string{"run", "label", "unit", "command", "go", "test", "-v", "quiet", "label", "x"})
	if code != 0 {
		t.Fatalf("an argv line answered %d, want 0", code)
	}
	want := []string{"go", "test", "-v", "quiet", "label", "x"}
	if !slices.Equal(got.Values("command"), want) {
		t.Errorf("argv = %q, want %q", got.Values("command"), want)
	}
	if got.Has("quiet") {
		t.Error("a quiet after the command keyword was read as this action's switch")
	}
	if got.One("label") != "unit" {
		t.Errorf("label = %q, want unit", got.One("label"))
	}
}

// TestArgvWithNoProgramIsRefused proves the keyword alone is a usage mistake,
// answered 2 before the action runs, and that Parse agrees with Answer.
func TestArgvWithNoProgramIsRefused(t *testing.T) {
	var got Arguments
	area := argvArea(&got)
	stderr := captureStderr(t, func() {
		if _, code := area.Answer([]string{"run", "label", "unit", "command"}); code != 2 {
			t.Errorf("an empty argv answered %d, want 2", code)
		}
	})
	if !strings.Contains(stderr, `"command" requires <argv>`) {
		t.Errorf("the refusal does not name the missing argv:\n%s", stderr)
	}
	if got != nil {
		t.Error("the action ran on a refused line")
	}
	if _, err := area.Parse("run", []string{"label", "unit", "command"}); err == nil {
		t.Error("Parse accepted the line Answer refused")
	}
}

// TestArgvLastWordIsDataUnlessItIsAnOption proves a bare trailing `help` after
// an argv keyword is the program's word, and `--help` is the question.
func TestArgvLastWordIsDataUnlessItIsAnOption(t *testing.T) {
	parameters := argvArea(new(Arguments)).actions[0].Parameters
	if !trailingIsValue(parameters, []string{"label", "unit", "command", "git", "help"}) {
		t.Error("a bare help inside the argv was read as a question")
	}
	if trailingIsValue(parameters, []string{"label", "unit", "command", "git", "--help"}) {
		t.Error("--help after the argv was read as data")
	}
}

// TestArgvDeclarationsAreRefusedAtNew proves the shapes an argv cannot have.
func TestArgvDeclarationsAreRefusedAtNew(t *testing.T) {
	quiet := Parameter{Keyword: "quiet", Kind: KindSwitch, Help: "h"}
	cases := map[string][]Parameter{
		"positional beside a keyword": {{Kind: KindArgv, Value: "argv", Requirement: Required, Help: "h"}, quiet},
		"repeat":                      {{Keyword: "command", Kind: KindArgv, Value: "argv", Requirement: Required, Help: "h", Repeat: true}},
		"pair":                        {{Keyword: "command", Kind: KindArgv, Value: "argv", Pair: "more", Requirement: Required, Help: "h"}},
	}
	for name, parameters := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("New accepted an argv declared as %s", name)
				}
			}()
			New("job", Action{Verb: "run", Why: "w", Parameters: parameters,
				AnswerArgs: func(Arguments) (any, int) { return nil, 0 }})
		})
	}
}

// TestALonePositionalArgvIsTheWholeLine proves the shape of
// `commit push "<owner authorisation>"`: an action whose only parameter is a
// positional argv reads every word after the verb as typed, one beginning with
// a dash or spelled like a keyword included, and a trailing option still asks
// for help, as it does after an argv keyword.
// VALIDATES: D-P23-push-argv. PREVENTS: the owner's words refused as an option.
func TestALonePositionalArgvIsTheWholeLine(t *testing.T) {
	var got Arguments
	area := New("a", Action{Verb: "push", Why: "why",
		Parameters: []Parameter{{Kind: KindArgv, Value: "words", Requirement: Required, Help: "h"}},
		AnswerArgs: func(args Arguments) (any, int) { got = args; return nil, 0 }})
	for _, line := range [][]string{{"the owner said yes"}, {"-yes, push it"}, {"quiet", "two"}} {
		got = Arguments{}
		if _, code := area.Answer(append([]string{"push"}, line...)); code != 0 {
			t.Fatalf("push %q answered %d, want 0", line, code)
		}
		if !slices.Equal(got.Values("words"), line) {
			t.Errorf("push %q read back as %q", line, got.Values("words"))
		}
	}
	if !trailingAsksForHelp(area.actions[0].Parameters, []string{"--help"}) {
		t.Error("push --help was read as the owner's words, want the help question")
	}
}
