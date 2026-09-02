// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-8 -- the feature matrix is
// derived by a function call, answers structured rows, and keeps 1 (the tree
// does not type-check) apart from 2 (the matrix could not be judged).
// PREVENTS: a scoped run that silently drops a shipped combination. all_features
// and core_only are what Ze ships, and a matrix missing either goes green
// having judged neither.

package staticcheckfeaturematrix

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
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

// VALIDATES: cutting a matrix into pieces deals every row to exactly one piece,
// for every matrix the manifest can produce and every count CI can be cut into.
// PREVENTS: a row no piece claims. That row is coverage which vanished with
// nothing red: the pieces all pass, the gate reads green, and the combination
// nobody type-checked is the one that breaks.
func TestEveryRowIsDealtToExactlyOnePiece(t *testing.T) {
	// The row set the assertion reads is DERIVED, never listed: one matrix per
	// tag count, so a manifest of any size is covered rather than today's.
	for tagCount := 1; tagCount <= 40; tagCount++ {
		tags := make([]string, 0, tagCount)
		for index := range tagCount {
			tags = append(tags, "ze_f"+strconv.Itoa(index))
		}
		matrix, err := buildMatrix(tags)
		if err != nil {
			t.Fatalf("build a matrix of %d tags: %v", tagCount, err)
		}

		for count := 1; count <= 12; count++ {
			claimed := make(map[string]int, len(matrix))
			for index := 1; index <= count; index++ {
				piece, err := matrix.Part(index, count)
				if err != nil {
					t.Fatalf("%d tags cut %d ways, part %d: %v", tagCount, count, index, err)
				}
				for _, row := range piece {
					claimed[row.Name]++
				}
			}
			for _, row := range matrix {
				if claimed[row.Name] != 1 {
					t.Errorf("%d tags cut %d ways: row %q is judged by %d pieces, want 1",
						tagCount, count, row.Name, claimed[row.Name])
				}
			}
			if len(claimed) != len(matrix) {
				t.Errorf("%d tags cut %d ways: the pieces judge %d distinct rows, want %d",
					tagCount, count, len(claimed), len(matrix))
			}
		}
	}
}

// VALIDATES: this checkout's own matrix is dealt whole across the pieces the
// verify population runs, and an undivided run still judges every row.
// PREVENTS: a partition proven only over synthetic tags, which would pass while
// the rows this repository really judges went uncovered.
func TestThisCheckoutIsDealtWholeAcrossItsPieces(t *testing.T) {
	tree, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}
	matrix, _, err := DeriveScoped(tree, "")
	if err != nil {
		t.Fatalf("the matrix could not be derived: %v", err)
	}

	whole, err := matrix.Part(1, 1)
	if err != nil {
		t.Fatalf("an undivided run: %v", err)
	}
	if len(whole) != len(matrix) {
		t.Errorf("an undivided run judges %d of %d rows", len(whole), len(matrix))
	}

	// The count is the verify population's, read from the stage that runs it,
	// so a change to either side is caught rather than assumed.
	count := 0
	for _, identity := range verifyengine.StagesForMode(verifyengine.Mode) {
		if identity.Identity.Command == area {
			count++
		}
	}
	if count < 2 {
		t.Fatalf("the verify population runs %d pieces of the matrix, want the cut it declares", count)
	}

	claimed := make(map[string]int, len(matrix))
	widest := 0
	for index := 1; index <= count; index++ {
		piece, err := matrix.Part(index, count)
		if err != nil {
			t.Fatalf("part %d of %d: %v", index, count, err)
		}
		if len(piece) > widest {
			widest = len(piece)
		}
		for _, row := range piece {
			claimed[row.Name]++
		}
	}
	for _, row := range matrix {
		if claimed[row.Name] != 1 {
			t.Errorf("row %q is judged by %d of the %d pieces, want 1", row.Name, claimed[row.Name], count)
		}
	}
	if widest*count < len(matrix) {
		t.Errorf("%d pieces of at most %d rows cannot hold %d rows", count, widest, len(matrix))
	}
}

