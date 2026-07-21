package main

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const verifyWiringDocsTimeout = 60 * time.Second

// VALIDATES: Changing a command declaration schedules command reference validation.
// PREVENTS: YANG command tree drift staying advisory until commit-time hooks.
func TestVerifyWiringDocsRoutesCommandChanges(t *testing.T) {
	out := runVerifyWiringDocsDryRun(t, "internal/component/cmd/show/yang/ze-cli-show-cmd.yang")
	mustContain(t, out, "ze-validate-commands")
}

// VALIDATES: Changing anchored docs schedules documentation validation and stale-anchor checks.
// PREVENTS: source-anchored documentation drift bypassing the normal verify path.
func TestVerifyWiringDocsRoutesAnchoredDocChanges(t *testing.T) {
	out := runVerifyWiringDocsDryRun(t, "docs/DESIGN.md")
	mustContain(t, out, "ze-doc-test")
	mustContain(t, out, "ze-doc-check-stale")
}

// VALIDATES: Changing a test file that declares a `func Fuzz` schedules the
// fuzz-target enumeration freshness gate, so a new/removed fuzzer cannot leave
// mk/test-fuzz-targets.mk stale.
// PREVENTS: a fuzz target being added to source but never reaching the runner.
func TestVerifyWiringDocsRoutesFuzzTargetChanges(t *testing.T) {
	out := runVerifyWiringDocsDryRun(t, "internal/plugins/isis/packet/fuzz_test.go")
	mustContain(t, out, "ze-fuzz-targets-check")

	// The generated fragment and its generator route the gate too.
	out = runVerifyWiringDocsDryRun(t, "mk/test-fuzz-targets.mk")
	mustContain(t, out, "ze-fuzz-targets-check")

	// A real internal/ test file that declares no `func Fuzz` must NOT route the
	// gate -- exercises the read-and-reject branch, not just the prefix guard.
	out = runVerifyWiringDocsDryRun(t, "internal/core/clock/clock_test.go")
	if strings.Contains(out, "ze-fuzz-targets-check") {
		t.Fatalf("internal non-fuzz test file routed the fuzz gate:\n%s", out)
	}

	// A non-internal test path is rejected at the prefix guard, before any read.
	out = runVerifyWiringDocsDryRun(t, "scripts/dev/does-not-declare-fuzz_test.go")
	if strings.Contains(out, "ze-fuzz-targets-check") {
		t.Fatalf("non-fuzz path routed the fuzz gate:\n%s", out)
	}
}

// VALIDATES: Changing plugin registration schedules registry-backed inventory checks.
// PREVENTS: plugin/all.go and command inventory drift staying outside ze-verify.
func TestVerifyWiringDocsRoutesPluginRegistrationChanges(t *testing.T) {
	out := runVerifyWiringDocsDryRun(t, "internal/plugins/static/register.go")
	mustContain(t, out, "ze-inventory-json")
	mustContain(t, out, "ze-command-list-json")
	mustContain(t, out, "ze-plugin-imports-check")
}

// VALIDATES: The plugin-import check verifies generated all.go without rewriting it.
// PREVENTS: registry drift checks from mutating generated files during verification.
func TestVerifyWiringDocsChecksPluginImports(t *testing.T) {
	out := runCommand(t, repoRoot(t), "make", "ze-plugin-imports-check")
	mustContain(t, out, "is current")
}

// VALIDATES: A new exported symbol with no production reference is reported by the wiring check.
// PREVENTS: library-only feature code being treated as complete because unit tests pass.
func TestVerifyWiringDocsReportsUnwiredExportedSymbol(t *testing.T) {
	root := makeFixtureRoot(t)
	writeFixture(t, root, "internal/example/example.go", "package example\n\nfunc UnwiredFeature() {}\n")

	out := runPythonSnippet(t, fmt.Sprintf(`
import pathlib
import sys
sys.path.insert(0, %q)
import verify_wiring_docs
root = pathlib.Path(%q)
issues = verify_wiring_docs.check_wiring(root, ["internal/example/example.go"], lambda path: "package example\n")
print("\n".join(issues))
if not any("UnwiredFeature" in issue for issue in issues):
    raise SystemExit(1)
`, filepath.Join(repoRoot(t), "scripts", "dev"), root))
	mustContain(t, out, "UnwiredFeature")
}

// VALIDATES: A new exported symbol with a production reference passes the wiring check.
// PREVENTS: the wiring gate from blocking correctly wired entry points.
func TestVerifyWiringDocsAcceptsReferencedExportedSymbol(t *testing.T) {
	root := makeFixtureRoot(t)
	writeFixture(t, root, "internal/example/example.go", "package example\n\nfunc WiredFeature() {}\n")
	writeFixture(t, root, "cmd/ze/main.go", "package main\n\nfunc main() { _ = example.WiredFeature }\n")

	out := runPythonSnippet(t, fmt.Sprintf(`
import pathlib
import sys
sys.path.insert(0, %q)
import verify_wiring_docs
root = pathlib.Path(%q)
issues = verify_wiring_docs.check_wiring(root, ["internal/example/example.go"], lambda path: "package example\n")
print("\n".join(issues))
if issues:
    raise SystemExit(1)
`, filepath.Join(repoRoot(t), "scripts", "dev"), root))
	if out != "" {
		t.Fatalf("expected no wiring issues, got:\n%s", out)
	}
}

