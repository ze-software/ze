// Design: docs/contributing/testing.md -- lint admission measures the packages lint actually loads.
// Related: input_population_test.go -- the excluded-input refusal on a module with no Git
package golint

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
)

// lintGuardScratch is a preserved scratch backup in the checkout's own shape:
// beneath tmp/, whose go.mod makes it a nested module the root ./... never
// loads. Its body is deliberately not Go, so compiling it would fail.
const lintGuardScratch = "tmp/session/2026-11-10-120000/scratch/probe.go"

// TestLintInputGuardMatchesProducer asks the guard (CheckLintInputs, run
// inside the real go-list population producer) and the fingerprint
// (job.InputHash under job.LintLabel) about one Git checkout laid out as the
// real one is. A real Go input in an excluded tree (backups/ with no nested
// module) is selected and refused by name. With it gone, every selected input
// is checked: an edit to each one changes the lint fingerprint, and restoring
// it restores the fingerprint, so no selected input can be shared across. The
// preserved scratch backup is neither selected, nor compiled by a real go
// test, nor touched by either side.
//
// MUTATION: verifylint.go goList `job.CheckLintInputs(root, read.files)` ->
// `job.CheckLintInputs(root, nil)` reddens "lint admitted excluded real input".
// MUTATION: treehash.go labelIgnores `LintLabel: lintIgnores,` ->
// `LintLabel: {".claude/", "ai/", "backups/", "docs/", "live/", "plan/", "rfc/", "website/"},`
// reddens "did not change the lint fingerprint" for live/live.go.
func TestLintInputGuardMatchesProducer(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=mod")
	root := t.TempDir()
	writeLintFile(t, root, "go.mod", "module lintfixture\n\ngo 1.27.0\n")
	writeLintFile(t, root, ".gitignore", "tmp/*\n!tmp/go.mod\n/backups/\n")
	writeLintFile(t, root, "live/live.go", "package live\n\nconst Value = 7\n")
	writeLintFile(t, root, "live/live_test.go", "package live\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) {\n\tif Value != 7 {\n\t\tt.Fatal(Value)\n\t}\n}\n")
	writeLintFile(t, root, "tmp/go.mod", "module scratch\n\ngo 1.27.0\n")
	scratch := "preserved evidence, deliberately not a Go package\n"
	writeLintFile(t, root, lintGuardScratch, scratch)
	lintGuardGit(t, root, "init", "--quiet")
	lintGuardGit(t, root, "add", ".")
	lintGuardGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.com", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "lint guard fixture")

	runner := &Runner{
		ctx: t.Context(), root: root, toolchain: fixtureChain(root),
		ops: runnerOps{capture: captureCommand},
	}
	host := Flavor{Name: "host", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}

	const excluded = "backups/probe.go"
	writeLintFile(t, root, excluded, "package probe\n\nconst Value = 9\n")
	_, selected, _, err := runner.goList(host, []string{"./..."})
	if err == nil {
		t.Fatalf("lint admitted excluded real input %s (selected=%v)", excluded, selected)
	}
	if !strings.Contains(err.Error(), excluded) {
		t.Fatalf("refusal does not name excluded real input %s: %v", excluded, err)
	}
	if err := os.RemoveAll(filepath.Join(root, "backups")); err != nil {
		t.Fatal(err)
	}

	_, selected, _, err = runner.goList(host, []string{"./..."})
	if err != nil {
		t.Fatalf("checkout with only a preserved backup stopped lint: %v", err)
	}
	for _, input := range []string{"live/live.go", "live/live_test.go"} {
		if !selected[input] {
			t.Fatalf("producer omitted real input %s: %v", input, selected)
		}
	}
	if selected[lintGuardScratch] {
		t.Fatalf("producer selected preserved backup %s", lintGuardScratch)
	}

	baseline, err := job.InputHash(root, job.LintLabel)
	if err != nil {
		t.Fatal(err)
	}
	for input := range selected {
		path := filepath.Join(root, filepath.FromSlash(input))
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(bytes.Clone(original), "\n// edited\n"...), 0o600); err != nil {
			t.Fatal(err)
		}
		edited, err := job.InputHash(root, job.LintLabel)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal(err)
		}
		if edited == baseline {
			t.Errorf("an edit to selected input %s did not change the lint fingerprint", input)
		}
		restored, err := job.InputHash(root, job.LintLabel)
		if err != nil {
			t.Fatal(err)
		}
		if restored != baseline {
			t.Errorf("restoring selected input %s did not restore the lint fingerprint", input)
		}
	}

	cmd := exec.CommandContext(t.Context(), "go", "test", "-mod=mod", "./...")
	cmd.Dir = root
	cmd.Env = runner.toolchain.Environment(gotoolchain.EnvOptions{Test: true})
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		t.Fatalf("real Go compilation included preserved backup %s: %v\n%s", lintGuardScratch, runErr, output)
	}
	kept, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(lintGuardScratch)))
	if err != nil {
		t.Fatalf("preserved backup %s is gone: %v", lintGuardScratch, err)
	}
	if string(kept) != scratch {
		t.Fatalf("preserved backup %s changed: %q", lintGuardScratch, kept)
	}
}

