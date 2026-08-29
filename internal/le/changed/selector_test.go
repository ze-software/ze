// Goal: prove the native change-set selector answers with the packages to
// retest and the feature tags the change can reach, and that it over-selects
// rather than under-selects whenever it cannot classify its input.
// Method: drive Scope.Resolve directly against this repository and against a
// fixture module whose graph, manifest, and depth boundaries are controlled.
//
// Design: docs/architecture/testing/verify-freshness-scope.md -- the scoping model
// Detail: selector.go -- the selector these tests drive

package changed

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// scopeSelectorBudget is AC-6: the selector must answer in under 30 seconds on
// the current tree.
const scopeSelectorBudget = 30 * time.Second

// toolingReaders is the answer for a path read by the common repository tools.
var toolingReaders = []string{"./" + docValidationPackage, "./" + verifyRunnerPackage, "./" + checksPackage}

// scopeSelectorAction names the native action these legacy fixture cases drive.
func scopeSelectorAction(t *testing.T) string {
	t.Helper()
	return "le changed scope"
}

// runScopeSelector drives Scope.Resolve directly and captures its compatibility
// text and diagnostics. No deleted-script subprocess sits between the action and
// the selector.
func runScopeSelector(t *testing.T, _, dir string, args ...string) (string, string, int) {
	t.Helper()
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "off")

	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("capture selector diagnostics: %v", err)
	}
	original := os.Stderr
	os.Stderr = writeEnd
	defer func() { os.Stderr = original }()
	report, code := (Scope{Root: dir}).Resolve(args)
	os.Stderr = original
	if err := writeEnd.Close(); err != nil {
		t.Fatalf("close selector diagnostics: %v", err)
	}
	stderr, err := io.ReadAll(readEnd)
	if err != nil {
		t.Fatalf("read selector diagnostics: %v", err)
	}
	if err := readEnd.Close(); err != nil {
		t.Fatalf("close captured selector diagnostics: %v", err)
	}
	return report.Text(), string(stderr), code
}

// scopeLines splits selector output into its non-empty lines.
func scopeLines(out string) []string {
	var lines []string
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// scopePathsFile writes the changed-path list the selector reads with
// --paths-from, one path per line, and returns its name.
func scopePathsFile(t *testing.T, paths ...string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "changed-paths.txt")
	body := strings.Join(paths, "\n") + "\n"
	if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
		t.Fatalf("write the changed-path list: %v", err)
	}
	return name
}

// writeScopeFixture builds a module whose reverse-dependency depths are known:
// core <- mid <- high <- top is a chain of three importers, and hub imports ssh
// only from a file gated by //go:build ze_ssh. The manifest gates ssh and bgp,
// so the tag answer is a pure function of the manifest.
func writeScopeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":            "module example.com/fixture\n\ngo 1.26\n",
		"feature-gates.txt": "# fixture manifest\nze_ssh  ssh\nze_bgp  bgp\n",
		"core/core.go":      "package core\n\n// Value is the leaf of the fixture chain.\nfunc Value() int { return 1 }\n",
		"mid/mid.go": "package mid\n\nimport \"example.com/fixture/core\"\n\n" +
			"// Value reads the leaf.\nfunc Value() int { return core.Value() }\n",
		"high/high.go": "package high\n\nimport \"example.com/fixture/mid\"\n\n" +
			"// Value reads the middle.\nfunc Value() int { return mid.Value() }\n",
		"top/top.go": "package top\n\nimport \"example.com/fixture/high\"\n\n" +
			"// Value reads the high package.\nfunc Value() int { return high.Value() }\n",
		"ssh/ssh.go": "package ssh\n\n// Listen is the gated feature.\nfunc Listen() int { return 2 }\n",
		"bgp/bgp.go": "package bgp\n\n// Speak is the other gated feature.\nfunc Speak() int { return 3 }\n",
		// The one file whose constraint NEGATES a feature: no build with ze_ssh
		// on compiles it, so the only build that can see a change to it is one
		// that dropped ze_ssh.
		"bgp/plain.go": "//go:build ze_bgp && !ze_ssh\n\npackage bgp\n\n" +
			"// SpeakPlain is compiled only where the ssh feature is off.\nfunc SpeakPlain() int { return 5 }\n",
		"hub/hub.go": "package hub\n\n// Start runs the always-on hub.\nfunc Start() int { return 0 }\n",
		// The one file here requires a tag the selector never passes, so the
		// directory builds no package under the full tag set.
		"tools/tools.go": "//go:build ze_never_passed\n\npackage tools\n\n" +
			"// Value belongs to no build.\nfunc Value() int { return 4 }\n",
		"hub/hub_ssh.go": "//go:build ze_ssh\n\npackage hub\n\nimport \"example.com/fixture/ssh\"\n\n" +
			"// StartSSH is compiled only when the ssh feature is on.\nfunc StartSSH() int { return ssh.Listen() }\n",
	}
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("create the fixture directory for %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write the fixture file %s: %v", name, err)
		}
	}
	return root
}

