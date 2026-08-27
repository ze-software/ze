// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-8, and AC-11. The
// rule-format linter runs as a function and returns structured data. Its answers
// match the script for the corpus and each malformed form.
// PREVENTS: agreement with rules_lint.py only on a clean tree. Authors use each
// message to fix one rule. A wrong line or quotation causes a search instead of
// an edit.

package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goodRule is a rendered rule that violates nothing. Cases mutate one thing in
// it, so each case's expectation is about the mutation and nothing else.
const goodRule = `# A Title

**When:** when something happens
**Severity:** blocking
**Related:** planning, testing

## A Section

- **MUST do the thing.**
`

func TestATriggerMustNameASituation(t *testing.T) {
	cases := []struct {
		trigger string
		want    string
	}{
		{"when a peer opens a session", ""},
		{"writing Go in Ze", ""},
		{"prior to a commit", ""},
		{"as soon as the gate is red", ""},
		{"all CLI commands MUST follow these patterns", "must name a situation, not a directive"},
		{"when a peer opens,", "is a truncated sentence"},
		{"when the check is enforced by", "cut off mid-clause"},
		{"**when a peer opens a session", "unbalanced '**'"},
		{"`test/**/*.ci` is written", "must name a situation"},
		{"**", "has no text once markup is stripped"},
		{"nothing is written", "must name a situation"},
		{"doing the work", ""},
	}
	for _, tc := range cases {
		problems := checkTrigger(tc.trigger)
		joined := strings.Join(problems, "\n")
		if tc.want == "" {
			if len(problems) > 0 {
				t.Errorf("checkTrigger(%q) refused a valid trigger: %s", tc.trigger, joined)
			}
			continue
		}
		if !strings.Contains(joined, tc.want) {
			t.Errorf("checkTrigger(%q) = %q, want it to mention %q", tc.trigger, joined, tc.want)
		}
	}
}

func TestTheSeverityMessageQuotesSixtyRunesNotSixtyBytes(t *testing.T) {
	// Python's stripped[:60] counts runes. Rune 60 of this line is in the
	// three-byte em dash sequence. A byte slice would split it and produce
	// invalid UTF-8 with incorrect text.
	line := "BLOCKING ————————————————————————————————————————————————————tail"
	problems := checkSeverityAgrees("advisory", "T", []string{line}, 0)
	if len(problems) != 1 {
		t.Fatalf("checkSeverityAgrees = %v, want one problem", problems)
	}
	if strings.Contains(problems[0], "tail") {
		t.Errorf("the message quoted past the 60th rune: %s", problems[0])
	}
	if !strings.Contains(problems[0], pyRepr(firstRunes(line, 60))) {
		t.Errorf("the message does not quote the first 60 runes: %s", problems[0])
	}
	if strings.Contains(problems[0], "�") {
		t.Errorf("the message carries a broken rune: %s", problems[0])
	}
}

func TestSeverityMustAgreeWithTheProse(t *testing.T) {
	lines := []string{"# T", "", "**When:** when x", "**Severity:** advisory", "", "This is BLOCKING."}
	problems := checkSeverityAgrees("advisory", "T", lines, 4)
	if len(problems) != 1 || !strings.Contains(problems[0], "but line 6 says BLOCKING") {
		t.Fatalf("checkSeverityAgrees = %v, want one problem naming line 6", problems)
	}

	// A table row tabulates another rule's severity, and a marked prose line
	// says whose severity it is. Neither is this rule's own claim.
	exempt := []string{"", "", "", "", "| a | BLOCKING |", "> quoted BLOCKING",
		"the other doc is BLOCKING <!-- severity-note: that one -->"}
	if problems := checkSeverityAgrees("advisory", "T", exempt, 4); len(problems) != 0 {
		t.Errorf("checkSeverityAgrees refused an exempt line: %v", problems)
	}

	if problems := checkSeverityAgrees("blocking", "T (BLOCKING)", lines, 4); len(problems) != 1 ||
		!strings.Contains(problems[0], "the title must not say BLOCKING") {
		t.Errorf("a BLOCKING title was not refused: %v", problems)
	}
}

