package changed

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: every verb answers over the checkout the environment names, each
// carries the exit code its caller reads, and the selector grammar reaches the
// native change-set implementation.
// PREVENTS: wiring the native unit and scoped verification actions to a command
// whose verbs answer the wrong thing. Both callers size which tests run, so a
// verb that answers 0 and nothing runs no test and reports success.

// useCheckout points every verb in this test at one checkout.
//
// env.Get answers from a CACHE built once from os.Environ(). Thus, t.Setenv
// alone changes nothing that lepath.Root() sees. Without a cache reset, the test
// would use the DEVELOPER's own tree.
func useCheckout(t *testing.T, root string) {
	t.Helper()
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// gitFixture builds a checkout with one mapped and one unmapped change.
func gitFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}

	root := t.TempDir()
	writeFile(t, root, "go.mod", "module fixture\n\ngo 1.25\n")
	writeFile(t, root, "README.md", "base\n")
	gitIn(t, root, "init", "-q", ".")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "-c", "user.email=t@ze", "-c", "user.name=t", "commit", "-qm", "base")

	writeFile(t, root, "internal/core/env/env.go", "package env\n")
	writeFile(t, root, "internal/le/tool/tool.go", "package tool\n")
	return root
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	argv := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", argv...) //nolint:gosec,noctx // this test's own fixture
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestTheGroupVerbsAnswerTheSelectionOfTheCheckoutNamed(t *testing.T) {
	goToolchain(t)
	root := gitFixture(t)
	useCheckout(t, root)

	payload, code := Answer([]string{"groups"})
	if code != 0 {
		t.Fatalf("the groups verb exited %d", code)
	}
	names, isNames := payload.(groupNames)
	if !isNames {
		t.Fatalf("the payload is %T, want GroupNames", payload)
	}
	if names.Text() != "core\nrest\n" {
		t.Errorf("the groups verb rendered %q, want \"core\\nrest\\n\"", names.Text())
	}

	payload, code = Answer([]string{"group-packages"})
	if code != 0 {
		t.Fatalf("the group-packages verb exited %d", code)
	}
	packages, isPackages := payload.(groupPackages)
	if !isPackages {
		t.Fatalf("the payload is %T, want GroupPackages", payload)
	}
	if packages.Text() != "./internal/core/...\n./internal/le/tool\n" {
		t.Errorf("the group-packages verb rendered %q", packages.Text())
	}
}

// A checkout git cannot read answers 2, which a caller tells apart from a
// selection that came back empty. Answering 0 and nothing is the shell half's
// behavior and the reason this port exists.
func TestTheGroupVerbsAnswerTwoForACheckoutThatCannotBeRead(t *testing.T) {
	useCheckout(t, t.TempDir())

	for _, verb := range []string{"groups", "group-packages"} {
		payload, code := Answer([]string{verb})
		if code != 2 {
			t.Errorf("%q answered code %d with payload %v, want 2", verb, code, payload)
		}
	}
}

