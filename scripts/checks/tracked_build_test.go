// Tests for the //go:build ignore checker tracked_build.go.
//
// The gate is a fail-closed guard. `ai/rules/evidence.md` requires driving it
// from its ENTRY POINT rather than from its helpers, so every test here compiles
// the checker and runs the binary against a throwaway git repository. The live
// checkout cannot be doctored to prove the gate fires: withholding a real
// producer file from a real commit IS the defect the gate exists to refuse.

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

const trackedBuildTimeout = 300 * time.Second

// fixtureFloor lowers the package floor for the fixture repositories, which hold
// a handful of packages rather than the repository's 637.
const fixtureFloor = "--package-floor=1"

// fixtureRepo creates a git repository holding `files`, commits everything in
// `committed`, and leaves the rest in the working tree only.
//
// GIT_CONFIG_GLOBAL/SYSTEM are pinned to os.DevNull so the fixture inherits none
// of the developer's git configuration, notably commit signing, which would fail
// in a throwaway repository and which this project never disables by flag
// (ai/rules/git-safety.md).
func fixtureRepo(t *testing.T, files map[string]string, committed []string) string {
	t.Helper()
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), trackedBuildTimeout)
	defer cancel()

	git := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+os.DevNull,
			"GIT_CONFIG_SYSTEM="+os.DevNull,
			"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
			"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	git("init", "--quiet")
	git(append([]string{"add", "--"}, committed...)...)
	git("commit", "--quiet", "-m", "fixture")
	return dir
}

// baseFixture mirrors the tag TOPOLOGY the build matrix depends on, not just its
// shape: cmd/ze/main.go carries no constraint (so the package resolves under any
// tag set, which is the trap the anchor-file guard exists to close), and every
// anchor file the matrix names carries the same build tag its real counterpart
// does.
func baseFixture() map[string]string {
	gated := func(constraint, sym string) string {
		return "//go:build " + constraint + "\n\npackage main\n\nfunc " + sym + "() {}\n"
	}
	return map[string]string{
		"go.mod":            "module example.invalid/fixture\n\ngo 1.21\n",
		"feature-gates.txt": "# tag\tpath\nze_web\tinternal/component/web\n",
		"pkg/consumer.go":   "package pkg\n\nfunc Use() int { return Produce() }\n",
		"pkg/producer.go":   "package pkg\n\nfunc Produce() int { return 1 }\n",
		// No build constraint, exactly like the real one.
		"cmd/ze/main.go": "package main\n\nimport \"example.invalid/fixture/pkg\"\n\n" +
			"func main() { _ = pkg.Use() }\n",
		"cmd/ze/ze_core_dispatch.go":         gated("ze_core", "coreDispatch"),
		"cmd/ze/ze_test_register.go":         gated("ze_test", "testRegister"),
		"cmd/ze/setup_features_distro.go":    gated("ze_distro", "distroFeatures"),
		"cmd/ze/setup_features_appliance.go": gated("ze_appliance", "applianceFeatures"),
		"cmd/ze/setup_features_setup.go":     gated("ze_setup", "setupFeatures"),
		"cmd/ze/setup_dispatch.go":           gated("ze_setup && !ze_core", "setupDispatch"),
		"cmd/ze-installer/main.go":           gated("linux && ze_installer", "main"),
	}
}

