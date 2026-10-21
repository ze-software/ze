// Related: gotoolchain.go -- Environment, the environment the verify gate's unit stages run under
//
// VALIDATES: which environment reads the Go test result cache keys on, under
// the gate's own Toolchain.Environment (spec-verify-gate-warm-timed-first-red,
// assumption A-7). cmd/go hashes only the variables a test logged through
// internal/testlog (computeTestInputsID, cmd/go/internal/test/test.go, case
// "getenv"), and os.Getenv and os.LookupEnv log (testlog.Getenv in os/env.go)
// while os.Environ does not.
// PREVENTS: a claim that a cached pass is safe when the variable a test reads
// changed between two verify runs, and a gate that changes one itself: two
// verify worktrees of one commit must hand a test child the same environment,
// so the cached pass the second run reuses is the verdict a rerun would give.

package gotoolchain

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// cacheEnvProbe is the variable the probe tests read, changed between runs.
const cacheEnvProbe = "ZE_TESTCACHE_ENV_PROBE"

// cacheEnvModule holds two packages that fail when cacheEnvProbe is "red":
// one reads it through os.Getenv, one finds it by scanning os.Environ.
var cacheEnvModule = map[string]string{
	"go.mod": "module example.com/cacheenv\n\ngo 1.22\n",
	"getenv/getenv_test.go": "package getenv\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n" +
		"func TestProbe(t *testing.T) {\n\tif os.Getenv(\"" + cacheEnvProbe + "\") == \"red\" {\n\t\tt.Fatal(\"probe is red\")\n\t}\n}\n",
	"environ/environ_test.go": "package environ\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n" +
		"func TestProbe(t *testing.T) {\n\tfor _, entry := range os.Environ() {\n" +
		"\t\tif entry == \"" + cacheEnvProbe + "=red\" {\n\t\t\tt.Fatal(\"probe is red\")\n\t\t}\n\t}\n}\n",
}

