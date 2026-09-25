// Design: docs/architecture/core-design.md -- changed-file wiring and docs router.
// Detail: sources.go -- which changed file needs which native action.
// Detail: checks.go -- checks implemented directly by this package.
// Detail: groups.go -- failure attribution.
// Detail: delegate.go -- linked native action callbacks.
//
// Package docwiring selects checks for the current diff and calls their Go
// owners directly. It attributes each failure at the failure point so the
// verifier can charge the session that caused it.
package docwiring

import (
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// name is the word this command is typed as.
const name = "doc wiring"

const actionRerun = "./le doc wiring"

// checkVerb names the changed-file router, which is what a bare command line
// runs. The whole area is one table. The router's two keywords are declared
// beside the action they belong to, not read by a parser of this package's own.
const checkVerb = "check"

// The keywords the router takes. Each types the value that follows it, so a
// path that happens to spell a keyword is still a path.
const (
	changedFileKeyword = "changed-file"
	dryRunKeyword      = "dry-run"
)

// actions is the whole command surface.
var actions = leaction.New(name,
	leaction.Action{
		Verb: checkVerb,
		Why: "run every wiring, documentation, command and inventory check the changed files select." +
			" Naming no file reads the changed set from git, which is what the gate runs",
		Parameters: []leaction.Parameter{
			// Optional: a run that names no file asks git for the changed set.
			// Repeat: a caller names every file of a commit, one keyword each.
			{Keyword: changedFileKeyword, Value: "path", Requirement: leaction.Optional, Repeat: true},
			{Keyword: dryRunKeyword},
		},
		AnswerArgs: answerRouter,
	},
	leaction.Action{
		Verb:   templOrphanVerb,
		Why:    "report a .templ source outside internal/ or a generated templ Go file whose source is absent",
		Answer: answerTemplOrphansHere,
	},
)

// Options is what the operator asked for.
type Options struct {
	// Changed names the files to evaluate. An empty list means "ask git".
	Changed []string
	// DryRun prints the selected gates instead of running them.
	DryRun bool
}

// checker is one run: the tree it judges, what the operator asked for, and what it
// has found so far.
type checker struct {
	root   string
	opts   Options
	report Report
}

// Answer is the `le doc wiring` command.
//
// A bare command line runs the router, and so does a line opening with one of
// the router's keywords. Both are the `check` action, so the verb is supplied
// here and ONE parser reads the rest of the line. The bare spelling stays the
// command a reader copies: `internal/le/verify/engine` runs the structural
// stage as `le doc wiring`, and every failure group prints that rerun line.
//
// The table decides which line is which. A word the area declares is a verb.
// It travels as it stands, so `le doc wiring check` runs the action the bare
// line runs. Comparing the word against the verbs by hand is what made `check`
// unreachable. The line grew a second `check`, and the keyword parser refused
// the published verb.
func Answer(args []string) (any, int) {
	if len(args) > 0 && actions.Holds(args[0]) {
		return actions.Answer(args)
	}

	line := make([]string, 0, len(args)+1)
	line = append(line, checkVerb)
	line = append(line, args...)
	return actions.Answer(line)
}

// answerRouter runs the router over the files the invocation named.
func answerRouter(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 2
	}

	report, code := Run(root, optionsFrom(args))
	return report, code
}

// optionsFrom reads the parsed keywords as what the operator asked for. Values
// answers nothing for an absent keyword, which is the whole-tree population a
// bare command already read.
func optionsFrom(args leaction.Arguments) Options {
	return Options{
		Changed: args.Values(changedFileKeyword),
		DryRun:  args.Has(dryRunKeyword),
	}
}

// Run judges one tree and answers the report plus the exit code.
// Run evaluates the selected native actions and returns their aggregate status.
func Run(root string, opts Options) (Report, int) {
	g := &checker{root: root, opts: opts}
	changed := make([]string, 0, len(opts.Changed))
	for _, path := range opts.Changed {
		changed = append(changed, normalizeChangedPath(root, path))
	}
	if len(changed) == 0 {
		discovered, err := ChangedFiles(root)
		if err != nil {
			reportError(err)
			return Report{Failed: true, Error: err.Error()}, 1
		}
		changed = discovered
	}

	g.report.Changed = changed
	actions, err := selectedActions(root, changed)
	if err != nil {
		reportError(err)
		return Report{Failed: true, Error: err.Error()}, 2
	}
	g.report.Actions = actions
	g.report.Advisory = functionalTestAdvisory(changed)
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

func (g *checker) run() int {
	code := 0
	for _, check := range []struct {
		name string
		run  func() CheckResult
	}{
		{checkSleepRatchetName, func() CheckResult { return g.checkSleepRatchet() }},
		{checkSleepJustificationName, func() CheckResult { return g.checkSleepJustification() }},
		{checkLoadExcuseName, func() CheckResult { return g.checkLoadExcuses() }},
		{checkLogSubsystemName, func() CheckResult { return g.checkLogSubsystemKeys() }},
		{checkDesignRefsName, func() CheckResult { return g.checkDesignRefs() }},
		{checkDocDriftName, func() CheckResult { return g.checkDocDrift() }},
	} {
		if current := g.runCheck(check.name, actionRerun, check.run); current != 0 && code == 0 {
			code = current
		}
	}
	if len(g.report.Actions) == 0 {
		return code
	}
	for _, action := range g.report.Actions {
		if action != wiringTarget {
			continue
		}
		if current := g.runCheck(wiringTarget, actionRerun, g.checkWiring); current != 0 {
			return current
		}
	}
	for _, action := range g.report.Actions {
		if action == wiringTarget {
			continue
		}
		var text textbuf.Buffer
		rerun := text.Str("./le ").Str(strings.ReplaceAll(action, "/", " ")).String()
		if current := g.runCheck(action, rerun, func() CheckResult { return g.runAction(action) }); current != 0 {
			return current
		}
	}
	return code
}

// runCheck runs one sub-check and lets no failure of it leave the failure index.
//
// A failed check without a group would publish a self-consistent count because
// the count includes only MADE declarations. Neither the count nor group lines
// would include that failure. The reader CAN treat the empty group set as
// complete and drop the failure. runCheck adds one no-file group, whose
// unattributable kind CHARGES the committing session.
func (g *checker) runCheck(check, rerun string, run func() CheckResult) int {
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

// checkWiring runs the added-symbol wiring check and declares its group.
func (g *checker) checkWiring() CheckResult {
	issues, err := checkWiring(g.root, g.report.Changed, func(path string) string {
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
		"an exported symbol added by this change has no non-test reference", actionRerun)
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
	tb.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI output
}

// Actions answers the command surface as data. The listing, the Subs line help
// renders, the manifest and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// leAnswer keeps the registered handler's type honest against leroot.Answer.
var _ leroot.Answer = Answer
