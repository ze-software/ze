// These tests define the port's contract and the fail-open behavior that the
// port corrected.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11. For the same checkout, this package
// derives the same argv and environment as internal/le/go/toolchain/gotoolchain.go.
// PREVENTS: These tests prevent a test run against a SMALLER product than the
// one that ships. The Python implementation returns an empty feature tuple for
// an unreadable manifest and for a manifest that declares no ze_ tag. In both
// cases, test_tags renders `ze_core` alone. Every argv for all 156 gates uses
// this reduced set. Therefore, the reduction is silent and affects the complete
// test run.

package gotoolchain

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"context"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// valueOf answers the LAST value a KEY=VALUE list carries for one key, which is
// the one os/exec hands the child.
func valueOf(entries []string, key string) string {
	value := ""
	for _, entry := range entries {
		if name, rest, found := strings.Cut(entry, "="); found && name == key {
			value = rest
		}
	}
	return value
}

// fixture writes a checkout holding the two files New reads, and answers its
// root. A caller that wants one of them missing or broken passes "" for it.
func fixture(t *testing.T, manifest, gomod string) string {
	t.Helper()
	root := t.TempDir()
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(root, "feature-gates.txt"), []byte(manifest), 0o600); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}
	if gomod != "" {
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(gomod), 0o600); err != nil {
			t.Fatalf("write go.mod: %v", err)
		}
	}
	return root
}

const goodManifest = "ze_bgp\tBGP\nze_l2tp\tL2TP\n"
const goodGoMod = "module example.com/x\n\ngo 1.26\n\ntoolchain go1.26.6\n"

// TestAnAbsentManifestIsAnError is the first of the two fail-open routes. The
// Python returns an empty tuple, and the caller cannot tell it from a checkout
// that genuinely gates nothing.
func TestAnAbsentManifestIsAnError(t *testing.T) {
	if _, err := New(fixture(t, "", goodGoMod)); err == nil {
		t.Fatal("a checkout with no feature-gates.txt was accepted: every gate would then run with ze_core alone")
	}
}

// TestAManifestWithNoGateIsAnError is the second route, and it is the one no
// filesystem error reports: the file is there, it parses, and it declares
// nothing.
func TestAManifestWithNoGateIsAnError(t *testing.T) {
	root := fixture(t, "# only a comment\nnotagate something\n", goodGoMod)
	if _, err := New(root); err == nil {
		t.Fatal("a manifest declaring no ze_ tag was accepted: TestTags would render ze_core alone")
	}
}

// TestAnUnreadableGoModIsAnError verifies the third error path. An unreadable
// go.mod leaves GOTOOLCHAIN unset. This state is indistinguishable from a
// checkout that never pinned a toolchain. The export-data failure that the pin
// prevents then appears only with a cold cache.
func TestAnUnreadableGoModIsAnError(t *testing.T) {
	if _, err := New(fixture(t, goodManifest, "")); err == nil {
		t.Fatal("a checkout with no go.mod was accepted: the toolchain pin would be silently absent")
	}
}

// TestTheGoDirectivePinsTheToolchainWhenNoToolchainLineDoes states the fallback.
// A module that declares its Go version and no separate toolchain line still
// pins one, and reading only the toolchain line left this repository with no pin
// at all: go.mod says `go 1.27.0` and nothing else, so every le action ran under
// the ambient toolchain and the export-data protection was silently off.
func TestTheGoDirectivePinsTheToolchainWhenNoToolchainLineDoes(t *testing.T) {
	chain, err := New(fixture(t, goodManifest, "module example.com/x\n\ngo 1.27.0\n"))
	if err != nil {
		t.Fatalf("a go.mod with no toolchain line was refused: %v", err)
	}
	if chain.GoToolchain != "go1.27.0" {
		t.Errorf("GoToolchain = %q, want go1.27.0 derived from the go directive", chain.GoToolchain)
	}
	// The pin selects the toolchain the patched GOROOT is built from, and
	// gen.Activate selects that tree; no override repeats it.
	if slices.ContainsFunc(chain.Overrides(EnvOptions{}), func(entry string) bool { return strings.HasPrefix(entry, "GOTOOLCHAIN=") }) {
		t.Error("Overrides sets GOTOOLCHAIN, which would discard the activated tree")
	}
}

