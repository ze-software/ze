// Design: docs/contributing/ze-python-style.md -- the gate that holds the Python half of the tree
//
// Package pylint is scripts/le/application/lint.py, ported. It lints and
// type-checks the Python half of this tree.
//
// TWO SCOPES, because the tree is two populations and measuring them says so.
// The strict rule set reports 59,207 findings against the code written before
// any of this existed, and 53,625 of those are the quote style alone. A gate
// nobody can pass is not a gate.
//
//	strict   scripts/le and ./le. Every ruff rule, enforced formatting, and
//	         mypy --strict. Clean from the first commit, so there is nothing to
//	         ratchet down from.
//	legacy   everything else. Real defect shapes only -- undefined names,
//	         mutable defaults, loop-variable capture -- and no style at all.
//	         Held to a ceiling that must FALL rather than to a zero it cannot
//	         reach.
//
// This file does not duplicate either rule set.
// Ruff selects pyproject.toml for the legacy tree and scripts/le/ruff.toml for the strict scope.
// One `ruff check` therefore matches the editor's result for both scopes.
//
// A missing checker is a failure, not a skip.
// Otherwise, the gate reports "checked" when no check occurred.

package pylint

import (
	"context"
	"os/exec"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// StrictScope lists the strictly checked paths relative to the checkout.
// It includes the root entry-point shim `le`, which follows the same standard.
var StrictScope = []string{"scripts/le", "le"}

// legacyExclude is what the legacy count leaves out: the strict scope, which is
// counted by its own stages above.
const legacyExclude = "scripts/le"

// These constants name both checkers and the ruff subcommand once.
// The argv, heading, and action table share them, so a typo cannot make those values diverge.
const (
	ruffBin    = "ruff"
	mypyBin    = "mypy"
	ruffCheck  = "check"
	ruffFormat = "format"
)

// Options is what one run was asked for, apart from how it was asked for. Each
// field is one action of the area, so no two of them are ever set together.
type Options struct {
	// Fix applies the fixes ruff can make, and formats, instead of only
	// reporting.
	Fix bool
	// StrictOnly checks the strict scope alone and skips the legacy ratchet.
	StrictOnly bool
	// TypesOnly runs the type checker alone.
	TypesOnly bool
	// LintOnly runs the linter alone.
	LintOnly bool
}

// Linter runs the checkers. Every seam is a field rather than a package
// function, so a test drives the same code the command runs.
type Linter struct {
	// Root is the checkout every checker runs in.
	Root string
	// Ctx cancels a running checker. It defaults to a background context: the
	// script had no ceiling either, and a checker that hangs is a checker
	// somebody is watching.
	Ctx context.Context
	// Which reports whether a checker is on PATH. It defaults to a real lookup.
	Which func(name string) bool
	// Exec runs one checker and answers its stdout, its stderr and its exit
	// code. It defaults to a real fork.
	Exec func(argv []string, dir string) (out, errOut string, code int)
}

// present reports whether a checker is installed.
func (l *Linter) present(name string) bool {
	if l.Which != nil {
		return l.Which(name)
	}
	_, err := exec.LookPath(name)
	return err == nil
}

// run runs one checker.
func (l *Linter) run(argv []string) (string, string, int) {
	if l.Exec != nil {
		return l.Exec(argv, l.Root)
	}

	ctx := l.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // the argv is built here from a fixed table
	cmd.Dir = l.Root

	var out, errOut textbuf.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		code := cmd.ProcessState.ExitCode()
		if code <= 0 {
			// A checker that never started differs from one that ran and reported an error.
			// Shells use 127 for this condition, so the report keeps those results separate.
			var why textbuf.Buffer
			return out.String(), why.Str(errOut.String()).Err(err).String(), 127
		}
		return out.String(), errOut.String(), code
	}
	return out.String(), errOut.String(), 0
}

