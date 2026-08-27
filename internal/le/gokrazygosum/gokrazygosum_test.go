// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7 -- the packed go.sum gate is
// called as a function and answers structured data.
// PREVENTS: a port that reports every difference between the two files instead
// of the one that cannot be legitimate. A builddir-only module and a version
// skew are normal, so a gate that fires on them is noise nobody can act on,
// and a gate that stops firing on a hash disagreement ships a contradiction
// into the image.

package gokrazygosum

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// tree writes a fixture checkout and tracks every file in it, because the gate
// asks git what the repository ships rather than walking the disk.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
	for _, argv := range [][]string{
		{"init", "--quiet"},
		{"add", "--all"},
	} {
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, argv...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", argv[0], err, out)
		}
	}
	return root
}

// resetRootCache makes a ZE_REPO_ROOT set by a test visible to lepath.Root.
// env.Get reads os.Environ() once per process and caches it, so a t.Setenv
// after the first read answers the real checkout without this.
func resetRootCache(t *testing.T) {
	t.Helper()
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

const (
	hashOne = "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	hashTwo = "h1:BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="
)

func TestOneKeyHashingTwoWaysIsAConflict(t *testing.T) {
	root := tree(t, map[string]string{
		"go.sum": "example.com/x v1.0.0 " + hashOne + "\n",
		"gokrazy/ze/builddir/example.com/y/go.sum": "example.com/x v1.0.0 " + hashTwo + "\n",
	})

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(report.Conflicts) != 1 {
		t.Fatalf("Check reported %d conflicts, want 1: %+v", len(report.Conflicts), report)
	}
	got := report.Conflicts[0]
	if got.Module != "example.com/x" || got.Version != "v1.0.0" {
		t.Errorf("the conflict names %s %s, want example.com/x v1.0.0", got.Module, got.Version)
	}
	if got.RootHash != hashOne || got.BuilddirHash != hashTwo {
		t.Errorf("the conflict carries %s and %s", got.RootHash, got.BuilddirHash)
	}
	if got.Path != "gokrazy/ze/builddir/example.com/y/go.sum" {
		t.Errorf("the conflict names the file %q", got.Path)
	}
}

func TestAKeyTheRootDoesNotHoldIsNotAConflict(t *testing.T) {
	root := tree(t, map[string]string{
		"go.sum": "example.com/x v1.0.0 " + hashOne + "\n",
		"gokrazy/ze/builddir/example.com/y/go.sum": "example.com/only v2.0.0 " + hashTwo + "\n",
	})

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(report.Conflicts) != 0 {
		t.Fatalf("a builddir-only module was reported: %+v", report.Conflicts)
	}
	if report.Shared != 0 {
		t.Errorf("Shared is %d, want 0: no key is in both files", report.Shared)
	}
}

func TestVersionSkewIsNotAConflict(t *testing.T) {
	root := tree(t, map[string]string{
		"go.sum": "example.com/x v1.0.0 " + hashOne + "\n",
		"gokrazy/ze/builddir/example.com/y/go.sum": "example.com/x v2.0.0 " + hashTwo + "\n",
	})

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(report.Conflicts) != 0 {
		t.Fatalf("two versions of one module were reported as a conflict: %+v", report.Conflicts)
	}
}

func TestTheZipLineAndTheGoModLineAreSeparateEntries(t *testing.T) {
	root := tree(t, map[string]string{
		"go.sum": "example.com/x v1.0.0 " + hashOne + "\n" +
			"example.com/x v1.0.0/go.mod " + hashOne + "\n",
		"gokrazy/ze/builddir/example.com/y/go.sum": "example.com/x v1.0.0 " + hashOne + "\n" +
			"example.com/x v1.0.0/go.mod " + hashTwo + "\n",
	})

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Shared != 2 {
		t.Errorf("Shared is %d, want 2: the zip line and the go.mod line are two entries", report.Shared)
	}
	if len(report.Conflicts) != 1 || report.Conflicts[0].Version != "v1.0.0/go.mod" {
		t.Fatalf("want one conflict on the go.mod line, got %+v", report.Conflicts)
	}
}

func TestAnUntrackedBuilddirGosumIsNotRead(t *testing.T) {
	root := tree(t, map[string]string{
		"go.sum": "example.com/x v1.0.0 " + hashOne + "\n",
	})
	// Written after `git add`, so it is on disk and not in the index. A local
	// build leaves such a file behind and the image does not carry it.
	path := filepath.Join(root, "gokrazy", "ze", "builddir", "example.com", "y", "go.sum")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("example.com/x v1.0.0 "+hashTwo+"\n"), 0o600); err != nil {
		t.Fatalf("fixture file: %v", err)
	}

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Files != 0 || len(report.Conflicts) != 0 {
		t.Fatalf("an untracked go.sum was read: %+v", report)
	}
}

