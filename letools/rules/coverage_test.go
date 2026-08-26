// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, and AC-11. Function calls build
// the structured gate map. The map refuses every gated-to-ungated route that
// leaves the other gates green.
// PREVENTS: incorrect parsing of dispatcher checks. Published counts state what
// the repository enforces. A missed check hides an unguarded rule. A dropped
// binding causes the same false pass.

package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dispatcherSource has the shape of the two large PreToolUse dispatchers. It
// contains a CHECKS table, bindings above checks, and one check that binds none.
const dispatcherSource = `#!/usr/bin/env python3
"""A dispatcher. def c_in_a_docstring() is not a check."""
import sys


# ze point: alpha/section/first
def c_first(ctx):
    return None


# ze point: alpha/section/second
# ze point: alpha/section/first
def check_second(ctx):
    return None


# ze point: none -- there is nothing written to bind
def c_third(ctx):
    return None


# ze point: alpha/section/gone
def c_fourth(ctx):
    return None


CHECKS = (
    c_first,
    check_second,
    c_third,
    c_fourth,
)


def main():
    return 0
`

func TestABindingBindsTheNextTopLevelDef(t *testing.T) {
	bindings := parseBindings(dispatcherSource, "pretool-x.py")
	got := map[string][]string{}
	for _, binding := range bindings {
		got[binding.Check] = append(got[binding.Check], binding.Ref)
	}
	if strings.Join(got["c_first"], ",") != "alpha/section/first" {
		t.Errorf("c_first bound %v", got["c_first"])
	}
	if strings.Join(got["check_second"], ",") != "alpha/section/second,alpha/section/first" {
		t.Errorf("check_second bound %v", got["check_second"])
	}
	if len(got["c_third"]) != 1 || got["c_third"][0] != "" {
		t.Errorf("c_third bound %v, want the none declaration", got["c_third"])
	}
	for _, binding := range bindings {
		if binding.Check == "c_third" && binding.Reason != "there is nothing written to bind" {
			t.Errorf("the none declaration lost its reason: %q", binding.Reason)
		}
	}
}

func TestABindingAboveNoCheckIsReportedNotDropped(t *testing.T) {
	// A comment that names a point and gates nothing is a claim, and claims are
	// what this design removes.
	source := "# ze point: alpha/section/first\nx = 1\n\n\n# ze point:\ndef c_bare(ctx):\n    pass\n"
	bindings := parseBindings(source, "pretool-x.py")
	if len(bindings) != 2 {
		t.Fatalf("parseBindings found %d bindings: %+v", len(bindings), bindings)
	}
	if bindings[0].Check != noCheck {
		t.Errorf("the orphan binding was attributed to %q", bindings[0].Check)
	}
	// An EMPTY payload is kept as a ref no point matches, so a bare marker
	// fails as dangling instead of vanishing.
	if bindings[1].Ref != emptyRef {
		t.Errorf("the empty binding is %q, want %s", bindings[1].Ref, emptyRef)
	}
}

func TestATypoInABindingFailsAsDangling(t *testing.T) {
	// Dropping an unparsable payload would let a typo un-gate a check with
	// nothing going red.
	bindings := parseBindings("# ze point: none but no reason\ndef c_x(ctx):\n    pass\n", "pretool-x.py")
	if len(bindings) != 1 || bindings[0].Ref != "none but no reason" {
		t.Fatalf("parseBindings = %+v", bindings)
	}
}

func TestTheDispatcherRosterReadsWhatItRuns(t *testing.T) {
	bindings := parseBindings(dispatcherSource, "pretool-x.py")
	roster, err := dispatcherChecks(dispatcherSource, bindings)
	if err != nil {
		t.Fatalf("dispatcherChecks: %v", err)
	}
	for _, want := range []string{"c_first", "check_second", "c_third", "c_fourth"} {
		if !roster[want] {
			t.Errorf("the roster misses %s: %v", want, sortedKeys(roster))
		}
	}
	// A def named only inside a docstring is not a check, and Python's ast
	// would never see one.
	if roster["c_in_a_docstring"] {
		t.Errorf("the roster read a docstring: %v", sortedKeys(roster))
	}
}