// VALIDATES: a symbol merely relocated (file deleted at the old path, re-added
// at a new path) is NOT reported as a new unwired symbol, even when it has no
// production reference -- a behavior-preserving move (e.g. a component<->plugin
// tier migration) introduces no new API.
// PREVENTS: every package relocation false-flagging the unwired helpers it carries.
func TestVerifyWiringDocsIgnoresRelocatedSymbol(t *testing.T) {
	root := makeFixtureRoot(t)
	// the moved file exists at the NEW path; the OLD path is deleted (absent).
	writeFixture(t, root, "internal/component/widget/w.go", "package widget\n\nfunc Helper() {}\n")

	out := runPythonSnippet(t, fmt.Sprintf(`
import pathlib
import sys
sys.path.insert(0, %q)
import verify_wiring_docs
root = pathlib.Path(%q)
def base(path):
    if path == "internal/plugins/widget/w.go":
        return "package widget\n\nfunc Helper() {}\n"
    return ""
issues = verify_wiring_docs.check_wiring(
    root,
    ["internal/plugins/widget/w.go", "internal/component/widget/w.go"],
    base,
)
print("\n".join(issues))
if issues:
    raise SystemExit(1)
`, filepath.Join(repoRoot(t), "scripts", "dev"), root))
	if out != "" {
		t.Fatalf("relocated symbol must not be flagged, got:\n%s", out)
	}
}

// VALIDATES: a diff touching user-facing code (cli/web/config/cmd) with no
// test/ change emits a functional-test advisory naming the expected suite dir.
// PREVENTS: direct-to-code sessions bypassing the functional-test gate
// (ai/rules/functional-test-gate.md) with no signal at verify time.
func TestVerifyWiringDocsAdvisesFunctionalTests(t *testing.T) {
	out := runVerifyWiringDocsDryRun(t, "internal/component/web/handler_admin.go")
	mustContain(t, out, "ADVISORY: user-facing code changed without a functional-test change")
	mustContain(t, out, "test/web/")
}

// VALIDATES: the .ci sleep ratchet fails when time.sleep occurrences exceed
// the committed baseline, and asks for a baseline lower-down when below it.
// PREVENTS: new timing-dependent .ci tests (sleeps hide real races; ze_api
// provides wait_for_event) growing the 400+ legacy sleep surface.
func TestVerifyWiringDocsSleepRatchet(t *testing.T) {
	root := makeFixtureRoot(t)
	writeFixture(t, root, "go.mod", "module example.com/m\n")
	writeFixture(t, root, "test/.ci-sleep-baseline", "1\n")
	writeFixture(t, root, "test/x/a.ci", "run=python\ntime.sleep(1)\ntime.sleep(2)\n")

	out := runCommandAllowError(t, repoRoot(t), "python3",
		"scripts/dev/verify_wiring_docs.py", "--root", root,
		"--changed-file", "test/x/a.ci")
	mustContain(t, out, "ci-sleep ratchet FAILED")
	mustContain(t, out, "wait_for_event")
}

// VALIDATES: the ratchet passes at or below the baseline and, when the count
// dropped, suggests appending a negative delta line (the composable form) to
// tighten the baseline.
func TestVerifyWiringDocsSleepRatchetSuggestsLowering(t *testing.T) {
	root := makeFixtureRoot(t)
	writeFixture(t, root, "go.mod", "module example.com/m\n")
	writeFixture(t, root, "test/.ci-sleep-baseline", "5\n")
	// The sleep carries a justifying comment so this fixture isolates the
	// ratchet's advisory path: an unjustified sleep would additionally trip
	// check_ci_sleep_justification and exit non-zero, which runCommand rejects.
	writeFixture(t, root, "test/x/a.ci", "run=python\n# deliberate timer, kept\ntime.sleep(1)\n")

	out := runCommand(t, repoRoot(t), "python3",
		"scripts/dev/verify_wiring_docs.py", "--root", root,
		"--changed-file", "test/x/a.ci")
	// ceiling 5 - count 1 = a `-4` delta tightens it.
	mustContain(t, out, "append a `-4` delta line to test/.ci-sleep-baseline")
}

// VALIDATES: the advisory stays silent when the diff already touches test/.
func TestVerifyWiringDocsNoAdvisoryWhenTestsChanged(t *testing.T) {
	out := runVerifyWiringDocsDryRun(t, "internal/component/web/handler_admin.go", "test/web/example.wb")
	if strings.Contains(out, "ADVISORY: user-facing code changed") {
		t.Fatalf("advisory must not fire when test/ files changed, got:\n%s", out)
	}
}

func runVerifyWiringDocsDryRun(t *testing.T, files ...string) string {
	t.Helper()
	args := []string{"scripts/dev/verify_wiring_docs.py", "--dry-run"}
	for _, file := range files {
		args = append(args, "--changed-file", file)
	}
	return runCommand(t, repoRoot(t), "python3", args...)
}

// runCommandAllowError returns combined output regardless of exit code,
// for asserting on intentional gate failures.
func runCommandAllowError(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), verifyWiringDocsTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("%s %s timed out", name, strings.Join(args, " "))
	}
	return strings.TrimSpace(string(out))
}

func runPythonSnippet(t *testing.T, script string) string {
	t.Helper()
	return runCommand(t, repoRoot(t), "python3", "-c", script)
}

func runCommand(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), verifyWiringDocsTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func makeFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(repoRoot(t), "tmp", "verify-wiring-docs-test", strings.ReplaceAll(t.Name(), "/", "-"))
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove old fixture root: %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create fixture root: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root
}

func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", rel, err)
	}
}

func mustContain(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("missing %q in:\n%s", want, got)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}
