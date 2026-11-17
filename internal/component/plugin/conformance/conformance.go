// Design: docs/architecture/plugin/plugin-system.md -- the plugin conformance checker
// Detail: checks.go -- the checks this version registers
// Detail: register.go -- the `show plugin conformance run` command
//
// Package conformance judges whether a program behaves as a Ze plugin, the
// way docs/plugin-development/protocol.md says a plugin behaves. It starts
// the program in declare mode, the one fork Ze already makes for `show plugin
// declarations`, and judges what the program wrote with the daemon's own
// Stage 1 check table (plugin/server, JudgeDeclaration).
//
// A report holds one row for each check. A behavior this version does not
// judge is a row too, with the verdict not-judged, so a report with no failed
// row is never read as full conformance.

package conformance

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/server"
)

// Verdict is what one row says about one check.
type Verdict string

const (
	// VerdictPass is a check the program passed.
	VerdictPass Verdict = "pass"
	// VerdictFail is a check the program failed. The row's Reason says what
	// broke and its Fix says what to change.
	VerdictFail Verdict = "fail"
	// VerdictNotJudged is a check nobody ran. The row's Reason says why.
	VerdictNotJudged Verdict = "not-judged"
)

// The exit codes a caller of the checker answers with.
const (
	// ExitConforms is a report with no failed row.
	ExitConforms = 0
	// ExitViolation is a report with a failed row.
	ExitViolation = 1
	// ExitNoRun is a run that could not happen or was interrupted, so there is
	// no completed report.
	ExitNoRun = 2
)

// Target names the program to judge, as an operator would configure it.
type Target struct {
	// Run is the run string of `plugin { external <name> { run ...; } }`. A
	// shell starts it, as the daemon's live start does.
	Run string
	// Name is the plugin name the daemon's checks name in their refusals.
	Name string
	// WorkDir is the directory the program starts in. Empty starts it in the
	// caller's own directory.
	WorkDir string
}

// Row is one check of a report.
type Row struct {
	// Check is the kebab-case name of the check.
	Check string `json:"check"`
	// Stage is the part of the protocol the check belongs to: declare for
	// query mode, a stage number, or the name of a later part.
	Stage   string  `json:"stage"`
	Verdict Verdict `json:"verdict"`
	// Reason says what the check found, in the plugin author's terms.
	Reason string `json:"reason,omitempty"`
	// Fix is the exact thing to change. A passed row carries none.
	Fix string `json:"fix,omitempty"`
}

// Report is the answer of one run: the program judged, and one row for each
// check, in the order the protocol reaches them.
type Report struct {
	Plugin string `json:"plugin"`
	Run    string `json:"run"`
	// Verdict is pass when no row failed. It says nothing about the rows that
	// were not judged.
	Verdict Verdict `json:"verdict"`
	Rows    []Row   `json:"checks"`
}

// ExitCode answers the code a caller exits with for this report.
func (r Report) ExitCode() int {
	if r.Verdict == VerdictFail {
		return ExitViolation
	}
	return ExitConforms
}

// Evidence is everything one run learned about the program. Each check reads
// it and answers its rows.
type Evidence struct {
	Target Target
	// Budget is the time the program had to write its declaration and exit.
	Budget time.Duration
	// Query is the declare-mode run.
	Query plugin.DeclarationQuery
	// Judgement is the daemon's Stage 1 judgement of Query.Declaration. It is
	// meaningful only when Query.Declared is true. Its check names are
	// meaningful either way, so a check can name the rows it could not judge.
	Judgement plugin.DeclarationJudgement
}

// Check is one registered conformance check. A check registers from an
// init() and is found by Run: no list names the checks.
type Check struct {
	// Name identifies the registration. It is the row's name when the check
	// answers one row.
	Name string
	// Order places the check's rows in the report, lowest first. Two checks
	// of one order follow each other by name.
	Order int
	// Rows judges the evidence and answers the check's rows.
	Rows func(evidence *Evidence) []Row
}

