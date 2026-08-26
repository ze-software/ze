// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-8 -- the tracked-build gate is
// called as a function, answers structured data, and keeps 1 (the commit does
// not compile) apart from 2 (the run could not judge it).
// PREVENTS: the vacuity fail-open this gate exists for. `go build ./...` exits 0
// over a pattern that matched nothing buildable, so a flavor that compiled zero
// packages would report success.

package trackedbuild

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/letools/lepath"
)

// fixture writes the selftest's own fixture tree and answers the environment
// every build case runs in.
func fixture(t *testing.T) selftestEnv {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), selftestDeadline)
	t.Cleanup(cancel)

	env, err := WriteFixture(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("write the fixture: %v", err)
	}
	return env
}

// VALIDATES: both polarities of the anchor guard over one fixture module.
// PREVENTS: a tag set that selects none of the flavor's own code passing as a
// build, which is what `go build ./...` on its own reports.
func TestBuildTellsACoherentFlavorFromAVacuousOne(t *testing.T) {
	env := fixture(t)

	if result := Build(env.ctx, env.dir, okFlavor, nil, 1); !result.OK {
		t.Fatalf("a coherent flavor was refused: %s", result.Output)
	}

	vacuous := Flavor{Name: "v", Tags: []string{"ze_absent"}, Anchor: "./cmd/probe", AnchorFiles: []string{"gated.go"}, Why: "v"}
	result := Build(env.ctx, env.dir, vacuous, nil, 1)
	if result.OK {
		t.Error("a flavor whose tags select none of its anchor files was accepted")
	}
	if !strings.Contains(result.Output, "gated.go") {
		t.Errorf("the anchor failure does not name the unselected file: %s", result.Output)
	}

	gone := Flavor{Name: "g", Tags: []string{"ze_probe"}, Anchor: "./cmd/absent", AnchorFiles: []string{"main.go"}, Why: "g"}
	missing := Build(env.ctx, env.dir, gone, nil, 1)
	if missing.OK {
		t.Error("a flavor whose anchor package does not exist was accepted")
	}
	if !strings.Contains(missing.Output, "anchor package did not resolve") {
		t.Errorf("a missing anchor package was misdiagnosed: %s", missing.Output)
	}
}

// VALIDATES: the package floor refuses a tree far below it, and the passing run
// above proves the floor is not simply always-red.
// PREVENTS: a tree that lost most of the module passing as "compiles".
func TestThePackageFloorRefusesAShrunkTree(t *testing.T) {
	env := fixture(t)

	if Build(env.ctx, env.dir, okFlavor, nil, 99).OK {
		t.Error("a one-package tree passed a floor of 99")
	}
	if result := Build(env.ctx, env.dir, okFlavor, nil, 1); !result.OK {
		t.Errorf("the same tree failed a floor of 1: %s", result.Output)
	}
}

// VALIDATES: both sanity guards refuse the trees they exist for, and accept the
// trees they must.
// PREVENTS: a build with nothing to compile reporting a clean commit for a tree
// that never existed.
func TestTheSanityGuardsRefuseAnUnbuildableTree(t *testing.T) {
	env := fixture(t)

	if _, err := FeatureTags(env.bare); err == nil {
		t.Error("FeatureTags accepted a tree with no feature manifest")
	}
	tags, err := FeatureTags(env.dir)
	if err != nil {
		t.Fatalf("FeatureTags refused the fixture: %v", err)
	}
	if len(tags) != 1 || tags[0] != "ze_probe" {
		t.Errorf("FeatureTags answered %v, want the fixture's one tag", tags)
	}

	if err := SanityCheck(env.ctx, env.dir, "HEAD", env.bare); err == nil {
		t.Error("SanityCheck accepted a tree with no go.mod")
	}
	if err := SanityCheck(env.ctx, env.probe, "HEAD", env.dir); err == nil {
		t.Error("SanityCheck accepted a tree with no vendor directory against a commit that tracks one")
	}
}

// VALIDATES: the commit-tracking probe answers both polarities.
// PREVENTS: an error read as "absent", which would SKIP the partial-extraction
// check and quietly restore the fail-open it exists to close.
func TestCommitHasPathAnswersBothPolarities(t *testing.T) {
	env := fixture(t)

	tracked, err := CommitHasPath(env.ctx, env.probe, "HEAD", "vendor/modules.txt")
	if err != nil || !tracked {
		t.Errorf("a tracked path answered (%v, %v), want (true, nil)", tracked, err)
	}
	absent, err := CommitHasPath(env.ctx, env.probe, "HEAD", "no/such/path.txt")
	if err != nil || absent {
		t.Errorf("an absent path answered (%v, %v), want (false, nil)", absent, err)
	}
	if _, err := CommitHasPath(env.ctx, env.bare, "HEAD", "go.mod"); err == nil {
		t.Error("a directory that is not a git repository answered no error")
	}
}