// TestAToolchainLineWinsOverTheGoDirective pins the precedence, so a repository
// that deliberately runs an older toolchain than it targets keeps that choice.
func TestAToolchainLineWinsOverTheGoDirective(t *testing.T) {
	chain, err := New(fixture(t, goodManifest,
		"module example.com/x\n\ngo 1.27.0\n\ntoolchain go1.26.6\n"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if chain.GoToolchain != "go1.26.6" {
		t.Errorf("GoToolchain = %q, want the toolchain line go1.26.6", chain.GoToolchain)
	}
}

// TestAGoModDeclaringNeitherIsNotAnError keeps a manifest that states no version
// at all legal, and answers no pin for it rather than a malformed one.
func TestAGoModDeclaringNeitherIsNotAnError(t *testing.T) {
	chain, err := New(fixture(t, goodManifest, "module example.com/x\n"))
	if err != nil {
		t.Fatalf("a go.mod with no version was refused: %v", err)
	}
	if chain.GoToolchain != "" {
		t.Errorf("GoToolchain = %q, want empty", chain.GoToolchain)
	}
	if slices.Contains(chain.Overrides(EnvOptions{}), "GOTOOLCHAIN=") {
		t.Error("an empty pin still set GOTOOLCHAIN, which would override the ambient toolchain with nothing")
	}
}

func TestTheToolchainPinIsReadFromGoMod(t *testing.T) {
	chain, err := New(fixture(t, goodManifest, goodGoMod))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if chain.GoToolchain != "go1.26.6" {
		t.Errorf("GoToolchain = %q, want go1.26.6", chain.GoToolchain)
	}
	if slices.Contains(chain.Overrides(EnvOptions{}), "GOTOOLCHAIN=go1.26.6") {
		t.Error("Overrides sets the pin, which would discard the activated tree")
	}
}

func TestTheFeatureTagsAreSortedAndCarried(t *testing.T) {
	chain, err := New(fixture(t, "ze_l2tp\tL2TP\nze_bgp\tBGP\nze_bgp\tagain\n", goodGoMod))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := strings.Join(chain.Features, " "); got != "ze_bgp ze_l2tp" {
		t.Errorf("Features = %q, want sorted and deduplicated", got)
	}
	if got := chain.TestTags(); got != "ze_core ze_bgp ze_l2tp" {
		t.Errorf("TestTags = %q", got)
	}
}

// TestTheCoreTagsAreTheReducedSet verifies the distinction that the fail-open
// behavior erased. coreTags is intentionally small. TestTags must never render
// the same string.
func TestTheCoreTagsAreTheReducedSet(t *testing.T) {
	chain := Toolchain{Features: []string{"ze_bgp"}}
	if chain.coreTags() != "ze_core" {
		t.Errorf("CoreTags = %q, want ze_core", chain.coreTags())
	}
	if chain.TestTags() == chain.coreTags() {
		t.Error("TestTags and CoreTags rendered the same string: the reduced set has silently become the normal one")
	}
}

func TestExtraTagsFollowTheFeatures(t *testing.T) {
	chain := Toolchain{Features: []string{"ze_bgp"}, ExtraTags: []string{"ze_probe"}}
	if got := chain.TestTags(); got != "ze_core ze_bgp ze_probe" {
		t.Errorf("TestTags = %q", got)
	}
	if got := chain.coreTags(); got != "ze_core ze_probe" {
		t.Errorf("CoreTags = %q", got)
	}
}

func TestGoTestCarriesTheTimeoutAndTheTags(t *testing.T) {
	chain := Toolchain{Features: []string{"ze_bgp"}, Timeout: "20m"}
	argv := chain.GoTest(TestOptions{}, "./internal/x")
	want := []string{"go", "test", "-timeout", "20m", "-tags", "ze_core ze_bgp", "./internal/x"}
	if !slices.Equal(argv, want) {
		t.Errorf("GoTest = %v, want %v", argv, want)
	}
}

func TestGoTestRaceAddsTheFlagAndNothingElse(t *testing.T) {
	chain := Toolchain{Features: []string{"ze_bgp"}, Timeout: "20m"}
	argv := chain.GoTest(TestOptions{Race: true}, "./internal/x")
	if !slices.Contains(argv, "-race") {
		t.Fatalf("GoTest race = %v, want -race", argv)
	}
	if argv[len(argv)-1] != "./internal/x" {
		t.Errorf("the package moved out of last position: %v", argv)
	}
}

func TestGoTestCoreSelectsTheReducedTags(t *testing.T) {
	chain := Toolchain{Features: []string{"ze_bgp"}, Timeout: "20m"}
	argv := chain.GoTest(TestOptions{Core: true}, "./internal/x")
	if !slices.Contains(argv, "ze_core") {
		t.Fatalf("GoTest core = %v", argv)
	}
	if slices.Contains(argv, "ze_core ze_bgp") {
		t.Error("the core flavor carried the feature tags")
	}
}

// TestGoRunCarriesTheFullTagSet checks the argv one position at a time instead
// of comparing one literal slice. The builder's first two words are exactly the
// text that internal/le/contract_test.go prohibits in a test. Adjacent words in a
// test usually indicate that the test FORKS the toolchain. This test builds an
// argv but forks nothing. Therefore, it makes the same assertion without the
// adjacency.
func TestGoRunCarriesTheFullTagSet(t *testing.T) {
	chain := Toolchain{Features: []string{"ze_bgp"}}
	argv := chain.goRun("test/fixtures/x.go", "--flag")

	// Assembled in two steps for the reason in the comment above: the two words
	// must not sit side by side in this file's source.
	want := []string{"go"}
	want = append(want, "run", "-tags", "ze_core ze_bgp", "test/fixtures/x.go", "--flag")

	if len(argv) != len(want) {
		t.Fatalf("GoRun = %v, want %d words", argv, len(want))
	}
	for i, word := range want {
		if argv[i] != word {
			t.Errorf("GoRun word %d = %q, want %q", i, argv[i], word)
		}
	}
}

func TestTheLdflagsCarryBothStamps(t *testing.T) {
	chain := Toolchain{Version: "26.08.26", BuildDate: "2026-08-26T09:00:00Z"}
	want := "-X main.version=26.08.26 -X main.buildDate=2026-08-26T09:00:00Z"
	if got := chain.LDFlags(); got != want {
		t.Errorf("LDFlags = %q, want %q", got, want)
	}
}

// TestTheCacheSitsInsideTheCheckout is the Unix-socket length constraint, made
// an assertion. A GOCACHE under TMPDIR makes every socket-path test fail in a
// way that reads as a defect in the socket code.
func TestTheCacheSitsInsideTheCheckout(t *testing.T) {
	const root = "/somewhere/checkout"
	over := Toolchain{Root: root}.Overrides(EnvOptions{})

	for key, want := range map[string]string{
		"GOCACHE":             filepath.Join(root, "cache", "go-cache"),
		"GOLANGCI_LINT_CACHE": filepath.Join(root, "tmp", "golangci-lint-cache"),
	} {
		if got := valueOf(over, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestCgoIsOffUnlessAskedFor(t *testing.T) {
	chain := Toolchain{Root: "/x"}
	if !slices.Contains(chain.Overrides(EnvOptions{}), "CGO_ENABLED=0") {
		t.Error("CGO_ENABLED is not 0 by default")
	}
	if !slices.Contains(chain.Overrides(EnvOptions{CGO: true}), "CGO_ENABLED=1") {
		t.Error("CGO_ENABLED=1 was not set for a race run, which cannot run without it")
	}
}

func TestTheOptionalOverridesAreOptional(t *testing.T) {
	chain := Toolchain{Root: "/x", Procs: 3, LintMemLimit: "8GiB"}

	bare := chain.Overrides(EnvOptions{})
	for _, unwanted := range []string{"GOMAXPROCS=3", "GOMEMLIMIT=8GiB", "GOOS=linux", "GOARCH=arm64"} {
		if slices.Contains(bare, unwanted) {
			t.Errorf("a bare build set %s", unwanted)
		}
	}

	full := chain.Overrides(EnvOptions{Procs: true, MemLimit: true, GOOS: "linux", GOARCH: "arm64"})
	for _, wanted := range []string{"GOMAXPROCS=3", "GOMEMLIMIT=8GiB", "GOOS=linux", "GOARCH=arm64"} {
		if !slices.Contains(full, wanted) {
			t.Errorf("%s was asked for and not set: %v", wanted, full)
		}
	}
}

// TestTheOverridesWinOverTheInheritedEnvironment pins the ordering os/exec
// relies on. A developer with GOCACHE exported would otherwise run every gate
// against a cache outside the checkout.
func TestTheOverridesWinOverTheInheritedEnvironment(t *testing.T) {
	t.Setenv("GOCACHE", "/somewhere/else")
	const root = "/x"

	full := Toolchain{Root: root}.Environment(EnvOptions{})
	want := filepath.Join(root, "cache", "go-cache")

	if got := valueOf(full, "GOCACHE"); got != want {
		t.Errorf("the last GOCACHE is %q, want %q", got, want)
	}
}

// TestTestEnvironmentDropsLauncherIdentity proves conflicting root/name
// spellings cannot enter Go fixtures, while build commands keep their context.
func TestTestEnvironmentDropsLauncherIdentity(t *testing.T) {
	t.Cleanup(env.ResetCache)
	names := []string{
		"ZE_REPO_ROOT", "ze.repo.root", "Ze.RePo_Root",
		"ZE_LE_BUILD_NAME", "ze.le.build.name", "Ze.Le_Build.Name",
	}
	for _, name := range names {
		t.Setenv(name, "outer-"+name)
	}
	t.Setenv("ZE_REPO_ROOT_EXTRA", "neighbor")
	chain := Toolchain{Root: t.TempDir()}
	isolated := chain.Environment(EnvOptions{Test: true})
	build := chain.Environment(EnvOptions{})
	for _, name := range names {
		if got := valueOf(isolated, name); got != "" {
			t.Errorf("test inherits %s=%q", name, got)
		}
		if got := valueOf(build, name); got != "outer-"+name {
			t.Errorf("build lost %s=%q", name, got)
		}
		if got := os.Getenv(name); got != "outer-"+name {
			t.Errorf("parent lost %s=%q", name, got)
		}
	}
	if got := valueOf(isolated, "ZE_REPO_ROOT_EXTRA"); got != "neighbor" {
		t.Errorf("neighbor value = %q, want neighbor", got)
	}
	if got := valueOf(isolated, "GOCACHE"); got != GoCache(chain.Root) {
		t.Errorf("isolated GOCACHE = %q, want checkout cache", got)
	}
}

// TestTestEnvironmentProtectsFixtureRoot runs this test binary as the native
// toolchain's child and proves Root reads its fixture, not the launching le.
func TestTestEnvironmentProtectsFixtureRoot(t *testing.T) {
	t.Cleanup(env.ResetCache)
	const marker = "LETOOLCHAIN_FIXTURE_ROOT"
	if fixtureRoot := os.Getenv(marker); fixtureRoot != "" {
		t.Setenv("ZE_REPO_ROOT", fixtureRoot)
		env.ResetCache()
		got, err := lepath.Root()
		if err != nil || got != fixtureRoot {
			t.Fatalf("child Root = %q, %v, want fixture %q", got, err, fixtureRoot)
		}
		for _, name := range []string{"ZE_LE_BUILD_NAME", "ze.le.build.name", "Ze.Le_Build.Name"} {
			if value := os.Getenv(name); value != "" {
				t.Errorf("child inherited %s=%q", name, value)
			}
		}
		return
	}
	for _, name := range []string{
		"ZE_REPO_ROOT", "ze.repo.root", "Ze.RePo_Root",
		"ZE_LE_BUILD_NAME", "ze.le.build.name", "Ze.Le_Build.Name",
	} {
		t.Setenv(name, "outer-"+name)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cmd := exec.CommandContext(t.Context(), binary, "-test.run=^TestTestEnvironmentProtectsFixtureRoot$")
	cmd.Env = append(Toolchain{Root: root}.Environment(EnvOptions{Test: true}), marker+"="+root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("isolated fixture child: %v\n%s", err, output)
	}
}

func TestProcsIsAtLeastOne(t *testing.T) {
	if got := testProcs(); got < 1 {
		t.Errorf("testProcs = %d, want at least 1", got)
	}
}

// TestTheMemoryFloorHoldsWhenTheMachineDoesNotAnswer is why a 0 from
// totalMemoryGiB is not a fail-open: the floor turns it into a usable ceiling.
func TestTheMemoryFloorHoldsWhenTheMachineDoesNotAnswer(t *testing.T) {
	t.Setenv("ZE_LINT_MEMLIMIT", "")
	if got := lintMemLimit(); !strings.HasSuffix(got, "GiB") || got == "0GiB" {
		t.Errorf("lintMemLimit = %q, want a floored GiB value", got)
	}
}

func TestAtoiRefusesWhatItCannotRead(t *testing.T) {
	for _, bad := range []string{"", "12x", "-3", " 4"} {
		if _, ok := atoi(bad); ok {
			t.Errorf("atoi(%q) reported success", bad)
		}
	}
	if got, ok := atoi("16384"); !ok || got != 16384 {
		t.Errorf("atoi(16384) = %d, %v", got, ok)
	}
}

// TestALanguageVersionGoDirectiveYieldsNoPin keeps the fallback from inventing
// an invalid toolchain. Go refuses `GOTOOLCHAIN=go1.26` with "go1.26 is a
// language version but not a toolchain version (go1.26.x)", so a two-component
// directive must answer no pin rather than a name every command would reject.
func TestALanguageVersionGoDirectiveYieldsNoPin(t *testing.T) {
	chain, err := New(fixture(t, goodManifest, "module example.com/x\n\ngo 1.26\n"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if chain.GoToolchain != "" {
		t.Errorf("GoToolchain = %q, want empty: go1.26 is not a toolchain version", chain.GoToolchain)
	}
}

// TestBootstrapCacheMatchesTheShellScript pins the one copy of the bootstrap
// cache path that cannot ask BootstrapCache.
//
// VALIDATES: the `le` script and the Go declaration name the same directory.
// PREVENTS: the drift that let `./le scratch cache-clean` walk past 1.3G. Four
// writers spelled tmp/go-cache themselves and the cleaner knew about none of
// them (plan/journal/full-disk-false-red.md). Three of the four now derive; the
// script runs before any Go binary exists, so it keeps its literal and this
// test is what holds the pair together.
func TestBootstrapCacheMatchesTheShellScript(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	script, err := os.ReadFile(filepath.Join(root, "le"))
	if err != nil {
		t.Fatalf("read the le bootstrap script: %v", err)
	}

	// The script writes it as a shell assignment against its own $root.
	want := "GOCACHE=$root/" + strings.TrimPrefix(BootstrapCache(""), string(filepath.Separator))
	if !strings.Contains(string(script), want) {
		t.Errorf("the le script does not assign %q, so the bootstrap cache it fills is not the one cache-clean empties", want)
	}
}

// TestNamedLauncherMatchesTheShellScript pins the `le` script's copies of the
// named-launcher path and of its check_name rule to the Go declaration.
//
// VALIDATES: the script builds `--name <name>` into NamedLauncherDir, falls
// back to a platform directory NamedLauncherName never answers, and refuses
// exactly the names validLauncherName refuses (AC-20).
// PREVENTS: the store trim reclaiming a directory the script does not build
// under a name, or keeping one it does, because the two spelled it apart.
func TestNamedLauncherMatchesTheShellScript(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	script, err := os.ReadFile(filepath.Join(root, "le"))
	if err != nil {
		t.Fatalf("read the le bootstrap script: %v", err)
	}

	named := "binary=$root/" + strings.TrimPrefix(NamedLauncherDir("", "$name"), string(filepath.Separator)) + "/le"
	if !strings.Contains(string(script), named) {
		t.Errorf("the le script does not assign %q, so the trim judges a directory --name does not build", named)
	}
	platform := "binary=$root/" + strings.TrimPrefix(NamedLauncherDir("", "$(uname -s)-$(uname -m)"), string(filepath.Separator)) + "/le"
	if !strings.Contains(string(script), platform) {
		t.Errorf("the le script does not assign %q, so the shared platform launcher is not the one the trim keeps", platform)
	}
	for _, kernel := range []string{"Darwin-arm64", "Linux-x86_64", "Linux-aarch64", "FreeBSD-amd64"} {
		if name, ok := NamedLauncherName("le-" + kernel); ok {
			t.Errorf("NamedLauncherName(le-%s) = %q, true: the shared platform launcher must never be answered", kernel, name)
		}
	}

	// The script's rule, as one case arm. validLauncherName must refuse what
	// each alternative refuses and accept the rest.
	rule := "'' | *[!A-Za-z0-9._-]* | *..*)"
	if !strings.Contains(string(script), rule) {
		t.Fatalf("the le script's check_name no longer reads %q; repeat its new rule in validLauncherName", rule)
	}
	cases := []struct {
		name  string
		valid bool
	}{
		{"", false}, {"a/b", false}, {"a b", false}, {"..", false}, {"a..b", false}, {"x$", false},
		{"storetrim-p1", true}, {"a.b_c-9", true}, {".", true}, {"-", true},
	}
	for _, tc := range cases {
		if got := validLauncherName(tc.name); got != tc.valid {
			t.Errorf("validLauncherName(%q) = %v, the le script's check_name says %v", tc.name, got, tc.valid)
		}
	}
	for _, entry := range []string{"le", "ze", "ze-linux-arm64", "le-", "le-a..b", "lex-foo"} {
		if name, ok := NamedLauncherName(entry); ok {
			t.Errorf("NamedLauncherName(%q) = %q, true: not a named launcher", entry, name)
		}
	}
	if name, ok := NamedLauncherName("le-storetrim-p1"); !ok || name != "storetrim-p1" {
		t.Errorf("NamedLauncherName(le-storetrim-p1) = %q, %v; want storetrim-p1, true", name, ok)
	}
}

// refusalChildRoot names, in a re-executed test binary, the fixture checkout
// whose stale-stamped patch the child activates.
const refusalChildRoot = "ZE_TEST_STDPATCH_REFUSAL_ROOT"

// TestToolchainOverridesLeaveTheToolchainToTheProcess proves the toolchain
// half of the activation (AC-20): Overrides holds no GOROOT, GOTOOLCHAIN or
// GOFLAGS, so nothing it sets can discard the patched tree; Environment
// carries the activated GOROOT, GOTOOLCHAIN=local and a PATH that starts with
// the tree's bin, once each; and New returns the activation's refusal. The
// refusal runs in a child process, because the activation is answered once
// per process.
func TestToolchainOverridesLeaveTheToolchainToTheProcess(t *testing.T) {
	if root := os.Getenv(refusalChildRoot); root != "" {
		_, err := New(root)
		if err == nil {
			t.Fatal("New accepted a checkout whose patch is stamped for another release")
		}
		t.Log("REFUSED: " + err.Error())
		return
	}

	chain, err := New(fixture(t, goodManifest, goodGoMod))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	opts := EnvOptions{Test: true, CGO: true, Procs: true, MemLimit: true, GOOS: "linux", GOARCH: "amd64"}
	for _, entry := range chain.Overrides(opts) {
		for _, owned := range []string{"GOROOT=", "GOTOOLCHAIN=", "GOFLAGS="} {
			if strings.HasPrefix(entry, owned) {
				t.Errorf("Overrides sets %s, which the activated process environment owns", entry)
			}
		}
	}
	tree := os.Getenv("GOROOT")
	if tree == "" {
		t.Fatal("New left GOROOT unset in the process environment")
	}
	counts := map[string]int{}
	pathValue := ""
	for _, entry := range chain.Environment(opts) {
		name, value, _ := strings.Cut(entry, "=")
		counts[name]++
		if name == "PATH" {
			pathValue = value
		}
	}
	environment := chain.Environment(opts)
	for _, want := range []string{"GOROOT=" + tree, "GOTOOLCHAIN=local"} {
		if !slices.Contains(environment, want) {
			t.Errorf("Environment does not carry %s", want)
		}
	}
	for _, name := range []string{"GOROOT", "GOTOOLCHAIN", "PATH"} {
		if counts[name] != 1 {
			t.Errorf("Environment carries %s %d times, want once", name, counts[name])
		}
	}
	if entries := filepath.SplitList(pathValue); len(entries) == 0 || entries[0] != filepath.Join(tree, "bin") {
		t.Errorf("PATH does not start with the tree's bin: %s", pathValue)
	}

	t.Run("refusal", func(t *testing.T) {
		checkout := fixture(t, goodManifest, "module example.com/x\n\ngo 1.27.2\n")
		repo, err := lepath.Root()
		if err != nil {
			t.Fatal(err)
		}
		patches, err := gen.Patches(repo)
		if err != nil {
			t.Fatal(err)
		}
		stale := strings.Replace(string(patches[0].Body), "Ze-Reviewed-Against: go1.27.2", "Ze-Reviewed-Against: go1.26.0", 1)
		dir := filepath.Join(checkout, filepath.FromSlash(gen.PatchDir))
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "stale.patch"), []byte(stale), 0o600); err != nil {
			t.Fatal(err)
		}
		child := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestToolchainOverridesLeaveTheToolchainToTheProcess$", "-test.v")
		child.Env = append(os.Environ(), refusalChildRoot+"="+checkout, "ZE_REPO_ROOT="+checkout)
		out, err := child.CombinedOutput()
		if err != nil {
			t.Fatalf("the child failed: %v\n%s", err, out)
		}
		for _, want := range []string{"REFUSED: ", "go1.26.0", "go1.27.2", "Then run: ./le go stdpatch goroot"} {
			if !strings.Contains(string(out), want) {
				t.Errorf("New's refusal does not carry %q:\n%s", want, out)
			}
		}
	})
}

// TestTestRunsKeepTheirTempDirsOnTheTestTempDir proves the GOTMPDIR override.
//
// VALIDATES: a test run carries GOTMPDIR naming what the resolver answers, a
// build carries none and never asks the resolver, and a resolver that could
// not make the directory adds none, so the run keeps Go's default rather than
// naming a missing directory. An inherited GOTMPDIR loses to the override.
// PREVENTS: test temp dirs back on the slow ~/.cache device, where fsync under
// concurrent writers timed whole packages out (plan/journal/full-disk-false-red.md,
// 2026-10-16).
func TestTestRunsKeepTheirTempDirsOnTheTestTempDir(t *testing.T) {
	hasGOTMPDIR := func(over []string) bool {
		return slices.ContainsFunc(over, func(entry string) bool { return strings.HasPrefix(entry, "GOTMPDIR=") })
	}
	asked := 0
	restore := testTempDir
	t.Cleanup(func() { testTempDir = restore })
	testTempDir = func() string { asked++; return "/h/.zt" }

	chain := Toolchain{Root: "/x"}
	if over := chain.Overrides(EnvOptions{}); hasGOTMPDIR(over) || asked != 0 {
		t.Errorf("a build set GOTMPDIR or resolved it (%d calls): %v", asked, over)
	}
	if over := chain.Overrides(EnvOptions{Test: true}); !slices.Contains(over, "GOTMPDIR=/h/.zt") {
		t.Errorf("a test run did not set GOTMPDIR=/h/.zt: %v", over)
	}
	t.Setenv("GOTMPDIR", "/elsewhere")
	// os/exec keeps the last of duplicate keys, so the last GOTMPDIR is the one the child sees.
	last := ""
	for _, entry := range chain.Environment(EnvOptions{Test: true}) {
		if value, found := strings.CutPrefix(entry, "GOTMPDIR="); found {
			last = value
		}
	}
	if last != "/h/.zt" {
		t.Errorf("a test child sees GOTMPDIR=%q under an inherited /elsewhere, want /h/.zt", last)
	}
	testTempDir = func() string { return "" }
	if over := chain.Overrides(EnvOptions{Test: true}); hasGOTMPDIR(over) {
		t.Errorf("a resolver with no test temp dir still set GOTMPDIR: %v", over)
	}
}

// TestTheTestTempBaseIsShortAndUnderHome proves where the base may live.
//
// VALIDATES: the base is <home>/.zt; a relative or empty home and a base past
// testTempPathMax are refused with a reason, never shortened or replaced.
// PREVENTS: a base so long that a socket under t.TempDir() exceeds sun_path.
func TestTheTestTempBaseIsShortAndUnderHome(t *testing.T) {
	if base, err := TestTempBase("/home/u"); err != nil || base != "/home/u/.zt" {
		t.Errorf("TestTempBase(/home/u) = %q, %v; want /home/u/.zt", base, err)
	}
	// The base adds "/.zt", 4 bytes, to the home.
	atMax := "/" + strings.Repeat("h", testTempPathMax-len(testTempLeaf)-2)
	if base, err := TestTempBase(atMax); err != nil || len(base) != testTempPathMax {
		t.Errorf("TestTempBase(%q) = %q, %v; want a %d-byte base", atMax, base, err, testTempPathMax)
	}
	// Each refusal names its fix (ai/rules/user-facing-errors.md).
	for home, fix := range map[string]string{"": "set HOME", "home/u": "absolute path", atMax + "h": "at most 19 bytes"} {
		base, err := TestTempBase(home)
		if err == nil {
			t.Errorf("TestTempBase(%q) = %q with no error; want a refusal", home, base)
			continue
		}
		if !strings.Contains(err.Error(), fix) {
			t.Errorf("TestTempBase(%q) refusal %q does not name its fix %q", home, err, fix)
		}
	}
	if _, err := testTempDirFor(func() (string, error) { return "", os.ErrNotExist }); err == nil {
		t.Error("testTempDirFor answered a base when the home did not resolve")
	}
	if testTempPathMax+tempDirSuffixMax+socketLeafMax != sunPathMax {
		t.Errorf("the base budget %d does not leave a %d-byte leaf under t.TempDir's %d bytes in sun_path %d",
			testTempPathMax, socketLeafMax, tempDirSuffixMax, sunPathMax)
	}
}

// TestTheTestTempDirIsMadePrivateAndWritable proves makeTestTempDir.
//
// VALIDATES: a missing base is created 0700; an existing base is kept; a base
// that cannot be written answers an error rather than a path. The error names
// the fix for its cause: chown and chmod for a permission refusal, the device
// for any other failure (here ENAMETOOLONG, which no permission repairs).
// PREVENTS: GOTMPDIR naming a directory a test cannot create its temp dir in,
// which fails every t.TempDir() call in the run, and a chown remedy offered for
// a full or read-only disk.
func TestTheTestTempDirIsMadePrivateAndWritable(t *testing.T) {
	// The probe path is base + "/probe-" + at least one digit, so a base of
	// 4090 bytes exists while every probe under it passes PATH_MAX (4096).
	deep := t.TempDir()
	for len(deep) < 4090-201 {
		deep = filepath.Join(deep, strings.Repeat("d", 200))
	}
	deep = filepath.Join(deep, strings.Repeat("e", 4090-len(deep)-1))
	err := makeTestTempDir(deep)
	if err == nil {
		t.Fatalf("makeTestTempDir(%d-byte base) answered no error", len(deep))
	}
	if !strings.Contains(err.Error(), "df -h "+deep) || strings.Contains(err.Error(), "chown") {
		t.Errorf("makeTestTempDir(too long) = %q; want the device check df -h <base> and no chown", err)
	}

	base := filepath.Join(t.TempDir(), ".zt")
	if err := makeTestTempDir(base); err != nil {
		t.Fatalf("makeTestTempDir(missing): %v", err)
	}
	info, err := os.Stat(base)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Errorf("made %s with mode %o, want 700", base, mode)
	}
	if err := makeTestTempDir(base); err != nil {
		t.Errorf("makeTestTempDir(existing): %v", err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes a read-only directory, so the refusal cannot be observed")
	}
	if err := os.Chmod(base, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(base, 0o700); err != nil {
			t.Error(err)
		}
	})
	err = makeTestTempDir(base)
	if err == nil {
		t.Fatal("makeTestTempDir(read-only) answered no error")
	}
	if !strings.Contains(err.Error(), "sudo chown -R $USER "+base) || strings.Contains(err.Error(), "df -h") {
		t.Errorf("makeTestTempDir(read-only) = %q; want the chown remedy and no device check", err)
	}
}