// TestTheTestCacheSeesGetenvButNotEnviron runs each probe package twice under
// Toolchain.Environment with a private cache, the probe "green" then "red".
// The os.Getenv package is rerun and fails, because its read is a cache input.
// The os.Environ package answers its first pass `(cached)`, because nothing
// told cmd/go it read the variable: a cached pass can hide a red that only the
// second environment shows.
func TestTheTestCacheSeesGetenvButNotEnviron(t *testing.T) {
	if _, err := exec.LookPath(goProgram); err != nil {
		t.Skipf("no %s executable on PATH (%v): this test runs the real go command", goProgram, err)
	}
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "")
	goenv := filepath.Join(t.TempDir(), "goenv")
	if err := os.WriteFile(goenv, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", goenv)

	root := t.TempDir()
	for name, content := range cacheEnvModule {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, probe := range []struct {
		pkg        string
		secondRuns bool
	}{
		{"./getenv", true},
		{"./environ", false},
	} {
		t.Setenv(cacheEnvProbe, "green")
		if output, failed := runCacheEnvTest(t, root, probe.pkg); failed || strings.Contains(output, "(cached)") {
			t.Fatalf("%s first run, probe green, over an empty cache: failed=%v\n%s", probe.pkg, failed, output)
		}
		t.Setenv(cacheEnvProbe, "red")
		output, failed := runCacheEnvTest(t, root, probe.pkg)
		cached := strings.Contains(output, "(cached)")
		if probe.secondRuns && (cached || !failed) {
			t.Errorf("%s: an os.Getenv read is a cache input, so the red probe must rerun and fail; failed=%v\n%s",
				probe.pkg, failed, output)
		}
		if !probe.secondRuns && (!cached || failed) {
			t.Errorf("%s: an os.Environ read is not a cache input, so the first pass is answered (cached); failed=%v\n%s",
				probe.pkg, failed, output)
		}
	}
}

// worktreeMarker names the second worktree's directory. The probe below fails
// when any variable it can see names it, which is how a run-varying entry
// would reach a test.
const worktreeMarker = "wt-second"

// worktreeProbeModule holds one package that scans os.Environ, as
// internal/core/env does, and fails when an entry names worktreeMarker. PWD is
// skipped: cmd/go sets it to the package directory for the test binary
// (base.AppendPWD in cmd/go/internal/test/test.go), and os.Getwd and
// os.Getenv log their read of it, so it is a cache input.
var worktreeProbeModule = map[string]string{
	"go.mod": "module example.com/worktreeenv\n\ngo 1.22\n",
	"environ/environ_test.go": "package environ\n\nimport (\n\t\"os\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
		"func TestProbe(t *testing.T) {\n\tfor _, entry := range os.Environ() {\n" +
		"\t\tif !strings.HasPrefix(entry, \"PWD=\") && strings.Contains(entry, \"" + worktreeMarker + "\") {\n" +
		"\t\t\tt.Fatal(\"the environment names this run's worktree: \" + entry)\n\t\t}\n\t}\n}\n",
}

// TestTwoWorktreesGiveATestChildOneEnvironment builds two verify worktrees the
// way the gate lays them out (internal/le/verify, lifecycle.go): cache/ a link
// to one shared cache, tmp/golangci-lint-cache a link to the checkout's, and
// the variables the gate names to its stages pointing into each worktree. It
// asserts Toolchain.Environment answers both the same test environment, then
// runs a package that scans os.Environ first in one worktree and then in the
// other: the second answer is `(cached)`, and a -count=1 rerun there agrees
// with it. A cache path spelled through the worktree turns the rerun red while
// the cache still answers the first pass.
func TestTwoWorktreesGiveATestChildOneEnvironment(t *testing.T) {
	if _, err := exec.LookPath(goProgram); err != nil {
		t.Fatalf("no %s executable on PATH (%v): this test runs the real go command, so put Go's bin directory on PATH", goProgram, err)
	}
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "")
	base := t.TempDir()
	goenv := filepath.Join(base, "goenv")
	if err := os.WriteFile(goenv, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", goenv)

	shared := filepath.Join(base, "shared-cache")
	lint := filepath.Join(base, "checkout-lint-cache")
	for _, dir := range []string{shared, lint} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	first := layOutWorktree(t, filepath.Join(base, "wt-first"), shared, lint)
	second := layOutWorktree(t, filepath.Join(base, worktreeMarker), shared, lint)

	environments := make([][]string, 0, 2)
	for _, root := range []string{first, second} {
		t.Setenv("ze.repo.root", root)
		t.Setenv("ze.verify.scope.packages", filepath.Join(root, "tmp", "verify", "scope-packages.txt"))
		t.Setenv("ze.verify.scope.tags", filepath.Join(root, "tmp", "verify", "scope-tags.txt"))
		environments = append(environments, worktreeTestEnvironment(root))
	}
	for _, entry := range environments[1] {
		if !slices.Contains(environments[0], entry) {
			t.Errorf("the second worktree's test environment carries %q, which the first's does not", entry)
		}
	}
	if len(environments[0]) != len(environments[1]) {
		t.Errorf("the two worktrees' test environments differ in length: %d and %d",
			len(environments[0]), len(environments[1]))
	}

	if output, failed := runWorktreeProbe(t, first, environments[0]); failed || strings.Contains(output, "(cached)") {
		t.Fatalf("first worktree, over an empty cache: failed=%v\n%s", failed, output)
	}
	output, failed := runWorktreeProbe(t, second, environments[1])
	if failed || !strings.Contains(output, "(cached)") {
		t.Errorf("second worktree: the same commit under the same environment must answer the first pass (cached); failed=%v\n%s",
			failed, output)
	}
	rerun, rerunFailed := runWorktreeProbe(t, second, environments[1], "-count=1")
	if rerunFailed {
		t.Errorf("second worktree, -count=1: the rerun disagrees with the cached pass, so the cache hid a red\n%s", rerun)
	}
}

// layOutWorktree writes the probe module into root and links root's caches as
// the gate links a verify worktree's, and answers root.
func layOutWorktree(t *testing.T, root, shared, lint string) string {
	t.Helper()
	writeModule(t, root, worktreeProbeModule)
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, filepath.Join(root, "cache")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(lint, LintCache(root)); err != nil {
		t.Fatal(err)
	}
	return root
}

// worktreeTestEnvironment answers the environment the unit stages give a test
// run in root.
func worktreeTestEnvironment(root string) []string {
	return Toolchain{Root: root, Procs: 1}.Environment(EnvOptions{Test: true, Procs: true})
}

// runWorktreeProbe runs the probe package in root under environ.
func runWorktreeProbe(t *testing.T, root string, environ []string, flags ...string) (string, bool) {
	t.Helper()
	args := append([]string{"test"}, flags...)
	cmd := exec.CommandContext(t.Context(), goProgram, append(args, "./environ")...)
	cmd.Dir = root
	cmd.Env = environ
	output, err := cmd.CombinedOutput()
	return string(output), err != nil
}

// writeModule writes each file of module under root.
func writeModule(t *testing.T, root string, module map[string]string) {
	t.Helper()
	for name, content := range module {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// runCacheEnvTest runs `go test pkg` in root under the unit stages'
// environment and answers its combined output and whether it failed.
func runCacheEnvTest(t *testing.T, root, pkg string) (string, bool) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), goProgram, "test", pkg)
	cmd.Dir = root
	cmd.Env = Toolchain{Root: root, Procs: 1}.Environment(EnvOptions{Test: true, Procs: true})
	output, err := cmd.CombinedOutput()
	return string(output), err != nil
}
