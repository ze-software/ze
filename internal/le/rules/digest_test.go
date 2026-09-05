// VALIDATES: spec-le-is-a-ze-binary AC-1, AC-5, AC-7, and AC-8. Function calls
// invoke the six gates ported from rules_index.py, rules_condensed.py, and
// rules_router.py.
// PREVENTS: success over an unread corpus or a routing index without its source
// ladder. Every session loads both artifacts. An empty artifact is not a smaller
// instruction set. It is an absent instruction set with a false pass.

package rules

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// digestTree writes a fixture checkout and answers its root. Paths are
// repo-relative.
func digestTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
	return root
}

// ladder is a precedence rule whose table the core derivation can read. The
// rungs name two rules the fixture also carries, so the core is non-empty.
const ladder = `# Rule Precedence

**When:** when two rules disagree
**Severity:** blocking

## The Ladder

| Rung | Governs | Rules | What it does |
|------|---------|-------|--------------|
| 1 | Destruction | ` + "`never-destroy-work`" + ` | STOP and ask |
| 2 | Outside correctness | ` + "`rfc-compliance`" + ` | Implement it |
| 3 | Scope | ` + "`completion`" + ` | Never reduce scope |
`

// digestCorpus is the smallest fixture whose ladder resolves: the precedence
// rule, the two rules its rungs name, and one routed rule.
func digestCorpus() map[string]string {
	return map[string]string{
		"ai/rules/rule-precedence.md": ladder,
		"ai/rules/never-destroy-work.md": "# Never Destroy Work\n\n" +
			"**When:** before deleting a file holding uncommitted work\n" +
			"**Severity:** blocking\n\n## Directives\n\n- MUST ask first.\n",
		"ai/rules/rfc-compliance.md": "# RFC Compliance\n\n" +
			"**When:** writing protocol code for any RFC ze implements\n" +
			"**Severity:** blocking\n\n## Directives\n\n- MUST conform.\n",
		"ai/rules/completion.md": "# Completion\n\n" +
			"**When:** before claiming any gokrazy qdisc work done\n" +
			"**Severity:** blocking\n\n## Directives\n\n- MUST finish it.\n",
	}
}

func TestTheRuleParseSeparatesTitleMetadataAndBody(t *testing.T) {
	root := digestTree(t, digestCorpus())

	rules, err := loadRules(rulesDir(root))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	if len(rules) != 4 {
		t.Fatalf("read %d rules, want 4", len(rules))
	}

	var completion Rule
	for _, rule := range rules {
		if rule.Stem == "completion" {
			completion = rule
		}
	}
	if completion.Title != "Completion" {
		t.Errorf("title = %q", completion.Title)
	}
	if completion.Severity != "blocking" {
		t.Errorf("severity = %q", completion.Severity)
	}
	if completion.Trigger != "before claiming any gokrazy qdisc work done" {
		t.Errorf("trigger = %q", completion.Trigger)
	}
	// The metadata block must not leak into the body: a paragraph-level match
	// swallowed Severity into the When text before parseRule existed.
	if strings.Contains(strings.Join(completion.Body, "\n"), "**Severity:**") {
		t.Errorf("the metadata block reached the body: %q", completion.Body)
	}
	if !strings.Contains(strings.Join(completion.Body, "\n"), "MUST finish it") {
		t.Errorf("the body lost its directive: %q", completion.Body)
	}

	// The block contains three keys. Another bold key ENDS it, so that line
	// belongs to the body. The rendered CORE.md metadata contains only canonical
	// keys.
	files := digestCorpus()
	files["ai/rules/completion.md"] = "# Completion\n\n" +
		"**When:** before claiming any gokrazy qdisc work done\n" +
		"**Severity:** blocking\n**Note:** this key is not canonical\n" +
		"**Related:** never read, the block ended above\n\n" +
		"## Directives\n\n- MUST finish it.\n"
	extra, err := loadRules(rulesDir(digestTree(t, files)))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	for i := range extra {
		rule := &extra[i]
		if rule.Stem != "completion" {
			continue
		}
		if len(rule.Meta) != 2 {
			t.Errorf("the block took %d keys, want the two above the non-canonical one: %+v",
				len(rule.Meta), rule.Meta)
		}
		if !strings.Contains(strings.Join(rule.Body, "\n"), "not canonical") {
			t.Errorf("a non-canonical key was eaten rather than left in the body: %q", rule.Body)
		}
		// The rendered metadata LINE is the third of the block, and it carries
		// the canonical keys only. The non-canonical one is body, so it is
		// rendered as the bold directive line it now is.
		block := strings.Split(ruleBlock(rule), "\n")
		if len(block) < 3 {
			t.Fatalf("the rendered block has %d lines: %q", len(block), block)
		}
		if strings.Contains(block[2], "**Note:**") {
			t.Errorf("the metadata line carries a key that is not one: %q", block[2])
		}
		if !strings.Contains(ruleBlock(rule), "**Note:** this key is not canonical") {
			t.Errorf("the non-canonical key was dropped rather than left as body: %q", ruleBlock(rule))
		}
	}
}

