// Design: docs/architecture/core-design.md -- the functional area, as one command
// Detail: run.go -- the gating run the bare command reaches
// Overview: suites.go -- the table every row here is derived from
//
// actions.go is the dispatch. Every action is derived from the suite table.
// Adding a suite once updates the dispatch, listing, help, completion, and census.
//
// A bare `le functional` runs the 24 gating suites, with their progress and budget report.
// The area has five more suites that ship but do not gate.
// A sweep of those suites would differ from `make ze-functional-test` despite sharing its name.
// The suite listing is under `le functional list`.
// That listing also supplies the richer table that two external guards read.
//
// One isolated binary set serves the invocation. The command builds that set lazily.
// It reads the label and chaos decision before any suite runs.
// Thus, suites named together share one set and one cleanup.

package functional

import (
	"context"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// listVerb prints the suite table instead of running anything. It is a keyword
// rather than a flag, because the rendering is a pipe operator and the tree is
// the checkout (ai/rules/cli.md).
const listVerb = "list"

// session is one invocation of the functional area: the toolchain it derived,
// and the isolated binary set it builds at most once.
type session struct {
	tc      gotoolchain.Toolchain
	set     BinarySet
	built   bool
	label   string
	chaos   bool
	buildFn func() (BinarySet, error)
}

// binaries answers the set this invocation runs against, building it on first
// use. A command line naming no suite never pays for a build.
func (s *session) binaries() (BinarySet, error) {
	if s.built {
		return s.set, nil
	}
	set, err := s.buildFn()
	if err != nil {
		return BinarySet{}, err
	}
	s.set, s.built = set, true
	return set, nil
}

// release removes the set this invocation built, if it built one.
func (s *session) release() {
	if s.built {
		Release(s.set)
	}
}

// table builds this invocation's action table: one row per suite, the two
// docker-exec analyzer gates, and the verifier-only ExaBGP action.
func table(s *session) leaction.Area {
	rows := make([]leaction.Action, 0, len(Suites)+3)
	for _, suite := range Suites {
		rows = append(rows, leaction.Action{
			Gate:   suite.Target(),
			Why:    suite.Why,
			Answer: s.suiteRunner(suite),
		})
	}
	rows = append(rows,
		leaction.Action{
			Gate: "ze-functional-docker-exec-selftest",
			Why: "every verdict of the fail-open scan fires on a known fixture, so the" +
				" scan below cannot pass vacuously. It runs first, and the gate pair" +
				" shares the same native analyzer",
			Answer: runDockerExecSelftest,
		},
		leaction.Action{
			Gate: "ze-functional-docker-exec-check",
			Why: "the fail-open call-site ratchet over the functional harness Python." +
				" docker_exec_quiet (test/interop/interop.py) returns \"\" on ANY non-zero" +
				" exit, so a caller that does not test the value for emptiness turns a" +
				" command that FAILED into a passing assertion over nothing. The flagged" +
				" set is derived to a fixpoint, so a new wrapper is covered the day it is" +
				" written, and the floor in test/health/docker-exec-baseline.json may" +
				" only go DOWN",
			Answer: s.runDockerExecCheck,
		},
		leaction.Action{
			Verb:   "exabgp-test",
			Why:    "build the isolated ze and ze-test subjects and run every ExaBGP compatibility case",
			Answer: s.runExaBGP,
		},
	)
	return leaction.New(Area, rows...)
}

// suiteRunner answers the action that runs one suite under its own cap.
func (s *session) suiteRunner(suite Suite) func() (any, int) {
	return func() (any, int) {
		set, err := s.binaries()
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		code, seconds := Execute(s.tc, suite, set, "")
		return SuiteRun{
			Suite:   suite.Name,
			Budget:  suite.Budget(),
			Seconds: seconds,
			Code:    code,
			Expired: code == killedByBudget,
		}, code
	}
}

func runDockerExecSelftest() (any, int) {
	report, err := SelftestDockerExec()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}

func (s *session) runDockerExecCheck() (any, int) {
	report, err := CheckDockerExec(s.tc.Root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, report.Code()
}

func (s *session) runExaBGP() (any, int) {
	return RunExaBGP(context.Background(), s.tc.Root, nil)
}

// Gates answers the Make target of every action this area serves, which is what
// the census claims. Read from the same table the dispatch reads, so a gate
// cannot be counted as ported by a command that does not run it.
func Gates() []string { return table(&session{}).Gates() }

// Actions answers the command surface as data.
func Actions() leaction.List { return table(&session{}).Actions() }

// Subs is the one-line hint help renders under the command.
//
// The hint shows the shape of a verb instead of all 32 verbs.
// This area differs because leaction usually derives a hint by naming every action.
// A list of 32 names is not a useful hint. The next keyword reveals those names.
func Subs() string {
	var tb textbuf.Buffer
	return tb.Str(listVerb).Str(" (every suite and its budget) | <suite>-test | ").
		Str("docker-exec-selftest | docker-exec-check | exabgp-test").String()
}

// Answer is the `le functional` command.
func Answer(args []string) (any, int) {
	if len(args) == 1 && args[0] == listVerb {
		return Catalog(), 0
	}

	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	tc, err := gotoolchain.New(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	if len(args) == 0 {
		return RunGating(tc)
	}

	current := newSession(tc, args)
	defer current.release()
	// StopAtFirstFailure matches `make a b` and the former two-command docker-exec target.
	// The selftest proves that the scan's verdicts fire.
	// A scan after a failed selftest would report findings from a checker just shown broken.
	return table(current).Sweep(args, leaction.StopAtFirstFailure)
}

// newSession reads the whole command line before anything runs.
// The binary set therefore carries the label and chaos dashboard for the named suites.
func newSession(tc gotoolchain.Toolchain, args []string) *session {
	named := make([]Suite, 0, len(args))
	for _, verb := range args {
		if suite, ok := suiteForVerb(verb); ok {
			named = append(named, suite)
		}
	}

	current := &session{tc: tc, label: "ze-functional-test"}
	if len(named) == 1 {
		current.label = named[0].Target()
	}
	for _, suite := range named {
		if suite.Chaos {
			current.chaos = true
		}
	}
	current.buildFn = func() (BinarySet, error) {
		return Prepare(current.tc, current.label, current.chaos)
	}
	return current
}

// suiteForVerb answers the suite a typed verb names. The verb is the suite's
// Make target with the area's own prefix removed, which is what leaction
// derives and what a developer types.
func suiteForVerb(verb string) (Suite, bool) {
	name, isSuite := strings.CutSuffix(verb, "-test")
	if !isSuite {
		return Suite{}, false
	}
	return SuiteNamed(name)
}
