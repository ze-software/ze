// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-8 -- the feature matrix is
// derived by a function call, answers structured rows, and keeps 1 (the tree
// does not type-check) apart from 2 (the matrix could not be judged).
// PREVENTS: a scoped run that silently drops a shipped combination. all_features
// and core_only are what Ze ships, and a matrix missing either goes green
// having judged neither.

package staticcheckmatrix

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/letools/lepath"
)

// manifestTags is a small feature manifest the derivation cases are built on.
var manifestTags = []string{"ze_alpha", "ze_beta", "ze_gamma"}

// VALIDATES: the derivation over this checkout answers one row per declared
// feature plus the two shipped combinations.
// PREVENTS: a matrix that lists its rows instead of deriving them, which stops
// covering a tag added to the manifest.
func TestTheRealManifestDerivesEveryRow(t *testing.T) {
	tree, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}

	matrix, notice, err := Derive(tree)
	if err != nil {
		t.Fatalf("the matrix could not be derived: %v", err)
	}
	if notice.Widened != "" {
		t.Errorf("an unscoped run reported a widening reason: %s", notice.Widened)
	}
	if len(matrix) != notice.Total {
		t.Errorf("the run judges %d of %d rows without being scoped", len(matrix), notice.Total)
	}
	if len(matrix) < minMatrixRows+1 {
		t.Fatalf("the matrix holds %d rows, want the two shipped combinations and at least one feature", len(matrix))
	}
	if matrix[0].Name != "all_features" || matrix[1].Name != "core_only" {
		t.Errorf("the first two rows are %q and %q, want all_features and core_only", matrix[0].Name, matrix[1].Name)
	}
	for _, row := range matrix[2:] {
		if !strings.HasPrefix(row.Name, "without_") || row.Omits == "" {
			t.Errorf("row %q omits %q, want a without_<tag> row naming its tag", row.Name, row.Omits)
		}
	}
}

// VALIDATES: the scope narrows to the rows a change set can move, and every
// doubt widens instead.
// PREVENTS: a guard that cannot read its input returning a valid-looking narrow
// answer, which would leave rows unjudged with nothing said.
func TestEveryDoubtJudgesTheWholeMatrix(t *testing.T) {
	dir := t.TempDir()
	answer := func(t *testing.T, name, body string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return path
	}

	cases := []struct {
		name   string
		path   string
		every  bool
		widens bool
	}{
		{name: "no answer named", path: "", every: true},
		{name: "answer cannot be read", path: filepath.Join(dir, "absent"), every: true, widens: true},
		{name: "answer names an undeclared tag", path: answer(t, "unknown", "ze_nope\n"), every: true, widens: true},
		{name: "answer names every declared tag", path: answer(t, "all", "ze_alpha\nze_beta\nze_gamma\n"), every: true},
		{name: "answer names a subset", path: answer(t, "one", "ze_beta\n")},
	}

	for _, testCase := range cases {
		scope, widen := readChangeScope(testCase.path, manifestTags)
		if scope.every != testCase.every {
			t.Errorf("%s: every=%v, want %v", testCase.name, scope.every, testCase.every)
		}
		if (widen != nil) != testCase.widens {
			t.Errorf("%s: widening reason %v, want widens=%v", testCase.name, widen, testCase.widens)
		}
	}
}

