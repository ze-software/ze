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

func runVerifyWiringDocsDryRun(t *testing.T, files ...string) string {
	t.Helper()
	args := []string{"scripts/dev/verify_wiring_docs.py", "--dry-run"}
	for _, file := range files {
		args = append(args, "--changed-file", file)
	}
	return runCommand(t, repoRoot(t), "python3", args...)
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
