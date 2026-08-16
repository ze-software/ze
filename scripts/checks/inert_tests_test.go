// Tests for the //go:build ignore checker inert_tests.go.
//
// The ratchet is a fail-closed guard, so ai/rules/evidence.md requires
// driving it from its ENTRY POINT, not from its helpers: every test here runs
// `go run scripts/checks/inert_tests.go` as a subprocess against a fixture tree
// supplied with --root. The live repository cannot be doctored to prove the
// ratchet fires (the test-deletion hook rightly refuses to remove a probe test),
// which is exactly why the entry point takes a root.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// escapeHint is the annotation the gate must suggest when it reports an
// assert-nothing test. Duplicated from inert_tests.go on purpose: that file is
// //go:build ignore, so this package cannot import its constants, and a silent
// rename of the annotation should fail this test.
const escapeHint = "test-asserts-nothing:"

// fixtureTree builds a minimal repository the gate can scan: the make wiring it
// derives the tag universe from, plus whatever test files the case needs.
func fixtureTree(t *testing.T, baselineJSON string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	write := func(rel, content string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("feature-gates.txt", "# manifest\nze_web\tinternal/component/web\nze_ssh\tinternal/component/ssh\n")
	write("Makefile", strings.Join([]string{
		"ZE_FEATURES := $(shell awk '$$1 ~ /^ze_/ {print $$1}' feature-gates.txt)",
		"GO_TEST_TAGS = ze_core $(ZE_FEATURES)",
		"ze-unit-test:",
		"\tgo test -tags '$(GO_TEST_TAGS)' ./...",
		"",
	}, "\n"))
	if baselineJSON != "" {
		write("test/health/sensitivity-baseline.json", baselineJSON)
	}
	// Every test root must exist: the gate treats a missing root as a failure
	// rather than an empty one, because a silently shrunken scan is a passing
	// scan (and `make ze-test-health-update` would then bake the small number into the
	// floor). TestMissingTestRootFailsClosed removes one on purpose.
	for _, r := range []string{"internal", "cmd", "pkg", "scripts", "test"} {
		if err := os.MkdirAll(filepath.Join(dir, r), 0o750); err != nil {
			t.Fatalf("mkdir root %s: %v", r, err)
		}
	}
	for rel, content := range files {
		write(rel, content)
	}
	return dir
}

// runGate drives the real entry point and returns stdout+stderr combined (for
// message assertions) plus the exit code.
func runGate(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	stdout, stderr, code := runGateSplit(t, root, args...)
	return stdout + stderr, code
}

// runGateSplit keeps the streams apart, because the gate writes its JSON report
// to stdout and its ratchet diagnostics to stderr. Parsing the combined stream
// as JSON fails the moment the ratchet has anything to say -- which is exactly
// the case the --json --check tests exercise.
func runGateSplit(t *testing.T, root string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	full := append([]string{"run", "scripts/checks/inert_tests.go", "--root=" + root}, args...)
	cmd := exec.CommandContext(ctx, "go", full...)
	cmd.Dir = repoRoot(t)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err == nil {
		return outBuf.String(), errBuf.String(), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run gate: %v\nstdout:\n%s\nstderr:\n%s", err, outBuf.String(), errBuf.String())
	}
	return outBuf.String(), errBuf.String(), exitErr.ExitCode()
}

// gateJSON runs the gate and decodes its stdout report into v.
func gateJSON(t *testing.T, v any, root string, args ...string) (string, int) {
	t.Helper()
	stdout, stderr, code := runGateSplit(t, root, args...)
	if err := json.Unmarshal([]byte(stdout), v); err != nil {
		t.Fatalf("parse gate JSON: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	return stderr, code
}

// TestInertTestsSelftest runs the checker's own --selftest, which proves both
// AST detectors fire on known-bad fixtures before they judge any tree.
//
// VALIDATES: the detectors are wired and discriminating (spec AC-7, AC-8, AC-9).
// PREVENTS: a broken detector silently reporting a clean tree forever.
func TestInertTestsSelftest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/inert_tests.go", "--selftest")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("inert-tests selftest failed:\n%s", out)
	}
	if !strings.Contains(string(out), "selftest OK") {
		t.Fatalf("selftest did not report OK:\n%s", out)
	}
}

// TestInertTestsRatchetFailsOnRegression is the gate's reason to exist: a newly
// added test that cannot fail must break the build.
//
// VALIDATES: spec AC-10 -- count above baseline exits non-zero and names the file.
// PREVENTS: the ratchet passing because it only ever counted, never compared.
func TestInertTestsRatchetFailsOnRegression(t *testing.T) {
	root := fixtureTree(t, `{"assert-nothing": 0, "tag-orphan": 0}`, map[string]string{
		"internal/example/inert_test.go": `package example

import "testing"

func TestInert(t *testing.T) {
	_ = 1 + 1
}
`,
	})
	out, code := runGate(t, root, "--check")
	if code == 0 {
		t.Fatalf("ratchet passed with an assert-nothing test above baseline:\n%s", out)
	}
	if !strings.Contains(out, "TestInert") {
		t.Errorf("failure output does not name the offending test:\n%s", out)
	}
	if !strings.Contains(out, "internal/example/inert_test.go") {
		t.Errorf("failure output does not name the offending file:\n%s", out)
	}
	if !strings.Contains(out, escapeHint) {
		t.Errorf("failure output does not tell the developer how to resolve it:\n%s", out)
	}
}

// TestInertTestsRatchetPassesAtBaseline pins the other side of the ratchet: a
// count equal to the floor is recorded debt, not a regression.
//
// VALIDATES: spec AC-11.
// PREVENTS: a gate that blocks every commit because it treats debt as failure.
func TestInertTestsRatchetPassesAtBaseline(t *testing.T) {
	root := fixtureTree(t, `{"assert-nothing": 1, "tag-orphan": 0}`, map[string]string{
		"internal/example/inert_test.go": `package example

import "testing"

func TestInert(t *testing.T) {
	_ = 1 + 1
}
`,
	})
	out, code := runGate(t, root, "--check")
	if code != 0 {
		t.Fatalf("ratchet failed at the baseline it was given:\n%s", out)
	}
}

// TestCrossPackageAssertHelperCredits drives the WHOLE gate over a fixture tree
// whose test asserts only through a helper in another first-party package.
//
// The selftest cannot stand in for this. It calls the judging helper directly,
// so it stays green even when scanTree forgets to hand canFail the file's
// imports or the package index, which is the wiring that makes the follow reach
// the live tree at all.
//
// VALIDATES: a shared assert helper (`markupcheck.AssertNoMarkup(t, ...)`,
// `golden.AssertPortFidelity(t, ...)`) counts as the caller's assertion.
// PREVENTS: the gate condemning nine live tests that do assert, which is what it
// did until the follow crossed the package boundary.
func TestCrossPackageAssertHelperCredits(t *testing.T) {
	root := fixtureTree(t, `{"assert-nothing": 0, "tag-orphan": 0}`, map[string]string{
		"go.mod": "module example.test\n\ngo 1.26\n",
		"internal/check/check.go": `package check

import "testing"

func AssertIt(t *testing.T, got int) {
	if got != 1 {
		t.Fatalf("got %d", got)
	}
}
`,
		"internal/example/caller_test.go": `package example

import (
	"testing"

	"example.test/internal/check"
)

func TestCaller(t *testing.T) {
	check.AssertIt(t, 1)
}
`,
	})
	out, code := runGate(t, root, "--check")
	if code != 0 {
		t.Fatalf("a test asserting through a helper in another package was called inert:\n%s", out)
	}
}

// TestCrossPackageHelperThatCannotFailIsNotCredited is the other half. Following
// a helper must credit its BODY, never the mere fact that a *testing.T was
// handed over: a fixture builder takes one and asserts nothing.
//
// VALIDATES: the follow narrows nothing the gate used to catch.
// PREVENTS: `anything.Build(t)` becoming a blanket pardon.
func TestCrossPackageHelperThatCannotFailIsNotCredited(t *testing.T) {
	root := fixtureTree(t, `{"assert-nothing": 0, "tag-orphan": 0}`, map[string]string{
		"go.mod": "module example.test\n\ngo 1.26\n",
		"internal/check/build.go": `package check

import "testing"

func Build(t *testing.T) string { return t.TempDir() }
`,
		"internal/example/caller_test.go": `package example

import (
	"testing"

	"example.test/internal/check"
)

func TestCaller(t *testing.T) {
	_ = check.Build(t)
}
`,
	})
	out, code := runGate(t, root, "--check")
	if code == 0 {
		t.Fatalf("a test whose helper cannot fail was credited anyway:\n%s", out)
	}
	if !strings.Contains(out, "TestCaller") {
		t.Errorf("the report does not name the test it condemned:\n%s", out)
	}
}

// TestAssertNothingEscapeComment proves the documented annotation suppresses a
// finding, so a genuine "must not panic" test is not forced to fake an assertion.
//
// VALIDATES: spec AC-8.
// PREVENTS: developers deleting real smoke tests to satisfy the gate.
func TestAssertNothingEscapeComment(t *testing.T) {
	root := fixtureTree(t, `{"assert-nothing": 0, "tag-orphan": 0}`, map[string]string{
		"internal/example/smoke_test.go": `package example

import "testing"

// test-asserts-nothing: the oracle is the absence of a panic
func TestSmoke(t *testing.T) {
	_ = 1 + 1
}
`,
	})
	out, code := runGate(t, root, "--check")
	if code != 0 {
		t.Fatalf("annotated test was still counted:\n%s", out)
	}
}

// TestTagOrphanDetection proves an unreachable build tag is reported, and that
// the negated and compile-out constraints the repo relies on are NOT.
//
// VALIDATES: spec AC-9.
// PREVENTS: the single-evaluation bug that condemns every `!tag` stub file --
// the real defect found while building this gate.
func TestTagOrphanDetection(t *testing.T) {
	root := fixtureTree(t, "", map[string]string{
		// Unreachable: no go test invocation supplies ze_nowhere.
		"internal/example/orphan_test.go": "//go:build ze_nowhere\n\npackage example\n\nimport \"testing\"\n\nfunc TestOrphan(t *testing.T) { t.Fatal(\"x\") }\n",
		// Reachable: ze_web comes from the feature manifest.
		"internal/example/gated_test.go": "//go:build ze_web\n\npackage example\n\nimport \"testing\"\n\nfunc TestGated(t *testing.T) { t.Fatal(\"x\") }\n",
		// Reachable: a GOOS stub builds on some platform.
		"internal/example/other_test.go": "//go:build !linux\n\npackage example\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) { t.Fatal(\"x\") }\n",
		// Reachable: the compile-out check runs with the feature tag off.
		"internal/example/absent_test.go": "//go:build ze_core && !ze_web\n\npackage example\n\nimport \"testing\"\n\nfunc TestAbsent(t *testing.T) { t.Fatal(\"x\") }\n",
	})
	out, code := runGate(t, root, "--json")
	if code != 0 {
		t.Fatalf("gate errored:\n%s", out)
	}
	var res struct {
		TagOrphan []struct {
			File   string `json:"file"`
			Detail string `json:"detail"`
		} `json:"tag-orphan"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("parse gate JSON: %v\n%s", err, out)
	}
	if len(res.TagOrphan) != 1 {
		t.Fatalf("expected exactly 1 tag-orphan, got %d: %+v", len(res.TagOrphan), res.TagOrphan)
	}
	if !strings.Contains(res.TagOrphan[0].File, "orphan_test.go") {
		t.Errorf("wrong file reported as orphan: %s", res.TagOrphan[0].File)
	}
	if res.TagOrphan[0].Detail != "ze_nowhere" {
		t.Errorf("orphan detail should name the unreachable tag, got %q", res.TagOrphan[0].Detail)
	}
}

// TestInertTestsFailsClosedOnEmptyScan pins the fail-closed contract: a scan
// that finds nothing is a broken scan, never a clean tree.
//
// VALIDATES: ai/rules/evidence.md -- no permissive value on a miss.
// PREVENTS: a path or glob regression turning the gate into a no-op that passes.
func TestInertTestsFailsClosedOnEmptyScan(t *testing.T) {
	root := fixtureTree(t, `{"assert-nothing": 0, "tag-orphan": 0}`, nil)
	out, code := runGate(t, root, "--check")
	if code == 0 {
		t.Fatalf("gate reported success on a tree with no test files:\n%s", out)
	}
	if !strings.Contains(out, "refusing to report a clean tree") {
		t.Errorf("gate did not explain the fail-closed refusal:\n%s", out)
	}
}

// TestInertTestsMissingBaselineFailsClosed proves a deleted or unreadable
// baseline stops the build rather than defaulting to an unlimited floor.
//
// VALIDATES: ai/rules/evidence.md.
// PREVENTS: deleting the baseline file as a way to silence the ratchet.
func TestInertTestsMissingBaselineFailsClosed(t *testing.T) {
	root := fixtureTree(t, "", map[string]string{
		"internal/example/inert_test.go": "package example\n\nimport \"testing\"\n\nfunc TestInert(t *testing.T) { _ = 1 }\n",
	})
	out, code := runGate(t, root, "--check")
	if code == 0 {
		t.Fatalf("gate passed with no baseline file:\n%s", out)
	}
	if !strings.Contains(out, "baseline") {
		t.Errorf("failure does not mention the missing baseline:\n%s", out)
	}
}

// gatedTestFile is a build-constrained test whose body does assert, used where
// the case under test is the CONSTRAINT rather than the assertions.
func gatedTestFile(tag string) string {
	return "//go:build " + tag + `

package example

import "testing"

func TestGated(t *testing.T) {
	t.Fatal("x")
}
`
}

// TestTagUniverseRequiresAGoTestReference is the regression this gate exists
// for, and the previous version of it could not fail.
//
// The feature manifest used to be poured straight into the universe, so every
// declared tag counted as reachable whether or not any `go test` line passed it.
// Dropping $(ZE_FEATURES) from GO_TEST_TAGS would then have stranded every
// feature-gated test while the gate still reported zero orphans: fail-open on
// precisely the regression it guards.
//
// VALIDATES: a tag is reachable only when a `go test -tags` line references it.
// PREVENTS: a manifest entry masquerading as test reachability.
func TestTagUniverseRequiresAGoTestReference(t *testing.T) {
	root := fixtureTree(t, "", map[string]string{
		"internal/example/gated_test.go": gatedTestFile("ze_web"),
	})
	// Rewrite the fixture Makefile so no `go test` invocation carries
	// $(ZE_FEATURES). ze_web stays declared in feature-gates.txt, so a
	// manifest-seeded universe would still call it reachable.
	makefile := strings.Join([]string{
		"ZE_FEATURES := $(shell awk '$$1 ~ /^ze_/ {print $$1}' feature-gates.txt)",
		"GO_TEST_TAGS = ze_core",
		"ze-unit-test:",
		"\tgo test -tags '$(GO_TEST_TAGS)' ./...",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(makefile), 0o600); err != nil {
		t.Fatalf("rewrite fixture Makefile: %v", err)
	}

	out, code := runGate(t, root, "--json")
	if code != 0 {
		t.Fatalf("gate errored:\n%s", out)
	}
	var res struct {
		TagUniverse []string `json:"test-tag-universe"`
		TagOrphan   []struct {
			File string `json:"file"`
		} `json:"tag-orphan"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("parse gate JSON: %v\n%s", err, out)
	}
	for _, tag := range res.TagUniverse {
		if tag == "ze_web" {
			t.Fatalf("ze_web reached the universe with no go test reference; universe=%v", res.TagUniverse)
		}
	}
	if len(res.TagOrphan) != 1 {
		t.Fatalf("expected the ze_web test to be an orphan, got %d orphans", len(res.TagOrphan))
	}
}

// TestJSONCheckStillEnforcesTheRatchet pins the combination that used to be a
// silent no-op: the JSON branch keyed its exit on a `Valid` field that was set
// unconditionally to true, so `--json --check` printed findings and exited 0.
//
// VALIDATES: spec AC-10 holds in every output mode.
// PREVENTS: a guard whose only enforcement path cannot deny.
func TestJSONCheckStillEnforcesTheRatchet(t *testing.T) {
	root := fixtureTree(t, `{"assert-nothing": 0, "tag-orphan": 0}`, map[string]string{
		"internal/example/inert_test.go": inertTestFile,
	})
	var res struct {
		Valid bool `json:"valid"`
	}
	stderr, code := gateJSON(t, &res, root, "--json", "--check")
	if code == 0 {
		t.Fatalf("--json --check exited 0 with a finding above baseline:\n%s", stderr)
	}
	if res.Valid {
		t.Error(`JSON reported "valid": true while the ratchet was breached`)
	}
}

// inertTestFile is a test with no failure path: the canonical finding.
const inertTestFile = `package example

import "testing"

func TestInert(t *testing.T) {
	_ = 1 + 1
}
`

// TestUnknownArgumentIsRejected stops a typo from silently demoting the gate to
// report-only, which exits 0 regardless of findings.
//
// VALIDATES: ai/rules/evidence.md.
// PREVENTS: a mistyped flag in a Makefile recipe disabling the ratchet forever.
func TestUnknownArgumentIsRejected(t *testing.T) {
	root := fixtureTree(t, `{"assert-nothing": 0, "tag-orphan": 0}`, map[string]string{
		"internal/example/inert_test.go": inertTestFile,
	})
	out, code := runGate(t, root, "--chek")
	if code == 0 {
		t.Fatalf("gate accepted an unknown argument and exited 0:\n%s", out)
	}
	if !strings.Contains(out, "unknown argument") {
		t.Errorf("gate did not name the bad argument:\n%s", out)
	}
}

// TestMissingTestRootFailsClosed stops a shrunken scan from being accepted, and
// from being baked into the ratchet floor by the next `make ze-test-health-update`.
//
// VALIDATES: ai/rules/evidence.md.
// PREVENTS: an unreadable internal/ yielding a small, passing count.
func TestMissingTestRootFailsClosed(t *testing.T) {
	root := fixtureTree(t, `{"assert-nothing": 0, "tag-orphan": 0}`, map[string]string{
		"internal/example/ok_test.go": gatedTestFile("linux"),
	})
	// Remove one configured root, as an unreadable or renamed tree would be.
	if err := os.RemoveAll(filepath.Join(root, "pkg")); err != nil {
		t.Fatalf("remove fixture root: %v", err)
	}
	out, code := runGate(t, root, "--check")
	if code == 0 {
		t.Fatalf("gate passed with test roots missing:\n%s", out)
	}
	if !strings.Contains(out, "missing or unreadable") {
		t.Errorf("gate did not explain which root was missing:\n%s", out)
	}
}

// TestSameNameHelperAcrossPackagesIsDeterministic pins the fix for a genuine
// flake: `foo` and `foo_test` may each declare `func check(...)`, and flattening
// both into one map made the verdict depend on Go's randomized map iteration, so
// the reported count varied between runs on an unchanged tree.
//
// VALIDATES: spec AC-2 -- a ratchet count must be reproducible.
// PREVENTS: a gate that fires spuriously in CI and cannot be diagnosed.
func TestSameNameHelperAcrossPackagesIsDeterministic(t *testing.T) {
	root := fixtureTree(t, "", map[string]string{
		"internal/example/i_test.go": `package example

import "testing"

func check(t *testing.T) { _ = 1 }

func TestInternal(t *testing.T) { check(t) }
`,
		"internal/example/j_test.go": `package example_test

import "testing"

func check(t *testing.T) { t.Fatal("boom") }

func TestExternal(t *testing.T) { check(t) }
`,
	})

	var first []string
	for run := range 6 {
		out, code := runGate(t, root, "--json")
		if code != 0 {
			t.Fatalf("gate errored on run %d:\n%s", run, out)
		}
		var res struct {
			AssertNothing []struct {
				Test string `json:"test"`
			} `json:"assert-nothing"`
		}
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("parse gate JSON: %v\n%s", err, out)
		}
		got := make([]string, 0, len(res.AssertNothing))
		for _, f := range res.AssertNothing {
			got = append(got, f.Test)
		}
		sort.Strings(got)
		if run == 0 {
			first = got
			continue
		}
		if strings.Join(got, ",") != strings.Join(first, ",") {
			t.Fatalf("findings differ between runs on an unchanged tree: run0=%v run%d=%v", first, run, got)
		}
	}
	// TestInternal's helper has no failure path; TestExternal's does. Only the
	// former may be reported, and per-package indexing is what makes that stable.
	if len(first) != 1 || first[0] != "TestInternal" {
		t.Errorf("expected exactly TestInternal to be inert, got %v", first)
	}
}