// VALIDATES: the selftest table passes over its own fixtures and answers 0.
// PREVENTS: a selftest that cannot pass, which would be discovered only by the
// gate that runs it.
func TestTheSelftestPassesOverItsOwnFixtures(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}

	report, err := Selftest(root)
	if err != nil {
		t.Fatalf("the selftest could not run: %v", err)
	}
	if failures := report.Failures(); len(failures) != 0 {
		t.Errorf("the selftest failed: %v", failures)
	}
	if code := report.Code(2); code != 0 {
		t.Errorf("a passing selftest answers %d, want 0", code)
	}
	if len(report.Results) != len(selftestCases) {
		t.Fatalf("the selftest answered %d rows for %d cases", len(report.Results), len(selftestCases))
	}
	for i, result := range report.Results {
		if result.Case != selftestCases[i].name {
			t.Errorf("row %d names %q, want %q", i, result.Case, selftestCases[i].name)
		}
	}
}

// VALIDATES: a declared option that cannot be used is refused.
// PREVENTS: a run bounded by a value nobody could parse, or judged against a
// floor nobody could read.
func TestABadOptionIsRefused(t *testing.T) {
	options, err := OptionsFrom("", "", "", "")
	if err != nil {
		t.Fatalf("the defaults were refused: %v", err)
	}
	if options.Rev != "HEAD" || options.Keep || options.PackageFloor != DefaultPackageFloor || options.Deadline != DefaultDeadline {
		t.Errorf("the defaults are %+v", options)
	}

	options, err = OptionsFrom(" 7abe8a07e ", "true", "5", "90s")
	if err != nil {
		t.Fatalf("a declared set was refused: %v", err)
	}
	if options.Rev != "7abe8a07e" || !options.Keep || options.PackageFloor != 5 || options.Deadline != 90*time.Second {
		t.Errorf("the declared set answered %+v", options)
	}

	for name, args := range map[string][4]string{
		"a floor that is not a number":    {"", "", "many", ""},
		"a floor below one":               {"", "", "0", ""},
		"a deadline that will not parse":  {"", "", "", "soon"},
		"a deadline that is not positive": {"", "", "", "-1s"},
		"a keep that is not a boolean":    {"", "maybe", "", ""},
	} {
		if _, err := OptionsFrom(args[0], args[1], args[2], args[3]); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// VALIDATES: AC-7 -- both payloads are data a JSON encoder takes, with the
// script's own keys.
// PREVENTS: a port that answers a rendered page, which no pipe operator can act
// on.
func TestBothAnswersAreStructuredData(t *testing.T) {
	raw, err := json.Marshal(Report{
		Rev: "HEAD", Commit: "abc", Tree: "scratch/tracked-build/1", Features: []string{"ze_web"},
		PackageFloor: DefaultPackageFloor, OK: true,
		Results: []Result{{Name: "distro", Tags: "ze_core", Anchor: "./cmd/ze", Packages: 672, OK: true, Seconds: 1.5}},
	})
	if err != nil {
		t.Fatalf("the report does not encode: %v", err)
	}
	for _, key := range []string{
		`"rev"`, `"commit"`, `"tree"`, `"features"`, `"results"`, `"package-floor"`, `"ok"`,
		`"name"`, `"tags"`, `"anchor"`, `"packages"`, `"seconds"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}

	matrixRaw, err := json.Marshal(BuildMatrix())
	if err != nil {
		t.Fatalf("the matrix does not encode: %v", err)
	}
	for _, key := range []string{`"name"`, `"tags"`, `"features"`, `"anchor"`, `"anchor-files"`, `"why"`} {
		if !strings.Contains(string(matrixRaw), key) {
			t.Errorf("the matrix payload has no %s key: %s", key, matrixRaw)
		}
	}
}

// VALIDATES: the page carries the verdict and the diagnosis stays apart from it.
// PREVENTS: a failing run whose page still reads OK, and a diagnosis leaking
// into the answer a caller pipes.
func TestThePageAndTheDiagnosisStayApart(t *testing.T) {
	clean := Report{Rev: "HEAD", Commit: "abcdef0123456789", PackageFloor: DefaultPackageFloor, OK: true,
		Results: []Result{{Name: "distro", Tags: "ze_core", Packages: 672, OK: true, Seconds: 10.9}}}
	page := clean.Text()
	if !strings.Contains(page, "tracked-build: HEAD (abcdef012345), 0 feature tags, ./...") {
		t.Errorf("the clean page does not carry its commit line:\n%s", page)
	}
	if !strings.Contains(page, "tracked-build: OK (every flavor of the committed tree compiles)") {
		t.Errorf("the clean page does not carry its verdict:\n%s", page)
	}
	if clean.Diagnosis() != "" {
		t.Errorf("a passing run answered a diagnosis: %q", clean.Diagnosis())
	}

	broken := Report{Rev: "HEAD", Commit: "abcdef0123456789", PackageFloor: DefaultPackageFloor,
		Results: []Result{{Name: "distro", Tags: "ze_core", Packages: 672, Output: "undefined: Symbol"}}}
	if strings.Contains(broken.Text(), "OK (") {
		t.Errorf("a failing page still reports success:\n%s", broken.Text())
	}
	diagnosis := broken.Diagnosis()
	for _, want := range []string{"the tree GIT HOLDS does not compile", "undefined: Symbol", "Commit the producer"} {
		if !strings.Contains(diagnosis, want) {
			t.Errorf("the diagnosis has no %q:\n%s", want, diagnosis)
		}
	}

	incomplete := Report{Rev: "HEAD", Incomplete: true,
		Results: []Result{{Name: "distro", Output: "killed"}}}
	if !strings.Contains(incomplete.Diagnosis(), "INCOMPLETE") {
		t.Errorf("an incomplete run is not named as one:\n%s", incomplete.Diagnosis())
	}
	if strings.Contains(incomplete.Diagnosis(), "does not compile") {
		t.Errorf("an incomplete run was reported as a broken commit:\n%s", incomplete.Diagnosis())
	}
}

// VALIDATES: a lowered package floor is stated on the page.
// PREVENTS: a green line pasted as evidence hiding that the shrink detector was
// weakened for that run.
func TestALoweredFloorIsSaidOutLoud(t *testing.T) {
	lowered := Report{Rev: "HEAD", Commit: "abc", PackageFloor: 1, OK: true}.Text()
	if !strings.Contains(lowered, "package floor LOWERED to 1") {
		t.Errorf("a lowered floor is not stated:\n%s", lowered)
	}
	raised := Report{Rev: "HEAD", Commit: "abc", PackageFloor: 900, OK: true}.Text()
	if !strings.Contains(raised, "package floor raised to 900") {
		t.Errorf("a raised floor is not stated:\n%s", raised)
	}
	if strings.Contains(Report{Rev: "HEAD", Commit: "abc", PackageFloor: DefaultPackageFloor, OK: true}.Text(), "floor") {
		t.Error("the default floor was announced, which is noise on every clean run")
	}
}

// VALIDATES: every flavor of the matrix names a tag-gated anchor FILE.
// PREVENTS: a flavor whose anchor is a package alone, which resolves under any
// tag set and ties the result back to nothing.
func TestEveryFlavorNamesATagGatedAnchorFile(t *testing.T) {
	for _, flavor := range BuildMatrix() {
		if flavor.Name == "" || flavor.Anchor == "" || flavor.Why == "" {
			t.Errorf("flavor %+v is missing a name, an anchor or a reason", flavor)
		}
		if len(flavor.Tags) == 0 {
			t.Errorf("flavor %s carries no build tag", flavor.Name)
		}
		if len(flavor.AnchorFiles) == 0 {
			t.Errorf("flavor %s names no anchor file, so its tags tie back to nothing", flavor.Name)
		}
	}
	if len(BuildMatrix()) < 6 {
		t.Errorf("the matrix holds %d flavors, want the six shipped ones", len(BuildMatrix()))
	}
}

// VALIDATES: the area dispatches its three actions and refuses the two
// mistakes.
// PREVENTS: a verb that drifts from its gate name, which would leave the Make
// target pointing at nothing after the swap.
func TestTheAreaDispatchesItsActions(t *testing.T) {
	payload, code := Answer([]string{"matrix"})
	if code != 0 {
		t.Errorf("the matrix action answers %d, want 0", code)
	}
	if matrix, ok := payload.(Matrix); !ok || len(matrix) < 6 {
		t.Errorf("the matrix action answered %T with %v", payload, payload)
	}
	if _, code := Answer([]string{"nope"}); code != 2 {
		t.Errorf("an unknown action answers %d, want 2", code)
	}
	if _, code := Answer([]string{"check", "value"}); code != 1 {
		t.Errorf("a value after an action answers %d, want 1", code)
	}

	verbs := Actions()
	if len(verbs.Actions) != 3 {
		t.Fatalf("the area holds %d actions, want 3", len(verbs.Actions))
	}
	for i, want := range []string{"check", "selftest", "matrix"} {
		if verbs.Actions[i].Verb != want {
			t.Errorf("action %d is %q, want %q", i, verbs.Actions[i].Verb, want)
		}
	}
}

// VALIDATES: the scratch tree is emptied before extraction.
// PREVENTS: a reused directory putting a file back into the view that the
// commit under test deleted -- `tar -x` overwrites archived paths but never
// removes extras.
func TestTheScratchTreeIsEmptiedBeforeUse(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	dir, err := scratchTree(ctx, root)
	if err != nil {
		t.Fatalf("create the scratch tree: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) }) //nolint:errcheck // temp fixture

	stale := filepath.Join(dir, "stale.txt")
	if err := os.WriteFile(stale, []byte("x"), 0o600); err != nil {
		t.Fatalf("write the stale file: %v", err)
	}
	again, err := scratchTree(ctx, root)
	if err != nil {
		t.Fatalf("recreate the scratch tree: %v", err)
	}
	if again != dir {
		t.Fatalf("the scratch path moved between runs of one process: %s then %s", dir, again)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("a stale file survived, so a deleted path could be put back into the view")
	}
}