func TestNoTrackedBuilddirGosumSaysSoAndPasses(t *testing.T) {
	root := tree(t, map[string]string{"go.sum": "example.com/x v1.0.0 " + hashOne + "\n"})

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Files != 0 {
		t.Fatalf("Files is %d over a tree with no builddir", report.Files)
	}
	if !strings.Contains(report.Text(), "nothing to check") {
		t.Errorf("a zero-file run reads as a clean one: %q", report.Text())
	}
}

func TestAMissingRootGosumIsAnError(t *testing.T) {
	root := tree(t, map[string]string{
		"gokrazy/ze/builddir/example.com/y/go.sum": "example.com/x v1.0.0 " + hashTwo + "\n",
	})

	if _, err := Check(root); err == nil {
		t.Fatal("Check answered no error over a tree with no root go.sum")
	}
}

func TestConflictsFollowTheOrderOfTheFile(t *testing.T) {
	root := tree(t, map[string]string{
		"go.sum": "example.com/b v1.0.0 " + hashOne + "\n" +
			"example.com/a v1.0.0 " + hashOne + "\n",
		"gokrazy/ze/builddir/example.com/y/go.sum": "example.com/b v1.0.0 " + hashTwo + "\n" +
			"example.com/a v1.0.0 " + hashTwo + "\n",
	})

	for range 8 {
		report, err := Check(root)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if len(report.Conflicts) != 2 {
			t.Fatalf("want two conflicts, got %+v", report.Conflicts)
		}
		if report.Conflicts[0].Module != "example.com/b" {
			t.Fatalf("the conflicts are not in file order: %+v", report.Conflicts)
		}
	}
}

func TestTextNamesEveryConflictAndTheRemedy(t *testing.T) {
	report := Report{
		Files:  1,
		Shared: 1,
		Conflicts: []Conflict{{
			Path: "gokrazy/ze/builddir/example.com/y/go.sum", Module: "example.com/x",
			Version: "v1.0.0", RootHash: hashOne, BuilddirHash: hashTwo,
		}},
	}

	text := report.Text()
	for _, want := range []string{
		"1 hash conflict(s)", "example.com/x v1.0.0", hashOne, hashTwo,
		"gokrazy/ze/builddir/example.com/y/go.sum", "would refuse",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the page does not carry %q:\n%s", want, text)
		}
	}
	if !strings.HasSuffix(text, "\n") {
		t.Error("the page does not end in a newline")
	}
}

func TestTextCountsWhatItComparedWhenItPasses(t *testing.T) {
	report := Report{Files: 5, Shared: 41}

	text := report.Text()
	if !strings.Contains(text, "5 builddir go.sum file(s)") || !strings.Contains(text, "41 entry/entries") {
		t.Errorf("the passing page hides what was compared: %q", text)
	}
}

func TestReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	raw, err := json.Marshal(Report{
		Files: 1, Shared: 1,
		Conflicts: []Conflict{{Path: "p", Module: "m", Version: "v", RootHash: "r", BuilddirHash: "b"}},
	})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, want := range []string{`"files"`, `"shared"`, `"conflicts"`, `"root-hash"`, `"builddir-hash"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the payload has no %s key: %s", want, raw)
		}
	}
	if strings.Contains(string(raw), "_") {
		t.Errorf("a JSON key is snake_case: %s", raw)
	}
}

func TestAnswerRefusesAnArgument(t *testing.T) {
	payload, code := Answer([]string{"gokrazy/"})
	if payload != nil || code != 1 {
		t.Fatalf("Answer took a path argument: payload=%v code=%d", payload, code)
	}
}

func TestAnswerReportsTheTreeItJudges(t *testing.T) {
	root := tree(t, map[string]string{
		"go.sum":            "example.com/x v1.0.0 " + hashOne + "\n",
		"feature-gates.txt": "ze_core\n",
		"go.mod":            "module example.com/fixture\n",
		"gokrazy/ze/builddir/example.com/y/go.sum": "example.com/x v1.0.0 " + hashTwo + "\n",
	})
	t.Setenv("ZE_REPO_ROOT", root)
	resetRootCache(t)

	payload, code := Answer(nil)
	if code != 1 {
		t.Fatalf("Answer exited %d over a tree holding a conflict", code)
	}
	report, ok := payload.(Report)
	if !ok {
		t.Fatalf("Answer did not answer a Report: %T", payload)
	}
	if len(report.Conflicts) != 1 {
		t.Fatalf("Answer reported %d conflicts", len(report.Conflicts))
	}
}