func TestADispatcherWithNoChecksTableReadsItsMain(t *testing.T) {
	// One dispatcher calls its two gates directly and neither carries the
	// prefix, so reading only the prefix would leave both invisible.
	source := `import sys


def verdict(prompt):
    return None


def style_guide_reminder(prompt):
    return None


def _helper(x):
    return x


def main() -> int:
    hit = verdict("x")
    reminder = style_guide_reminder("x")
    value = _helper(1)
    sys.stderr.write("no")
    return 0
`
	roster, err := dispatcherChecks(source, nil)
	if err != nil {
		t.Fatalf("dispatcherChecks: %v", err)
	}
	if !roster["verdict"] || !roster["style_guide_reminder"] {
		t.Errorf("the roster misses an unprefixed gate: %v", sortedKeys(roster))
	}
	// A `_`-prefixed name has declared itself private ON PURPOSE, and an
	// attribute call is not a bare name at all.
	if roster["_helper"] || roster["write"] {
		t.Errorf("the roster read a private helper or an attribute call: %v", sortedKeys(roster))
	}
}

func TestADispatcherThatCannotBeScannedIsRefused(t *testing.T) {
	// A dispatcher whose checks cannot be enumerated is one whose rows nobody
	// can check, so the reader fails closed rather than answering a short
	// roster.
	if _, err := dispatcherChecks("x = 'unterminated\n", nil); err == nil {
		t.Error("dispatcherChecks accepted an unterminated literal")
	}
	if _, err := dispatcherChecks("x = '''unterminated\nstill\n", nil); err == nil {
		t.Error("dispatcherChecks accepted an unterminated triple quote")
	}
}

func TestAnUnwiredDispatcherIsRefusedInBothDirections(t *testing.T) {
	// Each direction removes a check from the join while the report still says
	// "no dangling bindings". A registered command without a file removes its
	// bindings. An unregistered dispatcher contains checks that never run.
	write := func(t *testing.T, files map[string]string) string {
		t.Helper()
		root := t.TempDir()
		for rel, body := range files {
			path := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatalf("fixture: %v", err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("fixture: %v", err)
			}
		}
		return root
	}
	settings := func(name string) string {
		return `{"hooks": {"PreToolUse": [{"matcher": "Write", "hooks": [{"type": "command",` +
			` "command": "$CLAUDE_PROJECT_DIR/.claude/hooks/` + name + `"}]}]}}` + "\n"
	}

	wired := write(t, map[string]string{
		".claude/settings.json":      settings("pretool-a.py"),
		".claude/hooks/pretool-a.py": "def c_x(ctx):\n    pass\n",
	})
	paths, problems := dispatchers(wired)
	if len(paths) != 1 || len(problems) != 0 {
		t.Fatalf("a wired dispatcher was refused: %v / %v", paths, problems)
	}

	missing := write(t, map[string]string{".claude/settings.json": settings("pretool-a.py")})
	if _, problems := dispatchers(missing); len(problems) != 1 ||
		!strings.Contains(problems[0], "which does not exist") {
		t.Errorf("a registered file that is absent was accepted: %v", problems)
	}

	unwired := write(t, map[string]string{
		".claude/settings.json":      settings("pretool-a.py"),
		".claude/hooks/pretool-a.py": "def c_x(ctx):\n    pass\n",
		".claude/hooks/pretool-b.py": "def c_y(ctx):\n    pass\n",
	})
	if _, problems := dispatchers(unwired); len(problems) != 1 ||
		!strings.Contains(problems[0], "so its checks never fire") {
		t.Errorf("a dispatcher no entry runs was accepted: %v", problems)
	}

	// An unreadable settings.json is a failure rather than an empty roster: an
	// empty roster reads as "no dispatchers exist", which is never true here.
	if _, problems := dispatchers(t.TempDir()); len(problems) != 1 ||
		!strings.Contains(problems[0], "the dispatcher roster is unknown") {
		t.Errorf("an absent settings.json answered an empty roster: %v", problems)
	}
	broken := write(t, map[string]string{".claude/settings.json": "{not json\n"})
	if _, problems := dispatchers(broken); len(problems) != 1 ||
		!strings.Contains(problems[0], "the dispatcher roster is unknown") {
		t.Errorf("a malformed settings.json answered an empty roster: %v", problems)
	}
	empty := write(t, map[string]string{".claude/settings.json": "{}\n"})
	if _, problems := dispatchers(empty); len(problems) != 1 ||
		!strings.Contains(problems[0], "must not report success") {
		t.Errorf("settings.json with no PreToolUse entry was accepted: %v", problems)
	}
}