// TestLintRefusesAGitIgnoredInput selects a real Go package from a directory
// git ignores and that has no go.mod of its own. job.InputHash reads only the
// paths git reports, so that input is outside the lint fingerprint and an
// edit to it could not void a shared verdict: the guard must refuse it by
// name. The control gives the same directory its own go.mod, which takes it
// out of the root module, and lint admits the checkout without it.
//
// MUTATION: treehash.go CheckLintInputs `return refuseGitIgnoredInputs(root, canonicals)`
// -> `return nil` reddens "lint admitted git-ignored real input".
func TestLintRefusesAGitIgnoredInput(t *testing.T) {
	const ignored = "scratch/probe.go"
	for _, nested := range []bool{false, true} {
		name := "ignored-real-input"
		if nested {
			name = "ignored-nested-module"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("GOPROXY", "off")
			t.Setenv("GOWORK", "off")
			t.Setenv("GOFLAGS", "-mod=mod")
			root := t.TempDir()
			writeLintFile(t, root, "go.mod", "module lintfixture\n\ngo 1.27.0\n")
			writeLintFile(t, root, ".gitignore", "scratch/\n")
			writeLintFile(t, root, "live/live.go", "package live\n\nconst Value = 7\n")
			writeLintFile(t, root, ignored, "package probe\n\nconst Value = 9\n")
			if nested {
				writeLintFile(t, root, "scratch/go.mod", "module scratch\n\ngo 1.27.0\n")
			}
			lintGuardGit(t, root, "init", "--quiet")
			runner := &Runner{
				ctx: t.Context(), root: root, toolchain: fixtureChain(root),
				ops: runnerOps{capture: captureCommand},
			}
			_, selected, _, err := runner.goList(Flavor{Name: "host", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}, []string{"./..."})
			if nested {
				if err != nil {
					t.Fatalf("nested-module scratch stopped lint: %v", err)
				}
				if !selected["live/live.go"] {
					t.Fatalf("producer omitted real input live/live.go: %v", selected)
				}
				if selected[ignored] {
					t.Fatalf("producer selected nested-module scratch %s", ignored)
				}
				return
			}
			if err == nil {
				t.Fatalf("lint admitted git-ignored real input %s (selected=%v)", ignored, selected)
			}
			if !strings.Contains(err.Error(), ignored) {
				t.Fatalf("refusal does not name git-ignored input %s: %v", ignored, err)
			}
			if !strings.Contains(err.Error(), "ignored by git") {
				t.Fatalf("refusal does not say git ignores the input: %v", err)
			}
		})
	}
}

func lintGuardGit(t *testing.T, root string, argv ...string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", argv...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %q: %v: %s", argv, err, output)
	}
}
