// The fuzz port's contract, carried over from scripts/le/fuzz_test.py case for
// case.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- discovery finds the same targets
// the Python finds, the argv is the same argv, and the sweep stops where the
// Python sweep stops.
// PREVENTS: the two failures from the deleted Make generator.
// An unanchored -fuzz=Name can match multiple targets with the same prefix.
// A ./... package path can also match multiple packages. Go refuses both ambiguous forms.

package fuzz

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/letools/gotoolchain"
	"github.com/ze-software/ze/letools/lepath"
)

// probeChain is a toolchain with no manifest behind it, for the argv tests.
// The tags are not what those tests are about, and a fixture checkout would
// only make the assertion harder to read.
func probeChain() gotoolchain.Toolchain {
	return gotoolchain.Toolchain{Features: []string{"ze_bgp"}, Timeout: "20m"}
}

// tree writes a throwaway checkout holding internal/<path> = <content>. The key
// spells a slash as a double underscore, as the Python fixture did.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, text := range files {
		path := filepath.Join(root, "internal", filepath.FromSlash(strings.ReplaceAll(rel, "__", "/")))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return root
}

// discover fails the test when the walk fails.
// Every case treats an unreadable fixture as a broken test, not a finding.
func discover(t *testing.T, root string) []Target {
	t.Helper()
	targets, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover(%s): %v", root, err)
	}
	return targets
}

func names(targets []Target) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		out = append(out, target.Name)
	}
	return out
}

// --- The two Go fuzz requirements, asserted on the command that runs --------

func TestTheNameIsAnchored(t *testing.T) {
	argv := Target{Name: "FuzzParseVPN", Package: "./internal/x"}.Command(probeChain(), DefaultFuzzTime, DefaultTimeout)
	if !slices.Contains(argv, "-fuzz=^FuzzParseVPN$") {
		t.Errorf("unanchored: %v -- FuzzParseVPN also matches FuzzParseVPNAddPath", argv)
	}
}

func TestThePackageIsAnExactDirectory(t *testing.T) {
	argv := Target{Name: "FuzzX", Package: "./internal/plugins/isis/packet"}.Command(probeChain(), DefaultFuzzTime, DefaultTimeout)
	if !slices.Contains(argv, "./internal/plugins/isis/packet") {
		t.Fatalf("the package is missing: %v", argv)
	}
	for _, part := range argv {
		if strings.HasSuffix(part, "/...") {
			t.Errorf("a wildcard reached go test: %v", argv)
		}
	}
}

func TestNoDiscoveredPackageIsAWildcard(t *testing.T) {
	for _, target := range discoverRealTree(t) {
		if strings.HasSuffix(target.Package, "...") {
			t.Errorf("discovery answered a wildcard package: %s", target.Package)
		}
	}
}

func TestEveryDiscoveredNameIsAnchoredInItsCommand(t *testing.T) {
	chain := probeChain()
	for _, target := range discoverRealTree(t) {
		var want strings.Builder
		want.WriteString("-fuzz=^")
		want.WriteString(target.Name)
		want.WriteString("$")
		if !slices.Contains(target.Command(chain, DefaultFuzzTime, DefaultTimeout), want.String()) {
			t.Errorf("%s is not anchored in its command", target.Name)
		}
	}
}

func TestTheBudgetIsCarried(t *testing.T) {
	argv := Target{Name: "FuzzX", Package: "./internal/x"}.Command(probeChain(), DefaultFuzzTime, DefaultTimeout)
	for _, want := range []string{"-fuzztime=" + DefaultFuzzTime, "-timeout=" + DefaultTimeout} {
		if !slices.Contains(argv, want) {
			t.Errorf("%s is missing from %v", want, argv)
		}
	}
}

func TestALongerRunOverridesOnlyTheFuzztime(t *testing.T) {
	argv := Target{Name: "FuzzX", Package: "./internal/x"}.Command(probeChain(), "30s", DefaultTimeout)
	if !slices.Contains(argv, "-fuzztime=30s") {
		t.Errorf("the longer budget did not reach go test: %v", argv)
	}
	if !slices.Contains(argv, "-timeout="+DefaultTimeout) {
		t.Errorf("the hard timeout moved with it: %v", argv)
	}
}

// --- Discovery -------------------------------------------------------------