func TestARuleNeedsTheCanonicalMetadataBlock(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"clean", goodRule, ""},
		{"no title", "\nnot a title\n", "first non-blank line must be a single '# Title'"},
		{"mis-cased key", strings.Replace(goodRule, "**When:**", "**when:**", 1),
			"metadata key '**when:**' must be one of When/Severity/Related (exact case)"},
		{"bad severity", strings.Replace(goodRule, "**Severity:** blocking", "**Severity:** important", 1),
			"'**Severity:**' must be one of ['advisory', 'blocking'], got 'important'"},
		{"no when", strings.Replace(goodRule, "**When:** when something happens\n", "", 1),
			"missing required '**When:** <trigger>' line"},
		{"no severity", strings.Replace(goodRule, "**Severity:** blocking\n", "", 1),
			"missing required '**Severity:** blocking|advisory' line"},
		{"out of order", strings.Replace(goodRule,
			"**When:** when something happens\n**Severity:** blocking",
			"**Severity:** blocking\n**When:** when something happens", 1),
			"metadata keys must be ordered When, Severity, Related (found Severity, When, Related)"},
		{"pathy related", strings.Replace(goodRule, "**Related:** planning, testing",
			"**Related:** ai/rules/planning.md", 1),
			"'**Related:**' entry 'ai/rules/planning.md' must be a bare rule slug"},
	}
	for _, tc := range cases {
		problems := checkRule(tc.text)
		joined := strings.Join(problems, "\n")
		if tc.want == "" {
			if len(problems) > 0 {
				t.Errorf("%s: checkRule refused a clean rule: %s", tc.name, joined)
			}
			continue
		}
		if !strings.Contains(joined, tc.want) {
			t.Errorf("%s: checkRule = %q, want it to mention %q", tc.name, joined, tc.want)
		}
	}
}

func TestADirectivePointStatesItsLevel(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"clean", "---\nkind: directive\nlevel: MUST\n---\n- **MUST do it.**\n", ""},
		{"a table states nothing", "---\nkind: table\nlevel:\n---\n| a | b |\n", ""},
		{"no keyword", "---\nkind: directive\nlevel:\n---\n- Do the thing.\n",
			"a directive MUST state its obligation in RFC 2119 language"},
		{"lowercase modal", "---\nkind: directive\nlevel: MUST\n---\n- **MUST do it**, and you should too.\n",
			"lowercase obligation word(s) 'should'"},
		{"level disagrees", "---\nkind: directive\nlevel: MAY\n---\n- **MUST do it.**\n",
			"'level: MAY' disagrees with the body, whose strongest obligation is MUST"},
		{"level unknown", "---\nkind: directive\nlevel: SHALL\n---\n- **MUST do it.**\n",
			"'level: SHALL' is not one of MAY, SHOULD NOT, SHOULD, MUST NOT, MUST"},
		{"empty level", "---\nkind: directive\nlevel:\n---\n- **MUST do it.**\n",
			"'level: (empty)' disagrees with the body"},
		{"no frontmatter", "- **MUST do it.**\n", "no '---' frontmatter block"},
		// A quoted keyword is another artifact's obligation, not this point's.
		{"quoted only", "---\nkind: directive\nlevel:\n---\n- The RFC says `MUST` there.\n",
			"a directive MUST state its obligation"},
		// Either polarity of the strongest tier is a true answer.
		{"prohibition tier", "---\nkind: directive\nlevel: MUST NOT\n---\n- **MUST NOT do it**, and MUST stop.\n", ""},
	}
	for _, tc := range cases {
		problems := checkPoint(tc.text)
		joined := strings.Join(problems, "\n")
		if tc.want == "" {
			if len(problems) > 0 {
				t.Errorf("%s: checkPoint refused a clean point: %s", tc.name, joined)
			}
			continue
		}
		if !strings.Contains(joined, tc.want) {
			t.Errorf("%s: checkPoint = %q, want it to mention %q", tc.name, joined, tc.want)
		}
	}
}

