// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, and AC-11. Native Go hook
// sources produce the structured gate map. The map refuses every
// gated-to-ungated route that leaves the other gates green.
// PREVENTS: missing, unwired, dangling, regressed, or unpublished native hook
// bindings from silently shortening rule coverage.

package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const nativeSource = `package hookruntime

type hookCheck func()
type hookAction struct { checks []hookCheck }

var nativeHookActions = map[string]hookAction{
	"pretool-x": {
		checks: []hookCheck{firstCheck, secondCheck, unboundCheck},
	},
}

// ze point: alpha/section/first
func firstCheck() {}

// ze point: alpha/section/second
// ze point: alpha/section/first
func secondCheck() {}

// ze point: none -- there is nothing written to bind
func unboundCheck() {}
`

func TestABindingBelongsToItsGoFunctionDocComment(t *testing.T) {
	file, err := parseHookFile(nativeSource, "runtime.go")
	if err != nil {
		t.Fatalf("parseHookFile: %v", err)
	}
	got := map[string][]string{}
	for _, binding := range file.bindings {
		got[binding.Check] = append(got[binding.Check], binding.Ref)
	}
	if strings.Join(got["firstCheck"], ",") != "alpha/section/first" {
		t.Errorf("firstCheck bound %v", got["firstCheck"])
	}
	if strings.Join(got["secondCheck"], ",") != "alpha/section/second,alpha/section/first" {
		t.Errorf("secondCheck bound %v", got["secondCheck"])
	}
	if len(got["unboundCheck"]) != 1 || got["unboundCheck"][0] != "" {
		t.Errorf("unboundCheck bound %v, want the none declaration", got["unboundCheck"])
	}
	for _, binding := range file.bindings {
		if binding.Check == "unboundCheck" &&
			binding.Reason != "there is nothing written to bind" {
			t.Errorf("the none declaration lost its reason: %q", binding.Reason)
		}
	}
}

func TestABindingOutsideAFunctionDocCommentIsNotAttached(t *testing.T) {
	source := `package hookruntime
// ze point: alpha/section/first
var value = 1

// ze point:
func bareCheck() {}
`
	file, err := parseHookFile(source, "runtime.go")
	if err != nil {
		t.Fatalf("parseHookFile: %v", err)
	}
	if len(file.bindings) != 2 {
		t.Fatalf("parseHookFile found %d bindings: %+v", len(file.bindings), file.bindings)
	}
	found := map[string]Binding{}
	for _, binding := range file.bindings {
		found[binding.Check] = binding
	}
	if found[noCheck].Ref != "alpha/section/first" {
		t.Errorf("the orphan binding was lost: %+v", file.bindings)
	}
	if found["bareCheck"].Ref != emptyRef {
		t.Errorf("the empty binding is %q, want %s", found["bareCheck"].Ref, emptyRef)
	}
}

func TestATypoInAGoBindingFailsAsDangling(t *testing.T) {
	file, err := parseHookFile("package hookruntime\n// ze point: none but no reason\nfunc check() {}\n", "runtime.go")
	if err != nil {
		t.Fatalf("parseHookFile: %v", err)
	}
	if len(file.bindings) != 1 || file.bindings[0].Ref != "none but no reason" {
		t.Fatalf("bindings = %+v", file.bindings)
	}
}

func writeHookTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(hookRuntimeRel), name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	return root
}

func TestTheNativeActionRegistryFailsClosed(t *testing.T) {
	good := writeHookTree(t, map[string]string{"runtime.go": nativeSource})
	sources, problems := nativeHookSources(good)
	if len(sources) != 1 || len(problems) != 0 {
		t.Fatalf("a wired native registry was refused: %v / %v", sortedKeys(sources), problems)
	}

	cases := []struct {
		name    string
		source  string
		problem string
	}{
		{
			"registered check missing a binding",
			strings.Replace(nativeSource,
				"// ze point: alpha/section/first\nfunc firstCheck", "func firstCheck", 1),
			"registered check firstCheck has no",
		},
		{
			"bound check absent from the registry",
			nativeSource + "\n// ze point: alpha/section/first\nfunc strayCheck() {}\n",
			"binding on unwired check strayCheck",
		},
		{
			"registry names no function",
			strings.Replace(nativeSource, "firstCheck, secondCheck", "missingCheck, secondCheck", 1),
			"check missingCheck names no top-level",
		},
		{
			"registry is absent",
			"package hookruntime\n// ze point: alpha/section/first\nfunc firstCheck() {}\n",
			"nativeHookActions: native hook action registry is missing",
		},
		{
			"Go cannot be parsed",
			"package hookruntime\nfunc broken(\n",
			"cannot be parsed",
		},
	}
	for _, tc := range cases {
		root := writeHookTree(t, map[string]string{"runtime.go": tc.source})
		_, problems := nativeHookSources(root)
		if !containsString(problems, tc.problem) {
			t.Errorf("%s: problems %v do not mention %q", tc.name, problems, tc.problem)
		}
	}
}