// allFixtureFiles is every path baseFixture creates, for the coherent case.
func allFixtureFiles() []string {
	var out []string
	for path := range baseFixture() {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

// fixtureWithout is allFixtureFiles minus the named paths, for the cases that
// withhold a file from the commit.
func fixtureWithout(drop ...string) []string {
	gone := map[string]bool{}
	for _, d := range drop {
		gone[d] = true
	}
	var out []string
	for _, path := range allFixtureFiles() {
		if !gone[path] {
			out = append(out, path)
		}
	}
	return out
}

// gateBinary compiles the checker once per test and returns its path.
//
// The checker is compiled rather than driven with `go run`: `go run` collapses
// every nonzero exit of the program into its own exit 1, so the difference
// between "the commit does not build" (1) and "the gate could not judge" (2),
// which is the whole fail-closed contract, would be invisible.
func gateBinary(t *testing.T, ctx context.Context) string {
	t.Helper()
	gate := filepath.Join(t.TempDir(), "tracked-build")
	compile := exec.CommandContext(ctx, "go", "build", "-o", gate, "scripts/checks/tracked_build.go")
	compile.Dir = repoRoot(t)
	compile.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile the gate: %v\n%s", err, out)
	}
	return gate
}

// runTrackedBuild drives the gate's entry point and returns its EXIT CODE.
func runTrackedBuild(t *testing.T, repo string, args ...string) (string, int) {
	t.Helper()
	return runTrackedBuildEnv(t, repo, nil, args...)
}

// runTrackedBuildEnv is runTrackedBuild with extra environment, so a test can
// put a shim ahead of the real toolchain on PATH.
func runTrackedBuildEnv(t *testing.T, repo string, env []string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), trackedBuildTimeout)
	defer cancel()

	full := append([]string{"--repo=" + repo, fixtureFloor}, args...)
	cmd := exec.CommandContext(ctx, gateBinary(t, ctx), full...)
	cmd.Dir = repoRoot(t)
	if len(env) != 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !asExitError(err, &exit) {
			t.Fatalf("run gate: %v\n%s", err, out)
		}
		code = exit.ExitCode()
	}
	return string(out), code
}

func asExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok { //nolint:errorlint // the concrete type is what we need
		*target = e
		return true
	}
	return false
}

// TestTrackedBuildGreenOnCoherentCommit
//
// VALIDATES: a commit whose producer and consumer landed together compiles, and
// the gate exits 0 and says so.
// PREVENTS: a gate that is red for everyone, which would be switched off within
// a day and prove nothing about the commits it was written for.
func TestTrackedBuildGreenOnCoherentCommit(t *testing.T) {
	repo := fixtureRepo(t, baseFixture(), allFixtureFiles())
	out, code := runTrackedBuild(t, repo)
	if code != 0 {
		t.Fatalf("coherent commit reported broken (exit %d):\n%s", code, out)
	}
	if !strings.Contains(out, "tracked-build: OK") {
		t.Fatalf("gate did not report OK:\n%s", out)
	}
}

// TestTrackedBuildRedWhenProducerUncommitted is the mutation proof for this gate.
//
// VALIDATES: a tracked file referencing a symbol that exists ONLY in an
// uncommitted file makes the gate exit 1 and name the missing symbol.
// PREVENTS: the exact class of break that reached main four times on
// 2026-08-04, a consumer committed while its producer stayed in the working
// tree. Every other check in this repository reads the working tree, where this
// fixture is perfectly green.
func TestTrackedBuildRedWhenProducerUncommitted(t *testing.T) {
	// producer.go exists on disk and is NOT committed: the working tree builds,
	// the commit does not.
	repo := fixtureRepo(t, baseFixture(), fixtureWithout("pkg/producer.go"))

	// The working tree really is green, or the fixture proves nothing about
	// where the two populations differ.
	ctx, cancel := context.WithTimeout(context.Background(), trackedBuildTimeout)
	defer cancel()
	binDir := filepath.Join(repo, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", binDir, err)
	}
	tree := exec.CommandContext(ctx, "go", "build", "-o", binDir, "./...")
	tree.Dir = repo
	tree.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := tree.CombinedOutput(); err != nil {
		t.Fatalf("fixture working tree does not build, so it cannot show the gap:\n%s", out)
	}

	out, code := runTrackedBuild(t, repo)
	if code != 1 {
		t.Fatalf("gate exit %d, want 1 (a commit missing its producer must fail):\n%s", code, out)
	}
	if !strings.Contains(out, "Produce") {
		t.Fatalf("gate did not name the missing symbol:\n%s", out)
	}
	if !strings.Contains(out, "CONSUMER committed without its PRODUCER") {
		t.Fatalf("gate did not explain the class of failure:\n%s", out)
	}
}

