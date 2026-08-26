package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/letools/changed"
)

// VALIDATES: letools/changed answers what scripts/dev/changed-groups.sh and
// scripts/dev/changed-pkgs.sh answer, over the same checkout -- and fails
// CLOSED on the three routes where the shell half fails open.
// PREVENTS: a swap (step 14) that repoints mk/test-unit.mk and the scoped
// recipes at a command answering a different set. Both halves size which tests
// run, so a difference here is a suite that passes having tested less.
//
// It sits beside the scripts instead of the Go package. Thus, step 14 deletes
// the scripts and this proof together. A parity test that outlives one compared
// half has one side missing.
//
// Every case asserts the ABSOLUTE number of names each half answered, never
// that two numbers match. Two halves that both stop early agree perfectly.

// changedFixture is a checkout with a known change set. It has three mapped
// groups and one unmapped but buildable package. It also has an unmapped
// directory whose only Go file is build-ignored. The last file is under a
// subtree that the shell declares as a group but cannot reach.
func changedFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}

	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("go.mod", "module parity\n\ngo 1.25\n")
	write("README.md", "base\n")
	git(t, root, "init", "-q", ".")
	git(t, root, "add", "-A")
	git(t, root, "-c", "user.email=parity@ze", "-c", "user.name=parity", "commit", "-qm", "base")

	// Unstaged, staged and untracked in one fixture, so a half that reads only
	// one of the three git queries answers a shorter list.
	write("internal/component/bgp/reactor/peer.go", "package reactor\n")
	write("internal/core/env/env.go", "package env\n")
	git(t, root, "add", "internal/core/env/env.go")
	write("cmd/ze/main.go", "package main\n\nfunc main() {}\n")
	write("internal/component/l2tp/ppp/session.go", "package ppp\n")
	write("letools/tool/tool.go", "package tool\n")
	write("scripts/gen/gen.go", "//go:build zzz_never_defined\n\npackage main\n")
	return root
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec,noctx // fixture setup in a temp checkout
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// changedGroupsScript is the half runShell drives. changed-pkgs.sh is reached
// by its own two cases, because each needs an environment of its own.
const changedGroupsScript = "changed-groups.sh"

// runShell runs the group script against a fixture and answers its non-empty
// output lines and its exit code.
func runShell(t *testing.T, dir string, args ...string) ([]string, int) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash on PATH")
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}

	argv := append([]string{filepath.Join(repo, "scripts", "dev", changedGroupsScript)}, args...)
	cmd := exec.Command("bash", argv...) //nolint:gosec,noctx // the script under comparison
	cmd.Dir = dir
	out, err := cmd.Output()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !asExit(err, &exit) {
			t.Fatalf("%s: %v", changedGroupsScript, err)
		}
		code = exit.ExitCode()
	}
	return nonEmptyLines(string(out)), code
}

func asExit(err error, target **exec.ExitError) bool {
	exit, ok := err.(*exec.ExitError) //nolint:errorlint // the concrete type is what carries the code
	if ok {
		*target = exit
	}
	return ok
}

func nonEmptyLines(out string) []string {
	var lines []string
	for line := range strings.SplitSeq(out, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func equal(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d names, want exactly %d\n got: %v\nwant: %v", what, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s: name %d is %q, want %q", what, i, got[i], want[i])
		}
	}
}

// The group names, both halves, over one checkout.
//
// ONE difference is deliberate: the ppp row. The shell declares a `ppp` group
// for internal/component/l2tp/ppp/, but its hash order makes that group
// unreachable. Thus, both halves answer l2tp. The port does this by design
// because it dropped a row that only narrowed the run.
func TestBothHalvesGroupOneCheckoutTheSameWay(t *testing.T) {
	root := changedFixture(t)

	shell, code := runShell(t, root)
	if code != 0 {
		t.Fatalf("changed-groups.sh exited %d", code)
	}

	selection, err := changed.Selector{Root: root}.Select()
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	port := nonEmptyLines(changed.GroupNames{Selection: selection}.Text())

	want := []string{"bgp", "cmd", "core", "l2tp", "rest"}
	equal(t, "the shell's groups", sorted(shell), want)
	equal(t, "the port's groups", sorted(port), want)
}

// The package patterns, both halves. `scripts/gen` holds one build-ignored Go
// file, so both halves must drop it: `go test` refuses such a directory with
// "build constraints exclude all Go files".
func TestBothHalvesAnswerTheSamePackagePatterns(t *testing.T) {
	root := changedFixture(t)

	shell, code := runShell(t, root, "--pkgs")
	if code != 0 {
		t.Fatalf("changed-groups.sh --pkgs exited %d", code)
	}

	selection, err := changed.Selector{Root: root}.Select()
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	port := nonEmptyLines(changed.GroupPackages{Selection: selection}.Text())

	want := []string{
		"./cmd/...",
		"./internal/component/bgp/...",
		"./internal/component/l2tp/...",
		"./internal/core/...",
		"./letools/tool",
	}
	equal(t, "the shell's patterns", sorted(shell), want)
	equal(t, "the port's patterns", sorted(port), want)
}