func TestSelectorRefusesAMissingOrMalformedManifestWithExitTwo(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
	}{
		{name: "missing"},
		{name: "malformed", manifest: "ze_ssh\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "go.mod", "module example.com/fixture\n\ngo 1.26\n")
			if test.manifest != "" {
				writeFile(t, root, featureManifestPath, test.manifest)
			}
			paths := scopePathsFile(t, "core/core.go")

			stdout, stderr, code := runScopeSelector(
				t, scopeSelectorAction(t), root, "--paths-from="+paths,
			)
			if code != 2 {
				t.Fatalf("selector exited %d, want 2\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			if stdout != "" {
				t.Errorf("a manifest refusal wrote stdout %q", stdout)
			}
			if !strings.Contains(stderr, featureManifestPath) {
				t.Errorf("the refusal does not name the manifest:\n%s", stderr)
			}
		})
	}
}

// TestSelectorEmitsPackagesAndTags is the wiring test: the entry point runs and
// the two answers come back separable from one another.
func TestSelectorEmitsPackagesAndTags(t *testing.T) {
	binary := scopeSelectorAction(t)
	paths := scopePathsFile(t, "internal/component/ssh/ssh.go")

	stdout, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--print=both", "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	packages, tags, err := splitScopeSections(stdout)
	if err != nil {
		t.Fatalf("%v\nstdout:\n%s", err, stdout)
	}
	if len(packages) == 0 {
		t.Fatalf("the package answer is empty, which no input may produce\nstdout:\n%s", stdout)
	}
	if len(tags) == 0 {
		t.Fatalf("the feature-tag answer is empty\nstdout:\n%s", stdout)
	}
	if !slices.Contains(packages, "./internal/component/ssh") {
		t.Fatalf("the changed package itself is missing from the package answer: %v", packages)
	}
}

// splitScopeSections reads the two-section form --print=both writes.
func splitScopeSections(stdout string) (packages, tags []string, err error) {
	section := ""
	for _, line := range scopeLines(stdout) {
		switch line {
		case "# packages":
			section = "packages"
			continue
		case "# tags":
			section = "tags"
			continue
		}
		switch section {
		case "packages":
			packages = append(packages, line)
		case "tags":
			tags = append(tags, line)
		default:
			return nil, nil, errors.New("output before the first section header")
		}
	}
	if section == "" {
		return nil, nil, errors.New("no section header in the output")
	}
	return packages, tags, nil
}

// TestSelectorSeesGatedImporters is AC-1: cmd/ze/hub imports
// internal/component/ssh only from files carrying //go:build ze_ssh, so an
// untagged import graph reports zero importers and the selector must not.
func TestSelectorSeesGatedImporters(t *testing.T) {
	binary := scopeSelectorAction(t)
	paths := scopePathsFile(t, "internal/component/ssh/ssh.go")

	stdout, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	packages := scopeLines(stdout)
	if !slices.Contains(packages, "./cmd/ze/hub") {
		t.Fatalf("./cmd/ze/hub is missing: the reverse graph is not tag-aware\npackages: %v", packages)
	}
}

// TestSelectorTagAnswerNamesTheReachedFeature is AC-3: a change confined to a
// gated package reaches that feature and no other.
func TestSelectorTagAnswerNamesTheReachedFeature(t *testing.T) {
	binary := scopeSelectorAction(t)
	paths := scopePathsFile(t, "internal/component/ssh/ssh.go")

	stdout, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--print=tags", "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	tags := scopeLines(stdout)
	if !slices.Contains(tags, "ze_ssh") {
		t.Fatalf("ze_ssh is missing from the feature-tag answer: %v", tags)
	}
	if slices.Contains(tags, "ze_bgp") {
		t.Fatalf("ze_bgp is named by an ssh-only change: %v", tags)
	}
}