// TestTrackedBuildRefusesAVacuousFlavor
//
// VALIDATES: a flavor whose tags select NONE of its anchor files fails, even
// though `go build ./...` over that flavor exits 0 and the anchor PACKAGE
// resolves.
// PREVENTS: the silent no-op that two review rounds found in this gate. First
// the installer flavor pinned no GOOS and compiled nothing off Linux. Then the
// anchor-package check turned out to be inert for five of six rows, because
// cmd/ze/main.go carries no build constraint and so resolves under any tag set,
// `-tags ze_bogus` included. The guard has to name a tag-gated FILE, and this
// test is what holds it to that: the fixture keeps every file in the commit and
// only mistypes ONE build tag, so the package still resolves and the tree still
// compiles.
func TestTrackedBuildRefusesAVacuousFlavor(t *testing.T) {
	files := baseFixture()
	// The file is present and committed; its constraint no longer matches the
	// distro flavor's tags. Nothing else changes.
	files["cmd/ze/setup_features_distro.go"] = strings.Replace(
		files["cmd/ze/setup_features_distro.go"], "ze_distro", "ze_distro_typo", 1)
	repo := fixtureRepo(t, files, allFixtureFiles())

	out, code := runTrackedBuild(t, repo)
	if code != 1 {
		t.Fatalf("gate exit %d, want 1 (a flavor that compiled the wrong thing must fail):\n%s", code, out)
	}
	if !strings.Contains(out, "did not select setup_features_distro.go") {
		t.Fatalf("gate did not name the unselected anchor file:\n%s", out)
	}
	// Only the distro flavor loses a file, so the failure must be specific
	// rather than a blanket red.
	if !strings.Contains(out, "OK   installer") || !strings.Contains(out, "OK   host") {
		t.Fatalf("flavors that keep their anchor files should still pass:\n%s", out)
	}
}

// TestTrackedBuildRefusesAMissingAnchorPackage
//
// VALIDATES: a flavor whose anchor PACKAGE is absent from the commit fails, and
// says the package did not resolve rather than blaming a build tag.
// PREVENTS: a partial commit that drops a whole binary reading as success, and
// the misdiagnosis an earlier revision produced. It collapsed "the package did
// not resolve" into "the tags selected none of its files", so the message sent
// the reader hunting a build tag when the directory was simply not committed.
func TestTrackedBuildRefusesAMissingAnchorPackage(t *testing.T) {
	repo := fixtureRepo(t, baseFixture(), fixtureWithout(
		"cmd/ze/main.go", "cmd/ze/ze_core_dispatch.go", "cmd/ze/ze_test_register.go",
		"cmd/ze/setup_features_distro.go", "cmd/ze/setup_features_appliance.go",
		"cmd/ze/setup_features_setup.go", "cmd/ze/setup_dispatch.go"))

	out, code := runTrackedBuild(t, repo)
	if code != 1 {
		t.Fatalf("gate exit %d, want 1 (a missing anchor package must fail):\n%s", code, out)
	}
	if !strings.Contains(out, "anchor package did not resolve") {
		t.Fatalf("gate blamed the wrong cause; the package is absent, not the tags:\n%s", out)
	}
	if !strings.Contains(out, "./cmd/ze") {
		t.Fatalf("gate did not name the missing anchor package:\n%s", out)
	}
	if !strings.Contains(out, "OK   installer") {
		t.Fatalf("the installer flavor keeps its anchor and should still pass:\n%s", out)
	}
}

// TestTrackedBuildRefusesATreeBelowThePackageFloor
//
// VALIDATES: a tree that selects fewer packages than the floor fails, with the
// count in the message.
// PREVENTS: an extraction that silently shrank being read as "it compiles".
func TestTrackedBuildRefusesATreeBelowThePackageFloor(t *testing.T) {
	repo := fixtureRepo(t, baseFixture(), allFixtureFiles())
	// The fixture holds three packages; a floor of 99 is unreachable for it.
	out, code := runTrackedBuild(t, repo, "--package-floor=99")
	if code != 1 {
		t.Fatalf("gate exit %d, want 1 (a tree below the floor must fail):\n%s", code, out)
	}
	if !strings.Contains(out, "below the floor of 99") {
		t.Fatalf("gate did not report the floor:\n%s", out)
	}
}