// A checkout git cannot read.
//
// The test deliberately asserts that the SHELL fails OPEN. This row fails when
// somebody repairs the script. That failure shows that the port's guard is no
// longer the only guard (ai/rules/testing.md).
func TestTheShellFailsOpenWithoutGitAndThePortFailsClosed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	shell, code := runShell(t, root)
	if code != 0 || len(shell) != 0 {
		t.Fatalf("scripts/dev/changed-groups.sh answered %v with code %d;"+
			" this case pins its fail-open, so repair it and delete this case", shell, code)
	}

	if _, err := (changed.Selector{Root: root}).Select(); err == nil {
		t.Error("the port answered a selection for a checkout git cannot read")
	}
}

// The toolchain cannot load this module. Every changed file here is unmapped,
// so the shell gets its whole answer through `go list`. It answers nothing with
// exit 0. mk/test-unit.mk:123 reads that as "No changed .go files".
func TestTheShellFailsOpenOnABrokenModuleAndThePortFailsClosed(t *testing.T) {
	root := changedFixture(t)
	if err := os.WriteFile(filepath.Join(root, "go.mod"),
		[]byte("module parity\n\ngo 1.25\nrequire nosuch v9.9.9\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	// Leave ONLY unmapped changes, so no group name can stand in for the
	// packages `go list` was supposed to resolve.
	for _, rel := range []string{
		"internal/component/bgp/reactor/peer.go",
		"internal/core/env/env.go",
		"cmd/ze/main.go",
		"internal/component/l2tp/ppp/session.go",
	} {
		if err := os.Remove(filepath.Join(root, rel)); err != nil {
			t.Fatalf("remove %s: %v", rel, err)
		}
	}
	git(t, root, "reset", "-q")

	shell, code := runShell(t, root)
	if code != 0 || len(shell) != 0 {
		t.Fatalf("scripts/dev/changed-groups.sh answered %v with code %d;"+
			" this case pins its fail-open, so repair it and delete this case", shell, code)
	}

	if _, err := (changed.Selector{Root: root}).Select(); err == nil {
		t.Error("the port answered a selection over a module the toolchain cannot load")
	}
}

// ZE_VERIFY_SCOPE_PACKAGES naming a path that is readable but not a file.
//
// The shell uses `[ -r ]` to test it, and that test passes for a directory. The
// shell then ignores the exit status from `cat`. The scoped recipe gets an empty
// package list and exit 0, so it covers nothing and reports success. The script
// header states that every failure route must widen.
func TestTheShellFailsOpenOnAnUnreadableScopeFileAndThePortWidens(t *testing.T) {
	root := t.TempDir()
	scope := filepath.Join(root, "scope-dir")
	if err := os.Mkdir(scope, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}
	cmd := exec.Command("bash", filepath.Join(repo, "scripts", "dev", "changed-pkgs.sh")) //nolint:gosec,noctx // the script under comparison
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "ZE_VERIFY_SCOPE_PACKAGES="+scope)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("changed-pkgs.sh: %v", err)
	}
	if lines := nonEmptyLines(string(out)); len(lines) != 0 {
		t.Fatalf("scripts/dev/changed-pkgs.sh answered %v; this case pins its fail-open,"+
			" so repair it and delete this case", lines)
	}

	report := changed.Scope{Root: root, File: scope}.Resolve(nil)
	if !report.Widened {
		t.Error("the port did not widen for a scope file it could not read")
	}
	if len(report.Packages) != 1 || report.Packages[0] != "./..." {
		t.Errorf("the port answered %v, want exactly [./...]", report.Packages)
	}
}

// A scope file both halves CAN read: the answer is the file, byte for byte, and
// neither half runs the selector for it.
func TestBothHalvesHandBackThePrecomputedScopeFile(t *testing.T) {
	root := t.TempDir()
	scope := filepath.Join(root, "scope.txt")
	body := "./internal/core/env\n./cmd/ze\n"
	if err := os.WriteFile(scope, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}
	cmd := exec.Command("bash", filepath.Join(repo, "scripts", "dev", "changed-pkgs.sh")) //nolint:gosec,noctx // the script under comparison
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "ZE_VERIFY_SCOPE_PACKAGES="+scope)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("changed-pkgs.sh: %v", err)
	}

	report := changed.Scope{Root: root, File: scope}.Resolve(nil)
	want := []string{"./cmd/ze", "./internal/core/env"}
	equal(t, "the shell's scope", sorted(nonEmptyLines(string(out))), want)
	equal(t, "the port's scope", sorted(report.Packages), want)
	if report.Widened {
		t.Error("the port widened for a scope file it could read")
	}
}