func TestTheCoreIsDerivedFromTheLadderRatherThanFromAList(t *testing.T) {
	root := digestTree(t, digestCorpus())
	rules, err := loadRules(rulesDir(root))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}

	core, err := coreMembers(rules, taskCorpus{}, nil)
	if err != nil {
		t.Fatalf("CoreMembers: %v", err)
	}
	got := coreNames(core)
	for _, want := range []string{"never-destroy-work.md", "rfc-compliance.md", "rule-precedence.md"} {
		if !got[want] {
			t.Errorf("%s is not in the core: %v", want, sortedNames(got))
		}
	}
	if got["completion.md"] {
		t.Errorf("a rung 3 rule reached the core: %v", sortedNames(got))
	}

	// The derivation follows the LADDER, so moving a rule between rungs moves
	// it into and out of the core with no edit here.
	files := digestCorpus()
	files["ai/rules/rule-precedence.md"] = strings.Replace(ladder,
		"| 3 | Scope | `completion` |", "| 2 | Scope | `completion` |", 1)
	moved := digestTree(t, files)
	rules, err = loadRules(rulesDir(moved))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	core, err = coreMembers(rules, taskCorpus{}, nil)
	if err != nil {
		t.Fatalf("CoreMembers: %v", err)
	}
	if !coreNames(core)["completion.md"] {
		t.Errorf("moving the rule to rung 2 did not move it into the core: %v",
			sortedNames(coreNames(core)))
	}
}

func TestARewordedLadderRefusesRatherThanEmptyingTheCore(t *testing.T) {
	cases := []struct {
		name   string
		ladder string
		want   string
	}{
		{
			name:   "the Rung column is renamed",
			ladder: strings.Replace(ladder, "| Rung | Governs |", "| Level | Governs |", 1),
			want:   "found by header text",
		},
		{
			name: "the ladder holds no rung 1 or 2 row",
			ladder: strings.NewReplacer("| 1 | Destruction", "| 4 | Destruction",
				"| 2 | Outside correctness", "| 5 | Outside correctness").Replace(ladder),
			want: "has no rung 1/2 row",
		},
		{
			name: "the rungs name no rule under ai/rules",
			ladder: strings.NewReplacer("`never-destroy-work`", "`CLAUDE.md`",
				"`rfc-compliance`", "`AGENTS.md`").Replace(ladder),
			want: "names no",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := digestCorpus()
			files["ai/rules/rule-precedence.md"] = tc.ladder
			root := digestTree(t, files)
			rules, err := loadRules(rulesDir(root))
			if err != nil {
				t.Fatalf("LoadRules: %v", err)
			}
			core, err := coreMembers(rules, taskCorpus{}, nil)
			if err == nil {
				t.Fatalf("the core was derived from an unreadable ladder: %v",
					sortedNames(coreNames(core)))
			}
			var ladderErr *ladderError
			if !asLadderError(err, &ladderErr) {
				t.Fatalf("error is %T, want *LadderError", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to name %q", err, tc.want)
			}
		})
	}
}

// asLadderError spells out errors.As so the test states its required type. This
// avoids an import of errors for one call.
func asLadderError(err error, target **ladderError) bool {
	le, ok := err.(*ladderError) //nolint:errorlint // the port wraps nothing
	if ok {
		*target = le
	}
	return ok
}

func TestATableAfterTheLadderIsNotReadWithTheLaddersColumns(t *testing.T) {
	// A markdown table cannot contain gaps, so a non-row line ends it. Without
	// an index reset, retained column indexes parse a LATER table with this
	// table's layout. An unrelated rule then enters the core without a reason.
	files := digestCorpus()
	files["ai/rules/rule-precedence.md"] = ladder + "\n## Deferral\n\n" +
		"| Step | Note | Rules | Detail |\n" +
		"|------|------|-------|--------|\n" +
		"| 2 | home it | `completion` | write the spec |\n"
	root := digestTree(t, files)

	rules, err := loadRules(rulesDir(root))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	core, err := coreMembers(rules, taskCorpus{}, nil)
	if err != nil {
		t.Fatalf("CoreMembers: %v", err)
	}
	if coreNames(core)["completion.md"] {
		t.Errorf("a second table's row was read with the ladder's columns: %v",
			sortedNames(coreNames(core)))
	}
}