var (
	checksMu sync.RWMutex
	checks   = map[string]Check{}
)

// Register adds a check. It is called from an init(), so a name declared
// twice, or a check that cannot answer, is a defect in Ze and panics there.
//
// Safe for concurrent use.
func Register(check Check) {
	if check.Name == "" {
		panic("BUG: conformance.Register: a check with no name")
	}
	if check.Rows == nil {
		panic("BUG: conformance.Register: the check " + check.Name + " has no Rows function")
	}
	checksMu.Lock()
	defer checksMu.Unlock()
	if _, taken := checks[check.Name]; taken {
		panic("BUG: conformance.Register: the check " + check.Name + " is registered twice")
	}
	checks[check.Name] = check
}

// registered answers the checks in report order.
func registered() []Check {
	checksMu.RLock()
	ordered := make([]Check, 0, len(checks))
	for _, check := range checks {
		ordered = append(ordered, check)
	}
	checksMu.RUnlock()

	slices.SortFunc(ordered, func(a, b Check) int {
		if byOrder := cmp.Compare(a.Order, b.Order); byOrder != 0 {
			return byOrder
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return ordered
}

// errNoRunString and errNoPluginName are the two targets Run refuses before
// it starts anything.
var (
	errNoRunString = errors.New("the conformance check needs the run string of the plugin, " +
		"and none was given: name the command that starts it, as in run \"./my-plugin\"")
	errNoPluginName = errors.New("the conformance check needs a plugin name, and none was given: " +
		"add name <plugin-name>")
)

// Run starts the target in declare mode, within budget, and answers the
// report. An error means no report completed: the target names no program,
// the caller's context ended before or during the query, or this host has no
// shell to start a run string with. A program that fails to start is a failed
// row of the report, never an error.
//
// Caller cancellation or deadline stops the query's isolated process group
// and joins it before returning the caller's error with no report. An active
// caller whose independent query budget expires still receives a failed row.
// The caller MUST provide a non-nil context.
func Run(ctx context.Context, target Target, budget time.Duration) (Report, error) {
	if strings.TrimSpace(target.Run) == "" {
		return Report{}, errNoRunString
	}
	if target.Name == "" {
		return Report{}, errNoPluginName
	}
	if err := ctx.Err(); err != nil {
		return Report{}, fmt.Errorf("the conformance check was stopped before it started %s: %w", target.Run, err)
	}
	if err := plugin.ShellAvailable(); err != nil {
		return Report{}, err
	}
	evidence := &Evidence{Target: target, Budget: budget}
	evidence.Query = plugin.QueryDeclaration(ctx, plugin.PluginConfig{
		Name:    target.Name,
		Run:     target.Run,
		WorkDir: target.WorkDir,
	}, budget)
	if err := ctx.Err(); err != nil {
		return Report{}, fmt.Errorf("the conformance check was stopped while querying %s: %w", target.Name, err)
	}
	// The judge is called for a program that declared nothing too: its check
	// names are what the rows of an unread declaration are listed under.
	evidence.Judgement = server.JudgeDeclaration(target.Name, &evidence.Query.Declaration)

	report := Report{Plugin: target.Name, Run: target.Run, Verdict: VerdictPass}
	for _, check := range registered() {
		report.Rows = append(report.Rows, check.Rows(evidence)...)
	}
	for i := range report.Rows {
		if report.Rows[i].Verdict == VerdictFail {
			report.Verdict = VerdictFail
		}
	}
	return report, nil
}

// DefaultName answers the plugin name a run string is judged under when its
// caller names none: the file name of the program the run string starts. A
// run string with no word in it has no name, and Run refuses it.
func DefaultName(run string) string {
	words := strings.Fields(run)
	if len(words) == 0 {
		return ""
	}
	return filepath.Base(words[0])
}
