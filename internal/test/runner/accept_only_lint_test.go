package runner

// VALIDATES: the accept-only (weak) .ci predicate + ratchet baseline (spec
//   fixit-ci-accept-only-tests AC-4). A .ci whose ONLY assertion is
//   expect=exit:code=0 proves a config was ACCEPTED, never that it parsed to the
//   CORRECT tree; this lint flags a NEW one so the class cannot grow silently.
// PREVENTS: a new exit-code-only functional test being added without a readback
//   assertion or an accept-only annotation, and the baseline drifting stale.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRootForTest returns the repository root from this test file's location
// (<root>/internal/test/runner/accept_only_lint_test.go -> <root>).
func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// parseCI writes content to dir/name and returns the parsed Record.
func parseCI(t *testing.T, dir, name, content string) *Record {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	rec, err := NewEncodingTests(dir).parseAndAdd(p)
	if err != nil {
		t.Fatalf("parseAndAdd %s: %v", p, err)
	}
	return rec
}

// tempRepoWithCI builds a temp repo root with a test/ subdir holding the given
// files (name -> content) and returns the root. No baseline is written.
func tempRepoWithCI(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	testDir := filepath.Join(root, "test")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", testDir, err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(testDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

const acceptOnlyCI = `stdin=config:terminator=EOF
environment { ntp { enabled true } }
EOF
cmd=foreground:seq=1:exec=ze config validate -:stdin=config
expect=exit:code=0
`

const annotatedCI = `# accept-only: unit-covered by TestParseNTPConfig
stdin=config:terminator=EOF
environment { ntp { enabled true } }
EOF
cmd=foreground:seq=1:exec=ze config validate -:stdin=config
expect=exit:code=0
`

const strengthenedCI = `stdin=config:terminator=EOF
environment { ntp { enabled true } }
EOF
cmd=foreground:seq=1:exec=ze config dump --json -:stdin=config
expect=exit:code=0
expect=stdout:pattern="enabled": "true"
`

const setEShellCI = `tmpfs=check.sh:mode=755:terminator=EOF_SH
#!/bin/sh
set -e
echo ok
EOF_SH
cmd=foreground:seq=1:exec=./check.sh
expect=exit:code=0
`

const rejectCI = `stdin=config:terminator=EOF
environment { ntp { enabled true } }
EOF
cmd=foreground:seq=1:exec=ze config validate -:stdin=config
expect=exit:code=0
reject=stderr:pattern=error
`

// negativeExitCI asserts a per-command exit=1 (a real rejection), so it is NOT
// accept-only even though a file-level expect=exit:code=0 is also present.
const negativeExitCI = `stdin=config:terminator=EOF
environment { ntp { enabled true } }
EOF
cmd=foreground:seq=1:exec=ze config validate -:stdin=config:exit=1
expect=exit:code=0
`

// perCommandExitOnlyCI asserts ONLY a per-command exit=0 (no file-level
// expect=exit:code=0 line, no value). record.go steers multi-validate tests to
// this exact form, so it is just as weak as the file-level version and MUST be
// classified accept-only.
const perCommandExitOnlyCI = `stdin=config:terminator=EOF
environment { ntp { enabled true } }
EOF
cmd=foreground:seq=1:exec=ze config validate -:stdin=config:exit=0
`

// TestCIAcceptOnlyLintFlags proves a NEW unannotated accept-only .ci is flagged.
func TestCIAcceptOnlyLintFlags(t *testing.T) {
	dir := t.TempDir()

	// Predicate: an exit-code-only record is accept-only.
	rec := parseCI(t, dir, "weak.ci", acceptOnlyCI)
	if !isAcceptOnly(rec) {
		t.Fatalf("isAcceptOnly = false, want true for exit-code-only record")
	}
	if hasAcceptOnlyAnnotation([]byte(acceptOnlyCI)) {
		t.Fatalf("hasAcceptOnlyAnnotation = true, want false for unannotated content")
	}

	// Ratchet: an unannotated accept-only file not in the (empty) baseline is a
	// new violation naming the file.
	root := tempRepoWithCI(t, map[string]string{"weak.ci": acceptOnlyCI})
	res, err := checkAcceptOnlyRatchet(root)
	if err != nil {
		t.Fatalf("checkAcceptOnlyRatchet: %v", err)
	}
	want := "test/weak.ci"
	if len(res.newViolations) != 1 || res.newViolations[0] != want {
		t.Fatalf("newViolations = %v, want [%s]", res.newViolations, want)
	}
	if len(res.staleBaseline) != 0 {
		t.Fatalf("staleBaseline = %v, want empty", res.staleBaseline)
	}
}

// TestCIAcceptOnlyLintFlagsPerCommandExitOnly proves a test whose ONLY assertion
// is a per-command exit=0 (no file-level expect=exit:code=0, no value) is
// classified accept-only and flagged. Guards the false-strong hole where such a
// test escaped the file-level-only gate.
func TestCIAcceptOnlyLintFlagsPerCommandExitOnly(t *testing.T) {
	dir := t.TempDir()

	rec := parseCI(t, dir, "percmd.ci", perCommandExitOnlyCI)
	if rec.ExpectExitCode != nil {
		t.Fatalf("fixture invariant broken: expected no file-level exit assertion, got %d", *rec.ExpectExitCode)
	}
	if !isAcceptOnly(rec) {
		t.Fatalf("isAcceptOnly = false, want true for per-command exit=0-only record")
	}

	root := tempRepoWithCI(t, map[string]string{"percmd.ci": perCommandExitOnlyCI})
	res, err := checkAcceptOnlyRatchet(root)
	if err != nil {
		t.Fatalf("checkAcceptOnlyRatchet: %v", err)
	}
	want := "test/percmd.ci"
	if len(res.newViolations) != 1 || res.newViolations[0] != want {
		t.Fatalf("newViolations = %v, want [%s]", res.newViolations, want)
	}
}

// TestCIAcceptOnlyLintAllows proves annotated / strengthened / tmpfs-set-e /
// reject= / negative-exit .ci are NOT flagged.
func TestCIAcceptOnlyLintAllows(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name    string
		content string
		reason  string
	}{
		{"annotated.ci", annotatedCI, "annotation marker allowlists it"},
		{"strong.ci", strengthenedCI, "expect=stdout:pattern observes a value"},
		{"sete.ci", setEShellCI, "tmpfs set -e script does its own checking"},
		{"reject.ci", rejectCI, "reject= observes output"},
		{"negexit.ci", negativeExitCI, "per-command exit=1 is a real rejection"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := parseCI(t, dir, tc.name, tc.content)
			annotated := hasAcceptOnlyAnnotation([]byte(tc.content))
			weak := isAcceptOnly(rec)
			if weak && !annotated {
				t.Fatalf("%s: classified as unannotated accept-only, but %s", tc.name, tc.reason)
			}

			// Full ratchet: with an empty baseline, none of these is a violation.
			root := tempRepoWithCI(t, map[string]string{tc.name: tc.content})
			res, err := checkAcceptOnlyRatchet(root)
			if err != nil {
				t.Fatalf("checkAcceptOnlyRatchet: %v", err)
			}
			if len(res.newViolations) != 0 {
				t.Fatalf("%s: newViolations = %v, want empty (%s)", tc.name, res.newViolations, tc.reason)
			}
		})
	}
}