// VALIDATES: the subtraction keeps only the rows a change set can move, and the
// two shipped combinations always.
// PREVENTS: a filter that drops all_features or core_only, which leaves a
// combination Ze ships unjudged while the gate still goes green.
func TestTheSubtractionNeverDropsAShippedCombination(t *testing.T) {
	rows, err := buildMatrix(manifestTags)
	if err != nil {
		t.Fatalf("build the matrix: %v", err)
	}

	scoped, err := scopeMatrix(rows, changeScope{tags: map[string]bool{"ze_beta": true}})
	if err != nil {
		t.Fatalf("scope the matrix: %v", err)
	}
	want := []string{"all_features", "core_only", "without_ze_beta"}
	if len(scoped) != len(want) {
		t.Fatalf("the scoped matrix holds %d rows, want %v", len(scoped), want)
	}
	for i, name := range want {
		if scoped[i].Name != name {
			t.Errorf("scoped row %d is %q, want %q", i, scoped[i].Name, name)
		}
	}

	// A matrix that lost a floor row is refused rather than judged.
	if _, err := scopeMatrix(rows[1:], changeScope{tags: map[string]bool{"ze_beta": true}}); err == nil {
		t.Error("a scoped matrix missing all_features was accepted")
	}
	if err := validateScoped(Matrix{rows[0]}); err == nil {
		t.Error("a one-row scoped matrix was accepted")
	}
}

// VALIDATES: a manifest line the reader cannot understand is an ERROR.
// PREVENTS: a malformed manifest shrinking the matrix silently, which is the
// same false green as a matrix nobody ran.
func TestAMalformedManifestIsAnError(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"one field":           "ze_alpha\n",
		"invalid tag":         "alpha internal/plugins/alpha\n",
		"reserved tag":        "ze_core internal/plugins/alpha\n",
		"unclean import path": "ze_alpha internal//plugins/alpha\n",
		"no tags at all":      "# nothing here\n",
	}
	for name, body := range cases {
		path := filepath.Join(dir, "manifest")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write the manifest: %v", err)
		}
		if _, err := readFeatureTags(path); err == nil {
			t.Errorf("a manifest with %s was accepted", name)
		}
	}

	if _, err := readFeatureTags(filepath.Join(dir, "absent")); err == nil {
		t.Error("a manifest that does not exist was accepted")
	}
}

// VALIDATES: the rendering is what Staticcheck reads, and a matrix that cannot
// render is refused rather than handed over.
// PREVENTS: Staticcheck being asked about an empty or short document, which
// judges less than the caller believes.
func TestTheRenderingIsCheckedAgainstItsRows(t *testing.T) {
	rows, err := buildMatrix(manifestTags)
	if err != nil {
		t.Fatalf("build the matrix: %v", err)
	}
	rendered, err := rows.Render()
	if err != nil {
		t.Fatalf("render the matrix: %v", err)
	}
	if got := strings.Count(string(rendered), "\n"); got != len(rows) {
		t.Errorf("the rendering has %d lines for %d rows", got, len(rows))
	}
	if !strings.HasPrefix(string(rendered), "all_features: -tags=ze_core,ze_distro,ze_alpha") {
		t.Errorf("the rendering does not open with all_features:\n%s", rendered)
	}

	for name, broken := range map[string]Matrix{
		"no rows":        {},
		"blank name":     {{Name: "", Tags: []string{"ze_core"}}},
		"no tags":        {{Name: "x", Tags: nil}},
		"duplicate name": {{Name: "x", Tags: []string{"ze_core"}}, {Name: "x", Tags: []string{"ze_core"}}},
		"duplicate tag":  {{Name: "x", Tags: []string{"ze_core", "ze_core"}}},
	} {
		if _, err := broken.Render(); err == nil {
			t.Errorf("a matrix with %s rendered anyway", name)
		}
	}
}

// VALIDATES: a declared deadline that is not a positive duration is refused.
// PREVENTS: a run bounded by a value nobody could parse, which would either
// never end or end at once with no verdict.
func TestABadDeadlineIsRefused(t *testing.T) {
	if got, err := DeadlineFrom(""); err != nil || got != defaultDeadline {
		t.Errorf("an unset deadline answers (%v, %v), want the default", got, err)
	}
	if got, err := DeadlineFrom("  90s "); err != nil || got != 90*time.Second {
		t.Errorf("a declared deadline answers (%v, %v), want 90s", got, err)
	}

	for _, bad := range []string{"nope", "-1s", "0"} {
		if _, err := DeadlineFrom(bad); err == nil {
			t.Errorf("the deadline %q was accepted", bad)
			continue
		} else if !strings.Contains(err.Error(), "matrix could not be judged") {
			t.Errorf("the refusal of %q is %v, want one saying the matrix could not be judged", bad, err)
		}
	}

	// The env route reads the same parse, so an unset variable answers the
	// default rather than a refusal.
	if got, err := Deadline(); err != nil || got <= 0 {
		t.Errorf("the env route answers (%v, %v), want a positive duration", got, err)
	}
}