// TestSelectorMapsNativeRFCAndCorpusPaths covers both the native RFC producer
// and the corpus that the repository tooling tests read.
func TestSelectorMapsNativeRFCAndCorpusPaths(t *testing.T) {
	cases := []struct {
		changed string
		want    string
	}{
		{changed: "internal/le/rfc/actions.go", want: "./internal/le/rfc"},
		{changed: "rfc/enrolled.txt", want: "./" + rfcPackage},
	}
	for _, test := range cases {
		t.Run(test.changed, func(t *testing.T) {
			binary := scopeSelectorAction(t)
			paths := scopePathsFile(t, test.changed)

			stdout, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--paths-from="+paths)
			if code != 0 {
				t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
			}
			packages := scopeLines(stdout)
			if !slices.Contains(packages, test.want) {
				t.Fatalf("%s is missing for %s: %v", test.want, test.changed, packages)
			}
		})
	}
}

// TestSelectorBoundsCoreFanOut is AC-2: a core change reaches much of the
// repository, and the bounded answer must remain materially smaller while
// naming what it dropped.
func TestSelectorBoundsCoreFanOut(t *testing.T) {
	binary := scopeSelectorAction(t)
	paths := scopePathsFile(t, "internal/core/env/env.go")

	stdout, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	packages := scopeLines(stdout)
	if len(packages) < 2 {
		t.Fatalf("a core change selected %d packages, which under-selects: %v", len(packages), packages)
	}
	fullStdout, fullStderr, fullCode := runScopeSelector(t, binary, repoRoot(t), "--depth=1000", "--paths-from="+paths)
	if fullCode != 0 {
		t.Fatalf("full selector exited %d\nstderr:\n%s", fullCode, fullStderr)
	}
	fullPackages := scopeLines(fullStdout)
	if len(packages)*4 >= len(fullPackages)*3 {
		t.Fatalf("bounded core change selected %d of %d packages, want less than three quarters", len(packages), len(fullPackages))
	}
	if !strings.Contains(stderr, "beyond depth") {
		t.Fatalf("the selector did not report what the depth bound dropped\nstderr:\n%s", stderr)
	}
}

// TestSelectorRunsUnderBudget is AC-6.
func TestSelectorRunsUnderBudget(t *testing.T) {
	binary := scopeSelectorAction(t)
	paths := scopePathsFile(t, "internal/core/env/env.go")

	start := time.Now()
	_, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--print=both", "--paths-from="+paths)
	elapsed := time.Since(start)
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	t.Logf("selector runtime: %s", elapsed)
	if elapsed > scopeSelectorBudget {
		t.Fatalf("the selector took %s, over the %s budget", elapsed, scopeSelectorBudget)
	}
}

// TestSelectorWidensWhenTheModuleGraphMoves is AC-4: the three inputs that
// change what every Go package compiles to still select everything, say which
// path did it, and exit 0 so the run continues. This is the fail-open branch
// after the unclassified kinds stopped reaching it.
func TestSelectorWidensWhenTheModuleGraphMoves(t *testing.T) {
	binary := scopeSelectorAction(t)
	root := writeScopeFixture(t)
	for _, changed := range []string{"go.mod", "go.sum", "vendor/example.com/dep/dep.go"} {
		t.Run(changed, func(t *testing.T) {
			paths := scopePathsFile(t, "core/core.go", changed)

			stdout, stderr, code := runScopeSelector(t, binary, root, "--print=both", "--paths-from="+paths)
			if code != 0 {
				t.Fatalf("a moved dependency must not stop the run, got exit %d\nstderr:\n%s", code, stderr)
			}
			packages, tags, err := splitScopeSections(stdout)
			if err != nil {
				t.Fatalf("%v\nstdout:\n%s", err, stdout)
			}
			if !slices.Contains(packages, "./...") {
				t.Fatalf("%s must select every package, got %v", changed, packages)
			}
			if !slices.Contains(tags, "ze_ssh") || !slices.Contains(tags, "ze_bgp") {
				t.Fatalf("%s must select every feature tag, got %v", changed, tags)
			}
			if !strings.Contains(stderr, changed+" changed, so a dependency moved") {
				t.Fatalf("the selector did not name %s as the path that widened the run\nstderr:\n%s", changed, stderr)
			}
		})
	}
}

