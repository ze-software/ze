// VALIDATES: spec-le-is-a-ze-binary AC-1, AC-5, AC-7, and AC-8. Function calls
// invoke the gates ported from rules_condensed.py and rules_router.py.
// PREVENTS: success over an unread corpus or a routing index without its source
// ladder. Every session loads both artifacts. An empty artifact is not a smaller
// instruction set. It is an absent instruction set with a false pass.

package airules

import (
	"encoding/json"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/ai/rules/ruleset"
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

// situatedCorpus is digestCorpus with rfc-compliance, a core member, also
// carried as points: one point shown everywhere, and one that declares the
// situation applies names, or declares none when applies is empty.
func situatedCorpus(applies string) map[string]string {
	files := digestCorpus()
	files["ai/rules/rfc-compliance.md"] = "# RFC Compliance\n\n" +
		"**When:** writing protocol code for any RFC ze implements\n" +
		"**Severity:** blocking\n\n## Directives\n\n- MUST conform.\n\n- MUST walk the requirement list.\n"
	files["ai/rules/points/rfc-compliance/manifest.md"] = "---\ntitle: RFC Compliance\n" +
		"when: writing protocol code for any RFC ze implements\nseverity: blocking\n---\n" +
		"directives ## Directives\n  conform\n  walk\n"
	files["ai/rules/points/rfc-compliance/directives/conform.md"] =
		"---\nkind: directive\nlevel: MUST\nstage:\n---\n- MUST conform.\n"
	head := "---\nkind: directive\nlevel: MUST\nstage:\n"
	if applies != "" {
		head += "applies: " + applies + "\n"
	}
	files["ai/rules/points/rfc-compliance/directives/walk.md"] = head + "---\n- MUST walk the requirement list.\n"
	return files
}

// coreBlockOf answers the block CORE.md shows for the rule titled title.
func coreBlockOf(t *testing.T, root, title string) string {
	t.Helper()
	core, err := os.ReadFile(filepath.Join(root, "ai", "rules", "CORE.md"))
	if err != nil {
		t.Fatalf("reading CORE.md: %v", err)
	}
	for block := range strings.SplitSeq(string(core), "\n\n---\n\n") {
		if strings.HasPrefix(block, "## "+title+"\n") {
			return block
		}
	}
	t.Fatalf("CORE.md shows no block for %q: %s", title, core)
	return ""
}

func TestTheCoreRefusesASituationsLineOverItsBound(t *testing.T) {
	// Goal: the line a core block ends with is bounded like a trigger row, and
	// one over the bound is refused naming the rule, never cut. Method: a
	// situation that fills the line exactly is accepted, one character more is
	// refused.
	room := maxSituationsLine - runeLen(situationsLine("ai/rules/rfc-compliance.md", nil))
	if _, err := Digest(digestTree(t, situatedCorpus(strings.Repeat("x", room))), false); err != nil {
		t.Fatalf("a line exactly at the bound was refused: %v", err)
	}
	_, err := Digest(digestTree(t, situatedCorpus(strings.Repeat("x", room+1))), false)
	if err == nil {
		t.Fatalf("a line over the bound was accepted")
	}
	for _, want := range []string{"rfc-compliance.md", "-character bound", "Fix: "} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
}

func TestTheCoreLeavesOutAPointThatDeclaresItsSituation(t *testing.T) {
	// Goal: AC-6. A core member's point that declares `applies` is left out of
	// the core, and the member's block ends with one line naming the rule file
	// and the situation; the point stays in the rule file and the rule in the
	// trigger index. Method: generate over a tree whose rfc-compliance points
	// mark one of two directives.
	root := digestTree(t, situatedCorpus("walking a requirement list"))
	if _, err := Digest(root, false); err != nil {
		t.Fatalf("Digest: %v", err)
	}
	block := coreBlockOf(t, root, "RFC Compliance")
	if !strings.Contains(block, "- MUST conform.") {
		t.Errorf("the point with no situation left the core: %q", block)
	}
	if strings.Contains(block, "walk the requirement list") {
		t.Errorf("the point that declares its situation is still in the core: %q", block)
	}
	shown := strings.TrimSuffix(strings.TrimSuffix(block, "\n"), "\n\n<!-- always-on: precedence rung 1/2 -->")
	lines := strings.Split(shown, "\n")
	closing := "**Read `ai/rules/rfc-compliance.md` before acting when:** walking a requirement list"
	if lines[len(lines)-1] != closing {
		t.Errorf("the block does not end with the line naming the file and the situation: %q", block)
	}
	rule, err := os.ReadFile(filepath.Join(root, "ai", "rules", "rfc-compliance.md"))
	if err != nil || !strings.Contains(string(rule), "MUST walk the requirement list.") {
		t.Errorf("the point left the rule file: %v %q", err, rule)
	}
	triggers, err := os.ReadFile(filepath.Join(root, "ai", "rules", "TRIGGERS.md"))
	if err != nil || !strings.Contains(string(triggers), "| `ai/rules/rfc-compliance.md` | blocking, always-on |") {
		t.Errorf("the rule left the trigger index: %v %q", err, triggers)
	}
}

func TestTheCoreShowsAPointWithNoSituation(t *testing.T) {
	// Goal: AC-5. A point with no `applies` is shown in the core as today, so
	// an absent field never narrows the core. Method: the core generated from a
	// tree whose points declare no situation is byte-identical to the core of
	// the same tree with no points at all, and it shows both directives.
	withPoints := digestTree(t, situatedCorpus(""))
	plain := situatedCorpus("")
	for rel := range plain {
		if strings.HasPrefix(rel, "ai/rules/points/") {
			delete(plain, rel)
		}
	}
	without := digestTree(t, plain)
	for _, root := range []string{withPoints, without} {
		if _, err := Digest(root, false); err != nil {
			t.Fatalf("Digest: %v", err)
		}
	}
	block := coreBlockOf(t, withPoints, "RFC Compliance")
	for _, want := range []string{"- MUST conform.", "- MUST walk the requirement list."} {
		if !strings.Contains(block, want) {
			t.Errorf("a point with no situation left the core, %q: %q", want, block)
		}
	}
	if strings.Contains(block, "before acting when") {
		t.Errorf("a block with no situated point names situations: %q", block)
	}
	if coreBlockOf(t, without, "RFC Compliance") != block {
		t.Errorf("points with no situation changed the core block")
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
		block := strings.Split(ruleBlock(rule, nil), "\n")
		if len(block) < 3 {
			t.Fatalf("the rendered block has %d lines: %q", len(block), block)
		}
		if strings.Contains(block[2], "**Note:**") {
			t.Errorf("the metadata line carries a key that is not one: %q", block[2])
		}
		if !strings.Contains(ruleBlock(rule, nil), "**Note:** this key is not canonical") {
			t.Errorf("the non-canonical key was dropped rather than left as body: %q", ruleBlock(rule, nil))
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
	rows, err := triggerLines(rules, coreNames(core))
	if err != nil {
		t.Fatalf("triggerLines: %v", err)
	}
	for _, row := range rows {
		if strings.Contains(row, "odd-severity.md") {
			line = row
		}
	}
	if !strings.Contains(line, "unclassified, always-on") {
		t.Errorf("the row does not say what it could not classify: %q", line)
	}
}

// completionTrigger loads the digest corpus with completion's trigger replaced,
// and answers what triggerLines makes of it.
func completionTrigger(t *testing.T, trigger string) ([]string, error) {
	t.Helper()
	files := digestCorpus()
	files["ai/rules/completion.md"] = "# Completion\n\n**When:** " + trigger +
		"\n**Severity:** blocking\n\n## D\n\n- MUST.\n"
	root := digestTree(t, files)

	rules, err := loadRules(rulesDir(root))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	return triggerLines(rules, nil)
}

func TestAnOverLongTriggerIsRefusedAndNeverCut(t *testing.T) {
	// Goal: a trigger longer than its row has room for is refused, naming the
	// rule, its length, the room and the fix, and no row is answered at all.
	// A cut row would route by a narrower trigger than the rule declares.
	// Method: pad completion's trigger past the row bound.
	long := "when the encoder meets " + strings.Repeat("an em dash — ", 30)
	rows, err := completionTrigger(t, long)
	if err == nil {
		t.Fatalf("an over-long trigger was accepted: %q", rows)
	}
	if rows != nil {
		t.Errorf("a refused index still answered rows: %q", rows)
	}
	prefix := "| `ai/rules/completion.md` | blocking | "
	room := maxTriggerLine - runeLen(prefix) - runeLen(" |")
	for _, want := range []string{
		"rule completion ",
		"is " + strconv.Itoa(runeLen(strings.TrimSpace(long))) + " characters",
		"room for " + strconv.Itoa(room),
		"ai/rules/points/completion/manifest.md",
		"./le ai rules update condensed",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
}

func TestATriggerThatFitsIsKeptWholeAndCountsRunes(t *testing.T) {
	// Goal: the bound counts characters, not bytes, and a trigger that fills
	// its room exactly is kept whole. Method: fill the room with em dashes,
	// which cost three bytes and one character each, so a byte bound would
	// refuse it; then add one character and expect the refusal.
	prefix := "| `ai/rules/completion.md` | blocking | "
	room := maxTriggerLine - runeLen(prefix) - runeLen(" |")
	head := "when the encoder meets "
	fits := head + strings.Repeat("—", room-runeLen(head))

	rows, err := completionTrigger(t, fits)
	if err != nil {
		t.Fatalf("a trigger that fits was refused: %v", err)
	}
	row := ""
	for _, line := range rows {
		if strings.Contains(line, "completion.md") {
			row = line
		}
	}
	if row != prefix+fits+" |" {
		t.Errorf("the row is not the whole trigger: %q", row)
	}
	if runeLen(row) != maxTriggerLine {
		t.Errorf("the row is %d characters, want exactly %d", runeLen(row), maxTriggerLine)
	}

	if _, err := completionTrigger(t, fits+"—"); err == nil {
		t.Errorf("a trigger one character over its room was accepted")
	}
}

func TestARuleWithNoTriggerKeepsItsRowAndSaysAlwaysReadIt(t *testing.T) {
	// Goal: a rule that declares no trigger is still named in the index, with
	// a row telling the reader to always read it, so the hardest rules to route
	// are never dropped. Method: give completion no `When:` line and expect
	// exactly one row for it, carrying the always-read marker.
	files := digestCorpus()
	files["ai/rules/completion.md"] = "# Completion\n\n**Severity:** blocking\n\n## D\n\n- MUST.\n"
	root := digestTree(t, files)

	rules, err := loadRules(rulesDir(root))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	rows, err := triggerLines(rules, nil)
	if err != nil {
		t.Fatalf("triggerLines refused a rule with no trigger: %v", err)
	}
	var found []string
	for _, line := range rows {
		if strings.Contains(line, "completion.md") {
			found = append(found, line)
		}
	}
	want := "| `ai/rules/completion.md` | blocking | (no trigger: always read it) |"
	if len(found) != 1 || found[0] != want {
		t.Errorf("the rule with no trigger answered rows %q, want exactly %q", found, want)
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
	if !strings.Contains(stale.Text(), "./le ai rules update condensed") {
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
		"ai/META-DECISIONS.md": "decisions\n",
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

	// A complete payload carries the three headings the agent rule set is cut
	// at, and the part for one kind of work with one section, so no
	// measurement is missing a part.
	instructions := "# Hard Bans\ninstructions\n"
	decisions := "# Rules for every agent\ndecisions\n# Rules for the main session\n# " + ruleset.DecisionsSituationalPartHeading +
		"\n## Lab work\n" + ruleset.SituationPrefix + "running a lab.\n- Use Ze's kernel.\n"
	whole, err := Payload(digestTree(t, map[string]string{
		"ai/INSTRUCTIONS.md":   instructions,
		"ai/META-DECISIONS.md": decisions,
		"ai/rules/TRIGGERS.md": "triggers\n",
		"ai/rules/CORE.md":     "core\n",
	}))
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}
	if whole.Failed() {
		t.Errorf("a complete payload failed: %+v", whole)
	}
	if whole.Chars != len(instructions)+len(decisions)+len("triggers\n")+len("core\n") {
		t.Errorf("chars = %d", whole.Chars)
	}
}

func TestThePayloadMeasuresTheAgentRuleSet(t *testing.T) {
	// Goal: AC-12. The payload report measures the agent rule set beside the
	// main payload, each against its own budget, and the agent set holds only
	// the hard bans and the every-agent decisions, cut at their headings.
	// Method: a checkout whose instructions and decisions each carry text
	// outside the agent's parts, then the budget boundary, then a missing
	// heading.
	bans := "# Hard Bans\n\nNever push.\n"
	agentPart := "# " + ruleset.DecisionsAgentPartHeading + "\n\n- Commit through le.\n"
	situational := "## Website design\n\n" + ruleset.SituationPrefix + "editing the website.\n\n- Render into gh-pages.\n"
	files := map[string]string{
		"ai/INSTRUCTIONS.md": "# Ze\n\nintro\n\n" + bans + "\n# Finding the Rule\n\nmap\n",
		"ai/META-DECISIONS.md": "# Owner Meta Decisions\n\nopening\n\n" + agentPart + "\n# " + ruleset.DecisionsMainPartHeading + "\n\n- Report to the owner.\n" +
			"\n# " + ruleset.DecisionsSituationalPartHeading + "\n\nEach section binds one kind of work.\n\n" + situational,
		"ai/rules/TRIGGERS.md": "triggers\n",
		"ai/rules/CORE.md":     "core\n",
	}
	report, err := Payload(digestTree(t, files))
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}
	if report.Failed() {
		t.Fatalf("a complete checkout failed: %+v", report)
	}
	if report.Budget != tokenBudget || report.Agent.Budget != agentTokenBudget || tokenBudget == agentTokenBudget {
		t.Errorf("the two sets are not each against their own budget: %d and %d", report.Budget, report.Agent.Budget)
	}
	want := runeLen(bans+"\n") + runeLen(agentPart+"\n") + len("triggers\n") + len("core\n")
	if report.Agent.Chars != want {
		t.Errorf("agent chars = %d, want %d: the cut is not at the headings", report.Agent.Chars, want)
	}
	parts, err := readAgentRuleSet(digestTree(t, files))
	if err != nil {
		t.Fatalf("readAgentRuleSet: %v", err)
	}
	joined := ""
	for _, part := range parts {
		joined += part.Text
	}
	for _, kept := range []string{"Never push.", "Commit through le.", "triggers", "core"} {
		if !strings.Contains(joined, kept) {
			t.Errorf("the agent set lost %q: %q", kept, joined)
		}
	}
	for _, dropped := range []string{"intro", "map", "opening", "Report to the owner.", "Render into gh-pages."} {
		if strings.Contains(joined, dropped) {
			t.Errorf("the agent set carries %q, which is outside its parts: %q", dropped, joined)
		}
	}
	for _, line := range []string{"agent rule set:", "ai/META-DECISIONS.md (" + ruleset.DecisionsAgentPartHeading + "): ", "always-loaded payload:"} {
		if !strings.Contains(report.Text(), line) {
			t.Errorf("the page does not show %q: %q", line, report.Text())
		}
	}
	if len(report.Situational.Parts) != 1 || report.Situational.Chars != runeLen(situational) {
		t.Errorf("the situational sections are not measured apart: %+v, want one part of %d chars", report.Situational, runeLen(situational))
	}
	for _, line := range []string{
		"situational decisions (",
		"  ai/META-DECISIONS.md (Website design): " + strconv.Itoa(runeLen(situational)) + " chars",
	} {
		if !strings.Contains(report.Text(), line) {
			t.Errorf("the page does not show %q: %q", line, report.Text())
		}
	}

	// The boundary: one token under the budget is MET, the budget itself is
	// EXCEEDED. The padding brings the agent set to exactly that many tokens.
	for _, tc := range []struct {
		tokens int
		met    bool
	}{{agentTokenBudget - 1, true}, {agentTokenBudget, false}} {
		padded := map[string]string{}
		maps.Copy(padded, files)
		padded["ai/rules/CORE.md"] = "core\n" + strings.Repeat("x", tc.tokens*bytesPerToken-want)
		at, err := Payload(digestTree(t, padded))
		if err != nil {
			t.Fatalf("Payload: %v", err)
		}
		if at.Agent.Tokens != tc.tokens || at.Agent.Met != tc.met {
			t.Errorf("at %d tokens: measured %d, met %v, want met %v", tc.tokens, at.Agent.Tokens, at.Agent.Met, tc.met)
		}
	}
}

func TestAnAgentRuleSetWithoutItsHeadingIsIncomplete(t *testing.T) {
	// Goal: a decisions file without the every-agent heading leaves the agent
	// set incomplete rather than smaller, and the report names the heading it
	// looked for. Method: a checkout complete but for that heading.
	partial, err := Payload(digestTree(t, map[string]string{
		"ai/INSTRUCTIONS.md":   "# Hard Bans\n\nNever push.\n",
		"ai/META-DECISIONS.md": "# Owner Meta Decisions\n\n- Report to the owner.\n",
		"ai/rules/TRIGGERS.md": "triggers\n",
		"ai/rules/CORE.md":     "core\n",
	}))
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}
	if !partial.Failed() || len(partial.Agent.Missing) != 1 || len(partial.Missing) != 0 {
		t.Errorf("a missing heading did not leave only the agent set incomplete: %+v", partial)
	}
	if !strings.Contains(partial.Text(), "0 headings \""+ruleset.DecisionsAgentPartHeading+"\"") {
		t.Errorf("the page does not name the heading it looked for: %q", partial.Text())
	}
}

func TestSituationalSectionsTheReaderCannotPlaceLeaveThePayloadIncomplete(t *testing.T) {
	// Goal: AC-15. A decisions file whose part for one kind of work is absent,
	// or holds a section with no situation, fails the report and names why,
	// rather than measuring no situational sections as zero. The agent rule
	// set itself stays complete: its every-agent range is still found.
	// Method: a checkout complete but for the third part, then one whose only
	// section opens with a bullet.
	twoParts := "# " + ruleset.DecisionsAgentPartHeading + "\n- Commit through le.\n# " + ruleset.DecisionsMainPartHeading + "\n- Report to the owner.\n"
	for _, tc := range []struct{ name, decisions, want string }{
		{"no third part", twoParts, "0 headings \"" + ruleset.DecisionsSituationalPartHeading + "\""},
		{"a section with no situation", twoParts + "# " + ruleset.DecisionsSituationalPartHeading + "\n## Lab work\n- Use Ze's kernel.\n",
			"section \"Lab work\" names no work"},
	} {
		report, err := Payload(digestTree(t, map[string]string{
			"ai/INSTRUCTIONS.md":   "# Hard Bans\n\nNever push.\n",
			"ai/META-DECISIONS.md": tc.decisions,
			"ai/rules/TRIGGERS.md": "triggers\n",
			"ai/rules/CORE.md":     "core\n",
		}))
		if err != nil {
			t.Fatalf("%s: Payload: %v", tc.name, err)
		}
		if !report.Failed() || len(report.Situational.Missing) != 1 || len(report.Agent.Missing) != 0 {
			t.Errorf("%s: did not leave only the situational sections incomplete: %+v", tc.name, report)
		}
		if len(report.Situational.Parts) != 0 || report.Situational.Chars != 0 {
			t.Errorf("%s: measured sections it could not place: %+v", tc.name, report.Situational)
		}
		if !strings.Contains(report.Text(), tc.want) {
			t.Errorf("%s: the page does not say why: %q", tc.name, report.Text())
		}
	}
}

// mainSessionDecisions names, by its opening words and in file order, each
// bullet of the main-session part of ai/META-DECISIONS.md: the rules only the
// session that talks to the owner, spawns agents, plans or pushes can break,
// so a subagent is never given them. A bullet moved into that part or out of
// it turns TestTheDecisionsFileKeepsEveryBulletAcrossItsTwoParts red until
// this list moves with it, which is the moment its placement is judged.
var mainSessionDecisions = []string{
	"**Thomas requests subagent delegation as the default in",
	"Some harness builds carry a guard:",
	"An agent sent to find out how something",
	"Every spec phase runs in a subagent through",
	"The main thread supervises: it launches, verifies each",
	"The main thread schedules heavy work. Before it",
	"A serialized run never leaves agents idle. While",
	"A report to the owner opens with what",
	"Up to three spec closures may run at",
	"A session runs one CPU-intensive job at a",
	"When Thomas sets a goal that spans several",
}

// The bullet counts of ai/META-DECISIONS.md, counted from the file, never
// copied from a test run: the situational count is the bullets of the
// sections that part holds, and the every-agent count is what the part keeps
// besides them. A rule added or removed changes one of these counts, which is
// the moment its placement is judged.
const (
	decisionBulletsTotal       = 135
	decisionBulletsAgent       = 96
	decisionBulletsSituational = 28
)

// decisionBullets answers each bullet of lines whole, its continuation lines
// joined to it with their newlines: a line that opens "- " starts one, and an
// indented line continues the open one.
func decisionBullets(lines []string) []string {
	var bullets []string
	open := false
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "- "):
			bullets = append(bullets, line)
			open = true
		case open && strings.HasPrefix(line, "  "):
			bullets[len(bullets)-1] += "\n" + line
		default:
			open = false
		}
	}
	return bullets
}