func TestATriggerWithNoTermToMatchGoesIntoTheCore(t *testing.T) {
	files := digestCorpus()
	// Every word of this trigger is a stopword, so nothing survives the
	// tokenizer and no task can ever surface the rule.
	files["ai/rules/completion.md"] = "# Completion\n\n" +
		"**When:** when it is not the same as any of them\n" +
		"**Severity:** blocking\n\n## Directives\n\n- MUST finish it.\n"
	root := digestTree(t, files)

	rules, err := loadRules(rulesDir(root))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	core, err := coreMembers(rules, taskCorpus{}, nil)
	if err != nil {
		t.Fatalf("CoreMembers: %v", err)
	}
	reason := ""
	for _, rule := range core {
		if rule.Name == "completion.md" {
			reason = rule.CoreReason
		}
	}
	if reason != "trigger carries no term to match" {
		t.Errorf("core reason = %q, want the unroutable one", reason)
	}
}

func TestAMissingTriggerAndAnUnknownSeverityBothLandInTheCore(t *testing.T) {
	files := digestCorpus()
	files["ai/rules/no-trigger.md"] = "# No Trigger\n\n**Severity:** blocking\n\n## D\n\n- MUST.\n"
	files["ai/rules/odd-severity.md"] = "# Odd\n\n**When:** touching the qdisc encoder\n" +
		"**Severity:** urgent\n\n## D\n\n- MUST.\n"
	root := digestTree(t, files)

	rules, err := loadRules(rulesDir(root))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	core, err := coreMembers(rules, taskCorpus{}, nil)
	if err != nil {
		t.Fatalf("CoreMembers: %v", err)
	}
	want := map[string]string{
		"no-trigger.md":   "no trigger to route on",
		"odd-severity.md": "no severity to route on",
	}
	for _, rule := range core {
		if reason, ok := want[rule.Name]; ok {
			if rule.CoreReason != reason {
				t.Errorf("%s reason = %q, want %q", rule.Name, rule.CoreReason, reason)
			}
			delete(want, rule.Name)
		}
	}
	if len(want) > 0 {
		t.Errorf("these never reached the core: %v", want)
	}

	// An unclassified severity is still NAMED in the trigger index, because the
	// one thing it promises is that no rule goes unlisted.
	line := ""
	for _, row := range triggerLines(rules, coreNames(core)) {
		if strings.Contains(row, "odd-severity.md") {
			line = row
		}
	}
	if !strings.Contains(line, "unclassified, always-on") {
		t.Errorf("the row does not say what it could not classify: %q", line)
	}
}

func TestATriggerRowIsCutAtAWordBoundaryAndCountsRunes(t *testing.T) {
	// The trigger is padded past the row bound with em dashes, which cost three
	// bytes and one character each. A byte bound would cut one in half and
	// answer invalid UTF-8.
	long := "when the encoder meets " + strings.Repeat("an em dash — ", 30)
	files := digestCorpus()
	files["ai/rules/completion.md"] = "# Completion\n\n**When:** " + long +
		"\n**Severity:** blocking\n\n## D\n\n- MUST.\n"
	root := digestTree(t, files)

	rules, err := loadRules(rulesDir(root))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	row := ""
	for _, line := range triggerLines(rules, nil) {
		if strings.Contains(line, "completion.md") {
			row = line
		}
	}
	if runeLen(row) > maxTriggerLine {
		t.Errorf("the row is %d characters, over the %d bound: %q", runeLen(row), maxTriggerLine, row)
	}
	if !strings.HasSuffix(row, "... |") {
		t.Errorf("a cut row does not say it was cut: %q", row)
	}
	if strings.ContainsRune(row, '�') {
		t.Errorf("the cut split a rune: %q", row)
	}
	// The cut lands on a word boundary, so no half word survives it.
	if strings.Contains(row, "da... |") || strings.Contains(row, "em... |") {
		t.Errorf("the cut split a word: %q", row)
	}
}

