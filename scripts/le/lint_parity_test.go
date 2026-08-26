// The migration's proof for `le lint`: the script and the command run the same
// checkers over the same scopes and print the same page.
//
// letools/pylint replaces scripts/le/application/lint.py. Both versions remain
// until the swap. This file is deliberately HERE because it is a migration
// artifact. The commit that deletes the script also deletes this proof.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11. For each of the five invocations,
// both halves run the same checkers with the same argv and order. They also
// write byte-identical stdout and answer the same exit code.
// PREVENTS: a lint gate that reports one scope but checks another. The scope is
// an argument instead of configuration. Thus, a missing path changes all the
// work but none of the output.
//
// NOTHING REAL RUNS. Recording stand-ins replace ruff and mypy over a fixture
// checkout. The test compares the argv sent to each checker and the page written
// about their answers. A run over the real tools would only compare ruff with
// itself.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/letools/pylint"
)

// standIn is a checker that records its argv and answers what the case wants.
// One script serves both names, and it writes the name it was called by so the
// recording says which checker ran.
const standIn = `#!/bin/sh
printf '%s' "$(basename "$0")" >> "$LE_PARITY_CALLS"
for arg in "$@"; do printf ' %s' "$arg" >> "$LE_PARITY_CALLS"; done
printf '\n' >> "$LE_PARITY_CALLS"
if [ -f "$LE_PARITY_OUT/$(basename "$0")" ]; then cat "$LE_PARITY_OUT/$(basename "$0")"; fi
exit "${LE_PARITY_CODE:-0}"
`

// lintFixture is a checkout standing in for the real one: the two markers
// lepath and le.paths look for, and the pyproject the ratchet reads.
func lintFixture(t *testing.T, ceiling string) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module x\n\ngo 1.26\n")
	write(t, filepath.Join(root, "feature-gates.txt"), "ze_bgp\tBGP\n")
	write(t, filepath.Join(root, "pyproject.toml"), "[tool.le.lint]\nlegacy-max = "+ceiling+"\n")
	return root
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// checkerDir installs the two stand-ins and answers the directory to put in
// front of PATH.
func checkerDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"ruff", "mypy"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(standIn), 0o700); err != nil { //nolint:gosec // an executable stand-in must be executable
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return dir
}

// answerDir holds what each stand-in prints, keyed by the name it is called by.
func answerDir(t *testing.T, answers map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range answers {
		write(t, filepath.Join(dir, name), body)
	}
	return dir
}

// halves runs one invocation through both implementations and answers what each
// wrote, what each exec'd, and what each exited with.
type halves struct {
	scriptOut, portedOut     string
	scriptCalls, portedCalls []string
	scriptCode, portedCode   int
}

// drive runs one invocation on both sides of the migration.
//
// The script half is a subprocess because a Python tool uses the process
// boundary. The test calls the ported half in process because it is a function.
// This is the purpose of the port. A second binary would compare the dispatcher
// instead of the tool.
func drive(t *testing.T, flags []string, opts pylint.Options, root, checkers, answers string) halves {
	t.Helper()
	var out halves

	scriptCalls := filepath.Join(t.TempDir(), "calls")
	out.scriptOut, out.scriptCode = runScript(t, flags, root, checkers, answers, scriptCalls)
	out.scriptCalls = readCalls(t, scriptCalls)

	portedCalls := filepath.Join(t.TempDir(), "calls")
	t.Setenv("PATH", checkers+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LE_PARITY_CALLS", portedCalls)
	t.Setenv("LE_PARITY_OUT", answers)

	report, code := (&pylint.Linter{Root: root}).Run(opts)
	out.portedOut, out.portedCode = report.Text(), code
	out.portedCalls = readCalls(t, portedCalls)
	return out
}

// runScript runs the Python half in its own process, over the fixture checkout.
func runScript(t *testing.T, flags []string, root, checkers, answers, calls string) (string, int) {
	t.Helper()
	argv := append([]string{"-m", "le.application.lint"}, flags...)
	cmd := exec.CommandContext(t.Context(), "python3", argv...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PYTHONPATH="+filepath.Join(repoRoot(t), "scripts"),
		"ZE_REPO_ROOT="+root,
		"PATH="+checkers+string(os.PathListSeparator)+os.Getenv("PATH"),
		"LE_PARITY_CALLS="+calls,
		"LE_PARITY_OUT="+answers,
	)

	stdout, err := cmd.Output()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !asExitError(err, &exit) {
			t.Fatalf("run the script: %v", err)
		}
		code = exit.ExitCode()
	}
	return string(stdout), code
}