// TestTrackedBuildRefusesATreeWithoutGoMod
//
// VALIDATES: `sanityCheck` refuses an extracted tree with no go.mod, exit 2.
// PREVENTS: fail-open. `go build ./...` over a tree with nothing to compile
// exits 0, so this guard is the difference between "clean commit" and "the gate
// judged nothing".
func TestTrackedBuildRefusesATreeWithoutGoMod(t *testing.T) {
	files := baseFixture()
	delete(files, "go.mod")
	repo := fixtureRepo(t, files, fixtureWithout("go.mod"))
	out, code := runTrackedBuild(t, repo)
	if code != 2 {
		t.Fatalf("gate exit %d, want 2 (no go.mod means the gate cannot judge):\n%s", code, out)
	}
	if !strings.Contains(out, "no go.mod") {
		t.Fatalf("gate did not diagnose the missing go.mod:\n%s", out)
	}
}

// TestTrackedBuildRefusesATreeWithoutFeatureGates
//
// VALIDATES: `featureTags` refuses a tree with no feature-gates.txt, exit 2.
// PREVENTS: silently building every feature-carrying flavor with an EMPTY
// feature set, which compiles far less code and still reports OK.
func TestTrackedBuildRefusesATreeWithoutFeatureGates(t *testing.T) {
	files := baseFixture()
	delete(files, "feature-gates.txt")
	repo := fixtureRepo(t, files, fixtureWithout("feature-gates.txt"))
	out, code := runTrackedBuild(t, repo)
	if code != 2 {
		t.Fatalf("gate exit %d, want 2 (no feature manifest means the gate cannot judge):\n%s", code, out)
	}
	if !strings.Contains(out, "feature-gates.txt") {
		t.Fatalf("gate did not diagnose the missing manifest:\n%s", out)
	}
}

// TestTrackedBuildSelftestPasses
//
// VALIDATES: `--selftest` proves the vacuity guards still fire, and exits 0.
// PREVENTS: `make ze-repository-tracked-build-check` running a gate whose guards were
// broken; the target runs the selftest before it judges the live tree, matching
// every other checker in this directory.
func TestTrackedBuildSelftestPasses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), trackedBuildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, gateBinary(t, ctx), "--selftest")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("selftest failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "selftest OK") {
		t.Fatalf("selftest did not report OK:\n%s", out)
	}
}

// TestTrackedBuildRemovesItsScratchTree
//
// VALIDATES: the extracted tree is gone once the run ends, in both the green and
// the red case.
// PREVENTS: a ~180MB copy of the repository accumulating under tmp/ on every
// verify run in a checkout several sessions share.
func TestTrackedBuildRemovesItsScratchTree(t *testing.T) {
	for _, tc := range []struct {
		name      string
		committed []string
	}{
		{"green", allFixtureFiles()},
		{"red", fixtureWithout("pkg/producer.go")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := fixtureRepo(t, baseFixture(), tc.committed)
			runTrackedBuild(t, repo)
			leftovers, err := filepath.Glob(filepath.Join(repo, "tmp", "tracked-build", "*"))
			if err != nil {
				t.Fatalf("glob: %v", err)
			}
			if len(leftovers) != 0 {
				t.Fatalf("gate left its extracted tree behind: %v", leftovers)
			}
		})
	}
}

