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
		Verb: "claim", Why: "claim a spec for this session.",
		Parameters: []Parameter{
			{Kind: KindSpec, Value: "spec", Requirement: Required, Help: "the spec file to claim"},
			{Keyword: "quiet", Kind: KindSwitch, Help: "print nothing on success"},
		},
		AnswerArgs: func(Arguments) (any, int) { *ran = true; return nil, 0 },
	}, Action{
		Verb: "release", Why: "release the claim.",
		Answer: func() (any, int) { *ran = true; return nil, 0 },
	})
}

// VALIDATES: AC-9 at the table -- a line the parser refuses on a pipe (here a
// keyword the action does not take) is answered with the one action's usage
// line and its keyword lines, and nothing about the area's other actions; the
// code is the parser's, 2. Refusing a MISSING required parameter at parse is
// Phase 5's (D-P2-required); `le spec claim` refuses it in its own handler
// (internal/le/spec, TestClaimWithoutASpecRefusesWithClaimsUsageOnly).
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