func TestAFencedBlockStatesNoObligation(t *testing.T) {
	// A fence quotes code or output. A keyword inside one is not this point's
	// obligation, and neither is one inside a blockquote.
	body := "---\nkind: directive\nlevel:\n---\n- Do it.\n\n```\nMUST be there\n```\n\n> MUST be here\n"
	problems := checkPoint(body)
	if len(problems) != 1 || !strings.Contains(problems[0], "no\ncapitalised keyword") &&
		!strings.Contains(problems[0], "capitalised keyword") {
		t.Fatalf("checkPoint = %v, want the missing-keyword problem", problems)
	}

	// A tilde fence closes on tildes only, and a backtick run inside it is
	// content rather than a terminator.
	kept := stripFences("~~~\n```\nMUST\n```\n~~~\nMUST after\n")
	if strings.Contains(kept, "MUST\n") && !strings.Contains(kept, "MUST after") {
		t.Errorf("stripFences kept the wrong side: %q", kept)
	}
	if !strings.Contains(kept, "MUST after") {
		t.Errorf("stripFences dropped the line after the fence: %q", kept)
	}
}

func TestLowerModalsReadsTheNeighborsTheLookbehindRefused(t *testing.T) {
	// The Python pattern refuses a word character or a hyphen on either side.
	// RE2 has no lookbehind, so the neighbors are read by hand and this case
	// is what says the two agree.
	cases := map[string][]string{
		"you must do it":    {"must"},
		"a must-have":       nil,
		"the mustard":       nil,
		"remust":            nil,
		"may, and should":   {"may", "should"},
		"MUST is fine":      nil,
		"(may) and [must]":  {"may", "must"},
		"non-must":          nil,
		"must_not":          nil,
		"shall and shall":   {"shall"},
		"the word is must.": {"must"},
	}
	for text, want := range cases {
		got := lowerModals(text)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("lowerModals(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestLintRefusesTheEmptyCorpusTheScriptPasses(t *testing.T) {
	tree := t.TempDir()
	rulesDir := filepath.Join(tree, "ai", "rules")
	if err := os.MkdirAll(rulesDir, 0o750); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	report, err := Lint(tree)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	if !report.Failed() {
		t.Fatalf("Lint passed a tree with no rule and no point tree: %+v", report)
	}
	if len(report.Empty) != 2 {
		t.Errorf("Lint named %d empty populations, want 2: %v", len(report.Empty), report.Empty)
	}
	text := report.Text()
	for _, want := range []string{"no rule file under ai/rules/", "no rule point file under ai/rules/points/"} {
		if !strings.Contains(text, want) {
			t.Errorf("the page does not name %q: %s", want, text)
		}
	}

	// A rule present and a point tree absent is the half the script also
	// passes, and it is the half a real checkout can reach.
	if err := os.WriteFile(filepath.Join(rulesDir, "sample.md"), []byte(goodRule), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	report, err = Lint(tree)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	if !report.Failed() || len(report.Empty) != 1 {
		t.Errorf("Lint passed a tree with no point tree: %+v", report)
	}
}

func TestLintRefusesAnUnreadableCorpus(t *testing.T) {
	tree := t.TempDir()
	if _, err := Lint(tree); err == nil {
		t.Fatal("Lint accepted a tree with no ai/rules at all")
	}
}

func TestTheLintAnswerIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	report := LintReport{
		Rules:          2,
		RuleViolations: []LintProblems{{File: "ai/rules/x.md", Problems: []string{"bad"}}},
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshaling the report: %v", err)
	}
	for _, key := range []string{`"rules"`, `"rule-violations"`, `"points"`, `"point-violations"`, `"empty"`, `"file"`, `"problems"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}
	if strings.Contains(string(raw), "_") {
		t.Errorf("a JSON key is not kebab-case: %s", raw)
	}
}

func TestTheLintPageIsTheScriptsPage(t *testing.T) {
	clean := LintReport{Rules: 29, Points: 2310}
	want := "rules_lint: 29 rule file(s) conform to ai/rules/rule-format.md\n" +
		"rules_lint: 2310 rule point(s) state an RFC 2119 level\n"
	if got := clean.Text(); got != want {
		t.Errorf("the clean page is\n%q\nwant\n%q", got, want)
	}

	failing := LintReport{
		Rules:          29,
		RuleViolations: []LintProblems{{File: "ai/rules/x.md", Problems: []string{"one", "two"}}},
	}
	want = "rules_lint: 1/29 rule file(s) violate the format\n\n" +
		"  ai/rules/x.md\n      - one\n      - two\n" +
		"\nFormat spec: ai/rules/rule-format.md\n"
	if got := failing.Text(); got != want {
		t.Errorf("the failing page is\n%q\nwant\n%q", got, want)
	}
}