// TestTrackedOnlyIgnoresUntrackedFiles proves the page's population is
// reproducible from a clean checkout while the ratchet still guards the tree
// about to be committed.
//
// VALIDATES: spec AC-2.
// PREVENTS: a developer's untracked scratch test moving the published numbers,
// so a clean CI checkout disagrees with the committed page.
func TestTrackedOnlyIgnoresUntrackedFiles(t *testing.T) {
	root := fixtureTree(t, "", map[string]string{
		"internal/example/tracked_test.go": inertTestFile,
	})
	gitInit(t, root)

	untracked := filepath.Join(root, "internal", "example", "scratch_test.go")
	body := `package example

import "testing"

func TestScratch(t *testing.T) {
	_ = 2
}
`
	if err := os.WriteFile(untracked, []byte(body), 0o600); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	countFor := func(args ...string) int {
		out, code := runGate(t, root, args...)
		if code != 0 {
			t.Fatalf("gate errored (%v):\n%s", args, out)
		}
		var res struct {
			AssertNothing []struct{} `json:"assert-nothing"`
		}
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("parse gate JSON: %v\n%s", err, out)
		}
		return len(res.AssertNothing)
	}

	if got := countFor("--json"); got != 2 {
		t.Errorf("working-tree scan should see the untracked test, got %d, want 2", got)
	}
	if got := countFor("--json", "--tracked-only"); got != 1 {
		t.Errorf("tracked-only scan should ignore the untracked test, got %d, want 1", got)
	}
}