// readCalls answers what the stand-ins recorded, or nothing when neither ran.
func readCalls(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // a test reads a file it wrote the path of
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

// compare asserts that both halves did and said the same thing.
func compare(t *testing.T, got halves) {
	t.Helper()
	if !slices.Equal(got.scriptCalls, got.portedCalls) {
		t.Errorf("the two halves ran different checkers\nscript:\n%s\ncommand:\n%s",
			strings.Join(got.scriptCalls, "\n"), strings.Join(got.portedCalls, "\n"))
	}
	if got.scriptOut != got.portedOut {
		t.Errorf("the two halves wrote different pages\nscript:\n%q\ncommand:\n%q", got.scriptOut, got.portedOut)
	}
	if got.scriptCode != got.portedCode {
		t.Errorf("the two halves exited %d and %d", got.scriptCode, got.portedCode)
	}
	if len(got.scriptCalls) == 0 {
		t.Error("neither half ran a checker: the comparison is vacuous")
	}
}

// TestEveryInvocationAgrees drives the five script invocations. The ceiling is
// 0, and the stand-ins print nothing, so every run is clean. Thus, the test
// compares the ARGV and page instead of a finding.
func TestEveryInvocationAgrees(t *testing.T) {
	cases := map[string]struct {
		flags []string
		opts  pylint.Options
	}{
		"check":     {nil, pylint.Options{}},
		"fix":       {[]string{"--fix"}, pylint.Options{Fix: true}},
		"strict":    {[]string{"--strict-only"}, pylint.Options{StrictOnly: true}},
		"types":     {[]string{"--types-only"}, pylint.Options{TypesOnly: true}},
		"lint-only": {[]string{"--lint-only"}, pylint.Options{LintOnly: true}},
	}

	checkers := checkerDir(t)
	answers := answerDir(t, nil)

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			root := lintFixture(t, "0")
			compare(t, drive(t, one.flags, one.opts, root, checkers, answers))
		})
	}
}

// TestTheRatchetAgreesOnEverySideOfTheCeiling covers the arithmetic. The halves
// can agree on what to run but differ in this calculation. The stand-in prints a
// statistics table with a total of 113.
//
// Each case ALSO pins the absolute script exit code, which distinguishes the
// three cases. Agreement alone is insufficient. If the count parses as 0, all
// ceilings use the under-ceiling branch. Both halves still agree, but the
// ratchet becomes inactive while every case passes. The three codes assert the
// arithmetic from outside it.
func TestTheRatchetAgreesOnEverySideOfTheCeiling(t *testing.T) {
	checkers := checkerDir(t)
	answers := answerDir(t, map[string]string{
		"ruff": "  100\tQ000\t[*] Single quotes found\n   13\tANN001\tMissing type annotation\n",
	})

	// The test applies 113 findings to each ceiling. Counts above and below the
	// ceiling are both failures, but they require different actions. Above means
	// fix the findings. Below means lower the ceiling because an unchanged
	// ceiling loses its purpose.
	cases := map[string]struct {
		ceiling string
		code    int
	}{
		"over":  {"100", 1},
		"at":    {"113", 0},
		"under": {"200", 1},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			root := lintFixture(t, one.ceiling)
			got := drive(t, nil, pylint.Options{}, root, checkers, answers)
			compare(t, got)
			if got.scriptCode != one.code {
				t.Errorf("the script exited %d against a ceiling of %s, want %d",
					got.scriptCode, one.ceiling, one.code)
			}
		})
	}
}

// TestAnAbsentCheckerAgrees pins the failure both halves must NOT read as a
// pass. PATH holds no ruff and no mypy here, so both must say so and fail.
func TestAnAbsentCheckerAgrees(t *testing.T) {
	empty := t.TempDir()
	root := lintFixture(t, "0")

	got := drive(t, nil, pylint.Options{}, root, empty, answerDir(t, nil))
	if got.scriptOut != got.portedOut {
		t.Errorf("the two halves wrote different pages\nscript:\n%q\ncommand:\n%q", got.scriptOut, got.portedOut)
	}
	if got.scriptCode != got.portedCode {
		t.Errorf("the two halves exited %d and %d", got.scriptCode, got.portedCode)
	}
	if got.scriptCode == 0 {
		t.Error("an absent checker passed: the gate reads as checked when it means not checked")
	}
}

// TestTheCeilingIsTheSameNumberOnBothSides uses each half to read the REAL
// pyproject.toml. A value one lower changes no output branch, so an output
// comparison cannot detect it. Only this case compares the value.
func TestTheCeilingIsTheSameNumberOnBothSides(t *testing.T) {
	root := repoRoot(t)

	scripted := strings.TrimSpace(python(t, root, `
from le.application.lint import legacy_ceiling
print(legacy_ceiling())
`))

	ported, err := pylint.LegacyCeiling(root)
	if err != nil {
		t.Fatalf("LegacyCeiling: %v", err)
	}

	var rendered strings.Builder
	rendered.WriteString(scripted)
	if got := itoa(ported); got != rendered.String() {
		t.Errorf("the script reads a ceiling of %s and the command reads %s", scripted, got)
	}
}

// itoa renders an integer and avoids a formatter that hooks refuse in non-test
// code. Thus, both halves retain the same spelling.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