// unparseableCI is rejected by the generic .ci parser (expect=json without
// conn=/seq= fails parseConnSeq), standing in for a file the ratchet cannot
// classify.
const unparseableCI = `expect=json:json={"x":1}
`

// TestCIAcceptOnlyLintFailsOnUnparseableOutsideAllowlist proves the fail-closed
// behavior: an unparseable .ci OUTSIDE the excluded-dialect allowlist makes the
// ratchet report it (so it cannot silently hide a new accept-only test), while an
// unparseable file UNDER an allowlisted prefix is skipped as documented.
func TestCIAcceptOnlyLintFailsOnUnparseableOutsideAllowlist(t *testing.T) {
	t.Run("outside allowlist is reported", func(t *testing.T) {
		root := tempRepoWithCI(t, map[string]string{"broken.ci": unparseableCI})
		res, err := checkAcceptOnlyRatchet(root)
		if err != nil {
			t.Fatalf("checkAcceptOnlyRatchet: %v", err)
		}
		if len(res.unexpectedParseErrors) != 1 ||
			!strings.HasPrefix(res.unexpectedParseErrors[0], "test/broken.ci:") {
			t.Fatalf("unexpectedParseErrors = %v, want one entry for test/broken.ci", res.unexpectedParseErrors)
		}
	})

	t.Run("under allowlisted prefix is skipped", func(t *testing.T) {
		root := t.TempDir()
		decodeDir := filepath.Join(root, "test", "decode")
		if err := os.MkdirAll(decodeDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(decodeDir, "broken.ci"), []byte(unparseableCI), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		res, err := checkAcceptOnlyRatchet(root)
		if err != nil {
			t.Fatalf("checkAcceptOnlyRatchet: %v", err)
		}
		if len(res.unexpectedParseErrors) != 0 {
			t.Fatalf("unexpectedParseErrors = %v, want empty (test/decode/ is allowlisted)", res.unexpectedParseErrors)
		}
		if res.excludedParseCount != 1 {
			t.Fatalf("excludedParseCount = %d, want 1", res.excludedParseCount)
		}
	})
}

// TestCIAcceptOnlyLintStaleBaseline proves the ratchet-down direction: a baseline
// entry that is no longer accept-only-and-unannotated (here: absent from the tree)
// is reported as stale so it must be removed. Covers baselineOrphans, which the
// synthetic tests above (no baseline) never exercise.
func TestCIAcceptOnlyLintStaleBaseline(t *testing.T) {
	root := tempRepoWithCI(t, map[string]string{"strong.ci": strengthenedCI})
	// Baseline claims two weak tests, but strong.ci is strengthened (not weak) and
	// gone.ci does not exist -- both are stale.
	baseline := "test/strong.ci\ntest/gone.ci\n"
	if err := os.WriteFile(filepath.Join(root, "test", ".accept-only-baseline"), []byte(baseline), 0o644); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	res, err := checkAcceptOnlyRatchet(root)
	if err != nil {
		t.Fatalf("checkAcceptOnlyRatchet: %v", err)
	}
	want := []string{"test/gone.ci", "test/strong.ci"}
	if len(res.staleBaseline) != 2 || res.staleBaseline[0] != want[0] || res.staleBaseline[1] != want[1] {
		t.Fatalf("staleBaseline = %v, want %v", res.staleBaseline, want)
	}
}

// TestCIAcceptOnlyLint is the real gate over test/**/*.ci: no new unannotated
// accept-only file may exist beyond the baseline, and no baseline entry may be
// stale. Runs under the native unit verification pass.
func TestCIAcceptOnlyLint(t *testing.T) {
	root := repoRootForTest(t)
	res, err := checkAcceptOnlyRatchet(root)
	if err != nil {
		t.Fatalf("checkAcceptOnlyRatchet: %v", err)
	}
	t.Logf("skipped %d .ci in documented excluded dialects", res.excludedParseCount)
	if len(res.newViolations) != 0 {
		t.Errorf("%d new unannotated accept-only .ci file(s) beyond the baseline. Strengthen each with a\n"+
			"readback assertion (expect=stdout:contains=/pattern=) or annotate it `# accept-only: <reason>`;\n"+
			"do NOT add it to %s:\n  %s",
			len(res.newViolations), acceptOnlyBaselinePath, strings.Join(res.newViolations, "\n  "))
	}
	if len(res.staleBaseline) != 0 {
		t.Errorf("%d stale baseline entry(ies) in %s (strengthened, annotated, or removed). Remove them and\n"+
			"regenerate: ZE_WRITE_ACCEPT_ONLY_BASELINE=1 go test -run TestRegenerateAcceptOnlyBaseline ./internal/test/runner:\n  %s",
			len(res.staleBaseline), acceptOnlyBaselinePath, strings.Join(res.staleBaseline, "\n  "))
	}
	if len(res.unexpectedParseErrors) != 0 {
		t.Errorf("%d unparseable .ci outside a known excluded dialect — the accept-only ratchet cannot classify it;\n"+
			"fix its syntax, or add its dialect/file to acceptOnlyExcludedParse* in accept_only.go with a reason:\n  %s",
			len(res.unexpectedParseErrors), strings.Join(res.unexpectedParseErrors, "\n  "))
	}
}

// TestRegenerateAcceptOnlyBaseline rewrites the baseline from the current tree.
// It is an opt-in one-shot (guarded by ZE_WRITE_ACCEPT_ONLY_BASELINE=1) so a
// normal `go test` never mutates the checked-in ledger.
func TestRegenerateAcceptOnlyBaseline(t *testing.T) {
	if os.Getenv("ZE_WRITE_ACCEPT_ONLY_BASELINE") != "1" {
		t.Skip("set ZE_WRITE_ACCEPT_ONLY_BASELINE=1 to regenerate the accept-only baseline")
	}
	root := repoRootForTest(t)
	if err := writeAcceptOnlyBaseline(root); err != nil {
		t.Fatalf("writeAcceptOnlyBaseline: %v", err)
	}
	t.Logf("regenerated %s", filepath.Join(root, acceptOnlyBaselinePath))
}