// The scope verb hands back the file a verify run published, and it reads that
// file's name from the environment.
func TestThePackagesVerbAnswersThePrecomputedFile(t *testing.T) {
	root := t.TempDir()
	scope := filepath.Join(root, "scope.txt")
	if err := os.WriteFile(scope, []byte("./cmd/ze\n./internal/core/env\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("ZE_VERIFY_SCOPE_PACKAGES", scope)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	payload, code := Answer([]string{"packages"})
	if code != 0 {
		t.Fatalf("the packages verb exited %d", code)
	}
	report, isReport := payload.(ScopeReport)
	if !isReport {
		t.Fatalf("the payload is %T, want a ScopeReport", payload)
	}
	if report.Widened {
		t.Errorf("the verb widened for a scope file it could read: %+v", report)
	}
	if len(report.Packages) != 2 {
		t.Fatalf("%d packages, want exactly 2: %v", len(report.Packages), report.Packages)
	}
}

// newScope reads the published filename from the environment. A run that
// omitted that read would repeat the selector pass in every scoped stage.
func TestTheScopeReadsThePublishedFileNameFromTheEnvironment(t *testing.T) {
	root := t.TempDir()
	scope := filepath.Join(root, "scope.txt")
	if err := os.WriteFile(scope, []byte("./cmd/ze\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("ZE_VERIFY_SCOPE_PACKAGES", scope)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	if got := newScope(root).File; got != scope {
		t.Errorf("the scope reads its file as %q, want %q", got, scope)
	}
}

// The native action translates the deleted flag surface to closed keywords,
// keeps both answers structured, and preserves the exact compatibility text.
func TestTheScopeActionAcceptsTheClosedSelectorGrammar(t *testing.T) {
	root := writeScopeFixture(t)
	useCheckout(t, root)
	paths := scopePathsFile(t, "core/core.go")
	dropLog := filepath.Join(t.TempDir(), "drops.txt")

	payload, code := Answer([]string{
		"scope",
		"drop-log", dropLog,
		"paths-from", paths,
		"depth", "1",
		"print", "both",
	})
	if code != 0 {
		t.Fatalf("the scope action exited %d", code)
	}
	report, ok := payload.(ScopeReport)
	if !ok {
		t.Fatalf("the scope payload is %T, want ScopeReport", payload)
	}
	if report.Print != "both" {
		t.Errorf("the structured print mode is %q, want both", report.Print)
	}
	if got := report.Text(); !strings.HasPrefix(got, "# packages\n./core\n./mid\n# tags\n") {
		t.Errorf("the both-mode text is %q", got)
	}
	body, err := os.ReadFile(dropLog)
	if err != nil {
		t.Fatalf("read drop log: %v", err)
	}
	if !strings.Contains(string(body), "# selected-at-depth-1-but-not-at-depth-1\n") {
		t.Errorf("the drop log does not carry the requested depth:\n%s", body)
	}

	payload, code = Answer([]string{"scope", "print", "tags", "paths-from", paths})
	if code != 0 {
		t.Fatalf("the tags mode exited %d", code)
	}
	report, ok = payload.(ScopeReport)
	if !ok {
		t.Fatalf("the tags payload is %T, want ScopeReport", payload)
	}
	if got := report.Text(); got != "ze_bgp\nze_ssh\n" {
		t.Errorf("the tags-mode text is %q", got)
	}
}

func TestTheScopeActionDefaultsToPackageText(t *testing.T) {
	root := writeScopeFixture(t)
	useCheckout(t, root)
	paths := scopePathsFile(t, "core/core.go")

	payload, code := Answer([]string{"scope", "paths-from", paths})
	if code != 0 {
		t.Fatalf("the scope action exited %d", code)
	}
	report, ok := payload.(ScopeReport)
	if !ok {
		t.Fatalf("the scope payload is %T, want ScopeReport", payload)
	}
	if report.Print != "packages" {
		t.Errorf("the default print mode is %q, want packages", report.Print)
	}
	if got := report.Text(); got != "./core\n./high\n./mid\n" {
		t.Errorf("the default package text is %q", got)
	}
}

// Grammar and value errors use the common caller-error exit code.
func TestTheScopeActionPreservesGrammarAndValueExitCodes(t *testing.T) {
	useCheckout(t, t.TempDir())
	cases := []struct {
		name string
		args []string
		code int
	}{
		{name: "missing value", args: []string{"scope", "print"}, code: 2},
		{name: "flag spelling", args: []string{"scope", "--print=tags"}, code: 2},
		{name: "duplicate keyword", args: []string{"scope", "print", "tags", "print", "both"}, code: 2},
		{name: "unknown mode", args: []string{"scope", "print", "nope"}, code: 2},
		{name: "non-number depth", args: []string{"scope", "depth", "one"}, code: 2},
		{name: "zero depth", args: []string{"scope", "depth", "0"}, code: 2},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, code := Answer(test.args); code != test.code {
				t.Errorf("Answer(%v) exited %d, want %d", test.args, code, test.code)
			}
		})
	}
}

// The default runner, against real programs. Every other case drives an
// injected one, so without this the code the binary actually runs is never
// executed.
func TestTheDefaultRunnerAnswersWhatAProgramPrinted(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}

	out, err := RunCommand(t.TempDir(), []string{"git", "--version"})
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if !strings.HasPrefix(out, "git version") {
		t.Errorf("RunCommand answered %q", out)
	}

	if _, err := RunCommand(t.TempDir(), nil); err == nil {
		t.Error("an empty command line answered no error")
	}
	if _, err := RunCommand(t.TempDir(), []string{"git", "no-such-subcommand"}); err == nil {
		t.Error("a failing command answered no error")
	}
}

// A package directory outside the checkout is not this run's business. `go
// list` answers absolute paths, and one that is not under the root would be
// spelled as a relative path that resolves somewhere else entirely.
func TestAPackageDirectoryOutsideTheCheckoutIsDropped(t *testing.T) {
	root := t.TempDir()
	rec := &recorder{answers: map[string]string{
		"go list -e -f {{if not .Error}}{{.Dir}}{{end}} ./a": root + "/a\n/elsewhere/b\n" + root + "\n",
	}}

	packages, err := Selector{Root: root, Run: rec.run}.Packages([]string{"./a"})
	if err != nil {
		t.Fatalf("Packages: %v", err)
	}
	want := []string{".", "./a"}
	if len(packages) != len(want) {
		t.Fatalf("%d packages, want exactly %d: %v", len(packages), len(want), packages)
	}
	for i := range want {
		if packages[i] != want[i] {
			t.Errorf("package %d is %q, want %q", i, packages[i], want[i])
		}
	}
}

// An empty directory list asks the toolchain nothing, so no command runs.
func TestNoPackageDirectoryAsksTheToolchainNothing(t *testing.T) {
	rec := &recorder{}
	packages, err := Selector{Root: t.TempDir(), Run: rec.run}.Packages(nil)
	if err != nil {
		t.Fatalf("Packages: %v", err)
	}
	if len(packages) != 0 {
		t.Errorf("%d packages answered for an empty list", len(packages))
	}
	if len(rec.calls) != 0 {
		t.Errorf("%d commands ran for an empty list", len(rec.calls))
	}
}