// Run executes the requested checks and returns their report and exit code.
//
// Every stage runs after a failure.
// Stopping at the first error would return only one problem instead of the complete lint result.
func (l *Linter) Run(opts Options) (Report, int) {
	report := Report{}

	if !opts.TypesOnly {
		l.ruffStrict(&report, opts.Fix)
	}
	if !opts.LintOnly {
		l.mypy(&report)
	}
	if !opts.StrictOnly && !opts.TypesOnly {
		l.ruffLegacy(&report)
	}

	if len(report.Failed) > 0 {
		return report, 1
	}
	report.OK = true
	return report, 0
}

// ruffStrict lints and format-checks the strict scope.
func (l *Linter) ruffStrict(report *Report, fix bool) {
	if !l.present(ruffBin) {
		report.skip(ruffBin, "not installed; run `./le setup`", "ruff (not installed)")
		return
	}

	check := append([]string{ruffBin, ruffCheck}, StrictScope...)
	if fix {
		check = append(check, "--fix")
	}
	if !l.stage(report, check) {
		report.fail("ruff check")
	}

	format := append([]string{ruffBin, ruffFormat}, StrictScope...)
	if !fix {
		format = append(format, "--check")
	}
	if !l.stage(report, format) {
		report.fail("ruff format")
	}
}

// mypy type-checks the strict scope. The scope comes from pyproject.toml's
// `files`, so it is not repeated here.
func (l *Linter) mypy(report *Report) {
	if !l.present(mypyBin) {
		report.skip(mypyBin, "not installed; run `./le setup`", "mypy (not installed)")
		return
	}
	if !l.stage(report, []string{mypyBin}) {
		report.fail("mypy")
	}
}

// ruffLegacy counts findings outside the strict scope and compares them with the ceiling.
//
// This is a ratchet because the current count is not zero.
// Excluding the tree would remove coverage. A zero requirement would fail every run.
// An increase fails. A decrease also produces a message so that developers lower the ceiling.
//
// One missing ruff produces one failure.
// The strict stages already report it, so this stage does not repeat the same absence.
func (l *Linter) ruffLegacy(report *Report) {
	if !l.present(ruffBin) {
		return
	}

	ceiling, err := LegacyCeiling(l.Root)
	if err != nil {
		report.skip("ruff check (legacy tree)", err.Error(), "ruff check (legacy ceiling unreadable)")
		return
	}
	report.Ceiling = ceiling

	// `--statistics` for the count, so the report is a table of rule totals
	// rather than a hundred diagnostics nobody reads on a green run. The
	// per-rule breakdown is what a reader needs when it goes red, which is why
	// both come from one invocation.
	argv := []string{ruffBin, ruffCheck, "--statistics", "--exclude", legacyExclude}
	out, _, _ := l.run(argv)
	found := countFindings(out)
	report.Findings = found

	stage := Stage{Name: legacyLabel(ceiling), Argv: argv, Findings: found, Ceiling: ceiling}

	switch {
	case found > ceiling:
		stage.Output = trimTrailingSpace(out)
		stage.Detail = overCeiling(found, ceiling)
		report.add(stage)
		report.fail("ruff check (legacy)")
	case found < ceiling:
		stage.Detail = underCeiling(found, ceiling)
		report.add(stage)
		report.fail("ruff check (legacy ceiling is stale)")
	default:
		stage.OK = true
		stage.Detail = atCeiling(found)
		report.add(stage)
	}
}

// stage runs one checker, records what it said, and reports whether it passed.
func (l *Linter) stage(report *Report, argv []string) bool {
	out, errOut, code := l.run(argv)
	var text textbuf.Buffer
	text.Str(trimTrailingSpace(out))
	if second := trimTrailingSpace(errOut); second != "" {
		if text.Len() > 0 {
			text.Byte('\n')
		}
		text.Str(second)
	}

	report.add(Stage{Name: label(argv), Argv: argv, Output: text.String(), OK: code == 0})
	return code == 0
}

// countFindings totals `ruff check --statistics` counts.
// It reads lines that start with a count and rule code, and ignores summary lines.
func countFindings(statistics string) int {
	total := 0
	for _, line := range splitLines(statistics) {
		head := firstField(line)
		if value, ok := parseCount(head); ok {
			total += value
		}
	}
	return total
}