// TestSelectorNamesAKindNoRuleNames is AC-9: a path no rule names is NAMED on
// stderr and seeds the packages that could read it, never the whole tree. Each
// path here sits one step from a rule -- the right directory with the wrong
// suffix, or the right suffix in the wrong directory -- so it proves the rules
// match a PAIR and that a near miss is still classified as unknown.
//
// Naming is the load-bearing half. The residual risk is a non-Go file read by a
// Go test that sits neither beside it nor in a tooling package, and this line is
// the only evidence a reader would have that a rule is missing.
func TestSelectorNamesAKindNoRuleNames(t *testing.T) {
	binary := scopeSelectorAction(t)
	for _, testcase := range []struct {
		changed string
		want    []string
	}{
		{changed: "docs/architecture/diagram.svg", want: toolingReaders},
		{changed: ".claude/hooks/README.md", want: toolingReaders},
		{changed: "demos/terminal/rpki/run.sh", want: toolingReaders},
		{changed: "test/interop/scenarios/eor/frr.conf", want: toolingReaders},
		{changed: ".gitignore", want: toolingReaders},
		{changed: "test/health/latest.json", want: toolingReaders},
	} {
		t.Run(testcase.changed, func(t *testing.T) {
			paths := scopePathsFile(t, testcase.changed)

			stdout, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--paths-from="+paths)
			if code != 0 {
				t.Fatalf("an unclassified path must not stop the run, got exit %d\nstderr:\n%s", code, stderr)
			}
			packages := scopeLines(stdout)
			if slices.Contains(packages, "./...") {
				t.Fatalf("%s widened the whole run instead of naming its readers\nstderr:\n%s", testcase.changed, stderr)
			}
			for _, reader := range testcase.want {
				if !slices.Contains(packages, reader) {
					t.Fatalf("%s did not select %s, got %v", testcase.changed, reader, packages)
				}
			}
			if !strings.Contains(stderr, "no rule names "+testcase.changed+", so it seeds ") {
				t.Fatalf("the selector did not name %s as a path no rule covers\nstderr:\n%s", testcase.changed, stderr)
			}
		})
	}
}