// TestTrackedBuildKeepsTreeOnRequest
//
// VALIDATES: --keep leaves the extracted tree in place for inspection.
// PREVENTS: the failure message advertising a diagnosis path that does nothing.
func TestTrackedBuildKeepsTreeOnRequest(t *testing.T) {
	repo := fixtureRepo(t, baseFixture(), allFixtureFiles())
	if _, code := runTrackedBuild(t, repo, "--keep"); code != 0 {
		t.Fatalf("gate exit %d on a coherent commit", code)
	}
	leftovers, err := filepath.Glob(filepath.Join(repo, "tmp", "tracked-build", "*", "go.mod"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(leftovers) == 0 {
		t.Fatal("--keep did not leave the extracted tree behind")
	}
}

// TestTrackedBuildRejectsUnknownArgument
//
// VALIDATES: an argument the gate does not understand exits 2 rather than being
// ignored.
// PREVENTS: a typo (`--rev 7abe8a07e` with a space, `--reev=...`) silently
// building HEAD while the operator believes another commit was judged.
func TestTrackedBuildRejectsUnknownArgument(t *testing.T) {
	repo := fixtureRepo(t, baseFixture(), allFixtureFiles())
	out, code := runTrackedBuild(t, repo, "--reev=HEAD")
	if code != 2 {
		t.Fatalf("unknown argument exit %d, want 2:\n%s", code, out)
	}
}

// TestTrackedBuildRefusesRepositoryWithoutCommits
//
// VALIDATES: a repository whose --rev names no commit exits 2 with a diagnosis.
// PREVENTS: fail-open. Building nothing and reporting OK is the one outcome a
// guard must never produce (ai/rules/evidence.md).
func TestTrackedBuildRefusesRepositoryWithoutCommits(t *testing.T) {
	repo := fixtureRepo(t, baseFixture(), allFixtureFiles())
	out, code := runTrackedBuild(t, repo, "--rev=refs/heads/no-such-branch")
	if code != 2 {
		t.Fatalf("unresolvable rev exit %d, want 2:\n%s", code, out)
	}
	if !strings.Contains(out, "does not name a commit") {
		t.Fatalf("gate did not diagnose the bad rev:\n%s", out)
	}
}

// TestTrackedBuildExpiredDeadlineNeverReportsABrokenCommit
//
// VALIDATES: a run whose deadline expires exits 2 (the gate could not judge) and
// names the deadline, never exit 1 (the commit does not compile).
// PREVENTS: the misreading a review found in an earlier revision. `build` runs
// under exec.CommandContext, so an expired deadline kills the build in flight
// and it fails like any compile error. Classifying that as "the tree GIT HOLDS
// does not compile" sends the reader hunting an uncommitted producer that does
// not exist. An earlier attempt to avoid a wording problem skipped the deadline
// check on the final flavor and reintroduced exactly that.
//
// The deadline here expires before the run reaches the build loop, which is what
// makes the test deterministic; the loop's own expiry takes the same
// classification path in `run`.
func TestTrackedBuildExpiredDeadlineNeverReportsABrokenCommit(t *testing.T) {
	repo := fixtureRepo(t, baseFixture(), allFixtureFiles())
	out, code := runTrackedBuild(t, repo, "--deadline=1ms")
	if code == 1 {
		t.Fatalf("an expired deadline was reported as a broken commit:\n%s", out)
	}
	if code != 2 {
		t.Fatalf("gate exit %d, want 2 (the gate could not judge):\n%s", code, out)
	}
	if !strings.Contains(out, "deadline expired") {
		t.Fatalf("gate did not name the deadline as the cause:\n%s", out)
	}
	if strings.Contains(out, "does not compile") {
		t.Fatalf("gate blamed the commit for a timeout:\n%s", out)
	}
}

// TestTrackedBuildDeadlineOnTheLastFlavorIsIncomplete
//
// VALIDATES: a deadline that expires while the LAST flavor is building yields
// exit 2, marks the run INCOMPLETE, and never says the commit does not compile.
// PREVENTS: the fail-open a review round found and a first fix for it missed.
// `build` runs under exec.CommandContext, so a killed build fails exactly like a
// compile error. An earlier revision skipped the deadline check on the final
// iteration -- to avoid saying "the rest were not judged" when none remained --
// which turned a timeout on that flavor into "the tree GIT HOLDS does not
// compile" plus advice to commit a producer that was never missing.
//
// The LAST flavor specifically is what makes this discriminating. A shim that
// delayed every flavor expires during the FIRST one, where the skipped check
// never mattered, and the mutant survived it. This shim delays only the
// installer flavor (`ze_installer`, the final row) and runs the real toolchain
// for the rest, so the expiry lands exactly where the defect lived.
func TestTrackedBuildDeadlineOnTheLastFlavorIsIncomplete(t *testing.T) {
	repo := fixtureRepo(t, baseFixture(), allFixtureFiles())

	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("locate go: %v", err)
	}
	shim := t.TempDir()
	body := "#!/bin/sh\ncase \" $* \" in *ze_installer*) exec sleep 120 ;; esac\nexec " +
		realGo + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(shim, "go"), []byte(body), 0o700); err != nil { //nolint:gosec // a test shim must be executable
		t.Fatalf("write go shim: %v", err)
	}

	// The five earlier flavors build a three-package fixture, well inside this
	// budget; the sixth then blocks until the deadline kills it.
	out, code := runTrackedBuildEnv(t, repo,
		[]string{"PATH=" + shim + string(os.PathListSeparator) + os.Getenv("PATH")},
		"--deadline=25s")

	if code == 1 {
		t.Fatalf("a deadline on the last flavor was reported as a broken commit:\n%s", out)
	}
	if code != 2 {
		t.Fatalf("gate exit %d, want 2 (the gate could not judge):\n%s", code, out)
	}
	if !strings.Contains(out, "INCOMPLETE") {
		t.Fatalf("gate did not mark the run incomplete:\n%s", out)
	}
	if !strings.Contains(out, "still building when") {
		t.Fatalf("gate did not name the deadline as the cause:\n%s", out)
	}
	// The expiry must land on the LAST flavor, or this test silently stops
	// killing the mutant it exists for: an expiry on an earlier flavor passes
	// every assertion above while exercising a path that was never broken. A row
	// added after `installer`, or five real builds overrunning the budget, would
	// do exactly that -- and this turns it red rather than vacuous.
	if !strings.Contains(out, "the installer flavor was still building") {
		t.Fatalf("the deadline expired on some other flavor, so this test no longer "+
			"covers the last-flavor case it was written for:\n%s", out)
	}
	if strings.Contains(out, "Commit the producer") {
		t.Fatalf("gate gave commit-the-producer advice for a timeout:\n%s", out)
	}
}

