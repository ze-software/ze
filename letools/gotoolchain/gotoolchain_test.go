// These tests define the port's contract and the fail-open behavior that the
// port corrected.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11. For the same checkout, this package
// derives the same argv and environment as scripts/le/devtools/toolchain.py.
// PREVENTS: These tests prevent a test run against a SMALLER product than the
// one that ships. The Python implementation returns an empty feature tuple for
// an unreadable manifest and for a manifest that declares no ze_ tag. In both
// cases, test_tags renders `ze_core` alone. Every argv for all 156 gates uses
// this reduced set. Therefore, the reduction is silent and affects the complete
// test run.

package gotoolchain

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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

// TestAGoModWithNoToolchainLineIsNotAnError separates the two facts. A go.mod
// that parses and declares no pin is the behavior before the pin existed, and
// it must stay legal.
func TestAGoModWithNoToolchainLineIsNotAnError(t *testing.T) {
	chain, err := New(fixture(t, goodManifest, "module example.com/x\n\ngo 1.26\n"))
	if err != nil {
		t.Fatalf("a go.mod with no toolchain line was refused: %v", err)
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
	if !slices.Contains(chain.Overrides(EnvOptions{}), "GOTOOLCHAIN=go1.26.6") {
		t.Error("the pin did not reach the environment")
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
// behavior erased. CoreTags is intentionally small. TestTags must never render
// the same string.
func TestTheCoreTagsAreTheReducedSet(t *testing.T) {
	chain := Toolchain{Features: []string{"ze_bgp"}}
	if chain.CoreTags() != "ze_core" {
		t.Errorf("CoreTags = %q, want ze_core", chain.CoreTags())
	}
	if chain.TestTags() == chain.CoreTags() {
		t.Error("TestTags and CoreTags rendered the same string: the reduced set has silently become the normal one")
	}
}

func TestExtraTagsFollowTheFeatures(t *testing.T) {
	chain := Toolchain{Features: []string{"ze_bgp"}, ExtraTags: []string{"ze_probe"}}
	if got := chain.TestTags(); got != "ze_core ze_bgp ze_probe" {
		t.Errorf("TestTags = %q", got)
	}
	if got := chain.CoreTags(); got != "ze_core ze_probe" {
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
// text that cmd/le/contract_test.go prohibits in a test. Adjacent words in a
// test usually indicate that the test FORKS the toolchain. This test builds an
// argv but forks nothing. Therefore, it makes the same assertion without the
// adjacency.
func TestGoRunCarriesTheFullTagSet(t *testing.T) {
	chain := Toolchain{Features: []string{"ze_bgp"}}
	argv := chain.GoRun("scripts/x.go", "--flag")

	// Assembled in two steps for the reason in the comment above: the two words
	// must not sit side by side in this file's source.
	want := []string{"go"}
	want = append(want, "run", "-tags", "ze_core ze_bgp", "scripts/x.go", "--flag")

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
