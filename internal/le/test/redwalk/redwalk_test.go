package testredwalk

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// brokenSource is the file every walk below breaks, and breakPatch its break:
// `return 1` becomes `return 2`, as `git diff` writes it.
const (
	brokenRel    = "pkg/a.go"
	brokenSource = "package pkg\n\nfunc Value() int {\n\treturn 1\n}\n"
	breakPatch   = "diff --git a/pkg/a.go b/pkg/a.go\n" +
		"index 1111111..2222222 100644\n" +
		"--- a/pkg/a.go\n" +
		"+++ b/pkg/a.go\n" +
		"@@ -3,3 +3,3 @@\n" +
		" func Value() int {\n" +
		"-\treturn 1\n" +
		"+\treturn 2\n" +
		" }\n"
)

// allTracked is the git answer for a tree whose every file is tracked.
func allTracked(string, []string) ([]string, error) { return nil, nil }

// newTree answers a checkout holding brokenSource, with a session identity so
// the overlay is staged in its scratch.
func newTree(t *testing.T) string {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "redwalk-test-sid")
	tree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tree, "pkg"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, brokenRel), []byte(brokenSource), 0o600); err != nil {
		t.Fatal(err)
	}
	return tree
}

// run is one canned carrier run.
type run struct {
	passed bool
	output string
	err    error
}

// fakeCarrier answers the clean run, then the broken one, and records the
// overlay each was handed and the replacement the overlay selected.
type fakeCarrier struct {
	t        *testing.T
	tree     string
	clean    run
	broken   run
	overlays []string
	replaced string
}