// TestTrackedBuildRejectsABadDeadline
//
// VALIDATES: a malformed or non-positive --deadline exits 2 rather than being
// ignored.
// PREVENTS: a typo silently restoring the 25-minute default while the operator
// believes they shortened it.
func TestTrackedBuildRejectsABadDeadline(t *testing.T) {
	repo := fixtureRepo(t, baseFixture(), allFixtureFiles())
	for _, bad := range []string{"--deadline=0s", "--deadline=nonsense", "--deadline=-5s"} {
		out, code := runTrackedBuild(t, repo, bad)
		if code != 2 {
			t.Errorf("%s: exit %d, want 2:\n%s", bad, code, out)
		}
	}
}

// makeZeTags extracts the tag spec the Makefile's `ze` target builds bin/ze with.
var makeZeTags = regexp.MustCompile(`(?m)^\tCGO_ENABLED=0 \$\(GO\) build -tags '([^']*)'[^\n]*\$\(ZEBIN_ZE\) \./cmd/ze$`)

// matrixRow mirrors the fields --matrix emits, so the assertion below reads the
// gate's own data rather than a text search over its source.
type matrixRow struct {
	Name        string   `json:"Name"`
	Tags        []string `json:"Tags"`
	Features    bool     `json:"Features"`
	GOOS        string   `json:"GOOS"`
	Anchor      string   `json:"Anchor"`
	AnchorFiles []string `json:"AnchorFiles"`
}

