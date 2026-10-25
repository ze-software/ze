package leaction

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/helpfmt"
)

// helpfmtPage names the page type the listing tests render.
type helpfmtPage = helpfmt.Page

// claimArea is one action with a required positional, as `spec claim` takes,
// beside a second action the refusal must not name.
func claimArea(ran *bool) Area {
	return New("spec", Action{
		Verb: "claim", Summary: "claim a spec", Why: "claim a spec for this session.",
		Parameters: []Parameter{
			{Kind: KindSpec, Value: "spec", Requirement: Required, Help: "the spec file to claim"},
			{Keyword: "quiet", Kind: KindSwitch, Help: "print nothing on success"},
		},
		AnswerArgs: func(Arguments) (any, int) { *ran = true; return nil, 0 },
	}, Action{
		Verb: "release", Summary: "release the claim", Why: "release the claim.",
		Answer: func() (any, int) { *ran = true; return nil, 0 },
	})
}

// VALIDATES: AC-9 at the table -- a line the parser refuses on a pipe (here a
// keyword the action does not take) is answered with the one action's usage
// line and its keyword lines, and nothing about the area's other actions; the
// code is the parser's, 2. A MISSING required parameter is refused the same way
// (TestMissingRequiredParameterOnAPipeRefusesWithOneUsage).
// PREVENTS: a refusal that dumps the whole family, so the reader cannot see
// which line they got wrong.
func TestAParseRefusalOnAPipePrintsOneActionUsage(t *testing.T) {
	ran := false
	area := claimArea(&ran)
	var code int
	stdout := captureStdout(t, func() {
		stderr := captureStderr(t, func() { _, code = area.Answer([]string{"claim", "plan/x.md", "loud"}) })
		if !strings.Contains(stderr, "\nusage: le spec claim <spec> [quiet] [| json | yaml | table]\n") {
			t.Errorf("the refusal does not carry claim's usage line:\n%s", stderr)
		}
		if !strings.Contains(stderr, "the spec file to claim") || !strings.Contains(stderr, "print nothing on success") {
			t.Errorf("the refusal does not carry claim's keyword lines:\n%s", stderr)
		}
		if strings.Contains(stderr, "release") {
			t.Errorf("the refusal names another action:\n%s", stderr)
		}
	})
	if code != 2 {
		t.Errorf("a refused line answered %d, want 2", code)
	}
	if ran {
		t.Error("a refused line ran the action")
	}
	if stdout != "" {
		t.Errorf("a refusal wrote to stdout: %q", stdout)
	}
}

// VALIDATES: AC-9 and AC-46 off a terminal (D-P2-required) -- a line that
// leaves out a Required parameter, read on a pipe through Area.Answer, is
// refused by the parser before the action's body runs: exit 2, the missing
// slot named, and the one action's usage and keyword lines on stderr. A
// terminal asks for the value before this point (leroot, Missing), so only
// the pipe path reaches the refusal.
// PREVENTS: a missing value reaching a body that answers its own code for it,
// so the same mistake answers 1 in one area and 2 in the next.
func TestMissingRequiredParameterOnAPipeRefusesWithOneUsage(t *testing.T) {
	ran := false
	area := claimArea(&ran)
	var code int
	stdout := captureStdout(t, func() {
		stderr := captureStderr(t, func() { _, code = area.Answer([]string{"claim", "quiet"}) })
		if !strings.Contains(stderr, "error: le spec claim requires <spec>\n") {
			t.Errorf("the refusal does not name the missing slot:\n%s", stderr)
		}
		if !strings.Contains(stderr, "\nusage: le spec claim <spec> [quiet] [| json | yaml | table]\n") {
			t.Errorf("the refusal does not carry claim's usage line:\n%s", stderr)
		}
		if !strings.Contains(stderr, "the spec file to claim") {
			t.Errorf("the refusal does not carry claim's keyword lines:\n%s", stderr)
		}
		if strings.Contains(stderr, "release") {
			t.Errorf("the refusal names another action:\n%s", stderr)
		}
	})
	if code != 2 {
		t.Errorf("a line missing a required value answered %d, want 2", code)
	}
	if ran {
		t.Error("a line missing a required value ran the action")
	}
	if stdout != "" {
		t.Errorf("a refusal wrote to stdout: %q", stdout)
	}
}