func TestWhitespaceCollapsesToOneSpaceWithNoOuterRun(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  a   b  ", "a b"},
		{"a\t\tb", "a b"},
		{"a\nb", "a b"},
		{"   ", ""},
		{"a", "a"},
	}
	for _, tc := range cases {
		if got := collapseSpaceTrimmed(tc.in); got != tc.want {
			t.Errorf("collapseSpaceTrimmed(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSignificantTermsSplitsCompoundsAndDropsTheSharedVocabulary(t *testing.T) {
	got := significantTerms("Wire-Encoding on the HOT path, and any qdisc")
	for _, want := range []string{"wire-encoding", "wire", "encoding", "hot", "path", "qdisc"} {
		if !got[want] {
			t.Errorf("%q is not a term of %v", want, sortedNames(got))
		}
	}
	for _, unwanted := range []string{"the", "and", "any", "on"} {
		if got[unwanted] {
			t.Errorf("the stopword %q survived: %v", unwanted, sortedNames(got))
		}
	}
}

func TestADistinctiveTermIsOneFewThanMaxTriggerDFTriggersShare(t *testing.T) {
	// The bound is on the DOCUMENT FREQUENCY of a term, so a word carried by
	// more than maxTriggerDF triggers separates nothing and is dropped. This
	// pins the boundary rather than the constant: maxTriggerDF+1 rules sharing
	// a word lose it, and maxTriggerDF keep it.
	build := func(count int) map[string]map[string]bool {
		rules := make([]Rule, 0, count)
		for i := range count {
			name := string(rune('a'+i)) + ".md"
			rules = append(rules, Rule{Name: name, Trigger: "shared qdisc" + name})
		}
		return distinctiveTerms(rules)
	}
	if terms := build(maxTriggerDF); !terms["a.md"]["shared"] {
		t.Errorf("a word %d triggers share was dropped: %v", maxTriggerDF, sortedNames(terms["a.md"]))
	}
	if terms := build(maxTriggerDF + 1); terms["a.md"]["shared"] {
		t.Errorf("a word %d triggers share survived: %v", maxTriggerDF+1, sortedNames(terms["a.md"]))
	}
}

func TestADeniedSectionDropsWholeAndItsSiblingSurvives(t *testing.T) {
	body := []string{
		"## Directives",
		"",
		"- MUST keep this.",
		"",
		"## Why This Matters (BLOCKING)",
		"",
		"- This bullet is explanation and must not survive.",
		"",
		"## (BLOCKING) Rationale",
		"",
		"- A leading parenthetical must not make the section survive.",
		"",
		"## Notes",
		"",
		"- An ambiguous heading keeps its directive.",
	}
	got := strings.Join(condenseBody(body), "\n")
	if !strings.Contains(got, "MUST keep this") {
		t.Errorf("a directive was dropped: %q", got)
	}
	if strings.Contains(got, "must not survive") || strings.Contains(got, "Why This Matters") {
		t.Errorf("a denylisted section survived: %q", got)
	}
	// The denylist reads the first word of the heading with its parentheticals
	// removed, so a leading `(BLOCKING)` does not shelter a Rationale section.
	if strings.Contains(got, "leading parenthetical") {
		t.Errorf("a parenthetical sheltered a denylisted section: %q", got)
	}
	if !strings.Contains(got, "ambiguous heading keeps its directive") {
		t.Errorf("an ambiguous heading was dropped: %q", got)
	}
}

func TestACondensedSectionKeepsOneProseSentenceAndDropsTheFence(t *testing.T) {
	body := []string{
		"## Directives",
		"",
		"The first sentence is the rule statement. The second is elaboration.",
		"",
		"A second paragraph must not be kept.",
		"",
		"```",
		"- MUST not survive: this bullet is inside a fence.",
		"```",
		"",
		"Rationale: this pointer line is never directive material.",
	}
	got := strings.Join(condenseBody(body), "\n")
	if !strings.Contains(got, "The first sentence is the rule statement.") {
		t.Errorf("the rule statement was dropped: %q", got)
	}
	for _, unwanted := range []string{"elaboration", "second paragraph", "inside a fence", "pointer line"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("%q survived the condense: %q", unwanted, got)
		}
	}
}

func TestFirstSentenceCutsAtRunesAndAtAWordBoundary(t *testing.T) {
	// No sentence end, and every character is an em dash pair, so the bound is
	// reached at a rune count far below the byte count.
	long := strings.Repeat("dash — ", 60)
	got := firstSentence(long)
	if runeLen(got) > maxProse {
		t.Errorf("the cut kept %d characters, over the %d bound", runeLen(got), maxProse)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("a cut sentence does not say it was cut: %q", got)
	}
	if strings.ContainsRune(got, '�') {
		t.Errorf("the cut split a rune: %q", got)
	}
	if sentence := firstSentence("One. Two."); sentence != "One." {
		t.Errorf("firstSentence = %q, want the first sentence only", sentence)
	}
	// A period inside a word is not a sentence end, because the next character
	// is not whitespace.
	if sentence := firstSentence("ai/rules/cli.md governs it. Next."); sentence != "ai/rules/cli.md governs it." {
		t.Errorf("firstSentence = %q, want the period inside the path ignored", sentence)
	}

	// The pair that discriminates: under the bound in characters, over it in
	// bytes. A byte bound cuts a rule statement that fits, and the directive
	// loses its second half.
	dashes := strings.Repeat("— ", maxProse/2)
	if runeLen(dashes) > maxProse || len(dashes) <= maxProse {
		t.Fatalf("the fixture is %d characters and %d bytes, which discriminates nothing",
			runeLen(dashes), len(dashes))
	}
	if kept := firstSentence(dashes); strings.HasSuffix(kept, "...") {
		t.Errorf("a paragraph under the character bound was cut: %q", kept)
	}
}

func TestTheIndexSummaryPrefersTheTriggerThenBlockingThenProse(t *testing.T) {
	cases := []struct {
		name string
		rule string
		want string
	}{
		{
			name: "an explicit trigger wins",
			rule: "# R\n\n**When:** when the qdisc encoder changes\n**Severity:** blocking\n\n" +
				"**BLOCKING:** do not read this one.\n",
			want: "when the qdisc encoder changes",
		},
		{
			name: "a BLOCKING directive beats prose",
			rule: "# R\n\nThis prose comes first in the file.\n\n**BLOCKING:** run the gate first.\n",
			want: "run the gate first.",
		},
		{
			name: "prose is the last resort",
			rule: "# R\n\n| a | b |\n\nThis prose is the rule statement.\n",
			want: "This prose is the rule statement.",
		},
		{
			name: "a rule with nothing derivable answers nothing",
			rule: "# R\n\n- a bullet\n\n| a | b |\n",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := indexSummary(splitLines(tc.rule)); got != tc.want {
				t.Errorf("indexSummary = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTheIndexSummaryProtectsCodeSpansAndEscapesPipes(t *testing.T) {
	// A blanket bold strip corrupts globs: the trigger of testing.md carries
	// `test/**/*.ci`, which rendered as `test//*.ci` until code spans were
	// protected.
	got := cleanSummary("**BLOCKING:** every `test/**/*.ci` and **every** row | cell")
	if !strings.Contains(got, "`test/**/*.ci`") {
		t.Errorf("the glob inside a code span was corrupted: %q", got)
	}
	if strings.Contains(got, "**every**") {
		t.Errorf("a bold marker outside a code span survived: %q", got)
	}
	if strings.Contains(got, "row | cell") {
		t.Errorf("an unescaped pipe would break the table row: %q", got)
	}
	if !strings.Contains(got, `row \| cell`) {
		t.Errorf("the pipe was not escaped: %q", got)
	}
	if strings.HasPrefix(got, "**BLOCKING") {
		t.Errorf("the marker prefix survived: %q", got)
	}
}

func TestTheIndexSummaryIsCutAtRunesNotBytes(t *testing.T) {
	long := "when " + strings.Repeat("an em dash — ", 40)
	got := cleanSummary(long)
	if runeLen(got) > maxSummary {
		t.Errorf("the summary kept %d characters, over the %d bound", runeLen(got), maxSummary)
	}
	if strings.ContainsRune(got, '�') {
		t.Errorf("the cut split a rune: %q", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("a cut summary does not say it was cut: %q", got)
	}

	// This pair distinguishes the implementations. The summary is UNDER the
	// character bound and OVER the byte bound. A byte bound wrongly removes its
	// final clause. A summary over both bounds proves nothing because both forms
	// remove it.
	dashes := strings.Repeat("— ", maxSummary/2)
	if runeLen(dashes) > maxSummary {
		t.Fatalf("the fixture is %d characters, not under the bound", runeLen(dashes))
	}
	if len(dashes) <= maxSummary {
		t.Fatalf("the fixture is %d bytes, so a byte bound would not cut it", len(dashes))
	}
	if kept := cleanSummary(dashes); strings.HasSuffix(kept, "...") {
		t.Errorf("a summary under the character bound was cut: %d characters, %d bytes",
			runeLen(dashes), len(dashes))
	}
}

func TestTheIndexNamesEveryRuleWithNoDerivableSummary(t *testing.T) {
	files := digestCorpus()
	files["ai/rules/quiet.md"] = "# Quiet\n\n- only a bullet\n"
	root := digestTree(t, files)

	report, err := Index(root, true)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if len(report.Missing) != 1 || report.Missing[0] != "quiet.md" {
		t.Fatalf("missing = %v, want [quiet.md]", report.Missing)
	}
	if !report.Failed() {
		t.Errorf("a rule with no trigger did not fail the check")
	}
	if !strings.Contains(report.Text(), "missing a derivable summary") {
		t.Errorf("the page does not name the problem: %q", report.Text())
	}
	// The row still exists. Dropping it would hide exactly the rules that are
	// hardest to route.
	found := false
	for _, row := range report.Rows {
		if row.Rule != "Quiet" {
			continue
		}
		found = true
		if row.Summary != noSummary {
			t.Errorf("the row's summary = %q, want the placeholder", row.Summary)
		}
		// A rule without a metadata block still gets a severity CELL. An empty
		// cell makes the table harder to scan. This rule especially needs a row
		// because it has no routing text.
		if row.Severity != "-" {
			t.Errorf("the row's severity = %q, want the placeholder", row.Severity)
		}
	}
	if !found {
		t.Errorf("the unroutable rule has no row: %v", report.Rows)
	}
}

func TestARungIsAnASCIINumberAndNothingElse(t *testing.T) {
	// Python's str.isdigit() and int() accept fullwidth and Arabic-Indic digits.
	// Thus, the script treats them as rungs. This implementation fails CLOSED.
	// A ladder without an ASCII rung number produces a ladderError instead of a
	// core derived from a typo.
	cases := []struct {
		cell  string
		value int
		ok    bool
	}{
		{"1", 1, true},
		{"12", 12, true},
		{"", 0, false},
		{"1a", 0, false},
		{" 1", 0, false},
		{"１", 0, false},
		{"-1", 0, false},
	}
	for _, tc := range cases {
		value, ok := asciiDigits(tc.cell)
		if ok != tc.ok || value != tc.value {
			t.Errorf("asciiDigits(%q) = %d, %v; want %d, %v", tc.cell, value, ok, tc.value, tc.ok)
		}
	}
}

func TestTheIndexRefusesARulesDirectoryItReadNothingFrom(t *testing.T) {
	// The script writes a header with no rows over the real index and exits 0,
	// so an empty read is what success looks like. Here it is an error, and the
	// file on disk is untouched.
	root := digestTree(t, map[string]string{
		"ai/rules/INDEX.md": "# Ze Rules Index\n\ncontent that must survive\n",
	})
	if _, err := Index(root, false); err == nil {
		t.Fatalf("the update reported success over a corpus it read nothing from")
	}
	body, err := os.ReadFile(filepath.Join(root, "ai", "rules", "INDEX.md"))
	if err != nil {
		t.Fatalf("reading the index back: %v", err)
	}
	if !strings.Contains(string(body), "content that must survive") {
		t.Errorf("the refused update overwrote the index anyway: %q", body)
	}
}

func TestTheDigestRefusesARulesDirectoryItReadNothingFrom(t *testing.T) {
	root := digestTree(t, map[string]string{"ai/rules/TRIGGERS.md": "stale\n"})
	_, err := Digest(root, true)
	if err == nil {
		t.Fatalf("the digest reported a verdict over a corpus it read nothing from")
	}
	// The message must name the EMPTY READ. Without this guard, the ladder error
	// causes the failure. That message points to rule-precedence.md, which is
	// missing only because the function read nothing.
	if !strings.Contains(err.Error(), "read nothing") {
		t.Errorf("the refusal blames the wrong thing: %v", err)
	}
}

func TestTheDigestWritesBothArtifactsAndTheCheckThenPasses(t *testing.T) {
	root := digestTree(t, digestCorpus())

	stale, err := Digest(root, true)
	if err != nil {
		t.Fatalf("Digest check: %v", err)
	}
	if !stale.Failed() {
		t.Fatalf("an absent artifact did not read as stale: %+v", stale.Artifacts)
	}
	if !strings.Contains(stale.Text(), "./le rules condensed-update") {
		t.Errorf("the page does not name the fix: %q", stale.Text())
	}

	written, err := Digest(root, false)
	if err != nil {
		t.Fatalf("Digest update: %v", err)
	}
	if !written.Written || len(written.Artifacts) != 2 {
		t.Fatalf("update wrote %+v", written)
	}

	fresh, err := Digest(root, true)
	if err != nil {
		t.Fatalf("Digest recheck: %v", err)
	}
	if fresh.Failed() {
		t.Errorf("the check is still stale after the update: %q", fresh.Text())
	}

	// A drift of one word is drift. Comparing sizes would call this tree fresh,
	// and the trigger index every session loads would keep a row nobody
	// generated.
	triggers := filepath.Join(root, "ai", "rules", "TRIGGERS.md")
	body, err := os.ReadFile(triggers)
	if err != nil {
		t.Fatalf("reading TRIGGERS.md: %v", err)
	}
	edited := strings.Replace(string(body), "blocking", "advisory", 1)
	if edited == string(body) || len(edited) != len(body) {
		t.Fatalf("the edit is not the same size, so it discriminates nothing")
	}
	if err := os.WriteFile(triggers, []byte(edited), 0o600); err != nil {
		t.Fatalf("writing TRIGGERS.md: %v", err)
	}
	drifted, err := Digest(root, true)
	if err != nil {
		t.Fatalf("Digest recheck: %v", err)
	}
	if !drifted.Failed() {
		t.Errorf("a same-size edit read as fresh: %q", drifted.Text())
	}
	if _, err := Digest(root, false); err != nil {
		t.Fatalf("Digest rewrite: %v", err)
	}

	core, err := os.ReadFile(filepath.Join(root, "ai", "rules", "CORE.md"))
	if err != nil {
		t.Fatalf("reading CORE.md: %v", err)
	}
	// The core carries the DIRECTIVES, not just the names: a core that named
	// its members and dropped their bodies would pass every count.
	if !strings.Contains(string(core), "MUST ask first") {
		t.Errorf("the core dropped a member's directive: %q", core)
	}
	if !strings.Contains(string(core), "<!-- always-on: precedence rung 1/2 -->") {
		t.Errorf("the core does not say why a member is eager: %q", core)
	}
	if strings.Contains(string(core), "## Completion") {
		t.Errorf("a routed rule reached the core: %q", core)
	}
}

func TestThePayloadRefusesToMeasureAFileThatIsNotThere(t *testing.T) {
	// The script counts an absent file as zero characters, so deleting CORE.md
	// is the cheapest way to make the budget look MET.
	root := digestTree(t, map[string]string{
		"ai/INSTRUCTIONS.md":   "instructions\n",
		"ai/rules/TRIGGERS.md": "triggers\n",
	})
	report, err := Payload(root)
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}
	if !report.Failed() {
		t.Fatalf("an absent payload file passed: %+v", report)
	}
	if len(report.Missing) != 1 || report.Missing[0] != "ai/rules/CORE.md" {
		t.Errorf("missing = %v", report.Missing)
	}
	if !report.Met {
		t.Errorf("the fixture is under budget; the verdict is what the WARNING qualifies")
	}
	if !strings.Contains(report.Text(), "ai/rules/CORE.md") {
		t.Errorf("the page does not name the absent file: %q", report.Text())
	}

	whole, err := Payload(digestTree(t, map[string]string{
		"ai/INSTRUCTIONS.md":   "instructions\n",
		"ai/rules/TRIGGERS.md": "triggers\n",
		"ai/rules/CORE.md":     "core\n",
	}))
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}
	if whole.Failed() {
		t.Errorf("a complete payload failed: %+v", whole)
	}
	if whole.Chars != len("instructions\n")+len("triggers\n")+len("core\n") {
		t.Errorf("chars = %d", whole.Chars)
	}
}

func TestTheRouterSubtractsACoreDerivedWithoutTheCorpus(t *testing.T) {
	files := digestCorpus()
	files["plan/spec-thing.md"] = "# Spec\n\n## Task\n\nRewrite the gokrazy qdisc encoder.\n"
	root := digestTree(t, files)

	report, err := Router(root)
	if err != nil {
		t.Fatalf("Router: %v", err)
	}
	if report.CorpusSize != 1 {
		t.Fatalf("corpus size = %d, want 1", report.CorpusSize)
	}
	if report.RulesTotal != 4 || report.Routed != 1 {
		t.Fatalf("rules = %d, routed = %d", report.RulesTotal, report.Routed)
	}
	// completion.md's trigger carries `gokrazy` and `qdisc`, which the one task
	// also carries, so it is surfaced and nothing is missed.
	if len(report.MissedBlocking) != 0 {
		t.Errorf("missed = %v, want none", report.MissedBlocking)
	}
	if len(report.Tasks) != 1 || len(report.Tasks[0].Surfaced) != 1 {
		t.Fatalf("tasks = %+v", report.Tasks)
	}

	// One shared term is not enough: the threshold is minHits distinctive
	// words, because a single shared word routes everything.
	files["plan/spec-thing.md"] = "# Spec\n\n## Task\n\nRewrite the gokrazy encoder.\n"
	thin := digestTree(t, files)
	thinReport, err := Router(thin)
	if err != nil {
		t.Fatalf("Router: %v", err)
	}
	if len(thinReport.MissedBlocking) != 1 {
		t.Errorf("one distinctive word surfaced the rule: %+v", thinReport.Tasks)
	}
	if !strings.Contains(thinReport.Text(), "MISSED: 1 blocking rule(s)") {
		t.Errorf("the page does not name the miss: %q", thinReport.Text())
	}
}

func TestTheRouterReadsOnlyTheSectionsThatStateTheWork(t *testing.T) {
	files := digestCorpus()
	files["plan/spec-thing.md"] = "# Spec\n\n## Summary\n\nqdisc gokrazy elsewhere\n\n" +
		"## Task\n\nthe real statement\n\n### Sub\n\nstill inside the task\n\n" +
		"## Risks\n\nqdisc gokrazy again\n"
	files["plan/TEMPLATE.md"] = "# T\n\n## Task\n\ntemplate text\n"
	files["plan/RECURRING.md"] = "# R\n\n## Task\n\nall caps stem\n"
	root := digestTree(t, files)

	corpus, err := loadCorpus(root)
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	if len(corpus) != 1 || corpus[0].Source != "spec-thing.md" {
		t.Fatalf("corpus = %+v, want the one spec", corpus)
	}
	if !strings.Contains(corpus[0].Text, "still inside the task") {
		t.Errorf("a deeper heading closed the section: %q", corpus[0].Text)
	}
	for _, unwanted := range []string{"elsewhere", "again"} {
		if strings.Contains(corpus[0].Text, unwanted) {
			t.Errorf("a section that states no work was read: %q", corpus[0].Text)
		}
	}
}

func TestAnEmptyCorpusIsReportedRatherThanPassingForAResult(t *testing.T) {
	root := digestTree(t, digestCorpus())
	report, err := Digest(root, false)
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if !report.EmptyCorpus {
		t.Errorf("a tree with no plan/ did not report an empty corpus: %+v", report)
	}

	files := digestCorpus()
	files["plan/spec-thing.md"] = "# Spec\n\n## Task\n\nRewrite the gokrazy qdisc encoder.\n"
	full, err := Digest(digestTree(t, files), false)
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if full.EmptyCorpus {
		t.Errorf("a tree with a spec reported an empty corpus: %+v", full)
	}
}

func TestTheEmptyCorpusAnswerWarnsOnStderrAndStillAnswersZero(t *testing.T) {
	// The fact travels twice on purpose: in the payload, so `| json` reaches
	// it, and on stderr, so a developer watching a terminal sees it. A payload
	// field alone would be invisible to the person running the gate.
	root := digestTree(t, digestCorpus())
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	saved := os.Stderr
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = write
	payload, code := condensedAnswer(false)
	os.Stderr = saved
	if err := write.Close(); err != nil {
		t.Fatalf("closing the pipe: %v", err)
	}
	captured, err := io.ReadAll(read)
	if err != nil {
		t.Fatalf("reading the pipe: %v", err)
	}

	// The run SUCCEEDED: both artifacts were written, and one derivation of the
	// core is what was lost.
	if code != 0 {
		t.Errorf("the update answered %d over a tree with no plan/", code)
	}
	report, ok := payload.(DigestReport)
	if !ok || !report.EmptyCorpus {
		t.Errorf("the payload does not carry the empty corpus: %+v", payload)
	}
	line := strings.TrimSpace(string(captured))
	if line != "warning: "+emptyCorpusWarning {
		t.Errorf("stderr = %q, want the warning line", line)
	}
	if strings.HasPrefix(line, "error:") {
		t.Errorf("a run that answered 0 called itself an error: %q", line)
	}
}

func TestTheNewReportsAreStructuredDataWithKebabCaseKeys(t *testing.T) {
	files := digestCorpus()
	files["plan/spec-thing.md"] = "# Spec\n\n## Task\n\nRewrite the gokrazy qdisc encoder.\n"
	root := digestTree(t, files)

	index, err := Index(root, true)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	digest, err := Digest(root, true)
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	payload, err := Payload(root)
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}
	router, err := Router(root)
	if err != nil {
		t.Fatalf("Router: %v", err)
	}

	cases := []struct {
		name    string
		payload any
		keys    []string
	}{
		{"index", index, []string{"file", "rules", "written", "stale", "missing", "rows"}},
		{"digest", digest, []string{"written", "empty-corpus", "artifacts"}},
		{"payload", payload, []string{"parts", "chars", "tokens", "budget", "met", "headroom-percent", "missing"}},
		{"router", router, []string{"tasks", "corpus-size", "rules-total", "core", "routed",
			"blocking-routed", "surfaced-any", "missed-blocking", "unroutable-terms"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("the answer does not encode: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("the answer is not an object: %v", err)
			}
			for _, key := range tc.keys {
				if _, ok := got[key]; !ok {
					t.Errorf("no %q key: %v", key, jsonKeys(got))
				}
			}
			for key := range got {
				if strings.ToLower(key) != key || strings.Contains(key, "_") {
					t.Errorf("key %q is not kebab-case", key)
				}
			}
		})
	}
}

// jsonKeys names an object's keys for a failure message.
func jsonKeys(object map[string]any) []string {
	set := make(map[string]bool, len(object))
	for key := range object {
		set[key] = true
	}
	return sortedNames(set)
}
