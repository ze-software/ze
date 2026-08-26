// Design: docs/architecture/core-design.md -- the changed-file wiring and docs router
// Detail: sources.go -- which changed file needs which gate
// Detail: checks.go -- the checks this gate runs itself
// Detail: wiring.go -- the added-symbol check, and allowlist.go its exceptions
// Detail: groups.go -- how a failure says which files it is about
// Detail: report.go -- what the command answers
// Detail: delegate.go -- the selected gates this binary answers itself
// Detail: forks.go -- the ones it still starts another program for
//
// Package docwiring runs the changed-file-aware wiring, documentation, command
// and inventory gate.
//
// It is intentionally a ROUTER that selects checks for the current diff. Each
// check keeps its own Make target. Eight of twelve checks are linked Go
// packages, so the router CALLS them instead of starting make (delegate.go).
// The router also attributes each failure to repository paths at the failure
// point. The verify runner can then charge the failure to the session that
// caused it.
//
// The Python half had two unused features that are not ported. ZE_REPO_ROOT
// replaces `--root DIR` and follows keyword-before-value grammar. The
// `--check-plugin-imports` option ran only one delegated check. `le
// plugin-imports` provides that check, and this gate selects its Make target
// when needed.
package docwiring

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/letools/lepath"
	"github.com/ze-software/ze/letools/leroot"
)

// name is the word this command is typed as.
const name = "doc-wiring"

// gateTarget is this gate's Make target. Every rule, document, journal row, and
// census claim uses this spelling.
const gateTarget = "ze-doc-wiring-check"

// gateRerun is the command a reader runs to reproduce any failure of this gate.
const gateRerun = "make " + gateTarget

// delegatedTimeout bounds one delegated Make target. The longest target builds
// and runs a tree-wide documentation check. This limit is ten times longer and
// still stops a hung target.
const delegatedTimeout = 30 * time.Minute

// Options is what the operator asked for.
type Options struct {
	// Changed names the files to evaluate. An empty list means "ask git".
	Changed []string
	// DryRun prints the selected gates instead of running them.
	DryRun bool
	// Make is the make executable delegated targets are run with.
	Make string
}

// gate is one run: the tree it judges, what the operator asked for, and what it
// has found so far.
type gate struct {
	root   string
	opts   Options
	report Report
}

// Answer is the `le doc-wiring` command.
func Answer(args []string) (any, int) {
	opts, code, ok := parseOptions(args)
	if !ok {
		return nil, code
	}

	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 2
	}

	report, code := Run(root, opts)
	return report, code
}

// Run judges one tree and answers the report plus the exit code.
//
// The code is the gate verdict: 0 when every selected check passes, and 1
// otherwise. A delegated target reports through its own Make invocation. This
// gate reports only whether the complete run is clean.
func Run(root string, opts Options) (Report, int) {
	g := &gate{root: root, opts: opts}
	g.report.Make = opts.Make

	changed := make([]string, 0, len(opts.Changed))
	for _, path := range opts.Changed {
		changed = append(changed, normalizeChangedPath(root, path))
	}
	if len(changed) == 0 {
		discovered, err := ChangedFiles(root)
		if err != nil {
			// The RUN failed before any check judged the tree because git is
			// unusable. With no declared group, the verify runner uses its own
			// classifier instead of preferring an empty group set.
			reportError(err)
			return Report{Failed: true, Error: err.Error()}, 1
		}
		changed = discovered
	}

	g.report.Changed = changed
	targets, err := SelectedTargets(root, changed)
	if err != nil {
		// The router cannot select gates when it cannot read a changed file.
		// Returning a partial selection would route that file to no gate.
		reportError(err)
		return Report{Failed: true, Error: err.Error()}, 2
	}
	g.report.Targets = targets
	g.report.Advisory = FunctionalTestAdvisory(changed)

	if opts.DryRun {
		g.report.DryRun = true
		return g.report, 0
	}

	code := g.run()
	g.report.Failed = code != 0
	if code != 0 {
		g.report.DeclaredGroups = len(g.report.Groups)
	}
	return g.report, code
}