func containsString(lines []string, fragment string) bool {
	for _, line := range lines {
		if strings.Contains(line, fragment) {
			return true
		}
	}
	return false
}

func TestCurrentNativeHookChecksAreBound(t *testing.T) {
	root := filepath.Clean("../../..")
	sources, problems := nativeHookSources(root)
	if len(problems) != 0 {
		t.Fatalf("native hook roster: %v", problems)
	}
	gateMap, err := buildGateMap(sources,
		filepath.Join(root, filepath.FromSlash(pointsRel)), root)
	if err != nil {
		t.Fatalf("buildGateMap: %v", err)
	}
	if len(gateMap.Dangling) != 0 {
		t.Fatalf("native hook bindings name missing points: %+v", gateMap.Dangling)
	}
	published, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(hookDoc)))
	if err != nil {
		t.Fatalf("published hook tables: %v", err)
	}
	if problems := hookTableProblems(gateMap, string(published), sources); len(problems) != 0 {
		t.Fatalf("published native hook rows: %v", problems)
	}
}

func TestAnEscapedPipeInsideACellIsContent(t *testing.T) {
	got := cells(`| c_x | a \| b | rest |`)
	want := []string{"", "c_x", `a \| b`, "rest", ""}
	if strings.Join(got, "~") != strings.Join(want, "~") {
		t.Errorf("cells = %v, want %v", got, want)
	}
}

func TestThePublishedTableIsReadUnderItsGoSourceHeading(t *testing.T) {
	doc := "## Bash (`internal/le/hookruntime/bash.go`)\n\n" + tableHead + " Triggers on |\n" +
		"|---|---|---|\n" +
		"| `firstCheck` | `alpha.md` | always |\n" +
		"| `unboundCheck` | nothing | always |\n" +
		"\nprose after the table\n" +
		"| `neverCheck` | `alpha.md` | not a row |\n"
	rows := publishedRows(doc)["bash.go"]
	if len(rows) != 2 {
		t.Fatalf("publishedRows found %d rows: %+v", len(rows), rows)
	}
	if rows[0].Check != "firstCheck" || rows[0].Enforces != "`alpha.md`" {
		t.Errorf("the first row is %+v", rows[0])
	}
	if rows[1].Check != "unboundCheck" {
		t.Errorf("the second row is %+v", rows[1])
	}
}