// TestTrackedBuildPrimaryFlavorMatchesMakeZe
//
// VALIDATES: the `distro` row's tags are EXACTLY the literal tags `make ze-build`
// builds bin/ze with, derived from the Makefile and compared against the gate's
// own `--matrix` output (ai/rules/evidence.md).
// PREVENTS: the daemon's tag set changing in the Makefile while the gate keeps
// compiling the old one. An earlier version of this test searched the checker's
// SOURCE TEXT for each tag, so it passed as long as the string appeared anywhere
// in the file, including inside another row or a comment.
func TestTrackedBuildPrimaryFlavorMatchesMakeZe(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	m := makeZeTags.FindSubmatch(raw)
	if m == nil {
		t.Fatal("no `CGO_ENABLED=0 $(GO) build -tags '...' ... $(ZEBIN_ZE) ./cmd/ze` line in the Makefile; the ze target moved")
	}
	var want []string
	wantFeatures := false
	for tag := range strings.FieldsSeq(string(m[1])) {
		switch tag {
		case "$(ZE_FEATURES)":
			wantFeatures = true
		case "$(ZE_TAGS)":
			// operator-supplied extras, never part of the baseline
		default:
			want = append(want, tag)
		}
	}
	if len(want) == 0 {
		t.Fatal("parsed no literal tags from the Makefile's ze target; this test must not pass vacuously")
	}

	ctx, cancel := context.WithTimeout(context.Background(), trackedBuildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, gateBinary(t, ctx), "--matrix")
	cmd.Dir = repoRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--matrix: %v", err)
	}
	var rows []matrixRow
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("decode --matrix: %v\n%s", err, out)
	}

	var distro *matrixRow
	for i := range rows {
		if rows[i].Name == "distro" {
			distro = &rows[i]
		}
		if rows[i].Anchor == "" {
			t.Errorf("flavor %q has no anchor package, so it can compile nothing and still pass", rows[i].Name)
		}
		if len(rows[i].AnchorFiles) == 0 {
			t.Errorf("flavor %q names no anchor FILE, so its tags are unpinned: a package with one "+
				"unconstrained file resolves under any tag set", rows[i].Name)
		}
	}
	if distro == nil {
		t.Fatal("--matrix has no `distro` flavor; the row the four 2026-08-04 breaks lived in is gone")
	}
	if strings.Join(distro.Tags, " ") != strings.Join(want, " ") {
		t.Errorf("distro tags %v, Makefile builds bin/ze with %v", distro.Tags, want)
	}
	if distro.Features != wantFeatures {
		t.Errorf("distro Features=%v, Makefile ze target expands $(ZE_FEATURES)=%v", distro.Features, wantFeatures)
	}
}

// TestTrackedBuildOSGatedFlavorsPinGOOS
//
// VALIDATES: every flavor whose anchor package is OS-gated in this repository
// pins the matching GOOS.
// PREVENTS: the no-op that BLOCKER-1 of the first review found. cmd/ze-installer
// is `//go:build linux && ze_installer`, so a flavor that leaves GOOS empty
// compiles zero installer packages on a non-Linux host.
func TestTrackedBuildOSGatedFlavorsPinGOOS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), trackedBuildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, gateBinary(t, ctx), "--matrix")
	cmd.Dir = repoRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--matrix: %v", err)
	}
	var rows []matrixRow
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("decode --matrix: %v\n%s", err, out)
	}

	for _, row := range rows {
		anchorDir := filepath.Join(repoRoot(t), filepath.FromSlash(strings.TrimPrefix(row.Anchor, "./")))
		entries, readErr := os.ReadDir(anchorDir)
		if readErr != nil {
			t.Errorf("flavor %q anchors on %s, which does not exist: %v", row.Name, row.Anchor, readErr)
			continue
		}
		osGated := false
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			body, readErr := os.ReadFile(filepath.Join(anchorDir, e.Name())) //nolint:gosec // path derived from the gate's own matrix
			if readErr != nil {
				t.Fatalf("read %s: %v", e.Name(), readErr)
			}
			for line := range strings.SplitSeq(string(body), "\n") {
				if strings.HasPrefix(line, "//go:build ") && strings.Contains(line, "linux") {
					osGated = true
				}
			}
		}
		if osGated && row.GOOS == "" {
			t.Errorf("flavor %q anchors on %s, whose files are linux-gated, but pins no GOOS: "+
				"`go build ./...` would skip it and still exit 0 on another host", row.Name, row.Anchor)
		}
	}
}