func TestAnEscapedPipeInsideACellIsContent(t *testing.T) {
	got := cells(`| c_x | a \| b | rest |`)
	want := []string{"", "c_x", `a \| b`, "rest", ""}
	if strings.Join(got, "~") != strings.Join(want, "~") {
		t.Errorf("cells = %v, want %v", got, want)
	}
}

func TestThePublishedTableIsReadUnderItsHeading(t *testing.T) {
	doc := "## The `pretool-x.py` dispatcher\n\n" + tableHead + " Triggers on |\n" +
		"|---|---|---|\n" +
		"| `c_first` | `alpha.md` | always |\n" +
		"| `c_third` | nothing | always |\n" +
		"\nprose after the table\n" +
		"| `c_never` | `alpha.md` | not a row |\n"
	rows := publishedRows(doc)["pretool-x.py"]
	if len(rows) != 2 {
		t.Fatalf("publishedRows found %d rows: %+v", len(rows), rows)
	}
	if rows[0].Check != "c_first" || rows[0].Enforces != "`alpha.md`" {
		t.Errorf("the first row is %+v", rows[0])
	}
	if rows[1].Check != "c_third" {
		t.Errorf("the second row is %+v", rows[1])
	}
}

func TestOnlyABacktickedStemNamesARule(t *testing.T) {
	stems := map[string]bool{"planning": true}
	// A path names a rule outside this corpus. Its check binds no point, so the
	// path must not claim anything about planning.md.
	if got := ruleStemsNamed("see `.claude/rules/planning.md`", stems); len(got) != 0 {
		t.Errorf("a path was read as a stem: %v", sortedKeys(got))
	}
	if got := ruleStemsNamed("see `planning.md`", stems); !got["planning"] {
		t.Errorf("a bare stem was not read: %v", sortedKeys(got))
	}
}

func TestTheRetiredLedgerRefusesARowThatDeclaresNothing(t *testing.T) {
	head := map[string]bool{"alpha/section/gone": true, "alpha/section/kept": true}
	now := map[string]bool{"alpha/section/kept": true}
	rows := func(lines ...string) string {
		return retiredTableHead + "\n|---|---|\n" + strings.Join(lines, "\n") + "\n"
	}

	declared, problems := retiredRowsSince(
		rows("| `alpha/section/gone` | it left |"), "", head, true, now)
	if !declared["alpha/section/gone"] || len(problems) != 0 {
		t.Fatalf("a good row was refused: %v / %v", sortedKeys(declared), problems)
	}

	cases := []struct{ name, row, want string }{
		{"not a row shape", "| nonsense |", "is not '<rule>/<section>/<slug> -- <why>'"},
		{"never at HEAD", "| `alpha/section/invented` | why |", "names no point at HEAD"},
		{"still on disk", "| `alpha/section/kept` | why |", "is still on disk"},
	}
	for _, tc := range cases {
		_, problems := retiredRowsSince(rows(tc.row), "", head, true, now)
		if len(problems) != 1 || !strings.Contains(problems[0], tc.want) {
			t.Errorf("%s: retiredRowsSince said %v, want it to mention %q", tc.name, problems, tc.want)
		}
	}

	_, problems = retiredRowsSince(
		rows("| `alpha/section/gone` | one |", "| `alpha/section/gone` | two |"), "", head, true, now)
	if len(problems) != 1 || !strings.Contains(problems[0], "is declared twice") {
		t.Errorf("a duplicate row was accepted: %v", problems)
	}
}

func TestARowCommittedAtHeadIsNotRejudged(t *testing.T) {
	// Re-judging a committed line would fail somebody else's later run, and
	// re-wording one would mint a second deletion for the same point. Both are
	// what keep the ledger a scope rather than an allowlist.
	head := map[string]bool{"alpha/section/gone": true}
	now := map[string]bool{"alpha/section/gone": true}
	was := retiredTableHead + "\n|---|---|\n| `alpha/section/gone` | it left |\n"
	reworded := retiredTableHead + "\n|---|---|\n| `alpha/section/gone` | it left, in detail |\n"

	declared, problems := retiredRowsSince(reworded, was, head, true, now)
	if len(problems) != 0 {
		t.Errorf("a reworded committed row was refused: %v", problems)
	}
	if declared["alpha/section/gone"] {
		t.Errorf("a reworded committed row minted a fresh retirement: %v", sortedKeys(declared))
	}
}