func TestPublishedRowsMustMatchNativeBindings(t *testing.T) {
	gm := gateMap{
		Points: map[string]string{"alpha/section/first": kindDirective},
		Bindings: []Binding{
			{File: "bash.go", Check: "firstCheck", Ref: "alpha/section/first"},
			{File: "bash.go", Check: "unboundCheck", Reason: "none"},
		},
	}
	doc := "## Bash (`bash.go`)\n\n" + tableHead + " Triggers on |\n" +
		"|---|---|---|\n" +
		"| `firstCheck` | `alpha.md` | always |\n" +
		"| `unboundCheck` | no rule | always |\n"
	if problems := hookTableProblems(gm, doc, map[string]string{"bash.go": "package hookruntime\n"}); len(problems) != 0 {
		t.Fatalf("matching published rows were refused: %v", problems)
	}
	missing := strings.Replace(doc, "| `firstCheck` | `alpha.md` | always |\n", "", 1)
	if problems := hookTableProblems(gm, missing, map[string]string{"bash.go": "package hookruntime\n"}); !containsString(problems, "firstCheck") {
		t.Fatalf("a missing published row passed: %v", problems)
	}
	wrong := strings.Replace(doc, "| `firstCheck` | `alpha.md` |", "| `firstCheck` | `beta.md` |", 1)
	if problems := hookTableProblems(gm, wrong, map[string]string{"bash.go": "package hookruntime\n"}); !containsString(problems, "Enforces names") {
		t.Fatalf("a row naming the wrong point passed: %v", problems)
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

func TestARenamedPointCannotBeLaunderedIntoDeclaredNone(t *testing.T) {
	gm := gateMap{
		Points: map[string]string{"alpha/s/live": kindDirective},
		Gated:  map[string][]Binding{},
		Unbound: []Binding{
			{File: "bash.go", Check: "firstCheck", Reason: "moved on"},
		},
	}
	baseline := map[string]map[string]bool{"firstCheck": {"alpha/s/renamed": true}}

	onDisk := map[string]bool{"alpha/s/renamed": true}
	got := unboundRegressions(gm, baseline, onDisk)
	if len(got) != 1 || !strings.Contains(got[0], "named alpha/s/renamed at HEAD, declares `none` now") {
		t.Fatalf("unboundRegressions = %v", got)
	}
	// An instruction that LEFT the corpus is the ordinary route out, and git
	// history says where it went. The check rightly enforces nothing.
	if got := unboundRegressions(gm, baseline, nil); len(got) != 0 {
		t.Errorf("a point that left the corpus was still a regression: %v", got)
	}
	// One ref gone and one still on disk still lost a gate.
	baseline["firstCheck"]["alpha/s/other"] = true
	if got := unboundRegressions(gm, baseline, onDisk); len(got) != 1 {
		t.Errorf("a partly-removed baseline excused the whole check: %v", got)
	}
}

func TestAPointGatedAtHeadAndGatedByNothingNowFails(t *testing.T) {
	gm := gateMap{
		Points: map[string]string{"alpha/s/live": kindDirective},
		Gated:  map[string][]Binding{},
	}
	baseline := map[string]map[string]bool{"firstCheck": {"alpha/s/live": true, "alpha/s/deleted": true}}
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
	empty := fillCoverage(gateMap{}, true)
	if !empty.Failed() || len(empty.Empty) != 1 ||
		!strings.Contains(empty.Empty[0], "no points under ai/rules/points/") {
		t.Fatalf("an empty corpus passed: %+v", empty)
	}
	noBindings := fillCoverage(gateMap{Points: map[string]string{"a/s/p": kindDirective}}, true)
	if !noBindings.Failed() || !strings.Contains(noBindings.Empty[0], "no `// ze point:` binding") {
		t.Fatalf("a corpus with no binding passed: %+v", noBindings)
	}
}

func TestAMissingBaselinePrintsItsAbsenceRatherThanAZero(t *testing.T) {
	// A ratchet that ran against nothing must say so: a zero printed over a
	// comparison that never ran is the permissive answer the guard refuses.
	gm := gateMap{
		Points:   map[string]string{"a/s/p": kindDirective},
		Bindings: []Binding{{File: "bash.go", Check: "firstCheck", Ref: "a/s/p"}},
		Gated:    map[string][]Binding{"a/s/p": {{Check: "firstCheck", Ref: "a/s/p"}}},
	}
	report := fillCoverage(gm, false)
	page := report.Text()
	if !strings.Contains(page, "REGRESSED: no HEAD baseline") {
		t.Errorf("the native source ratchet absence is not reported: %s", page)
	}
	if strings.Contains(page, "REGRESSED: 0") {
		t.Errorf("the page printed a zero for a comparison that never ran: %s", page)
	}
}

func TestTheCoverageAnswerIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	report := CoverageReport{
		Points: 1, Bindings: 1,
		Gated:            []GatedPoint{{Ref: "a/s/p", Checks: []string{"firstCheck"}}},
		MissingRationale: []MissingLink{{Ref: "a/s/p", Target: "x.md", Why: "no such record"}},
	}
	raw, err := json.Marshal(&report)
	if err != nil {
		t.Fatalf("marshaling the report: %v", err)
	}
	for _, key := range []string{
		`"ungated-by-kind"`, `"most-ungated"`, `"no-head-for"`,
		`"declared-none"`, `"missing-rationale"`, `"missing-exception"`,
		`"doc-missing"`, `"diagnosis"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}
	// The values carry Go function names; only JSON keys are checked here.
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