// VALIDATES: AC-7 -- both payloads are data a JSON encoder takes.
// PREVENTS: a port that answers a rendered page, which no pipe operator can act
// on.
func TestBothAnswersAreStructuredData(t *testing.T) {
	rows, err := buildMatrix(manifestTags)
	if err != nil {
		t.Fatalf("build the matrix: %v", err)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("the matrix does not encode: %v", err)
	}
	for _, key := range []string{`"name"`, `"tags"`, `"omits"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the matrix payload has no %s key: %s", key, raw)
		}
	}

	verdictRaw, err := json.Marshal(Verdict{Rows: 3, Diagnostics: []string{"a.go:1:1: x"}, Passed: false})
	if err != nil {
		t.Fatalf("the verdict does not encode: %v", err)
	}
	for _, key := range []string{`"rows"`, `"diagnostics"`, `"passed"`} {
		if !strings.Contains(string(verdictRaw), key) {
			t.Errorf("the verdict payload has no %s key: %s", key, verdictRaw)
		}
	}
}

// VALIDATES: the two pages a person reads carry what the script printed.
// PREVENTS: a failing run whose page still reads as a pass, which is what a
// reader acts on.
func TestThePagesCarryTheirVerdicts(t *testing.T) {
	passed := Verdict{Rows: 38, Passed: true}.Text()
	if passed != "staticcheck feature matrix: checked 38 rows\n" {
		t.Errorf("the passing page is %q", passed)
	}

	failed := Verdict{Rows: 38, Diagnostics: []string{"a.go:1:1: undefined: x"}}.Text()
	if strings.Contains(failed, "checked") {
		t.Errorf("a failing page still reports a check:\n%s", failed)
	}
	if !strings.Contains(failed, "a.go:1:1: undefined: x") {
		t.Errorf("the failing page does not carry its diagnostic:\n%s", failed)
	}

	scoped := Notice{Scoped: true, Reached: 2, Judged: 4, Total: 38}.Text()
	if scoped != "staticcheck feature matrix: the change set reaches 2 feature tag(s), so 4 of 38 rows are judged\n" {
		t.Errorf("the scoped notice is %q", scoped)
	}
	if quiet := (Notice{Judged: 38, Total: 38}).Text(); quiet != "" {
		t.Errorf("an unscoped run said %q, want nothing", quiet)
	}
}

// VALIDATES: the area dispatches its two actions and refuses the two mistakes.
// PREVENTS: a verb that drifts from its gate name, which would leave the Make
// target pointing at nothing after the swap.
func TestTheAreaDispatchesItsActions(t *testing.T) {
	payload, code := Answer([]string{"rows"})
	if code != 0 {
		t.Errorf("the rows action answers %d, want 0", code)
	}
	if matrix, ok := payload.(Matrix); !ok || len(matrix) < minMatrixRows {
		t.Errorf("the rows action answered %T with %v", payload, payload)
	}
	if _, code := Answer([]string{"nope"}); code != 2 {
		t.Errorf("an unknown action answers %d, want 2", code)
	}
	if _, code := Answer([]string{"rows", "value"}); code != 1 {
		t.Errorf("a value after an action answers %d, want 1", code)
	}

	verbs := Actions()
	if len(verbs.Actions) != 2 {
		t.Fatalf("the area holds %d actions, want 2", len(verbs.Actions))
	}
	if verbs.Actions[0].Verb != "check" || verbs.Actions[1].Verb != "rows" {
		t.Errorf("the verbs are %q and %q, want check and rows", verbs.Actions[0].Verb, verbs.Actions[1].Verb)
	}
}