func TestTheCorpusRatchetReportsIdentityNotACount(t *testing.T) {
	// An addition masks a count because the rule retains its point total. A
	// specific instruction still leaves.
	head := map[string]bool{"alpha/s/one": true, "alpha/s/two": true}
	now := map[string]bool{"alpha/s/one": true, "alpha/s/three": true}
	got := corpusShrink(head, now, nil)
	if len(got) != 1 || !strings.Contains(got[0], "alpha/s/two") {
		t.Fatalf("corpusShrink = %v", got)
	}
	if len(corpusShrink(head, now, map[string]bool{"alpha/s/two": true})) != 0 {
		t.Errorf("a declared retirement still counted")
	}
}

func TestARenamedPointCannotBeLaunderedIntoDeclaredNone(t *testing.T) {
	gm := GateMap{
		Points: map[string]string{"alpha/s/live": kindDirective},
		Gated:  map[string][]Binding{},
		Unbound: []Binding{
			{File: "pretool-x.py", Check: "c_first", Reason: "moved on"},
		},
	}
	baseline := map[string]map[string]bool{"c_first": {"alpha/s/renamed": true}}

	got := unboundRegressions(gm, baseline, nil)
	if len(got) != 1 || !strings.Contains(got[0], "named alpha/s/renamed at HEAD, declares `none` now") {
		t.Fatalf("unboundRegressions = %v", got)
	}
	// A validated retirement IS the ordinary route out of the corpus.
	if got := unboundRegressions(gm, baseline, map[string]bool{"alpha/s/renamed": true}); len(got) != 0 {
		t.Errorf("a declared retirement was still a regression: %v", got)
	}
	// A PARTIAL declaration still lost a gate.
	baseline["c_first"]["alpha/s/other"] = true
	if got := unboundRegressions(gm, baseline, map[string]bool{"alpha/s/renamed": true}); len(got) != 1 {
		t.Errorf("a partial declaration excused the whole check: %v", got)
	}
}

func TestAPointGatedAtHeadAndGatedByNothingNowFails(t *testing.T) {
	gm := GateMap{
		Points: map[string]string{"alpha/s/live": kindDirective},
		Gated:  map[string][]Binding{},
	}
	baseline := map[string]map[string]bool{"c_first": {"alpha/s/live": true, "alpha/s/deleted": true}}
	got := gatedRegressions(gm, baseline)
	// A point that no longer EXISTS is outside this gate. Its instruction can
	// leave through a rule-content diff that a reader sees in review.
	if len(got) != 1 || got[0] != "alpha/s/live" {
		t.Fatalf("gatedRegressions = %v", got)
	}
}

func TestARationaleOutsideTheTreeIsMissing(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "ai", "rationale")
	if err := os.MkdirAll(inside, 0o750); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inside, "real.md"), []byte("why\n"), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inside, "blank.md"), []byte("   \n"), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	points := map[string]Point{
		"a/s/good":    {Rationale: "ai/rationale/real.md"},
		"a/s/blank":   {Rationale: "ai/rationale/blank.md"},
		"a/s/escaped": {Rationale: "../outside.md"},
		"a/s/absent":  {Rationale: "ai/rationale/nope.md"},
		"a/s/none":    {},
	}
	declared, missing := rationaleProblems(points, root)
	if len(declared) != 4 {
		t.Errorf("rationaleProblems declared %d links, want 4", len(declared))
	}
	got := map[string]bool{}
	for _, pair := range missing {
		got[pair[0]] = true
	}
	for _, ref := range []string{"a/s/blank", "a/s/escaped", "a/s/absent"} {
		if !got[ref] {
			t.Errorf("%s was not reported missing: %v", ref, missing)
		}
	}
	if got["a/s/good"] {
		t.Errorf("a real record was reported missing")
	}
}