// TestSelectorSeedsThePackageAnUnclassifiedPathSitsIn is the second rule of the
// unclassified answer: a file in a directory that holds Go source is a fixture
// of that package, read by the tests beside it.
//
// The fixture module is what discriminates. It has no development-tool
// packages, so removing this rule sends core/fixture.json to packages that do
// not exist here, and the answer widens to ./... instead of naming the chain.
func TestSelectorSeedsThePackageAnUnclassifiedPathSitsIn(t *testing.T) {
	binary := scopeSelectorAction(t)
	root := writeScopeFixture(t)
	paths := scopePathsFile(t, "core/fixture.json")

	stdout, stderr, code := runScopeSelector(t, binary, root, "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	packages := scopeLines(stdout)
	if slices.Contains(packages, "./...") {
		t.Fatalf("a fixture beside a package widened the whole run\nstderr:\n%s", stderr)
	}
	if !slices.Equal(packages, []string{"./core", "./high", "./mid"}) {
		t.Fatalf("core/fixture.json must seed ./core and its two levels of importers, got %v", packages)
	}
	if !strings.Contains(stderr, "no rule names core/fixture.json, so it seeds ./core") {
		t.Fatalf("the selector did not name the path or the package it seeded\nstderr:\n%s", stderr)
	}
}

// TestSelectorMapsTheExternalPluginExample covers the one directory whose Go
// files belong to another module: examples/plugin/go is module
// example/acme-monitor, so go list ./... never reports it and seeding it would
// widen the run. Nothing in this module compiles or reads that module either,
// so the honest answer is no package at all, the same answer a .ci body gets.
// The rule is matched BEFORE the .go branch that would otherwise claim main.go.
func TestSelectorMapsTheExternalPluginExample(t *testing.T) {
	binary := scopeSelectorAction(t)
	for _, changed := range []string{"examples/plugin/go/main.go", "examples/plugin/go/go.mod"} {
		t.Run(changed, func(t *testing.T) {
			paths := scopePathsFile(t, changed)

			stdout, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--paths-from="+paths)
			if code != 0 {
				t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
			}
			packages := scopeLines(stdout)
			// Deleting the rule sends main.go down the .go branch, which seeds a
			// directory go list never reports, and the selector fails open.
			if slices.Contains(packages, "./...") {
				t.Fatalf("%s widened the whole run\nstderr:\n%s", changed, stderr)
			}
			// Seeding a package would be a mapping that looks like coverage and
			// is not: no test in it can fail when the example breaks.
			if len(packages) != 0 {
				t.Fatalf("%s must seed no package, got %v", changed, packages)
			}
			if !strings.Contains(stderr, "no changed path is compiled or read by a Go package") {
				t.Fatalf("the selector selected no package without saying so\nstderr:\n%s", stderr)
			}
		})
	}
}

// TestSelectorMapsToolingInputKinds is AC-8: the kinds a real session dirties
// each seed the tooling packages that READ them, never the whole tree. Deleting
// any row from nonGoPathRules turns that row's case back into a widen, which is
// what the ./... assertion catches.
func TestSelectorMapsToolingInputKinds(t *testing.T) {
	binary := scopeSelectorAction(t)
	for _, testcase := range []struct {
		changed string
		want    []string
	}{
		{changed: ".claude/settings.json", want: []string{"./" + hookRuntimePackage}},
		{changed: "ai/rules/testing.md", want: []string{"./" + rulesPackage, "./" + docCheckPackage}},
		{changed: "plan/journal/tooling.md", want: []string{"./" + specPackage, "./" + docCheckPackage}},
		{changed: "docs/guide/firewall.md", want: []string{"./" + docValidationPackage, "./" + docCheckPackage}},
		{changed: ".github/workflows/verify.yml", want: []string{"./" + workflowPackage}},
	} {
		t.Run(testcase.changed, func(t *testing.T) {
			paths := scopePathsFile(t, testcase.changed)

			stdout, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--paths-from="+paths)
			if code != 0 {
				t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
			}
			packages := scopeLines(stdout)
			if slices.Contains(packages, "./...") {
				t.Fatalf("%s widened the whole run instead of naming its readers\nstderr:\n%s", testcase.changed, stderr)
			}
			for _, reader := range testcase.want {
				if !slices.Contains(packages, reader) {
					t.Fatalf("%s did not select %s, got %v", testcase.changed, reader, packages)
				}
			}
		})
	}
}

// TestSelectorMapsTheFunctionalCorpusToItsWalkers is the other half of AC-8 and
// the regression guard for the silent fail-CLOSED this rule once carried.
//
// Native Go test packages parse the committed functional corpora.
//
// Reverting the row to an empty dirs list makes the non-empty assertion below
// fail on every case, which is the mutation this test exists to catch.
func TestSelectorMapsTheFunctionalCorpusToItsWalkers(t *testing.T) {
	binary := scopeSelectorAction(t)
	for _, testcase := range []struct {
		changed string
		want    []string
	}{
		{
			changed: "test/runner/bgp-open.ci",
			want:    []string{"./internal/test/runner", "./" + docValidationPackage, "./" + checksPackage},
		},
		{
			changed: "test/editor/config-commit.et",
			want:    []string{"./internal/component/cli/testing"},
		},
		{
			changed: "test/web/dashboard.wb",
			want:    []string{"./internal/component/web/testing"},
		},
	} {
		t.Run(testcase.changed, func(t *testing.T) {
			paths := scopePathsFile(t, testcase.changed)

			stdout, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--paths-from="+paths)
			if code != 0 {
				t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
			}
			packages := scopeLines(stdout)
			if len(packages) == 0 {
				t.Fatalf("%s selected no package at all, so the scoped run would test nothing and exit 0\nstderr:\n%s",
					testcase.changed, stderr)
			}
			if slices.Contains(packages, "./...") {
				t.Fatalf("%s widened the whole run instead of naming its walkers\nstderr:\n%s", testcase.changed, stderr)
			}
			for _, walker := range testcase.want {
				if !slices.Contains(packages, walker) {
					t.Fatalf("%s did not select %s, got %v", testcase.changed, walker, packages)
				}
			}
		})
	}
}

// TestSelectorMapsGoTreesTheUnitBuildNeverCompiles covers the tracked Go
// directories go list ./... does not report under the unit tag set. Widening for
// one of them buys nothing, because ./... does not compile it either: the wide
// answer runs the same checks over it as the narrow one and pays for the whole
// tree to do it.
//
// cmd/ze-installer is deliberately NOT in this table any more, and
// TestSelectorWidensForTheInstallerInitrd is where it went: since 2026-08-24 the
// lint over ./... reaches it under a ze_installer flavor, so the wide answer
// stopped running the same checks as the narrow one.
//
// Deleting a branch from uncompiledTreeReaders sends its case back through the
// unruled seed path in seedPackages, which answers ./... -- the assertion below.
func TestSelectorMapsGoTreesTheUnitBuildNeverCompiles(t *testing.T) {
	binary := scopeSelectorAction(t)
	modcache := "gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/empty/empty.go"
	for _, testcase := range []struct {
		changed string
		want    []string
	}{
		// The module root holds tools.go alone, gated //go:build tools. No lint
		// flavor selects it either.
		{changed: "tools.go", want: []string{"./" + checksPackage}},
		// A third-party module cache every tree walker names in a skip list.
		{changed: modcache, want: nil},
	} {
		t.Run(testcase.changed, func(t *testing.T) {
			paths := scopePathsFile(t, testcase.changed)

			stdout, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--paths-from="+paths)
			if code != 0 {
				t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
			}
			packages := scopeLines(stdout)
			if slices.Contains(packages, "./...") {
				t.Fatalf("%s widened the whole run, which compiles it no more than the narrow answer does\nstderr:\n%s",
					testcase.changed, stderr)
			}
			for _, reader := range testcase.want {
				if !slices.Contains(packages, reader) {
					t.Fatalf("%s did not select %s, got %v", testcase.changed, reader, packages)
				}
			}
			if testcase.want == nil {
				if len(packages) != 0 {
					t.Fatalf("%s is read by no Go test, so it seeds no package, got %v", testcase.changed, packages)
				}
				if !strings.Contains(stderr, "no changed path is compiled or read by a Go package") {
					t.Fatalf("the selector selected no package without saying so\nstderr:\n%s", stderr)
				}
			}
		})
	}
}

// TestSelectorWidensForTheInstallerInitrd pins the one directory whose narrow
// answer reports on nothing.
//
// VALIDATES: an edit to cmd/ze-installer answers ./..., so ze-lint-changed runs
// the flavor driver over the whole tree and the installer flavor selects that
// package.
// PREVENTS: the regression this test was written for. Every file there is
// //go:build linux && ze_installer, so go list under the unit tag set reports
// "build constraints exclude all Go files" and the package is in no import
// graph. uncompiledTreeReaders used to answer with the tree-walking packages
// instead. The changed-scope linter then inspected only development tooling.
// The initrd's PID 1 partitions and writes a disk, and it passed the changed-file
// gate with no lint pass over a line of it. The gate exited 0.
//
// The wide answer is what the selector is FOR here, not a fallback it fell into.
// ./cmd/ze-installer cannot be named in the narrow answer instead: the same list
// drives ze-unit-test-changed, and `go test ./cmd/ze-installer` under the unit
// tag set fails with "build constraints exclude all Go files [setup failed]".
func TestSelectorWidensForTheInstallerInitrd(t *testing.T) {
	binary := scopeSelectorAction(t)
	paths := scopePathsFile(t, "cmd/ze-installer/main.go")

	stdout, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	if got := scopeLines(stdout); !slices.Equal(got, []string{"./..."}) {
		t.Fatalf("an installer edit answered %v, want ./...: no narrower answer names a "+
			"package any lint pass loads the installer under\nstderr:\n%s", got, stderr)
	}
	if !strings.Contains(stderr, "cmd/ze-installer") {
		t.Fatalf("the selector widened without naming the directory that widened it\nstderr:\n%s", stderr)
	}
}

// TestSelectorScopesARealisticDirtyTree is AC-8 end to end: the whole kind list
// a session dirties in one change set answers with the readers of those kinds
// and nothing else. Before the classification table this set answered ./...,
// which is no scoping at all, and the last four paths are why the table alone
// was not enough: each is a kind no rule names, and one of them widened the
// whole answer again on the live tree.
func TestSelectorScopesARealisticDirtyTree(t *testing.T) {
	binary := scopeSelectorAction(t)
	paths := scopePathsFile(t,
		"plan/spec-verify-scope-2-change-set-selector.md",
		"ai/rules/testing.md",
		"docs/functional-tests.md",
		"test/runner/bgp-open.ci",
		".github/workflows/verify.yml",
		"internal/le/commit/actions.go",
		".claude/plan/ze-plan-config-completion",
		"test/health/latest.json",
		"demos/terminal/zefs-config/demo.tape",
		"ze.audit.jsonl",
	)

	stdout, stderr, code := runScopeSelector(t, binary, repoRoot(t), "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	packages := scopeLines(stdout)
	if slices.Contains(packages, "./...") {
		t.Fatalf("a realistic dirty tree widened the whole run\nstderr:\n%s", stderr)
	}
	for _, reader := range []string{"./" + rulesPackage, "./" + specPackage, "./" + docValidationPackage, "./" + docCheckPackage} {
		if !slices.Contains(packages, reader) {
			t.Fatalf("%s is missing from the answer, got %v", reader, packages)
		}
	}
}

// TestSelectorReadsManifestAtRunTime is AC-5: the tag answer follows an edit to
// feature-gates.txt with no second file to change.
func TestSelectorReadsManifestAtRunTime(t *testing.T) {
	binary := scopeSelectorAction(t)
	root := writeScopeFixture(t)
	paths := scopePathsFile(t, "bgp/bgp.go")

	stdout, stderr, code := runScopeSelector(t, binary, root, "--print=tags", "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	if got := scopeLines(stdout); !slices.Equal(got, []string{"ze_bgp"}) {
		t.Fatalf("before the manifest edit the tag answer is %v, want [ze_bgp]", got)
	}

	manifest := filepath.Join(root, "feature-gates.txt")
	if err := os.WriteFile(manifest, []byte("ze_ssh  ssh\nze_renamed  bgp\n"), 0o600); err != nil {
		t.Fatalf("rewrite the fixture manifest: %v", err)
	}

	stdout, stderr, code = runScopeSelector(t, binary, root, "--print=tags", "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d after the manifest edit\nstderr:\n%s", code, stderr)
	}
	if got := scopeLines(stdout); !slices.Equal(got, []string{"ze_renamed"}) {
		t.Fatalf("after the manifest edit the tag answer is %v, want [ze_renamed]", got)
	}
}

// TestSelectorTagAnswerHoldsTheFeaturesAChangedFileNegates
//
// VALIDATES: a changed file constrained !ze_T puts ze_T in the tag answer, and a
// changed Go file the selector cannot read widens to every feature.
// PREVENTS: the row filter subtracting the ONLY build that compiles the changed
// file. bgp/plain.go reads `ze_bgp && !ze_ssh`, so without_ze_ssh is the one
// matrix row that compiles it: an answer of ze_bgp alone would keep
// all_features, core_only and without_ze_bgp, and none of the three would type
// check the file that changed.
func TestSelectorTagAnswerHoldsTheFeaturesAChangedFileNegates(t *testing.T) {
	binary := scopeSelectorAction(t)
	root := writeScopeFixture(t)

	t.Run("a negated feature joins the gating one", func(t *testing.T) {
		paths := scopePathsFile(t, "bgp/plain.go")

		stdout, stderr, code := runScopeSelector(t, binary, root, "--print=tags", "--paths-from="+paths)
		if code != 0 {
			t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
		}
		if got := scopeLines(stdout); !slices.Equal(got, []string{"ze_bgp", "ze_ssh"}) {
			t.Fatalf("the tag answer is %v, want [ze_bgp ze_ssh]: without_ze_ssh is the only build that compiles the changed file", got)
		}
	})

	t.Run("a changed Go file that cannot be read widens", func(t *testing.T) {
		paths := scopePathsFile(t, "ssh/deleted.go")

		stdout, stderr, code := runScopeSelector(t, binary, root, "--print=tags", "--paths-from="+paths)
		if code != 0 {
			t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
		}
		if got := scopeLines(stdout); !slices.Equal(got, []string{"ze_bgp", "ze_ssh"}) {
			t.Fatalf("a file whose constraint could not be read answered %v, want every feature", got)
		}
		if !strings.Contains(stderr, "build constraint of ssh/deleted.go could not be read") {
			t.Fatalf("the selector widened without saying why\nstderr:\n%s", stderr)
		}
	})
}

// TestSelectorSeesGatedImportersInFixture proves the gated edge is what carries
// hub into the answer: hub imports ssh only from its //go:build ze_ssh file.
func TestSelectorSeesGatedImportersInFixture(t *testing.T) {
	binary := scopeSelectorAction(t)
	root := writeScopeFixture(t)
	paths := scopePathsFile(t, "ssh/ssh.go")

	stdout, stderr, code := runScopeSelector(t, binary, root, "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	if got := scopeLines(stdout); !slices.Contains(got, "./hub") {
		t.Fatalf("./hub is missing, so the gated import edge was invisible: %v", got)
	}
}

// TestSelectorFailsOpenWhenTheSeedIsNoPackage closes the zero-value trap: a
// changed directory that go list does not report has no importers to find, and
// the narrow answer would be an empty list nobody can tell from "nothing
// changed". cmd/ze-installer is such a directory in this repository, and
// TestSelectorWidensForTheInstallerInitrd is that case on the live tree.
func TestSelectorFailsOpenWhenTheSeedIsNoPackage(t *testing.T) {
	binary := scopeSelectorAction(t)
	root := writeScopeFixture(t)
	paths := scopePathsFile(t, "tools/tools.go")

	stdout, stderr, code := runScopeSelector(t, binary, root, "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	if got := scopeLines(stdout); !slices.Equal(got, []string{"./..."}) {
		t.Fatalf("a seed that builds no package answered %v, want the wide answer", got)
	}
	if !strings.Contains(stderr, "tools") {
		t.Fatalf("the selector did not name the directory that widened the run\nstderr:\n%s", stderr)
	}
}

// TestSelectorRefusesDepthZero is the low boundary: depth 0 retests only the
// edited package and misses every importer, so the selector refuses it rather
// than answering with a set nobody asked for.
func TestSelectorRefusesDepthZero(t *testing.T) {
	binary := scopeSelectorAction(t)
	root := writeScopeFixture(t)
	paths := scopePathsFile(t, "core/core.go")

	stdout, stderr, code := runScopeSelector(t, binary, root, "--depth=0", "--paths-from="+paths)
	if code == 0 {
		t.Fatalf("depth 0 was accepted\nstdout:\n%s", stdout)
	}
	if !strings.Contains(stderr, "--depth") {
		t.Fatalf("the refusal does not name the flag it refused\nstderr:\n%s", stderr)
	}
}

// TestSelectorDepthOneOnlyByOverride pins the chosen bound: the default answer
// carries the second-level importer, and depth 1 is reachable only by asking.
func TestSelectorDepthOneOnlyByOverride(t *testing.T) {
	binary := scopeSelectorAction(t)
	root := writeScopeFixture(t)
	paths := scopePathsFile(t, "core/core.go")

	stdout, stderr, code := runScopeSelector(t, binary, root, "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	if got := scopeLines(stdout); !slices.Equal(got, []string{"./core", "./high", "./mid"}) {
		t.Fatalf("the default depth answered %v, want the depth-2 set [./core ./high ./mid]", got)
	}

	stdout, stderr, code = runScopeSelector(t, binary, root, "--depth=1", "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d for depth 1\nstderr:\n%s", code, stderr)
	}
	if got := scopeLines(stdout); !slices.Equal(got, []string{"./core", "./mid"}) {
		t.Fatalf("depth 1 answered %v, want [./core ./mid]", got)
	}
}

// TestSelectorDepthThreeMatchesClosure is the high boundary: on a chain three
// importers deep, depth 3 is the whole reverse closure, so nothing above it can
// add a package.
func TestSelectorDepthThreeMatchesClosure(t *testing.T) {
	binary := scopeSelectorAction(t)
	root := writeScopeFixture(t)
	paths := scopePathsFile(t, "core/core.go")

	deep, stderr, code := runScopeSelector(t, binary, root, "--depth=3", "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d for depth 3\nstderr:\n%s", code, stderr)
	}
	want := []string{"./core", "./high", "./mid", "./top"}
	if got := scopeLines(deep); !slices.Equal(got, want) {
		t.Fatalf("depth 3 answered %v, want the closure %v", got, want)
	}

	deeper, stderr, code := runScopeSelector(t, binary, root, "--depth=9", "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("selector exited %d for depth 9\nstderr:\n%s", code, stderr)
	}
	if got := scopeLines(deeper); !slices.Equal(got, want) {
		t.Fatalf("depth 9 answered %v, want the same closure %v", got, want)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, featureManifestPath)); err != nil {
		t.Fatalf("repository root %s has no feature manifest: %v", root, err)
	}
	return root
}