// VALIDATES: spec-agent-economy-3-preamble AC-4, AC-15 and R-3, on the
// checkout's own ai/META-DECISIONS.md: the three part headings once and in
// order, the opening naming each part, the range ruleset.AgentDecisions hands
// the start hook running from the first heading to the line before the second,
// the main-session part holding exactly the bullets mainSessionDecisions
// names, the bullet counts of each part, and no bullet in two parts or above
// the first one. The name predates the third part; the spec cites it.
// PREVENTS: a subagent given main-session rules, or denied a rule it can break,
// because a bullet changed part unseen or the reader cut at the wrong line.
// The test cannot know the committed file's earlier bullet set: a bullet
// deleted or reworded in the every-agent part passes here, and the sorted
// before-and-after comparison of the move is that check.
func TestTheDecisionsFileKeepsEveryBulletAcrossItsTwoParts(t *testing.T) {
	root := filepath.Clean("../../../..")
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ruleset.DecisionsFile)))
	if err != nil {
		t.Fatalf("read the decisions file: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	headingAt := func(heading string) int {
		t.Helper()
		found, count := -1, 0
		for index, line := range lines {
			text := strings.TrimLeft(line, "#")
			if len(text) < len(line) && strings.HasPrefix(text, " ") && strings.TrimSpace(text) == heading {
				found = index
				count++
			}
		}
		if count != 1 {
			t.Fatalf("%s holds %d headings %q, want exactly one", ruleset.DecisionsFile, count, heading)
		}
		return found
	}
	agentAt := headingAt(ruleset.DecisionsAgentPartHeading)
	mainAt := headingAt(ruleset.DecisionsMainPartHeading)
	situationalAt := headingAt(ruleset.DecisionsSituationalPartHeading)
	if mainAt < agentAt {
		t.Fatalf("the main-session heading (line %d) comes before the every-agent heading (line %d)", mainAt+1, agentAt+1)
	}
	if situationalAt < mainAt {
		t.Fatalf("the heading %q (line %d) comes before the main-session heading (line %d)", ruleset.DecisionsSituationalPartHeading, situationalAt+1, mainAt+1)
	}

	opening := strings.Join(lines[:agentAt], "\n")
	for _, heading := range []string{ruleset.DecisionsAgentPartHeading, ruleset.DecisionsMainPartHeading, ruleset.DecisionsSituationalPartHeading} {
		if !strings.Contains(strings.Join(strings.Fields(opening), " "), "\""+heading+"\"") {
			t.Errorf("the opening does not say when a new rule goes under %q", heading)
		}
	}
	if above := decisionBullets(lines[:agentAt]); len(above) != 0 {
		t.Errorf("%d bullets sit above the first part, in neither: %q", len(above), above)
	}

	part, err := ruleset.AgentDecisions(root)
	if err != nil {
		t.Fatalf("AgentDecisions: %v", err)
	}
	if part.First != int64(agentAt)+1 || part.Last != int64(mainAt) {
		t.Errorf("the every-agent range is lines %d-%d, want %d-%d", part.First, part.Last, agentAt+1, mainAt)
	}
	if want := strings.Join(lines[agentAt:mainAt], "\n") + "\n"; part.Text != want {
		t.Errorf("the every-agent text is not the file's lines %d-%d", agentAt+1, mainAt)
	}

	agentBullets := decisionBullets(lines[agentAt:mainAt])
	mainBullets := decisionBullets(lines[mainAt:situationalAt])
	situationalBullets := decisionBullets(lines[situationalAt:])
	if got := len(agentBullets) + len(mainBullets) + len(situationalBullets); got != decisionBulletsTotal {
		t.Errorf("the decisions file holds %d bullets across its three parts, want %d", got, decisionBulletsTotal)
	}
	if len(agentBullets) != decisionBulletsAgent {
		t.Errorf("the every-agent part holds %d bullets, want %d", len(agentBullets), decisionBulletsAgent)
	}
	if len(situationalBullets) != decisionBulletsSituational {
		t.Errorf("the part %q holds %d bullets, want %d", ruleset.DecisionsSituationalPartHeading, len(situationalBullets), decisionBulletsSituational)
	}
	// The part's bullets counted again through the ranges the start hook
	// names: a bullet of the part inside no section is one no agent is told
	// to read.
	sections, err := ruleset.SituationalDecisions(root)
	if err != nil {
		t.Fatalf("SituationalDecisions: %v", err)
	}
	inSections := 0
	for _, section := range sections {
		inSections += len(decisionBullets(strings.Split(section.Text, "\n")))
	}
	if inSections != len(situationalBullets) {
		t.Errorf("the sections the start hook names hold %d bullets, the part %d", inSections, len(situationalBullets))
	}
	inOtherPart := make(map[string]bool, len(agentBullets)+len(mainBullets))
	for _, bullet := range append(slices.Clone(agentBullets), mainBullets...) {
		inOtherPart[bullet] = true
	}
	for _, bullet := range situationalBullets {
		if inOtherPart[bullet] {
			t.Errorf("a bullet sits in the part %q and in another: %q", ruleset.DecisionsSituationalPartHeading, strings.SplitN(bullet, "\n", 2)[0])
		}
	}
	if len(mainBullets) != len(mainSessionDecisions) {
		t.Fatalf("the main-session part holds %d bullets, want %d", len(mainBullets), len(mainSessionDecisions))
	}
	for index, words := range mainSessionDecisions {
		if !strings.HasPrefix(mainBullets[index], "- "+words) {
			t.Errorf("main-session bullet %d opens %q, want %q", index+1, strings.SplitN(mainBullets[index], "\n", 2)[0], words)
		}
	}
	if len(agentBullets) == 0 {
		t.Fatal("the every-agent part holds no bullet")
	}
	inAgentPart := make(map[string]bool, len(agentBullets))
	for _, bullet := range agentBullets {
		inAgentPart[bullet] = true
	}
	for _, bullet := range mainBullets {
		if inAgentPart[bullet] {
			t.Errorf("a bullet sits in both parts: %q", strings.SplitN(bullet, "\n", 2)[0])
		}
	}
}

func TestTheRouterSubtractsACoreDerivedWithoutTheCorpus(t *testing.T) {
	files := digestCorpus()
	files["plan/post-release/engineering/spec-thing.md"] = "# Spec\n\n## Task\n\nRewrite the gokrazy qdisc encoder.\n"
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
	files["plan/post-release/engineering/spec-thing.md"] = "# Spec\n\n## Task\n\nRewrite the gokrazy encoder.\n"
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
	files["plan/post-release/engineering/spec-thing.md"] = "# Spec\n\n## Summary\n\nqdisc gokrazy elsewhere\n\n" +
		"## Task\n\nthe real statement\n\n### Sub\n\nstill inside the task\n\n" +
		"## Risks\n\nqdisc gokrazy again\n"
	files["plan/templates/TEMPLATE.md"] = "# T\n\n## Task\n\ntemplate text\n"
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
	files["plan/post-release/engineering/spec-thing.md"] = "# Spec\n\n## Task\n\nRewrite the gokrazy qdisc encoder.\n"
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
	files["plan/post-release/engineering/spec-thing.md"] = "# Spec\n\n## Task\n\nRewrite the gokrazy qdisc encoder.\n"
	root := digestTree(t, files)

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