func TestAPointCannotExceptItself(t *testing.T) {
	points := map[string]Point{
		"a/s/general": {ExceptedBy: "a/s/carve, a/s/gone,"},
		"a/s/carve":   {},
		"a/s/self":    {ExceptedBy: "a/s/self"},
	}
	declared, missing := exceptionProblems(points)
	if len(declared["a/s/general"]) != 2 {
		t.Errorf("a trailing comma became a ref: %v", declared["a/s/general"])
	}
	found := map[string]string{}
	for _, pair := range missing {
		found[pair[1]] = pair[0]
	}
	if found["a/s/gone"] != "a/s/general" || found["a/s/self"] != "a/s/self" {
		t.Errorf("exceptionProblems missed a shape: %v", missing)
	}
}

func TestTheGateMapReadingNothingIsNeverAPass(t *testing.T) {
	empty := fillCoverage(GateMap{}, true, true)
	if !empty.Failed() || len(empty.Empty) != 1 ||
		!strings.Contains(empty.Empty[0], "no points under ai/rules/points/") {
		t.Fatalf("an empty corpus passed: %+v", empty)
	}
	noBindings := fillCoverage(GateMap{Points: map[string]string{"a/s/p": kindDirective}}, true, true)
	if !noBindings.Failed() || !strings.Contains(noBindings.Empty[0], "no `# ze point:` binding") {
		t.Fatalf("a corpus with no binding passed: %+v", noBindings)
	}
}

func TestAMissingBaselinePrintsItsAbsenceRatherThanAZero(t *testing.T) {
	// A ratchet that ran against nothing must say so: a zero printed over a
	// comparison that never ran is the permissive answer the guard refuses.
	gm := GateMap{
		Points:   map[string]string{"a/s/p": kindDirective},
		Bindings: []Binding{{File: "pretool-x.py", Check: "c_x", Ref: "a/s/p"}},
		Gated:    map[string][]Binding{"a/s/p": {{Check: "c_x", Ref: "a/s/p"}}},
	}
	report := fillCoverage(gm, false, false)
	page := report.Text()
	if !strings.Contains(page, "REGRESSED: no HEAD baseline") {
		t.Errorf("the page does not say the dispatcher ratchet did not run: %s", page)
	}
	if !strings.Contains(page, "SHRUNK: no HEAD point baseline") {
		t.Errorf("the page does not say the corpus ratchet did not run: %s", page)
	}
	if strings.Contains(page, "REGRESSED: 0") || strings.Contains(page, "SHRUNK: 0") {
		t.Errorf("the page printed a zero for a comparison that never ran: %s", page)
	}
}

func TestTheCoverageAnswerIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	report := CoverageReport{
		Points: 1, Bindings: 1,
		Gated:            []GatedPoint{{Ref: "a/s/p", Checks: []string{"c_x"}}},
		MissingRationale: []MissingLink{{Ref: "a/s/p", Target: "x.md", Why: "no such record"}},
	}
	raw, err := json.Marshal(&report)
	if err != nil {
		t.Fatalf("marshaling the report: %v", err)
	}
	for _, key := range []string{
		`"ungated-by-kind"`, `"most-ungated"`, `"corpus-baseline"`, `"no-head-for"`,
		`"declared-none"`, `"retired-ledger"`, `"missing-rationale"`, `"missing-exception"`,
		`"doc-missing"`, `"diagnosis"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}
	// The VALUES carry Python check names, which are snake_case by their own
	// convention, so only the keys are read here.
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decoding the report: %v", err)
	}
	for key := range decoded {
		if strings.Contains(key, "_") {
			t.Errorf("the JSON key %q is not kebab-case", key)
		}
	}
}

func TestTheMostUngatedTieBreakIsStable(t *testing.T) {
	// Counter.most_common sorts the insertion order by count and keeps the
	// first among equals, and the insertion order here is the sorted ungated
	// list. Without the stable sort the top-five line would vary per run.
	ungated := []string{"beta/s/1", "alpha/s/1", "alpha/s/2", "gamma/s/1"}
	first := mostUngatedRules(ungated, 5)
	for range 20 {
		if got := mostUngatedRules(ungated, 5); tallyLine(got) != tallyLine(first) {
			t.Fatalf("mostUngatedRules is not stable: %s then %s", tallyLine(first), tallyLine(got))
		}
	}
	if tallyLine(first) != "alpha 2, beta 1, gamma 1" {
		t.Errorf("mostUngatedRules = %s", tallyLine(first))
	}
}