func TestAFuncFuzzIsFoundWithItsExactPackage(t *testing.T) {
	root := tree(t, map[string]string{"a__b__x_test.go": "package b\n\nfunc FuzzThing(f *testing.F) {}\n"})
	got := discover(t, root)
	want := []Target{{Name: "FuzzThing", Package: "./internal/a/b"}}
	if !slices.Equal(got, want) {
		t.Errorf("Discover = %v, want %v", got, want)
	}
}

func TestSeveralTargetsInOneFileAreAllFound(t *testing.T) {
	root := tree(t, map[string]string{
		"a__x_test.go": "package a\n\nfunc FuzzOne(f *testing.F) {}\nfunc FuzzTwo(f *testing.F) {}\n",
	})
	if got := names(discover(t, root)); !slices.Equal(got, []string{"FuzzOne", "FuzzTwo"}) {
		t.Errorf("Discover = %v", got)
	}
}

func TestAFileWithNoFuzzContributesNothing(t *testing.T) {
	root := tree(t, map[string]string{"a__x_test.go": "package a\n\nfunc TestThing(t *testing.T) {}\n"})
	if got := discover(t, root); len(got) != 0 {
		t.Errorf("Discover = %v, want none", got)
	}
}

// TestANameMerelyBeginningWithFuzzIsNotATarget is Go's own rule: `func Fuzz` or
// `func FuzzXxx` with Xxx upper-case. The deleted generator's pattern took
// `func Fuzzy`, and would have emitted -fuzz=^Fuzzy$ against a package holding
// no such target.
func TestANameMerelyBeginningWithFuzzIsNotATarget(t *testing.T) {
	root := tree(t, map[string]string{"a__x_test.go": "package a\n\nfunc Fuzzy() {}\n"})
	if got := discover(t, root); len(got) != 0 {
		t.Errorf("Discover took a non-target: %v", got)
	}
}

func TestABareFuncFuzzIsATarget(t *testing.T) {
	root := tree(t, map[string]string{"a__x_test.go": "package a\n\nfunc Fuzz(f *testing.F) {}\n"})
	if got := names(discover(t, root)); !slices.Equal(got, []string{"Fuzz"}) {
		t.Errorf("Discover = %v, want [Fuzz]", got)
	}
}

func TestAnIndentedFuncIsNotATopLevelTarget(t *testing.T) {
	root := tree(t, map[string]string{"a__x_test.go": "package a\n\nfunc T() {\n\tfunc FuzzInner() {}\n}\n"})
	if got := discover(t, root); len(got) != 0 {
		t.Errorf("Discover took a nested func: %v", got)
	}
}

func TestVendorAndTestdataAreNotWalked(t *testing.T) {
	root := tree(t, map[string]string{
		"vendor__dep__x_test.go": "package dep\n\nfunc FuzzVendored(f *testing.F) {}\n",
		"a__testdata__x_test.go": "package a\n\nfunc FuzzFixture(f *testing.F) {}\n",
		"a__x_test.go":           "package a\n\nfunc FuzzReal(f *testing.F) {}\n",
	})
	if got := names(discover(t, root)); !slices.Equal(got, []string{"FuzzReal"}) {
		t.Errorf("Discover = %v, want [FuzzReal]", got)
	}
}

func TestANonTestFileIsNotRead(t *testing.T) {
	root := tree(t, map[string]string{"a__x.go": "package a\n\nfunc FuzzNotATest(f *testing.F) {}\n"})
	if got := discover(t, root); len(got) != 0 {
		t.Errorf("Discover read a non-test file: %v", got)
	}
}

func TestTheOrderIsPackageThenName(t *testing.T) {
	root := tree(t, map[string]string{
		"z__x_test.go": "package z\n\nfunc FuzzB(f *testing.F) {}\n",
		"a__x_test.go": "package a\n\nfunc FuzzZ(f *testing.F) {}\nfunc FuzzA(f *testing.F) {}\n",
	})
	want := []Target{
		{Name: "FuzzA", Package: "./internal/a"},
		{Name: "FuzzZ", Package: "./internal/a"},
		{Name: "FuzzB", Package: "./internal/z"},
	}
	if got := discover(t, root); !slices.Equal(got, want) {
		t.Errorf("Discover = %v, want %v", got, want)
	}
}

func TestATreeWithNoInternalYieldsNothing(t *testing.T) {
	if got := discover(t, t.TempDir()); len(got) != 0 {
		t.Errorf("Discover = %v, want none", got)
	}
}