// run executes the selected checks and answers the gate's exit code.
//
// Every check passes through runCheck, which makes attribution structural. If a
// failed check declares nothing, runCheck adds an unattributable group. The
// failure is then CHARGED instead of disappearing from the failure index. A
// check that knows its files declares them at the failure point.
func (g *gate) run() int {
	gateRC := 0
	for _, check := range []struct {
		name string
		run  func() CheckResult
	}{
		{checkSleepRatchetName, func() CheckResult { return g.checkSleepRatchet() }},
		{checkSleepJustificationName, func() CheckResult { return g.checkSleepJustification() }},
		{checkLoadExcuseName, func() CheckResult { return g.checkLoadExcuses() }},
		{checkLogSubsystemName, func() CheckResult { return g.checkLogSubsystemKeys() }},
		{checkDesignRefsName, func() CheckResult { return g.checkDesignRefs() }},
	} {
		// Every check runs, whatever an earlier one answered, and gateRC keeps
		// the first non-zero.
		if rc := g.runCheck(check.name, gateRerun, check.run); rc != 0 && gateRC == 0 {
			gateRC = rc
		}
	}

	if len(g.report.Targets) == 0 {
		return gateRC
	}

	for _, target := range g.report.Targets {
		if target != wiringTarget {
			continue
		}
		if rc := g.runCheck(wiringTarget, gateRerun, g.checkWiring); rc != 0 {
			// A wiring failure stops the run. Every later delegated target would
			// judge a tree already shown to contain unwired code.
			return 1
		}
	}

	for _, target := range g.report.Targets {
		if target == wiringTarget {
			continue
		}
		var tb textbuf.Buffer
		rerun := tb.Str("make ").Str(target).String()
		if rc := g.runCheck(target, rerun, func() CheckResult { return g.runTarget(target) }); rc != 0 {
			// A failed delegated target stops the run, as in the script. Later
			// targets would judge a tree that an earlier target refused.
			return rc
		}
	}
	return gateRC
}

// runCheck runs one sub-check and lets no failure of it leave the failure index.
//
// A failed check without a group would publish a self-consistent count because
// the count includes only MADE declarations. Neither the count nor group lines
// would include that failure. The reader CAN treat the empty group set as
// complete and drop the failure. runCheck adds one no-file group, whose
// unattributable kind CHARGES the committing session.
func (g *gate) runCheck(check, rerun string, run func() CheckResult) int {
	before := len(g.report.Groups)
	result := run()
	result.Name = check
	g.report.Checks = append(g.report.Checks, result)
	if !result.Failed {
		return 0
	}
	if len(g.report.Groups) == before {
		var tb textbuf.Buffer
		g.declareFailureGroup(check, nil,
			tb.Str(check).Str(" failed without naming the files it is about").String(), rerun)
	}
	if result.Code != 0 {
		return result.Code
	}
	return 1
}