func (f *fakeCarrier) run(overlayPath string) (bool, string, error) {
	f.overlays = append(f.overlays, overlayPath)
	if overlayPath == "" {
		return f.clean.passed, f.clean.output, f.clean.err
	}
	raw, err := os.ReadFile(overlayPath)
	if err != nil {
		f.t.Fatalf("the broken run's overlay cannot be read: %v", err)
	}
	var parsed struct {
		Replace map[string]string `json:"Replace"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		f.t.Fatalf("the overlay is not Go's overlay JSON: %v", err)
	}
	replacement, held := parsed.Replace[filepath.Join(f.tree, brokenRel)]
	if !held {
		f.t.Fatalf("the overlay does not replace %s: %s", brokenRel, raw)
	}
	content, err := os.ReadFile(replacement)
	if err != nil {
		f.t.Fatalf("the replacement cannot be read: %v", err)
	}
	f.replaced = string(content)
	return f.broken.passed, f.broken.output, f.broken.err
}

// TestRedwalkCheckUnitRoute proves AC-10 to AC-12 on the unit carrier: a red
// of the unit's own `--- FAIL:` line exits 0 with both lines and the overlay
// list; a green, a clean red, a build failure and a sibling's red each exit 1
// with the verdict that names which. Method: the break is parsed from a real
// patch over a temporary checkout, and the carrier is a fake whose canned
// `go test -v` output is what the real runner prints.
func TestRedwalkCheckUnitRoute(t *testing.T) {
	pass := run{passed: true, output: "=== RUN   TestValue\n--- PASS: TestValue (0.00s)\nPASS\nok  \tpkg\t0.01s\n"}
	cases := []struct {
		name    string
		clean   run
		broken  run
		verdict string
		code    int
		runs    int
		clean2  string
		broken2 string
		reason  string
	}{
		{
			name:  "red of the unit (AC-10)",
			clean: pass,
			broken: run{output: "=== RUN   TestValue\n    a_test.go:9: got 2, want 1\n" +
				"--- FAIL: TestValue (0.00s)\nFAIL\nFAIL\tpkg\t0.01s\n"},
			verdict: "red", code: 0, runs: 2,
			clean2: "--- PASS: TestValue (0.00s)", broken2: "--- FAIL: TestValue (0.00s)",
		},
		{
			name: "stays green (AC-11)", clean: pass, broken: pass,
			verdict: "green", code: 1, runs: 2, broken2: "--- PASS: TestValue (0.00s)",
			reason: "does not discriminate",
		},
		{
			name:    "clean red is no baseline (AC-12)",
			clean:   run{output: "=== RUN   TestValue\n--- FAIL: TestValue (0.00s)\nFAIL\n"},
			verdict: "no-baseline", code: 1, runs: 1, reason: "already red before the break",
		},
		{
			name:    "a unit that did not run is no baseline",
			clean:   run{passed: true, output: "testing: warning: no tests to run\nPASS\n"},
			verdict: "no-baseline", code: 1, runs: 1, reason: "no test of that name ran",
		},
		{
			name:  "compile failure is not a red (AC-12)",
			clean: pass,
			broken: run{output: "# pkg\npkg/a.go:4:9: cannot use x\n" +
				"FAIL\tpkg [build failed]\nFAIL\n"},
			verdict: "not-a-red", code: 1, runs: 2, reason: "does not compile",
		},
		{
			name:  "a sibling's red is not the unit's",
			clean: pass,
			broken: run{output: "=== RUN   TestValueSibling\n--- FAIL: TestValueSibling (0.00s)\n" +
				"FAIL\n"},
			verdict: "not-a-red", code: 1, runs: 2, reason: "without a `--- FAIL: TestValue` line",
		},
	}
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			tree := newTree(t)
			files, err := parseBreak(tree, []byte(breakPatch), allTracked)
			if err != nil {
				t.Fatalf("parseBreak: %v", err)
			}
			chosen, err := unitTarget(tree, "TestValue", "./pkg")
			if err != nil {
				t.Fatalf("unitTarget: %v", err)
			}
			carrier := &fakeCarrier{t: t, tree: tree, clean: tc.clean, broken: tc.broken}
			report, err := (&walk{tree: tree, target: chosen, files: files, run: carrier.run}).check()
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			if report.Verdict != tc.verdict {
				t.Fatalf("verdict %q, want %q: %+v", report.Verdict, tc.verdict, report)
			}
			if report.Code() != tc.code {
				t.Errorf("code %d, want %d", report.Code(), tc.code)
			}
			if len(carrier.overlays) != tc.runs {
				t.Fatalf("%d runs, want %d", len(carrier.overlays), tc.runs)
			}
			if carrier.overlays[0] != "" {
				t.Errorf("the clean run was handed an overlay: %s", carrier.overlays[0])
			}
			if tc.clean2 != "" {
				if report.Clean != tc.clean2 {
					t.Errorf("clean line %q, want %q", report.Clean, tc.clean2)
				}
			}
			if tc.broken2 != "" {
				if report.Broken != tc.broken2 {
					t.Errorf("broken line %q, want %q", report.Broken, tc.broken2)
				}
			}
			if !strings.Contains(report.Reason, tc.reason) {
				t.Errorf("reason %q does not say %q", report.Reason, tc.reason)
			}
			if tc.code != 0 {
				if report.Excerpt == "" {
					t.Error("a refused walk carries no excerpt of the run")
				}
				if report.Fix == "" {
					t.Error("a refused walk names no fix")
				}
			}
			if len(report.Overlay) != 1 || report.Overlay[0] != brokenRel {
				t.Errorf("overlay list %v, want [%s]", report.Overlay, brokenRel)
			}
			if tc.runs < 2 {
				return
			}
			if !strings.Contains(carrier.replaced, "\treturn 2\n") {
				t.Errorf("the broken run did not compile the break: %q", carrier.replaced)
			}
			if _, err := os.Stat(filepath.Dir(carrier.overlays[1])); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("the overlay directory outlived the walk: %v", err)
			}
			if tc.code != 0 {
				return
			}
			text := report.Text()
			for _, want := range []string{tc.clean2, tc.broken2, brokenRel, ": red"} {
				if !strings.Contains(text, want) {
					t.Errorf("the text does not show %q:\n%s", want, text)
				}
			}
		})
	}
}

// TestRedwalkInteropCountsOnlyTheChecker proves AC-13: a scenario's red counts
// only when its FAIL line says `in checker`, and a .ci's red is its runner's
// FAIL result. Method: canned progress and result lines, judged by the moved
// verdict readers through check.
func TestRedwalkInteropCountsOnlyTheChecker(t *testing.T) {
	scenario := target{kind: kindScenario, subject: "bgp/route-a", name: "route-a"}
	ci := target{kind: kindCI, subject: "parse/conf-a", name: "conf-a"}
	cases := []struct {
		name    string
		target  target
		clean   run
		broken  run
		verdict string
		reason  string
	}{
		{
			name: "checker red", target: scenario,
			clean:   run{passed: true, output: "interop: [1/1] route-a: pass (3s)\n"},
			broken:  run{output: "interop: [1/1] route-a: FAIL (4s) in checker: scenario route-a assertion 2: no route\n"},
			verdict: "red",
		},
		{
			name: "setup red refused", target: scenario,
			clean:   run{passed: true, output: "interop: [1/1] route-a: pass (3s)\n"},
			broken:  run{output: "interop: [1/1] route-a: FAIL (1s) in setup: image build failed\n"},
			verdict: "not-a-red", reason: "failed in setup",
		},
		{
			name: "readiness red refused", target: scenario,
			clean:   run{passed: true, output: "interop: [1/1] route-a: pass (3s)\n"},
			broken:  run{output: "interop: [1/1] route-a: FAIL (1s) in readiness: peer never answered\n"},
			verdict: "not-a-red", reason: "failed in readiness",
		},
		{
			name: "scenario that never ran clean", target: scenario,
			clean:   run{passed: true, output: "interop: 0 passed, 0 failed\n"},
			verdict: "no-baseline", reason: "did not run in the clean run",
		},
		{
			name: "ci red", target: ci,
			clean:   run{passed: true, output: "  0.4s  1/1  PASS  a1  conf-a\n"},
			broken:  run{output: "  0.4s  1/1  FAIL  a1  conf-a\n"},
			verdict: "red",
		},
		{
			name: "ci skipped clean", target: ci,
			clean:   run{passed: true, output: "  0.0s  1/1  SKIP  a1  conf-a\n"},
			verdict: "no-baseline", reason: "did not pass in the clean run",
		},
		{
			name: "ci build failure", target: ci,
			clean:   run{passed: true, output: "  0.4s  1/1  PASS  a1  conf-a\n"},
			broken:  run{err: &buildError{err: errors.New("cannot build the isolated binaries")}},
			verdict: "not-a-red", reason: "does not compile",
		},
	}
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			tree := newTree(t)
			files, err := parseBreak(tree, []byte(breakPatch), allTracked)
			if err != nil {
				t.Fatalf("parseBreak: %v", err)
			}
			carrier := &fakeCarrier{t: t, tree: tree, clean: tc.clean, broken: tc.broken}
			report, err := (&walk{tree: tree, target: tc.target, files: files, run: carrier.run}).check()
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			if report.Verdict != tc.verdict {
				t.Fatalf("verdict %q, want %q: %+v", report.Verdict, tc.verdict, report)
			}
			if !strings.Contains(report.Reason, tc.reason) {
				t.Errorf("reason %q does not say %q", report.Reason, tc.reason)
			}
		})
	}
}

// TestRedwalkLeavesTheTreeUntouched proves AC-10 and R-7: while the broken
// run runs, and after it is killed, every file of the checkout outside its
// scratch holds the bytes it held before. Method: hash the tree, hash it again
// from inside the broken run, which then fails as a killed child does, and
// once more after the walk.
func TestRedwalkLeavesTheTreeUntouched(t *testing.T) {
	tree := newTree(t)
	before := hashTree(t, tree)
	files, err := parseBreak(tree, []byte(breakPatch), allTracked)
	if err != nil {
		t.Fatalf("parseBreak: %v", err)
	}
	chosen, err := unitTarget(tree, "TestValue", "pkg")
	if err != nil {
		t.Fatalf("unitTarget: %v", err)
	}
	var during map[string][32]byte
	killed := func(overlayPath string) (bool, string, error) {
		if overlayPath == "" {
			return true, "--- PASS: TestValue (0.00s)\n", nil
		}
		during = hashTree(t, tree)
		return false, "=== RUN   TestValue\nsignal: killed\n", errors.New("the run was killed")
	}
	report, err := (&walk{tree: tree, target: chosen, files: files, run: killed}).check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if report.Verdict != "unobserved" {
		t.Errorf("a killed run gave verdict %q, want unobserved", report.Verdict)
	}
	sameTree(t, "while the broken run ran", before, during)
	sameTree(t, "after the walk", before, hashTree(t, tree))
}

// hashTree answers the hash of every file under tree outside its tmp/.
func hashTree(t *testing.T, tree string) map[string][32]byte {
	t.Helper()
	hashes := map[string][32]byte{}
	err := filepath.WalkDir(tree, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(tree, path)
		if entry.IsDir() {
			if rel == "tmp" {
				return filepath.SkipDir
			}
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hashes[rel] = sha256.Sum256(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hashes
}

// sameTree fails when two tree hashes differ.
func sameTree(t *testing.T, when string, want, got map[string][32]byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s the tree holds %d files, before it held %d", when, len(got), len(want))
	}
	for rel, hash := range want {
		if got[rel] != hash {
			t.Errorf("%s %s changed", when, rel)
		}
	}
}

// TestRedwalkRefusesABreakItCannotWalk proves the input refusals: a path that
// leaves the checkout, a symlink out of it, a non-Go file (D2 b), an untracked
// file, a created file, and a hunk that does not match the file.
func TestRedwalkRefusesABreakItCannotWalk(t *testing.T) {
	header := func(rel string) string {
		return "--- a/" + rel + "\n+++ b/" + rel + "\n@@ -1 +1 @@\n-old\n+new\n"
	}
	cases := []struct {
		name      string
		patch     string
		untracked untrackedFunc
		want      string
	}{
		{name: "path escape", patch: header("../outside.go"), want: "leaves the checkout"},
		{name: "non-Go file", patch: header("test/parse/conf-a.ci"), want: "cannot reach this file"},
		{name: "go.mod", patch: header("go.mod"), want: "not a Go file"},
		{name: "symlink escape", patch: header("link.go"), want: "through a symlink"},
		{
			name: "untracked file", patch: breakPatch, want: "git does not track",
			untracked: func(string, []string) ([]string, error) { return []string{brokenRel}, nil },
		},
		{
			name:  "created file",
			patch: "--- /dev/null\n+++ b/pkg/new.go\n@@ -0,0 +1 @@\n+package pkg\n", want: "creates this file",
		},
		{
			name:  "mismatch",
			patch: "--- a/pkg/a.go\n+++ b/pkg/a.go\n@@ -4 +4 @@\n-\treturn 7\n+\treturn 2\n",
			want:  "line 4 reads",
		},
		{name: "no change", patch: "diff --git a/x b/x\n", want: "holds no file change"},
	}
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			tree := newTree(t)
			outside := filepath.Join(t.TempDir(), "outside.go")
			if err := os.WriteFile(outside, []byte("old\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(tree, "link.go")); err != nil {
				t.Fatal(err)
			}
			untracked := tc.untracked
			if untracked == nil {
				untracked = allTracked
			}
			_, err := parseBreak(tree, []byte(tc.patch), untracked)
			if err == nil {
				t.Fatalf("the break was accepted, want a refusal saying %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("refusal %q does not say %q", err, tc.want)
			}
			if !strings.Contains(err.Error(), "fix: ") {
				t.Errorf("refusal %q names no fix", err)
			}
		})
	}
}

// TestRedwalkAppliesABreakExactly proves the applier on the shapes git writes:
// two hunks in one file, a file with no final newline, and a deletion.
func TestRedwalkAppliesABreakExactly(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		hunks   string
		want    string
		deleted bool
	}{
		{
			name:   "two hunks",
			source: "a\nb\nc\nd\ne\nf\ng\n",
			hunks:  "@@ -1,2 +1,2 @@\n-a\n+A\n b\n@@ -6,2 +6,2 @@\n f\n-g\n+G\n",
			want:   "A\nb\nc\nd\ne\nf\nG\n",
		},
		{
			name:   "no final newline",
			source: "a\nb",
			hunks:  "@@ -1,2 +1,2 @@\n a\n-b\n\\ No newline at end of file\n+B\n\\ No newline at end of file\n",
			want:   "a\nB",
		},
		{
			name:   "newline added at the end",
			source: "a\nb",
			hunks:  "@@ -1,2 +1,2 @@\n a\n-b\n\\ No newline at end of file\n+b\n",
			want:   "a\nb\n",
		},
	}
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			tree := newTree(t)
			if err := os.WriteFile(filepath.Join(tree, brokenRel), []byte(tc.source), 0o600); err != nil {
				t.Fatal(err)
			}
			patch := "--- a/pkg/a.go\n+++ b/pkg/a.go\n" + tc.hunks
			files, err := parseBreak(tree, []byte(patch), allTracked)
			if err != nil {
				t.Fatalf("parseBreak: %v", err)
			}
			if files[0].Content != tc.want {
				t.Errorf("content %q, want %q", files[0].Content, tc.want)
			}
		})
	}
	t.Run("deletion", func(t *testing.T) {
		tree := newTree(t)
		patch := "--- a/pkg/a.go\n+++ /dev/null\n@@ -1,5 +0,0 @@\n-package pkg\n-\n-func Value() int {\n" +
			"-\treturn 1\n-}\n"
		files, err := parseBreak(tree, []byte(patch), allTracked)
		if err != nil {
			t.Fatalf("parseBreak: %v", err)
		}
		if !files[0].Absent {
			t.Errorf("a deleted file is not marked absent: %+v", files[0])
		}
	})
}
