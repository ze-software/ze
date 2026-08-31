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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// name is the word this command is typed as.
const name = "doc wiring"

const actionRerun = "./le doc wiring"

// zeroArgumentActions holds exact actions that share this area but do not run
// the changed-file router.
var zeroArgumentActions = leaction.New(name,
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
// The templ-orphans action is exact and takes no arguments. A bare command and
// the changed-file and dry-run keywords retain the router contract.
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		if args[0] == templOrphanVerb {
			return zeroArgumentActions.Answer(args)
		}
	}

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

// parseOptions reads the keywords this command takes. Each keyword that carries
// a value consumes exactly one word after it, so a path that spells a keyword
// is still a legal value.
func parseOptions(args []string) (Options, int, bool) {
	opts := Options{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "dry-run":
			opts.DryRun = true
		case "changed-file":
			if i+1 >= len(args) {
				return opts, refuseMissingValue(args[i]), false
			}
			i++
			opts.Changed = append(opts.Changed, args[i])
		default:
			return opts, refuseKeyword(args[i]), false
		}
	}
	return opts, 0, true
}

func refuseKeyword(got string) int {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(name).Str(": no such keyword: ").Quoted(got).Byte('\n').StdErr() //nolint:errcheck // CLI output
	fmt.Fprintln(os.Stderr, usageLine())                                                   //nolint:errcheck // CLI output
	return 1
}

func refuseMissingValue(keyword string) int {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(name).Byte(' ').Str(keyword).Str(" needs a value").Byte('\n').StdErr() //nolint:errcheck // CLI output
	fmt.Fprintln(os.Stderr, usageLine())                                                         //nolint:errcheck // CLI output
	return 1
}

// usageLine states the whole grammar, which is what a developer needs after a
// refusal.
func usageLine() string {
	var tb textbuf.Buffer
	return tb.Str("usage: le ").Str(name).
		Str(" [changed-file <path>]... [dry-run] [| json | yaml | table]\n").
		Str("       le ").Str(name).Byte(' ').Str(templOrphanVerb).
		Str(" [| json | yaml | table]").String()
}

// Subs is the one-line hint help renders under the command.
func Subs() string {
	var tb textbuf.Buffer
	return tb.Str("changed-file <path> | dry-run | ").Str(templOrphanVerb).String()
}

// leAnswer keeps the registered handler's type honest against leroot.Answer.
var _ leroot.Answer = Answer