// VALIDATES: D-P1-validate -- a value its Kind does not accept is refused at
// parse with exit 2, naming the slot, and the body never runs; a well-formed
// value of the same kind runs.
// PREVENTS: a malformed value reaching a body that answers 1 for it.
func TestAMalformedValueIsRefusedAtParse(t *testing.T) {
	reached := 0
	area := New("probe", Action{
		Verb: "run", Why: "run the probe a number of times",
		Parameters: []Parameter{{Keyword: "count", Value: "n", Requirement: Optional, Kind: KindUint, Help: "h"}},
		AnswerArgs: func(Arguments) (any, int) { reached++; return nil, 0 },
	})
	var code int
	stderr := captureStderr(t, func() { _, code = area.Answer([]string{"run", "count", "three"}) })
	if code != 2 {
		t.Errorf("a malformed uint answered %d, want 2", code)
	}
	if !strings.Contains(stderr, `le probe run cannot take "three" as count <n>`) {
		t.Errorf("the refusal does not name the value and its slot:\n%s", stderr)
	}
	if reached != 0 {
		t.Errorf("a malformed value ran the action %d time(s)", reached)
	}
	if _, code = area.Answer([]string{"run", "count", "3"}); code != 0 || reached != 1 {
		t.Errorf("a well-formed uint answered %d and ran %d time(s), want 0 and 1", code, reached)
	}
}

// VALIDATES: AC-12 -- every usage line and refusal names the program the
// dispatcher recorded, `ze le` in a ze_le build.
// PREVENTS: a hard-coded `usage: le ` a reader of `ze le` copies and fails.
func TestUsageNamesTheRunningProgram(t *testing.T) {
	SetProgram("ze le")
	t.Cleanup(func() { SetProgram("le") })
	ran := false
	area := claimArea(&ran)
	stderr := captureStderr(t, func() { area.Answer([]string{"claim", "plan/x.md", "loud"}) })
	if !strings.Contains(stderr, "usage: ze le spec claim <spec>") {
		t.Errorf("the refusal does not name ze le:\n%s", stderr)
	}
	stderr = captureStderr(t, func() { area.Answer([]string{"release", "now"}) })
	if !strings.Contains(stderr, "usage: ze le spec release") {
		t.Errorf("the zero-argument refusal does not name ze le:\n%s", stderr)
	}
	help := captureStdout(t, func() { area.Answer([]string{"claim", "--help"}) })
	if !strings.HasPrefix(help, "ze le spec claim - ") {
		t.Errorf("the action page does not name ze le:\n%s", help)
	}
	if strings.Contains(help+stderr, "usage: le ") {
		t.Errorf("a line still names le:\n%s%s", help, stderr)
	}
}

// VALIDATES: AC-8 at the table -- a keyword page carries its kind, values,
// required and repeat facts and its help; an unknown keyword is refused naming
// what the action takes.
// PREVENTS: a keyword question answered with the whole area.
func TestKeywordPageCarriesTheDeclaration(t *testing.T) {
	ran := false
	list := claimArea(&ran).Actions()
	page, err := list.KeywordPage("le", "claim", "quiet")
	if err != nil {
		t.Fatalf("quiet: %v", err)
	}
	text := renderAt(page, 100)
	for _, want := range []string{"le spec claim quiet - print nothing on success", "kind: switch", "required: no", "repeat: no"} {
		if !strings.Contains(text, want) {
			t.Errorf("the keyword page lacks %q:\n%s", want, text)
		}
	}
	if _, err := list.KeywordPage("le", "claim", "nosuch"); err == nil || !strings.Contains(err.Error(), "quiet") {
		t.Errorf("an unknown keyword was answered %v, want a refusal naming quiet", err)
	}
}