// TestTheRealTreeMatchesAnIndependentGrep is equality against ground truth
// rather than a threshold. A walk that silently drops one package still passes
// a `more than 50` assertion, which is the failure the equality exists to
// catch.
func TestTheRealTreeMatchesAnIndependentGrep(t *testing.T) {
	root := repoRoot(t)
	discovered := map[string]bool{}
	for _, target := range discoverRealTree(t) {
		discovered[target.Name] = true
	}

	ground := grepFuzzNames(t, root)
	for name := range ground {
		if !discovered[name] {
			t.Errorf("grep found %s and discovery did not", name)
		}
	}
	for name := range discovered {
		if !ground[name] {
			t.Errorf("discovery found %s and grep did not", name)
		}
	}
	// Non-vacuity: an empty set satisfies the equality above if the grep broke
	// in the same direction.
	if len(discovered) <= 50 {
		t.Errorf("discovery found %d targets, which is too few for this tree to be the one under test", len(discovered))
	}
}

// grepFuzzNames repeats the walk with a SECOND definition of the pattern,
// written out here rather than shared with Discover. A second opinion that uses
// the same definition is not independence.
func grepFuzzNames(t *testing.T, root string) map[string]bool {
	t.Helper()
	// Go's rule, restated: `Fuzz` alone, or `Fuzz` followed by a character that
	// is not a lower-case letter.
	pattern := regexp.MustCompile(`(?m)^func (Fuzz(?:[A-Z][A-Za-z0-9_]*)?)[\t ]*\(`)
	found := map[string]bool{}

	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if SkipDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, readErr := os.ReadFile(path) //nolint:gosec // a test reads the checkout it runs in
		if readErr != nil {
			return readErr
		}
		for _, match := range pattern.FindAllStringSubmatch(string(raw), -1) {
			found[match[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return found
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Skipf("no checkout: %v", err)
	}
	return root
}

// realTree walks the checkout once for the package.
// Four cases use the result, and each walk reads every _test.go under internal/.
// Reusing the result keeps the unit suite fast.
var realTree = sync.OnceValues(func() ([]Target, error) {
	root, err := lepath.Root()
	if err != nil {
		return nil, err
	}
	return Discover(root)
})

func discoverRealTree(t *testing.T) []Target {
	t.Helper()
	targets, err := realTree()
	if err != nil {
		t.Fatalf("Discover over the checkout: %v", err)
	}
	return targets
}

// planOf answers what a sweeper would run, failing the test on a walk error.
func planOf(t *testing.T, sweeper *Sweeper) Plan {
	t.Helper()
	plan, err := sweeper.Plan()
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return plan
}

// --- Naming one target passes the caller through ---------------------------

// TestARegexpNameReachesGoUnaltered is the `ze-fuzz-test-one` contract. An
// earlier version of the Python filtered discovery by exact equality on both
// arguments, so the documented PKG=./internal/component/bgp/wireu/... exited 2
// and a Go regexp in FUZZ stopped matching.
func TestARegexpNameReachesGoUnaltered(t *testing.T) {
	plan := planOf(t, &Sweeper{Chain: probeChain(), Name: "FuzzParse.*"})
	if len(plan.Runs) != 1 {
		t.Fatalf("a named run planned %d runs", len(plan.Runs))
	}
	if !slices.Contains(plan.Runs[0].Argv, "-fuzz=FuzzParse.*") {
		t.Errorf("the regexp was rewritten: %v", plan.Runs[0].Argv)
	}
}

func TestAWildcardPackageReachesGoUnaltered(t *testing.T) {
	plan := planOf(t, &Sweeper{Chain: probeChain(), Package: "./internal/component/bgp/wireu/..."})
	if len(plan.Runs) != 1 {
		t.Fatalf("a named package planned %d runs", len(plan.Runs))
	}
	if !slices.Contains(plan.Runs[0].Argv, "./internal/component/bgp/wireu/...") {
		t.Errorf("the wildcard was rewritten: %v", plan.Runs[0].Argv)
	}
}

func TestNamingOneDoesNotConsultDiscovery(t *testing.T) {
	plan := planOf(t, &Sweeper{Chain: probeChain(), Root: t.TempDir(), Name: "FuzzNothingDeclaresThisName"})
	if len(plan.Runs) != 1 {
		t.Fatalf("a name no target declares planned %d runs", len(plan.Runs))
	}
	if !slices.Contains(plan.Runs[0].Argv, "-fuzz=FuzzNothingDeclaresThisName") {
		t.Errorf("discovery narrowed a name the caller had already given: %v", plan.Runs[0].Argv)
	}
}

// TestANamedRunDefaultsToTheLongerBudget keeps the Python's second default: a
// sweep gives each target 10s and a single named target gets 30s.
func TestANamedRunDefaultsToTheLongerBudget(t *testing.T) {
	plan := planOf(t, &Sweeper{Chain: probeChain(), Name: "FuzzX"})
	if plan.FuzzTime != NamedFuzzTime {
		t.Errorf("a named run got %s, want %s", plan.FuzzTime, NamedFuzzTime)
	}
	sweep := planOf(t, &Sweeper{Chain: probeChain(), Root: t.TempDir()})
	if sweep.FuzzTime != DefaultFuzzTime {
		t.Errorf("a sweep got %s, want %s", sweep.FuzzTime, DefaultFuzzTime)
	}
}

// --- The sweep stops at the first failure -----------------------------------

// runSweep drives the sweep over len(codes) targets, each answering its code,
// and reports the exit code and how many actually ran.
func driveSweep(t *testing.T, codes []int) (int, int) {
	t.Helper()
	targets := make([]Target, 0, len(codes))
	for i := range codes {
		var name, pkg strings.Builder
		name.WriteString("Fuzz")
		name.WriteString(string(rune('A' + i)))
		pkg.WriteString("./internal/p")
		pkg.WriteString(string(rune('A' + i)))
		targets = append(targets, Target{Name: name.String(), Package: pkg.String()})
	}

	ran := 0
	sweeper := &Sweeper{
		Chain:    probeChain(),
		FuzzTime: "1s",
		Targets:  targets,
		Progress: &bytes.Buffer{},
		Exec: func([]string) int {
			ran++
			return codes[ran-1]
		},
	}
	report, code := sweeper.Run()
	if len(report.Results) != ran {
		t.Errorf("the report holds %d results and %d targets ran", len(report.Results), ran)
	}
	return code, ran
}

func TestAFailingFuzzerStopsTheSweep(t *testing.T) {
	code, ran := driveSweep(t, []int{0, 3, 0, 0})
	if code != 3 {
		t.Errorf("exit = %d, want 3: the failing target's code must survive", code)
	}
	if ran != 2 {
		t.Errorf("the sweep ran %d of 4 targets, want 2", ran)
	}
}

// TestAnAllGreenSweepRunsEveryTarget is the non-vacuity twin: the stop above is
// a stop, not a sweep that never ran.
func TestAnAllGreenSweepRunsEveryTarget(t *testing.T) {
	code, ran := driveSweep(t, []int{0, 0, 0, 0})
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if ran != 4 {
		t.Errorf("the sweep ran %d of 4 targets", ran)
	}
}

// TestASweepWithNoTargetIsAFailure keeps the Python's answer: a tree holding no
// `func Fuzz` is a broken checkout rather than a clean run.
func TestASweepWithNoTargetIsAFailure(t *testing.T) {
	sweeper := &Sweeper{Chain: probeChain(), Root: t.TempDir(), Progress: &bytes.Buffer{}}
	if _, code := sweeper.Run(); code != 1 {
		t.Errorf("an empty sweep exited %d, want 1", code)
	}
}

// --- The rendering ----------------------------------------------------------

func TestThePlanListingNamesEveryTargetAndTheBudget(t *testing.T) {
	plan := Plan{
		FuzzTime: "10s",
		Runs: []Run{
			{Name: "FuzzA", Package: "./internal/a"},
			{Name: "FuzzB", Package: "./internal/b"},
		},
	}
	text := plan.Text()
	for _, want := range []string{"FuzzA", "./internal/a", "FuzzB", "2 fuzz target(s), 10s each"} {
		if !strings.Contains(text, want) {
			t.Errorf("the listing does not name %q:\n%s", want, text)
		}
	}
}

func TestANamedPlanPrintsTheArgv(t *testing.T) {
	plan := Plan{Named: true, FuzzTime: "30s", Runs: []Run{{Argv: []string{"go", "test", "-fuzz=FuzzX"}}}}
	if got := plan.Text(); !strings.Contains(got, "go test -fuzz=FuzzX") {
		t.Errorf("a named plan rendered %q, want the argv", got)
	}
}