// runMakeTarget runs one delegated Make target, for a gate no Go package in
// this binary holds (delegate.go names the eight that are held here).
//
// The target is checked against the declared set first. An unknown target is a
// programming error in the selection table, not a tree property. The router
// refuses it instead of invoking Make with an undeclared name.
func (g *gate) runMakeTarget(target string) CheckResult {
	if !makeTargets[target] {
		var tb textbuf.Buffer
		return CheckResult{
			Failed:  true,
			Message: tb.Str("unknown make target ").Str(target).String(),
		}
	}

	makeExe := g.opts.Make
	if makeExe == "" {
		makeExe = defaultMake
	}

	ctx, cancel := context.WithTimeout(context.Background(), delegatedTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, makeExe, "--no-print-directory", target) //nolint:gosec // a Make target from the declared set, run with the caller's make
	cmd.Dir = g.root
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()

	var tb textbuf.Buffer
	if err == nil {
		return CheckResult{Message: tb.Str(target).Str(" PASSED").String(), Output: out.String()}
	}

	// The child's stderr passes through as the script's does. Its stdout is
	// carried in the report, where the rendering puts it exactly where the
	// script prints it.
	if errOut.Len() > 0 {
		fmt.Fprint(os.Stderr, errOut.String()) //nolint:errcheck // CLI output
	}

	// This gate deliberately names no file here. Another program produced the
	// paths in the child output. That attribution can make an unexplained failure
	// look like another session's. The group makes the failure CHARGED. A target
	// that knows its files declares its own groups.
	g.declareFailureGroup(target, nil,
		tb.Str("delegated target ").Str(target).Str(" failed").String(),
		tb.Reset().Str("make ").Str(target).String())

	tb.Reset()
	return CheckResult{
		Failed:  true,
		Message: tb.Str(target).Str(" failed").String(),
		Output:  out.String(),
	}
}

// checkWiring runs the added-symbol wiring check and declares its group.
func (g *gate) checkWiring() CheckResult {
	issues, err := CheckWiring(g.root, g.report.Changed, func(path string) string {
		return readHeadOrEmpty(g.root, path)
	})
	if err != nil {
		return g.readFailure(wiringTarget, err)
	}
	if len(issues) == 0 {
		return CheckResult{Message: "Wiring check PASSED"}
	}

	// Each issue starts with `<path>:<line>: exported ...`. The doc-link prefix
	// reader can parse the same form, so the group names each reported file.
	g.declareFailureGroup(wiringTarget, findingPaths(g.root, issues),
		"an exported symbol added by this change has no non-test reference", gateRerun)
	return CheckResult{Failed: true, Message: "Wiring check FAILED:", Violations: issues}
}

// normalizeChangedPath answers a caller's path as a repository-relative one.
func normalizeChangedPath(root, path string) string {
	if !filepath.IsAbs(path) {
		return filepath.ToSlash(path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

// reportError writes one failure line to stderr, in the spelling every ported
// le tool uses.
func reportError(err error) {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("error: ").Err(err).String()) //nolint:errcheck // CLI output
}

// parseOptions reads the keywords this command takes. Each keyword that carries
// a value consumes exactly one word after it, so a path that spells a keyword
// is still a legal value.
func parseOptions(args []string) (Options, int, bool) {
	opts := Options{Make: defaultMake}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "dry-run":
			opts.DryRun = true
		case "changed-file", "make":
			if i+1 >= len(args) {
				return opts, refuseMissingValue(args[i]), false
			}
			i++
			if args[i-1] == "make" {
				opts.Make = args[i]
				continue
			}
			opts.Changed = append(opts.Changed, args[i])
		default:
			return opts, refuseKeyword(args[i]), false
		}
	}
	return opts, 0, true
}

func refuseKeyword(got string) int {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("error: ").Str(name).Str(": no such keyword: ").Quoted(got).String()) //nolint:errcheck // CLI output
	fmt.Fprintln(os.Stderr, usageLine())                                                                 //nolint:errcheck // CLI output
	return 1
}

func refuseMissingValue(keyword string) int {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("error: ").Str(name).Byte(' ').Str(keyword).Str(" needs a value").String()) //nolint:errcheck // CLI output
	fmt.Fprintln(os.Stderr, usageLine())                                                                       //nolint:errcheck // CLI output
	return 1
}

// usageLine states the whole grammar in one line, which is what a developer
// needs after a refusal.
func usageLine() string {
	var tb textbuf.Buffer
	return tb.Str("usage: le ").Str(name).
		Str(" [changed-file <path>]... [make <exe>] [dry-run] [| json | yaml | table]").String()
}

// Subs is the one-line hint help renders under the command.
func Subs() string {
	return "changed-file <path> | make <exe> | dry-run"
}

// leAnswer keeps the registered handler's type honest against leroot.Answer.
var _ leroot.Answer = Answer