func gitInit(t *testing.T, root string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "T"},
		{"add", "-A"},
		{"-c", "commit.gpgsign=false", "commit", "-q", "-m", "fixture"},
	} {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// TestInertTestsDerivesTagUniverse proves the universe comes from the make
// wiring rather than a hardcoded list, so a new gated feature cannot silently
// orphan its own tests.
//
// VALIDATES: ai/rules/evidence.md.
// PREVENTS: a stale built-in tag list drifting from feature-gates.txt.
func TestInertTestsDerivesTagUniverse(t *testing.T) {
	root := fixtureTree(t, "", map[string]string{
		"internal/example/gated_test.go": "//go:build ze_ssh\n\npackage example\n\nimport \"testing\"\n\nfunc TestGated(t *testing.T) { t.Fatal(\"x\") }\n",
	})
	out, code := runGate(t, root, "--json")
	if code != 0 {
		t.Fatalf("gate errored:\n%s", out)
	}
	var res struct {
		TagUniverse []string `json:"test-tag-universe"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("parse gate JSON: %v\n%s", err, out)
	}
	want := map[string]bool{"ze_core": true, "ze_web": true, "ze_ssh": true}
	for _, tag := range res.TagUniverse {
		delete(want, tag)
	}
	if len(want) != 0 {
		t.Errorf("tag universe %v missing tags derived from the fixture make wiring: %v", res.TagUniverse, want)
	}
}