// VALIDATES: the part grammar takes both keywords or neither, and refuses a
// value that is not a counting number.
// PREVENTS: `check part 3` judging a piece of a cut nobody declared, which
// judges a subset of the rows while reading as a whole run.
func TestThePartGrammarNeedsBothKeywords(t *testing.T) {
	index, count, err := partFrom(leaction.Arguments{})
	if err != nil || index != 1 || count != 1 {
		t.Errorf("an uncut run answers (%d, %d, %v), want part 1 of 1", index, count, err)
	}
	if index, count, err := partFrom(leaction.Arguments{"part": "3", "of": "6"}); err != nil || index != 3 || count != 6 {
		t.Errorf("`part 3 of 6` answers (%d, %d, %v)", index, count, err)
	}

	for name, args := range map[string]leaction.Arguments{
		"an index with no count": {"part": "3"},
		"a count with no index":  {"of": "6"},
		"an index of zero":       {"part": "0", "of": "6"},
		"a negative index":       {"part": "-1", "of": "6"},
		"a count of zero":        {"part": "1", "of": "0"},
		"a word for an index":    {"part": "first", "of": "6"},
	} {
		if _, _, err := partFrom(args); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}

	// A piece outside the cut is refused by the deal itself, so no caller can
	// ask for part 7 of 6 and receive an empty run that reads as a pass.
	rows, err := buildMatrix(manifestTags)
	if err != nil {
		t.Fatalf("build the matrix: %v", err)
	}
	if _, err := rows.Part(7, 6); err == nil {
		t.Error("part 7 of 6 was accepted")
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
	// An unset key scales with the work: the bound a 38-row run gets is not the
	// bound a 7-row piece of it gets.
	if got, err := DeadlineFrom("", 38); err != nil || got != 38*deadlinePerRow {
		t.Errorf("an unset deadline over 38 rows answers (%v, %v), want %v", got, err, 38*deadlinePerRow)
	}
	if got, err := DeadlineFrom("", 7); err != nil || got != 7*deadlinePerRow {
		t.Errorf("an unset deadline over 7 rows answers (%v, %v), want %v", got, err, 7*deadlinePerRow)
	}
	// A declared value is absolute, so the same string bounds both sizes.
	if got, err := DeadlineFrom("  90s ", 38); err != nil || got != 90*time.Second {
		t.Errorf("a declared deadline answers (%v, %v), want 90s", got, err)
	}
	// A run with no row to judge is refused rather than bounded at zero, which
	// would end at once and report no verdict.
	if _, err := DeadlineFrom("", 0); err == nil {
		t.Error("a deadline over zero rows was accepted")
	}

	for _, bad := range []string{"nope", "-1s", "0"} {
		if _, err := DeadlineFrom(bad, 38); err == nil {
			t.Errorf("the deadline %q was accepted", bad)
			continue
		} else if !strings.Contains(err.Error(), "matrix could not be judged") {
			t.Errorf("the refusal of %q is %v, want one saying the matrix could not be judged", bad, err)
		}
	}

	// The env route reads the same parse, so an unset variable answers the
	// default rather than a refusal.
	if got, err := Deadline(38); err != nil || got <= 0 {
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
	passed := Verdict{Part: 2, Parts: 6, Rows: 2, Names: []string{"core_only", "without_ze_ssh"}, Passed: true}.Text()
	if passed != "staticcheck feature matrix: part 2 of 6 checked 2 row(s): core_only, without_ze_ssh\n" {
		t.Errorf("the passing page is %q", passed)
	}

	// A piece with no row says so. Its rows are judged by a sibling piece of the
	// same run, and a reader of this log alone can see which fact it is.
	empty := Verdict{Part: 5, Parts: 6, Passed: true}.Text()
	if !strings.Contains(empty, "part 5 of 6 was dealt no row") {
		t.Errorf("an empty piece reports %q", empty)
	}

	failed := Verdict{
		Part: 1, Parts: 6, Rows: 1, Names: []string{"all_features"},
		Diagnostics: []string{"a.go:1:1: undefined: x"},
	}.Text()
	if strings.Contains(failed, "checked") {
		t.Errorf("a failing page still reports a check:\n%s", failed)
	}
	if !strings.Contains(failed, "a.go:1:1: undefined: x") {
		t.Errorf("the failing page does not carry its diagnostic:\n%s", failed)
	}
	// The scope of a red is read in one shard's log, with no sibling beside it.
	if !strings.Contains(failed, "part 1 of 6 judged 1 row(s): all_features") {
		t.Errorf("the failing page does not name the piece it judged:\n%s", failed)
	}
	// The failure group repeats the piece, so the rerun a reader is handed
	// judges the rows that failed rather than the whole matrix.
	if !strings.Contains(failed, `"rerun":"./le staticcheck-feature-matrix check part 1 of 6"`) {
		t.Errorf("the failure group does not rerun this piece:\n%s", failed)
	}
	if !strings.Contains(failed, `"group-id":"files:staticcheck-feature-matrix/check/part/1/of/6"`) {
		t.Errorf("the failure group does not identify this piece:\n%s", failed)
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
// PREVENTS: a verb that drifts from its registered native action and becomes
// unreachable.
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
	if _, code := Answer([]string{"rows", "value"}); code != 2 {
		t.Errorf("a value after an action answers %d, want 2", code)
	}

	verbs := Actions()
	if len(verbs.Actions) != 2 {
		t.Fatalf("the area holds %d actions, want 2", len(verbs.Actions))
	}
	if verbs.Actions[0].Verb != "check" || verbs.Actions[1].Verb != "rows" {
		t.Errorf("the verbs are %q and %q, want check and rows", verbs.Actions[0].Verb, verbs.Actions[1].Verb)
	}
}
